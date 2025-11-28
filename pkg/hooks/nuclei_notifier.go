package hooks

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"pipeliner/internal/notification"
	"pipeliner/pkg/logger"
	"pipeliner/pkg/parsers"
	"pipeliner/pkg/tools"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type NucleiNotifierHookConfig struct {
	Filename string
}

type NucleiNotifierHook struct {
	Config NucleiNotifierHookConfig
	logger *logger.Logger
}

func NewNucleiNotifierHook(config NucleiNotifierHookConfig) *NucleiNotifierHook {
	return &NucleiNotifierHook{
		Config: config,
		logger: logger.NewLogger(logrus.InfoLevel),
	}
}

func (n *NucleiNotifierHook) Name() string {
	return "nuclei_notifier"
}

func (n *NucleiNotifierHook) Description() string {
	return "Sends Discord notifications for nuclei vulnerability findings"
}

func (n *NucleiNotifierHook) Execute(ctx tools.HookContext) error {
	return n.executeNotification(ctx)
}

func (n *NucleiNotifierHook) ExecuteForStage(ctx tools.HookContext) error {
	return n.executeNotification(ctx)
}

func (n *NucleiNotifierHook) PostHook(ctx tools.HookContext) error {
	return n.executeNotification(ctx)
}

func (n *NucleiNotifierHook) executeNotification(ctx tools.HookContext) error {
	filename := n.Config.Filename

	if !filepath.IsAbs(filename) && ctx.OutputDir != "" {
		filename = filepath.Join(ctx.OutputDir, filename)
	}

	file, err := os.Open(filename)
	if err != nil {
		n.logger.WithFields(logger.Fields{
			"filename": filename,
			"error":    err,
		}).Error("Error opening nuclei output file")
		return err
	}
	defer file.Close()

	discord, err := notification.NewNotificationClient()
	if err != nil {
		n.logger.WithError(err).Error("Error creating discord client")
		return err
	}
	defer discord.Close()

	const workerCount = 3
	findings := make(chan parsers.NucleiResult)

	var wg sync.WaitGroup

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for result := range findings {
				msg := n.buildNucleiMessage(result)
				if err := discord.Send(msg); err != nil {
					n.logger.WithFields(logger.Fields{
						"template": result.TemplateID,
						"error":    err,
					}).Error("Failed to send Discord notification")
				}
				time.Sleep(500 * time.Millisecond)
			}
		}()
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var result parsers.NucleiResult
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			n.logger.WithFields(logger.Fields{
				"error": err,
			}).Warn("Failed to parse nuclei JSON line")
			continue
		}

		severity := parsers.GetNucleiSeverity(result.Info)
		if severity == "info" {
			continue
		}

		findings <- result
	}

	close(findings)
	wg.Wait()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file %s: %w", filename, err)
	}

	return nil
}

func (n *NucleiNotifierHook) buildNucleiMessage(result parsers.NucleiResult) notification.Message {
	severity := parsers.GetNucleiSeverity(result.Info)
	templateName := parsers.GetNucleiTemplateName(result.Info)
	description := parsers.GetNucleiDescription(result.Info)

	host := result.Host
	if host == "" {
		host = result.URL
	}

	descText := fmt.Sprintf("**Target:** `%s`", result.MatchedAt)
	if description != "" {
		if len(description) > 200 {
			description = description[:197] + "..."
		}
		descText = fmt.Sprintf("%s\n\n%s", description, descText)
	}

	msg := notification.Message{
		Title:       fmt.Sprintf("%s %s", parsers.GetSeverityEmoji(severity), templateName),
		Description: descText,
		Severity:    severity,
		Fields: map[string]string{
			"Severity": strings.ToUpper(severity),
			"Host":     host,
		},
	}

	if result.MatcherName != "" {
		msg.Fields["Matcher"] = result.MatcherName
	}

	if result.IP != "" {
		msg.Fields["IP"] = result.IP
	}

	tags := parsers.GetNucleiTags(result.Info)
	if tags != "" {
		msg.Fields["Tags"] = tags
	}

	return msg
}
