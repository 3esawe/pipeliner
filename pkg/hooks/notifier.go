package hooks

import (
	"bufio"
	"fmt"
	"os"
	"pipeliner/internal/notification"
	"pipeliner/pkg/tools"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type NotifierHookConfig struct {
	Filename string
}

type NotifierHook struct {
	Config NotifierHookConfig
}

func (n *NotifierHook) Name() string {
	return "notification"
}

func (n *NotifierHook) PostHook(ctx tools.HookContext) error {

	filename := n.Config.Filename
	file, err := os.Open(filename)
	if err != nil {
		log.Errorf("Error opening domain file %s: %v", filename, err)
		return err
	}
	defer file.Close()

	discord, err := notification.NewNotificationClient()
	if err != nil {
		log.Errorf("Error creating discord client: %v", err)
		return err
	}
	defer discord.Close()

	const workerCount = 5
	lines := make(chan string)

	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range lines {
				if err := discord.SendDomainAddedMessage(line); err != nil {
					log.Errorf("Failed to send Discord notification: %v", err)
				}
				time.Sleep(250 * time.Millisecond)
			}
		}()
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.TrimSpace(scanner.Text())
		if data == "" {
			continue // Skip empty lines and comments
		} else {
			lines <- data
		}

	}

	close(lines)
	wg.Wait()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %w", filename, err)
	}

	return nil
}
