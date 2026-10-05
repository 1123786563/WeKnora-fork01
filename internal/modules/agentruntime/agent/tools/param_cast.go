package tools

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// CastParams performs schema-driven type casting on tool arguments.
// LLMs sometimes return incorrect types (e.g., "true" instead of true, "123" instead of 123).
// This function attempts safe conversions based on the JSON Schema definition of the tool's parameters.
//
// Only parameters that actually undergo a conversion are rewritten; every other
// parameter keeps its original JSON bytes, so untouched numbers (large integer
// IDs, high-precision decimals, values nested in arrays/objects) are never
// rounded as a side effect. When nothing is cast the input is returned as-is.
//
// If the schema is nil or cannot be parsed, the original args are returned unchanged.
func CastParams(args json.RawMessage, schema json.RawMessage) json.RawMessage {
	if len(schema) == 0 || len(args) == 0 {
		return args
	}

	var schemaDef map[string]interface{}
	if err := json.Unmarshal(schema, &schemaDef); err != nil {
		return args
	}

	properties, ok := schemaDef["properties"].(map[string]interface{})
	if !ok || len(properties) == 0 {
		return args
	}

	entries, ok := splitObjectEntries(args)
	if !ok {
		return args
	}

	changed := false
	for i, entry := range entries {
		propDef, exists := properties[entry.key]
		if !exists {
			continue
		}
		prop, ok := propDef.(map[string]interface{})
		if !ok {
			continue
		}
		targetType, _ := prop["type"].(string)
		if targetType == "" {
			continue
		}

		// Decode only the parameter under consideration so every other
		// parameter keeps its original bytes.
		var val interface{}
		if err := json.Unmarshal(entry.raw, &val); err != nil {
			continue
		}

		newVal, didCast := castValue(val, targetType)
		if !didCast {
			continue
		}
		encoded, err := json.Marshal(newVal)
		if err != nil {
			return args
		}
		entries[i].raw = encoded
		changed = true
	}

	if !changed {
		return args
	}

	// Reassemble in the original key order to avoid spurious diffs.
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, entry := range entries {
		if i > 0 {
			buf.WriteByte(',')
		}
		keyJSON, err := json.Marshal(entry.key)
		if err != nil {
			return args
		}
		buf.Write(keyJSON)
		buf.WriteByte(':')
		buf.Write(entry.raw)
	}
	buf.WriteByte('}')
	return buf.Bytes()
}

// objectEntry is a single top-level key/value pair of the args object,
// retaining the value's original JSON bytes.
type objectEntry struct {
	key string
	raw json.RawMessage
}

// splitObjectEntries decodes the top-level object of args while preserving the
// original key order and the untouched JSON bytes of each value. Duplicate keys
// keep the last value (matching map unmarshal semantics) at the first
// occurrence's position. It reports false when args is not a single valid JSON
// object, in which case callers return the input unchanged.
func splitObjectEntries(args json.RawMessage) ([]objectEntry, bool) {
	dec := json.NewDecoder(bytes.NewReader(args))

	tok, err := dec.Token()
	if err != nil {
		return nil, false
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, false
	}

	entries := make([]objectEntry, 0, 8)
	index := make(map[string]int, 8)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, false
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, false
		}
		if i, dup := index[key]; dup {
			entries[i].raw = raw
			continue
		}
		index[key] = len(entries)
		entries = append(entries, objectEntry{key: key, raw: raw})
	}

	// Consume the closing '}' and require that no trailing content follows.
	if _, err := dec.Token(); err != nil {
		return nil, false
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, false
	}
	return entries, true
}

// castValue attempts to convert val to the expected targetType.
// Returns (newValue, true) if a conversion was made, (val, false) otherwise.
func castValue(val interface{}, targetType string) (interface{}, bool) {
	switch targetType {
	case "array":
		if s, ok := val.(string); ok {
			// Try JSON parsing first (handles "[{...}]" → []interface{})
			var parsed []interface{}
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				return parsed, true
			}
			// Fall back: single string → string array
			return []string{s}, true
		}

	case "boolean":
		if s, ok := val.(string); ok {
			lower := strings.ToLower(s)
			switch lower {
			case "true", "1", "yes":
				return true, true
			case "false", "0", "no":
				return false, true
			}
		}
		// JSON number 0/1 -> bool
		if n, ok := val.(float64); ok {
			if n == 0 {
				return false, true
			}
			if n == 1 {
				return true, true
			}
		}

	case "integer":
		if s, ok := val.(string); ok {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				return i, true
			}
		}
		// JSON numbers are float64 in Go; convert to int if it's a whole number
		if f, ok := val.(float64); ok {
			if f == float64(int64(f)) {
				return int64(f), true
			}
		}

	case "number":
		if s, ok := val.(string); ok {
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f, true
			}
		}

	case "string":
		// Non-string values -> string (e.g., number or bool passed as non-string)
		switch v := val.(type) {
		case bool:
			if v {
				return "true", true
			}
			return "false", true
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), true
		case int64:
			return strconv.FormatInt(v, 10), true
		}
	}

	return val, false
}
