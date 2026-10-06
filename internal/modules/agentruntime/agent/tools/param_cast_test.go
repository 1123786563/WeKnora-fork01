package tools

import (
	"encoding/json"
	"testing"
)

func TestCastParams_StringToBool(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"enabled":{"type":"boolean"}}}`)
	args := json.RawMessage(`{"enabled":"true"}`)
	result := CastParams(args, schema)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["enabled"] != true {
		t.Errorf("expected true, got %v (%T)", parsed["enabled"], parsed["enabled"])
	}
}

func TestCastParams_StringToInt(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"count":{"type":"integer"}}}`)
	args := json.RawMessage(`{"count":"42"}`)
	result := CastParams(args, schema)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	// JSON numbers are float64 in Go
	if parsed["count"] != float64(42) {
		t.Errorf("expected 42, got %v (%T)", parsed["count"], parsed["count"])
	}
}

func TestCastParams_StringToFloat(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"score":{"type":"number"}}}`)
	args := json.RawMessage(`{"score":"3.14"}`)
	result := CastParams(args, schema)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["score"] != 3.14 {
		t.Errorf("expected 3.14, got %v", parsed["score"])
	}
}

func TestCastParams_NoChangeNeeded(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	args := json.RawMessage(`{"name":"hello"}`)
	result := CastParams(args, schema)

	if string(result) != string(args) {
		t.Errorf("expected no change, got %s", result)
	}
}

func TestCastParams_NilSchema(t *testing.T) {
	args := json.RawMessage(`{"foo":"bar"}`)
	result := CastParams(args, nil)
	if string(result) != string(args) {
		t.Errorf("expected no change with nil schema")
	}
}

func TestCastParams_BoolFalseString(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"flag":{"type":"boolean"}}}`)
	args := json.RawMessage(`{"flag":"false"}`)
	result := CastParams(args, schema)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["flag"] != false {
		t.Errorf("expected false, got %v (%T)", parsed["flag"], parsed["flag"])
	}
}

func TestCastParams_StringToStringArray(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"patterns":{"type":"array","items":{"type":"string"}}}}`)
	args := json.RawMessage(`{"patterns":"OpenClaw"}`)
	result := CastParams(args, schema)

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}

	patterns, ok := parsed["patterns"].([]interface{})
	if !ok {
		t.Fatalf("expected patterns to be array, got %T", parsed["patterns"])
	}
	if len(patterns) != 1 || patterns[0] != "OpenClaw" {
		t.Fatalf("expected [OpenClaw], got %v", patterns)
	}
}

func TestCastParams_UntouchedSiblingNumberExact(t *testing.T) {
	// Issue #3934: casting "enabled" must not round the undeclared 64-bit id.
	schema := json.RawMessage(`{"type":"object","properties":{"enabled":{"type":"boolean"}}}`)
	args := json.RawMessage(`{"enabled":"true","id":9007199254740993}`)
	result := CastParams(args, schema)

	if got, want := string(result), `{"enabled":true,"id":9007199254740993}`; got != want {
		t.Errorf("expected %s, got %s", want, got)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["enabled"] != true {
		t.Errorf("expected enabled=true, got %v (%T)", parsed["enabled"], parsed["enabled"])
	}
}

func TestCastParams_NestedHighPrecisionValuesPreserved(t *testing.T) {
	// Numbers nested next to a cast target (arrays, objects, undeclared
	// params) must keep their original bytes.
	schema := json.RawMessage(`{"type":"object","properties":{"score":{"type":"number"},"items":{"type":"array"}}}`)
	args := json.RawMessage(`{"score":"3.14","items":[{"id":9223372036854775807}],"precise":0.1234567890123456789012345}`)
	result := CastParams(args, schema)

	if got, want := string(result),
		`{"score":3.14,"items":[{"id":9223372036854775807}],"precise":0.1234567890123456789012345}`; got != want {
		t.Errorf("expected %s, got %s", want, got)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed["score"] != 3.14 {
		t.Errorf("expected score=3.14, got %v", parsed["score"])
	}
}

func TestCastParams_NoOpReturnsIdenticalBytes(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}}}`)
	// Whitespace and extra undeclared params must survive byte-for-byte.
	args := json.RawMessage(`{ "name" : "hello" , "extra" : 9007199254740993 , "nested" : {"a":[1,2,3]} }`)
	result := CastParams(args, schema)

	if string(result) != string(args) {
		t.Errorf("expected identical bytes, got %s", result)
	}
}

func TestCastParams_InvalidArgsUnchanged(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"enabled":{"type":"boolean"}}}`)
	for name, args := range map[string]string{
		"truncated":      `{"enabled":`,
		"trailing value": `{"enabled":true} {"other":1}`,
		"not an object":  `[1,2,3]`,
		"null args":      `null`,
	} {
		result := CastParams(json.RawMessage(args), schema)
		if string(result) != args {
			t.Errorf("%s: expected unchanged %s, got %s", name, args, result)
		}
	}
}
