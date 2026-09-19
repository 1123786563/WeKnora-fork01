package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestExpertSourceJSONRoundTrip locks the persisted shape of the provenance
// block: a set ExpertSource survives a config marshal/unmarshal cycle, a
// config without one omits the key entirely (legacy payloads keep parsing),
// and EnsureDefaults never invents provenance for hand-built agents.
func TestExpertSourceJSONRoundTrip(t *testing.T) {
	cfg := CustomAgentConfig{
		AgentMode:    AgentModeSmartReasoning,
		ExpertSource: &ExpertSourceStruct{ExpertID: "stock-assistant", Source: "builtin"},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back CustomAgentConfig
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.ExpertSource == nil {
		t.Fatal("round-trip lost ExpertSource")
	}
	if back.ExpertSource.ExpertID != "stock-assistant" || back.ExpertSource.Source != "builtin" {
		t.Fatalf("round-trip lost fields: %+v", back.ExpertSource)
	}

	// omitempty: a config without provenance must not carry the key at all.
	plain, err := json.Marshal(CustomAgentConfig{AgentMode: AgentModeQuickAnswer})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "expert_source") {
		t.Fatalf("expert_source must be omitted when unset, got %s", plain)
	}

	// Field names follow the established snake_case JSON convention.
	if !strings.Contains(string(b), `"expert_id":"stock-assistant"`) ||
		!strings.Contains(string(b), `"source":"builtin"`) {
		t.Fatalf("unexpected json keys in %s", b)
	}
}

// TestExpertSourceLegacyPayloadAndDefaults verifies that payloads persisted
// before the provenance block existed still parse, and that EnsureDefaults —
// which runs inside CreateAgent for every new agent — does not fabricate an
// ExpertSource for hand-built agents.
func TestExpertSourceLegacyPayloadAndDefaults(t *testing.T) {
	var legacy CustomAgentConfig
	if err := json.Unmarshal([]byte(`{"agent_mode":"quick-answer"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.ExpertSource != nil {
		t.Fatalf("legacy payload must not gain an ExpertSource, got %+v", legacy.ExpertSource)
	}
	agent := &CustomAgent{Config: legacy}
	agent.EnsureDefaults()
	if agent.Config.ExpertSource != nil {
		t.Fatalf("EnsureDefaults must not invent an ExpertSource, got %+v", agent.Config.ExpertSource)
	}
}
