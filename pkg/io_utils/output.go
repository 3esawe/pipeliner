package output

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

var (
	procMutex      sync.Mutex
	procMap        = make(map[string]bool)
	textExtensions = map[string]bool{
		".log": true, ".txt": true, ".csv": true, ".json": true,
	}
)

func WatchDirectory(ctx context.Context) {
	path, _ := os.Getwd()
	log.Infof("Watching directory: %s", path)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(err)
		return
	}
	defer func() {
		if closeErr := watcher.Close(); closeErr != nil {
			log.Errorf("Error closing watcher: %v", closeErr)
		}
	}()

	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Errorf("Directory does not exist: %s", path)
		return
	}
	if !fileInfo.IsDir() {
		log.Errorf("%s is not a directory", path)
		return
	}

	// Add the path to watcher first, before starting the goroutine
	if err := watcher.Add(path); err != nil {
		log.Errorf("Error adding path to watcher: %v", err)
		return
	}

	go func() {
		defer log.Info("File watcher goroutine stopped")

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					log.Debug("Watcher events channel closed")
					return
				}

				if event.Op&fsnotify.Write != fsnotify.Write {
					continue
				}

				if !isTextFile(event.Name) {
					continue
				}

				// Process write events
				fi, err := os.Stat(event.Name)
				if err != nil {
					log.Errorf("Error stating %s: %v", event.Name, err)
					continue
				}
				if fi.IsDir() {
					continue
				}

				// Concurrency control with timeout
				procMutex.Lock()
				if procMap[event.Name] {
					procMutex.Unlock()
					continue
				}
				procMap[event.Name] = true
				procMutex.Unlock()

				// Process file in goroutine with proper cleanup
				go func(file string) {
					defer func() {
						procMutex.Lock()
						delete(procMap, file)
						procMutex.Unlock()
					}()

					// Add timeout context for file processing
					fileCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
					defer cancel()

					select {
					case <-fileCtx.Done():
						log.Warnf("File processing timed out for %s", file)
						return
					default:
						handleDuplicate(file)
					}
				}(event.Name)

			case err, ok := <-watcher.Errors:
				if !ok {
					log.Debug("Watcher errors channel closed")
					return
				}
				log.Errorf("Watcher error: %v", err)

			case <-ctx.Done():
				log.Info("Watcher context cancelled, stopping...")
				return
			}
		}
	}()

	// Block until context is done
	<-ctx.Done()
	log.Info("Directory watcher stopped")
}

func isTextFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return textExtensions[ext]
}

func handleDuplicate(path string) {
	fi, err := os.Stat(path)
	if err != nil {
		log.Errorf("Error stating file %s: %v", path, err)
		return
	}
	if fi.IsDir() {
		return
	}

	content, err := os.ReadFile(path)
	if err != nil {
		log.Errorf("Error reading file %s: %v", path, err)
		return
	}

	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	lines := strings.Split(string(normalized), "\n")

	seen := make(map[string]bool)
	var newLines []string
	duplicatesFound := false

	for i, line := range lines {
		// Preserve trailing empty line (last line)
		if i == len(lines)-1 && line == "" {
			newLines = append(newLines, line)
			continue
		}

		if !seen[line] {
			seen[line] = true
			newLines = append(newLines, line)
		} else {
			duplicatesFound = true
		}
	}

	if !duplicatesFound {
		return
	}

	newContent := strings.Join(newLines, "\n")
	err = os.WriteFile(path, []byte(newContent), fi.Mode().Perm())
	if err != nil {
		log.Errorf("Error writing file %s: %v", path, err)
	}
}
