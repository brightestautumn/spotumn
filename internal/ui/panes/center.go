// Center content pane - renders tracks, albums, artists, search results, queue, and lyrics views.
package panes

import (
	"fmt"
	"strings"

	"spotumn/internal/backend"
	"spotumn/internal/media/lyrics"
	"spotumn/internal/ui/state"
	"spotumn/internal/ui/theme"

	"github.com/charmbracelet/x/ansi"
)

// render center pane composing the search bar, tab selector, and active content table
func RenderCenterLines(
	currentTab state.CenterTab,
	tracks []backend.Track,
	albums []backend.Playlist,
	artists []backend.Playlist,
	history []backend.Track,
	playlistName string,
	currentPlayingTrackURI string,
	lyricsLines []lyrics.Line,
	lyricsCursor int,
	progressMs int,
	searchQuery string,
	searchFocused bool,
	selectedIndex int,
	focused bool,
	width, height int,
	containerURI ...string,
) []string {
	if width < 20 {
		width = 20
	}
	if height < 6 {
		height = 6
	}

	var lines []string

	searchBarLines := renderProminentSearchBar(searchQuery, searchFocused, width)
	lines = append(lines, searchBarLines...)

	tabBar := renderTabBar(currentTab, width)
	lines = append(lines, tabBar)
	lines = append(lines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", width)), width))

	bodyH := height - len(lines)
	if bodyH < 1 {
		bodyH = 1
	}

	var bodyLines []string
	switch currentTab {
	case state.TabTracks:
		var curURI string
		if len(containerURI) > 0 {
			curURI = containerURI[0]
		}
		bodyLines = renderTracks(tracks, albums, artists, playlistName, currentPlayingTrackURI, selectedIndex, focused && !searchFocused, width, bodyH, curURI)
	case state.TabHistory:
		bodyLines = renderHistory(history, currentPlayingTrackURI, selectedIndex, focused && !searchFocused, width, bodyH)
	case state.TabLyrics:
		bodyLines = renderLyrics(lyricsLines, lyricsCursor, progressMs, focused && !searchFocused, width, bodyH)
	}

	lines = append(lines, bodyLines...)

	for len(lines) < height {
		lines = append(lines, theme.PadToWidth("", width))
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	return lines
}

func RenderCenter(
	currentTab state.CenterTab,
	tracks []backend.Track,
	albums []backend.Playlist,
	artists []backend.Playlist,
	history []backend.Track,
	playlistName string,
	currentPlayingTrackURI string,
	lyricsLines []lyrics.Line,
	lyricsCursor int,
	progressMs int,
	searchQuery string,
	searchFocused bool,
	selectedIndex int,
	focused bool,
	width, height int,
	containerURI ...string,
) string {
	contentW := width - 2
	contentH := height - 2
	if contentW < 10 {
		contentW = 10
	}
	if contentH < 3 {
		contentH = 3
	}

	var curURI string
	if len(containerURI) > 0 {
		curURI = containerURI[0]
	}

	lines := RenderCenterLines(
		currentTab,
		tracks,
		albums,
		artists,
		history,
		playlistName,
		currentPlayingTrackURI,
		lyricsLines,
		lyricsCursor,
		progressMs,
		searchQuery,
		searchFocused,
		selectedIndex,
		focused,
		contentW,
		contentH,
		curURI,
	)

	boxStyle := theme.PanelBox(focused, width, height)
	return boxStyle.Render(strings.Join(lines, "\n"))
}

func RenderTabBar(currentTab state.CenterTab, width int) string {
	tabs := []struct {
		tab   state.CenterTab
		label string
	}{
		{state.TabTracks, "1 Tracks"},
		{state.TabLyrics, "2 Lyrics"},
		{state.TabHistory, "3 History"},
	}

	var sb strings.Builder
	sb.WriteString(theme.BgPad(1))
	for _, t := range tabs {
		if t.tab == currentTab {
			style := theme.StylePrimary
			sb.WriteString(style.Render("[ " + t.label + " ]"))
		} else {
			style := theme.StyleFaint
			sb.WriteString(style.Render("  " + t.label + "  "))
		}
		sb.WriteString(theme.BgPad(2))
	}

	return theme.PadToWidth(sb.String(), width)
}

func renderTabBar(currentTab state.CenterTab, width int) string {
	return RenderTabBar(currentTab, width)
}

func RenderProminentSearchBar(query string, focused bool, width int) []string {
	prompt := "  Search: "
	displayQuery := query
	if focused {
		displayQuery += "█"
	} else if displayQuery == "" {
		displayQuery = "Press [/] to search Spotify..."
	}

	barW := width - 4
	if barW < 14 {
		barW = 14
	}

	maxQueryW := barW - 2 - ansi.StringWidth(prompt)
	if maxQueryW > 4 {
		displayQuery = theme.TruncateString(displayQuery, maxQueryW)
	}

	var promptStyled, textStyled string
	if focused {
		promptStyled = theme.StylePrimary.Render(prompt)
		textStyled = theme.StyleNormal.Render(displayQuery)
	} else {
		promptStyled = theme.StyleFaint.Render(prompt)
		textStyled = theme.StyleFaint.Render(displayQuery)
	}

	innerContent := theme.PadToWidth(promptStyled+textStyled, barW-2)
	barStyle := theme.PanelBox(focused, barW, 0)

	var res []string
	for _, l := range strings.Split(barStyle.Render(innerContent), "\n") {
		res = append(res, theme.PadToWidth(theme.BgPad(2)+l, width))
	}
	if len(res) > 3 {
		res = res[:3]
	}
	return res
}

func renderProminentSearchBar(query string, focused bool, width int) []string {
	return RenderProminentSearchBar(query, focused, width)
}

func RenderTrackRow(idx int, t backend.Track, isSelected bool, isPlaying bool, focused bool, titleW, artistW int, width int) string {
	numCol := fmt.Sprintf("%2d ", idx+1)
	if isPlaying {
		numCol = " ▶ "
	}

	titleCol := theme.PadPlain(t.Name, titleW)
	artistCol := theme.PadPlain(t.Artist, artistW)
	durCol := theme.PadPlain(theme.FormatDuration(t.DurationMs), 6)
	rawRow := numCol + titleCol + artistCol + durCol

	if isSelected && focused {
		return theme.RenderPaddedLine(rawRow, theme.StyleActiveFocusedBlock, width)
	} else if isSelected {
		return theme.RenderPaddedLine(rawRow, theme.StyleActiveUnfocusedBlock, width)
	}

	var numStyled, titleStyled string
	if isPlaying {
		numStyled = theme.StyleTertiary.Render(numCol)
		titleStyled = theme.StyleTertiary.Render(titleCol)
	} else {
		numStyled = theme.StyleFaint.Render(numCol)
		titleStyled = theme.StyleBold.Render(titleCol)
	}
	artistStyled := theme.StyleSecondary.Render(artistCol)
	durStyled := theme.StyleFaint.Render(durCol)

	rowContent := numStyled + titleStyled + artistStyled + durStyled
	return theme.PadToWidth(rowContent, width)
}

func renderTrackRow(idx int, t backend.Track, isSelected bool, isPlaying bool, focused bool, titleW, artistW int, width int) string {
	return RenderTrackRow(idx, t, isSelected, isPlaying, focused, titleW, artistW, width)
}

func RenderAlbumRow(idx int, a backend.Playlist, isSelected bool, focused bool, titleW, descW, countW int, width int) string {
	numCol := theme.PadPlain(fmt.Sprintf("%2d", idx+1), 4)
	iconCol := " 💿 "
	titleCol := theme.PadPlain(a.Name, titleW)
	descCol := theme.PadPlain(a.OwnerID, descW)

	countStr := fmt.Sprintf("%d tracks", a.TrackCount)
	if a.TrackCount == 1 {
		countStr = "1 track"
	} else if a.TrackCount == 0 {
		countStr = ""
	}
	countCol := theme.PadPlain(countStr, countW)

	rawRow := numCol + iconCol + titleCol + descCol + countCol

	if isSelected && focused {
		return theme.RenderPaddedLine(rawRow, theme.StyleActiveFocusedBlock, width)
	} else if isSelected {
		return theme.RenderPaddedLine(rawRow, theme.StyleActiveUnfocusedBlock, width)
	}

	numStyled := theme.StyleFaint.Render(numCol)
	iconStyled := theme.StyleSecondary.Render(iconCol)
	titleStyled := theme.StyleBold.Render(titleCol)
	descStyled := theme.StyleHover.Render(descCol)
	countStyled := theme.StyleFaint.Render(countCol)

	rowContent := numStyled + iconStyled + titleStyled + descStyled + countStyled
	return theme.PadToWidth(rowContent, width)
}

func renderAlbumRow(idx int, a backend.Playlist, isSelected bool, focused bool, titleW, descW, countW int, width int) string {
	return RenderAlbumRow(idx, a, isSelected, focused, titleW, descW, countW, width)
}

func FormatTotalDuration(totalMs int, trackCount int) string {
	if trackCount == 0 {
		return ""
	}
	countLabel := fmt.Sprintf("%d tracks", trackCount)
	if trackCount == 1 {
		countLabel = "1 track"
	}
	if totalMs <= 0 {
		return countLabel
	}

	totalSeconds := totalMs / 1000
	hours := totalSeconds / 3600
	mins := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60

	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("%s • %d hr %d min", countLabel, hours, mins)
		}
		return fmt.Sprintf("%s • %d hr", countLabel, hours)
	}
	if mins > 0 {
		return fmt.Sprintf("%s • %d min %d sec", countLabel, mins, secs)
	}
	return fmt.Sprintf("%s • %d sec", countLabel, secs)
}

func renderPlaylistHeader(title string, totalMs int, trackCount int, width int) string {
	infoStr := FormatTotalDuration(totalMs, trackCount)
	infoW := ansi.StringWidth(infoStr)

	leftMargin := "  ♫ "
	rightMargin := "  "

	availForTitle := width - infoW - ansi.StringWidth(leftMargin) - ansi.StringWidth(rightMargin) - 2
	if availForTitle < 10 {
		availForTitle = 10
	}

	titleTrunc := theme.TruncateString(title, availForTitle)
	leftStyled := theme.StylePrimary.Render(leftMargin + titleTrunc)
	rightStyled := theme.StyleFaint.Render(infoStr + rightMargin)

	usedW := ansi.StringWidth(leftMargin+titleTrunc) + infoW + ansi.StringWidth(rightMargin)
	gap := width - usedW
	if gap < 0 {
		gap = 0
	}

	return leftStyled + theme.BgPad(gap) + rightStyled
}

func RenderArtistRow(idx int, a backend.Playlist, isSelected bool, focused bool, nameW, typeW, popW int, width int) string {
	numCol := theme.PadPlain(fmt.Sprintf("%2d", idx+1), 4)
	iconCol := " 󰠃 "
	nameCol := theme.PadPlain(a.Name, nameW)
	typeCol := theme.PadPlain(a.OwnerID, typeW)

	popStr := fmt.Sprintf("%d%% pop", a.TrackCount)
	if a.TrackCount <= 0 {
		popStr = ""
	}
	popCol := theme.PadPlain(popStr, popW)

	rawRow := numCol + iconCol + nameCol + typeCol + popCol

	if isSelected && focused {
		return theme.RenderPaddedLine(rawRow, theme.StyleActiveFocusedBlock, width)
	} else if isSelected {
		return theme.RenderPaddedLine(rawRow, theme.StyleActiveUnfocusedBlock, width)
	}

	numStyled := theme.StyleFaint.Render(numCol)
	iconStyled := theme.StyleHover.Render(iconCol)
	nameStyled := theme.StyleBold.Render(nameCol)
	typeStyled := theme.StyleSecondary.Render(typeCol)
	popStyled := theme.StyleFaint.Render(popCol)

	rowContent := numStyled + iconStyled + nameStyled + typeStyled + popStyled
	return theme.PadToWidth(rowContent, width)
}

func renderArtistRow(idx int, a backend.Playlist, isSelected bool, focused bool, nameW, typeW, popW int, width int) string {
	return RenderArtistRow(idx, a, isSelected, focused, nameW, typeW, popW, width)
}

func RenderTracks(
	tracks []backend.Track,
	albums []backend.Playlist,
	artists []backend.Playlist,
	playlistName string,
	currentPlayingURI string,
	selectedIndex int,
	focused bool,
	width, height int,
	containerURI ...string,
) []string {
	var curContainerURI string
	if len(containerURI) > 0 {
		curContainerURI = containerURI[0]
	}

	if len(tracks) == 0 && len(albums) == 0 && len(artists) == 0 {
		var lines []string
		title := "Tracks"
		if playlistName != "" {
			title = playlistName
		}
		titleHeader := theme.StylePrimary.Render("  ♫ " + title)
		lines = append(lines, theme.PadToWidth(titleHeader, width))
		emptyMsg := theme.StyleFaint.Render("No tracks, albums, or artists found. Select a playlist or press [/] to search.")
		lines = append(lines, theme.PadToWidth(theme.BgPad(2)+emptyMsg, width))
		return lines
	}

	availRows := height
	if availRows < 1 {
		availRows = 1
	}

	totalEstimatedLines := 1
	if len(albums) > 0 || len(artists) > 0 {
		if len(tracks) > 0 {
			totalEstimatedLines += 4 + len(tracks)
		}
		if len(albums) > 0 {
			totalEstimatedLines += 4 + len(albums)
		}
		if len(artists) > 0 {
			totalEstimatedLines += 4 + len(artists)
		}
	} else {
		totalEstimatedLines += 2 + len(tracks)
	}

	needsScroll := totalEstimatedLines > availRows
	tableW := width
	if needsScroll {
		tableW = width - 2
	}
	if tableW < 14 {
		tableW = 14
	}

	numW := 4
	durW := 6
	remW := tableW - numW - durW - 2
	if remW < 12 {
		remW = 12
	}
	titleW := remW * 55 / 100
	artistW := remW - titleW

	var allVisualLines []string
	selectedLineIdx := 0

	totalMs := 0
	for _, t := range tracks {
		totalMs += t.DurationMs
	}

	title := "Tracks"
	if playlistName != "" {
		title = playlistName
	}
	headerLine := renderPlaylistHeader(title, totalMs, len(tracks), tableW)
	allVisualLines = append(allVisualLines, headerLine)

	if len(albums) > 0 || len(artists) > 0 {

		if len(tracks) > 0 {
			allVisualLines = append(allVisualLines, theme.PadToWidth("", tableW))
			subHeader := "  ♪ Top Tracks"
			isAlbumView := strings.HasPrefix(curContainerURI, "spotify:album:") ||
				(len(tracks) > 0 && tracks[0].Artist != "" && !strings.EqualFold(strings.TrimSpace(playlistName), strings.TrimSpace(tracks[0].Artist)) && !strings.HasPrefix(curContainerURI, "spotify:artist:"))
			if strings.HasPrefix(playlistName, "Search:") {
				subHeader = "  ♪ Songs"
			} else if isAlbumView {
				subHeader = "  ♪ Tracks"
			}
			allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleSecondary.Render(subHeader), tableW))
			headerNum := theme.PadPlain(" #", numW)
			headerTitle := theme.PadPlain("Title", titleW)
			headerArtist := theme.PadPlain("Artist", artistW)
			headerDuration := theme.PadPlain("Time", durW)
			tableHeader := theme.StyleFaint.Render(headerNum + headerTitle + headerArtist + headerDuration)
			allVisualLines = append(allVisualLines, theme.PadToWidth(tableHeader, tableW))
			allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", tableW)), tableW))

			for idx, t := range tracks {
				if idx == selectedIndex {
					selectedLineIdx = len(allVisualLines)
				}
				isSelected := idx == selectedIndex
				isPlaying := t.URI == currentPlayingURI && currentPlayingURI != ""
				lineContent := renderTrackRow(idx, t, isSelected, isPlaying, focused, titleW, artistW, tableW)
				allVisualLines = append(allVisualLines, lineContent)
			}
		}

		if len(albums) > 0 {
			numW := 4
			iconW := 4
			countW := 12
			remAlbW := tableW - numW - iconW - countW - 2
			if remAlbW < 12 {
				remAlbW = 12
			}
			albTitleW := remAlbW * 55 / 100
			albDescW := remAlbW - albTitleW

			isAlbumView := strings.HasPrefix(curContainerURI, "spotify:album:") ||
				(len(tracks) > 0 && tracks[0].Artist != "" && !strings.EqualFold(strings.TrimSpace(playlistName), strings.TrimSpace(tracks[0].Artist)) && !strings.HasPrefix(curContainerURI, "spotify:artist:"))

			allVisualLines = append(allVisualLines, theme.PadToWidth("", tableW))
			albSubHeader := "  💿 Albums & Discography"
			if strings.HasPrefix(playlistName, "Search:") {
				albSubHeader = "  💿 Albums"
			} else if isAlbumView {
				artistName := ""
				if len(tracks) > 0 && tracks[0].Artist != "" {
					artistName = tracks[0].Artist
				}
				if artistName != "" {
					albSubHeader = fmt.Sprintf("  💿 More from %s", artistName)
				} else {
					albSubHeader = "  💿 More from this Artist"
				}
			}
			allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleSecondary.Render(albSubHeader), tableW))
			albHeader := theme.StyleFaint.Render(theme.PadPlain(" #", numW) + "    " + theme.PadPlain("Album", albTitleW) + theme.PadPlain("Type • Year", albDescW) + theme.PadPlain("Tracks", countW))
			allVisualLines = append(allVisualLines, theme.PadToWidth(albHeader, tableW))
			allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", tableW)), tableW))

			for aIdx, a := range albums {
				itemIdx := len(tracks) + aIdx
				if itemIdx == selectedIndex {
					selectedLineIdx = len(allVisualLines)
				}
				isSelected := itemIdx == selectedIndex
				lineContent := renderAlbumRow(aIdx, a, isSelected, focused, albTitleW, albDescW, countW, tableW)
				allVisualLines = append(allVisualLines, lineContent)
			}
		}

		if len(artists) > 0 {
			numW := 4
			iconW := 4
			popW := 12
			remArtW := tableW - numW - iconW - popW - 2
			if remArtW < 12 {
				remArtW = 12
			}
			artNameW := remArtW * 60 / 100
			artTypeW := remArtW - artNameW

			allVisualLines = append(allVisualLines, theme.PadToWidth("", tableW))
			allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleHover.Render("  󰠃 Artists"), tableW))
			artHeader := theme.StyleFaint.Render(theme.PadPlain(" #", numW) + "    " + theme.PadPlain("Artist", artNameW) + theme.PadPlain("Type", artTypeW) + theme.PadPlain("Popularity", popW))
			allVisualLines = append(allVisualLines, theme.PadToWidth(artHeader, tableW))
			allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", tableW)), tableW))

			for artIdx, art := range artists {
				itemIdx := len(tracks) + len(albums) + artIdx
				if itemIdx == selectedIndex {
					selectedLineIdx = len(allVisualLines)
				}
				isSelected := itemIdx == selectedIndex
				lineContent := renderArtistRow(artIdx, art, isSelected, focused, artNameW, artTypeW, popW, tableW)
				allVisualLines = append(allVisualLines, lineContent)
			}
		}
	} else {

		headerNum := theme.PadPlain(" #", numW)
		headerTitle := theme.PadPlain("Title", titleW)
		headerArtist := theme.PadPlain("Artist", artistW)
		headerDuration := theme.PadPlain("Time", durW)
		tableHeader := theme.StyleSecondary.Render(headerNum + headerTitle + headerArtist + headerDuration)
		allVisualLines = append(allVisualLines, theme.PadToWidth(tableHeader, tableW))
		allVisualLines = append(allVisualLines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", tableW)), tableW))

		for idx, t := range tracks {
			if idx == selectedIndex {
				selectedLineIdx = len(allVisualLines)
			}
			isSelected := idx == selectedIndex
			isPlaying := t.URI == currentPlayingURI && currentPlayingURI != ""
			lineContent := renderTrackRow(idx, t, isSelected, isPlaying, focused, titleW, artistW, tableW)
			allVisualLines = append(allVisualLines, lineContent)
		}
	}

	scrollOffset := 0
	if selectedLineIdx >= availRows {
		scrollOffset = selectedLineIdx - availRows + 1
	}

	totalLines := len(allVisualLines)
	thumbH := 1
	thumbStart := 0
	if totalLines > availRows && availRows > 0 {
		thumbH = (availRows * availRows) / totalLines
		if thumbH < 1 {
			thumbH = 1
		}
		maxScroll := totalLines - availRows
		if maxScroll > 0 {
			thumbStart = (scrollOffset * (availRows - thumbH)) / maxScroll
		}
		if thumbStart+thumbH > availRows {
			thumbStart = availRows - thumbH
		}
		if thumbStart < 0 {
			thumbStart = 0
		}
	}

	var lines []string
	for i := 0; i < availRows; i++ {
		lineIdx := scrollOffset + i
		var rawLine string
		if lineIdx < len(allVisualLines) {
			rawLine = allVisualLines[lineIdx]
		}
		if totalLines > availRows {
			var scrollIndicator string
			if i >= thumbStart && i < thumbStart+thumbH {
				scrollIndicator = theme.StylePrimary.Render("█")
			} else {
				scrollIndicator = theme.StyleFaint.Render("│")
			}
			trimmed := theme.PadToWidth(rawLine, tableW)
			lines = append(lines, trimmed+theme.BgPad(1)+scrollIndicator)
		} else {
			lines = append(lines, theme.PadToWidth(rawLine, width))
		}
	}

	return lines
}

func renderTracks(
	tracks []backend.Track,
	albums []backend.Playlist,
	artists []backend.Playlist,
	playlistName string,
	currentPlayingURI string,
	selectedIndex int,
	focused bool,
	width, height int,
	containerURI ...string,
) []string {
	return RenderTracks(tracks, albums, artists, playlistName, currentPlayingURI, selectedIndex, focused, width, height, containerURI...)
}

func renderHistory(
	history []backend.Track,
	currentPlayingURI string,
	selectedIndex int,
	focused bool,
	width, height int,
) []string {
	var lines []string

	titleHeader := theme.StylePrimary.Render("   Listening History (Recently Played)")
	lines = append(lines, theme.PadToWidth(titleHeader, width))

	if len(history) == 0 {
		emptyMsg := theme.StyleFaint.Render("No listening history available yet. Play some tracks on Spotify!")
		lines = append(lines, theme.PadToWidth("", width))
		lines = append(lines, theme.PadToWidth(theme.BgPad(2)+emptyMsg, width))
		return lines
	}

	availRows := height - len(lines) - 2
	if availRows < 1 {
		availRows = 1
	}

	needsScroll := len(history) > availRows
	tableW := width
	if needsScroll {
		tableW = width - 2
	}
	if tableW < 14 {
		tableW = 14
	}

	numW := 4
	durW := 6
	remW := tableW - numW - durW - 2
	if remW < 12 {
		remW = 12
	}
	titleW := remW * 55 / 100
	artistW := remW - titleW

	headerNum := theme.PadPlain(" #", numW)
	headerTitle := theme.PadPlain("Title", titleW)
	headerArtist := theme.PadPlain("Artist", artistW)
	headerDuration := theme.PadPlain("Time", durW)

	tableHeader := theme.StyleSecondary.Render(headerNum + headerTitle + headerArtist + headerDuration)
	if needsScroll {
		lines = append(lines, theme.PadToWidth(tableHeader, tableW)+theme.BgPad(1)+theme.StyleFaint.Render("│"))
		lines = append(lines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", tableW)), tableW)+theme.BgPad(1)+theme.StyleFaint.Render("│"))
	} else {
		lines = append(lines, theme.PadToWidth(tableHeader, width))
		lines = append(lines, theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", width)), width))
	}

	scrollOffset := 0
	if selectedIndex >= availRows {
		scrollOffset = selectedIndex - availRows + 1
	}

	totalItems := len(history)
	thumbH := 1
	thumbStart := 0
	if totalItems > availRows && availRows > 0 {
		thumbH = (availRows * availRows) / totalItems
		if thumbH < 1 {
			thumbH = 1
		}
		maxScroll := totalItems - availRows
		if maxScroll > 0 {
			thumbStart = (scrollOffset * (availRows - thumbH)) / maxScroll
		}
		if thumbStart+thumbH > availRows {
			thumbStart = availRows - thumbH
		}
		if thumbStart < 0 {
			thumbStart = 0
		}
	}

	for i := 0; i < availRows; i++ {
		idx := scrollOffset + i
		if idx >= len(history) {
			if needsScroll {
				lines = append(lines, theme.PadToWidth("", tableW)+theme.BgPad(1)+theme.StyleFaint.Render("│"))
			} else {
				lines = append(lines, theme.PadToWidth("", width))
			}
			continue
		}

		t := history[idx]
		isSelected := idx == selectedIndex
		isPlaying := t.URI == currentPlayingURI && currentPlayingURI != ""

		lineContent := renderTrackRow(idx, t, isSelected, isPlaying, focused, titleW, artistW, tableW)
		if needsScroll {
			var scrollIndicator string
			if i >= thumbStart && i < thumbStart+thumbH {
				scrollIndicator = theme.StylePrimary.Render("█")
			} else {
				scrollIndicator = theme.StyleFaint.Render("│")
			}
			lines = append(lines, lineContent+theme.BgPad(1)+scrollIndicator)
		} else {
			lines = append(lines, lineContent)
		}
	}

	return lines
}

func RenderLyrics(lines []lyrics.Line, cursorLine int, progressMs int, focused bool, width, height int) []string {
	var output []string

	if len(lines) == 0 {
		msg := theme.StyleFaint.Render("No lyrics found for this track")
		output = append(output, theme.PadToWidth("", width))
		output = append(output, theme.PadToWidth(theme.BgPad(2)+msg, width))
		return output
	}

	isSynced := false
	for _, l := range lines {
		if l.TimeMs > 0 {
			isSynced = true
			break
		}
	}

	if !isSynced {
		output = append(output, theme.PadToWidth(theme.BgPad(2)+theme.StyleFaint.Render("[Lyrics not synced]"), width))
		height--
		if height < 1 {
			height = 1
		}

		targetCenter := cursorLine
		if targetCenter < 0 {
			targetCenter = 0
		}
		if targetCenter >= len(lines) {
			targetCenter = len(lines) - 1
		}

		halfHeight := height / 2
		startIdx := targetCenter - halfHeight
		if startIdx < 0 {
			startIdx = 0
		}

		for i := 0; i < height; i++ {
			idx := startIdx + i
			if idx >= len(lines) {
				output = append(output, theme.PadToWidth("", width))
				continue
			}

			line := lines[idx]
			text := line.Text
			if text == "" {
				text = "♪"
			}

			maxLyricW := width - 6
			if maxLyricW < 4 {
				maxLyricW = 4
			}
			text = theme.TruncateString(text, maxLyricW)

			isCursor := idx == cursorLine
			var lineRendered string
			if isCursor {
				lineRendered = theme.RenderPaddedLine("  ❯  "+text, theme.StyleActiveFocusedBlock, width)
			} else {
				lineRendered = theme.PadToWidth(theme.BgPad(5)+theme.StyleFaint.Render(text), width)
			}
			output = append(output, lineRendered)
		}
		return output
	}

	activeIdx := lyrics.FindActiveIndex(lines, progressMs)
	if activeIdx < 0 {
		activeIdx = 0
	}

	targetCenter := activeIdx
	if cursorLine >= 0 && cursorLine < len(lines) {
		targetCenter = cursorLine
	}

	halfHeight := height / 2
	startIdx := targetCenter - halfHeight
	if startIdx < 0 {
		startIdx = 0
	}

	for i := 0; i < height; i++ {
		idx := startIdx + i
		if idx >= len(lines) {
			output = append(output, theme.PadToWidth("", width))
			continue
		}

		line := lines[idx]
		text := line.Text
		if text == "" {
			text = "♪"
		}

		maxLyricW := width - 6
		if maxLyricW < 4 {
			maxLyricW = 4
		}
		text = theme.TruncateString(text, maxLyricW)

		isSinging := idx == activeIdx
		isCursor := idx == cursorLine

		var lineRendered string
		if isSinging {
			lineRendered = theme.RenderPaddedLine("  ❯  "+text, theme.StyleActiveLyricsBlock, width)
		} else if isCursor && focused {
			lineRendered = theme.RenderPaddedLine("  ➜  "+text, theme.StyleActiveFocusedBlock, width)
		} else {
			lineRendered = theme.PadToWidth(theme.BgPad(5)+theme.StyleFaint.Render(text), width)
		}

		output = append(output, lineRendered)
	}

	return output
}

func renderLyrics(lines []lyrics.Line, cursorLine int, progressMs int, focused bool, width, height int) []string {
	return RenderLyrics(lines, cursorLine, progressMs, focused, width, height)
}

func RenderCenterPane(
	currentTab state.CenterTab,
	tracks []backend.Track,
	albums []backend.Playlist,
	artists []backend.Playlist,
	history []backend.Track,
	playlistName string,
	currentPlayingTrackURI string,
	lyricsLines []lyrics.Line,
	lyricsCursor int,
	progressMs int,
	searchQuery string,
	searchFocused bool,
	selectedIndex int,
	focused bool,
	width, height int,
	containerURI ...string,
) string {
	return RenderCenter(
		currentTab,
		tracks,
		albums,
		artists,
		history,
		playlistName,
		currentPlayingTrackURI,
		lyricsLines,
		lyricsCursor,
		progressMs,
		searchQuery,
		searchFocused,
		selectedIndex,
		focused,
		width, height,
		containerURI...,
	)
}
