// Theme palette loader - parses Catppuccin, Nord, Gruvbox, and custom JSON color schemes with comments.
package theme

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"spotumn/internal/config"

	"charm.land/lipgloss/v2"
)

type ThemeConfig struct {
	MPrimary          string `json:"mPrimary"`
	MOnPrimary        string `json:"mOnPrimary"`
	MSecondary        string `json:"mSecondary"`
	MOnSecondary      string `json:"mOnSecondary"`
	MTertiary         string `json:"mTertiary"`
	MOnTertiary       string `json:"mOnTertiary"`
	MError            string `json:"mError"`
	MOnError          string `json:"mOnError"`
	MSurface          string `json:"mSurface"`
	MOnSurface        string `json:"mOnSurface"`
	MSurfaceVariant   string `json:"mSurfaceVariant"`
	MOnSurfaceVariant string `json:"mOnSurfaceVariant"`
	MOutline          string `json:"mOutline"`
	MShadow           string `json:"mShadow"`
	MHover            string `json:"mHover"`
	MOnHover          string `json:"mOnHover"`
	MGold             string `json:"mGold"`
	MSuccess          string `json:"mSuccess"`
}

type ThemeFile struct {
	Dark  ThemeConfig `json:"dark"`
	Light ThemeConfig `json:"light"`
}

type ThemePalette struct {
	Primary          color.Color
	OnPrimary        color.Color
	Secondary        color.Color
	OnSecondary      color.Color
	Tertiary         color.Color
	OnTertiary       color.Color
	Error            color.Color
	OnError          color.Color
	Surface          color.Color
	OnSurface        color.Color
	SurfaceVariant   color.Color
	OnSurfaceVariant color.Color
	Outline          color.Color
	Shadow           color.Color
	Hover            color.Color
	OnHover          color.Color
	Gold             color.Color
	Success          color.Color
}

var BuiltinPresets = map[string]ThemeFile{
	"spotify": {
		Dark: ThemeConfig{
			MPrimary: "#1db954", MOnPrimary: "#000000",
			MSecondary: "#1ed760", MOnSecondary: "#000000",
			MTertiary: "#1db954", MOnTertiary: "#000000",
			MError: "#e91429", MOnError: "#ffffff",
			MSurface: "#121212", MOnSurface: "#ffffff",
			MSurfaceVariant: "#282828", MOnSurfaceVariant: "#b3b3b3",
			MOutline: "#535353", MShadow: "#000000",
			MHover: "#1ed760", MOnHover: "#000000",
			MGold: "#f59b23", MSuccess: "#1db954",
		},
		Light: ThemeConfig{
			MPrimary: "#1db954", MOnPrimary: "#ffffff",
			MSecondary: "#169c46", MOnSecondary: "#ffffff",
			MTertiary: "#1db954", MOnTertiary: "#ffffff",
			MError: "#e91429", MOnError: "#ffffff",
			MSurface: "#cbc194ff", MOnSurface: "#121212",
			MSurfaceVariant: "#f2f2f2", MOnSurfaceVariant: "#6a6a6a",
			MOutline: "#cccccc", MShadow: "#000000",
			MHover: "#169c46", MOnHover: "#ffffff",
			MGold: "#d97706", MSuccess: "#1db954",
		},
	},
	"catppuccin": {
		Dark: ThemeConfig{
			MPrimary: "#cba6f7", MOnPrimary: "#11111b",
			MSecondary: "#b4befe", MOnSecondary: "#11111b",
			MTertiary: "#94e2d5", MOnTertiary: "#11111b",
			MError: "#f38ba8", MOnError: "#11111b",
			MSurface: "#1e1e2e", MOnSurface: "#cdd6f4",
			MSurfaceVariant: "#313244", MOnSurfaceVariant: "#a6adc8",
			MOutline: "#6c7086", MShadow: "#11111b",
			MHover: "#fab387", MOnHover: "#11111b",
			MGold: "#f9e2af", MSuccess: "#a6e3a1",
		},
		Light: ThemeConfig{
			MPrimary: "#8839ef", MOnPrimary: "#ffffff",
			MSecondary: "#7287fd", MOnSecondary: "#ffffff",
			MTertiary: "#179299", MOnTertiary: "#ffffff",
			MError: "#d20f39", MOnError: "#ffffff",
			MSurface: "#eff1f5", MOnSurface: "#4c4f69",
			MSurfaceVariant: "#ccd0da", MOnSurfaceVariant: "#6c6f85",
			MOutline: "#9ca0b0", MShadow: "#dce0e8",
			MHover: "#fe640b", MOnHover: "#ffffff",
			MGold: "#df8e1d", MSuccess: "#40a02b",
		},
	},
	"dracula": {
		Dark: ThemeConfig{
			MPrimary: "#bd93f9", MOnPrimary: "#282a36",
			MSecondary: "#ff79c6", MOnSecondary: "#282a36",
			MTertiary: "#8be9fd", MOnTertiary: "#282a36",
			MError: "#ff5555", MOnError: "#282a36",
			MSurface: "#282a36", MOnSurface: "#f8f8f2",
			MSurfaceVariant: "#44475a", MOnSurfaceVariant: "#bfbfbf",
			MOutline: "#6272a4", MShadow: "#191a21",
			MHover: "#ffb86c", MOnHover: "#282a36",
			MGold: "#f1fa8c", MSuccess: "#50fa7b",
		},
		Light: ThemeConfig{
			MPrimary: "#7b5ea7", MOnPrimary: "#ffffff",
			MSecondary: "#b83280", MOnSecondary: "#ffffff",
			MTertiary: "#0d768a", MOnTertiary: "#ffffff",
			MError: "#cb1a1a", MOnError: "#ffffff",
			MSurface: "#f8f8f2", MOnSurface: "#282a36",
			MSurfaceVariant: "#e2e4e9", MOnSurfaceVariant: "#6272a4",
			MOutline: "#a0a4b8", MShadow: "#d5d7e0",
			MHover: "#c96a00", MOnHover: "#ffffff",
			MGold: "#b08800", MSuccess: "#2b8a3e",
		},
	},
	"gruvbox": {
		Dark: ThemeConfig{
			MPrimary: "#fe8019", MOnPrimary: "#282828",
			MSecondary: "#fabd2f", MOnSecondary: "#282828",
			MTertiary: "#8ec07c", MOnTertiary: "#282828",
			MError: "#fb4934", MOnError: "#282828",
			MSurface: "#282828", MOnSurface: "#ebdbb2",
			MSurfaceVariant: "#3c3836", MOnSurfaceVariant: "#a89984",
			MOutline: "#665c54", MShadow: "#1d2021",
			MHover: "#d3869b", MOnHover: "#282828",
			MGold: "#fabd2f", MSuccess: "#b8bb26",
		},
		Light: ThemeConfig{
			MPrimary: "#b57614", MOnPrimary: "#fbf1c7",
			MSecondary: "#af3a03", MOnSecondary: "#fbf1c7",
			MTertiary: "#427b58", MOnTertiary: "#fbf1c7",
			MError: "#9d0006", MOnError: "#fbf1c7",
			MSurface: "#fbf1c7", MOnSurface: "#3c3836",
			MSurfaceVariant: "#ebdbb2", MOnSurfaceVariant: "#7c6f64",
			MOutline: "#bdae93", MShadow: "#d5c4a1",
			MHover: "#8f3f71", MOnHover: "#fbf1c7",
			MGold: "#b57614", MSuccess: "#79740e",
		},
	},
	"monochrome": {
		Dark: ThemeConfig{
			MPrimary: "#ffffff", MOnPrimary: "#000000",
			MSecondary: "#ffffff", MOnSecondary: "#000000",
			MTertiary: "#ffffff", MOnTertiary: "#000000",
			MError: "#ffffff", MOnError: "#000000",
			MSurface: "#000000", MOnSurface: "#ffffff",
			MSurfaceVariant: "#222222", MOnSurfaceVariant: "#888888",
			MOutline: "#555555", MShadow: "#000000",
			MHover: "#ffffff", MOnHover: "#000000",
			MGold: "#ffffff", MSuccess: "#ffffff",
		},
		Light: ThemeConfig{
			MPrimary: "#000000", MOnPrimary: "#ffffff",
			MSecondary: "#000000", MOnSecondary: "#ffffff",
			MTertiary: "#000000", MOnTertiary: "#ffffff",
			MError: "#000000", MOnError: "#ffffff",
			MSurface: "#ffffff", MOnSurface: "#000000",
			MSurfaceVariant: "#dddddd", MOnSurfaceVariant: "#777777",
			MOutline: "#aaaaaa", MShadow: "#ffffff",
			MHover: "#000000", MOnHover: "#ffffff",
			MGold: "#000000", MSuccess: "#000000",
		},
	},
	"tokyonight": {
		Dark: ThemeConfig{
			MPrimary: "#7aa2f7", MOnPrimary: "#1a1b26",
			MSecondary: "#bb9af7", MOnSecondary: "#1a1b26",
			MTertiary: "#7dcfff", MOnTertiary: "#1a1b26",
			MError: "#f7768e", MOnError: "#1a1b26",
			MSurface: "#1a1b26", MOnSurface: "#c0caf5",
			MSurfaceVariant: "#24283b", MOnSurfaceVariant: "#787c99",
			MOutline: "#565f89", MShadow: "#16161e",
			MHover: "#ff9e64", MOnHover: "#1a1b26",
			MGold: "#e0af68", MSuccess: "#9ece6a",
		},
		Light: ThemeConfig{
			MPrimary: "#2e7de9", MOnPrimary: "#ffffff",
			MSecondary: "#7847bd", MOnSecondary: "#ffffff",
			MTertiary: "#007197", MOnTertiary: "#ffffff",
			MError: "#8c4351", MOnError: "#ffffff",
			MSurface: "#e1e2e7", MOnSurface: "#3760bf",
			MSurfaceVariant: "#cbccd1", MOnSurfaceVariant: "#6172b0",
			MOutline: "#8990b3", MShadow: "#d0d1d6",
			MHover: "#b15c00", MOnHover: "#ffffff",
			MGold: "#8f5e15", MSuccess: "#387068",
		},
	},
}

// clean out single and multiline comments so json with comments unmarshals cleanly
func stripJSONComments(data []byte) []byte {
	var out []byte
	inString := false
	inSingleComment := false
	inMultiComment := false
	escape := false

	n := len(data)
	for i := 0; i < n; i++ {
		b := data[i]

		if inSingleComment {
			if b == '\n' {
				inSingleComment = false
				out = append(out, b)
			}
			continue
		}

		if inMultiComment {
			if b == '*' && i+1 < n && data[i+1] == '/' {
				inMultiComment = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, b)
			if escape {
				escape = false
			} else if b == '\\' {
				escape = true
			} else if b == '"' {
				inString = false
			}
			continue
		}

		if b == '"' {
			inString = true
			out = append(out, b)
			continue
		}

		if b == '/' && i+1 < n {
			if data[i+1] == '/' {
				inSingleComment = true
				i++
				continue
			} else if data[i+1] == '*' {
				inMultiComment = true
				i++
				continue
			}
		}

		out = append(out, b)
	}

	return out
}

func ValidateThemeFile(data []byte) (*ThemeFile, error) {
	cleaned := stripJSONComments(data)
	var tf ThemeFile
	if err := json.Unmarshal(cleaned, &tf); err != nil {
		var flat ThemeConfig
		if err2 := json.Unmarshal(cleaned, &flat); err2 == nil && flat.MSurface != "" && flat.MPrimary != "" {
			return &ThemeFile{Dark: flat, Light: flat}, nil
		}
		return nil, err
	}
	if tf.Dark.MSurface == "" && tf.Light.MSurface != "" {
		tf.Dark = tf.Light
	} else if tf.Light.MSurface == "" && tf.Dark.MSurface != "" {
		tf.Light = tf.Dark
	}
	if tf.Dark.MSurface == "" || tf.Dark.MPrimary == "" {
		return nil, fmt.Errorf("theme missing required color fields")
	}
	return &tf, nil
}

func ListAvailableThemes() []string {
	seen := make(map[string]bool)
	var themes []string

	presets := []string{"spotify", "catppuccin", "dracula", "gruvbox", "monochrome", "tokyonight"}
	for _, p := range presets {
		seen[p] = true
		themes = append(themes, p)
	}

	dir := config.GetThemesDir()
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				if config.LogError != nil {
					config.LogError("ui.theme", "read theme file: "+err.Error(), "palette.go")
				}
				continue
			}
			if _, err := ValidateThemeFile(data); err == nil {
				name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
				if !seen[name] {
					seen[name] = true
					themes = append(themes, name)
				}
			} else if config.LogError != nil {
				config.LogError("ui.theme", "invalid theme "+e.Name()+": "+err.Error(), "palette.go")
			}
		}
	}
	return themes
}

func LoadTheme(themeName string, isDark bool) ThemePalette {
	themeName = strings.ToLower(strings.TrimSpace(themeName))
	if themeName == "" {
		themeName = "spotify"
	}

	themePath := filepath.Join(config.GetThemesDir(), themeName+".json")
	if data, err := os.ReadFile(themePath); err == nil {
		if tf, err := ValidateThemeFile(data); err == nil {
			if isDark {
				return resolveConfig(tf.Dark)
			}
			return resolveConfig(tf.Light)
		} else if config.LogError != nil {
			config.LogError("ui.theme", "invalid theme "+themeName+": "+err.Error(), "palette.go")
		}
	} else if !os.IsNotExist(err) && config.LogError != nil {
		config.LogError("ui.theme", "read theme "+themeName+": "+err.Error(), "palette.go")
	}

	if tf, ok := BuiltinPresets[themeName]; ok {
		if isDark {
			return resolveConfig(tf.Dark)
		}
		return resolveConfig(tf.Light)
	}

	tf := BuiltinPresets["spotify"]
	if isDark {
		return resolveConfig(tf.Dark)
	}
	return resolveConfig(tf.Light)
}

func resolveConfig(cfg ThemeConfig) ThemePalette {
	return ThemePalette{
		Primary:          lipgloss.Color(cfg.MPrimary),
		OnPrimary:        lipgloss.Color(cfg.MOnPrimary),
		Secondary:        lipgloss.Color(cfg.MSecondary),
		OnSecondary:      lipgloss.Color(cfg.MOnSecondary),
		Tertiary:         lipgloss.Color(cfg.MTertiary),
		OnTertiary:       lipgloss.Color(cfg.MOnTertiary),
		Error:            lipgloss.Color(cfg.MError),
		OnError:          lipgloss.Color(cfg.MOnError),
		Surface:          lipgloss.Color(cfg.MSurface),
		OnSurface:        lipgloss.Color(cfg.MOnSurface),
		SurfaceVariant:   lipgloss.Color(cfg.MSurfaceVariant),
		OnSurfaceVariant: lipgloss.Color(cfg.MOnSurfaceVariant),
		Outline:          lipgloss.Color(cfg.MOutline),
		Shadow:           lipgloss.Color(cfg.MShadow),
		Hover:            lipgloss.Color(cfg.MHover),
		OnHover:          lipgloss.Color(cfg.MOnHover),
		Gold:             lipgloss.Color(cfg.MGold),
		Success:          lipgloss.Color(cfg.MSuccess),
	}
}

func CatppuccinMocha() ThemeConfig {
	return BuiltinPresets["catppuccin"].Dark
}

func catppuccinMocha() ThemeConfig {
	return CatppuccinMocha()
}

func MergeConfig(base, custom ThemeConfig) ThemeConfig {
	if custom.MPrimary != "" {
		base.MPrimary = custom.MPrimary
	}
	if custom.MOnPrimary != "" {
		base.MOnPrimary = custom.MOnPrimary
	}
	if custom.MSecondary != "" {
		base.MSecondary = custom.MSecondary
	}
	if custom.MOnSecondary != "" {
		base.MOnSecondary = custom.MOnSecondary
	}
	if custom.MTertiary != "" {
		base.MTertiary = custom.MTertiary
	}
	if custom.MOnTertiary != "" {
		base.MOnTertiary = custom.MOnTertiary
	}
	if custom.MError != "" {
		base.MError = custom.MError
	}
	if custom.MOnError != "" {
		base.MOnError = custom.MOnError
	}
	if custom.MSurface != "" {
		base.MSurface = custom.MSurface
	}
	if custom.MOnSurface != "" {
		base.MOnSurface = custom.MOnSurface
	}
	if custom.MSurfaceVariant != "" {
		base.MSurfaceVariant = custom.MSurfaceVariant
	}
	if custom.MOnSurfaceVariant != "" {
		base.MOnSurfaceVariant = custom.MOnSurfaceVariant
	}
	if custom.MOutline != "" {
		base.MOutline = custom.MOutline
	}
	if custom.MShadow != "" {
		base.MShadow = custom.MShadow
	}
	if custom.MHover != "" {
		base.MHover = custom.MHover
	}
	if custom.MOnHover != "" {
		base.MOnHover = custom.MOnHover
	}
	if custom.MGold != "" {
		base.MGold = custom.MGold
	}
	if custom.MSuccess != "" {
		base.MSuccess = custom.MSuccess
	}
	return base
}

func mergeConfig(base, custom ThemeConfig) ThemeConfig {
	return MergeConfig(base, custom)
}

func LoadThemeFile(customPath ...string) ThemeConfig {
	if len(customPath) > 0 && customPath[0] != "" {
		data, err := os.ReadFile(customPath[0])
		if err == nil {
			var tf ThemeFile
			if err := json.Unmarshal(data, &tf); err == nil {
				return tf.Dark
			}
		}
	}
	return BuiltinPresets["spotify"].Dark
}

var CurrentTheme ThemePalette

func SetTheme(isDark bool) {
	cfg := config.Get()
	if cfg.AppearanceMode == "dark" {
		isDark = true
	} else if cfg.AppearanceMode == "light" {
		isDark = false
	}
	themeName := cfg.Theme
	if themeName == "" {
		themeName = "spotify"
	}
	ApplyTheme(themeName, isDark)
}

func SetThemeFromConfig(cfg ThemeConfig) {
	CurrentTheme = resolveConfig(cfg)
	UpdateStyles()
}
