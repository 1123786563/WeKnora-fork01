package opencode

// CFT-S02-T015: unknown outcomes keep their honesty. Four pinned assertions
// through the runtime fake with fault injection:
//  1. the remote accepted the prompt but the read side timed out — a
//     RE-ENTERED Execute must NOT POST the prompt again (the pre-snapshot
//     sees promptSeen and skips submission)
//  2. a broken stream is never interpreted as success
//  3. unknown keeps the main tool call pending (no stored result, ErrUnknown)
//  4. a delegation whose reconcile deadline already passed reports a
//     DIAGNOSTIC unknown naming the deadline — never a guessed outcome
import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/craft"
)

func TestUnknownAcceptanceRemoteAcceptedReadTimeoutNoSecondPost(t *testing.T) {
	f := newRuntimeFake(t)
	// The prompt POST's response is lost (hijacked) but the remote ACCEPTED
	// it: the message list shows the user entry, the assistant is unfinished.
	f.hijackPrompt = true
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build the deck"),
			assistantEntry("msg_asst", promptID, 0, "", ""))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	// A client with a REAL read timeout: the hung GET /message must fail as
	// a read timeout (bounded), not hang until the delegation deadline.
	timeoutClient, clientErr := NewClient(f.server.URL, &http.Client{Timeout: 800 * time.Millisecond})
	if clientErr != nil {
		t.Fatalf("NewClient: %v", clientErr)
	}
	executor := NewExecutor(timeoutClient, store, recorder.emit)

	// One identity across every re-entry: same tool call, same payload hash
	// AND the same deadline (the store's conflict rule compares them all).
	deadline := time.Now().Add(20 * time.Second)
	first := runExecute(executor, delegationTask(deadline))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("the prompt was never submitted")
	}
	f.cutStream()
	outcome := awaitOutcome(t, first)
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("first outcome = %#v, err = %v", outcome.result, outcome.err)
	}

	// The read side now times out against a still-running remote: the
	// snapshot GET hangs, so the re-entered Execute cannot prove anything.
	// Re-entry REUSES the persisted identity (same tool call AND the prompt
	// message id the first pass stored) — a fresh delegationTask() would
	// legitimately conflict (same key, different payload).
	f.setHangMessages(true)
	reenter := delegationTask(deadline)
	reenter.PromptMessageID = store.lastPreparedPromptID()
	second := runExecute(executor, reenter)
	retried := awaitOutcome(t, second)
	if retried.result.Status != "unknown" || !errors.Is(retried.err, craft.ErrUnknown) {
		t.Fatalf("second outcome = %#v, err = %v", retried.result, retried.err)
	}
	f.setHangMessages(false)

	// A third re-entry with a READABLE snapshot sees the remote still
	// working on the SAME prompt: still exactly ONE prompt POST — the
	// side-effect submission count is pinned by injection.
	reenter3 := delegationTask(deadline)
	reenter3.PromptMessageID = store.lastPreparedPromptID()
	third := runExecute(executor, reenter3)
	thirdOutcome := awaitOutcome(t, third)
	if thirdOutcome.result.Status != "unknown" || !errors.Is(thirdOutcome.err, craft.ErrUnknown) {
		t.Fatalf("third outcome = %#v, err = %v", thirdOutcome.result, thirdOutcome.err)
	}
	if posts := f.promptCount(); posts != 1 {
		t.Fatalf("prompt POSTs = %d (want exactly 1 across all re-entries)", posts)
	}
	if saves := store.savedStatuses(); len(saves) != 0 {
		t.Fatalf("unknown outcomes must store no result: %v", saves)
	}
}

func TestUnknownAcceptanceStreamBreakIsNeverSuccess(t *testing.T) {
	// Pinned restatement (companion of TestExecuteStreamBreakWithRemoteStill
	// RunningIsUnknown): a cut stream with an unfinished assistant ends
	// unknown, and nothing is stored.
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build"),
			assistantEntry("msg_asst", promptID, 0, "", ""))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	done := runExecute(executor, delegationTask(time.Now().Add(5*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("prompt never submitted")
	}
	f.cutStream()
	outcome := awaitOutcome(t, done)
	if outcome.result.Status == "succeeded" {
		t.Fatal("a broken stream must never settle as succeeded")
	}
	if !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("err = %v (want ErrUnknown)", outcome.err)
	}
	if len(store.savedStatuses()) != 0 {
		t.Fatal("no result may be stored for an unknown")
	}
}

func TestUnknownAcceptancePassedDeadlineReportsDiagnostics(t *testing.T) {
	f := newRuntimeFake(t)
	store := newMemStore()
	store.workspace = craftWorkspace()
	executor := newExecutorFor(t, f, store, &emitRecorder{})
	// The reconcile deadline is already in the past: the executor must say so
	// as an unknown with the deadline IN the summary — a diagnostic state,
	// not a guessed outcome and not a silent resubmission.
	outcome := awaitOutcome(t, runExecute(executor, delegationTask(time.Now().Add(-time.Minute))))
	if outcome.result.Status != "unknown" || !errors.Is(outcome.err, craft.ErrUnknown) {
		t.Fatalf("outcome = %#v, err = %v", outcome.result, outcome.err)
	}
	if !strings.Contains(outcome.result.Summary, "deadline") {
		t.Fatalf("summary lacks the deadline diagnostic: %q", outcome.result.Summary)
	}
	if f.promptCount() != 0 {
		t.Fatalf("an expired delegation must not submit side effects (posts = %d)", f.promptCount())
	}
}
