// Keybinding definitions, custom user mappings, action categories, and key manager state.
package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"spotumn/internal/config"
	"spotumn/internal/ui/state"
)

const (
	ActionPlayPause     = "play_pause"
	ActionPrevTrack     = "prev"
	ActionNextTrack     = "next"
	ActionSeekBack      = "seek_back"
	ActionSeekFwd       = "seek_fwd"
	ActionSeekBackBig   = "seek_back_big"
	ActionSeekFwdBig    = "seek_fwd_big"
	ActionVolumeUp      = "vol_up"
	ActionVolumeDown    = "vol_down"
	ActionVolumeUpBig   = "vol_up_big"
	ActionVolumeDownBig = "vol_down_big"
	ActionQueueTrack    = "queue"
	ActionDevices       = "devices"
	ActionShuffle       = "shuffle"
	ActionRepeat        = "repeat"
	ActionFocusPrev     = "focus_prev"
	ActionFocusNext     = "focus_next"
	ActionCursorUp      = "cursor_up"
	ActionCursorDown    = "cursor_down"
	ActionSelect        = "select"
	ActionOpenArtist    = "open_artist"
	ActionBack          = "back"
	ActionTabTracks     = "tab_tracks"
	ActionTabLyrics     = "tab_lyrics"
	ActionTabHistory    = "tab_history"
	ActionSearch        = "search"
	ActionFilter        = "filter"
	ActionPin           = "pin"
	ActionToggleSidebar = "sidebar"
	ActionZenMode       = "zen_mode"
	ActionZenLayout     = "zen_layout"
	ActionHelp          = "help"
	ActionSettings      = "settings"
	ActionEscape        = "escape"
	ActionQuit          = "quit"
)

// default keybind catalog; core playback and system shortcuts are flagged ReadOnly
func GetDefaultKeybindItems() []state.KeybindItem {
	return []state.KeybindItem{
		{ID: ActionPlayPause, Category: "Playback Controls", Key: "Space", Keys: []string{"space"}, Desc: "Play / Pause playback", DefaultKey: "Space", DefaultKeys: []string{"space"}, ReadOnly: true},
		{ID: ActionPrevTrack, Category: "Playback Controls", Key: "p", Keys: []string{"p"}, Desc: "Previous track", DefaultKey: "p", DefaultKeys: []string{"p"}},
		{ID: ActionNextTrack, Category: "Playback Controls", Key: "n", Keys: []string{"n"}, Desc: "Next track", DefaultKey: "n", DefaultKeys: []string{"n"}},
		{ID: ActionSeekBack, Category: "Playback Controls", Key: "←", Keys: []string{"left", "<", ","}, Desc: "Seek backward 5s", DefaultKey: "←", DefaultKeys: []string{"left", "<", ","}, ReadOnly: true},
		{ID: ActionSeekFwd, Category: "Playback Controls", Key: "→", Keys: []string{"right", ">", "."}, Desc: "Seek forward 5s", DefaultKey: "→", DefaultKeys: []string{"right", ">", "."}, ReadOnly: true},
		{ID: ActionSeekBackBig, Category: "Playback Controls", Key: "Shift+←", Keys: []string{"shift+left"}, Desc: "Seek backward 10s", DefaultKey: "Shift+←", DefaultKeys: []string{"shift+left"}, ReadOnly: true},
		{ID: ActionSeekFwdBig, Category: "Playback Controls", Key: "Shift+→", Keys: []string{"shift+right"}, Desc: "Seek forward 10s", DefaultKey: "Shift+→", DefaultKeys: []string{"shift+right"}, ReadOnly: true},
		{ID: ActionVolumeUp, Category: "Playback Controls", Key: "=", Keys: []string{"="}, Desc: "Volume up 5 points", DefaultKey: "=", DefaultKeys: []string{"="}, ReadOnly: true},
		{ID: ActionVolumeDown, Category: "Playback Controls", Key: "-", Keys: []string{"-"}, Desc: "Volume down 5 points", DefaultKey: "-", DefaultKeys: []string{"-"}, ReadOnly: true},
		{ID: ActionVolumeUpBig, Category: "Playback Controls", Key: "+", Keys: []string{"+", "shift+=", "shift++"}, Desc: "Volume up 10 points (or Shift+=)", DefaultKey: "+", DefaultKeys: []string{"+", "shift+=", "shift++"}, ReadOnly: true},
		{ID: ActionVolumeDownBig, Category: "Playback Controls", Key: "_", Keys: []string{"_", "shift+-", "shift+_"}, Desc: "Volume down 10 points (or Shift+-)", DefaultKey: "_", DefaultKeys: []string{"_", "shift+-", "shift+_"}, ReadOnly: true},
		{ID: ActionQueueTrack, Category: "Playback Controls", Key: "q", Keys: []string{"q"}, Desc: "Add highlighted song to Spotify queue", DefaultKey: "q", DefaultKeys: []string{"q"}},
		{ID: ActionDevices, Category: "Playback Controls", Key: "d", Keys: []string{"d"}, Desc: "Open Connect devices popup", DefaultKey: "d", DefaultKeys: []string{"d"}},
		{ID: ActionShuffle, Category: "Playback Controls", Key: "s", Keys: []string{"s"}, Desc: "Toggle shuffle mode (󰒝 on / 󰒞 off)", DefaultKey: "s", DefaultKeys: []string{"s"}},
		{ID: ActionRepeat, Category: "Playback Controls", Key: "r", Keys: []string{"r"}, Desc: "Cycle repeat mode (󰑗 off / 󰑖 all / 󰑘 once)", DefaultKey: "r", DefaultKeys: []string{"r"}},

		{ID: ActionFocusPrev, Category: "Navigation & Library", Key: "[", Keys: []string{"["}, Desc: "Cycle pane focus backward", DefaultKey: "[", DefaultKeys: []string{"["}},
		{ID: ActionFocusNext, Category: "Navigation & Library", Key: "]", Keys: []string{"]"}, Desc: "Cycle pane focus forward", DefaultKey: "]", DefaultKeys: []string{"]"}},
		{ID: ActionCursorUp, Category: "Navigation & Library", Key: "↑", Keys: []string{"up"}, Desc: "Move cursor / navigate up", DefaultKey: "↑", DefaultKeys: []string{"up"}},
		{ID: ActionCursorDown, Category: "Navigation & Library", Key: "↓", Keys: []string{"down"}, Desc: "Move cursor / navigate down", DefaultKey: "↓", DefaultKeys: []string{"down"}},
		{ID: ActionSelect, Category: "Navigation & Library", Key: "Enter", Keys: []string{"enter"}, Desc: "Play track, open album, or seek lyrics", DefaultKey: "Enter", DefaultKeys: []string{"enter"}},
		{ID: ActionOpenArtist, Category: "Navigation & Library", Key: "a", Keys: []string{"a"}, Desc: "Open artist page of currently playing track", DefaultKey: "a", DefaultKeys: []string{"a"}},
		{ID: ActionBack, Category: "Navigation & Library", Key: "b", Keys: []string{"backspace", "b"}, Desc: "Go back to previous artist / container (or Backspace)", DefaultKey: "b", DefaultKeys: []string{"backspace", "b"}},
		{ID: ActionTabTracks, Category: "Navigation & Library", Key: "1", Keys: []string{"1"}, Desc: "Switch tab: 1:Tracks", DefaultKey: "1", DefaultKeys: []string{"1"}},
		{ID: ActionTabLyrics, Category: "Navigation & Library", Key: "2", Keys: []string{"2"}, Desc: "Switch tab: 2:Lyrics", DefaultKey: "2", DefaultKeys: []string{"2"}},
		{ID: ActionTabHistory, Category: "Navigation & Library", Key: "3", Keys: []string{"3"}, Desc: "Switch tab: 3:History", DefaultKey: "3", DefaultKeys: []string{"3"}},
		{ID: ActionSearch, Category: "Navigation & Library", Key: "/", Keys: []string{"/"}, Desc: "Search", DefaultKey: "/", DefaultKeys: []string{"/"}},
		{ID: ActionFilter, Category: "Navigation & Library", Key: "f", Keys: []string{"f"}, Desc: "Cycle library filters", DefaultKey: "f", DefaultKeys: []string{"f"}},
		{ID: ActionPin, Category: "Navigation & Library", Key: "*", Keys: []string{"*"}, Desc: "Pin / Unpin highlighted playlist", DefaultKey: "*", DefaultKeys: []string{"*"}},

		{ID: ActionToggleSidebar, Category: "Layout & System", Key: "Shift+H", Keys: []string{"shift+h", "H"}, Desc: "Hide focused sidebar (restores both if hidden)", DefaultKey: "Shift+H", DefaultKeys: []string{"shift+h", "H"}},
		{ID: ActionZenMode, Category: "Layout & System", Key: "z", Keys: []string{"z"}, Desc: "Toggle Zen mode", DefaultKey: "z", DefaultKeys: []string{"z"}},
		{ID: ActionZenLayout, Category: "Layout & System", Key: "v", Keys: []string{"v"}, Desc: "Cycle Zen layout", DefaultKey: "v", DefaultKeys: []string{"v"}},
		{ID: ActionSettings, Category: "Layout & System", Key: "`", Keys: []string{"`", "~"}, Desc: "Open Settings tab", DefaultKey: "`", DefaultKeys: []string{"`", "~"}, ReadOnly: true},
		{ID: ActionHelp, Category: "Layout & System", Key: "?", Keys: []string{"?"}, Desc: "Open / Close Help", DefaultKey: "?", DefaultKeys: []string{"?"}, ReadOnly: true},
		{ID: ActionEscape, Category: "Layout & System", Key: "Esc", Keys: []string{"esc"}, Desc: "Close modal / reset lyrics scroll / exit search", DefaultKey: "Esc", DefaultKeys: []string{"esc"}, ReadOnly: true},
		{ID: ActionQuit, Category: "Layout & System", Key: "Ctrl+C", Keys: []string{"ctrl+c"}, Desc: "Quit spotumn", DefaultKey: "Ctrl+C", DefaultKeys: []string{"ctrl+c"}, ReadOnly: true},
	}
}

type KeyManager struct {
	Items []state.KeybindItem
	file  string
}

func NewKeyManager() *KeyManager {
	km := &KeyManager{
		Items: GetDefaultKeybindItems(),
		file:  filepath.Join(config.GetDir(), "keybinds.json"),
	}
	km.Load()
	return km
}

func (km *KeyManager) Load() {
	data, err := os.ReadFile(km.file)
	if err != nil {
		if !os.IsNotExist(err) && config.LogError != nil {
			config.LogError("ui.keybinds", "read keybinds: "+err.Error(), "keybinds.go")
		}
		return
	}
	var custom map[string]string
	if err := json.Unmarshal(data, &custom); err != nil {
		if config.LogError != nil {
			config.LogError("ui.keybinds", "unmarshal keybinds: "+err.Error(), "keybinds.go")
		}
		return
	}
	for i := range km.Items {
		if km.Items[i].ReadOnly {
			continue
		}
		if k, ok := custom[km.Items[i].ID]; ok && strings.TrimSpace(k) != "" {
			km.Items[i].Key = FormatKeyDisplay(k)
			km.Items[i].Keys = []string{strings.ToLower(k)}
		}
	}
}

func (km *KeyManager) Save() error {
	custom := make(map[string]string)
	for _, it := range km.Items {
		if !it.ReadOnly && it.Key != it.DefaultKey {
			custom[it.ID] = it.Key
		}
	}
	data, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(km.file, data, 0600)
}

func (km *KeyManager) SetKey(idx int, key string) {
	if idx < 0 || idx >= len(km.Items) || km.Items[idx].ReadOnly {
		return
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	km.Items[idx].Key = FormatKeyDisplay(key)
	km.Items[idx].Keys = []string{strings.ToLower(key)}
}

func (km *KeyManager) ResetItem(idx int) {
	if idx >= 0 && idx < len(km.Items) && !km.Items[idx].ReadOnly {
		km.Items[idx].Key = km.Items[idx].DefaultKey
		km.Items[idx].Keys = make([]string, len(km.Items[idx].DefaultKeys))
		copy(km.Items[idx].Keys, km.Items[idx].DefaultKeys)
	}
}

func (km *KeyManager) ResetAll() {
	for i := range km.Items {
		km.ResetItem(i)
	}
}

func (km *KeyManager) IsKeyUsed(key string) bool {
	if km == nil {
		return false
	}
	norm := strings.ToLower(strings.TrimSpace(key))
	for _, it := range km.Items {
		for _, k := range it.Keys {
			if strings.ToLower(k) == norm {
				return true
			}
		}
	}
	return false
}

func (km *KeyManager) Action(pressedKey string) string {
	if km == nil {
		return ""
	}
	norm := strings.ToLower(pressedKey)
	for _, it := range km.Items {
		for _, k := range it.Keys {
			if strings.ToLower(k) == norm {
				return it.ID
			}
		}
	}
	return ""
}

func FormatKeyDisplay(key string) string {
	switch strings.ToLower(key) {
	case " ", "space":
		return "Space"
	case "enter":
		return "Enter"
	case "esc", "escape":
		return "Esc"
	case "tab":
		return "Tab"
	case "backspace":
		return "Backspace"
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	default:
		if strings.HasPrefix(strings.ToLower(key), "ctrl+") {
			return "Ctrl+" + strings.ToUpper(key[5:])
		}
		if strings.HasPrefix(strings.ToLower(key), "alt+") {
			return "Alt+" + strings.ToUpper(key[4:])
		}
		if strings.HasPrefix(strings.ToLower(key), "shift+") {
			return "Shift+" + strings.ToUpper(key[6:])
		}
		return key
	}
}
