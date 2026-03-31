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
// SlackConnector formats and sends webhook payloads to Slack incoming webhooks.
// It converts a generic JSON payload into Slack's expected format with text,
// blocks, and optional attachments.
// ---------------------------------------------------------------------------
type SlackConnector struct {
	client *http.Client
}

// --- NewSlackConnector creates a Slack-optimized connector ---
func NewSlackConnector() *SlackConnector {
	return &SlackConnector{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Send formats the payload for Slack and delivers it ---
func (c *SlackConnector) Send(ctx context.Context, url string, payload []byte) (int, error) {
	// --- Parse the incoming payload ---
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return 0, fmt.Errorf("slack: failed to parse payload: %w", err)
	}

	// --- Build Slack message: use "text" field if present, otherwise stringify ---
	slackMsg := make(map[string]interface{})
	if text, ok := data["text"]; ok {
		slackMsg["text"] = text
	} else {
		// --- Fallback: convert entire payload to a readable text block ---
		pretty, _ := json.MarshalIndent(data, "", "  ")
		slackMsg["text"] = fmt.Sprintf("```\n%s\n```", string(pretty))
	}

	// --- Forward existing Slack-specific fields if present ---
	for _, key := range []string{"blocks", "attachments", "channel", "username", "icon_emoji", "icon_url"} {
		if v, ok := data[key]; ok {
			slackMsg[key] = v
		}
	}

	body, _ := json.Marshal(slackMsg)
	return c.post(ctx, url, body)
}

// --- post sends a JSON POST request to the Slack webhook URL ---
func (c *SlackConnector) post(ctx context.Context, url string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("slack: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, fmt.Errorf("slack: received status %d", resp.StatusCode)
}
