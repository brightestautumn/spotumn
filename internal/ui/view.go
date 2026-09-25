// Layout compositor - splits window into panes, calculates responsive geometry, and handles modal overlays.
package ui

import (
	"fmt"
	"strings"

	"spotumn/internal/backend"
	"spotumn/internal/media/lyrics"
	"spotumn/internal/ui/theme"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// calculate responsive layout geometry and render active panes or modal overlays
func RenderFullUI(p ViewParams) string {
	if p.Width < 48 || p.Height < 16 {
		msg := "Terminal window too small for spotumn. Please enlarge window."
		return lipgloss.NewStyle().
			Width(p.Width).
			Height(p.Height).
			Background(theme.CurrentTheme.Surface).
			Foreground(theme.CurrentTheme.OnSurface).
			Align(lipgloss.Center, lipgloss.Center).
			Render(msg)
	}

	if p.ShowSettings {
		modal := RenderSettingsModal(p.SettingsState, p.Width, p.Height)
		return CenterOverlay(modal, p.Width, p.Height)
	}

	if p.ShowHelp {
		return renderHelpOverlay(p)
	}

	if p.ShowDevices {
		return renderDevicesOverlay(p)
	}

	if p.ZenMode {
		return renderZenMode(p)
	}

	innerW := p.Width - 2
	innerH := p.Height - 2
	playerH := 4
	bodyH := innerH - playerH
	if bodyH < 4 {
		bodyH = 4
	}

	curURI := ""
	curProgress := 0
	var curTrack *backend.Track
	if p.Playback != nil {
		curProgress = p.Playback.ProgressMs
		curTrack = p.Playback.CurrentTrack
		if p.Playback.CurrentTrack != nil {
			curURI = p.Playback.CurrentTrack.URI
		}
	}

	showNav := p.ShowLeftSidebar
	showRight := p.ShowRightSidebar
	if p.AutoShrinkSidebars {
		if innerW < 125 {
			showRight = false
		}
		if innerW < 90 {
			showNav = false
		}
	}

	var navW, rightW int
	if showNav && showRight {
		navW = innerW * 28 / 100
		if navW < 30 {
			navW = 30
		}
		if navW > 45 {
			navW = 45
		}
		rightW = innerW * 34 / 100
		if rightW < 34 {
			rightW = 34
		}
		if rightW > 50 {
			rightW = 50
		}
	} else if showNav {
		navW = innerW * 30 / 100
		if navW < 30 {
			navW = 30
		}
		if navW > 45 {
			navW = 45
		}
	} else if showRight {
		rightW = innerW * 36 / 100
		if rightW < 34 {
			rightW = 34
		}
	}

	centerW := innerW - navW - rightW
	if centerW < 20 {
		centerW = 20
	}

	centerView := RenderCenter(
		p.CurrentTab,
		p.PlaylistTracks,
		p.ArtistAlbums,
		p.SearchArtists,
		p.History,
		p.PlaylistName,
		curURI,
		p.LyricsLines,
		p.LyricsCursor,
		curProgress,
		p.SearchQuery,
		p.SearchFocused,
		p.CenterIndex,
		p.Focused == PaneCenter,
		centerW,
		bodyH,
		p.CurrentPlURI,
	)

	var views []string
	var viewWidths []int
	if navW > 0 {
		views = append(views, RenderNav(p.Playlists, p.PinnedURIs, p.PlaylistFilter, p.NavIndex, p.Focused == PaneNav, navW, bodyH))
		viewWidths = append(viewWidths, navW)
	}
	views = append(views, centerView)
	viewWidths = append(viewWidths, centerW)
	if rightW > 0 {
		views = append(views, RenderMergedRight(p.ArtANSI, curTrack, p.Queue, p.QueueIndex, p.Focused == PaneRight, rightW, bodyH))
		viewWidths = append(viewWidths, rightW)
	}
	bodyContent := joinHorizontalThemed(views, viewWidths, bodyH)

	playerView := RenderPlayer(p.Playback, p.Focused == PanePlayer, innerW)
	innerCombined := bodyContent + "\n" + playerView

	userName := p.Username

	greenDot := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Success).Background(theme.CurrentTheme.Surface).Render("● ")
	titleTag := theme.StyleFaint.Render("─ ") + greenDot + theme.StylePrimary.Render("spotumn ")
	var userTag string
	if userName != "" {
		personIcon := theme.StyleSecondary.Render(" ")
		userTag = BgPad(1) + personIcon + theme.StyleSecondary.Render(userName) + theme.StyleFaint.Render(" ─")
	}

	cornerTL := theme.StyleFaint.Render("╭")
	cornerTR := theme.StyleFaint.Render("╮")
	sideBar := theme.StyleFaint.Render("│")

	focusTag := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("[/]") + theme.StylePrimary.Render("⌟") + BgPad(1) + theme.StyleSecondary.Render("Focus")
	rawFocusTag := "⌜[/]⌟ Focus"

	dashesNeeded := innerW - ansi.StringWidth(titleTag) - ansi.StringWidth(userTag) - ansi.StringWidth(rawFocusTag) - 2
	var topBorder string
	if dashesNeeded >= 2 {
		leftD := dashesNeeded / 2
		rightD := dashesNeeded - leftD
		topBorder = cornerTL + titleTag + theme.StyleFaint.Render(strings.Repeat("─", leftD)) + BgPad(1) + focusTag + BgPad(1) + theme.StyleFaint.Render(strings.Repeat("─", rightD)) + userTag + cornerTR
	} else {
		midDashes := innerW - ansi.StringWidth(titleTag) - ansi.StringWidth(userTag)
		if midDashes < 1 {
			midDashes = 1
		}
		topBorder = cornerTL + titleTag + theme.StyleFaint.Render(strings.Repeat("─", midDashes)) + userTag + cornerTR
	}

	var items []keybindItem
	if innerW >= 110 {
		items = []keybindItem{
			{"Space", "Play"},
			{"n", "Next"},
			{"p", "Prev"},
			{"←/→", "Seek"},
			{"-/=", "Vol"},
			{"a", "Artist"},
			{"s", "Shuffle"},
			{"r", "Repeat"},
			{"`", "Settings"},
			{"?", "Help"},
		}
	} else if innerW >= 75 {
		items = []keybindItem{
			{"Space", "Play"},
			{"n/p", "Next/Prev"},
			{"←/→", "Seek"},
			{"-/=", "Vol"},
			{"a", "Artist"},
			{"`", "Settings"},
			{"?", "Help"},
		}
	} else {
		items = []keybindItem{
			{"Space", "Play"},
			{"n/p", "Next/Prev"},
			{"`", "Settings"},
			{"?", "Help"},
		}
	}

	bottomBorder := renderKeybindBar(items, innerW)

	var finalLines []string
	finalLines = append(finalLines, PadToWidth(topBorder, p.Width))

	rawInnerLines := strings.Split(innerCombined, "\n")
	for i := 0; i < innerH; i++ {
		row := ""
		if i < len(rawInnerLines) {
			row = rawInnerLines[i]
		}
		paddedRow := PadToWidth(row, innerW)
		borderedRow := sideBar + paddedRow + sideBar
		finalLines = append(finalLines, PadToWidth(borderedRow, p.Width))
	}

	finalLines = append(finalLines, PadToWidth(bottomBorder, p.Width))

	for len(finalLines) < p.Height {
		finalLines = append(finalLines, PadToWidth("", p.Width))
	}
	if len(finalLines) > p.Height {
		finalLines = finalLines[:p.Height]
	}

	return strings.Join(finalLines, "\n")
}

type keybindItem struct {
	key   string
	label string
}

func renderKeybindBar(items []keybindItem, innerW int) string {
	cornerBL := theme.StyleFaint.Render("╰")
	cornerBR := theme.StyleFaint.Render("╯")
	if innerW < 10 {
		return cornerBL + theme.StyleFaint.Render(strings.Repeat("─", innerW)) + cornerBR
	}

	sep := BgPad(1)
	rawSep := " "
	if innerW >= 130 {
		sep = BgPad(2)
		rawSep = "  "
	}

	curItems := make([]keybindItem, len(items))
	copy(curItems, items)

	for len(curItems) > 0 {
		var renderedParts []string
		var rawParts []string
		for _, it := range curItems {
			pill := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render(it.key) + theme.StylePrimary.Render("⌟")
			renderedParts = append(renderedParts, pill+BgPad(1)+theme.StyleSecondary.Render(it.label))
			rawParts = append(rawParts, "⌜"+it.key+"⌟ "+it.label)
		}

		rawShortcuts := strings.Join(rawParts, rawSep)
		shortcutsWidth := ansi.StringWidth(rawShortcuts) + 2

		if innerW >= shortcutsWidth+2 {
			remDashes := innerW - shortcutsWidth
			leftD := remDashes / 2
			rightD := remDashes - leftD
			return cornerBL +
				theme.StyleFaint.Render(strings.Repeat("─", leftD)) +
				BgPad(1) + strings.Join(renderedParts, sep) + BgPad(1) +
				theme.StyleFaint.Render(strings.Repeat("─", rightD)) +
				cornerBR
		}
		curItems = curItems[:len(curItems)-1]
	}

	return cornerBL + theme.StyleFaint.Render(strings.Repeat("─", innerW)) + cornerBR
}

func renderHelpOverlay(p ViewParams) string {
	modal := RenderKeybindsModal(p.KeybindItems, p.HelpIndex, p.HelpEditing, p.Width, p.Height)
	return CenterOverlay(modal, p.Width, p.Height)
}

func renderDevicesOverlay(p ViewParams) string {
	modal := RenderDevicesModal(p.Devices, p.DeviceIndex, p.DeviceScanning, p.Width, p.Height)
	return CenterOverlay(modal, p.Width, p.Height)
}

func renderZenLyrics(lines []lyrics.Line, cursorLine int, progressMs int, width, height int) []string {
	var output []string
	if width < 10 {
		width = 10
	}
	if height < 1 {
		height = 1
	}

	if len(lines) == 0 {
		msg := theme.StyleFaint.Render("No lyrics available")
		midRow := height / 2
		for i := 0; i < height; i++ {
			if i == midRow {
				output = append(output, CenterLine(msg, width))
			} else {
				output = append(output, PadToWidth("", width))
			}
		}
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
		midRow := height / 2
		targetCenter := cursorLine
		if targetCenter < 0 {
			targetCenter = 0
		}
		for r := 0; r < height; r++ {
			if r == 0 {
				output = append(output, CenterLine(theme.StyleFaint.Render("[Lyrics not synced]"), width))
				continue
			}
			idx := targetCenter - (midRow - r)
			if idx < 0 || idx >= len(lines) {
				output = append(output, PadToWidth("", width))
				continue
			}
			text := lines[idx].Text
			if text == "" {
				text = "♪"
			}
			if idx == cursorLine {
				styled := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Primary).Background(theme.CurrentTheme.Surface).Bold(true).Render("❯  " + text + "  ❮")
				output = append(output, CenterLine(styled, width))
			} else {
				output = append(output, CenterLine(theme.StyleFaint.Render(text), width))
			}
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

	midRow := height / 2

	for r := 0; r < height; r++ {
		idx := targetCenter - (midRow - r)
		if idx < 0 || idx >= len(lines) {
			output = append(output, PadToWidth("", width))
			continue
		}

		line := lines[idx]
		text := line.Text
		if text == "" {
			text = "♪"
		}

		isSinging := idx == activeIdx
		isCursor := idx == cursorLine

		var styledText string
		if isCursor && isSinging {
			styledText = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render("❯  " + text + "  ❮")
		} else if isCursor {
			styledText = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Primary).Background(theme.CurrentTheme.Surface).Bold(true).Render("➜  " + text + "  ➜")
		} else if isSinging {
			styledText = lipgloss.NewStyle().Foreground(theme.CurrentTheme.Tertiary).Background(theme.CurrentTheme.Surface).Bold(true).Render("❯  " + text + "  ❮")
		} else {
			styledText = theme.StyleFaint.Render(text)
		}

		output = append(output, CenterLine(styledText, width))
	}

	return output
}

func renderZenMode(p ViewParams) string {
	innerW := p.Width - 2
	innerH := p.Height - 2

	greenDot := lipgloss.NewStyle().Foreground(theme.CurrentTheme.Success).Background(theme.CurrentTheme.Surface).Render("● ")
	titleTag := theme.StyleFaint.Render("─ ") + greenDot + theme.StylePrimary.Render("spotumn ") + theme.StyleSecondary.Render("◖Zen Mode◗")
	viewModeStr := p.ZenView.String()
	viewBadge := theme.StylePrimary.Render("◖") + theme.StyleBold.Render(viewModeStr) + theme.StylePrimary.Render("◗")
	viewTag := BgPad(1) + viewBadge + theme.StyleFaint.Render(" ─")

	rawTitle := "─ ● spotumn ◖Zen Mode◗"
	rawView := " ◖" + viewModeStr + "◗ ─"
	middleDashesLen := innerW - ansi.StringWidth(rawTitle) - ansi.StringWidth(rawView)
	if middleDashesLen < 1 {
		middleDashesLen = 1
	}

	cornerTL := theme.StyleFaint.Render("╭")
	cornerTR := theme.StyleFaint.Render("╮")
	sideBar := theme.StyleFaint.Render("│")

	topBorder := cornerTL + titleTag + theme.StyleFaint.Render(strings.Repeat("─", middleDashesLen)) + viewTag + cornerTR

	var items []keybindItem
	if innerW >= 70 {
		items = []keybindItem{
			{"Space", "Play"},
			{"n/p", "Prev/Next"},
			{"↑/↓", "Lyrics"},
			{"Enter", "Seek"},
			{"v", "View"},
			{"z", "Exit Zen"},
		}
	} else if innerW >= 50 {
		items = []keybindItem{
			{"Space", "Play"},
			{"↑/↓", "Lyrics"},
			{"v", "View"},
			{"z", "Exit"},
		}
	} else {
		items = []keybindItem{
			{"Space", "Play"},
			{"v", "View"},
			{"z", "Exit"},
		}
	}

	bottomBorder := renderKeybindBar(items, innerW)

	curProgress := 0
	durationMs := 0
	volume := 50
	playing := false
	trackName := "No Track Playing"
	artistName := "spotumn"
	if p.Playback != nil {
		curProgress = p.Playback.ProgressMs
		durationMs = p.Playback.DurationMs
		volume = p.Playback.Volume
		playing = p.Playback.Playing
		if p.Playback.CurrentTrack != nil {
			trackName = p.Playback.CurrentTrack.Name
			artistName = p.Playback.CurrentTrack.Artist
		}
	}
	elapsed := FormatDuration(curProgress)
	total := FormatDuration(durationMs)
	ratio := 0.0
	if durationMs > 0 {
		ratio = float64(curProgress) / float64(durationMs)
	}
	playIcon := "▶"
	if playing {
		playIcon = "❚❚"
	}

	volBar := RenderMiniSlider(volume, 8)
	vStyled := theme.StyleFaint.Render("Vol: [") + volBar + theme.StyleFaint.Render(fmt.Sprintf("] %2d%%", volume))

	artANSI := p.ZenArtANSI
	if artANSI == "" {
		artANSI = p.ArtANSI
	}

	var innerLines []string

	switch p.ZenView {
	case ZenViewBoth:
		gapW := 2
		leftW := (innerW - gapW) * 48 / 100
		if leftW < 30 {
			leftW = (innerW - gapW) / 2
		}
		if leftW > innerW-25 {
			leftW = innerW - 25
		}
		if leftW < 10 {
			leftW = innerW / 2
		}
		rightW := innerW - leftW - gapW
		if rightW < 5 {
			rightW = 5
		}

		maxArtRows := innerH - 8
		if maxArtRows < 4 {
			maxArtRows = 4
		}

		var displayedArtRows []string
		if artANSI != "" {
			artRows := strings.Split(artANSI, "\n")
			if len(artRows) > maxArtRows {
				artRows = artRows[:maxArtRows]
			}
			for _, row := range artRows {
				displayedArtRows = append(displayedArtRows, CenterLine(row, leftW))
			}
		}

		artStackH := len(displayedArtRows) + 7
		topPad := (innerH - artStackH) / 2
		if topPad < 0 {
			topPad = 0
		}

		var leftLines []string
		for i := 0; i < topPad; i++ {
			leftLines = append(leftLines, PadToWidth("", leftW))
		}
		leftLines = append(leftLines, displayedArtRows...)

		leftLines = append(leftLines, PadToWidth("", leftW))
		tStyled := theme.StyleBold.Render(TruncateString(trackName, leftW-4))
		leftLines = append(leftLines, CenterLine(tStyled, leftW))

		aStyled := theme.StyleSecondary.Render(TruncateString(artistName, leftW-4))
		leftLines = append(leftLines, CenterLine(aStyled, leftW))

		leftLines = append(leftLines, PadToWidth("", leftW))

		ctrls := fmt.Sprintf("%s   [ ⏮  %s  ⏭ ]   %s", elapsed, playIcon, total)
		if leftW < 36 {
			ctrls = fmt.Sprintf("%s  %s  %s", elapsed, playIcon, total)
		}
		cStyled := theme.StylePrimary.Render(ctrls)
		leftLines = append(leftLines, CenterLine(cStyled, leftW))

		barLen := leftW - 10
		if barLen > 36 {
			barLen = 36
		}
		if barLen < 10 {
			barLen = 10
		}
		seek := renderProgressBar(ratio, barLen)
		leftLines = append(leftLines, CenterLine(seek, leftW))
		leftLines = append(leftLines, CenterLine(vStyled, leftW))

		for len(leftLines) < innerH {
			leftLines = append(leftLines, PadToWidth("", leftW))
		}
		if len(leftLines) > innerH {
			leftLines = leftLines[:innerH]
		}

		rightLines := renderZenLyrics(p.LyricsLines, p.LyricsCursor, curProgress, rightW, innerH)

		divider := BgPad(gapW)
		for i := 0; i < innerH; i++ {
			innerLines = append(innerLines, leftLines[i]+divider+rightLines[i])
		}

	case ZenViewArt:
		maxArtRows := innerH - 8
		if maxArtRows < 4 {
			maxArtRows = 4
		}

		var displayedArtRows []string
		if artANSI != "" {
			artRows := strings.Split(artANSI, "\n")
			if len(artRows) > maxArtRows {
				artRows = artRows[:maxArtRows]
			}
			for _, row := range artRows {
				displayedArtRows = append(displayedArtRows, CenterLine(row, innerW))
			}
		}

		artStackH := len(displayedArtRows) + 7
		topPad := (innerH - artStackH) / 2
		if topPad < 0 {
			topPad = 0
		}

		for i := 0; i < topPad; i++ {
			innerLines = append(innerLines, PadToWidth("", innerW))
		}
		innerLines = append(innerLines, displayedArtRows...)

		innerLines = append(innerLines, PadToWidth("", innerW))
		tStyled := theme.StyleBold.Render(TruncateString(trackName, innerW-4))
		innerLines = append(innerLines, CenterLine(tStyled, innerW))

		aStyled := theme.StyleSecondary.Render(TruncateString(artistName, innerW-4))
		innerLines = append(innerLines, CenterLine(aStyled, innerW))

		innerLines = append(innerLines, PadToWidth("", innerW))

		ctrls := fmt.Sprintf("%s   [ ⏮  %s  ⏭ ]   %s", elapsed, playIcon, total)
		cStyled := theme.StylePrimary.Render(ctrls)
		innerLines = append(innerLines, CenterLine(cStyled, innerW))

		barLen := 44
		if barLen > innerW-10 {
			barLen = innerW - 10
		}
		if barLen < 12 {
			barLen = 12
		}
		seek := renderProgressBar(ratio, barLen)
		innerLines = append(innerLines, CenterLine(seek, innerW))
		innerLines = append(innerLines, CenterLine(vStyled, innerW))

		for len(innerLines) < innerH {
			innerLines = append(innerLines, PadToWidth("", innerW))
		}
		if len(innerLines) > innerH {
			innerLines = innerLines[:innerH]
		}

	case ZenViewLyrics:

		banner := theme.StyleBold.Render(TruncateString(trackName, innerW/2)) + theme.StyleFaint.Render(" ─ ") + theme.StyleSecondary.Render(TruncateString(artistName, innerW/2))
		topRow := CenterLine(banner, innerW)

		ctrls := fmt.Sprintf("%s   [ ⏮  %s  ⏭ ]   %s", elapsed, playIcon, total)
		barLen := 40
		if barLen > innerW-10 {
			barLen = innerW - 10
		}
		seek := renderProgressBar(ratio, barLen)
		rowControls := CenterLine(theme.StylePrimary.Render(ctrls), innerW)
		rowSeek := CenterLine(seek, innerW)
		rowVol := CenterLine(vStyled, innerW)

		bottomH := 4
		topH := 2

		lyricsH := innerH - topH - bottomH
		if lyricsH < 1 {
			lyricsH = 1
		}

		lLines := renderZenLyrics(p.LyricsLines, p.LyricsCursor, curProgress, innerW, lyricsH)

		innerLines = append(innerLines, PadToWidth("", innerW))
		innerLines = append(innerLines, topRow)
		innerLines = append(innerLines, lLines...)
		innerLines = append(innerLines, PadToWidth("", innerW))
		innerLines = append(innerLines, rowControls)
		innerLines = append(innerLines, rowSeek)
		innerLines = append(innerLines, rowVol)

		for len(innerLines) < innerH {
			innerLines = append(innerLines, PadToWidth("", innerW))
		}
		if len(innerLines) > innerH {
			innerLines = innerLines[:innerH]
		}
	}

	var finalLines []string
	finalLines = append(finalLines, PadToWidth(topBorder, p.Width))
	for i := 0; i < innerH; i++ {
		row := ""
		if i < len(innerLines) {
			row = innerLines[i]
		}
		borderedRow := sideBar + PadToWidth(row, innerW) + sideBar
		finalLines = append(finalLines, PadToWidth(borderedRow, p.Width))
	}
	finalLines = append(finalLines, PadToWidth(bottomBorder, p.Width))

	for len(finalLines) < p.Height {
		finalLines = append(finalLines, PadToWidth("", p.Width))
	}
	if len(finalLines) > p.Height {
		finalLines = finalLines[:p.Height]
	}

	return strings.Join(finalLines, "\n")
}

func joinHorizontalThemed(views []string, widths []int, height int) string {
	var splitViews [][]string
	for i, v := range views {
		lines := strings.Split(v, "\n")
		w := widths[i]
		for len(lines) < height {
			lines = append(lines, PadToWidth("", w))
		}
		if len(lines) > height {
			lines = lines[:height]
		}
		splitViews = append(splitViews, lines)
	}

	var combined []string
	for r := 0; r < height; r++ {
		var row strings.Builder
		for i, vLines := range splitViews {
			line := vLines[r]
			w := widths[i]
			if StringDisplayWidth(line) > w {
				line = TruncateVisualWidth(line, w, "") + "\x1b[0m"
			}
			row.WriteString(line)
		}
		combined = append(combined, row.String())
	}
	return strings.Join(combined, "\n")
}
