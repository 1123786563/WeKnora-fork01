package trpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCapabilitySnapshotRoundTripAndCompatibility(t *testing.T) {
	before := CapabilitySnapshot{
		ToolIdentities:  []string{"svc1/search", "thinking"},
		DeferredNames:   []string{"svc1/search"},
		SkillDigests:    map[string]string{"skill1": "sha256:a"},
		MemoryPrompt:    "<memory>prefers Chinese</memory>",
		ImageReferences: []string{"artifact://img-1"},
	}
	raw, err := json.Marshal(before)
	require.NoError(t, err)
	after, err := UnmarshalCapabilitySnapshot(raw)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoError(t, before.CompatibleWith(after))
	after.SkillDigests["skill1"] = "sha256:b"
	require.ErrorContains(t, before.CompatibleWith(after), "skill capability digest changed")
}

func TestCapabilitySnapshotRejectsDeferredToolOutsideRegistry(t *testing.T) {
	_, err := UnmarshalCapabilitySnapshot([]byte(`{"tool_identities":["thinking"],"deferred_names":["mcp/search"]}`))
	require.Error(t, err)
}

func TestGraphRunnerReceivesCapabilitySnapshot(t *testing.T) {
	snapshot := CapabilitySnapshot{ToolIdentities: []string{"thinking"}}
	r, err := NewGraphRunner(GraphBindings{Model: &engineModel{}, Store: &minimalStore{}, Capabilities: snapshot})
	require.NoError(t, err)
	require.Equal(t, snapshot, r.bindings.InitialState.Capabilities)
}

func TestGraphRunnerRejectsCapabilityDrift(t *testing.T) {
	_, err := NewGraphRunner(GraphBindings{
		Model: &engineModel{}, Store: &minimalStore{},
		Capabilities: CapabilitySnapshot{ToolIdentities: []string{"thinking"}},
		InitialState: State{Version: StateVersion, Capabilities: CapabilitySnapshot{ToolIdentities: []string{"changed"}}},
	})
	require.ErrorContains(t, err, "capability compatibility")
}

func TestCloneStatePreservesNilSkillDigests(t *testing.T) {
	cloned, err := cloneState(State{Version: StateVersion})
	require.NoError(t, err)
	require.Nil(t, cloned.Capabilities.SkillDigests)
}
