// Spotify Web API client - manages playback controls, tracks, playlists, search, devices, and library caching.
package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"spotumn/internal/config"

	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

type Track struct {
	ID         string
	URI        string
	Name       string
	Artist     string
	ArtistID   string
	Album      string
	DurationMs int
	ArtURL     string
}

type Playlist struct {
	ID         string
	URI        string
	Name       string
	OwnerID    string
	TrackCount int
	ImageURL   string
	Index      int
}

type QueueData struct {
	Current *Track
	Items   []Track
}

type PlaybackState struct {
	Playing      bool
	ProgressMs   int
	DurationMs   int
	Volume       int
	Shuffle      bool
	Repeat       string
	DeviceName   string
	DeviceID     string
	DeviceType   string
	CurrentTrack *Track
	ContextURI   string
}

type SessionState int

const (
	StateIdle SessionState = iota
	StateRemoteActive
	StateLocalActive
	StateTransferring
)

func (s SessionState) String() string {
	switch s {
	case StateRemoteActive:
		return "REMOTE_ACTIVE"
	case StateLocalActive:
		return "LOCAL_ACTIVE"
	case StateTransferring:
		return "TRANSFERRING"
	default:
		return "IDLE"
	}
}

type Client struct {
	spClient         *spotify.Client
	mu               sync.RWMutex
	lastState        *PlaybackState
	cachedDevices    []spotify.PlayerDevice
	devicesFetchedAt time.Time
	localDeviceID    string
	sessionState     SessionState
	activeDeviceID   string
	activeDeviceName string
}

func (c *Client) SessionState() SessionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionState
}

func (c *Client) SetSessionState(st SessionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionState = st
}

func (c *Client) ActiveDevice() (id, name string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeDeviceID, c.activeDeviceName
}

func (c *Client) SetActiveDevice(id, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activeDeviceID = id
	c.activeDeviceName = name
}

func (c *Client) SetLocalDeviceID(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.localDeviceID = id
}

func (c *Client) LocalDeviceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.localDeviceID
}

type rateLimitTransport struct {
	base http.RoundTripper
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		retrySec := 2
		if s := resp.Header.Get("Retry-After"); s != "" {
			if sec, err := strconv.Atoi(s); err == nil && sec > 0 {
				retrySec = sec
			}
		}
		if retrySec > 5 {
			retrySec = 5
		}
		if err := resp.Body.Close(); err != nil {
			if config.LogError != nil {
				config.LogError("backend.api", "rate limit body close: "+err.Error(), "api.go")
			}
		}
		time.Sleep(time.Duration(retrySec) * time.Second)
		return t.base.RoundTrip(req)
	}
	return resp, nil
}

func NewClient(ctx context.Context, ts oauth2.TokenSource) *Client {
	httpClient := oauth2.NewClient(ctx, ts)
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient.Transport = &rateLimitTransport{base: base}
	sp := spotify.New(httpClient)
	return &Client{
		spClient: sp,
	}
}

func extractTrack(t *spotify.FullTrack) Track {
	if t == nil {
		return Track{}
	}

	var artists []string
	artistID := ""
	for i, a := range t.Artists {
		artists = append(artists, a.Name)
		if i == 0 {
			artistID = string(a.ID)
		}
	}

	artURL := ""
	if len(t.Album.Images) > 0 {
		artURL = t.Album.Images[0].URL
	}

	return Track{
		ID:         string(t.ID),
		URI:        string(t.URI),
		Name:       t.Name,
		Artist:     strings.Join(artists, ", "),
		ArtistID:   artistID,
		Album:      t.Album.Name,
		DurationMs: int(t.Duration),
		ArtURL:     artURL,
	}
}

func (c *Client) GetPlaybackState(ctx context.Context) (*PlaybackState, error) {
	state, err := c.spClient.PlayerState(ctx)
	if err == nil && state != nil {
		ps := &PlaybackState{
			Playing:    state.Playing,
			ProgressMs: int(state.Progress),
			Volume:     int(state.Device.Volume),
			Shuffle:    state.ShuffleState,
			Repeat:     state.RepeatState,
			DeviceName: state.Device.Name,
			DeviceID:   string(state.Device.ID),
			DeviceType: state.Device.Type,
		}
		if state.Item != nil {
			track := extractTrack(state.Item)
			ps.CurrentTrack = &track
			ps.DurationMs = int(state.Item.Duration)
		}
		if state.PlaybackContext.URI != "" {
			ps.ContextURI = string(state.PlaybackContext.URI)
		}

		c.mu.Lock()
		c.activeDeviceID = string(state.Device.ID)
		c.activeDeviceName = state.Device.Name
		localID := c.localDeviceID
		if c.sessionState != StateTransferring {
			if (localID != "" && string(state.Device.ID) == localID) || strings.EqualFold(state.Device.Name, "spotumn") {
				c.sessionState = StateLocalActive
			} else if state.Playing || state.Device.ID != "" {
				c.sessionState = StateRemoteActive
			} else {
				c.sessionState = StateIdle
			}
		}
		c.mu.Unlock()

		return ps, nil
	}

	if cp, cpErr := c.spClient.PlayerCurrentlyPlaying(ctx); cpErr == nil && cp != nil && cp.Item != nil {
		track := extractTrack(cp.Item)
		ctxURI := ""
		if cp.PlaybackContext.URI != "" {
			ctxURI = string(cp.PlaybackContext.URI)
		}

		c.mu.Lock()
		if c.sessionState != StateTransferring && c.sessionState != StateLocalActive {
			c.sessionState = StateRemoteActive
		}
		c.mu.Unlock()

		return &PlaybackState{
			Playing:      cp.Playing,
			ProgressMs:   int(cp.Progress),
			DurationMs:   int(cp.Item.Duration),
			Volume:       50,
			CurrentTrack: &track,
			ContextURI:   ctxURI,
		}, nil
	}

	c.mu.Lock()
	if c.sessionState != StateTransferring && c.sessionState != StateLocalActive {
		c.sessionState = StateIdle
		c.activeDeviceID = ""
		c.activeDeviceName = ""
	}
	c.mu.Unlock()

	return &PlaybackState{Volume: 50}, nil
}

func (c *Client) GetQueue(ctx context.Context) (*QueueData, error) {
	res := &QueueData{}
	q, err := c.spClient.GetQueue(ctx)
	if err != nil || q == nil {
		return res, nil
	}

	if q.CurrentlyPlaying.ID != "" {
		cur := extractTrack(&q.CurrentlyPlaying)
		res.Current = &cur
	}

	for _, item := range q.Items {
		if item.ID == "" {
			continue
		}
		t := extractTrack(&item)
		res.Items = append(res.Items, t)
	}

	return res, nil
}

func (c *Client) GetRecommendationsOrTopTracks(ctx context.Context) ([]Track, error) {
	var tracks []Track

	top, err := c.spClient.CurrentUsersTopTracks(ctx, spotify.Limit(30))
	if err == nil && top != nil && len(top.Tracks) > 0 {
		for _, t := range top.Tracks {
			tracks = append(tracks, extractTrack(&t))
		}
		return tracks, nil
	}

	saved, err := c.spClient.CurrentUsersTracks(ctx, spotify.Limit(30))
	if err == nil && saved != nil && len(saved.Tracks) > 0 {
		for _, st := range saved.Tracks {
			tracks = append(tracks, extractTrack(&st.FullTrack))
		}
		return tracks, nil
	}

	searchRes, err := c.spClient.Search(ctx, "top hits", spotify.SearchTypeTrack, spotify.Limit(25))
	if err == nil && searchRes.Tracks != nil {
		for _, t := range searchRes.Tracks.Tracks {
			tracks = append(tracks, extractTrack(&t))
		}
	}

	return tracks, nil
}

func extractSimpleTrack(t *spotify.SimpleTrack) Track {
	if t == nil {
		return Track{}
	}
	var artists []string
	artistID := ""
	for i, a := range t.Artists {
		artists = append(artists, a.Name)
		if i == 0 {
			artistID = string(a.ID)
		}
	}
	return Track{
		ID:         string(t.ID),
		URI:        string(t.URI),
		Name:       t.Name,
		Artist:     strings.Join(artists, ", "),
		ArtistID:   artistID,
		DurationMs: int(t.Duration),
	}
}

func extractTrackID(uriOrID string) string {
	if strings.HasPrefix(uriOrID, "spotify:track:") {
		return strings.TrimPrefix(uriOrID, "spotify:track:")
	}
	return uriOrID
}

func (c *Client) GetAllPlaylists(ctx context.Context) ([]Playlist, error) {
	var playlists []Playlist
	limit := 50
	offset := 0

	for {
		page, err := c.spClient.CurrentUsersPlaylists(ctx, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return nil, err
		}

		for _, p := range page.Playlists {
			img := ""
			if len(p.Images) > 0 {
				img = p.Images[0].URL
			}
			playlists = append(playlists, Playlist{
				ID:         string(p.ID),
				URI:        string(p.URI),
				Name:       p.Name,
				OwnerID:    string(p.Owner.ID),
				TrackCount: int(p.Tracks.Total),
				ImageURL:   img,
				Index:      len(playlists),
			})
		}

		offset += len(page.Playlists)
		if offset >= int(page.Total) || len(page.Playlists) == 0 {
			break
		}
	}

	return playlists, nil
}

func (c *Client) GetPlaylistTracks(ctx context.Context, playlistID string) ([]Track, error) {
	var tracks []Track
	limit := 100
	offset := 0

	for {
		page, err := c.spClient.GetPlaylistItems(ctx, spotify.ID(playlistID), spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			break
		}

		for _, item := range page.Items {
			if item.Track.Track != nil && item.Track.Track.ID != "" {
				tracks = append(tracks, extractTrack(item.Track.Track))
			}
		}

		offset += len(page.Items)
		if offset >= int(page.Total) || len(page.Items) == 0 {
			break
		}
	}

	if len(tracks) == 0 {
		if pl, err := c.spClient.GetPlaylist(ctx, spotify.ID(playlistID)); err == nil && pl != nil {
			for _, item := range pl.Tracks.Tracks {
				if item.Track.ID != "" {
					tracks = append(tracks, extractTrack(&item.Track))
				}
			}
		}
	}

	return tracks, nil
}

func (c *Client) GetAlbums(ctx context.Context) ([]Playlist, error) {
	var albums []Playlist
	limit := 50
	offset := 0

	for {
		page, err := c.spClient.CurrentUsersAlbums(ctx, spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			break
		}

		for _, a := range page.Albums {
			img := ""
			if len(a.Images) > 0 {
				img = a.Images[0].URL
			}
			artistName := ""
			if len(a.Artists) > 0 {
				artistName = a.Artists[0].Name
			}
			albums = append(albums, Playlist{
				ID:         string(a.ID),
				URI:        string(a.URI),
				Name:       a.Name,
				OwnerID:    artistName,
				TrackCount: int(a.Tracks.Total),
				ImageURL:   img,
				Index:      len(albums),
			})
		}

		offset += len(page.Albums)
		if offset >= int(page.Total) || len(page.Albums) == 0 {
			break
		}
	}

	return albums, nil
}

func (c *Client) GetArtists(ctx context.Context) ([]Playlist, error) {
	var artists []Playlist
	after := ""

	for {
		var opts []spotify.RequestOption
		opts = append(opts, spotify.Limit(50))
		if after != "" {
			opts = append(opts, spotify.After(after))
		}
		cursor, err := c.spClient.CurrentUsersFollowedArtists(ctx, opts...)
		if err != nil || cursor == nil || len(cursor.Artists) == 0 {
			break
		}
		for _, a := range cursor.Artists {
			img := ""
			if len(a.Images) > 0 {
				img = a.Images[0].URL
			}
			artists = append(artists, Playlist{
				ID:         string(a.ID),
				URI:        string(a.URI),
				Name:       a.Name,
				OwnerID:    "Artist",
				TrackCount: int(a.Popularity),
				ImageURL:   img,
				Index:      len(artists),
			})
		}
		if cursor.Cursor.After == "" || len(cursor.Artists) < 50 {
			break
		}
		after = cursor.Cursor.After
	}

	if len(artists) > 0 {
		return artists, nil
	}

	top, err := c.spClient.CurrentUsersTopArtists(ctx, spotify.Limit(50))
	if err == nil && top != nil {
		for _, a := range top.Artists {
			img := ""
			if len(a.Images) > 0 {
				img = a.Images[0].URL
			}
			artists = append(artists, Playlist{
				ID:         string(a.ID),
				URI:        string(a.URI),
				Name:       a.Name,
				OwnerID:    "Artist",
				TrackCount: int(a.Popularity),
				ImageURL:   img,
				Index:      len(artists),
			})
		}
	}

	return artists, nil
}

func (c *Client) GetAlbumTracks(ctx context.Context, albumID string) ([]Track, error) {
	album, err := c.spClient.GetAlbum(ctx, spotify.ID(albumID))
	if err == nil && album != nil {
		artURL := ""
		if len(album.Images) > 0 {
			artURL = album.Images[0].URL
		}
		var tracks []Track
		for _, t := range album.Tracks.Tracks {
			artistNames := make([]string, len(t.Artists))
			artistID := ""
			for i, a := range t.Artists {
				artistNames[i] = a.Name
				if i == 0 {
					artistID = string(a.ID)
				}
			}
			tracks = append(tracks, Track{
				ID:         string(t.ID),
				URI:        string(t.URI),
				Name:       t.Name,
				Artist:     strings.Join(artistNames, ", "),
				ArtistID:   artistID,
				Album:      album.Name,
				DurationMs: int(t.Duration),
				ArtURL:     artURL,
			})
		}
		return tracks, nil
	}

	page, err := c.spClient.GetAlbumTracks(ctx, spotify.ID(albumID), spotify.Limit(50))
	if err != nil {
		return nil, err
	}
	var tracks []Track
	for _, t := range page.Tracks {
		artistNames := make([]string, len(t.Artists))
		artistID := ""
		for i, a := range t.Artists {
			artistNames[i] = a.Name
			if i == 0 {
				artistID = string(a.ID)
			}
		}
		tracks = append(tracks, Track{
			ID:         string(t.ID),
			URI:        string(t.URI),
			Name:       t.Name,
			Artist:     strings.Join(artistNames, ", "),
			ArtistID:   artistID,
			DurationMs: int(t.Duration),
		})
	}
	return tracks, nil
}

func (c *Client) GetArtistTracks(ctx context.Context, artistID string) ([]Track, error) {
	fts, err := c.spClient.GetArtistsTopTracks(ctx, spotify.ID(artistID), "from_token")
	if err != nil || len(fts) == 0 {
		fts, err = c.spClient.GetArtistsTopTracks(ctx, spotify.ID(artistID), "US")
	}
	if err != nil {
		return nil, err
	}
	var tracks []Track
	for _, t := range fts {
		tracks = append(tracks, extractTrack(&t))
	}
	return tracks, nil
}

func (c *Client) GetArtistAlbums(ctx context.Context, artistID string) ([]Playlist, error) {
	if c.spClient == nil {
		return nil, nil
	}

	types := []spotify.AlbumType{spotify.AlbumTypeAlbum, spotify.AlbumTypeSingle}
	page, err := c.spClient.GetArtistAlbums(ctx, spotify.ID(artistID), types, spotify.Limit(50), spotify.Market("from_token"))
	if err != nil || page == nil || len(page.Albums) == 0 {
		page, err = c.spClient.GetArtistAlbums(ctx, spotify.ID(artistID), types, spotify.Limit(50), spotify.Market("US"))
	}
	if err != nil || page == nil {
		return nil, err
	}

	var rawAlbums []spotify.SimpleAlbum
	rawAlbums = append(rawAlbums, page.Albums...)

	for pageCount := 0; pageCount < 2; pageCount++ {
		err := c.spClient.NextPage(ctx, page)
		if err != nil || len(page.Albums) == 0 {
			break
		}
		rawAlbums = append(rawAlbums, page.Albums...)
	}

	var albums []Playlist
	seen := make(map[string]bool)
	for _, a := range rawAlbums {
		cleanName := strings.ToLower(strings.TrimSpace(a.Name))
		if seen[cleanName] {
			continue
		}
		seen[cleanName] = true

		img := ""
		if len(a.Images) > 0 {
			img = a.Images[0].URL
		}
		year := a.ReleaseDate
		if len(year) > 4 {
			year = year[:4]
		}
		typeLabel := "Album"
		grp := a.AlbumGroup
		if grp == "" {
			grp = a.AlbumType
		}
		if grp != "" {
			typeLabel = strings.ToUpper(grp[:1]) + strings.ToLower(grp[1:])
		}
		desc := typeLabel
		if year != "" {
			desc = fmt.Sprintf("%s • %s", typeLabel, year)
		}

		albums = append(albums, Playlist{
			ID:         string(a.ID),
			URI:        string(a.URI),
			Name:       a.Name,
			OwnerID:    desc,
			TrackCount: int(a.TotalTracks),
			ImageURL:   img,
		})
	}
	return albums, nil
}

func (c *Client) GetContainerTracks(ctx context.Context, id, uri string) ([]Track, error) {
	if strings.HasPrefix(uri, "spotify:album:") {
		return c.GetAlbumTracks(ctx, id)
	}
	if strings.HasPrefix(uri, "spotify:artist:") {
		return c.GetArtistTracks(ctx, id)
	}
	return c.GetPlaylistTracks(ctx, id)
}

func (c *Client) GetArtistByName(ctx context.Context, name string) (*Playlist, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.spClient == nil || strings.TrimSpace(name) == "" {
		return nil, nil
	}
	res, err := c.spClient.Search(ctx, name, spotify.SearchTypeArtist, spotify.Limit(1))
	if err != nil || res == nil || res.Artists == nil || len(res.Artists.Artists) == 0 {
		return nil, err
	}
	art := res.Artists.Artists[0]
	imgURL := ""
	if len(art.Images) > 0 {
		imgURL = art.Images[0].URL
	}
	return &Playlist{
		ID:         string(art.ID),
		URI:        string(art.URI),
		Name:       art.Name,
		ImageURL:   imgURL,
		TrackCount: int(art.Popularity),
	}, nil
}

func (c *Client) Search(ctx context.Context, query string) (tracks []Track, albums []Playlist, artists []Playlist, err error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil, nil, nil
	}

	res, err := c.spClient.Search(ctx, query, spotify.SearchTypeTrack|spotify.SearchTypeAlbum|spotify.SearchTypeArtist, spotify.Limit(50))
	if err != nil {
		return nil, nil, nil, err
	}

	if res.Tracks != nil {
		for _, t := range res.Tracks.Tracks {
			tracks = append(tracks, extractTrack(&t))
		}
	}

	if res.Albums != nil {
		for _, a := range res.Albums.Albums {
			img := ""
			if len(a.Images) > 0 {
				img = a.Images[0].URL
			}
			year := a.ReleaseDate
			if len(year) > 4 {
				year = year[:4]
			}
			typeLabel := "Album"
			if a.AlbumType != "" {
				typeLabel = strings.ToUpper(a.AlbumType[:1]) + strings.ToLower(a.AlbumType[1:])
			}
			desc := typeLabel
			if year != "" {
				desc = fmt.Sprintf("%s • %s", typeLabel, year)
			}
			albums = append(albums, Playlist{
				ID:         string(a.ID),
				URI:        string(a.URI),
				Name:       a.Name,
				OwnerID:    desc,
				TrackCount: int(a.TotalTracks),
				ImageURL:   img,
			})
		}
	}

	if res.Artists != nil {
		for _, a := range res.Artists.Artists {
			img := ""
			if len(a.Images) > 0 {
				img = a.Images[0].URL
			}
			artists = append(artists, Playlist{
				ID:         string(a.ID),
				URI:        string(a.URI),
				Name:       a.Name,
				OwnerID:    "Artist",
				TrackCount: int(a.Popularity),
				ImageURL:   img,
			})
		}
	}

	return tracks, albums, artists, nil
}

func (c *Client) PlayTrackList(ctx context.Context, tracks []Track, startIndex int, contextURI string) error {
	opts := &spotify.PlayOptions{}

	if contextURI != "" && !strings.HasPrefix(contextURI, "search:") {
		cURI := spotify.URI(contextURI)
		opts.PlaybackContext = &cURI
		if startIndex >= 0 && startIndex < len(tracks) {
			tURI := spotify.URI(tracks[startIndex].URI)
			opts.PlaybackOffset = &spotify.PlaybackOffset{URI: tURI}
		}
	} else if len(tracks) > 0 && startIndex >= 0 && startIndex < len(tracks) {
		var uris []spotify.URI
		endIndex := len(tracks)
		if endIndex-startIndex > 50 {
			endIndex = startIndex + 50
		}
		for i := startIndex; i < endIndex; i++ {
			uris = append(uris, spotify.URI(tracks[i].URI))
		}
		opts.URIs = uris
	}

	c.mu.RLock()
	last := c.lastState
	c.mu.RUnlock()
	if last == nil || !last.Playing {
		if devID := c.getTargetDeviceID(ctx); devID != nil {
			opts.DeviceID = devID
		}
	}

	err := c.spClient.PlayOpt(ctx, opts)
	if err != nil && (strings.Contains(err.Error(), "No active device") || strings.Contains(err.Error(), "404")) {
		if opts.DeviceID == nil {
			if devID := c.getTargetDeviceID(ctx); devID != nil {
				opts.DeviceID = devID
				return c.spClient.PlayOpt(ctx, opts)
			}
		}
	}
	return err
}

func (c *Client) PlayTrack(ctx context.Context, trackURI, contextURI string) error {
	opts := &spotify.PlayOptions{}

	if contextURI != "" && !strings.HasPrefix(contextURI, "search:") {
		cURI := spotify.URI(contextURI)
		opts.PlaybackContext = &cURI
		if trackURI != "" {
			tURI := spotify.URI(trackURI)
			opts.PlaybackOffset = &spotify.PlaybackOffset{URI: tURI}
		}
	} else if trackURI != "" {
		opts.URIs = []spotify.URI{spotify.URI(trackURI)}
	}

	c.mu.RLock()
	last := c.lastState
	c.mu.RUnlock()
	if last == nil || !last.Playing {
		if devID := c.getTargetDeviceID(ctx); devID != nil {
			opts.DeviceID = devID
		}
	}

	err := c.spClient.PlayOpt(ctx, opts)
	if err != nil && (strings.Contains(err.Error(), "No active device") || strings.Contains(err.Error(), "404")) {
		if opts.DeviceID == nil {
			if devID := c.getTargetDeviceID(ctx); devID != nil {
				opts.DeviceID = devID
				return c.spClient.PlayOpt(ctx, opts)
			}
		}
	}
	return err
}

func (c *Client) findSpotumnDeviceID(ctx context.Context) spotify.ID {
	c.mu.RLock()
	localID := c.localDeviceID
	c.mu.RUnlock()
	if localID != "" {
		return spotify.ID(localID)
	}

	devices, err := c.spClient.PlayerDevices(ctx)
	if err != nil {
		return ""
	}
	for _, d := range devices {
		if strings.EqualFold(d.Name, "spotumn") {
			return d.ID
		}
	}
	return ""
}

func (c *Client) SetCachedDevices(devices []spotify.PlayerDevice) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedDevices = devices
}

func (c *Client) GetTargetDeviceID(ctx context.Context) *spotify.ID {
	return c.getTargetDeviceID(ctx)
}

func (c *Client) getTargetDeviceID(ctx context.Context) *spotify.ID {
	c.mu.RLock()
	activeID := c.activeDeviceID
	localID := c.localDeviceID
	c.mu.RUnlock()

	if activeID != "" {
		id := spotify.ID(activeID)
		return &id
	}

	devices, err := c.GetDevices(ctx)
	if err == nil && len(devices) > 0 {
		for _, d := range devices {
			if d.Active {
				return &d.ID
			}
		}
		for _, d := range devices {
			if strings.EqualFold(d.Name, "spotumn") || (localID != "" && string(d.ID) == localID) {
				return &d.ID
			}
		}
		return &devices[0].ID
	}

	if localID != "" {
		id := spotify.ID(localID)
		return &id
	}
	return nil
}

func SortDevicesWithSpotumnFirst(devices []spotify.PlayerDevice) []spotify.PlayerDevice {
	if len(devices) <= 1 {
		return devices
	}
	for i, d := range devices {
		if strings.EqualFold(d.Name, "spotumn") {
			if i > 0 {
				devCopy := make([]spotify.PlayerDevice, len(devices))
				copy(devCopy, devices)
				spotumnDev := devCopy[i]
				copy(devCopy[1:i+1], devCopy[0:i])
				devCopy[0] = spotumnDev
				return devCopy
			}
			break
		}
	}
	return devices
}

func (c *Client) GetDevices(ctx context.Context) ([]spotify.PlayerDevice, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	c.mu.RLock()
	cached := c.cachedDevices
	fetchedAt := c.devicesFetchedAt
	c.mu.RUnlock()

	if len(cached) > 0 && (c.spClient == nil || time.Since(fetchedAt) < 4*time.Second) {
		return cached, nil
	}
	if c.spClient == nil {
		return nil, nil
	}

	callCtx, cancel := context.WithTimeout(ctx, 2500*time.Millisecond)
	defer cancel()

	devices, err := c.spClient.PlayerDevices(callCtx)
	c.mu.RLock()
	localID := c.localDeviceID
	last := c.lastState
	sessionSt := c.sessionState
	c.mu.RUnlock()

	var result []spotify.PlayerDevice
	if err == nil && len(devices) > 0 {
		result = make([]spotify.PlayerDevice, len(devices))
		copy(result, devices)
	}

	if localID != "" {
		hasLocal := false
		for i := range result {
			if strings.EqualFold(result[i].Name, "spotumn") || string(result[i].ID) == localID {
				hasLocal = true
				break
			}
		}
		if !hasLocal {
			isLocalActive := (sessionSt == StateLocalActive)
			if last != nil && (last.DeviceID == localID || strings.EqualFold(last.DeviceName, "spotumn")) {
				isLocalActive = true
			}
			result = append([]spotify.PlayerDevice{{
				ID:     spotify.ID(localID),
				Name:   "spotumn",
				Type:   "Computer",
				Volume: 50,
				Active: isLocalActive,
			}}, result...)
		}
	}

	if len(result) == 0 {
		if len(cached) > 0 {
			return cached, nil
		}
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	sorted := SortDevicesWithSpotumnFirst(result)
	c.mu.Lock()
	c.cachedDevices = sorted
	c.devicesFetchedAt = time.Now()
	c.mu.Unlock()
	return sorted, nil
}

func (c *Client) TransferPlayback(ctx context.Context, deviceID spotify.ID) error {
	c.mu.Lock()
	if c.lastState != nil {
		c.lastState.DeviceID = string(deviceID)
	}
	c.mu.Unlock()

	return c.spClient.TransferPlayback(ctx, deviceID, true)
}

func (c *Client) ToggleDevice(ctx context.Context) (string, error) {
	devices, err := c.GetDevices(ctx)
	if err != nil || len(devices) == 0 {
		return "", err
	}

	var spotumnDev *spotify.PlayerDevice
	var otherDev *spotify.PlayerDevice
	for i := range devices {
		if strings.EqualFold(devices[i].Name, "spotumn") {
			spotumnDev = &devices[i]
		} else if otherDev == nil {
			otherDev = &devices[i]
		}
	}

	if spotumnDev != nil && spotumnDev.Active && otherDev != nil {
		if err := c.TransferPlayback(ctx, otherDev.ID); err != nil {
			if config.LogError != nil {
				config.LogError("backend.api", "transfer to other device: "+err.Error(), "api.go")
			}
		}
		return otherDev.Name, nil
	}

	if spotumnDev != nil {
		if err := c.TransferPlayback(ctx, spotumnDev.ID); err != nil {
			if config.LogError != nil {
				config.LogError("backend.api", "transfer to spotumn: "+err.Error(), "api.go")
			}
		}
		return "spotumn", nil
	}

	return "", nil
}

func (c *Client) Play(ctx context.Context) error {
	err := c.spClient.Play(ctx)
	if err != nil && (strings.Contains(err.Error(), "No active device") || strings.Contains(err.Error(), "404")) {
		if devID := c.getTargetDeviceID(ctx); devID != nil {
			return c.spClient.PlayOpt(ctx, &spotify.PlayOptions{DeviceID: devID})
		}
	}
	return err
}

func (c *Client) Pause(ctx context.Context) error {
	return c.spClient.Pause(ctx)
}

func (c *Client) PlayPause(ctx context.Context, currentlyPlaying bool) error {
	if currentlyPlaying {
		return c.Pause(ctx)
	}
	return c.Play(ctx)
}

func (c *Client) Next(ctx context.Context) error {
	return c.spClient.Next(ctx)
}

func (c *Client) Previous(ctx context.Context) error {
	return c.spClient.Previous(ctx)
}

func (c *Client) Seek(ctx context.Context, positionMs int) error {
	return c.spClient.Seek(ctx, positionMs)
}

func (c *Client) SetVolume(ctx context.Context, percent int) error {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return c.spClient.Volume(ctx, percent)
}

func (c *Client) ToggleShuffle(ctx context.Context, current bool) error {
	return c.spClient.Shuffle(ctx, !current)
}

func (c *Client) CycleRepeat(ctx context.Context, current string) error {
	var next string
	switch current {
	case "off":
		next = "context"
	case "context":
		next = "track"
	default:
		next = "off"
	}
	return c.spClient.Repeat(ctx, next)
}

func (c *Client) EnsureActiveDevice(ctx context.Context) error {
	devices, err := c.GetDevices(ctx)
	if err != nil {
		return err
	}

	for _, d := range devices {
		if d.Active {
			return nil
		}
	}

	for _, d := range devices {
		if strings.EqualFold(d.Name, "spotumn") {
			return c.spClient.TransferPlayback(ctx, d.ID, false)
		}
	}

	if len(devices) > 0 {
		return c.spClient.TransferPlayback(ctx, devices[0].ID, false)
	}

	return nil
}

func (c *Client) DefaultToSpotumnDevice(ctx context.Context) error {
	state, err := c.spClient.PlayerState(ctx)
	if err == nil && state != nil && state.Device.ID != "" {
		if state.Playing || state.Device.Active {
			return nil
		}
	}

	spotumnID := c.findSpotumnDeviceID(ctx)
	if spotumnID != "" {
		return c.spClient.TransferPlayback(ctx, spotumnID, false)
	}
	return nil
}

func (c *Client) GetCurrentUser(ctx context.Context) (displayName, userID string) {
	user, err := c.spClient.CurrentUser(ctx)
	if err != nil || user == nil {
		return "spotumn", ""
	}
	name := user.DisplayName
	if name == "" {
		name = user.ID
	}
	return name, user.ID
}

func (c *Client) GetTasteRecommendations(ctx context.Context) ([]Track, error) {
	var seedIDs []spotify.ID
	top, err := c.spClient.CurrentUsersTopTracks(ctx, spotify.Limit(5))
	if err == nil && top != nil {
		for _, t := range top.Tracks {
			seedIDs = append(seedIDs, t.ID)
		}
	}

	if len(seedIDs) > 0 {
		seeds := spotify.Seeds{Tracks: seedIDs}
		recs, err := c.spClient.GetRecommendations(ctx, seeds, nil, spotify.Limit(24))
		if err == nil && recs != nil && len(recs.Tracks) > 0 {
			var tracks []Track
			for _, st := range recs.Tracks {
				tracks = append(tracks, extractSimpleTrack(&st))
			}
			return tracks, nil
		}
	}

	return c.GetRecommendationsOrTopTracks(ctx)
}

func (c *Client) GetRecentlyPlayed(ctx context.Context) ([]Track, error) {
	recent, err := c.spClient.PlayerRecentlyPlayedOpt(ctx, &spotify.RecentlyPlayedOptions{Limit: 50})
	if err != nil || recent == nil {
		return nil, err
	}

	var tracks []Track
	seen := make(map[string]bool)
	for _, item := range recent {
		if item.Track.ID != "" && !seen[string(item.Track.ID)] {
			seen[string(item.Track.ID)] = true
			tracks = append(tracks, extractSimpleTrack(&item.Track))
			if len(tracks) >= 30 {
				break
			}
		}
	}
	return tracks, nil
}

func (c *Client) QueueSong(ctx context.Context, trackURI string) error {
	id := extractTrackID(trackURI)
	if id == "" {
		return nil
	}
	return c.spClient.QueueSong(ctx, spotify.ID(id))
}

func (c *Client) PlayTrackAtPosition(ctx context.Context, trackURI, contextURI string, positionMs int) error {
	opts := &spotify.PlayOptions{
		PositionMs: spotify.Numeric(positionMs),
	}

	if contextURI != "" && !strings.Contains(contextURI, "collection") {
		cURI := spotify.URI(contextURI)
		opts.PlaybackContext = &cURI
		if trackURI != "" {
			tURI := spotify.URI(trackURI)
			opts.PlaybackOffset = &spotify.PlaybackOffset{URI: tURI}
		}
	} else if trackURI != "" {
		opts.URIs = []spotify.URI{spotify.URI(trackURI)}
	}

	err := c.spClient.PlayOpt(ctx, opts)
	if err != nil && (strings.Contains(err.Error(), "No active device") || strings.Contains(err.Error(), "404")) {
		if devID := c.getTargetDeviceID(ctx); devID != nil {
			opts.DeviceID = devID
			err = c.spClient.PlayOpt(ctx, opts)
		}
	}
	if err == nil && positionMs > 0 {
		if err := c.spClient.Seek(ctx, positionMs); err != nil {
			if config.LogError != nil {
				config.LogError("backend.api", "seek after play: "+err.Error(), "api.go")
			}
		}
	}
	return err
}

func (c *Client) SaveLastState(ps *PlaybackState) {
	if ps == nil || ps.CurrentTrack == nil {
		return
	}
	c.mu.Lock()
	cp := *ps
	if ps.CurrentTrack != nil {
		ct := *ps.CurrentTrack
		cp.CurrentTrack = &ct
	}
	c.lastState = &cp
	c.mu.Unlock()

	data, err := json.Marshal(&cp)
	if err != nil {
		return
	}
	path := filepath.Join(config.GetCacheDir(), "last_state.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		if config.LogError != nil {
			config.LogError("backend.api", "save last state: "+err.Error(), "api.go")
		}
	}
}

func (c *Client) GetLastSavedState() *PlaybackState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastState
}

func (c *Client) LoadLastState() *PlaybackState {
	path := filepath.Join(config.GetCacheDir(), "last_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) && config.LogError != nil {
			config.LogError("backend.api", "read last state: "+err.Error(), "api.go")
		}
		return nil
	}
	var ps PlaybackState
	if err := json.Unmarshal(data, &ps); err != nil {
		if config.LogError != nil {
			config.LogError("backend.api", "unmarshal last state: "+err.Error(), "api.go")
		}
		return nil
	}

	ps.DeviceName = "spotumn"
	c.mu.Lock()
	if c.localDeviceID != "" {
		ps.DeviceID = c.localDeviceID
	} else {
		ps.DeviceID = ""
	}
	c.lastState = &ps
	c.mu.Unlock()
	return &ps
}
