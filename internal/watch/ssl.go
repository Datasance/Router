package watch

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/datasance/router/internal/qdr"
)

const debounceDuration = 500 * time.Millisecond

// ScanSSLProfileDir scans basePath for profile subdirs (each with ca.crt, and optionally tls.crt/tls.key)
// and returns a map of profile name to qdr.SslProfile with absolute paths.
func ScanSSLProfileDir(basePath string) (map[string]qdr.SslProfile, error) {
	entries, err := os.ReadDir(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	profiles := make(map[string]qdr.SslProfile)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		dir := filepath.Join(basePath, name)
		caPath := filepath.Join(dir, "ca.crt")
		certPath := filepath.Join(dir, "tls.crt")
		keyPath := filepath.Join(dir, "tls.key")
		if _, err := os.Stat(caPath); err != nil {
			continue
		}
		profile := qdr.SslProfile{
			Name:       name,
			CaCertFile: caPath,
		}
		if _, err := os.Stat(certPath); err == nil {
			profile.CertFile = certPath
		}
		if _, err := os.Stat(keyPath); err == nil {
			profile.PrivateKeyFile = keyPath
		}
		profiles[name] = profile
	}
	return profiles, nil
}

// WatchSSLProfileDir watches basePath (and subdirs) for changes, debounces events,
// then rescans and calls onUpdate with the new profiles map. Runs until ctx is cancelled.
func WatchSSLProfileDir(ctx context.Context, basePath string, onUpdate func(profiles map[string]qdr.SslProfile)) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("ERROR: Failed to create fsnotify watcher for SSL profile path: %v", err)
		return
	}
	defer watcher.Close()
	if err := watcher.Add(basePath); err != nil {
		if !os.IsNotExist(err) {
			log.Printf("ERROR: Failed to add watch on %s: %v", basePath, err)
		}
		return
	}
	// Watch new subdirs when they appear
	subdirs := make(map[string]struct{})
	var mu sync.Mutex
	addSubdir := func(path string) {
		if path == basePath {
			return
		}
		rel, err := filepath.Rel(basePath, path)
		if err != nil || len(rel) == 0 || rel == ".." || strings.HasPrefix(rel, "..") {
			return
		}
		if filepath.Dir(rel) != "." {
			return
		}
		mu.Lock()
		if _, ok := subdirs[path]; !ok {
			subdirs[path] = struct{}{}
			_ = watcher.Add(path)
		}
		mu.Unlock()
	}
	// Initial scan of subdirs
	if entries, err := os.ReadDir(basePath); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				addSubdir(filepath.Join(basePath, e.Name()))
			}
		}
	}
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex
	scheduleRescan := func() {
		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(debounceDuration, func() {
			profiles, err := ScanSSLProfileDir(basePath)
			if err != nil {
				log.Printf("ERROR: Failed to rescan SSL profile dir: %v", err)
				return
			}
			if len(profiles) > 0 {
				onUpdate(profiles)
			}
		})
		debounceMu.Unlock()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() && event.Op == fsnotify.Create {
					addSubdir(event.Name)
				}
				scheduleRescan()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("ERROR: SSL profile watcher error: %v", err)
		}
	}
}
