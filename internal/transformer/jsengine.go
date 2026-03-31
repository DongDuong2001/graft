package transformer

import (
	"encoding/json"
	"fmt"
)

// ---------------------------------------------------------------------------
// transformJS executes a JavaScript transformation against a JSON payload.
//
// The script receives the payload as a global object and must return the
// transformed result. Currently supports static JSON scripts and pass-through.
//
// For full JavaScript VM support (Goja), enable the "js" build tag.
// ---------------------------------------------------------------------------
func transformJS(payload []byte, script string) ([]byte, error) {
	// --- Empty script returns payload unchanged ---
	if script == "" {
		return payload, nil
	}

	// --- Parse the incoming JSON payload ---
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("js transform: failed to parse JSON: %w", err)
	}

	// --- Evaluate the script expression ---
	result, err := evaluateSimpleScript(data, script)
	if err != nil {
		return nil, fmt.Errorf("js transform: %w", err)
	}

	// --- Serialize the result back to JSON ---
	out, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("js transform: failed to serialize result: %w", err)
	}

	if !json.Valid(out) {
		return nil, fmt.Errorf("js transform: output is not valid JSON")
	}

	return out, nil
}

// ---------------------------------------------------------------------------
// evaluateSimpleScript provides a lightweight expression evaluator for basic
// JavaScript-like transformations without requiring a full JS runtime.
// ---------------------------------------------------------------------------
func evaluateSimpleScript(data map[string]interface{}, script string) (interface{}, error) {
	// --- Pass-through: return entirety of the data ---
	if script == "payload" {
		return data, nil
	}

	// --- Static JSON: parse the script as a literal JSON value ---
	var staticResult interface{}
	if err := json.Unmarshal([]byte(script), &staticResult); err == nil {
		return staticResult, nil
	}

	// --- Fallback: return data with advisory that full JS requires Goja ---
	return data, fmt.Errorf("complex JavaScript requires full Goja engine (build tag: js)")
}
