package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

// Transformer applies Go text/template to JSON payloads (low-code mapping).
type Transformer struct{}

// New creates a Transformer.
func New() *Transformer {
	return &Transformer{}
}

// Transform executes tmplString against JSON-decoded payload (object root). Empty template returns payload unchanged.
func (t *Transformer) Transform(payload []byte, tmplString string) ([]byte, error) {
	if tmplString == "" {
		return payload, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("failed to parse incoming JSON: %w", err)
	}

	tmpl, err := template.New("webhook").Parse(tmplString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transformation template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to execute transformation template: %w", err)
	}

	out := buf.Bytes()
	if !json.Valid(out) {
		return nil, fmt.Errorf("transformed output is not valid JSON")
	}

	return out, nil
}
