package notification

import (
	"fmt"
	"os"

	"github.com/bwmarrin/discordgo"
)

// NotificationClient represents a Discord client.
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

	// Open a websocket connection to Discord
	if err := sg.Open(); err != nil {
		return nil, err
	}

	return &NotificationClient{sg: sg}, nil
}

func (c *NotificationClient) SendDomainAddedMessage(domain string) error {
	if c.sg == nil {
		return fmt.Errorf("Discord client not initialized")
	}

	channelID := os.Getenv("DISCORD_CHANNEL_ID")
	if channelID == "" {
		return fmt.Errorf("DISCORD_CHANNEL_ID not set")
	}

	// Create clean, simple message
	_, err := c.sg.ChannelMessageSend(channelID, fmt.Sprintf(
		"🚀 New domain discovered: `%s`\nSource: Security Pipeline", domain))
	return err
}

func (c *NotificationClient) Close() error {
	if c.sg != nil {
		return c.sg.Close()
	}
	return nil
}
