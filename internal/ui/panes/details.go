// Right sidebar pane - renders album artwork, track information, and synchronized lyrics display.
package panes

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"spotumn/internal/backend"
	"spotumn/internal/ui/theme"
)

// render right sidebar with album art, track credits, and upcoming queue
func RenderRightLines(
	artANSI string,
	currentTrack *backend.Track,
	queue []backend.Track,
	queueIndex int,
	focused bool,
	width, height int,
) []string {
	if width < 16 {
		width = 16
	}
	if height < 6 {
		height = 6
	}

	var lines []string

	infoTitle := " Now Playing "
	if focused && queueIndex < 0 {
		infoTitle = " ● Now Playing "
	}
	lines = append(lines, theme.PadToWidth(theme.StylePrimary.Render(infoTitle), width))

	if artANSI != "" {
		for _, row := range strings.Split(artANSI, "\n") {
			row = strings.TrimRight(row, "\r")
			if row == "" {
				continue
			}
			if strings.Contains(row, "\x1b[") {
				if !strings.HasSuffix(row, "\x1b[0m") {
					row += "\x1b[0m"
				}
				w := ansi.StringWidth(row)
				if w > width {
					row = ansi.Truncate(row, width, "") + "\x1b[0m"
				}
			}
			lines = append(lines, theme.CenterLine(row, width))
		}
	}

	if currentTrack != nil && currentTrack.Name != "" {
		trackName := theme.TruncateString(currentTrack.Name, width-4)
		artistName := theme.TruncateString(currentTrack.Artist, width-4)
		albumName := theme.TruncateString(currentTrack.Album, width-4)

		lines = append(lines, theme.PadToWidth(theme.BgPad(2)+theme.StyleBold.Render(trackName), width))
		lines = append(lines, theme.PadToWidth(theme.BgPad(2)+theme.StyleSecondary.Render(artistName), width))

		if albumName != "" {
			lines = append(lines, theme.PadToWidth(theme.BgPad(2)+theme.StyleFaint.Render(albumName), width))
		}
	} else {
		lines = append(lines, theme.PadToWidth(theme.BgPad(2)+theme.StyleFaint.Render("No track playing"), width))
	}

	lines = append(lines, theme.PadToWidth("", width))
	prefix := "── UP NEXT "
	badge := theme.BgPad(1) + theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("q") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Queue") + theme.StyleFaint.Render(" ──")
	rawBadge := " ⌜q⌟ Queue ──"
	neededDashes := width - ansi.StringWidth(prefix) - ansi.StringWidth(rawBadge)
	var queueHeaderLine string
	if neededDashes > 0 {
		queueHeaderLine = theme.StyleFaint.Render(prefix+strings.Repeat("─", neededDashes)) + badge
	} else {
		queueHeaderLine = theme.StyleFaint.Render(theme.TruncateString(prefix, width))
	}
	lines = append(lines, theme.PadToWidth(queueHeaderLine, width))

	headerLinesCount := len(lines)
	availableQueueRows := height - headerLinesCount
	if availableQueueRows < 1 {
		availableQueueRows = 1
	}

	scrollOffset := 0
	if queueIndex >= availableQueueRows {
		scrollOffset = queueIndex - availableQueueRows + 1
	}

	for i := 0; i < availableQueueRows; i++ {
		qIdx := scrollOffset + i
		if qIdx >= len(queue) {
			lines = append(lines, theme.PadToWidth("", width))
			continue
		}

		item := queue[qIdx]
		isSelected := qIdx == queueIndex

		numPrefix := fmt.Sprintf("%d. ", qIdx+1)
		availTitleW := width - len(numPrefix) - 4
		if availTitleW < 6 {
			availTitleW = 6
		}

		display := numPrefix + item.Name
		if item.Artist != "" {
			display += " - " + item.Artist
		}
		truncDisplay := theme.TruncateString(display, width-4)

		rawDisplay := "  " + truncDisplay
		if isSelected {
			rawDisplay = "❯ " + truncDisplay
		}

		var lineContent string
		if isSelected && focused {
			lineContent = theme.RenderPaddedLine(rawDisplay, theme.StyleActiveFocusedBlock, width)
		} else if isSelected {
			lineContent = theme.RenderPaddedLine(rawDisplay, theme.StyleActiveUnfocusedBlock, width)
		} else {
			numStyled := theme.StyleFaint.Render("  " + numPrefix)
			remW := width - 4 - ansi.StringWidth(numPrefix)
			if remW < 4 {
				remW = 4
			}
			var contentStyled string
			if item.Artist != "" {
				namePart := theme.TruncateString(item.Name, remW*60/100)
				artistPart := theme.TruncateString(item.Artist, remW-ansi.StringWidth(namePart)-3)
				contentStyled = theme.StyleBold.Render(namePart) + theme.StyleFaint.Render(" - ") + theme.StyleSecondary.Render(artistPart)
			} else {
				contentStyled = theme.StyleBold.Render(theme.TruncateString(item.Name, remW))
			}
			rowText := numStyled + contentStyled
			remPad := width - ansi.StringWidth(rowText)
			if remPad > 0 {
				rowText += theme.BgPad(remPad)
			}
			lineContent = rowText
		}

		lines = append(lines, lineContent)
	}

	for len(lines) < height {
		lines = append(lines, theme.PadToWidth("", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return lines
}

func RenderMergedRight(
	artANSI string,
	currentTrack *backend.Track,
	queue []backend.Track,
	queueIndex int,
	focused bool,
	width, height int,
) string {
	contentW := width - 2
	if contentW < 16 {
		contentW = 16
	}
	contentH := height - 2
	if contentH < 6 {
		contentH = 6
	}

	lines := RenderRightLines(artANSI, currentTrack, queue, queueIndex, focused, contentW, contentH)
	boxStyle := theme.PanelBox(focused, width, height)
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func RenderRightPane(
	artANSI string,
	currentTrack *backend.Track,
	queue []backend.Track,
	queueIndex int,
	focused bool,
	width, height int,
) string {
	return RenderMergedRight(artANSI, currentTrack, queue, queueIndex, focused, width, height)
}
