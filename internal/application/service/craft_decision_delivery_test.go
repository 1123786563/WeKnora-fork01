package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The C02 reliable decision delivery worker. The fakes from the R06 control
// tests (fakeControlReplier, fakeControlRuns, fakeInteractionStore) are
// reused; the outbox double below records acks and unknowns.

type fakeOutbox struct {
	mu       sync.Mutex
	items    []CraftOutboxItem
	acks     []string
	unknowns []string
	notes    []string
	listing  int
}

func (f *fakeOutbox) PendingDecisions(_ context.Context, _ agentruntime.RunKey) ([]CraftOutboxItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listing++
	return append([]CraftOutboxItem(nil), f.items...), nil
}

func (f *fakeOutbox) AckDecision(_ context.Context, _ agentruntime.RunKey, interactionID, decisionID, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acks = append(f.acks, interactionID+":"+decisionID)
	f.notes = append(f.notes, note)
	f.items = removeOutboxItem(f.items, interactionID, decisionID)
	return nil
}

func (f *fakeOutbox) MarkDecisionUnknown(_ context.Context, _ agentruntime.RunKey, interactionID, decisionID, note string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.unknowns = append(f.unknowns, interactionID+":"+decisionID)
	f.notes = append(f.notes, note)
	for i := range f.items {
		if f.items[i].InteractionID == interactionID && f.items[i].DecisionID == decisionID {
			f.items[i].Delivery = "unknown"
		}
	}
	return nil
}

func removeOutboxItem(items []CraftOutboxItem, interactionID, decisionID string) []CraftOutboxItem {
	out := items[:0]
	for _, item := range items {
		if item.InteractionID != interactionID || item.DecisionID != decisionID {
			out = append(out, item)
		}
	}
	return out
}

func (f *fakeOutbox) calls() (acks, unknowns []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.acks...), append([]string(nil), f.unknowns...)
}

func deliveryKey() agentruntime.RunKey {
	return agentruntime.RunKey{TenantID: 1, RunID: "run-deliver"}
}

func deliveryItem(kind, action string) CraftOutboxItem {
	return CraftOutboxItem{
		TenantID: 1, OwnerID: "u1", SessionID: "s1",
		RunID: "run-deliver", InteractionID: "itx_d1", PendingID: "itx_d1",
		DecisionID: "dec_d1", Kind: kind, Action: action, ArgsHash: "iargs_d1", ExpectedRevision: 2,
		OpenCodeSessionID: "oc_d1", OpenCodeRequestID: "qst_d1", Delivery: "pending",
	}
}

func deliveryInteraction(kind, action string) CraftInteractionRecord {
	return CraftInteractionRecord{
		Interaction: craft.Interaction{
			ID: "itx_d1", Kind: kind, ArgsHash: "iargs_d1", Prompt: "decide", Revision: 3,
		},
		Scope:             controlScope(),
		RunID:             "run-deliver",
		PendingID:         "itx_d1",
		OpenCodeSessionID: "oc_d1",
		OpenCodeRequestID: "qst_d1",
		Status:            "decided",
		Delivery:          "pending",
		DecisionID:        "dec_d1",
		DecidedAction:     action,
	}
}

func newDeliveryFixture(t *testing.T, kind, action string) (*CraftDecisionDelivery, *fakeOutbox, *fakeInteractionStore, *fakeControlReplier, *fakeControlRuns) {
	t.Helper()
	outbox := &fakeOutbox{items: []CraftOutboxItem{deliveryItem(kind, action)}}
	interactions := newFakeInteractionStore()
	record := deliveryInteraction(kind, action)
	record.Revision = 3
	interactions.records[record.ID] = record
	replier := &fakeControlReplier{}
	runs := &fakeControlRuns{run: agentruntime.Run{
		Key: deliveryKey(), SessionID: "s1", UserID: "u1", Status: "waiting_user", Revision: 9, Epoch: 2,
	}}
	return NewCraftDecisionDelivery(interactions, outbox, replier, runs), outbox, interactions, replier, runs
}

// TestDeliveryForwardsPendingItemAndAcks: a pending item under the run fence
// is re-checked against the durable interaction, forwarded over the locked
// reply route, and acked exactly once on success.
func TestDeliveryForwardsPendingItemAndAcks(t *testing.T) {
	delivery, outbox, _, replier, _ := newDeliveryFixture(t, craft.InteractionQuestion, craft.DecisionAnswer)
	ctx := context.Background()
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""))
	require.Len(t, replier.calls(), 1, "the decision forwards exactly once")
	acks, unknowns := outbox.calls()
	require.Equal(t, []string{"itx_d1:dec_d1"}, acks)
	require.Empty(t, unknowns, "a confirmed forward never marks unknown")
}

// TestDeliveryMapsAnswersToLockedProtocol: a question answer maps its choices
// and custom text onto the locked answers:[[string]] route; a question reject
// maps onto the reject route; a permission maps onto once/reject only.
func TestDeliveryMapsAnswersToLockedProtocol(t *testing.T) {
	ctx := context.Background()

	delivery, outbox, _, replier, _ := newDeliveryFixture(t, craft.InteractionQuestion, craft.DecisionAnswer)
	withAnswers(t, delivery, outbox, []craft.Answer{{QuestionID: "q_color", Choices: []string{"red", "green"}, Text: "sans"}})
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""))
	require.Equal(t, []string{"question_reply:qst_d1:[[red green sans]]"}, replier.calls(),
		"choices and custom text map onto one answers row per question")

	delivery, _, _, replier, _ = newDeliveryFixture(t, craft.InteractionQuestion, craft.DecisionReject)
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""))
	require.Equal(t, []string{"question_reject:qst_d1"}, replier.calls())

	delivery, _, _, replier, _ = newDeliveryFixture(t, craft.InteractionPermission, craft.DecisionApprove)
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""))
	require.Equal(t, []string{"permission:oc_d1:qst_d1:once"}, replier.calls(),
		"the first phase approves exactly once — never a session grant")

	delivery, _, _, replier, _ = newDeliveryFixture(t, craft.InteractionPermission, craft.DecisionReject)
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""))
	require.Equal(t, []string{"permission:oc_d1:qst_d1:reject"}, replier.calls())
}

func withAnswers(t *testing.T, delivery *CraftDecisionDelivery, outbox *fakeOutbox, answers []craft.Answer) {
	t.Helper()
	outbox.mu.Lock()
	if len(outbox.items) > 0 {
		outbox.items[0].Answers = answers
	}
	outbox.mu.Unlock()
}

// TestDeliveryTimeoutRechecksStateAndStaysUnknown: when the forward errors
// (network timeout), the worker first re-reads the durable state — if another
// worker already acked, that result stands; otherwise the delivery stays
// honestly unknown, never fabricated as accepted.
func TestDeliveryTimeoutRechecksStateAndStaysUnknown(t *testing.T) {
	delivery, outbox, _, replier, runs := newDeliveryFixture(t, craft.InteractionQuestion, craft.DecisionAnswer)
	replier.err = errors.New("network timeout")
	runs.run.Status = "waiting_user"
	ctx := context.Background()

	err := delivery.Deliver(ctx, deliveryKey(), "")
	require.Error(t, err, "an unconfirmable delivery reports an error, never success")
	acks, unknowns := outbox.calls()
	require.Empty(t, acks, "a timed-out forward is never acked")
	require.Equal(t, []string{"itx_d1:dec_d1"}, unknowns)
	_, _, waits := runs.counts()
	require.Zero(t, waits, "the waiting park is never re-written by delivery")
	require.Equal(t, 0, strings.Index(err.Error(), ""), "error text preserved")
	require.Contains(t, err.Error(), "unconfirmed", "the error names the honest state")

	// Another worker acked while this attempt was in flight: the recheck sees
	// the confirmed state and a redelivery does not forward again.
	outbox.mu.Lock()
	outbox.items = nil
	outbox.mu.Unlock()
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""))
	require.Len(t, replier.calls(), 1, "no second forward after an external ack")
}

// TestDeliveryRefusesSupersededDecision: when the durable interaction no
// longer carries this decision (superseded, canceled or a changed argument
// digest), the item is NOT forwarded — a stale decision can never authorize a
// different payload.
func TestDeliveryRefusesSupersededDecision(t *testing.T) {
	delivery, outbox, interactions, replier, _ := newDeliveryFixture(t, craft.InteractionQuestion, craft.DecisionAnswer)
	superseded := deliveryInteraction(craft.InteractionQuestion, craft.DecisionAnswer)
	superseded.DecisionID = "dec_other"
	interactions.records[superseded.ID] = superseded

	err := delivery.Deliver(context.Background(), deliveryKey(), "")
	require.Error(t, err)
	acks, _ := outbox.calls()
	require.Empty(t, acks)
	require.Empty(t, replier.calls(), "a superseded decision is never forwarded")
}

// TestDeliveryTargetsPendingFilterAndGoneRun: the plain sweep only takes
// pending items (unknown requires an explicit target), and a terminal run
// stops the sweep without forwarding into a dead execution.
func TestDeliveryTargetsPendingFilterAndGoneRun(t *testing.T) {
	delivery, outbox, _, replier, runs := newDeliveryFixture(t, craft.InteractionQuestion, craft.DecisionAnswer)
	outbox.mu.Lock()
	outbox.items[0].Delivery = "unknown"
	outbox.mu.Unlock()
	ctx := context.Background()
	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), ""), "unknown items are not auto-swept")
	require.Empty(t, replier.calls())

	require.NoError(t, delivery.Deliver(ctx, deliveryKey(), "itx_d1"), "a targeted redelivery retries the unknown item")
	require.Len(t, replier.calls(), 1)

	// A terminal run parks the sweep: the decision stays durable, nothing is
	// forwarded into the dead execution.
	outbox.mu.Lock()
	outbox.items = []CraftOutboxItem{deliveryItem(craft.InteractionQuestion, craft.DecisionAnswer)}
	outbox.mu.Unlock()
	runs.run.Status = "canceled"
	err := delivery.Deliver(ctx, deliveryKey(), "")
	require.Error(t, err)
	require.Len(t, replier.calls(), 1, "no forward after the run terminated")
}

var _ = fmt.Sprint

// ---- C02 process-failure matrix (re-exec of this test binary) ---------------
//
// The provider runs the SAME compiled production packages against a durable
// migrated SQLite database; only the OpenCode serve is a deterministic
// in-parent HTTP double that survives every child kill — exactly the "external
// OC stays alive" premise. Two crash windows are exercised:
//
//   decision_saved_kill  — the decision + outbox are committed, no forward;
//   oc_accepted_no_ack   — OpenCode already accepted the reply, the ack is
//                          not persisted yet.
//
// After SIGKILL + restart the delivery worker must deliver exactly the same
// durable decision: same decision id, one outbox row, the recorded answers
// intact for re-display, and never a duplicate new decision.

const (
	decisionProcCaseSaved     = "decision_saved_kill"
	decisionProcCaseAccepted  = "oc_accepted_no_ack"
	decisionProcRunID         = "craft-dec-run"
	decisionProcInteractionID = "itx_decision_proc"
	decisionProcRequestID     = "qst_decision_proc"
	decisionProcDecisionID    = "dec_decision_proc"
)

// decisionFakeOpenCode is the in-parent OpenCode double: it counts and accepts
// question replies (idempotently) and survives child kills.
type decisionFakeOpenCode struct {
	mu      sync.Mutex
	replies int
}

func (f *decisionFakeOpenCode) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.replies
}

func (f *decisionFakeOpenCode) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/question/{id}/reply", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.replies++
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("true"))
	})
	return mux
}

type decisionProcReport struct {
	Case              string `json:"case"`
	Delivered         bool   `json:"delivered"`
	OcReplies         int    `json:"oc_replies"`
	OutboxRows        int    `json:"outbox_rows"`
	InteractionStatus string `json:"interaction_status"`
	Delivery          string `json:"delivery"`
	DecisionID        string `json:"decision_id"`
	AnswersPreserved  bool   `json:"answers_preserved"`
	Error             string `json:"error,omitempty"`
}

func decisionProcScope() craft.Scope {
	return craft.Scope{TenantID: 1, UserID: "craft-user", SessionID: craftProcSessionID}
}

// decisionProviderCrash runs the pre-kill phase: a live run parked at
// waiting_user, a registered pending question, then the case-specific crash
// window. The parent SIGKILLs at the barrier.
func decisionProviderCrash(ctx context.Context, caseName string, db *gorm.DB, ocURL, barrier string) {
	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "craft decision provider: "+format+"\n", args...)
		os.Exit(2)
	}
	runs := repository.NewAgentRunStore(db)
	key := agentruntime.RunKey{TenantID: 1, RunID: decisionProcRunID}
	user, _ := json.Marshal(map[string]any{"role": "user", "content": "delegate"})
	assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": ""})
	if _, err := runs.Admit(ctx, agentruntime.Admission{
		Key: key, SessionID: craftProcSessionID, UserID: "craft-user", RequestID: "req-dec",
		AssistantMessageID: "asst-dec", RequestHash: "rh-dec",
		Snapshot: json.RawMessage(`{"version":1,"craft":true}`), UserMessage: user, AssistantMessage: assistant,
		Deadline: time.Now().Add(10 * time.Minute),
	}); err != nil {
		fail("admit: %v", err)
	}
	fence, err := runs.Claim(ctx, key, "worker-dec", 5*time.Minute)
	if err != nil {
		fail("claim: %v", err)
	}
	store := NewGormCraftInteractionStore(db)
	pending := craft.PendingDecision{
		Interaction: craft.Interaction{
			ID: decisionProcInteractionID, Kind: craft.InteractionQuestion,
			ArgsHash: "iargs_proc", Prompt: "which layout?",
		},
		Options:  map[string][]string{"q_layout": {"grid", "list"}},
		Multiple: map[string]bool{},
	}
	registered, err := store.PutInteraction(ctx, CraftInteractionRecord{
		Interaction:       pending.Interaction,
		Scope:             decisionProcScope(),
		RunID:             decisionProcRunID,
		ToolCallID:        "call-dec",
		PendingID:         decisionProcInteractionID,
		OpenCodeSessionID: craftProcOCSession,
		OpenCodeRequestID: decisionProcRequestID,
		Pending:           pending,
	})
	if err != nil {
		fail("register interaction: %v", err)
	}
	// Park the main run at waiting_user exactly like the registration path.
	if err := runs.SetStatus(ctx, fence, "waiting_user", registered.PendingID); err != nil {
		fail("park run: %v", err)
	}
	// The durable decision + outbox land in one transaction.
	if _, err := store.ApplyInteractionDecision(ctx, CraftDecisionRequest{
		Scope: decisionProcScope(), InteractionID: decisionProcInteractionID, DecisionID: decisionProcDecisionID,
		Action: craft.DecisionAnswer, ArgsHash: "iargs_proc", ExpectedRevision: 1,
		Answers:  []craft.Answer{{QuestionID: "q_layout", Choices: []string{"grid"}}},
		Operator: "craft-user",
	}); err != nil {
		fail("apply decision: %v", err)
	}
	if caseName == decisionProcCaseSaved {
		touchCraftBarrier(barrier)
		select {}
	}
	// oc_accepted_no_ack: forward through the real client; OpenCode accepts,
	// then the process dies before the ack is persisted.
	client, cerr := opencode.NewClient(ocURL, nil)
	if cerr != nil {
		fail("client: %v", cerr)
	}
	if err := client.ReplyQuestion(ctx, decisionProcRequestID, [][]string{{"grid"}}); err != nil {
		fail("forward: %v", err)
	}
	touchCraftBarrier(barrier)
	select {}
}

// decisionProviderResume runs after the kill: redeliver, then report the
// durable truth.
func decisionProviderResume(ctx context.Context, db *gorm.DB, ocURL, reportPath string) decisionProcReport {
	report := decisionProcReport{Case: os.Getenv("CRAFT_DECISION_PROVIDER_CASE")}
	store := NewGormCraftInteractionStore(db)
	runs := repository.NewAgentRunStore(db)
	client, cerr := opencode.NewClient(ocURL, nil)
	if cerr != nil {
		report.Error = cerr.Error()
		return report
	}
	delivery := NewCraftDecisionDelivery(store, store, client, NewAgentRunService(runs))
	key := agentruntime.RunKey{TenantID: 1, RunID: decisionProcRunID}
	if err := delivery.Deliver(ctx, key, ""); err != nil {
		report.Error = err.Error()
	}
	var outboxRows int64
	_ = db.Table("craft_decision_outbox").Where("tenant_id = ? AND run_id = ?", 1, decisionProcRunID).Count(&outboxRows).Error
	report.OutboxRows = int(outboxRows)
	record, err := store.GetInteraction(ctx, decisionProcScope(), decisionProcInteractionID)
	if err != nil {
		report.Error = fmt.Sprintf("re-read interaction: %v", err)
		return report
	}
	report.InteractionStatus = record.Status
	report.Delivery = record.Delivery
	report.DecisionID = record.DecisionID
	report.AnswersPreserved = len(record.RecordedAnswers) == 1 &&
		record.RecordedAnswers[0].QuestionID == "q_layout" &&
		len(record.RecordedAnswers[0].Choices) == 1 && record.RecordedAnswers[0].Choices[0] == "grid"
	report.Delivered = record.Delivery == "delivered"
	raw, _ := json.Marshal(report)
	if reportPath != "" {
		_ = os.WriteFile(reportPath, append(raw, '\n'), 0o644)
	}
	fmt.Println(string(raw))
	return report
}

func craftDecisionProviderMain() {
	caseName := os.Getenv("CRAFT_DECISION_PROVIDER_CASE")
	phase := os.Getenv("CRAFT_DECISION_PROVIDER_PHASE")
	dbPath := os.Getenv("CRAFT_DECISION_DB")
	ocURL := os.Getenv("CRAFT_DECISION_OC_URL")
	barrier := os.Getenv("CRAFT_DECISION_BARRIER")
	reportPath := os.Getenv("CRAFT_DECISION_REPORT")
	if caseName == "" || dbPath == "" || ocURL == "" {
		fmt.Fprintln(os.Stderr, "craft decision provider: missing env")
		os.Exit(2)
	}
	db, err := openCraftRecoveryProviderDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "craft decision provider: open db: %v\n", err)
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if phase != "resume" {
		decisionProviderCrash(ctx, caseName, db, ocURL, barrier)
		return // unreachable; the parent SIGKILLs at the barrier
	}
	decisionProviderResume(ctx, db, ocURL, reportPath)
}

// runDecisionProvider executes one provider phase in a child process.
func runDecisionProvider(t *testing.T, ocURL, dbPath string, extraEnv map[string]string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestNothingMatches")
	cmd.Env = append(os.Environ(), "CRAFT_DECISION_DB="+dbPath, "CRAFT_DECISION_OC_URL="+ocURL)
	for k, v := range extraEnv {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return cmd
}

func waitForDecisionBarrier(t *testing.T, barrier string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(barrier); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("provider barrier %s never appeared", barrier)
}

func decisionReportOf(t *testing.T, out []byte) decisionProcReport {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.NotEmpty(t, lines)
	var report decisionProcReport
	require.NoError(t, json.Unmarshal([]byte(lines[len(lines)-1]), &report), "last output line must be the report JSON: %s", string(out))
	return report
}

// TestDecisionSurvivesKillAfterSavedDecision: the decision and its outbox are
// durable when the process dies before any forward; after restart the SAME
// pending decision delivers once and the original answers are intact.
func TestDecisionSurvivesKillAfterSavedDecision(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "decision.db")
	dir := t.TempDir()
	oc := &decisionFakeOpenCode{}
	server := httptest.NewServer(oc.handler())
	defer server.Close()
	barrier := filepath.Join(dir, "barrier")

	crash := runDecisionProvider(t, server.URL, dbPath, map[string]string{
		"CRAFT_DECISION_PROVIDER_CASE":  decisionProcCaseSaved,
		"CRAFT_DECISION_PROVIDER_PHASE": "crash",
		"CRAFT_DECISION_BARRIER":        barrier,
	})
	require.NoError(t, crash.Start())
	waitForDecisionBarrier(t, barrier, 30*time.Second)
	// The crash phase parks at the barrier; the kill (or the parking deadlock
	// itself) ends it before any ack is written.
	if err := crash.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("kill crash provider: %v", err)
	}
	_ = crash.Wait()

	resume := runDecisionProvider(t, server.URL, dbPath, map[string]string{
		"CRAFT_DECISION_PROVIDER_CASE":  decisionProcCaseSaved,
		"CRAFT_DECISION_PROVIDER_PHASE": "resume",
		"CRAFT_DECISION_REPORT":         filepath.Join(dir, "report.json"),
	})
	out, err := resume.CombinedOutput()
	require.NoError(t, err, "resume failed: %s", string(out))

	report := decisionReportOf(t, out)
	require.True(t, report.Delivered, "the redelivery must confirm after restart")
	require.Equal(t, 1, oc.count(), "exactly one OpenCode reply after the restart")
	require.Equal(t, 1, report.OutboxRows, "no duplicate outbox item")
	require.Equal(t, decisionProcDecisionID, report.DecisionID, "the same decision id survives")
	require.True(t, report.AnswersPreserved, "the original answers survive for re-display")
	require.Equal(t, "decided", report.InteractionStatus)
}

// TestDecisionNoDuplicateAfterAcceptedButUnackedKill: OpenCode already
// accepted the reply when the process died before the ack; the restart
// redelivers the SAME decision (bounded idempotent re-reply) and never creates
// a duplicate new decision.
func TestDecisionNoDuplicateAfterAcceptedButUnackedKill(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "decision.db")
	dir := t.TempDir()
	oc := &decisionFakeOpenCode{}
	server := httptest.NewServer(oc.handler())
	defer server.Close()
	barrier := filepath.Join(dir, "barrier")

	crash := runDecisionProvider(t, server.URL, dbPath, map[string]string{
		"CRAFT_DECISION_PROVIDER_CASE":  decisionProcCaseAccepted,
		"CRAFT_DECISION_PROVIDER_PHASE": "crash",
		"CRAFT_DECISION_BARRIER":        barrier,
	})
	require.NoError(t, crash.Start())
	waitForDecisionBarrier(t, barrier, 30*time.Second)
	if err := crash.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Fatalf("kill crash provider: %v", err)
	}
	_ = crash.Wait()
	require.Equal(t, 1, oc.count(), "OpenCode accepted the reply exactly once before the kill")

	resume := runDecisionProvider(t, server.URL, dbPath, map[string]string{
		"CRAFT_DECISION_PROVIDER_CASE":  decisionProcCaseAccepted,
		"CRAFT_DECISION_PROVIDER_PHASE": "resume",
		"CRAFT_DECISION_REPORT":         filepath.Join(dir, "report.json"),
	})
	out, err := resume.CombinedOutput()
	require.NoError(t, err, "resume failed: %s", string(out))

	report := decisionReportOf(t, out)
	require.True(t, report.Delivered)
	require.Equal(t, 2, oc.count(), "the redelivery re-replies at most once more (idempotent)")
	require.Equal(t, 1, report.OutboxRows, "never a duplicate new decision")
	require.Equal(t, decisionProcDecisionID, report.DecisionID)
	require.True(t, report.AnswersPreserved)
}
