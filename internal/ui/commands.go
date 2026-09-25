// Async Bubble Tea commands - handles background Spotify API calls, lyric queries, and art downloads.
package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"spotumn/internal/auth"
	"spotumn/internal/backend"
	"spotumn/internal/config"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/oauth2"
)

// logErr is defined in events.go (same package)

func (m *AppModel) doTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *AppModel) fetchUserCmd() tea.Cmd {
	return func() tea.Msg {
		name, id := m.client.GetCurrentUser(context.Background())
		return UserMsg{DisplayName: name, UserID: id}
	}
}

func (m *AppModel) fetchDevicesCmd() tea.Cmd {
	return func() tea.Msg {
		devs, err := m.client.GetDevices(context.Background())
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return DevicesMsg(nil)
		}
		logInfo("ui.commands", fmt.Sprintf("fetched %d audio devices", len(devs)), "commands.go")
		return DevicesMsg(devs)
	}
}

func (m *AppModel) fetchPlaylistsCmd() tea.Cmd {
	return func() tea.Msg {
		playlists, err := m.client.GetAllPlaylists(context.Background())
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return ErrorMsg(err)
		}
		logInfo("ui.commands", fmt.Sprintf("fetched %d playlists", len(playlists)), "commands.go")
		return PlaylistsMsg(playlists)
	}
}

func (m *AppModel) fetchAlbumsCmd() tea.Cmd {
	return func() tea.Msg {
		albums, err := m.client.GetAlbums(context.Background())
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return AlbumsMsg(nil)
		}
		logInfo("ui.commands", fmt.Sprintf("fetched %d albums", len(albums)), "commands.go")
		return AlbumsMsg(albums)
	}
}

func (m *AppModel) fetchArtistsCmd() tea.Cmd {
	return func() tea.Msg {
		artists, err := m.client.GetArtists(context.Background())
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return ArtistsMsg(nil)
		}
		logInfo("ui.commands", fmt.Sprintf("fetched %d followed artists", len(artists)), "commands.go")
		return ArtistsMsg(artists)
	}
}

func (m *AppModel) fetchHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		recent, _ := m.client.GetRecentlyPlayed(context.Background())
		return HistoryMsg(recent)
	}
}

func (m *AppModel) fetchPlaybackCmd() tea.Cmd {
	return func() tea.Msg {
		st, err := m.client.GetPlaybackState(context.Background())
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return ErrorMsg(err)
		}
		return PlaybackMsg(st)
	}
}

func (m *AppModel) fetchQueueCmd() tea.Cmd {
	return func() tea.Msg {
		q, err := m.client.GetQueue(context.Background())
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return ErrorMsg(err)
		}
		logInfo("ui.commands", fmt.Sprintf("fetched queue (%d upcoming tracks)", len(q.Items)), "commands.go")
		return QueueMsg(q)
	}
}

func (m *AppModel) fetchPlaylistTracksCmd(plID, plURI, plName string) tea.Cmd {
	return func() tea.Msg {
		if plID == "" && strings.Contains(plURI, ":") {
			parts := strings.Split(plURI, ":")
			if len(parts) >= 3 {
				plID = parts[2]
			}
		}

		if strings.HasPrefix(plURI, "spotify:artist:") {
			tracks, err := m.client.GetArtistTracks(context.Background(), plID)
			if err != nil {
				logErr("ui.commands", err, "commands.go")
				return ErrorMsg(err)
			}
			albums, err := m.client.GetArtistAlbums(context.Background(), plID)
			if err != nil {
				logErr("ui.commands", err, "commands.go")
			}
			return TracksMsg{
				PlaylistURI:  plURI,
				PlaylistName: plName,
				Tracks:       tracks,
				Albums:       albums,
			}
		}

		if strings.HasPrefix(plURI, "spotify:album:") {
			tracks, err := m.client.GetAlbumTracks(context.Background(), plID)
			if err != nil {
				logErr("ui.commands", err, "commands.go")
				return ErrorMsg(err)
			}
			return TracksMsg{
				PlaylistURI:  plURI,
				PlaylistName: plName,
				Tracks:       tracks,
				Albums:       nil,
			}
		}

		tracks, err := m.client.GetContainerTracks(context.Background(), plID, plURI)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return ErrorMsg(err)
		}
		logInfo("ui.commands", fmt.Sprintf("fetched %d tracks for '%s'", len(tracks), plName), "commands.go")
		return TracksMsg{
			PlaylistURI:  plURI,
			PlaylistName: plName,
			Tracks:       tracks,
			Albums:       nil,
		}
	}
}

func (m *AppModel) fetchLyricsCmd(trackURI, trackName, artistName string, durSec int) tea.Cmd {
	return func() tea.Msg {
		lines, synced, err := m.lyrProv.FetchSyncedLyrics(trackName, artistName, durSec)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
		}
		logInfo("ui.commands", fmt.Sprintf("lyrics for '%s': synced=%v, lines=%d", trackName, synced, len(lines)), "commands.go")
		return LyricsMsg{TrackURI: trackURI, Lines: lines, Synced: synced, Duration: durSec}
	}
}

func (m *AppModel) fetchArtCmd(url string, w, h int, zen bool) tea.Cmd {
	return func() tea.Msg {
		ansiStr, diskPath, err := m.artRen.Render(url, w, h)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
		}
		if diskPath != "" {
			logInfo("ui.commands", fmt.Sprintf("album art rendered (%dx%d, file=%s)", w, h, filepath.Base(diskPath)), "commands.go")
		}
		return ArtMsg{
			ANSI:     ansiStr,
			DiskPath: diskPath,
			IsZen:    zen,
		}
	}
}

func (m *AppModel) searchCmd(query string) tea.Cmd {
	return func() tea.Msg {
		query = strings.TrimSpace(query)
		if query == "" {
			return SearchResultsMsg{}
		}
		tracks, albums, artists, err := m.client.Search(context.Background(), query)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
		}
		logInfo("ui.commands", fmt.Sprintf("search '%s' returned %d tracks, %d albums, %d artists", query, len(tracks), len(albums), len(artists)), "commands.go")
		return SearchResultsMsg{
			Tracks:  tracks,
			Albums:  albums,
			Artists: artists,
		}
	}
}

func (m *AppModel) addAccountCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := config.Get()
		authSvc := auth.NewAuthService(cfg)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		tok, err := authSvc.AuthorizeNew(ctx)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return accountActionMsg{err: err}
		}

		tempClient := backend.NewClient(ctx, oauth2.StaticTokenSource(tok))
		dispName, uid := tempClient.GetCurrentUser(ctx)
		if dispName == "" {
			dispName = "Spotify User"
		}

		accMgr := auth.NewAccountManager()
		idx, err := accMgr.AddAccount(uid, dispName, tok)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
			return accountActionMsg{err: err}
		}

		return accountActionMsg{
			addedIndex:  idx,
			displayName: dispName,
			userID:      uid,
			token:       tok,
		}
	}
}

func (m *AppModel) getRightSidebarArtGeometry() (w, h int) {
	innerW := m.width - 2
	if innerW < 40 {
		innerW = 40
	}
	rightW := innerW * 34 / 100
	if rightW < 34 {
		rightW = 34
	}
	if rightW > 50 {
		rightW = 50
	}
	w = rightW - 4
	if w < 16 {
		w = 16
	}
	h = w / 2
	if h < 8 {
		h = 8
	}
	return w, h
}

func (m *AppModel) getZenArtGeometry() (row, col, w, h int) {
	if m.zenView == ZenViewLyrics {
		return 0, 0, 0, 0
	}

	innerW := m.width - 2
	innerH := m.height - 2
	maxH := innerH - 8
	if maxH < 4 {
		return 0, 0, 0, 0
	}

	if m.zenView == ZenViewBoth {
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
		maxW := leftW - 4
		if maxW < 8 {
			return 0, 0, 0, 0
		}
		targetW := maxH * 2
		if targetW > maxW {
			targetW = maxW
		}
		if targetW%2 != 0 {
			targetW--
		}
		if targetW > 56 {
			targetW = 56
		}
		targetH := targetW / 2
		if targetH > maxH {
			targetH = maxH
		}

		artStackH := targetH + 7
		topPad := (innerH - artStackH) / 2
		if topPad < 0 {
			topPad = 0
		}
		row = 2 + topPad
		col = 2 + (leftW-targetW)/2
		return row, col, targetW, targetH
	}

	maxW := innerW - 8
	if maxW < 8 {
		return 0, 0, 0, 0
	}
	targetW := maxH * 2
	if targetW > maxW {
		targetW = maxW
	}
	if targetW%2 != 0 {
		targetW--
	}
	if targetW > 64 {
		targetW = 64
	}
	targetH := targetW / 2
	if targetH > maxH {
		targetH = maxH
	}

	artStackH := targetH + 7
	topPad := (innerH - artStackH) / 2
	if topPad < 0 {
		topPad = 0
	}
	row = 2 + topPad
	col = (m.width-targetW)/2 + 1
	return row, col, targetW, targetH
}

func (m *AppModel) defaultDeviceCmd() tea.Cmd {
	return nil
}

type RelatedAlbumsMsg struct {
	PlaylistURI string
	Albums      []backend.Playlist
}

func (m *AppModel) fetchRelatedAlbumsCmd(artistID, plURI, plID, plName string) tea.Cmd {
	return func() tea.Msg {
		allAlbums, err := m.client.GetArtistAlbums(context.Background(), artistID)
		if err != nil {
			logErr("ui.commands", err, "commands.go")
		}
		var moreAlbums []backend.Playlist
		for _, a := range allAlbums {
			if a.ID != plID && a.URI != plURI && !strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(plName)) {
				moreAlbums = append(moreAlbums, a)
			}
		}
		return RelatedAlbumsMsg{
			PlaylistURI: plURI,
			Albums:      moreAlbums,
		}
	}
}
