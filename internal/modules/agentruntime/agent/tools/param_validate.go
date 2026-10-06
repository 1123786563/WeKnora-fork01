package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ValidationError describes a specific parameter validation failure.
type ValidationError struct {
	Param   string // parameter name (or "" for top-level errors)
	Message string // human-readable error description
}

// ValidateParams checks args against the JSON Schema declared by tool.Parameters().
// Returns nil if valid, or a list of validation errors.
//
// Supported checks:
//   - required: ensures all required fields are present and non-null
//   - type: verifies the JSON type matches (string, number, integer, boolean, array, object)
//   - enum: checks the value is in the allowed set
//   - minimum / maximum: numeric bounds
//   - minLength / maxLength: string length bounds
func ValidateParams(args json.RawMessage, schema json.RawMessage) []ValidationError {
	if len(schema) == 0 || len(args) == 0 {
		return nil
	}

	var schemaDef map[string]any
	if err := json.Unmarshal(schema, &schemaDef); err != nil {
		return nil
	}

	properties, _ := schemaDef["properties"].(map[string]any)
	if len(properties) == 0 {
		return nil
	}

	var argsMap map[string]any
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return nil
	}

	var errs []ValidationError

	// Check required fields
	if reqRaw, ok := schemaDef["required"]; ok {
		if reqList, ok := reqRaw.([]any); ok {
			for _, r := range reqList {
				fieldName, ok := r.(string)
				if !ok {
					continue
				}
				val, exists := argsMap[fieldName]
				if !exists || val == nil {
					errs = append(errs, ValidationError{
						Param:   fieldName,
						Message: fmt.Sprintf("required parameter '%s' is missing", fieldName),
					})
				}
			}
		}
	}

	// Validate each provided argument against its property schema
	for key, val := range argsMap {
		propDef, exists := properties[key]
		if !exists {
			continue // extra params are allowed (LLMs sometimes add them)
		}
		prop, ok := propDef.(map[string]any)
		if !ok {
			continue
		}

		propErrs := validateProperty(key, val, prop)
		errs = append(errs, propErrs...)
	}

	return errs
}

// validateProperty validates a single parameter value against its schema definition.
func validateProperty(name string, val any, prop map[string]any) []ValidationError {
	if val == nil {
		return nil // nil values are handled by required check
	}

	var errs []ValidationError

	targetType, _ := prop["type"].(string)

	// Type check
	if targetType != "" && !checkType(val, targetType) {
		errs = append(errs, ValidationError{
			Param:   name,
			Message: fmt.Sprintf("parameter '%s' should be type '%s'", name, targetType),
		})
		return errs // skip further checks if type is wrong
	}

	// Enum check
	if enumRaw, ok := prop["enum"]; ok {
		if enumList, ok := enumRaw.([]any); ok && len(enumList) > 0 {
			if !isInEnum(val, enumList) {
				allowed := formatEnum(enumList)
				errs = append(errs, ValidationError{
					Param:   name,
					Message: fmt.Sprintf("parameter '%s' must be one of [%s]", name, allowed),
				})
			}
		}
	}

	// Numeric bounds
	if targetType == "number" || targetType == "integer" {
		numVal := toFloat64(val)
		if minVal, ok := getFloat(prop, "minimum"); ok && numVal < minVal {
			errs = append(errs, ValidationError{
				Param:   name,
				Message: fmt.Sprintf("parameter '%s' must be >= %v", name, minVal),
			})
		}
		if maxVal, ok := getFloat(prop, "maximum"); ok && numVal > maxVal {
			errs = append(errs, ValidationError{
				Param:   name,
				Message: fmt.Sprintf("parameter '%s' must be <= %v", name, maxVal),
			})
		}
	}

	// String length bounds
	if targetType == "string" {
		if s, ok := val.(string); ok {
			// Length counts Unicode code points (runes), not UTF-8 bytes, per
			// JSON Schema semantics ("你好" has length 2, not 6).
			if minLen, ok := getFloat(prop, "minLength"); ok && float64(utf8.RuneCountInString(s)) < minLen {
				errs = append(errs, ValidationError{
					Param: name,
					Message: fmt.Sprintf("parameter '%s' must have at least %d characters",
						name, int(minLen)),
				})
			}
			if maxLen, ok := getFloat(prop, "maxLength"); ok && float64(utf8.RuneCountInString(s)) > maxLen {
				errs = append(errs, ValidationError{
					Param: name,
					Message: fmt.Sprintf("parameter '%s' must have at most %d characters",
						name, int(maxLen)),
				})
			}
		}
	}

	return errs
}

// checkType verifies that val matches the expected JSON Schema type.
func checkType(val any, targetType string) bool {
	switch targetType {
	case "string":
		_, ok := val.(string)
		return ok
	case "number":
		_, ok := val.(float64)
		return ok
	case "integer":
		f, ok := val.(float64)
		return ok && f == float64(int64(f))
	case "boolean":
		_, ok := val.(bool)
		return ok
	case "array":
		_, ok := val.([]any)
		return ok
	case "object":
		_, ok := val.(map[string]any)
		return ok
	default:
		return true // unknown type, don't reject
	}
}

// isInEnum checks if val matches any value in the enum list using JSON
// structural equality (JSON Schema instance equality): values must share the
// same JSON type and structure. Formatting coincidences must not match
// (e.g. 1 vs "1", true vs "true", ["a","b"] vs [["a b"]]). Numbers compare
// numerically (1 == 1.0), object key order is irrelevant, arrays are ordered.
func isInEnum(val any, enumList []any) bool {
	for _, e := range enumList {
		if jsonValuesEqual(val, e) {
			return true
		}
	}
	return false
}

// jsonValuesEqual recursively compares two values decoded from JSON.
// Both sides are produced by encoding/json (numbers arrive as float64 and
// compare numerically); mismatched JSON types never match. Unknown types
// fall through to inequality so non-JSON values cannot match by accident.
func jsonValuesEqual(a, b any) bool {
	switch av := a.(type) {
	case nil:
		return b == nil
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !jsonValuesEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			bvVal, exists := bv[k]
			if !exists || !jsonValuesEqual(v, bvVal) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// formatEnum formats enum values for error messages.
func formatEnum(enumList []any) string {
	parts := make([]string, len(enumList))
	for i, e := range enumList {
		parts[i] = fmt.Sprintf("%v", e)
	}
	return strings.Join(parts, ", ")
}

// getFloat extracts a float64 value from a map by key.
func getFloat(m map[string]any, key string) (float64, bool) {
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	return f, ok
}

// toFloat64 converts a numeric value to float64.
func toFloat64(val any) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	case int:
		return float64(v)
	default:
		return 0
	}
}

// FormatValidationErrors formats a list of validation errors into a human-readable string.
func FormatValidationErrors(errs []ValidationError) string {
	if len(errs) == 0 {
		return ""
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Message
	}
	return "Parameter validation failed: " + strings.Join(msgs, "; ")
}
