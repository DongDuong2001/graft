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
// TeamsConnector formats and sends webhook payloads to Microsoft Teams
// incoming webhook connectors. It converts generic JSON into the Adaptive
// Card format that Teams expects.
// ---------------------------------------------------------------------------
type TeamsConnector struct {
	client *http.Client
}

// --- NewTeamsConnector creates a MS Teams-optimized connector ---
func NewTeamsConnector() *TeamsConnector {
	return &TeamsConnector{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// --- Send formats the payload as an Adaptive Card for Teams ---
func (c *TeamsConnector) Send(ctx context.Context, url string, payload []byte) (int, error) {
	// --- Parse the incoming payload ---
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return 0, fmt.Errorf("teams: failed to parse payload: %w", err)
	}

	// --- Build Adaptive Card wrapper ---
	var text string
	if t, ok := data["text"]; ok {
		text = fmt.Sprintf("%v", t)
	} else if t, ok := data["summary"]; ok {
		text = fmt.Sprintf("%v", t)
	} else {
		// --- Fallback: format payload as readable text ---
		pretty, _ := json.MarshalIndent(data, "", "  ")
		text = string(pretty)
	}

	// --- Construct the Teams message card ---
	teamsMsg := map[string]interface{}{
		"type": "message",
		"attachments": []map[string]interface{}{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"content": map[string]interface{}{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": "1.4",
					"body": []map[string]interface{}{
						{
							"type": "TextBlock",
							"text": text,
							"wrap": true,
						},
					},
				},
			},
		},
	}

	body, _ := json.Marshal(teamsMsg)
	return c.post(ctx, url, body)
}

// --- post sends a JSON POST request to the Teams webhook URL ---
func (c *TeamsConnector) post(ctx context.Context, url string, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("teams: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp.StatusCode, nil
	}
	return resp.StatusCode, fmt.Errorf("teams: received status %d", resp.StatusCode)
}
