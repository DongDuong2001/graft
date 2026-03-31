package engine

import (
	"encoding/json"
	"fmt"
	"strings"

	"Graft/internal/models"
)

// ---------------------------------------------------------------------------
// EvaluateConditions checks whether a webhook payload satisfies all the
// rule-level conditions. Returns true if there are no conditions or if
// all conditions pass. Returns false (with reason) if any condition fails.
// ---------------------------------------------------------------------------
func EvaluateConditions(payload []byte, conditions []models.Condition) (bool, string) {
	// --- No conditions means unconditional routing ---
	if len(conditions) == 0 {
		return true, ""
	}

	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		return false, fmt.Sprintf("failed to parse payload for condition check: %v", err)
	}

	// --- Evaluate each condition; all must pass (AND logic) ---
	for i, cond := range conditions {
		fieldValue, exists := resolveField(data, cond.Field)

		switch cond.Operator {
		case "exists":
			// --- Check if the field exists in the payload ---
			if !exists {
				return false, fmt.Sprintf("condition %d: field %q does not exist", i, cond.Field)
			}
		case "eq":
			// --- Check if the field equals the expected value ---
			if !exists || fmt.Sprintf("%v", fieldValue) != cond.Value {
				return false, fmt.Sprintf("condition %d: field %q != %q", i, cond.Field, cond.Value)
			}
		case "neq":
			// --- Check if the field does NOT equal the value ---
			if exists && fmt.Sprintf("%v", fieldValue) == cond.Value {
				return false, fmt.Sprintf("condition %d: field %q == %q (should differ)", i, cond.Field, cond.Value)
			}
		case "contains":
			// --- Check if the field's string value contains the substring ---
			if !exists {
				return false, fmt.Sprintf("condition %d: field %q does not exist", i, cond.Field)
			}
			str := fmt.Sprintf("%v", fieldValue)
			if !strings.Contains(str, cond.Value) {
				return false, fmt.Sprintf("condition %d: field %q does not contain %q", i, cond.Field, cond.Value)
			}
		default:
			return false, fmt.Sprintf("condition %d: unknown operator %q", i, cond.Operator)
		}
	}

	return true, ""
}

// ---------------------------------------------------------------------------
// resolveField extracts a value from a nested JSON map using dot notation.
// Example: "event.type" resolves data["event"]["type"].
// ---------------------------------------------------------------------------
func resolveField(data map[string]interface{}, field string) (interface{}, bool) {
	parts := strings.Split(field, ".")
	current := interface{}(data)

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}
