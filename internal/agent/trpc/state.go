package trpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/Tencent/WeKnora/internal/types"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

const (
	// StateVersion versions the durable business state, independently of the SDK envelope.
	StateVersion = 1
	// GraphVersion identifies the compatible graph topology.
	GraphVersion = "1"
	// StateKey is the graph channel carrying the durable business state.
	StateKey = "weknora_state"
)

// State contains only serializable execution data. Live capabilities belong to
// worker assembly, never a checkpoint. Messages contain completed model outputs.
type State struct {
	Version         int             `json:"version"`
	Messages        []model.Message `json:"messages,omitempty"`
	PendingCallIDs  []string        `json:"pending_call_ids,omitempty"`
	NextCallIndex   int             `json:"next_call_index"`
	AppliedCallIDs  map[string]bool `json:"applied_call_ids,omitempty"`
	CompactionState json.RawMessage `json:"compaction_state,omitempty"`
	ModelAttemptID  string          `json:"model_attempt_id,omitempty"`
	InputCursor     int64           `json:"input_cursor"`
	// AppliedSteerIDs records steering inputs whose messages are already in
	// Messages. The pending row stays unprocessed on purpose: the
	// checkpointed state is the exactly-once boundary for injection.
	AppliedSteerIDs []string                   `json:"applied_steer_ids,omitempty"`
	UsageAttempts   map[string]json.RawMessage `json:"usage_attempts,omitempty"`
	Capabilities    CapabilitySnapshot         `json:"capabilities"`
}

// CompactionSnapshot is version 1 of CompactionState's JSON schema.
type CompactionSnapshot struct {
	Version     int    `json:"version"`
	Summary     string `json:"summary"`
	InputCursor int64  `json:"input_cursor"`
}

// ModelAttempt is version 1 of UsageAttempts' JSON schema. Original usage keeps
// cache reporting fields that the SDK's narrower Usage type cannot represent.
type ModelAttempt struct {
	Version  int              `json:"version"`
	Response *model.Response  `json:"response"`
	Usage    types.TokenUsage `json:"usage"`
}

// Validate rejects incompatible schemas and impossible execution cursors.
func (s State) Validate() error {
	if s.Version != StateVersion {
		return fmt.Errorf("unsupported state version %d", s.Version)
	}
	if err := s.Capabilities.Validate(); err != nil {
		return fmt.Errorf("capabilities: %w", err)
	}
	if s.NextCallIndex < 0 || s.NextCallIndex > len(s.PendingCallIDs) || s.InputCursor < 0 {
		return fmt.Errorf("invalid execution cursor")
	}
	seen := make(map[string]bool)
	for _, id := range s.PendingCallIDs {
		if id == "" || seen[id] {
			return fmt.Errorf("invalid pending call identity")
		}
		seen[id] = true
	}
	if len(s.CompactionState) > 0 {
		var cp CompactionSnapshot
		if err := decodeStrict(s.CompactionState, &cp); err != nil {
			return fmt.Errorf("compaction state: %w", err)
		}
		if cp.Version != 1 || cp.InputCursor < 0 {
			return fmt.Errorf("unsupported compaction schema or cursor")
		}
	}
	for id, raw := range s.UsageAttempts {
		var attempt ModelAttempt
		if err := decodeStrict(raw, &attempt); err != nil {
			return fmt.Errorf("usage attempt %s: %w", id, err)
		}
		if attempt.Version != 1 || attempt.Response == nil || attempt.Response.ID != id ||
			!attempt.Response.Done || attempt.Response.IsPartial || attempt.Response.Error != nil {
			return fmt.Errorf("invalid or unsupported completed usage attempt %s", id)
		}
	}
	return nil
}

// statePlain mirrors State without its UnmarshalJSON, for strict decoding.
type statePlain State

// UnmarshalJSON validates recovery state before exposing any decoded values.
func (s *State) UnmarshalJSON(raw []byte) error {
	var next statePlain
	if err := decodeStrict(raw, &next); err != nil {
		return err
	}
	// The graph schema seeds the state channel with the zero State before the
	// prepare node stamps the durable version, so the SDK's first checkpoint
	// legitimately carries a version-0 seed. A seed is accepted only when no
	// execution-owned key appears in the JSON at all: any populated field at
	// version 0 stays rejected because only prepare can stamp StateVersion.
	if next.Version == 0 && stateIsFreshSeed(raw, next) {
		*s = State(next)
		return nil
	}
	if err := State(next).Validate(); err != nil {
		return err
	}
	*s = State(next)
	return nil
}

// stateIsFreshSeed reports whether the raw JSON carries none of the
// execution-owned state keys and every decoded value is zero. Used only to
// accept the pre-prepare schema seed.
func stateIsFreshSeed(raw []byte, next statePlain) bool {
	if len(next.Messages) != 0 || len(next.PendingCallIDs) != 0 || next.NextCallIndex != 0 ||
		len(next.AppliedCallIDs) != 0 || len(next.CompactionState) != 0 || next.ModelAttemptID != "" ||
		next.InputCursor != 0 || len(next.UsageAttempts) != 0 || len(next.AppliedSteerIDs) != 0 ||
		!next.Capabilities.IsEmpty() {
		return false
	}
	var presence map[string]json.RawMessage
	if err := json.Unmarshal(raw, &presence); err != nil {
		return false
	}
	for key := range presence {
		switch key {
		case "version", "next_call_index", "input_cursor", "capabilities":
		default:
			return false
		}
	}
	return true
}

func decodeStrict(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing JSON data")
	}
	return nil
}
