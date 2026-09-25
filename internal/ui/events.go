// Keyboard input dispatcher and event routing - handles keypresses across navigation, playback, and modals.
package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"spotumn/internal/auth"
	"spotumn/internal/backend"
	"spotumn/internal/config"

	tea "charm.land/bubbletea/v2"
)

func logErr(component string, err error, file string) {
	if err != nil && config.LogError != nil {
		config.LogError(component, err.Error(), file)
	}
}

func logInfo(component string, msg string, file string) {
	if config.Log != nil {
		config.Log(component, msg, file)
	}
}

func getPinnedPath() string {
	return filepath.Join(config.GetDir(), "pinned.json")
}

func LoadPinned() ([]string, map[string]bool) {
	order := []string{}
	set := make(map[string]bool)
	data, err := os.ReadFile(getPinnedPath())
	if err == nil {
		if json.Unmarshal(data, &order) == nil && len(order) > 0 {
			for _, uri := range order {
				set[uri] = true
			}
		} else {
			var legacySet map[string]bool
			if json.Unmarshal(data, &legacySet) == nil {
				for uri, val := range legacySet {
					if val {
						order = append(order, uri)
						set[uri] = true
					}
				}
			}
		}
	}
	return order, set
}

func loadPinned() ([]string, map[string]bool) {
	return LoadPinned()
}

func savePinned(order []string) {
	data, _ := json.Marshal(order)
	logErr("ui.events", os.WriteFile(getPinnedPath(), data, 0600), "events.go")
}

// hoist pinned playlists to the top in user-defined order followed by remaining items
func sortPlaylistsWithPinned(playlists []backend.Playlist, pinnedOrder []string) []backend.Playlist {
	if len(playlists) <= 1 {
		return playlists
	}
	pinRank := make(map[string]int, len(pinnedOrder))
	for idx, uri := range pinnedOrder {
		pinRank[uri] = idx + 1
	}

	res := make([]backend.Playlist, len(playlists))
	copy(res, playlists)
	sort.SliceStable(res, func(i, j int) bool {
		rankI := pinRank[res[i].URI]
		rankJ := pinRank[res[j].URI]

		if rankI > 0 && rankJ > 0 {

			return rankI < rankJ
		}
		if rankI > 0 {
			return true
		}
		if rankJ > 0 {
			return false
		}
		return res[i].Index < res[j].Index
	})
	return res
}

func (m *AppModel) getKeyManager() *KeyManager {
	if m.keyManager == nil {
		m.keyManager = NewKeyManager()
	}
	return m.keyManager
}

func (m *AppModel) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	km := m.getKeyManager()
	act := km.Action(key)

	if act == ActionQuit || key == "ctrl+c" {
		logInfo("ui.app", "user initiated spotumn exit", "events.go")
		if m.playback != nil && m.playback.CurrentTrack != nil {
			m.client.SaveLastState(m.playback)
		}
		return m, tea.Quit
	}

	if m.showHelp {
		if m.helpEditing {
			if key == "esc" {
				m.helpEditing = false
				return m, nil
			}
			km.SetKey(m.helpIndex, key)
			logErr("ui.events", km.Save(), "events.go")
			m.helpEditing = false
			return m, nil
		}

		switch key {
		case "?", "esc":
			m.showHelp = false
			return m, nil
		case "up":
			if m.helpIndex > 0 {
				m.helpIndex--
			}
			return m, nil
		case "down":
			if m.helpIndex < len(km.Items)-1 {
				m.helpIndex++
			}
			return m, nil
		case "enter":
			if m.helpIndex >= 0 && m.helpIndex < len(km.Items) && !km.Items[m.helpIndex].ReadOnly {
				m.helpEditing = true
			}
			return m, nil
		case "0":
			if !km.IsKeyUsed("0") && m.helpIndex >= 0 && m.helpIndex < len(km.Items) && !km.Items[m.helpIndex].ReadOnly {
				if km.Items[m.helpIndex].Key != km.Items[m.helpIndex].DefaultKey {
					km.ResetItem(m.helpIndex)
				} else {
					km.ResetAll()
				}
				logErr("ui.events", km.Save(), "events.go")
			}
			return m, nil
		}
		return m, nil
	}

	if m.showSettings {
		switch key {
		case "`", "~":
			m.showSettings = false
			m.settingsState.ConfirmLogout = false
			return m, nil
		case "esc":
			if m.settingsState.ConfirmLogout {
				m.settingsState.ConfirmLogout = false
				m.settingsState.Status = "Logout cancelled."
				return m, nil
			}
			m.showSettings = false
			return m, nil
		case "up":
			m.settingsState.ConfirmLogout = false
			if m.settingsState.Index > 0 {
				m.settingsState.Index--
			}
			return m, nil
		case "down":
			m.settingsState.ConfirmLogout = false
			if m.settingsState.Index < SettingItemCount-1 {
				m.settingsState.Index++
			}
			return m, nil
		case "left":
			return m.handleSettingsArrow(-1)
		case "right":
			return m.handleSettingsArrow(1)
		case "enter":
			return m.handleSettingsAction()
		}
		return m, nil
	}

	if m.showDevices {
		switch key {
		case "esc", "d":
			m.showDevices = false
			m.deviceScanning = false
			return m, nil
		case "up":
			if m.deviceIndex > 0 {
				m.deviceIndex--
			}
			return m, nil
		case "down":
			if m.deviceIndex < len(m.devices)-1 {
				m.deviceIndex++
			}
			return m, nil
		case "enter":
			if m.deviceIndex >= 0 && m.deviceIndex < len(m.devices) {
				target := m.devices[m.deviceIndex]
				m.showDevices = false
				m.deviceScanning = false
				for i := range m.devices {
					m.devices[i].Active = (m.devices[i].ID == target.ID)
				}
				if m.playback == nil {
					m.playback = &backend.PlaybackState{
						DeviceID:   string(target.ID),
						DeviceName: target.Name,
						DeviceType: target.Type,
						Volume:     int(target.Volume),
						Playing:    true,
					}
				} else {
					m.playback.DeviceID = string(target.ID)
					m.playback.DeviceName = target.Name
					m.playback.DeviceType = target.Type
				}
				m.client.SaveLastState(m.playback)
				m.client.SetSessionState(backend.StateTransferring)
				logInfo("ui.player", fmt.Sprintf("transferred playback to '%s' (%s, id=%s)", target.Name, target.Type, target.ID), "events.go")
				return m, func() tea.Msg {
					logErr("ui.events", m.client.TransferPlayback(context.Background(), target.ID), "events.go")
					for attempt := 0; attempt < 5; attempt++ {
						time.Sleep(time.Duration(150*(attempt+1)) * time.Millisecond)
						st, err := m.client.GetPlaybackState(context.Background())
						if err == nil && st != nil && (st.DeviceID == string(target.ID) || strings.EqualFold(st.DeviceName, target.Name)) {
							return PlaybackMsg(st)
						}
					}
					st, _ := m.client.GetPlaybackState(context.Background())
					return PlaybackMsg(st)
				}
			}
			return m, nil
		case "r":
			m.deviceScanning = true
			return m, m.fetchDevicesCmd()
		}
		m.deviceScanning = false
		return m, nil
	}

	if m.searchFocused {
		switch key {
		case "esc":
			m.searchFocused = false
			return m, nil
		case "enter":
			query := strings.TrimSpace(m.searchQuery)
			m.searchFocused = false
			if query != "" {
				return m, m.searchCmd(query)
			}
			return m, nil
		case "backspace":
			if len(m.searchQuery) > 0 {
				r := []rune(m.searchQuery)
				m.searchQuery = string(r[:len(r)-1])
			}
			return m, nil
		case "space":
			m.searchQuery += " "
			return m, nil
		default:
			text := msg.Key().Text
			if text != "" {
				m.searchQuery += text
				return m, nil
			} else if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
				m.searchQuery += key
				return m, nil
			}
		}
		return m, nil
	}

	switch act {
	case ActionOpenArtist:
		if m.playback == nil || m.playback.CurrentTrack == nil || m.playback.CurrentTrack.Name == "" {
			return m, nil
		}
		track := m.playback.CurrentTrack
		artistID := track.ArtistID
		artistName := track.Artist
		if artistID == "" && artistName == "" {
			return m, nil
		}

		if m.zenMode {
			m.zenMode = false
		}
		if m.showHelp {
			m.showHelp = false
		}
		if m.showDevices {
			m.showDevices = false
		}

		currID := ""
		if parts := strings.Split(m.currentPlURI, ":"); len(parts) >= 3 {
			currID = parts[2]
		}
		if currID != artistID {
			m.navHistory = append(m.navHistory, containerHistoryItem{
				ID:          currID,
				URI:         m.currentPlURI,
				Name:        m.currentPlName,
				CenterIndex: m.centerIndex,
				Tracks:      m.playlistTracks,
				Albums:      m.artistAlbums,
				Artists:     m.searchArtists,
			})
		}

		m.currentTab = TabTracks
		m.focused = PaneCenter
		m.centerIndex = 0
		m.searchArtists = nil
		m.searchQuery = ""
		m.searchFocused = false

		if artistID != "" {
			artURI := "spotify:artist:" + artistID
			m.currentPlURI = artURI
			m.currentPlName = artistName
			return m, m.fetchPlaylistTracksCmd(artistID, artURI, artistName)
		} else {
			return m, func() tea.Msg {
				art, err := m.client.GetArtistByName(context.Background(), artistName)
				if err != nil || art == nil {
					return nil
				}
				return openArtistResolvedMsg{Artist: *art}
			}
		}

	case ActionBack:
		if len(m.navHistory) > 0 {
			prev := m.navHistory[len(m.navHistory)-1]
			m.navHistory = m.navHistory[:len(m.navHistory)-1]
			m.currentPlURI = prev.URI
			m.currentPlName = prev.Name
			m.centerIndex = prev.CenterIndex
			if strings.HasPrefix(prev.URI, "search:") {
				m.playlistTracks = prev.Tracks
				m.artistAlbums = prev.Albums
				m.searchArtists = prev.Artists
				return m, nil
			}
			return m, m.fetchPlaylistTracksCmd(prev.ID, prev.URI, prev.Name)
		}
		return m, nil

	case ActionEscape:
		if m.showHelp {
			m.showHelp = false
			m.helpEditing = false
			return m, nil
		}
		if m.showDevices {
			m.showDevices = false
			m.deviceScanning = false
			return m, nil
		}
		if m.showSettings {
			m.showSettings = false
			return m, nil
		}
		if m.zenMode {
			m.zenMode = false
			m.lyricsManualScroll = false
			return m, nil
		}
		if len(m.navHistory) > 0 {
			prev := m.navHistory[len(m.navHistory)-1]
			m.navHistory = m.navHistory[:len(m.navHistory)-1]
			m.currentPlURI = prev.URI
			m.currentPlName = prev.Name
			m.centerIndex = prev.CenterIndex
			if strings.HasPrefix(prev.URI, "search:") {
				m.playlistTracks = prev.Tracks
				m.artistAlbums = prev.Albums
				m.searchArtists = prev.Artists
				return m, nil
			}
			return m, m.fetchPlaylistTracksCmd(prev.ID, prev.URI, prev.Name)
		}
		if m.lyricsManualScroll {
			m.lyricsManualScroll = false
			return m, nil
		}
		if m.searchFocused {
			m.searchFocused = false
			return m, nil
		}
		return m, nil

	case ActionSettings:
		if m.showSettings {
			m.showSettings = false
			logInfo("ui.modals", "closed settings modal", "events.go")
			return m, nil
		}
		m.settingsState = NewSettingsState()
		m.showSettings = true
		m.showHelp = false
		m.showDevices = false
		logInfo("ui.modals", "opened settings modal", "events.go")
		return m, nil

	case ActionHelp:
		m.showHelp = !m.showHelp
		m.helpEditing = false
		if m.showHelp {
			logInfo("ui.modals", "opened keyboard shortcuts help", "events.go")
		} else {
			logInfo("ui.modals", "closed keyboard shortcuts help", "events.go")
		}
		return m, nil

	case ActionDevices:
		m.showDevices = !m.showDevices
		if m.showDevices {
			m.deviceScanning = true
			m.deviceIndex = 0
			logInfo("ui.modals", "opened audio devices picker", "events.go")
			return m, m.fetchDevicesCmd()
		}
		m.deviceScanning = false
		logInfo("ui.modals", "closed audio devices picker", "events.go")
		return m, nil

	case ActionZenLayout:
		if m.zenMode {
			m.zenView = (m.zenView + 1) % 3
			views := []string{"lyrics only", "album art only", "both lyrics & art"}
			logInfo("ui.view", "zen view layout switched to "+views[m.zenView], "events.go")
			if m.zenView != ZenViewLyrics && m.lastArtURL != "" {
				_, _, w, h := m.getZenArtGeometry()
				if w > 0 && h > 0 {
					return m, m.fetchArtCmd(m.lastArtURL, w, h, true)
				}
			}
			return m, nil
		}

	case ActionZenMode:
		m.zenMode = !m.zenMode
		if m.zenMode {
			logInfo("ui.view", "entered distraction-free zen mode", "events.go")
		} else {
			logInfo("ui.view", "exited zen mode", "events.go")
		}
		if m.zenMode && m.zenView != ZenViewLyrics && m.lastArtURL != "" {
			_, _, w, h := m.getZenArtGeometry()
			if w > 0 && h > 0 {
				return m, m.fetchArtCmd(m.lastArtURL, w, h, true)
			}
		}
		return m, nil

	case ActionToggleSidebar:
		if m.focused == PaneNav {
			m.showLeftSidebar = false
			m.focused = PaneCenter
		} else if m.focused == PaneRight {
			m.showRightSidebar = false
			m.focused = PaneCenter
		} else {
			if !m.showLeftSidebar || !m.showRightSidebar {
				m.showLeftSidebar = true
				m.showRightSidebar = true
			} else {
				m.showRightSidebar = false
			}
		}
		logInfo("ui.view", fmt.Sprintf("toggled sidebars (left=%v, right=%v)", m.showLeftSidebar, m.showRightSidebar), "events.go")
		return m, nil

	case ActionSearch:
		m.searchFocused = true
		logInfo("ui.navigation", "focused search input", "events.go")
		return m, nil

	case ActionTabTracks:
		m.currentTab = TabTracks
		m.centerIndex = 0
		logInfo("ui.navigation", "switched tab to 'Tracks'", "events.go")
		return m, nil

	case ActionTabLyrics:
		m.currentTab = TabLyrics
		m.lyricsManualScroll = false
		logInfo("ui.navigation", "switched tab to 'Lyrics'", "events.go")
		if m.playback != nil && m.playback.CurrentTrack != nil {
			if len(m.lyricsLines) == 0 {
				m.lyricsLines = nil
				return m, m.fetchLyricsCmd(m.playback.CurrentTrack.URI, m.playback.CurrentTrack.Name, m.playback.CurrentTrack.Artist, m.playback.DurationMs/1000)
			}
		}
		return m, nil

	case ActionTabHistory:
		m.currentTab = TabHistory
		m.centerIndex = 0
		logInfo("ui.navigation", "switched tab to 'History'", "events.go")
		if len(m.history) == 0 {
			return m, m.fetchHistoryCmd()
		}
		return m, nil

	case ActionFilter:
		cmd := m.cycleFilter(1)
		return m, cmd

	case ActionPin:
		if m.focused == PaneNav {
			pls := m.filteredPlaylists()
			if m.navIndex >= 0 && m.navIndex < len(pls) {
				uri := pls[m.navIndex].URI
				if m.pinnedURIs == nil {
					m.pinnedURIs = make(map[string]bool)
				}
				if m.pinnedURIs[uri] {
					delete(m.pinnedURIs, uri)
					newOrder := make([]string, 0, len(m.pinnedOrder))
					for _, u := range m.pinnedOrder {
						if u != uri {
							newOrder = append(newOrder, u)
						}
					}
					m.pinnedOrder = newOrder
				} else {
					m.pinnedURIs[uri] = true
					m.pinnedOrder = append(m.pinnedOrder, uri)
				}
				savePinned(m.pinnedOrder)
				m.playlists = sortPlaylistsWithPinned(m.playlists, m.pinnedOrder)
				m.albums = sortPlaylistsWithPinned(m.albums, m.pinnedOrder)
				m.artists = sortPlaylistsWithPinned(m.artists, m.pinnedOrder)

				plsAfter := m.filteredPlaylists()
				for idx, pl := range plsAfter {
					if pl.URI == uri {
						m.navIndex = idx
						break
					}
				}
			}
		}
		return m, nil

	case ActionFocusNext:
		m.handleTab(1)
		return m, nil

	case ActionFocusPrev:
		m.handleTab(-1)
		return m, nil

	case ActionQueueTrack:
		trackURI := ""
		if m.focused == PaneCenter && m.currentTab == TabTracks && m.centerIndex >= 0 && m.centerIndex < len(m.playlistTracks) {
			trackURI = m.playlistTracks[m.centerIndex].URI
		} else if m.focused == PaneCenter && m.currentTab == TabHistory && m.centerIndex >= 0 && m.centerIndex < len(m.history) {
			trackURI = m.history[m.centerIndex].URI
		} else if m.playback != nil && m.playback.CurrentTrack != nil {
			trackURI = m.playback.CurrentTrack.URI
		}
		if trackURI != "" {
			logInfo("ui.player", "added track to playback queue: "+trackURI, "events.go")
			return m, func() tea.Msg {
				if m.isLocalActive() {
					logErr("ui.events", m.daemon.AddToQueue(trackURI), "events.go")
				} else {
					logErr("ui.events", m.client.QueueSong(context.Background(), trackURI), "events.go")
				}
				time.Sleep(200 * time.Millisecond)
				q, _ := m.client.GetQueue(context.Background())
				return QueueMsg(q)
			}
		}

		return m, nil

	case ActionPlayPause:
		isPlaying := false
		if m.playback != nil {
			isPlaying = m.playback.Playing
			m.playback.Playing = !isPlaying
			m.client.SaveLastState(m.playback)
		}
		if isPlaying {
			logInfo("ui.player", "paused playback", "events.go")
		} else {
			logInfo("ui.player", "resumed playback", "events.go")
		}
		m.lastActionTime = time.Now()
		trackURI := ""
		contextURI := ""
		progressMs := 0
		if m.playback != nil && m.playback.CurrentTrack != nil {
			trackURI = m.playback.CurrentTrack.URI
			contextURI = m.playback.ContextURI
			progressMs = m.playback.ProgressMs
		}
		return m, func() tea.Msg {
			if m.isLocalActive() {
				if !isPlaying && !m.localPlaybackReady && trackURI != "" {
					targetURI := contextURI
					if targetURI == "" {
						targetURI = trackURI
					}
					if err := m.daemon.PlayURI(targetURI, trackURI, progressMs); err == nil {
						return nil
					}
				}
				if err := m.daemon.PlayPause(); err == nil {
					return nil
				}
			}
			if isPlaying {
				logErr("ui.events", m.client.Pause(context.Background()), "events.go")
			} else {
				err := m.client.Play(context.Background())
				if err != nil && trackURI != "" {
					logErr("ui.events", m.client.PlayTrackAtPosition(context.Background(), trackURI, contextURI, progressMs), "events.go")
				}
			}
			return nil
		}

	case ActionPrevTrack:
		m.lastActionTime = time.Now()
		logInfo("ui.player", "skipped to previous track", "events.go")
		return m, func() tea.Msg {
			if m.isLocalActive() {
				if err := m.daemon.Previous(); err == nil {
					return nil
				}
			}
			logErr("ui.events", m.client.Previous(context.Background()), "events.go")
			return nil
		}

	case ActionNextTrack:
		m.lastActionTime = time.Now()
		trackDesc := ""
		if len(m.queue) > 0 && m.playback != nil {
			nextTrack := m.queue[0]
			m.playback.CurrentTrack = &nextTrack
			m.queue = m.queue[1:]
			m.client.SaveLastState(m.playback)
			trackDesc = fmt.Sprintf(" -> now playing '%s' by '%s'", nextTrack.Name, nextTrack.Artist)
		}
		logInfo("ui.player", "skipped to next track"+trackDesc, "events.go")
		return m, func() tea.Msg {
			if m.isLocalActive() {
				if err := m.daemon.Next(); err == nil {
					return nil
				}
			}
			logErr("ui.events", m.client.Next(context.Background()), "events.go")
			return nil
		}

	case ActionShuffle:
		curShuffle := false
		if m.playback != nil {
			curShuffle = m.playback.Shuffle
			m.playback.Shuffle = !curShuffle
		}
		m.lastActionTime = time.Now()
		logInfo("ui.player", fmt.Sprintf("shuffle mode changed: %v -> %v", curShuffle, !curShuffle), "events.go")
		return m, func() tea.Msg {
			if m.isLocalActive() {
				logErr("ui.events", m.daemon.SetShuffle(!curShuffle), "events.go")
			}
			logErr("ui.events", m.client.ToggleShuffle(context.Background(), curShuffle), "events.go")
			return nil
		}

	case ActionRepeat:
		curRepeat := "off"
		if m.playback != nil {
			curRepeat = m.playback.Repeat
		}
		var nextRepeat string
		switch curRepeat {
		case "off":
			nextRepeat = "context"
		case "context":
			nextRepeat = "track"
		default:
			nextRepeat = "off"
		}
		if m.playback != nil {
			m.playback.Repeat = nextRepeat
		}
		m.lastActionTime = time.Now()
		logInfo("ui.player", fmt.Sprintf("repeat mode changed: %s -> %s", curRepeat, nextRepeat), "events.go")
		return m, func() tea.Msg {
			if m.isLocalActive() {
				logErr("ui.events", m.daemon.SetRepeat(nextRepeat), "events.go")
			}
			logErr("ui.events", m.client.CycleRepeat(context.Background(), curRepeat), "events.go")
			return nil
		}

	case ActionVolumeUp, ActionVolumeDown, ActionVolumeUpBig, ActionVolumeDownBig:
		delta := 5
		if act == ActionVolumeDown {
			delta = -5
		} else if act == ActionVolumeUpBig {
			delta = 10
		} else if act == ActionVolumeDownBig {
			delta = -10
		}
		vol := 50
		if m.playback != nil {
			vol = m.playback.Volume
		}
		vol += delta
		if vol > 100 {
			vol = 100
		}
		if vol < 0 {
			vol = 0
		}
		if m.playback != nil {
			m.playback.Volume = vol
			m.client.SaveLastState(m.playback)
		}
		m.volumeTarget = vol
		target := vol
		logInfo("ui.player", fmt.Sprintf("volume set to %d%%", target), "events.go")
		return m, func() tea.Msg {
			time.Sleep(150 * time.Millisecond)
			if m.volumeTarget == target {
				if m.isLocalActive() {
					logErr("ui.events", m.daemon.SetVolume(target), "events.go")
				}
				logErr("ui.events", m.client.SetVolume(context.Background(), target), "events.go")
			}
			return nil
		}

	case ActionSeekBack, ActionSeekFwd, ActionSeekBackBig, ActionSeekFwdBig:
		deltaMs := 5000
		if act == ActionSeekBack {
			deltaMs = -5000
		} else if act == ActionSeekBackBig {
			deltaMs = -10000
		} else if act == ActionSeekFwdBig {
			deltaMs = 10000
		}
		pos := 0
		if m.playback != nil {
			pos = m.playback.ProgressMs + deltaMs
			if pos < 0 {
				pos = 0
			}
			if m.playback.DurationMs > 0 && pos > m.playback.DurationMs {
				pos = m.playback.DurationMs
			}
			m.playback.ProgressMs = pos
			m.client.SaveLastState(m.playback)
		}
		m.seekTarget = pos
		target := pos
		logInfo("ui.player", fmt.Sprintf("seeked playback to %02d:%02d (%dms)", target/60000, (target%60000)/1000, target), "events.go")
		return m, func() tea.Msg {
			time.Sleep(150 * time.Millisecond)
			if m.seekTarget == target {
				if m.isLocalActive() {
					logErr("ui.events", m.daemon.Seek(target), "events.go")
				}
				logErr("ui.events", m.client.Seek(context.Background(), target), "events.go")
			}
			return nil
		}

	case ActionCursorDown:
		m.moveCursor(1)
		return m, nil

	case ActionCursorUp:
		m.moveCursor(-1)
		return m, nil

	case ActionSelect:
		return m.handleEnter()
	}

	switch key {
	case "up":
		m.moveCursor(-1)
		return m, nil
	case "down":
		m.moveCursor(1)
		return m, nil
	case "enter":
		return m.handleEnter()
	case "esc":
		if len(m.navHistory) > 0 {
			prev := m.navHistory[len(m.navHistory)-1]
			m.navHistory = m.navHistory[:len(m.navHistory)-1]
			m.currentPlURI = prev.URI
			m.currentPlName = prev.Name
			m.centerIndex = prev.CenterIndex
			if strings.HasPrefix(prev.URI, "search:") {
				m.playlistTracks = prev.Tracks
				m.artistAlbums = prev.Albums
				m.searchArtists = prev.Artists
				return m, nil
			}
			return m, m.fetchPlaylistTracksCmd(prev.ID, prev.URI, prev.Name)
		}
		if m.zenMode || m.currentTab == TabLyrics {
			m.lyricsManualScroll = false
		}
		return m, nil
	}

	return m, nil
}

func (m *AppModel) handleSettingsArrow(delta int) (tea.Model, tea.Cmd) {
	cfg := config.Get()
	switch m.settingsState.Index {
	case 0:
		themes := m.settingsState.Themes
		if len(themes) == 0 {
			return m, nil
		}
		curIdx := 0
		for i, t := range themes {
			if strings.EqualFold(t, m.settingsState.CurrentTheme) {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(themes)) % len(themes)
		newTheme := themes[nextIdx]
		m.settingsState.CurrentTheme = newTheme
		cfg.Theme = newTheme
		logErr("ui.events", config.Save(cfg), "events.go")
		isDark := true
		if cfg.AppearanceMode == "light" {
			isDark = false
		}
		ApplyTheme(newTheme, isDark)
		m.settingsState.Status = "Theme set to " + newTheme

	case 1:
		modes := []string{"dark", "light"}
		curIdx := 0
		for i, mode := range modes {
			if strings.EqualFold(mode, m.settingsState.Mode) {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(modes)) % len(modes)
		newMode := modes[nextIdx]
		m.settingsState.Mode = newMode
		cfg.AppearanceMode = newMode
		logErr("ui.events", config.Save(cfg), "events.go")
		isDark := true
		if newMode == "light" {
			isDark = false
		}
		ApplyTheme(cfg.Theme, isDark)
		m.settingsState.Status = "Appearance set to " + newMode

	case 2:
		cfg.AutoShrinkSidebars = !cfg.AutoShrinkSidebars
		m.settingsState.AutoShrink = cfg.AutoShrinkSidebars
		m.autoShrinkSidebars = cfg.AutoShrinkSidebars
		logErr("ui.events", config.Save(cfg), "events.go")
		if cfg.AutoShrinkSidebars {
			m.settingsState.Status = "Auto-shrink sidebars: Enabled"
		} else {
			m.settingsState.Status = "Auto-shrink sidebars: Disabled"
		}

	case 3:
		val := m.settingsState.CrossfadeSec + delta
		if val < 0 {
			val = 0
		} else if val > 12 {
			val = 12
		}
		m.settingsState.CrossfadeSec = val
		cfg.CrossfadeSec = val
		logErr("ui.events", config.Save(cfg), "events.go")
		if val == 0 {
			m.settingsState.Status = "Crossfade: Off (saved • applies on restart)"
		} else {
			m.settingsState.Status = fmt.Sprintf("Crossfade: %ds (saved • applies on restart)", val)
		}

	case 4:
		bitrates := []int{96, 160, 320}
		curIdx := 2
		for i, b := range bitrates {
			if b == m.settingsState.Bitrate {
				curIdx = i
				break
			}
		}
		nextIdx := (curIdx + delta + len(bitrates)) % len(bitrates)
		newBitrate := bitrates[nextIdx]
		m.settingsState.Bitrate = newBitrate
		cfg.Bitrate = newBitrate
		logErr("ui.events", config.Save(cfg), "events.go")
		m.settingsState.Status = fmt.Sprintf("Audio quality: %dkbps (saved • applies on restart)", newBitrate)

	case 5:
		m.settingsState.Normalisation = !m.settingsState.Normalisation
		cfg.Normalisation = m.settingsState.Normalisation
		logErr("ui.events", config.Save(cfg), "events.go")
		if cfg.Normalisation {
			m.settingsState.Status = "Volume normalisation: Enabled (saved • applies on restart)"
		} else {
			m.settingsState.Status = "Volume normalisation: Disabled (saved • applies on restart)"
		}

	case 6:
		m.settingsState.Autoplay = !m.settingsState.Autoplay
		cfg.AutoplayOnStartup = m.settingsState.Autoplay
		logErr("ui.events", config.Save(cfg), "events.go")
		if m.settingsState.Autoplay {
			m.settingsState.Status = "Autoplay on startup: Enabled"
		} else {
			m.settingsState.Status = "Autoplay on startup: Disabled"
		}

	case 7:
		accMgr := auth.NewAccountManager()
		accounts := accMgr.GetAccounts()
		if len(accounts) <= 1 {
			return m, nil
		}
		curIdx := accMgr.GetActiveIndex()
		nextIdx := (curIdx + delta + len(accounts)) % len(accounts)
		switched, err := accMgr.SwitchAccount(nextIdx)
		if err == nil && switched != nil {
			m.settingsState.ActiveAccIdx = nextIdx
			m.settingsState.Status = "✓ Switched to " + switched.DisplayName + " (experimental)"
			authSvc := auth.NewAuthService(config.Get())
			m.client = backend.NewClient(context.Background(), authSvc.GetTokenSource(context.Background()))
			if m.daemon != nil {
				m.client.SetLocalDeviceID(m.daemon.DeviceId())
				logErr("ui.events", m.daemon.Restart(""), "events.go")
			}
			return m, tea.Batch(m.fetchUserCmd(), m.fetchPlaylistsCmd(), m.fetchPlaybackCmd())
		}

	case 8:
		if m.settingsState.ConfirmLogout {
			m.settingsState.ConfirmLogout = false
			m.settingsState.Status = "Logout cancelled."
		}

	case 9:
		targets := 4
		next := (m.settingsState.CacheTarget + delta + targets) % targets
		m.settingsState.CacheTarget = next
		names := []string{"All Cache", "Album Art Only", "Audio Chunks Only", "Playback State Only"}
		m.settingsState.Status = "Cache target: " + names[next]
	}
	if m.settingsState.Status != "" {
		logInfo("ui.settings", m.settingsState.Status, "events.go")
	}
	return m, nil
}

func (m *AppModel) handleSettingsAction() (tea.Model, tea.Cmd) {
	defer func() {
		if m.settingsState.Status != "" {
			logInfo("ui.settings", m.settingsState.Status, "events.go")
		}
	}()
	switch m.settingsState.Index {
	case 6:
		m.settingsState.Autoplay = !m.settingsState.Autoplay
		cfg := config.Get()
		cfg.AutoplayOnStartup = m.settingsState.Autoplay
		logErr("ui.events", config.Save(cfg), "events.go")
		if m.settingsState.Autoplay {
			m.settingsState.Status = "Autoplay on startup: Enabled"
		} else {
			m.settingsState.Status = "Autoplay on startup: Disabled"
		}
		return m, nil

	case 7:
		m.settingsState.Status = "Opening browser for Spotify login (experimental)..."
		return m, m.addAccountCmd()

	case 8:
		if !m.settingsState.ConfirmLogout {
			m.settingsState.ConfirmLogout = true
			m.settingsState.Status = "⚠ Confirm logout: Press Enter again to confirm, or Esc to cancel."
			return m, nil
		}
		m.settingsState.ConfirmLogout = false
		credPath := filepath.Join(config.GetDir(), "credentials.json")
		logErr("ui.events", os.Remove(credPath), "events.go")
		logErr("ui.events", os.Remove(filepath.Join(config.GetCacheDir(), "librespot", "state.json")), "events.go")
		m.settingsState.Status = "Session credentials cleared. Spotumn will require login on next launch."
		return m, nil

	case 9:
		logErr("ui.events", ClearCacheTarget(m.settingsState.CacheTarget), "events.go")
		names := []string{
			"All cache purged (~/.cache/spotumn)",
			"Album art cache purged (~/art)",
			"Audio chunks cache purged (~/librespot/audio)",
			"Playback state purged",
		}
		if m.settingsState.CacheTarget >= 0 && m.settingsState.CacheTarget < len(names) {
			m.settingsState.Status = "✓ " + names[m.settingsState.CacheTarget]
		} else {
			m.settingsState.Status = "✓ Cache purged."
		}
		return m, nil
	}
	return m, nil
}

func (m *AppModel) moveCursor(delta int) {
	if m.zenMode {
		m.lyricsManualScroll = true
		m.lyricsPointerMovedAt = time.Now()
		newIdx := m.lyricsCursor + delta
		if newIdx >= 0 && newIdx < len(m.lyricsLines) {
			m.lyricsCursor = newIdx
		}
		return
	}

	switch m.focused {
	case PaneNav:
		pls := m.filteredPlaylists()
		newIdx := m.navIndex + delta
		if newIdx >= 0 && newIdx < len(pls) {
			m.navIndex = newIdx
		}

	case PaneCenter:
		switch m.currentTab {
		case TabTracks:
			totalItems := len(m.playlistTracks) + len(m.artistAlbums) + len(m.searchArtists)
			newIdx := m.centerIndex + delta
			if newIdx >= 0 && newIdx < totalItems {
				m.centerIndex = newIdx
			}
		case TabHistory:
			newIdx := m.centerIndex + delta
			if newIdx >= 0 && newIdx < len(m.history) {
				m.centerIndex = newIdx
			}
		case TabLyrics:
			m.lyricsManualScroll = true
			m.lyricsPointerMovedAt = time.Now()
			newIdx := m.lyricsCursor + delta
			if newIdx >= 0 && newIdx < len(m.lyricsLines) {
				m.lyricsCursor = newIdx
			}
		}

	case PaneRight:
		newIdx := m.queueIndex + delta
		if newIdx >= 0 && newIdx < len(m.queue) {
			m.queueIndex = newIdx
		}
	}
}

func (m *AppModel) handleEnter() (tea.Model, tea.Cmd) {
	if m.zenMode {
		if m.lyricsCursor >= 0 && m.lyricsCursor < len(m.lyricsLines) {
			timeMs := m.lyricsLines[m.lyricsCursor].TimeMs
			m.lyricsManualScroll = false
			logInfo("ui.player", fmt.Sprintf("seeked lyrics to %02d:%02d (%dms)", timeMs/60000, (timeMs%60000)/1000, timeMs), "events.go")
			if m.playback != nil {
				m.playback.ProgressMs = timeMs
				m.client.SaveLastState(m.playback)
			}
			return m, func() tea.Msg {
				if m.isLocalActive() {
					logErr("ui.events", m.daemon.Seek(timeMs), "events.go")
				}
				logErr("ui.events", m.client.Seek(context.Background(), timeMs), "events.go")
				return nil
			}
		}
		return m, nil
	}

	switch m.focused {
	case PaneNav:
		pls := m.filteredPlaylists()
		if m.navIndex >= 0 && m.navIndex < len(pls) {
			pl := pls[m.navIndex]
			logInfo("ui.navigation", fmt.Sprintf("opened playlist '%s' (%s)", pl.Name, pl.URI), "events.go")
			m.navHistory = nil
			m.currentPlURI = pl.URI
			m.currentPlName = pl.Name
			m.currentTab = TabTracks
			m.focused = PaneCenter
			m.searchArtists = nil
			m.artistAlbums = nil
			m.searchQuery = ""
			m.searchFocused = false
			if cached, ok := m.containerCache[pl.URI]; ok && len(cached.tracks) > 0 {
				m.playlistTracks = cached.tracks
				m.artistAlbums = cached.albums
				m.centerIndex = 0
				return m, nil
			}
			m.playlistTracks = nil
			return m, m.fetchPlaylistTracksCmd(pl.ID, pl.URI, pl.Name)
		}

	case PaneCenter:
		switch m.currentTab {
		case TabTracks:
			if m.centerIndex >= 0 && m.centerIndex < len(m.playlistTracks) {
				idx := m.centerIndex
				tracks := m.playlistTracks
				plURI := m.currentPlURI
				logInfo("ui.player", fmt.Sprintf("started playback: '%s' by '%s' [track %d in '%s']", tracks[idx].Name, tracks[idx].Artist, idx+1, m.currentPlName), "events.go")
				return m, func() tea.Msg {
					if m.daemon != nil && m.daemon.IsRunning() && len(tracks) > 0 {
						targetURI := plURI
						if targetURI == "" || strings.HasPrefix(targetURI, "search:") {
							targetURI = tracks[idx].URI
						}
						if err := m.daemon.PlayURI(targetURI, tracks[idx].URI, 0); err == nil {
							m.client.SetSessionState(backend.StateLocalActive)
							time.Sleep(150 * time.Millisecond)
							st, _ := m.client.GetPlaybackState(context.Background())
							return PlaybackMsg(st)
						}
					}
					logErr("ui.events", m.client.PlayTrackList(context.Background(), tracks, idx, plURI), "events.go")
					time.Sleep(200 * time.Millisecond)
					st, _ := m.client.GetPlaybackState(context.Background())
					return PlaybackMsg(st)
				}

			} else if m.centerIndex >= len(m.playlistTracks) && m.centerIndex < len(m.playlistTracks)+len(m.artistAlbums) {
				albumIdx := m.centerIndex - len(m.playlistTracks)
				album := m.artistAlbums[albumIdx]
				logInfo("ui.navigation", fmt.Sprintf("opened album '%s' by '%s'", album.Name, m.currentPlName), "events.go")

				currID := ""
				if parts := strings.Split(m.currentPlURI, ":"); len(parts) >= 3 {
					currID = parts[2]
				}
				m.navHistory = append(m.navHistory, containerHistoryItem{
					ID:          currID,
					URI:         m.currentPlURI,
					Name:        m.currentPlName,
					CenterIndex: m.centerIndex,
					Tracks:      m.playlistTracks,
					Albums:      m.artistAlbums,
					Artists:     m.searchArtists,
				})

				m.currentPlURI = album.URI
				m.currentPlName = album.Name
				m.centerIndex = 0
				m.searchArtists = nil
				return m, m.fetchPlaylistTracksCmd(album.ID, album.URI, album.Name)
			} else if m.centerIndex >= len(m.playlistTracks)+len(m.artistAlbums) && m.centerIndex < len(m.playlistTracks)+len(m.artistAlbums)+len(m.searchArtists) {
				artIdx := m.centerIndex - len(m.playlistTracks) - len(m.artistAlbums)
				artist := m.searchArtists[artIdx]
				logInfo("ui.navigation", fmt.Sprintf("opened artist profile '%s'", artist.Name), "events.go")

				currID := ""
				if parts := strings.Split(m.currentPlURI, ":"); len(parts) >= 3 {
					currID = parts[2]
				}
				m.navHistory = append(m.navHistory, containerHistoryItem{
					ID:          currID,
					URI:         m.currentPlURI,
					Name:        m.currentPlName,
					CenterIndex: m.centerIndex,
					Tracks:      m.playlistTracks,
					Albums:      m.artistAlbums,
					Artists:     m.searchArtists,
				})

				m.currentPlURI = artist.URI
				m.currentPlName = artist.Name
				m.centerIndex = 0
				m.searchArtists = nil
				return m, m.fetchPlaylistTracksCmd(artist.ID, artist.URI, artist.Name)
			}

		case TabHistory:
			if m.centerIndex >= 0 && m.centerIndex < len(m.history) {
				idx := m.centerIndex
				tracks := m.history
				logInfo("ui.player", fmt.Sprintf("replaying history track '%s' by '%s'", tracks[idx].Name, tracks[idx].Artist), "events.go")
				return m, func() tea.Msg {
					if m.isLocalActive() && len(tracks) > 0 {
						if err := m.daemon.PlayURI(tracks[idx].URI, "", 0); err == nil {
							time.Sleep(150 * time.Millisecond)
							st, _ := m.client.GetPlaybackState(context.Background())
							return PlaybackMsg(st)
						}
					}
					logErr("ui.events", m.client.PlayTrackList(context.Background(), tracks, idx, ""), "events.go")
					time.Sleep(200 * time.Millisecond)
					st, _ := m.client.GetPlaybackState(context.Background())
					return PlaybackMsg(st)
				}
			}

		case TabLyrics:
			if m.lyricsCursor >= 0 && m.lyricsCursor < len(m.lyricsLines) {
				timeMs := m.lyricsLines[m.lyricsCursor].TimeMs
				m.lyricsManualScroll = false
				logInfo("ui.player", fmt.Sprintf("seeked lyrics to %02d:%02d (%dms)", timeMs/60000, (timeMs%60000)/1000, timeMs), "events.go")
				if m.playback != nil {
					m.playback.ProgressMs = timeMs
					m.client.SaveLastState(m.playback)
				}
				return m, func() tea.Msg {
					if m.isLocalActive() {
						logErr("ui.events", m.daemon.Seek(timeMs), "events.go")
					}
					logErr("ui.events", m.client.Seek(context.Background(), timeMs), "events.go")
					return nil
				}
			}
		}

	case PaneRight:
		if m.queueIndex >= 0 && m.queueIndex < len(m.queue) {
			track := m.queue[m.queueIndex]
			logInfo("ui.player", fmt.Sprintf("playing queued track '%s' by '%s'", track.Name, track.Artist), "events.go")
			return m, func() tea.Msg {
				if m.isLocalActive() {
					if err := m.daemon.PlayURI(track.URI, "", 0); err == nil {
						time.Sleep(150 * time.Millisecond)
						st, _ := m.client.GetPlaybackState(context.Background())
						return PlaybackMsg(st)
					}
				}
				logErr("ui.events", m.client.PlayTrack(context.Background(), track.URI, ""), "events.go")
				time.Sleep(200 * time.Millisecond)
				st, _ := m.client.GetPlaybackState(context.Background())
				return PlaybackMsg(st)
			}
		}

	case PanePlayer:
		isPlaying := false
		if m.playback != nil {
			isPlaying = m.playback.Playing
			m.playback.Playing = !isPlaying
			m.client.SaveLastState(m.playback)
		}
		trackURI := ""
		contextURI := ""
		progressMs := 0
		if m.playback != nil && m.playback.CurrentTrack != nil {
			trackURI = m.playback.CurrentTrack.URI
			contextURI = m.playback.ContextURI
			progressMs = m.playback.ProgressMs
		}
		return m, func() tea.Msg {
			if m.isLocalActive() {
				if !isPlaying && !m.localPlaybackReady && trackURI != "" {
					targetURI := contextURI
					if targetURI == "" {
						targetURI = trackURI
					}
					if err := m.daemon.PlayURI(targetURI, trackURI, progressMs); err == nil {
						time.Sleep(100 * time.Millisecond)
						st, _ := m.client.GetPlaybackState(context.Background())
						return PlaybackMsg(st)
					}
				}
				if err := m.daemon.PlayPause(); err == nil {
					time.Sleep(100 * time.Millisecond)
					st, _ := m.client.GetPlaybackState(context.Background())
					return PlaybackMsg(st)
				}
			}
			logErr("ui.events", m.client.PlayPause(context.Background(), isPlaying), "events.go")
			time.Sleep(150 * time.Millisecond)
			st, _ := m.client.GetPlaybackState(context.Background())
			return PlaybackMsg(st)
		}
	}

	return m, nil
}

func (m *AppModel) handleTab(delta int) {
	var panes []FocusedPane
	if m.showLeftSidebar {
		panes = append(panes, PaneNav)
	}
	panes = append(panes, PaneCenter)
	if m.showRightSidebar {
		panes = append(panes, PaneRight)
	}
	panes = append(panes, PanePlayer)

	curIdx := 0
	for i, p := range panes {
		if p == m.focused {
			curIdx = i
			break
		}
	}
	nextIdx := (curIdx + delta + len(panes)) % len(panes)
	m.focused = panes[nextIdx]
}

func (m *AppModel) cycleFilter(delta int) tea.Cmd {
	filters := []PlaylistFilter{FilterAll, FilterSpotify, FilterByYou, FilterAlbums, FilterArtists}
	curIdx := 0
	for i, f := range filters {
		if f == m.playlistFilter {
			curIdx = i
			break
		}
	}
	nextIdx := (curIdx + delta + len(filters)) % len(filters)
	m.playlistFilter = filters[nextIdx]
	m.navIndex = 0

	if m.playlistFilter == FilterAlbums && len(m.albums) == 0 {
		return m.fetchAlbumsCmd()
	} else if m.playlistFilter == FilterArtists && len(m.artists) == 0 {
		return m.fetchArtistsCmd()
	}
	return nil
}

func (m *AppModel) filteredPlaylists() []backend.Playlist {
	switch m.playlistFilter {
	case FilterAll:
		return m.playlists
	case FilterSpotify:
		var res []backend.Playlist
		for _, pl := range m.playlists {
			if strings.EqualFold(pl.OwnerID, "spotify") {
				res = append(res, pl)
			}
		}
		return res
	case FilterByYou:
		var res []backend.Playlist
		for _, pl := range m.playlists {
			if !strings.EqualFold(pl.OwnerID, "spotify") {
				res = append(res, pl)
			}
		}
		return res
	case FilterAlbums:
		return m.albums
	case FilterArtists:
		return m.artists
	default:
		return m.playlists
	}
}
