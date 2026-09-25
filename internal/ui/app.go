// Main Bubble Tea AppModel - handles top-level state orchestration, subscriptions, and layout flow.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"spotumn/internal/auth"
	"spotumn/internal/backend"
	"spotumn/internal/config"
	"spotumn/internal/media/art"
	"spotumn/internal/media/lyrics"
	"spotumn/internal/ui/theme"

	tea "charm.land/bubbletea/v2"
	"github.com/devgianlu/go-librespot/daemon"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

type AppModel struct {
	client  *backend.Client
	daemon  *backend.Daemon
	artRen  *art.Renderer
	lyrProv *lyrics.Provider

	width  int
	height int

	focused    FocusedPane
	currentTab CenterTab

	showLeftSidebar    bool
	showRightSidebar   bool
	autoShrinkSidebars bool
	zenMode            bool
	zenView            ZenViewMode
	showHelp           bool
	helpIndex          int
	helpEditing        bool
	keyManager         *KeyManager
	showSettings       bool
	settingsState      SettingsState
	showDevices        bool
	deviceScanning     bool

	devices     []spotify.PlayerDevice
	deviceIndex int

	username string
	userID   string

	navIndex     int
	centerIndex  int
	queueIndex   int
	lyricsCursor int

	searchFocused bool
	searchQuery   string

	playlists      []backend.Playlist
	albums         []backend.Playlist
	artists        []backend.Playlist
	pinnedURIs     map[string]bool
	pinnedOrder    []string
	playlistFilter PlaylistFilter
	currentPlURI   string
	currentPlName  string
	playlistTracks []backend.Track
	artistAlbums   []backend.Playlist
	searchArtists  []backend.Playlist
	navHistory     []containerHistoryItem

	history []backend.Track

	playback *backend.PlaybackState
	queue    []backend.Track

	lyricsLines          []lyrics.Line
	lyricsSynced         bool
	lyricsManualScroll   bool
	lyricsPointerMovedAt time.Time
	artANSI              string
	zenArtANSI           string
	lastArtURL           string
	lastDiskPath         string
	lastTrackURI         string

	tickCount int

	volumeTarget int
	seekTarget   int

	lastActionTime time.Time

	startupEvaluated   bool
	localPlaybackReady bool
	containerCache     map[string]containerCacheEntry
	containerKeys      []string
}

type containerCacheEntry struct {
	tracks   []backend.Track
	albums   []backend.Playlist
	cachedAt time.Time
}

type DaemonEventMsg struct {
	Event *daemon.ApiEvent
}

func (m *AppModel) listenDaemonEventsCmd() tea.Cmd {
	if m.daemon == nil {
		return nil
	}
	events := m.daemon.Events()
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return nil
		}
		return DaemonEventMsg{Event: ev}
	}
}

func NewAppModel(client *backend.Client, daemon *backend.Daemon, cfg ...*config.Config) *AppModel {
	artMode := "auto"
	var activeCfg *config.Config
	if len(cfg) > 0 && cfg[0] != nil {
		activeCfg = cfg[0]
	} else if c, err := config.Load(); err == nil {
		activeCfg = c
	} else {
		activeCfg = config.Get()
	}

	if activeCfg.ArtRenderer != "" {
		artMode = activeCfg.ArtRenderer
	}

	ApplyTheme(activeCfg.Theme, activeCfg.AppearanceMode != "light")

	pinnedOrder, pinnedURIs := loadPinned()

	m := &AppModel{
		client:             client,
		daemon:             daemon,
		artRen:             art.NewRenderer(artMode),
		lyrProv:            lyrics.NewProvider(),
		width:              100,
		height:             30,
		focused:            PaneCenter,
		currentTab:         TabTracks,
		showLeftSidebar:    true,
		showRightSidebar:   true,
		autoShrinkSidebars: activeCfg.AutoShrinkSidebars,
		navIndex:           0,
		centerIndex:        0,
		queueIndex:         0,
		lyricsCursor:       0,
		pinnedURIs:         pinnedURIs,
		pinnedOrder:        pinnedOrder,
		playlistFilter:     FilterAll,
		username:           "",
		keyManager:         NewKeyManager(),
		containerCache:     make(map[string]containerCacheEntry, 5),
		containerKeys:      make([]string, 0, 5),
	}

	// restore cached playback state so UI isn't blank while initial status polls
	lastState := client.LoadLastState()
	if lastState != nil {
		lastState.Playing = false
		m.playback = lastState
		if lastState.CurrentTrack != nil {
			m.lastArtURL = lastState.CurrentTrack.ArtURL
			m.lastTrackURI = lastState.CurrentTrack.URI
		}
	}

	return m
}

func (m *AppModel) isLocalActive() bool {
	if m.daemon == nil || !m.daemon.IsRunning() {
		return false
	}
	if m.client.SessionState() == backend.StateLocalActive {
		return true
	}
	if m.client.SessionState() == backend.StateRemoteActive {
		return false
	}
	if m.localPlaybackReady {
		return true
	}
	return false
}

func (m *AppModel) Init() tea.Cmd {
	logInfo("ui.app", "TUI initialized, loading initial state", "app.go")
	cmds := []tea.Cmd{
		tea.RequestBackgroundColor,
		m.doTick(),
		m.fetchUserCmd(),
		m.fetchPlaylistsCmd(),
		m.fetchAlbumsCmd(),
		m.fetchArtistsCmd(),
		m.fetchPlaybackCmd(),
		m.fetchQueueCmd(),
		m.fetchDevicesCmd(),
		m.listenDaemonEventsCmd(),
		m.fetchHistoryCmd(),
	}
	if m.lastArtURL != "" {
		w, h := m.getRightSidebarArtGeometry()
		cmds = append(cmds, m.fetchArtCmd(m.lastArtURL, w, h, false))
		cmds = append(cmds, m.fetchArtCmd(m.lastArtURL, 56, 26, true))
	}
	if m.playback != nil && m.playback.CurrentTrack != nil {
		m.lyricsLines = nil
		cmds = append(cmds, m.fetchLyricsCmd(m.playback.CurrentTrack.URI, m.playback.CurrentTrack.Name, m.playback.CurrentTrack.Artist, m.playback.DurationMs/1000))
	}
	return tea.Batch(cmds...)
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		SetTheme(msg.IsDark())
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		logInfo("ui.app", fmt.Sprintf("terminal resized to %dx%d", msg.Width, msg.Height), "app.go")
		return m, nil

	case TickMsg:
		m.tickCount++
		if m.playback != nil && m.playback.Playing {
			if !m.isLocalActive() || m.localPlaybackReady {
				m.playback.ProgressMs += 500
				if m.playback.DurationMs > 0 && m.playback.ProgressMs > m.playback.DurationMs {
					m.playback.ProgressMs = m.playback.DurationMs
				}
				if m.tickCount%4 == 0 {
					m.client.SaveLastState(m.playback)
				}
			}
		}

		if m.lyricsSynced && m.lyricsManualScroll && len(m.lyricsLines) > 0 && m.playback != nil {
			if time.Since(m.lyricsPointerMovedAt) >= 3*time.Second {
				m.lyricsManualScroll = false
				activeIdx := lyrics.FindActiveIndex(m.lyricsLines, m.playback.ProgressMs)
				if activeIdx >= 0 {
					m.lyricsCursor = activeIdx
				}
			}
		}

		if m.lyricsSynced && !m.lyricsManualScroll && len(m.lyricsLines) > 0 && m.playback != nil {
			activeIdx := lyrics.FindActiveIndex(m.lyricsLines, m.playback.ProgressMs)
			if activeIdx >= 0 {
				m.lyricsCursor = activeIdx
			}
		}

		var cmds []tea.Cmd
		cmds = append(cmds, m.doTick())

		if m.tickCount%7 == 0 {
			cmds = append(cmds, m.fetchPlaybackCmd())
		}
		if m.tickCount%24 == 0 {
			cmds = append(cmds, m.fetchQueueCmd())
		}

		return m, tea.Batch(cmds...)

	case DaemonEventMsg:
		if msg.Event != nil {
			switch msg.Event.Type {
			case daemon.ApiEventTypePlaying:
				m.localPlaybackReady = true
				if m.playback != nil {
					m.playback.Playing = true
					m.client.SaveLastState(m.playback)
				}
			case daemon.ApiEventTypePaused:
				if m.playback != nil {
					m.playback.Playing = false
					m.client.SaveLastState(m.playback)
				}
			case daemon.ApiEventTypeStopped:
				m.localPlaybackReady = false
				if m.playback != nil {
					m.playback.Playing = false
					m.client.SaveLastState(m.playback)
				}
			case daemon.ApiEventTypeSeek:
				if data, ok := msg.Event.Data.(daemon.ApiEventDataSeek); ok && m.playback != nil {
					m.playback.ProgressMs = data.Position
				}
			case daemon.ApiEventTypeVolume:
				if data, ok := msg.Event.Data.(daemon.ApiEventDataVolume); ok && m.playback != nil {
					if data.Max > 0 {
						m.playback.Volume = int(data.Value * 100 / data.Max)
					} else {
						m.playback.Volume = int(data.Value)
					}
				}
			case daemon.ApiEventTypeMetadata:
				var cmds []tea.Cmd
				cmds = append(cmds, m.listenDaemonEventsCmd())

				if meta, ok := msg.Event.Data.(daemon.ApiEventDataMetadata); ok && meta.Uri != "" {
					artURL := ""
					if meta.AlbumCoverUrl != nil {
						artURL = *meta.AlbumCoverUrl
					}
					artistStr := strings.Join(meta.ArtistNames, ", ")
					track := backend.Track{
						URI:        meta.Uri,
						Name:       meta.Name,
						Artist:     artistStr,
						Album:      meta.AlbumName,
						DurationMs: meta.Duration,
						ArtURL:     artURL,
					}
					if parts := strings.Split(meta.Uri, ":"); len(parts) >= 3 {
						track.ID = parts[2]
					}

					if m.playback == nil {
						m.playback = &backend.PlaybackState{
							Volume:     50,
							Playing:    true,
							DeviceName: "spotumn",
						}
					}
					m.playback.CurrentTrack = &track
					m.playback.DurationMs = meta.Duration
					m.playback.ProgressMs = int(meta.Position)
					m.playback.Playing = true
					m.playback.DeviceName = "spotumn"
					m.localPlaybackReady = true
					m.client.SetSessionState(backend.StateLocalActive)
					m.client.SaveLastState(m.playback)

					if meta.Uri != m.lastTrackURI {
						m.lastTrackURI = meta.Uri
						m.lyricsCursor = 0
						m.lyricsManualScroll = false
						m.lyricsLines = nil
						m.recordHistory(track)
						cmds = append(cmds, m.fetchQueueCmd())
						cmds = append(cmds, m.fetchLyricsCmd(meta.Uri, meta.Name, artistStr, meta.Duration/1000))
						if artURL != "" && artURL != m.lastArtURL {
							m.lastArtURL = artURL
							w, h := m.getRightSidebarArtGeometry()
							cmds = append(cmds, m.fetchArtCmd(artURL, w, h, false))
							cmds = append(cmds, m.fetchArtCmd(artURL, 56, 26, true))
						}
					}
				} else {
					cmds = append(cmds, m.fetchPlaybackCmd())
				}
				return m, tea.Batch(cmds...)
			}
		}
		return m, m.listenDaemonEventsCmd()

	case UserMsg:
		if msg.DisplayName != "" {
			m.username = msg.DisplayName
		} else if msg.UserID != "" {
			m.username = msg.UserID
		}
		m.userID = msg.UserID
		logInfo("ui.app", "logged in as: "+m.username, "app.go")
		return m, nil

	case PlaybackMsg:
		if !m.startupEvaluated {
			m.startupEvaluated = true
			localID := m.client.LocalDeviceID()
			hasRemoteActive := msg != nil && msg.DeviceID != "" && msg.DeviceID != localID && !strings.EqualFold(msg.DeviceName, "spotumn") && msg.Playing

			if hasRemoteActive {
				m.client.SetSessionState(backend.StateRemoteActive)
				m.localPlaybackReady = false
				m.playback = msg
				m.client.SaveLastState(msg)
				if msg.CurrentTrack != nil {
					logInfo("ui.player", fmt.Sprintf("remote playback active: '%s' by '%s' on %s (playing=%v)", msg.CurrentTrack.Name, msg.CurrentTrack.Artist, msg.DeviceName, msg.Playing), "app.go")
					m.lastTrackURI = msg.CurrentTrack.URI
					m.lyricsCursor = 0
					m.lyricsManualScroll = false
					m.lyricsLines = nil
					m.recordHistory(*msg.CurrentTrack)
					var cmds []tea.Cmd
					cmds = append(cmds, m.fetchQueueCmd())
					cmds = append(cmds, m.fetchLyricsCmd(msg.CurrentTrack.URI, msg.CurrentTrack.Name, msg.CurrentTrack.Artist, msg.CurrentTrack.DurationMs/1000))
					if msg.CurrentTrack.ArtURL != "" && msg.CurrentTrack.ArtURL != m.lastArtURL {
						m.lastArtURL = msg.CurrentTrack.ArtURL
						w, h := m.getRightSidebarArtGeometry()
						cmds = append(cmds, m.fetchArtCmd(msg.CurrentTrack.ArtURL, w, h, false))
						cmds = append(cmds, m.fetchArtCmd(msg.CurrentTrack.ArtURL, 56, 26, true))
					}
					return m, tea.Batch(cmds...)
				}
				return m, nil
			}

			m.client.SetSessionState(backend.StateLocalActive)
			cfg := config.Get()
			if cfg.AutoplayOnStartup && m.playback != nil && m.playback.CurrentTrack != nil {
				targetURI := m.playback.ContextURI
				if targetURI == "" {
					targetURI = m.playback.CurrentTrack.URI
				}
				trackURI := m.playback.CurrentTrack.URI
				posMs := m.playback.ProgressMs
				m.playback.Playing = true
				var cmds []tea.Cmd
				cmds = append(cmds, func() tea.Msg {
					logErr("ui.app", m.daemon.PlayURI(targetURI, trackURI, posMs), "app.go")
					return nil
				})
				return m, tea.Batch(cmds...)
			}
			if m.playback != nil {
				m.playback.Playing = false
			}
			return m, nil
		}

		if msg != nil {
			localID := m.client.LocalDeviceID()
			isRemote := msg.DeviceID != "" && msg.DeviceID != localID && !strings.EqualFold(msg.DeviceName, "spotumn")
			if isRemote && msg.Playing {
				m.client.SetSessionState(backend.StateRemoteActive)
				m.localPlaybackReady = false
				m.playback = msg
				m.client.SaveLastState(msg)
				if msg.CurrentTrack != nil && msg.CurrentTrack.URI != m.lastTrackURI {
					logInfo("ui.player", fmt.Sprintf("track changed: '%s' by '%s' on %s (playing=%v)", msg.CurrentTrack.Name, msg.CurrentTrack.Artist, msg.DeviceName, msg.Playing), "app.go")
					m.lastTrackURI = msg.CurrentTrack.URI
					m.lyricsCursor = 0
					m.lyricsManualScroll = false
					m.lyricsLines = nil
					m.recordHistory(*msg.CurrentTrack)
					var cmds []tea.Cmd
					cmds = append(cmds, m.fetchQueueCmd())
					cmds = append(cmds, m.fetchLyricsCmd(msg.CurrentTrack.URI, msg.CurrentTrack.Name, msg.CurrentTrack.Artist, msg.CurrentTrack.DurationMs/1000))
					if msg.CurrentTrack.ArtURL != m.lastArtURL && msg.CurrentTrack.ArtURL != "" {
						m.lastArtURL = msg.CurrentTrack.ArtURL
						w, h := m.getRightSidebarArtGeometry()
						cmds = append(cmds, m.fetchArtCmd(msg.CurrentTrack.ArtURL, w, h, false))
						cmds = append(cmds, m.fetchArtCmd(msg.CurrentTrack.ArtURL, 56, 26, true))
					}
					return m, tea.Batch(cmds...)
				}
				return m, nil
			}

			if m.isLocalActive() && m.playback != nil && m.playback.CurrentTrack != nil {
				if msg.CurrentTrack != nil && msg.CurrentTrack.URI == m.playback.CurrentTrack.URI {
					m.playback.Shuffle = msg.Shuffle
					m.playback.Repeat = msg.Repeat
					if msg.Volume > 0 {
						m.playback.Volume = msg.Volume
					}

					if !msg.Playing && time.Since(m.lastActionTime) > 1500*time.Millisecond {
						m.playback.Playing = false
						m.client.SaveLastState(m.playback)
					}
				}
				return m, nil
			}

			if msg.CurrentTrack != nil {
				if time.Since(m.lastActionTime) < 1500*time.Millisecond && m.playback != nil {
					msg.Playing = m.playback.Playing
					msg.Shuffle = m.playback.Shuffle
					msg.Repeat = m.playback.Repeat
				}
				m.playback = msg
				m.client.SaveLastState(msg)
				if msg.CurrentTrack.URI != m.lastTrackURI {
					m.lastTrackURI = msg.CurrentTrack.URI
					m.lyricsCursor = 0
					m.lyricsManualScroll = false
					m.lyricsLines = nil
					m.recordHistory(*msg.CurrentTrack)
					var cmds []tea.Cmd
					cmds = append(cmds, m.fetchQueueCmd())
					cmds = append(cmds, m.fetchLyricsCmd(msg.CurrentTrack.URI, msg.CurrentTrack.Name, msg.CurrentTrack.Artist, msg.CurrentTrack.DurationMs/1000))
					if msg.CurrentTrack.ArtURL != m.lastArtURL && msg.CurrentTrack.ArtURL != "" {
						m.lastArtURL = msg.CurrentTrack.ArtURL
						w, h := m.getRightSidebarArtGeometry()
						cmds = append(cmds, m.fetchArtCmd(msg.CurrentTrack.ArtURL, w, h, false))
						cmds = append(cmds, m.fetchArtCmd(msg.CurrentTrack.ArtURL, 56, 26, true))
					}
					return m, tea.Batch(cmds...)
				}
			} else if m.playback != nil && m.playback.CurrentTrack != nil {
				m.playback.Playing = false
			} else {
				m.playback = msg
			}
		}
		return m, nil

	case QueueMsg:
		if msg != nil {
			m.queue = msg.Items
		}
		return m, nil

	case PlaylistsMsg:
		m.playlists = sortPlaylistsWithPinned(msg, m.pinnedOrder)
		var cmds []tea.Cmd
		if len(m.playlists) > 0 && len(m.playlistTracks) == 0 {
			top := m.playlists[0]
			m.currentPlURI = top.URI
			m.currentPlName = top.Name
			cmds = append(cmds, m.fetchPlaylistTracksCmd(top.ID, top.URI, top.Name))
		}
		return m, tea.Batch(cmds...)

	case AlbumsMsg:
		m.albums = sortPlaylistsWithPinned(msg, m.pinnedOrder)
		var cmds []tea.Cmd
		if m.playlistFilter == FilterAlbums && len(m.albums) > 0 && len(m.playlistTracks) == 0 {
			top := m.albums[0]
			m.currentPlURI = top.URI
			m.currentPlName = top.Name
			cmds = append(cmds, m.fetchPlaylistTracksCmd(top.ID, top.URI, top.Name))
		}
		return m, tea.Batch(cmds...)

	case ArtistsMsg:
		m.artists = sortPlaylistsWithPinned(msg, m.pinnedOrder)
		var cmds []tea.Cmd
		if m.playlistFilter == FilterArtists && len(m.artists) > 0 && len(m.playlistTracks) == 0 {
			top := m.artists[0]
			m.currentPlURI = top.URI
			m.currentPlName = top.Name
			cmds = append(cmds, m.fetchPlaylistTracksCmd(top.ID, top.URI, top.Name))
		}
		return m, tea.Batch(cmds...)

	case HistoryMsg:
		if len(m.history) == 0 {
			m.history = msg
		} else {
			seen := make(map[string]bool)
			merged := make([]backend.Track, 0, len(m.history)+len(msg))
			for _, t := range m.history {
				if !seen[t.URI] && t.URI != "" {
					seen[t.URI] = true
					merged = append(merged, t)
				}
			}
			for _, t := range msg {
				if !seen[t.URI] && t.URI != "" {
					seen[t.URI] = true
					merged = append(merged, t)
				}
			}
			if len(merged) > 50 {
				merged = merged[:50]
			}
			m.history = merged
		}
		return m, nil

	case TracksMsg:
		m.setContainerCache(msg.PlaylistURI, msg.Tracks, msg.Albums)
		if msg.PlaylistURI != "" && msg.PlaylistURI != m.currentPlURI {
			return m, nil
		}
		m.currentPlURI = msg.PlaylistURI
		if msg.PlaylistName != "" {
			m.currentPlName = msg.PlaylistName
		}
		m.playlistTracks = msg.Tracks
		m.artistAlbums = msg.Albums
		if !strings.HasPrefix(msg.PlaylistURI, "search:") {
			m.searchArtists = nil
		}
		m.centerIndex = 0
		m.currentTab = TabTracks

		if strings.HasPrefix(msg.PlaylistURI, "spotify:album:") && len(msg.Tracks) > 0 && msg.Tracks[0].ArtistID != "" {
			plID := ""
			if parts := strings.Split(msg.PlaylistURI, ":"); len(parts) >= 3 {
				plID = parts[2]
			}
			return m, m.fetchRelatedAlbumsCmd(msg.Tracks[0].ArtistID, msg.PlaylistURI, plID, msg.PlaylistName)
		}
		return m, nil

	case RelatedAlbumsMsg:
		if m.currentPlURI == msg.PlaylistURI {
			m.artistAlbums = msg.Albums
		}
		return m, nil

	case openArtistResolvedMsg:
		if msg.Artist.ID != "" {
			m.currentPlURI = msg.Artist.URI
			m.currentPlName = msg.Artist.Name
			m.centerIndex = 0
			m.searchArtists = nil
			return m, m.fetchPlaylistTracksCmd(msg.Artist.ID, msg.Artist.URI, msg.Artist.Name)
		}
		return m, nil

	case LyricsMsg:
		if msg.TrackURI != "" && m.playback != nil && m.playback.CurrentTrack != nil && msg.TrackURI != m.playback.CurrentTrack.URI {
			return m, nil
		}
		m.lyricsLines = msg.Lines
		m.lyricsSynced = msg.Synced
		m.lyricsManualScroll = false
		if m.playback != nil && len(m.lyricsLines) > 0 {
			if m.lyricsSynced {
				activeIdx := lyrics.FindActiveIndex(m.lyricsLines, m.playback.ProgressMs)
				if activeIdx >= 0 {
					m.lyricsCursor = activeIdx
				} else {
					m.lyricsCursor = 0
				}
			} else {
				m.lyricsCursor = 0
			}
		} else {
			m.lyricsCursor = 0
		}
		return m, nil

	case ErrorMsg:
		logErr("ui.app", msg, "app.go")
		if !m.startupEvaluated {
			m.startupEvaluated = true
			m.client.SetSessionState(backend.StateLocalActive)
		}
		return m, nil

	case ArtMsg:
		if msg.IsZen {
			m.zenArtANSI = msg.ANSI
		} else {
			m.artANSI = msg.ANSI
		}
		if msg.DiskPath != "" {
			m.lastDiskPath = msg.DiskPath
		}
		return m, nil

	case DevicesMsg:
		m.deviceScanning = false
		if len(msg) > 0 || len(m.devices) == 0 {
			var oldID spotify.ID
			if m.deviceIndex >= 0 && m.deviceIndex < len(m.devices) {
				oldID = m.devices[m.deviceIndex].ID
			}
			m.devices = msg
			found := false
			if oldID != "" {
				for i, d := range m.devices {
					if d.ID == oldID {
						m.deviceIndex = i
						found = true
						break
					}
				}
			}
			if !found {
				m.deviceIndex = 0
				for i, d := range m.devices {
					if d.Active {
						m.deviceIndex = i
						break
					}
				}
			}
		}
		return m, nil

	case SearchResultsMsg:
		m.playlistTracks = msg.Tracks
		m.artistAlbums = msg.Albums
		m.searchArtists = msg.Artists
		m.currentPlURI = "search:" + m.searchQuery
		m.currentPlName = "Search: " + m.searchQuery
		m.centerIndex = 0
		m.currentTab = TabTracks
		m.focused = PaneCenter
		return m, nil

	case accountActionMsg:
		if msg.err != nil {
			logErr("ui.app", msg.err, "app.go")
			return m, nil
		}
		if msg.token != nil {
			if m.daemon != nil {
				logErr("ui.app", m.daemon.Restart(""), "app.go")
			}
			m.client = backend.NewClient(context.Background(), oauth2.StaticTokenSource(msg.token))
			if m.daemon != nil {
				m.client.SetLocalDeviceID(m.daemon.DeviceId())
			}
			m.username = msg.displayName
			m.settingsState.Accounts = auth.NewAccountManager().ListAccounts()
			m.settingsState.ActiveAccIdx = auth.NewAccountManager().ActiveIndex()
			return m, tea.Batch(
				m.fetchPlaylistsCmd(),
				m.fetchPlaybackCmd(),
				m.fetchQueueCmd(),
			)
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m *AppModel) View() tea.View {
	rendered := RenderFullUI(ViewParams{
		Width:              m.width,
		Height:             m.height,
		Focused:            m.focused,
		CurrentTab:         m.currentTab,
		ShowLeftSidebar:    m.showLeftSidebar,
		ShowRightSidebar:   m.showRightSidebar,
		AutoShrinkSidebars: m.autoShrinkSidebars,
		ZenMode:            m.zenMode,
		ZenView:            m.zenView,
		ShowHelp:           m.showHelp,
		HelpIndex:          m.helpIndex,
		HelpEditing:        m.helpEditing,
		KeybindItems:       m.getKeyManager().Items,
		ShowSettings:       m.showSettings,
		SettingsState:      m.settingsState,
		ShowDevices:        m.showDevices,
		DeviceScanning:     m.deviceScanning,
		Devices:            m.devices,
		DeviceIndex:        m.deviceIndex,
		Username:           m.username,
		NavIndex:           m.navIndex,
		CenterIndex:        m.centerIndex,
		QueueIndex:         m.queueIndex,
		LyricsCursor:       m.lyricsCursor,
		SearchFocused:      m.searchFocused,
		SearchQuery:        m.searchQuery,
		Playlists:          m.filteredPlaylists(),
		PinnedURIs:         m.pinnedURIs,
		PlaylistFilter:     m.playlistFilter,
		PlaylistTracks:     m.playlistTracks,
		ArtistAlbums:       m.artistAlbums,
		SearchArtists:      m.searchArtists,
		PlaylistName:       m.currentPlName,
		CurrentPlURI:       m.currentPlURI,
		History:            m.history,
		Playback:           m.playback,
		Queue:              m.queue,
		LyricsLines:        m.lyricsLines,
		ArtANSI:            m.artANSI,
		ZenArtANSI:         m.zenArtANSI,
	})

	v := tea.NewView(rendered)
	v.AltScreen = true
	v.WindowTitle = "spotumn"
	v.BackgroundColor = theme.CurrentTheme.Surface
	return v
}

func (m *AppModel) GetPlaybackState() *backend.PlaybackState {
	return m.playback
}

func (m *AppModel) recordHistory(t backend.Track) {
	if t.URI == "" {
		return
	}
	if len(m.history) > 0 && m.history[0].URI == t.URI {
		return
	}
	filtered := make([]backend.Track, 0, len(m.history)+1)
	filtered = append(filtered, t)
	for _, item := range m.history {
		if item.URI != t.URI {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) > 50 {
		filtered = filtered[:50]
	}
	m.history = filtered
}

func (m *AppModel) setContainerCache(uri string, tracks []backend.Track, albums []backend.Playlist) {
	if uri == "" || strings.HasPrefix(uri, "search:") {
		return
	}
	if m.containerCache == nil {
		m.containerCache = make(map[string]containerCacheEntry, 5)
	}
	if _, exists := m.containerCache[uri]; !exists {
		if len(m.containerKeys) >= 5 {
			oldest := m.containerKeys[0]
			m.containerKeys = m.containerKeys[1:]
			delete(m.containerCache, oldest)
		}
		m.containerKeys = append(m.containerKeys, uri)
	}
	m.containerCache[uri] = containerCacheEntry{
		tracks:   tracks,
		albums:   albums,
		cachedAt: time.Now(),
	}
}
