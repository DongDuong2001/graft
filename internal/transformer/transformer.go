package transformer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/DongDuong2001/graft/internal/models"
)

// ---------------------------------------------------------------------------
// Transformer applies Go text/template or JavaScript transformations to
// JSON payloads. Supports both legacy single-template mode and multi-step
// pipeline transformations.
// ---------------------------------------------------------------------------
type Transformer struct{}

// New creates a Transformer.
func New() *Transformer {
	return &Transformer{}
}

// ---------------------------------------------------------------------------
// TransformPipeline executes an ordered list of transformation steps.
// Each step's output becomes the next step's input. If the steps slice
// is empty, falls back to the legacy single-template Transform method.
// ---------------------------------------------------------------------------
func (t *Transformer) TransformPipeline(payload []byte, steps []models.TransformStep, legacyTemplate string) ([]byte, error) {
	// --- Backward compatibility: use legacy template if no pipeline steps ---
	if len(steps) == 0 {
		return t.Transform(payload, legacyTemplate)
	}

	// --- Pipeline: execute each transformation step sequentially ---
	current := payload
	for i, step := range steps {
		var err error
		switch step.Type {
		case "go_template":
			// --- Go template transformation step ---
			current, err = t.transformGoTemplate(current, step.Script)
		case "javascript":
			// --- JavaScript transformation step (Goja engine) ---
			current, err = transformJS(current, step.Script)
		default:
			return nil, fmt.Errorf("step %d: unknown transform type %q", i, step.Type)
		}
		if err != nil {
			return nil, fmt.Errorf("step %d (%s): %w", i, step.Type, err)
		}
	}
	return current, nil
}

// ---------------------------------------------------------------------------
// Transform executes a single Go text/template against a JSON payload.
// Empty template returns payload unchanged. This is the legacy interface.
// ---------------------------------------------------------------------------
func (t *Transformer) Transform(payload []byte, tmplString string) ([]byte, error) {
	if tmplString == "" {
		return payload, nil
	}
	return t.transformGoTemplate(payload, tmplString)
}

// --- transformGoTemplate applies a Go text/template to the JSON payload ---
func (t *Transformer) transformGoTemplate(payload []byte, tmplString string) ([]byte, error) {
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
