package tools

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateParams(t *testing.T) {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "minLength": 1},
			"limit": {"type": "integer", "minimum": 1, "maximum": 100},
			"mode":  {"type": "string", "enum": ["fast", "deep"]},
			"score": {"type": "number", "minimum": 0, "maximum": 1},
			"enabled": {"type": "boolean"}
		},
		"required": ["query"]
	}`)

	t.Run("valid params pass", func(t *testing.T) {
		args := json.RawMessage(`{"query": "hello", "limit": 10, "mode": "fast"}`)
		errs := ValidateParams(args, schema)
		assert.Empty(t, errs)
	})

	t.Run("missing required field", func(t *testing.T) {
		args := json.RawMessage(`{"limit": 10}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "query", errs[0].Param)
		assert.Contains(t, errs[0].Message, "required")
	})

	t.Run("null required field", func(t *testing.T) {
		args := json.RawMessage(`{"query": null}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "required")
	})

	t.Run("wrong type", func(t *testing.T) {
		args := json.RawMessage(`{"query": 123}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "query", errs[0].Param)
		assert.Contains(t, errs[0].Message, "type")
	})

	t.Run("enum violation", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test", "mode": "slow"}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "mode", errs[0].Param)
		assert.Contains(t, errs[0].Message, "one of")
	})

	t.Run("minimum violation", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test", "limit": 0}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "limit", errs[0].Param)
		assert.Contains(t, errs[0].Message, ">= 1")
	})

	t.Run("maximum violation", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test", "limit": 200}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "limit", errs[0].Param)
		assert.Contains(t, errs[0].Message, "<= 100")
	})

	t.Run("minLength violation", func(t *testing.T) {
		args := json.RawMessage(`{"query": ""}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "query", errs[0].Param)
		assert.Contains(t, errs[0].Message, "at least 1 characters")
	})

	t.Run("number bounds", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test", "score": 1.5}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "score", errs[0].Param)
	})

	t.Run("multiple errors", func(t *testing.T) {
		args := json.RawMessage(`{"limit": -1, "mode": "invalid"}`)
		errs := ValidateParams(args, schema)
		assert.GreaterOrEqual(t, len(errs), 3) // missing query + limit min + mode enum
	})

	t.Run("extra params allowed", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test", "unknown_param": "value"}`)
		errs := ValidateParams(args, schema)
		assert.Empty(t, errs)
	})

	t.Run("nil schema returns nil", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test"}`)
		errs := ValidateParams(args, nil)
		assert.Nil(t, errs)
	})

	t.Run("empty args returns nil", func(t *testing.T) {
		errs := ValidateParams(nil, schema)
		assert.Nil(t, errs)
	})

	t.Run("boolean type check", func(t *testing.T) {
		args := json.RawMessage(`{"query": "test", "enabled": "yes"}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "enabled", errs[0].Param)
		assert.Contains(t, errs[0].Message, "boolean")
	})
}

// TestValidateParamsStringLengthRunes covers #3935: minLength/maxLength must
// count Unicode code points (runes), not UTF-8 bytes. Accented input using
// combining marks still counts code points ("e"+U+0301 is length 2), i.e.
// counting is per code point, not per grapheme cluster.
func TestValidateParamsStringLengthRunes(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		propDef string // property schema fragment
		wantErr string // "" for valid, else expected message substring
	}{
		// Chinese: 2 code points, 6 UTF-8 bytes.
		{"chinese within maxLength 2", `"你好"`, `{"type":"string","maxLength":2}`, ""},
		{"chinese below minLength 2", `"你"`, `{"type":"string","minLength":2}`, "at least 2 characters"},
		// Emoji: astral plane, 1 code point, 4 UTF-8 bytes.
		{"emoji within maxLength 1", `"😀"`, `{"type":"string","maxLength":1}`, ""},
		{"emoji below minLength 2", `"😀"`, `{"type":"string","minLength":2}`, "at least 2 characters"},
		// Combining acute accent: "e" + U+0301 = 2 code points, 1 grapheme cluster.
		{"combining accent within maxLength 2", `"é"`, `{"type":"string","maxLength":2}`, ""},
		{"combining accent satisfies minLength 2 (code points, not graphemes)", `"é"`, `{"type":"string","minLength":2}`, ""},
		// Mixed Chinese/ASCII.
		{"mixed within maxLength 3", `"a你b"`, `{"type":"string","maxLength":3}`, ""},
		{"mixed exceeds maxLength 2", `"a你b"`, `{"type":"string","maxLength":2}`, "at most 2 characters"},
		// Empty string.
		{"empty within maxLength 0", `""`, `{"type":"string","maxLength":0}`, ""},
		{"empty below minLength 1", `""`, `{"type":"string","minLength":1}`, "at least 1 characters"},
		// Pure ASCII: rune count equals byte count, behavior unchanged.
		{"ascii within maxLength 5", `"hello"`, `{"type":"string","maxLength":5}`, ""},
		{"ascii satisfies minLength 5", `"hello"`, `{"type":"string","minLength":5}`, ""},
		{"ascii exceeds maxLength 4", `"hello"`, `{"type":"string","maxLength":4}`, "at most 4 characters"},
		{"ascii below minLength 6", `"hello"`, `{"type":"string","minLength":6}`, "at least 6 characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := json.RawMessage(fmt.Sprintf(`{"type":"object","properties":{"text":%s}}`, tc.propDef))
			args := json.RawMessage(fmt.Sprintf(`{"text":%s}`, tc.value))
			errs := ValidateParams(args, schema)
			if tc.wantErr == "" {
				assert.Empty(t, errs, "value %s should pass schema %s", tc.value, tc.propDef)
				return
			}
			require.Len(t, errs, 1)
			assert.Equal(t, "text", errs[0].Param)
			assert.Contains(t, errs[0].Message, tc.wantErr)
		})
	}
}

// TestValidateParamsEnumStructuralEquality covers #3946: enum membership must
// use JSON structural equality instead of formatted-text comparison, so
// formatting coincidences no longer match, while legitimate matches
// (numeric 1 == 1.0, isomorphic arrays, key-order-independent objects) remain.
func TestValidateParamsEnumStructuralEquality(t *testing.T) {
	t.Run("array must not match enum of concatenated string", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"tags":{"enum":[["a b"]]}}}`)
		args := json.RawMessage(`{"tags":["a","b"]}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "tags", errs[0].Param)
		assert.Contains(t, errs[0].Message, "one of")
	})

	t.Run("number array must not match enum of string member", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"ids":{"enum":["1"]}}}`)
		args := json.RawMessage(`{"ids":[1]}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "ids", errs[0].Param)
		assert.Contains(t, errs[0].Message, "one of")
	})

	t.Run("object must not match enum of object with concatenated value", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"cfg":{"enum":[{"a":"b c:d"}]}}}`)
		args := json.RawMessage(`{"cfg":{"a":"b","c":"d"}}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "cfg", errs[0].Param)
		assert.Contains(t, errs[0].Message, "one of")
	})

	t.Run("enum-only number must not match string member", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"level":{"enum":["1","2"]}}}`)
		args := json.RawMessage(`{"level":1}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "level", errs[0].Param)
		assert.Contains(t, errs[0].Message, "one of")
	})

	t.Run("enum-only bool must not match string member", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"flag":{"enum":["true"]}}}`)
		args := json.RawMessage(`{"flag":true}`)
		errs := ValidateParams(args, schema)
		require.Len(t, errs, 1)
		assert.Equal(t, "flag", errs[0].Param)
		assert.Contains(t, errs[0].Message, "one of")
	})

	t.Run("numeric equality keeps 1 matching 1.0", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"score":{"enum":[1.0]}}}`)
		args := json.RawMessage(`{"score":1}`)
		assert.Empty(t, ValidateParams(args, schema))
	})

	t.Run("isomorphic nested array still matches", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"tags":{"enum":[["a","b"],["c"]]}}}`)
		args := json.RawMessage(`{"tags":["a","b"]}`)
		assert.Empty(t, ValidateParams(args, schema))
	})

	t.Run("object key order is irrelevant", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"cfg":{"enum":[{"b":"x","a":1.0}]}}}`)
		args := json.RawMessage(`{"cfg":{"a":1,"b":"x"}}`)
		assert.Empty(t, ValidateParams(args, schema))
	})
}

func TestFormatValidationErrors(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		assert.Equal(t, "", FormatValidationErrors(nil))
	})

	t.Run("single error", func(t *testing.T) {
		errs := []ValidationError{{Param: "q", Message: "required parameter 'q' is missing"}}
		result := FormatValidationErrors(errs)
		assert.Contains(t, result, "Parameter validation failed")
		assert.Contains(t, result, "required parameter 'q' is missing")
	})

	t.Run("multiple errors joined", func(t *testing.T) {
		errs := []ValidationError{
			{Param: "a", Message: "error a"},
			{Param: "b", Message: "error b"},
		}
		result := FormatValidationErrors(errs)
		assert.Contains(t, result, "error a; error b")
	})
}
