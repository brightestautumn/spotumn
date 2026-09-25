// Semantic Lip Gloss styles, Unicode width truncation, text formatting, and glyph rendering.
package theme

import (
	"os"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	StyleBase lipgloss.Style

	StyleNormal               lipgloss.Style
	StyleFaint                lipgloss.Style
	StyleBold                 lipgloss.Style
	StylePrimary              lipgloss.Style
	StyleSecondary            lipgloss.Style
	StyleTertiary             lipgloss.Style
	StyleHover                lipgloss.Style
	StyleError                lipgloss.Style
	StylePlaying              lipgloss.Style
	StyleActiveFocused        lipgloss.Style
	StyleActiveUnfocused      lipgloss.Style
	StyleActiveFocusedBlock   lipgloss.Style
	StyleActiveUnfocusedBlock lipgloss.Style
	StyleActiveLyricsBlock    lipgloss.Style
)

func init() {
	isDark := true
	if termIn, err := os.Open("/dev/tty"); err == nil {
		defer termIn.Close()
		isDark = lipgloss.HasDarkBackground(termIn, termIn)
	}
	SetTheme(isDark)
}

func ApplyTheme(themeName string, isDark bool) {
	CurrentTheme = LoadTheme(themeName, isDark)
	UpdateStyles()
}

func UpdateStyles() {
	StyleBase = lipgloss.NewStyle().Background(CurrentTheme.Surface)

	StyleNormal = StyleBase.Foreground(CurrentTheme.OnSurface)
	StyleFaint = StyleBase.Foreground(CurrentTheme.Outline)
	StyleBold = StyleBase.Bold(true).Foreground(CurrentTheme.OnSurface)

	StylePrimary = StyleBase.Foreground(CurrentTheme.Primary).Bold(true)
	StyleSecondary = StyleBase.Foreground(CurrentTheme.Secondary).Bold(true)
	StyleTertiary = StyleBase.Foreground(CurrentTheme.Tertiary).Bold(true)
	StyleHover = StyleBase.Foreground(CurrentTheme.Hover)
	StyleError = StyleBase.Foreground(CurrentTheme.Error).Bold(true)
	StylePlaying = StyleBase.Foreground(CurrentTheme.Tertiary).Bold(true)
	StyleActiveFocused = StyleBase.Foreground(CurrentTheme.Primary).Bold(true)
	StyleActiveUnfocused = StyleBase.Foreground(CurrentTheme.Secondary).Bold(true)

	StyleActiveFocusedBlock = lipgloss.NewStyle().
		Foreground(CurrentTheme.Primary).
		Bold(true)

	StyleActiveUnfocusedBlock = lipgloss.NewStyle().
		Foreground(CurrentTheme.Secondary).
		Bold(true)

	StyleActiveLyricsBlock = lipgloss.NewStyle().
		Foreground(CurrentTheme.Tertiary).
		Bold(true)
}

func BgPad(n int) string {
	if n <= 0 {
		return ""
	}
	return StyleBase.Render(strings.Repeat(" ", n))
}

// identify complex scripts and combining marks that standard width estimators miscalculate
func isComplexCombiningMark(r rune) bool {
	if !unicode.Is(unicode.M, r) {
		return false
	}
	return (r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x0590 && r <= 0x08FF) ||
		(r >= 0x0900 && r <= 0x0DFF) ||
		(r >= 0x0E00 && r <= 0x0EFF) ||
		(r >= 0x0F00 && r <= 0x0FFF) ||
		(r >= 0x1000 && r <= 0x109F) ||
		(r >= 0x1780 && r <= 0x17FF) ||
		(r >= 0x1900 && r <= 0x1BFF) ||
		(r >= 0x1CD0 && r <= 0x1CFF) ||
		(r >= 0x1DC0 && r <= 0x1DFF) ||
		(r >= 0x20D0 && r <= 0x20FF) ||
		(r >= 0xA800 && r <= 0xABFF) ||
		(r >= 0xFE20 && r <= 0xFE2F)
}

func runeDisplayWidth(r rune) int {
	w := ansi.StringWidth(string(r))
	if w == 0 && isComplexCombiningMark(r) {
		return 1
	}
	if unicode.Is(unicode.Mc, r) && w == 0 {
		return 1
	}
	return w
}

func StringDisplayWidth(s string) int {
	w := ansi.StringWidth(s)
	for _, r := range s {
		if isComplexCombiningMark(r) && ansi.StringWidth(string(r)) == 0 {
			w++
		}
	}
	return w
}

// truncate string by terminal visual cells while keeping ansi escape sequences intact
func TruncateVisualWidth(s string, maxW int, tail string) string {
	if maxW <= 0 {
		return ""
	}
	if StringDisplayWidth(s) <= maxW {
		return s
	}

	tailW := StringDisplayWidth(tail)
	targetW := maxW - tailW
	if targetW < 0 {
		targetW = 0
	}

	var sb strings.Builder
	curW := 0
	inEsc := false
	hadEsc := false

	for i, r := range s {
		if r == 0x1b {
			inEsc = true
			hadEsc = true
			sb.WriteRune(r)
			continue
		}
		if inEsc {
			sb.WriteRune(r)
			if r == 'm' || r == 'K' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z' && r != 'm') {
				inEsc = false
			}
			continue
		}

		rw := runeDisplayWidth(r)
		if curW+rw > targetW {
			_ = i
			break
		}
		curW += rw
		sb.WriteRune(r)
	}

	sb.WriteString(tail)
	if hadEsc {
		sb.WriteString("\x1b[0m")
	}
	return sb.String()
}

func RenderPaddedLine(rawText string, style lipgloss.Style, width int) string {
	if width <= 0 {
		return ""
	}
	trunc := TruncateString(rawText, width)
	w := StringDisplayWidth(trunc)
	pad := width - w
	if pad < 0 {
		pad = 0
	}
	return style.Render(trunc + strings.Repeat(" ", pad))
}

func PanelBox(focused bool, width, height int) lipgloss.Style {
	st := lipgloss.NewStyle().
		Width(width).
		Background(CurrentTheme.Surface)
	if height > 0 {
		st = st.Height(height)
	}
	if focused {
		return st.Border(lipgloss.ThickBorder()).
			BorderForeground(CurrentTheme.Primary).
			BorderBackground(CurrentTheme.Surface).
			Bold(true)
	}
	return st.Border(lipgloss.RoundedBorder()).
		BorderForeground(CurrentTheme.Outline).
		BorderBackground(CurrentTheme.Surface)
}

func PadPlain(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = TruncateString(s, width)
	w := StringDisplayWidth(s)
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func CenterLine(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if s != "" && !strings.Contains(s, "\x1b[") {
		s = StyleNormal.Render(s)
	}
	w := StringDisplayWidth(s)
	if w >= width {
		return TruncateString(s, width)
	}
	padLeft := (width - w) / 2
	padRight := width - w - padLeft
	return BgPad(padLeft) + s + BgPad(padRight)
}

func CenterOverlay(modal string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(modal, "\n")
	topPad := (height - len(lines)) / 2
	if topPad < 0 {
		topPad = 0
	}
	emptyRow := BgPad(width)
	var rows []string
	for i := 0; i < topPad; i++ {
		rows = append(rows, emptyRow)
	}
	for _, line := range lines {
		rows = append(rows, CenterLine(line, width))
	}
	for len(rows) < height {
		rows = append(rows, emptyRow)
	}
	if len(rows) > height {
		rows = rows[:height]
	}
	return strings.Join(rows, "\n")
}

func PadToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if s != "" && !strings.Contains(s, "\x1b[") {
		s = StyleNormal.Render(s)
	}
	w := StringDisplayWidth(s)
	if w > width {
		s = ansi.Truncate(s, width, "…")
		w = StringDisplayWidth(s)
	}
	pad := width - w
	if pad <= 0 {
		return s
	}
	return s + BgPad(pad)
}

func TruncateString(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if StringDisplayWidth(s) <= maxW {
		return s
	}
	return TruncateVisualWidth(s, maxW, "…")
}

func FormatDuration(ms int) string {
	totalSec := ms / 1000
	min := totalSec / 60
	sec := totalSec % 60
	return formatTwoDigits(min) + ":" + formatTwoDigits(sec)
}

func formatTwoDigits(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+(n/10))) + string(rune('0'+(n%10)))
}

func DeviceTypeGlyph(devType string) string {
	switch strings.ToLower(devType) {
	case "computer":
		return "󰍹 "
	case "smartphone", "phone":
		return "󰄡 "
	case "speaker":
		return "󰓃 "
	case "cast", "castaudio", "castvideo":
		return "󰋋 "
	default:
		return "󰓃 "
	}
}

func RenderProgressBar(ratio float64, width int) string {
	if width <= 0 {
		return ""
	}
	filledChars := int(ratio * float64(width))
	if filledChars > width {
		filledChars = width
	}
	emptyChars := width - filledChars

	filled := lipgloss.NewStyle().Foreground(CurrentTheme.Tertiary).Background(CurrentTheme.Surface).Bold(true).Render(strings.Repeat("━", filledChars))
	thumb := lipgloss.NewStyle().Foreground(CurrentTheme.Tertiary).Background(CurrentTheme.Surface).Bold(true).Render("●")

	if emptyChars > 0 {
		empty := StyleFaint.Render(strings.Repeat("─", emptyChars-1))
		return filled + thumb + empty
	}
	return filled
}
