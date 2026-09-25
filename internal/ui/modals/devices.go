// Connected Devices modal - displays Spotify Connect devices and handles playback target selection.
package modals

import (
	"fmt"
	"strings"

	"spotumn/internal/backend"
	"spotumn/internal/ui/theme"

	"github.com/zmb3/spotify/v2"
)

// render interactive modal listing active and available spotify connect devices
func RenderDevicesModal(devices []spotify.PlayerDevice, selectedIdx int, isScanning bool, width, height int) string {
	devices = backend.SortDevicesWithSpotumnFirst(devices)
	modalW := 58
	if modalW > width-4 {
		modalW = width - 4
	}
	if modalW < 36 {
		modalW = 36
	}

	var sb strings.Builder
	title := theme.StylePrimary.Render("  Connected Devices")
	if isScanning {
		title += theme.BgPad(1) + theme.StyleTertiary.Render("● Scanning...")
	}
	sb.WriteString(theme.PadToWidth(title, modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth(theme.StyleFaint.Render(strings.Repeat("─", modalW-2)), modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth("", modalW-2) + "\n")

	if len(devices) == 0 {
		emptyMsg := theme.StyleFaint.Render("  No devices found. Launch Spotify or spotumn.")
		if isScanning {
			emptyMsg = theme.StyleTertiary.Render("  Scanning for Spotify Connect devices...")
		}
		sb.WriteString(theme.PadToWidth(emptyMsg, modalW-2) + "\n")
		sb.WriteString(theme.PadToWidth("", modalW-2) + "\n")
	} else {
		for i, d := range devices {
			isSelected := i == selectedIdx
			prefix := "  "
			if isSelected {
				prefix = "❯ "
			}

			activeTag := ""
			if d.Active {
				activeTag = " [Active]"
			}

			devType := d.Type
			if devType == "" {
				devType = "Speaker"
			}
			glyph := theme.DeviceTypeGlyph(devType)

			line := fmt.Sprintf("%s%s%s (%s)%s", prefix, glyph, d.Name, devType, activeTag)
			truncLine := theme.TruncateString(line, modalW-4)

			if isSelected {
				sb.WriteString(theme.RenderPaddedLine(truncLine, theme.StyleActiveFocusedBlock, modalW-2) + "\n")
			} else if d.Active {
				sb.WriteString(theme.PadToWidth(theme.StyleTertiary.Render(truncLine), modalW-2) + "\n")
			} else {
				sb.WriteString(theme.PadToWidth(theme.StyleNormal.Render(truncLine), modalW-2) + "\n")
			}
		}
		sb.WriteString(theme.PadToWidth("", modalW-2) + "\n")
	}

	footer := theme.StyleFaint.Render("  ⌜Enter⌟ Select   ⌜r⌟ Rescan   ⌜Esc / d⌟ Close")
	if isScanning {
		footer = theme.StyleFaint.Render("  ⌜Enter⌟ Select   ") + theme.StyleTertiary.Render("⌜r⌟ Scanning...") + theme.StyleFaint.Render("   ⌜Esc / d⌟ Close")
	}
	sb.WriteString(theme.PadToWidth(footer, modalW-2) + "\n")

	boxStyle := theme.PanelBox(true, modalW, 0)
	return boxStyle.Render(sb.String())
}
