// Help and shortcut modal - lists categorized keybinds and handles interactive key rebinding.
package modals

import (
	"strings"

	"spotumn/internal/ui/state"
	"spotumn/internal/ui/theme"
)

type helpDisplayRow struct {
	isHeader bool
	category string
	itemIdx  int
}

func RenderKeybindsModal(items []state.KeybindItem, selectedIdx int, isEditing bool, width, height int) string {
	modalW := 100
	if modalW > width-4 {
		modalW = width - 4
	}
	if modalW < 44 {
		modalW = 44
	}

	var sb strings.Builder
	sb.WriteString(theme.PadToWidth("", modalW-2) + "\n")

	badgeNav := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("↑/↓") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Navigate")
	badgeEdit := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("Enter") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Customize")
	badgeReset := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("0") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Reset")
	badgeClose := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("Esc / ?") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Close")
	sb.WriteString(theme.PadToWidth("  "+badgeNav+theme.BgPad(3)+badgeEdit+theme.BgPad(3)+badgeReset+theme.BgPad(3)+badgeClose, modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")

	tipLine := theme.StyleError.Render("  ⓘ   Volume adjustments have a cooldown (~200ms) • Gray keybinds are fixed")
	sb.WriteString(theme.PadToWidth(tipLine, modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth("", modalW-2) + "\n")

	var displayRows []helpDisplayRow
	lastCategory := ""
	selectedRowIdx := 0

	for i, item := range items {
		cat := item.Category
		if cat == "" {
			cat = "General"
		}
		if cat != lastCategory {
			lastCategory = cat
			displayRows = append(displayRows, helpDisplayRow{
				isHeader: true,
				category: cat,
				itemIdx:  -1,
			})
		}
		if i == selectedIdx {
			selectedRowIdx = len(displayRows)
		}
		displayRows = append(displayRows, helpDisplayRow{
			isHeader: false,
			category: cat,
			itemIdx:  i,
		})
	}

	availH := height - 12
	if isEditing {
		availH = height - 15
	}
	if availH < 6 {
		availH = 6
	}
	if availH > 22 {
		availH = 22
	}

	totalRows := len(displayRows)
	startRow := 0
	if selectedRowIdx >= availH {
		startRow = selectedRowIdx - availH + 1
	}
	endRow := startRow + availH
	if endRow > totalRows {
		endRow = totalRows
		if endRow-availH >= 0 {
			startRow = endRow - availH
		} else {
			startRow = 0
		}
	}

	viewH := endRow - startRow
	thumbH := 1
	thumbStart := 0
	if totalRows > viewH && viewH > 0 {
		thumbH = (viewH * viewH) / totalRows
		if thumbH < 1 {
			thumbH = 1
		}
		maxScroll := totalRows - viewH
		if maxScroll > 0 {
			thumbStart = (startRow * (viewH - thumbH)) / maxScroll
		}
		if thumbStart+thumbH > viewH {
			thumbStart = viewH - thumbH
		}
		if thumbStart < 0 {
			thumbStart = 0
		}
	}

	contentW := modalW - 5
	if contentW < 30 {
		contentW = 30
	}

	for r := 0; r < viewH; r++ {
		rowIdx := startRow + r
		dRow := displayRows[rowIdx]

		var scrollIndicator string
		if totalRows > viewH {
			if r >= thumbStart && r < thumbStart+thumbH {
				scrollIndicator = theme.StylePrimary.Render("█")
			} else {
				scrollIndicator = theme.StyleFaint.Render("│")
			}
		} else {
			scrollIndicator = theme.BgPad(1)
		}

		if dRow.isHeader {
			base := theme.StylePrimary.Render("  ── " + dRow.category + " ")
			dashesCount := contentW - len(dRow.category) - 8
			if dashesCount < 4 {
				dashesCount = 4
			}
			dashes := theme.StylePrimary.Render(strings.Repeat("─", dashesCount))
			lineContent := theme.PadToWidth(base+dashes, contentW)
			sb.WriteString(theme.PadToWidth(lineContent+theme.BgPad(1)+scrollIndicator, modalW-2) + "\n")
			continue
		}

		item := items[dRow.itemIdx]
		isSelected := dRow.itemIdx == selectedIdx

		prefix := "  "
		if isSelected {
			prefix = "❯ "
		}

		keyText := item.Key
		if isSelected && isEditing {
			keyText = "[Press key...]"
		}

		keyPill := "⌜" + keyText + "⌟"
		if item.Key != item.DefaultKey && !(isSelected && isEditing) {
			keyPill = "⌜" + keyText + "⌟*"
		}

		pillFormatted := theme.PadPlain(keyPill, 16)
		descFormatted := theme.TruncateString(item.Desc, contentW-24)

		if isSelected {
			rawLine := prefix + pillFormatted + " " + descFormatted
			lineContent := theme.RenderPaddedLine(rawLine, theme.StyleActiveFocusedBlock, contentW)
			sb.WriteString(theme.PadToWidth(lineContent+theme.BgPad(1)+scrollIndicator, modalW-2) + "\n")
		} else {
			var styledPill string
			if item.ReadOnly {
				styledPill = theme.StyleFaint.Render(pillFormatted)
			} else if item.Key != item.DefaultKey {
				styledPill = theme.StyleTertiary.Render(pillFormatted)
			} else {
				styledPill = theme.StyleSecondary.Render(pillFormatted)
			}
			styledPrefix := theme.StyleNormal.Render(prefix)
			styledLine := styledPrefix + styledPill + theme.BgPad(1) + theme.StyleNormal.Render(descFormatted)
			lineContent := theme.PadToWidth(styledLine, contentW)
			sb.WriteString(theme.PadToWidth(lineContent+theme.BgPad(1)+scrollIndicator, modalW-2) + "\n")
		}
	}

	if isEditing {
		sb.WriteString(theme.PadToWidth("", modalW-2) + "\n" + theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")
		footer := theme.BgPad(2) + theme.StyleTertiary.Render("⌨  Press any key to assign...") + theme.BgPad(3) + theme.StyleFaint.Render("⌜Esc⌟ Cancel")
		sb.WriteString(theme.PadToWidth(footer, modalW-2) + "\n")
	} else {
		sb.WriteString(theme.PadToWidth("", modalW-2) + "\n" + theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")
		tip1 := theme.BgPad(4) + theme.StyleFaint.Render("Some keybinds cannot be modified.")
		sb.WriteString(theme.PadToWidth(tip1, modalW-2) + "\n")
	}

	boxStyle := theme.PanelBox(true, modalW, 0)
	return boxStyle.Render(sb.String())
}
