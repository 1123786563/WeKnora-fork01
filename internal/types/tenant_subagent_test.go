package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestSubagentsConfigJSONRoundTrip locks the persisted shape of the delegation
// list: slugs survive a config marshal/unmarshal cycle, a config without any
// omits the key entirely (legacy payloads keep parsing), and the JSON name
// follows the established snake_case convention.
func TestSubagentsConfigJSONRoundTrip(t *testing.T) {
	cfg := CustomAgentConfig{
		AgentMode: AgentModeSmartReasoning,
		Subagents: []string{"code-reviewer", "web-researcher"},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back CustomAgentConfig
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if len(back.Subagents) != 2 || back.Subagents[0] != "code-reviewer" || back.Subagents[1] != "web-researcher" {
		t.Fatalf("round-trip lost Subagents: %+v", back.Subagents)
	}
	if !strings.Contains(string(b), `"subagents":`) {
		t.Fatalf("expected the subagents key in %s", b)
	}

	// omitempty: a config without delegation must not carry the key at all.
	plain, err := json.Marshal(CustomAgentConfig{AgentMode: AgentModeQuickAnswer})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), "subagents") {
		t.Fatalf("subagents must be omitted when unset, got %s", plain)
	}
}

// TestSubagentsLegacyPayloadAndDefaultsNoInvent verifies that payloads
// persisted before the field existed still parse, and that EnsureDefaults —
// which runs inside CreateAgent for every new agent — neither invents slugs
// nor fabricates an empty list.
func TestSubagentsLegacyPayloadAndDefaultsNoInvent(t *testing.T) {
	var legacy CustomAgentConfig
	if err := json.Unmarshal([]byte(`{"agent_mode":"quick-answer"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Subagents != nil {
		t.Fatalf("legacy payload must not gain Subagents, got %+v", legacy.Subagents)
	}
	agent := &CustomAgent{Config: legacy}
	agent.EnsureDefaults()
	if agent.Config.Subagents != nil {
		t.Fatalf("EnsureDefaults must not invent Subagents, got %+v", agent.Config.Subagents)
	}
}
