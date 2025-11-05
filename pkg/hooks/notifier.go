package hooks

import (
	"bufio"
	"fmt"
	"os"
	"pipeliner/internal/notification"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/tools"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type NotifierHookConfig struct {
	Filename string
}

type NotifierHook struct {
	Config NotifierHookConfig
	logger *logger.Logger
}

func NewNotifierHook(config NotifierHookConfig) *NotifierHook {
	return &NotifierHook{
		Config: config,
		logger: logger.NewLogger(logrus.InfoLevel),
	}
}

func (n *NotifierHook) Name() string {
	return "notification"
}

func (n *NotifierHook) Description() string {
	return "Sends Discord notifications for each line in the specified output file (useful for vulnerability alerts)"
}

func (n *NotifierHook) Execute(ctx tools.HookContext) error {
	return n.executeNotification(ctx)
}

func (n *NotifierHook) ExecuteForStage(ctx tools.HookContext) error {
	return n.executeNotification(ctx)
}

func (n *NotifierHook) PostHook(ctx tools.HookContext) error {
	return n.executeNotification(ctx)
}

func (n *NotifierHook) executeNotification(ctx tools.HookContext) error {
	filename := n.Config.Filename
	file, err := os.Open(filename)
	if err != nil {
		n.logger.WithFields(logger.Fields{
			"filename": filename,
			"error":    err,
		}).Error("Error opening domain file")
		return err
	}
	defer file.Close()

	discord, err := notification.NewNotificationClient()
	if err != nil {
		n.logger.WithError(err).Error("Error creating discord client")
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
				msg := notification.Message{
					Title:       "New Finding",
					Description: line,
					Severity:    "info",
					Fields: map[string]string{
						"Source": filename,
					},
				}
				if err := discord.Send(msg); err != nil {
					n.logger.WithFields(logger.Fields{
						"line":  line,
						"error": err,
					}).Error("Failed to send Discord notification")
				}
				time.Sleep(250 * time.Millisecond)
			}
		}()
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		data := strings.TrimSpace(scanner.Text())
		if data != "" {
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
