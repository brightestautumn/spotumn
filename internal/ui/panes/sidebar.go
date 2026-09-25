// Left navigation sidebar - displays user profile, library categories, playlists, and pinned items.
package panes

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"spotumn/internal/backend"
	"spotumn/internal/ui/state"
	"spotumn/internal/ui/theme"
)

// render left navigation pane with category headers, pinned items, and active scroll offset
func RenderNavLines(playlists []backend.Playlist, pinnedURIs map[string]bool, filter state.PlaylistFilter, selectedIndex int, focused bool, width, height int) []string {
	if width < 10 {
		width = 10
	}
	if height < 3 {
		height = 3
	}

	categoryTitle := "Playlists"
	if filter == state.FilterAlbums {
		categoryTitle = "Albums"
	} else if filter == state.FilterArtists {
		categoryTitle = "Artists"
	}

	filterName := filter.String()
	prefixDot := ""
	if focused {
		prefixDot = "● "
	}
	titleText := fmt.Sprintf(" %s%s [%s] ", prefixDot, categoryTitle, filterName)
	title := theme.StylePrimary.Render(titleText)
	hints := theme.StyleFaint.Render(" [f] filter  [*] pin ")

	lines := []string{
		alignTwoItems(title, hints, width),
		theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", width)), width),
	}

	availableRows := height - len(lines)
	if availableRows < 1 {
		availableRows = 1
	}

	scrollOffset := 0
	if selectedIndex >= availableRows {
		scrollOffset = selectedIndex - availableRows + 1
	}

	for i := 0; i < availableRows; i++ {
		idx := scrollOffset + i
		if idx >= len(playlists) {
			lines = append(lines, theme.PadToWidth("", width))
			continue
		}

		pl := playlists[idx]
		isSelected := idx == selectedIndex

		prefix := "  "
		if isSelected {
			prefix = "❯ "
		}

		starPrefix := ""
		if pinnedURIs != nil && pinnedURIs[pl.URI] {
			starPrefix = "★ "
		}

		countStr := ""
		displayName := pl.Name
		numPrefix := ""
		if filter == state.FilterAlbums {
			numPrefix = fmt.Sprintf("%d. ", idx+1)
			if pl.OwnerID != "" && width >= 34 {
				displayName = fmt.Sprintf("%s ─ %s", pl.Name, pl.OwnerID)
			}
			countStr = fmt.Sprintf(" (%d)", pl.TrackCount)
		} else if filter == state.FilterArtists {
			numPrefix = fmt.Sprintf("%d. ", idx+1)
			countStr = ""
		} else if pl.TrackCount > 0 {
			countStr = fmt.Sprintf(" (%d)", pl.TrackCount)
		}

		availNameW := width - ansi.StringWidth(prefix) - ansi.StringWidth(starPrefix) - ansi.StringWidth(numPrefix) - ansi.StringWidth(countStr) - 1
		if availNameW < 4 {
			availNameW = 4
		}

		truncName := theme.TruncateString(displayName, availNameW)
		plainLine := prefix + starPrefix + numPrefix + truncName + countStr

		var lineContent string
		if isSelected && focused {
			lineContent = theme.RenderPaddedLine(plainLine, theme.StyleActiveFocusedBlock, width)
		} else if isSelected {
			lineContent = theme.RenderPaddedLine(plainLine, theme.StyleActiveUnfocusedBlock, width)
		} else if starPrefix != "" {
			starStyled := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Gold).Background(theme.CurrentTheme.Surface).Bold(true).Render(prefix + starPrefix)
			numStyled := theme.StyleFaint.Render(numPrefix)
			nameStyled := theme.StyleNormal.Render(truncName)
			cntStyled := theme.StyleFaint.Render(countStr)
			lineContent = theme.PadToWidth(starStyled+numStyled+nameStyled+cntStyled, width)
		} else {
			numStyled := theme.StyleFaint.Render(numPrefix)
			nameStyled := theme.StyleNormal.Render(truncName)
			cntStyled := theme.StyleFaint.Render(countStr)
			lineContent = theme.PadToWidth(prefix+numStyled+nameStyled+cntStyled, width)
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

func RenderNav(playlists []backend.Playlist, pinnedURIs map[string]bool, filter state.PlaylistFilter, selectedIndex int, focused bool, width, height int) string {
	contentW := width - 2
	if contentW < 10 {
		contentW = 10
	}
	contentH := height - 2
	if contentH < 3 {
		contentH = 3
	}

	lines := RenderNavLines(playlists, pinnedURIs, filter, selectedIndex, focused, contentW, contentH)
	boxStyle := theme.PanelBox(focused, width, height)
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func alignTwoItems(left, right string, totalW int) string {
	lW := ansi.StringWidth(left)
	rW := ansi.StringWidth(right)
	gap := totalW - (lW + rW)
	if gap < 1 {
		gap = 1
	}
	return theme.PadToWidth(left+theme.BgPad(gap)+right, totalW)
}
