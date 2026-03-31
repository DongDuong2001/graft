package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------------
// DiscordConnector formats and sends webhook payloads to Discord webhooks.
// It converts a generic JSON payload into Discord's embed format with
// support for username, avatar_url, and rich embeds.
// ---------------------------------------------------------------------------
type DiscordConnector struct {
	client *http.Client
}

// --- NewDiscordConnector creates a Discord-optimized connector ---
func NewDiscordConnector() *DiscordConnector {
	return &DiscordConnector{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Send formats the payload for Discord and delivers it ---
func (c *DiscordConnector) Send(ctx context.Context, url string, payload []byte) (int, error) {
	// --- Parse the incoming payload ---
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return 0, fmt.Errorf("discord: failed to parse payload: %w", err)
	}

	// --- Build Discord message ---
	discordMsg := make(map[string]interface{})

	// --- Use "content" field for plain text, fallback to "text" ---
	if content, ok := data["content"]; ok {
		discordMsg["content"] = content
	} else if text, ok := data["text"]; ok {
		discordMsg["content"] = text
	} else {
		// --- Fallback: format as code block ---
		pretty, _ := json.MarshalIndent(data, "", "  ")
		discordMsg["content"] = fmt.Sprintf("```json\n%s\n```", string(pretty))
	}

	// --- Forward Discord-specific fields if present ---
	for _, key := range []string{"embeds", "username", "avatar_url", "tts", "allowed_mentions"} {
		if v, ok := data[key]; ok {
			discordMsg[key] = v
		}
	}

	body, _ := json.Marshal(discordMsg)
	return c.post(ctx, url, body)
}

// --- post sends a JSON POST request to the Discord webhook URL ---
func (c *DiscordConnector) post(ctx context.Context, url string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("discord: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, fmt.Errorf("discord: received status %d", resp.StatusCode)
}
