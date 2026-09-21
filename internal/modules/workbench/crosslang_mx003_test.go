package workbench

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// MX-003 跨语言冻结测试：本文件与 tests/mobile-v2/probes/mx-003.ts 消费同一份
// tests/mobile-v2/fixtures/mx-003-crosslang.json。禁止任何一侧另造 fixture。

type mx003Fixture struct {
	ExecutionUnavailableCancel ExecutionDTO          `json:"execution_unavailable_cancel"`
	EventsValid                []ExecutionEvent       `json:"events_valid"`
	EventsInvalid              []ExecutionEvent       `json:"events_invalid"`
	DecisionsValid             []InteractionDecision  `json:"decisions_valid"`
	DecisionsInvalid           []InteractionDecision  `json:"decisions_invalid"`
	SnapshotGoShaped           ExecutionSnapshot      `json:"snapshot_go_shaped"`
}

func loadMX003Fixture(t *testing.T) *mx003Fixture {
	t.Helper()
	raw, err := os.ReadFile("../../tests/mobile-v2/fixtures/mx-003-crosslang.json")
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}
	var fixture mx003Fixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal shared fixture: %v", err)
	}
	return &fixture
}

func TestMX003CrossLangEventValidation(t *testing.T) {
	fixture := loadMX003Fixture(t)
	for _, event := range fixture.EventsValid {
		if err := event.Validate(); err != nil {
			t.Errorf("valid event type=%s seq=%d must pass: %v", event.Type, event.Seq, err)
		}
	}
	// 未知事件 type 必须可保留：不丢弃、不改写（未知控制状态才被拒绝）
	unknownPreserved := false
	for _, event := range fixture.EventsValid {
		if event.Type == "future.event" {
			unknownPreserved = true
		}
	}
	if !unknownPreserved {
		t.Error("future.event must survive validation with its type preserved")
	}
	for _, event := range fixture.EventsInvalid {
		if err := event.Validate(); err == nil {
			t.Errorf("invalid event type=%s seq=%d must be rejected", event.Type, event.Seq)
		}
	}
}

func TestMX003CrossLangDecisions(t *testing.T) {
	fixture := loadMX003Fixture(t)
	for _, decision := range fixture.DecisionsValid {
		if err := decision.Validate(); err != nil {
			t.Errorf("valid decision %s must pass: %v", decision.ID, err)
		}
	}
	for _, decision := range fixture.DecisionsInvalid {
		if err := decision.Validate(); err == nil {
			t.Errorf("invalid decision %s must be rejected", decision.ID)
		}
	}
	// 域不相交：预算域不允许 approve（互换 payload 即拒绝）
	if ValidateInteractionAction("budget", "approve") == nil {
		t.Error("budget/approve must be rejected by the action matrix")
	}
}

func TestMX003CrossLangSnapshotAndCapability(t *testing.T) {
	fixture := loadMX003Fixture(t)
	if err := fixture.SnapshotGoShaped.Validate(); err != nil {
		t.Fatalf("go-shaped snapshot must validate: %v", err)
	}
	if err := fixture.ExecutionUnavailableCancel.Validate(); err != nil {
		t.Fatalf("execution with unavailable cancel must validate as a DTO: %v", err)
	}
	cancel := fixture.ExecutionUnavailableCancel.Capabilities["cancel"]
	if cancel.State != CapabilityUnavailable || strings.TrimSpace(cancel.Reason) == "" {
		t.Fatalf("cancel capability must be unavailable with a reason, got %+v", cancel)
	}
}
