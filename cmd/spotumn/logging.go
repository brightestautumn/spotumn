// Internal logger - captures all TUI activity and errors to ~/.cache/spotumn/spotumn.log.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"spotumn/internal/config"
)

var (
	logFile *os.File
	logMu   sync.Mutex
)

// InitLogger opens ~/.cache/spotumn/spotumn.log (fresh log each session).
func InitLogger() {
	dir := config.GetCacheDir()
	path := filepath.Join(dir, "spotumn.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return
	}
	logFile = f

	config.Log = logMsg
	config.LogError = logError

	// Ensure no stale file at ~/.cache/spotumn.log (logs strictly inside ~/.cache/spotumn/)
	if home, err := os.UserHomeDir(); err == nil {
		_ = os.Remove(filepath.Join(home, ".cache", "spotumn.log"))
	}
}

// CloseLogger flushes and closes the log file.
func CloseLogger() {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile != nil {
		_ = logFile.Close()
		logFile = nil
	}
}

// logMsg writes a single event/change line. Format: datetime /component/ message /file
func logMsg(component, msg, file string) {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(logFile, "%s /%s/ %s /%s\n", ts, component, msg, file)
}

// logError writes an error line marked with [ERROR].
func logError(component, msg, file string) {
	logMsg(component, "[ERROR] "+msg, file)
}
