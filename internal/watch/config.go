package watch

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const configDebounceDuration = 500 * time.Millisecond

// WatchConfigFile watches the config file at configPath for changes. On write/create
// (after debounce), it reads the file and calls onUpdate with the content. Loop
// prevention is the caller's responsibility: compare content with last applied and
// skip calling UpdateRouter if unchanged. Runs until ctx is cancelled.
func WatchConfigFile(ctx context.Context, configPath string, onUpdate func(configJSON string) error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("ERROR: Failed to create fsnotify watcher for config file: %v", err)
		return
	}
	defer watcher.Close()

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("ERROR: Failed to create config dir %s: %v", dir, err)
		return
	}
	if err := watcher.Add(dir); err != nil {
		log.Printf("ERROR: Failed to add watch on %s: %v", dir, err)
		return
	}

	var debounceTimer *time.Timer
	var debounceMu sync.Mutex
	scheduleRead := func() {
		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(configDebounceDuration, func() {
			data, err := os.ReadFile(configPath)
			if err != nil {
				if !os.IsNotExist(err) {
					log.Printf("ERROR: Failed to read config file %s: %v", configPath, err)
				}
				return
			}
			if len(data) == 0 {
				return
			}
			if onUpdate(string(data)) != nil {
				// Caller logs the error
				return
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
			// We watch the directory; only react to changes to our config file
			if filepath.Clean(event.Name) != filepath.Clean(configPath) {
				continue
			}
			if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
				scheduleRead()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("ERROR: Config file watcher error: %v", err)
		}
	}
}
