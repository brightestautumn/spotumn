// Settings modal - configures audio engine, appearance, themes, cache management, and account actions.
package modals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"spotumn/internal/auth"
	"spotumn/internal/config"
	"spotumn/internal/ui/state"
	"spotumn/internal/ui/theme"
)

const SettingItemCount = 10

func NewSettingsState() state.SettingsState {
	cfg := config.Get()
	themes := theme.ListAvailableThemes()
	accMgr := auth.NewAccountManager()
	mode := cfg.AppearanceMode
	if mode != "light" {
		mode = "dark"
	}
	return state.SettingsState{
		Index:         0,
		Themes:        themes,
		CurrentTheme:  cfg.Theme,
		Mode:          mode,
		AutoShrink:    cfg.AutoShrinkSidebars,
		CrossfadeSec:  cfg.CrossfadeSec,
		Bitrate:       cfg.Bitrate,
		Normalisation: cfg.Normalisation,
		Autoplay:      cfg.AutoplayOnStartup,
		Accounts:      accMgr.GetAccounts(),
		ActiveAccIdx:  accMgr.GetActiveIndex(),
		CacheTarget:   state.CacheTargetAll,
		ConfirmLogout: false,
	}
}

// wipe selective cache directories (album art, cached audio blocks, or playback state)
func ClearCacheTarget(target int) error {
	cacheDir := config.GetCacheDir()
	le := func(err error, msg string) {
		if err != nil && config.LogError != nil {
			config.LogError("ui.modals", msg+": "+err.Error(), "settings.go")
		}
	}
	switch target {
	case state.CacheTargetArt:
		le(os.RemoveAll(filepath.Join(cacheDir, "art")), "rm art cache")
		le(os.MkdirAll(filepath.Join(cacheDir, "art"), 0700), "mkdir art cache")
	case state.CacheTargetAudio:
		le(os.RemoveAll(filepath.Join(cacheDir, "librespot", "audio")), "rm audio cache")
		le(os.MkdirAll(filepath.Join(cacheDir, "librespot", "audio"), 0700), "mkdir audio cache")
	case state.CacheTargetState:
		le(os.Remove(filepath.Join(cacheDir, "last_state.json")), "rm last_state")
		le(os.Remove(filepath.Join(cacheDir, "librespot", "state.json")), "rm librespot state")
	case state.CacheTargetAll:
		fallthrough
	default:
		le(os.RemoveAll(filepath.Join(cacheDir, "art")), "rm art cache")
		le(os.RemoveAll(filepath.Join(cacheDir, "librespot", "audio")), "rm audio cache")
		le(os.Remove(filepath.Join(cacheDir, "last_state.json")), "rm last_state")
		le(os.Remove(filepath.Join(cacheDir, "librespot", "state.json")), "rm librespot state")
		le(os.MkdirAll(filepath.Join(cacheDir, "art"), 0700), "mkdir art cache")
		le(os.MkdirAll(filepath.Join(cacheDir, "librespot", "audio"), 0700), "mkdir audio cache")
	}
	return nil
}

func ClearCache() error {
	return ClearCacheTarget(state.CacheTargetAll)
}

func RenderSettingsModal(s state.SettingsState, width, height int) string {
	modalW := 110
	if modalW > width-4 {
		modalW = width - 4
	}
	if modalW < 44 {
		modalW = 44
	}

	var sb strings.Builder
	sb.WriteString(theme.PadToWidth("", modalW-2) + "\n")

	badgeNav := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("↑/↓") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Select")
	badgeArrows := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("←/→") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Adjust")
	badgeEnter := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("Enter") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Action")
	badgeClose := theme.StylePrimary.Render("⌜") + theme.StyleBold.Render("Esc") + theme.StylePrimary.Render("⌟") + theme.BgPad(1) + theme.StyleSecondary.Render("Close")
	tipsLine := "  " + badgeNav + theme.BgPad(3) + badgeArrows + theme.BgPad(3) + badgeEnter + theme.BgPad(3) + badgeClose

	sb.WriteString(theme.PadToWidth(tipsLine, modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")

	disclaimerAudio := theme.StyleHover.Render("  ⓘ Audio Engine: Settings save immediately and apply on next Spotumn restart.")
	disclaimerAccount := theme.StyleError.Render("  ⚠ Multi-Account: Switching is experimental and may not work correctly.")
	sb.WriteString(theme.PadToWidth(disclaimerAudio, modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth(disclaimerAccount, modalW-2) + "\n")
	sb.WriteString(theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")

	contentW := modalW - 6
	if contentW < 30 {
		contentW = 30
	}

	cacheNames := []string{
		"◀  All Cache (~/.cache/spotumn/)  ▶",
		"◀  Album Art Only (~/art/)  ▶",
		"◀  Audio Chunks Only (~/audio/)  ▶",
		"◀  Playback State Only  ▶",
	}
	selectedCacheName := cacheNames[0]
	if s.CacheTarget >= 0 && s.CacheTarget < len(cacheNames) {
		selectedCacheName = cacheNames[s.CacheTarget]
	}

	appModeStr := "DARK"
	if strings.EqualFold(s.Mode, "light") {
		appModeStr = "LIGHT"
	}

	rows := []struct {
		title    string
		val      string
		desc     string
		category string
	}{
		{
			title:    "Theme",
			val:      fmt.Sprintf("◀  %s  ▶", s.CurrentTheme),
			desc:     "",
			category: "Appearance & Interface",
		},
		{
			title:    "Appearance",
			val:      fmt.Sprintf("◀  %s  ▶", appModeStr),
			desc:     "",
			category: "Appearance & Interface",
		},
		{
			title: "Auto-Shrink Panels",
			val: func() string {
				if s.AutoShrink {
					return "◀  Enabled  ▶"
				}
				return "◀  Disabled  ▶"
			}(),
			desc:     "Auto-collapse on narrow screens",
			category: "Appearance & Interface",
		},
		{
			title: "Crossfade",
			val: func() string {
				if s.CrossfadeSec == 0 {
					return "◀  Off  ▶"
				}
				return fmt.Sprintf("◀  %ds  ▶", s.CrossfadeSec)
			}(),
			desc:     "",
			category: "Audio Engine",
		},
		{
			title: "Audio Quality",
			val: func() string {
				switch s.Bitrate {
				case 96:
					return "◀  Normal (96k)  ▶"
				case 160:
					return "◀  High (160k)  ▶"
				default:
					return "◀  Very High (320k)  ▶"
				}
			}(),
			desc:     "",
			category: "Audio Engine",
		},
		{
			title: "Volume Normalisation",
			val: func() string {
				if s.Normalisation {
					return "◀  Enabled  ▶"
				}
				return "◀  Disabled  ▶"
			}(),
			desc:     "",
			category: "Audio Engine",
		},
		{
			title: "Autoplay on Startup",
			val: func() string {
				if s.Autoplay {
					return "◀  Enabled  ▶"
				}
				return "◀  Disabled  ▶"
			}(),
			desc:     "Auto-resume last song if no active device",
			category: "Audio Engine",
		},
		{
			title: "Spotify Account",
			val: func() string {
				if len(s.Accounts) > 1 && s.ActiveAccIdx >= 0 && s.ActiveAccIdx < len(s.Accounts) {
					return fmt.Sprintf("[ [%d/%d] %s ]", s.ActiveAccIdx+1, len(s.Accounts), s.Accounts[s.ActiveAccIdx].DisplayName)
				} else if len(s.Accounts) == 1 {
					return fmt.Sprintf("[ %s ] (+ Add)", s.Accounts[0].DisplayName)
				}
				return "[ None ] (+ Add Account)"
			}(),
			desc: "",
		},
		{
			title: "Session Control",
			val: func() string {
				if s.ConfirmLogout {
					return "[ Confirm? Enter:Yes • Esc:No ]"
				}
				return "[ Log Out Session ]"
			}(),
			desc: func() string {
				if s.ConfirmLogout {
					return "Enter: confirm • Esc: cancel"
				}
				return ""
			}(),
			category: "Account & Session",
		},
		{
			title:    "Cache Management",
			val:      selectedCacheName,
			desc:     "",
			category: "Storage & Cache",
		},
	}

	lastCat := ""
	for i, r := range rows {
		if r.category != lastCat {
			lastCat = r.category
			base := theme.StylePrimary.Render("  ── " + r.category + " ")
			dashesCount := modalW - len(r.category) - 10
			if dashesCount < 4 {
				dashesCount = 4
			}
			dashes := theme.StyleFaint.Render(strings.Repeat("─", dashesCount))
			catStyled := base + dashes
			sb.WriteString(theme.PadToWidth(catStyled, modalW-2) + "\n")
		}

		isSelected := i == s.Index
		prefix := "  "
		if isSelected {
			prefix = "❯ "
		}

		titleCol := theme.PadPlain(r.title, 24)
		valCol := theme.PadPlain(r.val, 36)
		descCol := theme.TruncateString(r.desc, contentW-62)

		if isSelected {
			rawLine := prefix + titleCol + valCol + descCol
			styled := theme.RenderPaddedLine(rawLine, theme.StyleActiveFocusedBlock, contentW)
			sb.WriteString(theme.PadToWidth("  "+styled, modalW-2) + "\n")
		} else {
			styledTitle := theme.StyleBold.Render(titleCol)
			var styledVal string
			if s.ConfirmLogout && i == 7 {
				styledVal = theme.StyleHover.Render(valCol)
			} else {
				styledVal = theme.StyleTertiary.Render(valCol)
			}
			styledDesc := theme.StyleFaint.Render(descCol)
			line := prefix + styledTitle + styledVal + styledDesc
			sb.WriteString(theme.PadToWidth("  "+line, modalW-2) + "\n")
		}
	}

	sb.WriteString(theme.PadToWidth(theme.StyleFaint.Render("  "+strings.Repeat("─", modalW-6)), modalW-2) + "\n")
	if s.Status != "" {
		statusText := theme.StyleTertiary.Render("  " + s.Status)
		sb.WriteString(theme.PadToWidth(statusText, modalW-2) + "\n")
	} else {
		hintText := theme.StyleFaint.Render("  Themes: ~/.config/spotumn/themes/  •  Cache: ~/.cache/spotumn/")
		sb.WriteString(theme.PadToWidth(hintText, modalW-2) + "\n")
	}

	boxStyle := theme.PanelBox(true, modalW, 0)
	return boxStyle.Render(sb.String())
}
