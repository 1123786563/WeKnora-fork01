package opencode

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
)

func TestCompletedRequiresExactPrompt(t *testing.T) {
	o := craft.Observation{SessionID: "s", PromptMessageID: "p2", AssistantParentID: "p1", Completed: true, Idle: true, Finish: "stop"}
	if Completed(o) {
		t.Fatal("old turn accepted")
	}
	o.AssistantParentID = "p2"
	o.PendingTool = true
	if Completed(o) {
		t.Fatal("pending tool accepted")
	}
	o.PendingTool = false
	if !Completed(o) {
		t.Fatal("valid terminal rejected")
	}
}

func TestCompletedRejectsEveryIncompleteObservation(t *testing.T) {
	valid := craft.Observation{
		SessionID: "s", PromptMessageID: "p", AssistantParentID: "p",
		Completed: true, Idle: true, Finish: "stop",
	}
	mutations := map[string]func(*craft.Observation){
		"empty session":        func(o *craft.Observation) { o.SessionID = "" },
		"empty prompt":         func(o *craft.Observation) { o.PromptMessageID = "" },
		"not completed":        func(o *craft.Observation) { o.Completed = false },
		"not idle":             func(o *craft.Observation) { o.Idle = false },
		"finish length":        func(o *craft.Observation) { o.Finish = "length" },
		"finish error":         func(o *craft.Observation) { o.Finish = "error" },
		"aborted":              func(o *craft.Observation) { o.Aborted = true },
		"pending tool":         func(o *craft.Observation) { o.PendingTool = true },
		"other turn assistant": func(o *craft.Observation) { o.AssistantParentID = "other" },
	}
	for name, mutate := range mutations {
		o := valid
		mutate(&o)
		if Completed(o) {
			t.Fatalf("%s: accepted as completed", name)
		}
	}
}

func TestMessageIDTimeMSRoundTrip(t *testing.T) {
	now := time.Now().UnixMilli()
	id, err := newMessageIDAt(now)
	if err != nil {
		t.Fatal(err)
	}
	stamp, ok := messageIDTimeMS(id)
	// The 48-bit packing keeps only the low 36 bits of the timestamp (the
	// ~795-day wrap), so the decode returns now modulo 2^36 milliseconds.
	if !ok || stamp != now%messageIDWrapMS {
		t.Fatalf("messageIDTimeMS(%q) = %d, %v; want %d", id, stamp, ok, now%messageIDWrapMS)
	}
	for _, bad := range []string{"", "msg_short", "msg_zzzzzzzzzzzzAAAAAAAAAAAAAA", "ses_0019a468fdc1ABCDEFGHIJKLMN"} {
		if _, ok := messageIDTimeMS(bad); ok {
			t.Fatalf("messageIDTimeMS(%q) accepted a malformed id", bad)
		}
	}
}

func TestMessageIDReusableAtRejectsCrossWindowIDs(t *testing.T) {
	now := time.Now()
	fresh, err := newMessageIDAt(now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if !messageIDReusableAt(fresh, now) {
		t.Fatal("a just-minted id must be reusable")
	}
	stale, err := newMessageIDAt(now.Add(-48 * time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if messageIDReusableAt(stale, now) {
		t.Fatal("an id from outside the reuse window was accepted")
	}
	future, err := newMessageIDAt(now.Add(48 * time.Hour).UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if messageIDReusableAt(future, now) {
		t.Fatal("an id minted in the future was accepted")
	}
	if messageIDReusableAt("not-a-message-id", now) {
		t.Fatal("a malformed id was accepted")
	}
}

func TestScanEventFramesParsesLockedDialect(t *testing.T) {
	payload := ": heartbeat\n\n" +
		"id: evt_1\n" +
		"data: {\"id\":\"evt_1\",\n" +
		"data: \"type\":\"session.idle\",\"properties\":{\"sessionID\":\"s\"}}\n\n" +
		"retry: 3000\n" +
		"data: not-json\n\n" +
		"data: {\"id\":\"evt_2\",\"type\":\"x\",\"properties\":{}}"
	var seen []string
	err := scanEventFrames(strings.NewReader(payload), func(frame eventFrame) bool {
		seen = append(seen, frame.Type)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != "session.idle" || seen[1] != "x" {
		t.Fatalf("frames = %v", seen)
	}
}

func TestScanEventFramesStopsEarlyAndBoundsFrames(t *testing.T) {
	var calls int
	frame := "data: {\"id\":\"e\",\"type\":\"t\",\"properties\":{}}\n\n"
	err := scanEventFrames(strings.NewReader(frame+frame), func(frame eventFrame) bool {
		calls++
		return false
	})
	if err != nil || calls != 1 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
	big := "data: " + strings.Repeat("x", maxFrameDataBytes+1) + "\n\n"
	if err := scanEventFrames(strings.NewReader(big), func(frame eventFrame) bool { return true }); !errors.Is(err, errEventFrameTooLarge) {
		t.Fatalf("oversized frame err = %v", err)
	}
}

// frameOf builds one normalized frame from arbitrary properties, keeping
// the tests free of raw string literals.
func frameOf(t *testing.T, typ string, properties any) eventFrame {
	t.Helper()
	raw, err := json.Marshal(properties)
	if err != nil {
		t.Fatal(err)
	}
	return eventFrame{ID: "evt_" + typ, Type: typ, Properties: raw}
}

func TestSubStateTracksOnlyTheBoundSessionAndTurn(t *testing.T) {
	state := newSubState("ses_oc", "msg_prompt", nil)
	state.apply(frameOf(t, "session.idle", map[string]any{"sessionID": "ses_other"}))
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_other",
		"info":      map[string]any{"id": "msg_x", "parentID": "msg_prompt", "role": "assistant"},
	}))
	if state.idle || len(state.assistants) != 0 {
		t.Fatal("another session polluted the state")
	}
	// Same session, another turn: the assistant is parented elsewhere.
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info": map[string]any{
			"id": "msg_old", "parentID": "msg_older", "role": "assistant",
			"finish": "stop", "time": map[string]any{"completed": 9},
		},
	}))
	// Same session, user message echo for our prompt.
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info":      map[string]any{"id": "msg_prompt", "parentID": "", "role": "user"},
	}))
	// Our assistant answers.
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info": map[string]any{
			"id": "msg_asst", "parentID": "msg_prompt", "role": "assistant",
			"finish": "stop", "time": map[string]any{"completed": 5},
		},
	}))
	// A text part on a foreign message id must not be tracked.
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part":      map[string]any{"type": "text", "id": "prt_foreign", "messageID": "msg_old", "text": "OLD"},
	}))
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part":      map[string]any{"type": "text", "id": "prt_mine", "messageID": "msg_asst", "text": "MINE"},
	}))
	state.apply(frameOf(t, "session.idle", map[string]any{"sessionID": "ses_oc"}))
	obs := state.observation()
	if !Completed(obs) {
		t.Fatalf("observation = %#v", obs)
	}
	if text := state.mergedText(); text != "MINE" {
		t.Fatalf("merged text = %q", text)
	}
}

func TestSubStateAbortMatchesLockedNameExactly(t *testing.T) {
	state := newSubState("ses_oc", "msg_prompt", nil)
	for _, name := range []string{"MessageCancelledError", "RequestCancelled", "cancelled", "AbortCancelled"} {
		state.apply(frameOf(t, "session.error", map[string]any{
			"sessionID": "ses_oc",
			"error":     map[string]any{"name": name},
		}))
		if state.aborted {
			t.Fatalf("%q was misclassified as an abort", name)
		}
	}
	if state.liveErrorName != "MessageCancelledError" {
		t.Fatalf("live error name = %q", state.liveErrorName)
	}
	// A global error without a session id is noise, not an abort.
	state.apply(frameOf(t, "session.error", map[string]any{"error": map[string]any{"name": "MessageAbortedError"}}))
	if state.aborted {
		t.Fatal("session-less error was accepted as an abort")
	}
	state.apply(frameOf(t, "session.error", map[string]any{
		"sessionID": "ses_oc",
		"error":     map[string]any{"name": "MessageAbortedError", "data": map[string]any{"message": "Aborted"}},
	}))
	if !state.aborted {
		t.Fatal("the locked MessageAbortedError was not recognized")
	}
}

func TestSubStateQuestionIsInteractionNotTool(t *testing.T) {
	var pending []string
	state := newSubState("ses_oc", "msg_prompt", func(kind, partID string) { pending = append(pending, kind+":"+partID) })
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info":      map[string]any{"id": "msg_a", "parentID": "msg_prompt", "role": "assistant"},
	}))
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_q", "messageID": "msg_a", "tool": "question",
			"state": map[string]any{"status": "pending"},
		},
	}))
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_t", "messageID": "msg_a", "tool": "bash",
			"state": map[string]any{"status": "running"},
		},
	}))
	if _, leaked := state.tools["prt_q"]; leaked {
		t.Fatal("a pending question leaked into the tool map")
	}
	if _, interaction := state.interactions["prt_q"]; !interaction {
		t.Fatal("the pending question was not kept as an interaction")
	}
	obs := state.observation()
	if !obs.PendingTool {
		t.Fatal("the running bash tool was not tracked")
	}
	if len(pending) != 1 || pending[0] != "question:prt_q" {
		t.Fatalf("interaction callbacks = %v", pending)
	}
}

func TestSubStateToolTerminalStatesAreKept(t *testing.T) {
	state := newSubState("ses_oc", "msg_prompt", nil)
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info":      map[string]any{"id": "msg_a", "parentID": "msg_prompt", "role": "assistant"},
	}))
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_b", "messageID": "msg_a", "tool": "bash",
			"callID": "call_b", "state": map[string]any{"status": "running"},
		},
	}))
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_b", "messageID": "msg_a", "tool": "bash",
			"callID": "call_b", "state": map[string]any{"status": "completed"},
		},
	}))
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part": map[string]any{
			"type": "tool", "id": "prt_e", "messageID": "msg_a", "tool": "edit",
			"callID": "call_e", "state": map[string]any{"status": "error"},
		},
	}))
	tools := state.terminalTools()
	if len(tools) != 2 || tools[0].partID != "prt_b" || tools[0].status != "completed" || tools[1].status != "error" {
		t.Fatalf("terminal tools = %#v", tools)
	}
	if state.hasPendingTool() {
		t.Fatal("a completed tool is still pending")
	}
}

func TestSubStateDuplicatePartUpdatesAreIdempotent(t *testing.T) {
	state := newSubState("ses_oc", "msg_prompt", nil)
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info":      map[string]any{"id": "msg_a", "parentID": "msg_prompt", "role": "assistant"},
	}))
	for i := 0; i < 3; i++ {
		state.apply(frameOf(t, "message.part.updated", map[string]any{
			"sessionID": "ses_oc",
			"part":      map[string]any{"type": "text", "id": "prt_t", "messageID": "msg_a", "text": "MOCK-REPLY"},
		}))
		state.apply(frameOf(t, "message.part.updated", map[string]any{
			"sessionID": "ses_oc",
			"part": map[string]any{
				"type": "tool", "id": "prt_b", "messageID": "msg_a", "tool": "bash",
				"state": map[string]any{"status": "completed"},
			},
		}))
	}
	if text := state.mergedText(); text != "MOCK-REPLY" {
		t.Fatalf("duplicate part updates duplicated text: %q", text)
	}
	if tools := state.terminalTools(); len(tools) != 1 {
		t.Fatalf("duplicate tool updates created %d entries", len(tools))
	}
}

func TestSubStateTextMergeIsBoundedAndDropsOldest(t *testing.T) {
	state := newSubState("ses_oc", "msg_prompt", nil)
	block := strings.Repeat("a", 64<<10)
	state.setText("prt_1", block)
	state.setText("prt_2", block)
	state.setText("prt_3", block)
	state.setText("prt_4", block)
	state.setText("prt_5", block[:16])
	if state.textBytes > maxMergedTextBytes {
		t.Fatalf("text budget exceeded: %d", state.textBytes)
	}
	if _, kept := state.texts["prt_1"]; kept {
		t.Fatal("the oldest text part was not dropped under overload")
	}
	if _, kept := state.texts["prt_4"]; !kept {
		t.Fatal("a recent text part was dropped")
	}
	if _, kept := state.texts["prt_5"]; !kept {
		t.Fatal("the newest text part was dropped")
	}
	state.appendDelta("prt_4", strings.Repeat("b", (64<<10)+16))
	if got := len(state.texts["prt_4"]); got > maxTextPartBytes {
		t.Fatalf("delta append exceeded the per-part cap: %d", got)
	}
	if !state.textOverflow {
		t.Fatal("overflow was not recorded")
	}
}

func TestSubStateDeltasMergeUntilSnapshotArrives(t *testing.T) {
	state := newSubState("ses_oc", "msg_prompt", nil)
	state.apply(frameOf(t, "message.updated", map[string]any{
		"sessionID": "ses_oc",
		"info":      map[string]any{"id": "msg_a", "parentID": "msg_prompt", "role": "assistant"},
	}))
	state.apply(frameOf(t, "message.part.delta", map[string]any{
		"sessionID": "ses_oc",
		"messageID": "msg_a", "partID": "prt_d", "field": "text", "delta": "Mock",
	}))
	state.apply(frameOf(t, "message.part.delta", map[string]any{
		"sessionID": "ses_oc",
		"messageID": "msg_a", "partID": "prt_d", "field": "text", "delta": "-Reply",
	}))
	// Deltas for a foreign message are dropped.
	state.apply(frameOf(t, "message.part.delta", map[string]any{
		"sessionID": "ses_oc",
		"messageID": "msg_other", "partID": "prt_x", "field": "text", "delta": "X",
	}))
	if text := state.mergedText(); text != "Mock-Reply" {
		t.Fatalf("merged deltas = %q", text)
	}
	// A full snapshot overwrites the accumulated deltas.
	state.apply(frameOf(t, "message.part.updated", map[string]any{
		"sessionID": "ses_oc",
		"part":      map[string]any{"type": "text", "id": "prt_d", "messageID": "msg_a", "text": "FINAL"},
	}))
	if text := state.mergedText(); text != "FINAL" {
		t.Fatalf("snapshot did not overwrite deltas: %q", text)
	}
}

func TestChooseAssistantPrefersCompletionIndependentOfOrder(t *testing.T) {
	older := assistantView{id: "msg_old", parentID: "p", finish: "stop", completedAt: 10, partCount: 1}
	newer := assistantView{id: "msg_new", parentID: "p", finish: "stop", completedAt: 20, partCount: 1}
	forward, _ := chooseAssistant([]assistantView{older, newer})
	backward, _ := chooseAssistant([]assistantView{newer, older})
	if forward.id != newer.id || backward.id != newer.id {
		t.Fatalf("selection depends on order: %q vs %q", forward.id, backward.id)
	}
	unfinished := assistantView{id: "msg_run", parentID: "p", finish: "", completedAt: 0, partCount: 5}
	finished := assistantView{id: "msg_done", parentID: "p", finish: "stop", completedAt: 0, partCount: 1}
	picked, _ := chooseAssistant([]assistantView{unfinished, finished})
	if picked.id != finished.id {
		t.Fatalf("unfinished message won: %#v", picked)
	}
}

func TestMergedTextKeepsPartOrderAndBudget(t *testing.T) {
	textPart := func(text string) json.RawMessage {
		raw, err := json.Marshal(map[string]any{"type": "text", "text": text})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	toolPart := func(tool string) json.RawMessage {
		raw, err := json.Marshal(map[string]any{"type": "tool", "tool": tool})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	parts := []json.RawMessage{textPart("one"), toolPart("bash"), textPart("two")}
	if got := mergedText(parts); got != "onetwo" {
		t.Fatalf("merged text = %q", got)
	}
	oversize := []json.RawMessage{textPart(strings.Repeat("x", maxMergedTextBytes)), textPart("tail")}
	if got := mergedText(oversize); len(got) != maxMergedTextBytes {
		t.Fatalf("merged text was not capped: %d", len(got))
	}
}
