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
	return "Sends Discord notifications for nuclei findings with proper formatting"
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

		severity := n.getSeverity(result.Info)
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

func (n *NotifierHook) buildNucleiMessage(result parsers.NucleiResult) notification.Message {
	severity := n.getSeverity(result.Info)
	templateName := n.getTemplateName(result.Info)
	description := n.getDescription(result.Info)

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

	tags := n.getTags(result.Info)
	if tags != "" {
		msg.Fields["Tags"] = tags
	}

	return msg
}

func (n *NotifierHook) getSeverity(info map[string]interface{}) string {
	if severity, ok := info["severity"].(string); ok {
		return strings.ToLower(severity)
	}
	return "info"
}

func (n *NotifierHook) getTemplateName(info map[string]interface{}) string {
	if name, ok := info["name"].(string); ok {
		return name
	}
	return "Unknown Template"
}

func (n *NotifierHook) getDescription(info map[string]interface{}) string {
	if desc, ok := info["description"].(string); ok {
		return strings.TrimSpace(desc)
	}
	return ""
}

func (n *NotifierHook) getTags(info map[string]interface{}) string {
	if tags, ok := info["tags"].([]interface{}); ok {
		var tagStrs []string
		for _, t := range tags {
			if s, ok := t.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}
		if len(tagStrs) > 5 {
			tagStrs = tagStrs[:5]
		}
		return strings.Join(tagStrs, ", ")
	}
	return ""
}
