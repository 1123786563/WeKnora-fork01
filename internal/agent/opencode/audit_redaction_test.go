package opencode

// CFT-S05-T032: audit surface redaction. The events this executor emits are
// what the workbench and the operations view correlate a failed task
// through. Their payloads must be STRUCTURAL: every event's keys come from a
// closed per-kind vocabulary — system credentials, preview tickets and raw
// tool inputs/outputs never appear because no code path puts them there. The
// deliverable text IS the user's artifact (it legitimately streams); the
// guarantees are the closed key vocabulary and the byte bound.
import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The structural guarantee: every emitted payload's keys come from the
// closed per-kind vocabulary, and the deliverable text is BOUNDED.
func TestCraftAuditRedactionPayloadVocabularyIsClosed(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(id string) string {
		if id == "" {
			return "[]"
		}
		return messagesJSON(t,
			userEntry(id, "goal"),
			assistantEntry("msg_asst", id, 3, "stop", "", textPart("prt_a", "msg_asst", strings.Repeat("交付文本", 200))))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)

	done := runExecute(executor, delegationTask(time.Now().Add(6*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("prompt never submitted")
	}
	f.setStatus("idle")
	f.push(t, "message.updated", map[string]any{"sessionID": "ses_oc", "info": map[string]any{
		"id": "msg_asst", "parentID": f.currentPromptID(), "role": "assistant", "finish": "stop",
		"time": map[string]any{"created": 2, "completed": 3}}})
	f.push(t, "session.idle", map[string]any{"sessionID": "ses_oc"})
	outcome := awaitOutcome(t, done)
	if outcome.err != nil || outcome.result.Status != "succeeded" {
		t.Fatalf("outcome = %#v err = %v", outcome.result, outcome.err)
	}

	allowed := map[string]map[string]bool{
		"delegation.started":  {"prompt_message_id": true},
		"delegation.text":     {"text": true},
		"delegation.finished": {"status": true},
	}
	seen := map[string]bool{}
	for _, kind := range recorder.kinds {
		seen[kind] = true
		vocab, ok := allowed[kind]
		if !ok {
			t.Fatalf("unexpected emitted event kind %q", kind)
		}
		for _, payload := range recorder.payloadsOf(kind) {
			if err := assertPayloadKeys(payload, vocab); err != nil {
				t.Fatalf("%s payload: %v (payload %s)", kind, err, payload)
			}
		}
	}
	for _, need := range []string{"delegation.started", "delegation.text", "delegation.finished"} {
		if !seen[need] {
			t.Fatalf("missing correlation event %q", need)
		}
	}
	// The deliverable text is BOUNDED: an 800-rune answer must not travel whole.
	for _, payload := range recorder.payloadsOf("delegation.text") {
		if len(payload) > maxEmitTextBytes+64 { // JSON wrapper allowance
			t.Fatalf("delegation.text payload exceeds the bound: %d bytes", len(payload))
		}
	}
}

// An unknown delegation still emits its started phase (correlatable from the
// UI down to the executor) but never a finished event.
func TestCraftAuditRedactionUnknownIsLocatable(t *testing.T) {
	f := newRuntimeFake(t)
	f.buildMessages = func(promptID string) string {
		if promptID == "" {
			return "[]"
		}
		return messagesJSON(t, userEntry(promptID, "build"), assistantEntry("msg_asst", promptID, 0, "", ""))
	}
	store := newMemStore()
	store.workspace = craftWorkspace()
	recorder := &emitRecorder{}
	executor := newExecutorFor(t, f, store, recorder)
	done := runExecute(executor, delegationTask(time.Now().Add(6*time.Second)))
	if !waitFor(t, func() bool { return f.promptCount() == 1 }) {
		t.Fatal("prompt never submitted")
	}
	f.cutStream()
	outcome := awaitOutcome(t, done)
	if outcome.result.Status != "unknown" || outcome.err == nil {
		t.Fatalf("outcome = %#v err = %v", outcome.result, outcome.err)
	}
	if recorder.count("delegation.started") == 0 {
		t.Fatal("the unknown run still emits its started phase for correlation")
	}
	if recorder.count("delegation.finished") != 0 {
		t.Fatal("an unknown outcome must NOT emit a finished event")
	}
}

func assertPayloadKeys(payload string, allowed map[string]bool) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &object); err != nil {
		return err
	}
	for key := range object {
		if !allowed[key] {
			return &keyError{key: key}
		}
	}
	return nil
}

type keyError struct{ key string }

func (e *keyError) Error() string { return "payload key not in the closed vocabulary: " + e.key }
