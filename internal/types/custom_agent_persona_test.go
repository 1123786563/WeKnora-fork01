package types

import (
	"encoding/json"
	"testing"
)

func TestPersonaConfigJSONRoundTrip(t *testing.T) {
	cfg := CustomAgentConfig{PersonaMBTI: "INTJ", PersonaStyle: "Be terse."}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var back CustomAgentConfig
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.PersonaMBTI != "INTJ" || back.PersonaStyle != "Be terse." {
		t.Fatalf("round-trip lost fields: %+v", back)
	}
	// Old payloads without the keys must not break, and EnsureDefaults must
	// not invent a persona.
	var legacy CustomAgentConfig
	if err := json.Unmarshal([]byte(`{"agent_mode":"quick-answer"}`), &legacy); err != nil {
		t.Fatal(err)
	}
	agent := &CustomAgent{Config: legacy}
	agent.EnsureDefaults()
	if agent.Config.PersonaMBTI != "" {
		t.Error("EnsureDefaults must not set a persona")
	}
}
