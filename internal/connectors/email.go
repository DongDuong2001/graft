package connectors

import (
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
)

// ---------------------------------------------------------------------------
// EmailConnector sends webhook payloads as email notifications via SMTP.
// It converts the JSON payload into a human-readable email body with a
// configurable subject line.
// ---------------------------------------------------------------------------
type EmailConnector struct {
	SMTPHost string // e.g. "smtp.gmail.com:587"
	Username string
	Password string
	From     string
}

// --- NewEmailConnector creates an SMTP email connector ---
func NewEmailConnector(host, username, password, from string) *EmailConnector {
	return &EmailConnector{
		SMTPHost: host,
		Username: username,
		Password: password,
		From:     from,
	}
}

// --- Send formats the payload as an email and delivers it via SMTP ---
func (c *EmailConnector) Send(to string, subject string, payload []byte) error {
	// --- Parse payload for a readable body ---
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("email: failed to parse payload: %w", err)
	}

	// --- Use "text" or "message" field if available ---
	var body string
	if text, ok := data["text"]; ok {
		body = fmt.Sprintf("%v", text)
	} else if msg, ok := data["message"]; ok {
		body = fmt.Sprintf("%v", msg)
	} else {
		// --- Fallback: pretty-print the entire payload ---
		pretty, _ := json.MarshalIndent(data, "", "  ")
		body = string(pretty)
	}

	// --- Default subject if none provided ---
	if subject == "" {
		subject = "Graft Webhook Notification"
	}

	// --- Construct the email message ---
	msg := strings.Join([]string{
		"From: " + c.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	// --- Send via SMTP with authentication ---
	auth := smtp.PlainAuth("", c.Username, c.Password, strings.Split(c.SMTPHost, ":")[0])
	return smtp.SendMail(c.SMTPHost, auth, c.From, []string{to}, []byte(msg))
}
