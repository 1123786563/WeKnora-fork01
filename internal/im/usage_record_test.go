package im

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// stubIMUsageRecorder captures RecordChatTurn calls from recordIMChatUsage.
type stubIMUsageRecorder struct {
	calls []stubIMUsageRecordCall
	err   error
}

type stubIMUsageRecordCall struct {
	tenantID uint64
	userID   string
	model    string
	usage    *types.TokenUsage
}

func (s *stubIMUsageRecorder) RecordChatTurn(_ context.Context, tenantID uint64, userID, model string, usage *types.TokenUsage) error {
	s.calls = append(s.calls, stubIMUsageRecordCall{tenantID: tenantID, userID: userID, model: model, usage: usage})
	if s.err != nil {
		return s.err
	}
	return nil
}

// imUsageFixture wires a Service literal the way the completion points use it:
// IM sessions are auto-created without a WeKnora owner, identity comes from
// the PrincipalIMUser principal injected by withIMIdentity, and agent-mode
// turns carry the custom agent's model binding (the IM assistant payload sets
// no ModelID of its own).
func imUsageFixture() (*Service, *stubIMUsageRecorder, *types.Message, *types.Session, *types.CustomAgent, context.Context) {
	rec := &stubIMUsageRecorder{}
	svc := &Service{usageRecorder: rec}
	msg := &types.Message{
		ID:        "am-im-1",
		SessionID: "sess-im-1",
		Usage:     &types.TokenUsage{PromptTokens: 120, CompletionTokens: 30},
	}
	session := &types.Session{ID: "sess-im-1", TenantID: 7}
	agent := &types.CustomAgent{Config: types.CustomAgentConfig{ModelID: "glm-4.7"}}
	ctx := withIMIdentity(context.Background(), 7, "ch-1", &IncomingMessage{Platform: "wecom", UserID: "wx-u1"})
	return svc, rec, msg, session, agent, ctx
}

func TestRecordIMChatUsageAttributesToIMPrincipal(t *testing.T) {
	svc, rec, msg, session, agent, ctx := imUsageFixture()

	svc.recordIMChatUsage(ctx, msg, session, agent)

	if len(rec.calls) != 1 {
		t.Fatalf("RecordChatTurn calls = %d, want 1", len(rec.calls))
	}
	got := rec.calls[0]
	if got.tenantID != 7 {
		t.Errorf("tenantID = %d, want 7 (session tenant)", got.tenantID)
	}
	if want := "7:ch-1:wecom:wx-u1"; got.userID != want {
		t.Errorf("userID = %q, want the IM principal %q", got.userID, want)
	}
	if got.model != "glm-4.7" {
		t.Errorf("model = %q, want the custom agent binding glm-4.7", got.model)
	}
	if got.usage != msg.Usage {
		t.Errorf("usage pointer mismatch: recorded %v, message %v", got.usage, msg.Usage)
	}
	if msg.ModelID != "glm-4.7" {
		t.Errorf("message ModelID = %q, want the resolved binding stamped for persistence", msg.ModelID)
	}
}

func TestRecordIMChatUsageIsOneShotPerMessage(t *testing.T) {
	svc, rec, msg, session, agent, ctx := imUsageFixture()

	svc.recordIMChatUsage(ctx, msg, session, agent)
	svc.recordIMChatUsage(ctx, msg, session, agent)

	if len(rec.calls) != 1 {
		t.Fatalf("RecordChatTurn calls = %d, want 1 (completion paths race on one message)", len(rec.calls))
	}
}

func TestRecordIMChatUsageSkipsWithoutUsageRecorderOrModel(t *testing.T) {
	msg := &types.Message{ID: "am-1", SessionID: "s", Usage: &types.TokenUsage{PromptTokens: 1}}
	session := &types.Session{ID: "s", TenantID: 1}
	ctx := context.Background()

	// No usage block on the message: nothing to account.
	svc, rec, m2, _, agent, _ := imUsageFixture()
	m2.Usage = nil
	svc.recordIMChatUsage(ctx, m2, session, agent)
	if len(rec.calls) != 0 {
		t.Fatalf("calls with nil usage = %d, want 0", len(rec.calls))
	}

	// No recorder wired (tests / Lite): skip silently.
	bare := &Service{}
	bare.recordIMChatUsage(ctx, msg, session, nil)
	if len(rec.calls) != 0 {
		t.Fatalf("calls without recorder = %d, want 0", len(rec.calls))
	}

	// No model resolvable (no message ModelID, no agent binding): the recorder
	// skips rather than guessing — mirrors RecordChatTurn's empty-model guard.
	svc2, rec2, m3, _, _, ctx2 := imUsageFixture()
	svc2.recordIMChatUsage(ctx2, m3, session, nil)
	if len(rec2.calls) != 0 {
		t.Fatalf("calls without model = %d, want 0", len(rec2.calls))
	}
	if m3.ModelID != "" {
		t.Errorf("ModelID stamped as %q, want untouched when unresolvable", m3.ModelID)
	}
}

func TestRecordIMChatUsageFallsBackToSessionOwnerWithoutPrincipal(t *testing.T) {
	svc, rec, msg, session, agent, _ := imUsageFixture()
	session.UserID = "weknora-owner"
	msg.Usage = &types.TokenUsage{PromptTokens: 5}

	// Plain context (no IM principal): a session with a real owner attributes
	// to it; principal-less and ownerless sessions record nothing.
	svc.recordIMChatUsage(context.Background(), msg, session, agent)
	if len(rec.calls) != 1 || rec.calls[0].userID != "weknora-owner" {
		t.Fatalf("calls = %+v, want one call attributed to weknora-owner", rec.calls)
	}

	svc2, rec2, m2, s2, agent2, _ := imUsageFixture()
	svc2.recordIMChatUsage(context.Background(), m2, s2, agent2)
	if len(rec2.calls) != 0 {
		t.Fatalf("calls for ownerless principal-less session = %d, want 0", len(rec2.calls))
	}
}

func TestRecordIMChatUsageRecordsDespiteCancelledContext(t *testing.T) {
	svc, rec, msg, session, agent, ctx := imUsageFixture()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()

	// /stop cancels the turn's context after usage landed; the WithoutCancel
	// recorder context must still carry the accounting through.
	svc.recordIMChatUsage(cancelled, msg, session, agent)
	if len(rec.calls) != 1 {
		t.Fatalf("RecordChatTurn calls with cancelled ctx = %d, want 1", len(rec.calls))
	}
}

func TestRecordIMChatUsageRecorderErrorIsFailSoft(t *testing.T) {
	svc, rec, msg, session, agent, ctx := imUsageFixture()
	rec.err = context.DeadlineExceeded

	// A repository failure is only logged; the gate is consumed by the first
	// attempt, so a retried completion must not re-record the turn.
	svc.recordIMChatUsage(ctx, msg, session, agent)
	svc.recordIMChatUsage(ctx, msg, session, agent)

	if len(rec.calls) != 1 {
		t.Fatalf("RecordChatTurn attempts = %d, want 1 (fail-soft, gate consumed)", len(rec.calls))
	}
}
