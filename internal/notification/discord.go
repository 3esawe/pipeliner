package notification

import (
	"fmt"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Message struct {
	Title       string
	Description string
	Severity    string
	Fields      map[string]string
	Timestamp   time.Time
}

type NotificationClient struct {
	sg *discordgo.Session
}

func NewNotificationClient() (*NotificationClient, error) {
	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN environment variable not set")
	}

	sg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	if err := sg.Open(); err != nil {
		return nil, err
	}

	return &NotificationClient{sg: sg}, nil
}

func (c *NotificationClient) getSeverityColor(severity string) int {
	switch severity {
	case "critical":
		return 0x8B0000
	case "high":
		return 0xFF0000
	case "medium":
		return 0xFF8C00
	case "low":
		return 0xFFD700
	case "info":
		return 0x00BFFF
	default:
		return 0x808080
	}
}

func (c *NotificationClient) Send(msg Message) error {
	if c.sg == nil {
		return fmt.Errorf("Discord client not initialized")
	}

	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		return fmt.Errorf("DISCORD_CHANNEL_ID not set")
	}

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	embed := &discordgo.MessageEmbed{
		Title:       msg.Title,
		Description: msg.Description,
		Color:       c.getSeverityColor(msg.Severity),
		Timestamp:   msg.Timestamp.Format(time.RFC3339),
	}

	if len(msg.Fields) > 0 {
		fields := make([]*discordgo.MessageEmbedField, 0, len(msg.Fields))
		for key, value := range msg.Fields {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   key,
				Value:  value,
				Inline: true,
			})
		}
		embed.Fields = fields
	}

	_, err := c.sg.ChannelMessageSendEmbed(channelID, embed)
	return err
}

func (c *NotificationClient) Close() error {
	if c.sg != nil {
		return c.sg.Close()
	}
	return nil
}
