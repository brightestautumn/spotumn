// Spotify OAuth 2.0 PKCE authentication flow, local callback listener, and token refresh handling.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"spotumn/internal/backend"
	"spotumn/internal/config"

	"golang.org/x/oauth2"
)

const (
	SpotifyAuthURL  = "https://accounts.spotify.com/authorize"
	SpotifyTokenURL = "https://accounts.spotify.com/api/token"
)

var Scopes = []string{
	"user-read-private",
	"user-read-email",
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"playlist-read-private",
	"playlist-read-collaborative",
	"user-library-read",
	"user-follow-read",
	"user-read-playback-position",
	"user-top-read",
	"user-read-recently-played",
	"streaming",
}

type AuthService struct {
	cfg        *config.Config
	oauthCfg   *oauth2.Config
	tokenMu    sync.RWMutex
	token      *oauth2.Token
	tokenFile  string
	httpClient *http.Client
}

func NewAuthService(cfg *config.Config) *AuthService {
	redirectURI := cfg.RedirectURI
	if redirectURI == "" {
		redirectURI = fmt.Sprintf("http://127.0.0.1:%d/login", cfg.Port)
	}
	oauthCfg := &oauth2.Config{
		ClientID: config.SpotifyClientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  SpotifyAuthURL,
			TokenURL: SpotifyTokenURL,
		},
		RedirectURL: redirectURI,
		Scopes:      Scopes,
	}

	return &AuthService{
		cfg:       cfg,
		oauthCfg:  oauthCfg,
		tokenFile: filepath.Join(config.GetDir(), "credentials.json"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (a *AuthService) RedirectURL() string {
	return a.oauthCfg.RedirectURL
}

func (a *AuthService) ClientID() string {
	return a.oauthCfg.ClientID
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func GeneratePKCE() (verifier, challenge string, err error) {
	bytes, err := GenerateRandomBytes(64)
	if err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(bytes)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

type StoredCredentials struct {
	oauth2.Token
	ClientID string `json:"client_id,omitempty"`
}

func (a *AuthService) LoadSavedToken() (*oauth2.Token, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	data, err := os.ReadFile(a.tokenFile)
	if err != nil {
		return nil, err
	}

	var stored StoredCredentials
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, err
	}

	if stored.ClientID != a.oauthCfg.ClientID &&
		stored.ClientID != config.SpotifyLibrespotClientID &&
		stored.ClientID != "" {
		return nil, errors.New("client ID mismatch")
	}

	a.token = &stored.Token
	return &stored.Token, nil
}

func (a *AuthService) SaveToken(tok *oauth2.Token) error {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	if tok.RefreshToken == "" && a.token != nil && a.token.RefreshToken != "" {
		tok.RefreshToken = a.token.RefreshToken
	}

	a.token = tok
	stored := StoredCredentials{
		Token:    *tok,
		ClientID: a.oauthCfg.ClientID,
	}
	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(a.tokenFile, data, 0600)
}

func (a *AuthService) GetTokenSource(ctx context.Context) oauth2.TokenSource {
	a.tokenMu.RLock()
	current := a.token
	a.tokenMu.RUnlock()

	initialRefresh := ""
	if current != nil {
		initialRefresh = current.RefreshToken
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, a.httpClient)
	ts := a.oauthCfg.TokenSource(ctx, current)

	return oauth2.ReuseTokenSource(current, &savingTokenSource{
		src:          ts,
		save:         a.SaveToken,
		refreshToken: initialRefresh,
	})
}

type savingTokenSource struct {
	src          oauth2.TokenSource
	save         func(*oauth2.Token) error
	refreshToken string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	if tok.RefreshToken == "" && s.refreshToken != "" {
		tok.RefreshToken = s.refreshToken
	} else if tok.RefreshToken != "" {
		s.refreshToken = tok.RefreshToken
	}
	if err := s.save(tok); err != nil {
		if config.LogError != nil {
			config.LogError("auth.session", "token save: "+err.Error(), "session.go")
		}
	}
	return tok, nil
}

func (a *AuthService) Authorize(ctx context.Context, onURL ...func(string)) (*oauth2.Token, error) {
	if tok, err := a.LoadSavedToken(); err == nil && tok != nil {
		if tok.Valid() {
			config.LogMsg("auth.session", "using valid cached OAuth session", "session.go")
			return tok, nil
		}
		if tok.RefreshToken != "" {
			ctxWithHTTP := context.WithValue(ctx, oauth2.HTTPClient, a.httpClient)
			ts := a.oauthCfg.TokenSource(ctxWithHTTP, tok)
			if refreshed, err := ts.Token(); err == nil && refreshed.Valid() {
				if refreshed.RefreshToken == "" {
					refreshed.RefreshToken = tok.RefreshToken
				}
				if err := a.SaveToken(refreshed); err != nil {
					if config.LogError != nil {
						config.LogError("auth.session", "refreshed token save: "+err.Error(), "session.go")
					}
				}
				config.LogMsg("auth.session", "refreshed OAuth token successfully", "session.go")
				return refreshed, nil
			}
		}
	}
	config.LogMsg("auth.session", "cached token expired or not found, starting fresh login", "session.go")
	return a.AuthorizeNew(ctx, onURL...)
}

func (a *AuthService) AuthorizeNew(ctx context.Context, onURL ...func(string)) (*oauth2.Token, error) {
	// generate cryptographic pkce challenge and state to prevent auth interception
	verifier, challenge, err := GeneratePKCE()
	if err != nil {
		return nil, fmt.Errorf("failed generating PKCE: %w", err)
	}

	stateBytes, err := GenerateRandomBytes(16)
	if err != nil {
		return nil, fmt.Errorf("failed generating state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	authURL := a.oauthCfg.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", challenge),
	)

	if len(onURL) > 0 && onURL[0] != nil {
		onURL[0](authURL)
	}

	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	var once sync.Once

	// spin up temporary loopback server to catch spotify's oauth redirect code
	addr := fmt.Sprintf("127.0.0.1:%d", a.cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind local callback on %s: %w", addr, err)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	callbackHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" && r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}

		q := r.URL.Query()
		code := q.Get("code")
		errStr := q.Get("error")

		if code == "" && errStr == "" {
			http.NotFound(w, r)
			return
		}

		queryState := q.Get("state")
		if queryState != state {
			http.Error(w, "Invalid state parameter (potential CSRF)", http.StatusBadRequest)
			once.Do(func() {
				errChan <- errors.New("state parameter mismatch: potential CSRF attack")
			})
			return
		}

		if errStr != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, "<h3>Authentication error: %s</h3>", html.EscapeString(errStr))
			once.Do(func() {
				errChan <- fmt.Errorf("spotify auth error: %s", errStr)
			})
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline';")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>spotumn Authorization</title>
<style>
body { font-family: sans-serif; background: #11111b; color: #cdd6f4; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
.card { background: #181825; border: 1px solid #b4befe; border-radius: 12px; padding: 32px 48px; text-align: center; }
h1 { color: #a6e3a1; font-size: 24px; margin-bottom: 8px; }
p { color: #a6adc8; font-size: 14px; }
</style>
</head>
<body>
<div class="card">
  <h1>Authentication Successful</h1>
  <p>You can close this tab and return to <strong>spotumn</strong>.</p>
</div>
</body>
</html>`))

		once.Do(func() {
			codeChan <- code
		})
	}

	mux.HandleFunc("/login", callbackHandler)
	mux.HandleFunc("/callback", callbackHandler)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if config.LogError != nil {
				config.LogError("auth.session", "oauth callback server: "+err.Error(), "session.go")
			}
		}
	}()

	if err := backend.OpenURL(authURL); err != nil {
		if config.LogError != nil {
			config.LogError("auth.session", "open auth url: "+err.Error(), "session.go")
		}
	}

	select {
	case <-ctx.Done():
		logErr := server.Shutdown(context.Background())
		if logErr != nil && config.LogError != nil {
			config.LogError("auth.session", "server shutdown (ctx done): "+logErr.Error(), "session.go")
		}
		return nil, ctx.Err()
	case err := <-errChan:
		logErr := server.Shutdown(context.Background())
		if logErr != nil && config.LogError != nil {
			config.LogError("auth.session", "server shutdown (auth err): "+logErr.Error(), "session.go")
		}
		return nil, err
	case code := <-codeChan:
		logErr := server.Shutdown(context.Background())
		if logErr != nil && config.LogError != nil {
			config.LogError("auth.session", "server shutdown: "+logErr.Error(), "session.go")
		}

		tok, err := a.exchangePKCE(ctx, code, verifier)
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
		}

		if err := a.SaveToken(tok); err != nil {
			if config.LogError != nil {
				config.LogError("auth.session", "final token save: "+err.Error(), "session.go")
			}
		}
		return tok, nil
	}
}

func (a *AuthService) exchangePKCE(ctx context.Context, code, verifier string) (*oauth2.Token, error) {
	v := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {a.oauthCfg.RedirectURL},
		"client_id":     {a.oauthCfg.ClientID},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", SpotifyTokenURL, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]any
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr != nil {
			if config.LogError != nil {
				config.LogError("auth.session", "decode token error resp: "+decErr.Error(), "session.go")
			}
		}
		return nil, fmt.Errorf("spotify token error (HTTP %d): %v", resp.StatusCode, errResp)
	}

	var tok oauth2.Token
	var raw struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	tok.AccessToken = raw.AccessToken
	tok.TokenType = raw.TokenType
	tok.RefreshToken = raw.RefreshToken
	tok.Expiry = time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)

	return &tok, nil
}
