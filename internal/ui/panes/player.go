// Bottom playback bar - displays track info, artist, progress bar, time counters, volume, and player state.
package panes

import (
	"fmt"
	"strings"

	"spotumn/internal/backend"
	"spotumn/internal/ui/theme"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// render bottom player bar with track info, progress bar, shuffle/repeat indicators, and volume
func RenderPlayerLines(state *backend.PlaybackState, focused bool, width int) []string {
	if width < 30 {
		width = 30
	}

	isPlaying := false
	progressMs := 0
	durationMs := 0
	volume := 50
	shuffle := false
	repeat := "off"
	trackName := "No playback active"
	artistName := ""

	if state != nil {
		isPlaying = state.Playing
		progressMs = state.ProgressMs
		durationMs = state.DurationMs
		volume = state.Volume
		shuffle = state.Shuffle
		repeat = state.Repeat
		if state.CurrentTrack != nil {
			trackName = state.CurrentTrack.Name
			artistName = state.CurrentTrack.Artist
			if durationMs == 0 {
				durationMs = state.CurrentTrack.DurationMs
			}
		}
	}

	var lines []string

	leftText := " ♫ " + trackName
	if artistName != "" {
		leftText += " - " + artistName
	}
	leftStyled := theme.StyleBold.Render(theme.TruncateString(leftText, width/3))

	playIcon := theme.StyleTertiary.Render("▶")
	if isPlaying {
		playIcon = theme.StyleTertiary.Render("❚❚")
	}

	var shuffIcon string
	if shuffle {
		shuffIcon = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render("󰒝")
	} else {
		shuffIcon = theme.StyleFaint.Render("󰒞")
	}

	var repIcon string
	switch repeat {
	case "track":
		repIcon = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render("󰑘")
	case "context":
		repIcon = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render("󰑖")
	default:
		repIcon = theme.StyleFaint.Render("󰑗")
	}

	prevIcon := theme.StylePrimary.Render("⏮")
	nextIcon := theme.StylePrimary.Render("⏭")
	ctrlsStyled := shuffIcon + theme.BgPad(3) + prevIcon + theme.BgPad(3) + playIcon + theme.BgPad(3) + nextIcon + theme.BgPad(3) + repIcon

	volBar := RenderMiniSlider(volume, 8)
	volPrefix := theme.StyleFaint.Render("Vol: [")
	volSuffix := theme.StyleFaint.Render(fmt.Sprintf("] %2d%% ", volume))
	rightStyled := volPrefix + volBar + volSuffix

	lines = append(lines, alignRow(leftStyled, ctrlsStyled, rightStyled, width))

	elapsedStr := theme.FormatDuration(progressMs)
	totalStr := theme.FormatDuration(durationMs)
	if durationMs == 0 {
		totalStr = "--:--"
	}

	seekW := width - len(elapsedStr) - len(totalStr) - 4
	if seekW < 10 {
		seekW = 10
	}
	seekRatio := 0.0
	if durationMs > 0 {
		seekRatio = float64(progressMs) / float64(durationMs)
		if seekRatio > 1.0 {
			seekRatio = 1.0
		}
	}
	seekBar := renderProgressBar(seekRatio, seekW)
	seekLine := theme.BgPad(1) + theme.StyleFaint.Render(elapsedStr) + theme.BgPad(1) + seekBar + theme.BgPad(1) + theme.StyleFaint.Render(totalStr) + theme.BgPad(1)
	lines = append(lines, theme.PadToWidth(seekLine, width))

	return lines
}

func RenderPlayer(state *backend.PlaybackState, focused bool, width int) string {
	contentW := width - 2
	if contentW < 30 {
		contentW = 30
	}

	lines := RenderPlayerLines(state, focused, contentW)

	devName := "spotumn"
	devGlyph := "󰓃 "
	if state != nil {
		if state.DeviceName != "" {
			devName = state.DeviceName
		}
		if state.DeviceType != "" {
			devGlyph = theme.DeviceTypeGlyph(state.DeviceType)
		}
	}
	devTag := theme.BgPad(1) + theme.StyleSecondary.Render(devGlyph+theme.TruncateString(devName, 18)) + theme.StyleFaint.Render(" ─")
	rawDevTag := " " + devGlyph + theme.TruncateString(devName, 18) + " ─"
	devTagW := ansi.StringWidth(rawDevTag)

	cornerTL := "╭"
	cornerTR := "╮"
	cornerBL := "╰"
	cornerBR := "╯"
	borderStyle := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Outline).Background(theme.CurrentTheme.Surface)
	if focused {
		borderStyle = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Primary).Background(theme.CurrentTheme.Surface).Bold(true)
	}

	dashesLen := contentW - devTagW
	if dashesLen < 1 {
		dashesLen = 1
	}

	topBorder := borderStyle.Render(cornerTL+strings.Repeat("─", dashesLen)) + devTag + borderStyle.Render(cornerTR)
	bottomBorder := borderStyle.Render(cornerBL + strings.Repeat("─", contentW) + cornerBR)
	sideBar := borderStyle.Render("│")

	var output []string
	output = append(output, theme.PadToWidth(topBorder, width))
	for _, l := range lines {
		row := sideBar + theme.PadToWidth(l, contentW) + sideBar
		output = append(output, theme.PadToWidth(row, width))
	}
	output = append(output, theme.PadToWidth(bottomBorder, width))

	return strings.Join(output, "\n")
}

func RenderPlayerBar(state *backend.PlaybackState, focused bool, width int) string {
	return RenderPlayer(state, focused, width)
}

func RenderMiniSlider(value, width int) string {
	if width <= 0 {
		return ""
	}
	ratio := float64(value) / 100.0
	if ratio < 0 {
		ratio = 0
	} else if ratio > 1 {
		ratio = 1
	}
	return renderProgressBar(ratio, width)
}

func renderProgressBar(ratio float64, width int) string {
	if width <= 0 {
		return ""
	}
	filledChars := int(ratio * float64(width))
	if filledChars > width {
		filledChars = width
	}
	emptyChars := width - filledChars

	filled := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render(strings.Repeat("━", filledChars))
	thumb := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render("●")

	if emptyChars > 0 {
		empty := theme.StyleFaint.Render(strings.Repeat("─", emptyChars-1))
		return filled + thumb + empty
	}
	return filled
}

func alignRow(left, center, right string, totalW int) string {
	leftW := ansi.StringWidth(left)
	centerW := ansi.StringWidth(center)
	rightW := ansi.StringWidth(right)

	centerPos := (totalW - centerW) / 2
	if centerPos < leftW+1 {
		centerPos = leftW + 1
	}

	leftPad := centerPos - leftW
	if leftPad < 1 {
		leftPad = 1
	}

	rightPad := totalW - (centerPos + centerW + rightW)
	if rightPad < 1 {
		rightPad = 1
	}

	mid := left + theme.BgPad(leftPad) + center + theme.BgPad(rightPad) + right
	return theme.PadToWidth(mid, totalW)
}
