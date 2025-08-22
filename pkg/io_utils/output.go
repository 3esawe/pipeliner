package output

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
	defer watcher.Close()

	fileInfo, err := os.Stat(path)
	if err != nil {
		log.Errorf("Directory does not exist: %s", path)
		return
	}
	if !fileInfo.IsDir() {
		log.Errorf("%s is not a directory", path)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
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

					// Concurrency control
					procMutex.Lock()
					if procMap[event.Name] {
						procMutex.Unlock()
						continue
					}
					procMap[event.Name] = true
					procMutex.Unlock()

					// Process file in goroutine
					go func(file string) {
						defer func() {
							procMutex.Lock()
							delete(procMap, file) // clean up to process the file agian
							procMutex.Unlock()
						}()
						handleDuplicate(file)
					}(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error(err)
			case <-ctx.Done():
				log.Info("Watcher closed")
				return
			}
		}
	}()

	err = watcher.Add(path)
	if err != nil {
		log.Error(err)
	}
	<-ctx.Done()
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
