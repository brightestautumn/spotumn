// Album art fetcher and caching engine - downloads covers and converts images to terminal graphics.
package art

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"spotumn/internal/config"
)

type Renderer struct {
	client        *http.Client
	cacheDir      string
	mode          string
	hasChafa      bool
	memCache      map[string]string
	diskCache     map[string]string
	cacheKeys     []string
	diskCacheKeys []string
	mu            sync.Mutex
}

func NewRenderer(mode string) *Renderer {
	cacheDir := filepath.Join(config.GetCacheDir(), "art")
	_ = os.MkdirAll(cacheDir, 0700)

	hasChafa := false
	if _, err := exec.LookPath("chafa"); err == nil {
		hasChafa = true
	}

	if mode == "" {
		mode = "auto"
	}

	return &Renderer{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		cacheDir:      cacheDir,
		mode:          strings.ToLower(strings.TrimSpace(mode)),
		hasChafa:      hasChafa,
		memCache:      make(map[string]string, 4),
		diskCache:     make(map[string]string, 50),
		cacheKeys:     make([]string, 0, 4),
		diskCacheKeys: make([]string, 0, 50),
	}
}

func (r *Renderer) Mode() string {
	return r.mode
}

func (r *Renderer) Render(imageURL string, width, height int) (string, string, error) {
	if imageURL == "" || width <= 0 || height <= 0 {
		return "", "", nil
	}

	key := fmt.Sprintf("%s:%d:%d", imageURL, width, height)

	r.mu.Lock()
	if val, ok := r.memCache[key]; ok {
		diskPath := r.diskCache[imageURL]
		r.mu.Unlock()
		return val, diskPath, nil
	}
	r.mu.Unlock()

	diskPath, img, err := r.ensureImage(imageURL)
	if err != nil {
		return "", "", err
	}

	var rendered string

	// try chafa rendering first, falling back to pure ansi half-blocks
	if r.hasChafa && diskPath != "" && r.mode != "ansi" {
		if out, err := r.renderWithChafa(diskPath, width, height); err == nil && len(strings.TrimSpace(out)) > 0 {
			rendered = out
		}
	}

	if rendered == "" && img != nil {
		rendered = r.ToHalfBlocks(img, width, height)
	}

	r.mu.Lock()
	if len(r.cacheKeys) >= 4 {
		oldest := r.cacheKeys[0]
		r.cacheKeys = r.cacheKeys[1:]
		delete(r.memCache, oldest)
	}
	if len(r.diskCacheKeys) >= 50 {
		oldestURL := r.diskCacheKeys[0]
		r.diskCacheKeys = r.diskCacheKeys[1:]
		delete(r.diskCache, oldestURL)
	}
	r.memCache[key] = rendered
	if _, exists := r.diskCache[imageURL]; !exists {
		r.diskCache[imageURL] = diskPath
		r.diskCacheKeys = append(r.diskCacheKeys, imageURL)
	}
	r.cacheKeys = append(r.cacheKeys, key)
	r.mu.Unlock()

	return rendered, diskPath, nil
}

func (r *Renderer) renderWithChafa(diskPath string, width, height int) (string, error) {
	cmd := exec.Command("chafa",
		"--probe=off",
		"-c", "full",
		"--format=symbols",
		"--symbols=block+sextant",
		fmt.Sprintf("--size=%dx%d", width, height),
		diskPath,
	)
	out, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return cleanChafaOutput(string(out)), nil
	}

	cmd = exec.Command("chafa",
		"--probe=off",
		"-c", "full",
		"--format=symbols",
		"--symbols=quad+half",
		fmt.Sprintf("--size=%dx%d", width, height),
		diskPath,
	)
	out, err = cmd.Output()
	if err != nil {
		return "", err
	}
	return cleanChafaOutput(string(out)), nil
}

func cleanChafaOutput(s string) string {
	s = strings.ReplaceAll(s, "\x1b[?25l", "")
	s = strings.ReplaceAll(s, "\x1b[?25h", "")
	return strings.TrimRight(s, "\r\n")
}

func (r *Renderer) ensureImage(imageURL string) (string, image.Image, error) {
	if !strings.HasPrefix(imageURL, "https://") {
		return "", nil, fmt.Errorf("insecure or invalid image URL: %s", imageURL)
	}

	h := sha256.Sum256([]byte(imageURL))
	diskPath := filepath.Join(r.cacheDir, hex.EncodeToString(h[:]))

	if f, err := os.Open(diskPath); err == nil {
		defer f.Close()
		img, _, err := image.Decode(f)
		if err == nil {
			return diskPath, img, nil
		}
	}

	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return "", nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("image fetch failed with status %d", resp.StatusCode)
	}

	lr := io.LimitReader(resp.Body, 10*1024*1024)
	tmpFile, err := os.CreateTemp(r.cacheDir, "art-*")
	if err != nil {
		return "", nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, lr); err != nil {
		tmpFile.Close()
		return "", nil, err
	}
	tmpFile.Close()

	_ = os.Rename(tmpFile.Name(), diskPath)

	f, err := os.Open(diskPath)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", nil, err
	}

	return diskPath, img, nil
}

// downscale image into half-block unicode cells using top/bottom truecolor fg/bg
func (r *Renderer) ToHalfBlocks(img image.Image, targetW, targetH int) string {
	bounds := img.Bounds()
	imgW := bounds.Dx()
	imgH := bounds.Dy()
	if imgW == 0 || imgH == 0 {
		return ""
	}

	pixelH := targetH * 2
	pixelW := targetW

	var sb strings.Builder
	sb.Grow(pixelH * pixelW * 42)

	for y := 0; y < pixelH; y += 2 {
		for x := 0; x < pixelW; x++ {
			srcX := bounds.Min.X + (x * imgW / pixelW)
			srcYTop := bounds.Min.Y + (y * imgH / pixelH)
			srcYBot := bounds.Min.Y + ((y + 1) * imgH / pixelH)

			rTop, gTop, bTop, _ := img.At(srcX, srcYTop).RGBA()
			rBot, gBot, bBot, _ := img.At(srcX, srcYBot).RGBA()

			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀\x1b[0m",
				rTop>>8, gTop>>8, bTop>>8,
				rBot>>8, gBot>>8, bBot>>8,
			)
		}
		if y+2 < pixelH {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}
