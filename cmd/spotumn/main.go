// Entry point for Spotumn - parses CLI flags, runs Spotify authentication, boots the player daemon, and starts the Bubble Tea TUI.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
	"spotumn/internal/auth"
	"spotumn/internal/backend"
	"spotumn/internal/config"
	"spotumn/internal/ui"
)

// listen on stdin for 'c' key to copy
func listenForCopyKey(targetURL string) func() {
	if !term.IsTerminal(os.Stdin.Fd()) {
		return func() {}
	}

	oldState, err := term.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return func() {}
	}

	cr, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		_ = term.Restore(os.Stdin.Fd(), oldState)
		return func() {}
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			cr.Cancel()
			_ = cr.Close()
			_ = term.Restore(os.Stdin.Fd(), oldState)
		})
	}

	go func() {
		buf := make([]byte, 1)
		for {
			n, err := cr.Read(buf)
			if err != nil || n == 0 {
				return
			}
			b := buf[0]
			if b == 'c' || b == 'C' {
				if err := backend.CopyToClipboard(targetURL); err != nil {
					logError("main", "copy pairing link: "+err.Error(), "main.go")
				} else {
					logMsg("main", "copied pairing URL to clipboard", "main.go")
				}
				fmt.Print("\r\x1b[2K    ✔ Link copied to clipboard!\r\n")
			} else if b == 3 {
				cleanup()
				os.Exit(0)
			}
		}
	}()

	return cleanup
}

func main() {
	// keep memory usage low and run gc eagerly for smooth tui rendering
	debug.SetMemoryLimit(40 * 1024 * 1024)
	debug.SetGCPercent(20)

	InitLogger()
	defer CloseLogger()

	logMsg("main", fmt.Sprintf("spotumn starting (pid=%d, args=%v)", os.Getpid(), os.Args[1:]), "main.go")

	isAuthMode := false
	isAuthOnly := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--auth", "-auth", "auth", "--login", "-login":
			isAuthMode = true
		case "--auth-only", "-auth-only":
			isAuthMode = true
			isAuthOnly = true
		case "--help", "-h", "-help", "help":
			logMsg("main", "show help and exit", "main.go")
			fmt.Println("Spotumn - Spotify TUI Client")
			fmt.Println()
			fmt.Println("Usage: spotumn [OPTIONS]")
			fmt.Println()
			fmt.Println("Options:")
			fmt.Println("  --auth, -auth       Authenticate with Spotify (forces fresh login)")
			fmt.Println("  --auth-only         Authenticate with Spotify and exit")
			fmt.Println("  --help, -h          Show this help message")
			os.Exit(0)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		logError("main", err.Error(), "main.go")
		fmt.Fprintf(os.Stderr, "spotumn: %v\n", err)
		os.Exit(1)
	}
	logMsg("config", fmt.Sprintf("loaded config (theme=%s, mode=%s, backend=%s, port=%d)", cfg.Theme, cfg.AppearanceMode, cfg.AudioBackend, cfg.Port), "prefs.go")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stopCopy func()
	urlPrinter := func(url string) {
		fmt.Println()
		fmt.Println("    Authentication Link:")
		fmt.Printf("    %s\n\n", url)
		fmt.Println("    [Press 'c' to copy link]")
		fmt.Println()
		fmt.Println("    If your browser does not open automatically, visit the link above.")
		fmt.Println("    Waiting for authorization callback...")
		stopCopy = listenForCopyKey(url)
	}

	authService := auth.NewAuthService(cfg)
	if isAuthMode {
		fmt.Println()
		fmt.Println("==> Spotumn Spotify Authentication")
		fmt.Println("    Opening browser to authenticate with Spotify...")
		_, err := authService.AuthorizeNew(ctx, urlPrinter)
		if stopCopy != nil {
			stopCopy()
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "spotumn: authorization failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("    ✔ Successfully authenticated!")
		if isAuthOnly {
			fmt.Println("\nRun 'spotumn' to start.")
			os.Exit(0)
		}
		time.Sleep(500 * time.Millisecond)
	} else {
		_, err := authService.Authorize(ctx, urlPrinter)
		if stopCopy != nil {
			stopCopy()
		}
		if err != nil {
			logError("main", "authorization failed: "+err.Error(), "main.go")
			fmt.Fprintf(os.Stderr, "spotumn: authorization failed: %v\n", err)
			os.Exit(1)
		}
		if stopCopy != nil {
			fmt.Println("    ✔ Successfully authenticated!")
			time.Sleep(500 * time.Millisecond)
		}
	}

	tokenSource := authService.GetTokenSource(ctx)
	client := backend.NewClient(ctx, tokenSource)

	daemon := backend.NewDaemon()
	client.SetLocalDeviceID(daemon.DeviceId())

	// save playback progress and kill daemon cleanly on sigterm or interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if last := client.GetLastSavedState(); last != nil {
			client.SaveLastState(last)
		}
		daemon.Stop()
		cancel()
		os.Exit(0)
	}()

	// prompt for connect pairing on first run if credentials aren't cached yet
	if !daemon.HasStoredCredentials() {
		fmt.Println()
		fmt.Println("==> Spotumn Embedded Player Setup (one-time)")
		fmt.Println("    Authenticating Spotify Connect playback engine...")
		if err := daemon.Start(""); err != nil {
			logError("main", "embedded player start: "+err.Error(), "main.go")
			fmt.Fprintf(os.Stderr, "spotumn: failed to start embedded player: %v\n", err)
		} else {
			defer daemon.Stop()

			var stopPairCopy func()
			select {
			case auth := <-daemon.AuthCodes():
				if auth != nil {
					fmt.Println()
					fmt.Printf("    Pairing Code: %s\n", auth.Code)
					fmt.Printf("    Pairing Link: %s\n\n", auth.Url)
					fmt.Println("    [Press 'c' to copy link]")
					fmt.Println()
					fmt.Println("    If your browser did not open automatically, visit the link above.")
					fmt.Println("    Please click 'Link account' / 'Pair' to connect the player.")
					fmt.Println("    Waiting for authorization...")
					stopPairCopy = listenForCopyKey(auth.Url)
				}
			case <-time.After(10 * time.Second):
			}

			waitCtx, waitCancel := context.WithTimeout(ctx, 2*time.Minute)
			defer waitCancel()
			if err := daemon.WaitUntilReady(waitCtx); err == nil {
				if stopPairCopy != nil {
					stopPairCopy()
				}
				fmt.Println("    ✔ Player authenticated and ready!")
				time.Sleep(500 * time.Millisecond)
			} else if stopPairCopy != nil {
				stopPairCopy()
			}
		}
	} else {
		if err := daemon.Start(""); err == nil {
			logMsg("backend.player", "embedded player engine started", "player.go")
			defer daemon.Stop()
		}
	}

	app := ui.NewAppModel(client, daemon, cfg)
	prog := tea.NewProgram(app)

	logMsg("main", "TUI interface started", "main.go")
	finalModel, err := prog.Run()
	logMsg("main", "TUI interface stopped", "main.go")
	if appModel, ok := finalModel.(*ui.AppModel); ok {
		if pb := appModel.GetPlaybackState(); pb != nil && pb.CurrentTrack != nil {
			client.SaveLastState(pb)
			logMsg("backend.api", "saved last playback state on exit", "api.go")
		}
	}

	if err != nil {
		logError("main", "tui crash: "+err.Error(), "main.go")
		daemon.Stop()
		fmt.Fprintf(os.Stderr, "spotumn error: %v\n", err)
		os.Exit(1)
	}
	logMsg("main", "spotumn exiting cleanly", "main.go")
}
