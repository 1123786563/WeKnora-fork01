package opencode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
)

// messageAbortedErrorName is the exact error.name the locked OpenCode 1.18.4
// runtime attaches to an aborted message. Verified against the pinned binary:
// both the session.error event and the message projection carry
// {"error":{"name":"MessageAbortedError","data":{"message":"Aborted"}}}.
// Abort classification compares this name for exact equality; error names
// that merely contain "cancel" or similar substrings are never aborts.
const messageAbortedErrorName = "MessageAbortedError"

// Bounded-merge budgets. Under event overload the normalizer keeps session
// state and tool terminal states first and degrades text aggregation: live
// text is truncated per part and oldest parts are dropped once the total
// budget is exhausted.
const (
	maxTextPartBytes   = 64 << 10
	maxMergedTextBytes = 256 << 10
	maxEmitTextBytes   = 64 << 10
	maxSummaryBytes    = 4 << 10
	maxTrackedParts    = 1024
	maxFrameDataBytes  = maxEventBytes
)

// Completed reports whether an Observation proves the delegated prompt
// finished successfully: the assistant chain must answer exactly this
// prompt message id, the message must be completed with finish "stop", the
// session must be idle, no tool may still be running and the run must not
// have been aborted. Anything else keeps the delegation unresolved.
func Completed(o craft.Observation) bool {
	return o.SessionID != "" && o.PromptMessageID != "" &&
		o.AssistantParentID == o.PromptMessageID && o.Completed && o.Idle &&
		o.Finish == "stop" && !o.PendingTool && !o.Aborted
}

// Message ids pack a 36-bit millisecond timestamp (the 48-bit value is
// timestamp<<12 plus a counter), so the timestamp wraps roughly every 795
// days. IDs must never be ordered or compared as timestamps, and a
// persisted id may only be re-submitted while it still decodes inside the
// current wrap window.
const (
	messageIDWrapMS        = int64(1) << 36
	messageIDReuseWindowMS = 24 * 60 * 60 * 1000
)

// messageIDTimeMS decodes the packed millisecond timestamp of a pinned
// OpenCode message id. The bool is false for ids that do not follow the
// locked format.
func messageIDTimeMS(id string) (int64, bool) {
	raw, ok := strings.CutPrefix(id, "msg_")
	if !ok || len(raw) < 12 {
		return 0, false
	}
	value, err := strconv.ParseUint(raw[:12], 16, 64)
	if err != nil {
		return 0, false
	}
	return int64(value >> 12), true
}

// messageIDReusableAt reports whether a persisted message id still decodes
// to a timestamp in the same wrap window as now (within the reuse window),
// so re-submitting it cannot collide with an id minted in a previous
// ~795-day epoch.
func messageIDReusableAt(id string, now time.Time) bool {
	stampMS, ok := messageIDTimeMS(id)
	if !ok {
		return false
	}
	delta := (now.UnixMilli() - stampMS) % messageIDWrapMS
	if delta < 0 {
		delta += messageIDWrapMS
	}
	if delta > messageIDWrapMS/2 {
		delta -= messageIDWrapMS
	}
	if delta < 0 {
		delta = -delta
	}
	return delta <= messageIDReuseWindowMS
}

// eventFrame is one flat SSE frame from GET /event:
// {"id":..., "type":..., "properties":...}.
type eventFrame struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties"`
}

var errEventFrameTooLarge = errors.New("opencode SSE event exceeds the frame budget")

// scanEventFrames parses the locked SSE dialect (data: lines, multiline
// payloads, heartbeat comments, ignored id/event/retry fields) and hands
// each decoded frame to handle. Handle returns false to stop early. Frames
// that are not flat event objects are dropped: the snapshot verification
// remains the authority, so a junk frame never interrupts observation.
func scanEventFrames(reader io.Reader, handle func(eventFrame) bool) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64<<10), maxFrameDataBytes+(64<<10))
	var lines []string
	frameBytes := 0
	flush := func() bool {
		if len(lines) == 0 {
			return true
		}
		raw := []byte(strings.Join(lines, "\n"))
		lines = nil
		var frame eventFrame
		if err := json.Unmarshal(raw, &frame); err != nil || frame.Type == "" {
			return true
		}
		return handle(frame)
	}
	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		switch {
		case line == "":
			if !flush() {
				return nil
			}
		case strings.HasPrefix(line, ":"):
			// heartbeat comment
		case strings.HasPrefix(line, "data:"):
			chunk := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			frameBytes += len(chunk)
			if frameBytes > maxFrameDataBytes {
				return errEventFrameTooLarge
			}
			lines = append(lines, chunk)
		default:
			// id:, event:, retry: are not used by this consumer
		}
	}
	flush()
	return scanner.Err()
}

// partWire is the tolerant decode of one message part. Tool state exposes
// only the status; raw tool inputs, outputs and environment payloads are
// deliberately not carried into Craft events (sensitive command arguments
// and model credentials never leave the runtime boundary).
type partWire struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	MessageID string `json:"messageID"`
	Tool      string `json:"tool"`
	CallID    string `json:"callID"`
	Text      string `json:"text"`
	State     *struct {
		Status string `json:"status"`
	} `json:"state"`
}

type partView struct {
	id, messageID, typ, tool, callID, status, text string
}

func decodePartView(raw json.RawMessage) (partView, bool) {
	var wire partWire
	if err := json.Unmarshal(raw, &wire); err != nil || wire.Type == "" {
		return partView{}, false
	}
	view := partView{id: wire.ID, messageID: wire.MessageID, typ: wire.Type,
		tool: wire.Tool, callID: wire.CallID, text: wire.Text}
	if wire.State != nil {
		view.status = wire.State.Status
	}
	return view, true
}

// partKind classifies a part. A question is delivered as the "question"
// tool in the locked runtime; it is an interaction, not a pending tool.
func partKind(view partView) string {
	switch {
	case view.typ == "text":
		return "text"
	case view.typ == "permission":
		return "permission"
	case view.typ == "question":
		return "question"
	case view.typ == "tool" && view.tool == "question":
		return "question"
	case view.typ == "tool":
		return "tool"
	default:
		return "other"
	}
}

func partPending(view partView) bool {
	return view.status == "pending" || view.status == "running"
}

func toolTerminal(view partView) bool {
	return view.status == "completed" || view.status == "error"
}

// mergedText concatenates the text parts of one message in part order with
// a hard budget.
func mergedText(parts []json.RawMessage) string {
	var builder strings.Builder
	for _, raw := range parts {
		view, ok := decodePartView(raw)
		if !ok || partKind(view) != "text" || view.text == "" {
			continue
		}
		if builder.Len()+len(view.text) > maxMergedTextBytes {
			remaining := maxMergedTextBytes - builder.Len()
			if remaining > 0 {
				builder.WriteString(view.text[:remaining])
			}
			break
		}
		builder.WriteString(view.text)
	}
	return builder.String()
}

// assistantState is the live projection of one assistant message that
// answers the delegated prompt (info.parentID == promptID, exact match).
type assistantState struct {
	id, parentID, finish, errorName string
	completedAt                     int64
	partCount                       int
}

type toolPartState struct {
	partID, messageID, tool, callID, status string
}

// subState normalizes the sub-execution live event stream. Only frames
// for the bound session and only the assistant chain parented to the exact
// prompt message id are tracked; other rounds in the same session never
// pollute this task.
type subState struct {
	sessionID, promptID string

	assistants map[string]*assistantState
	tools      map[string]*toolPartState
	texts      map[string]string
	textOrder  []string
	textBytes  int

	interactions map[string]string

	aborted       bool
	idle          bool
	statusType    string
	liveErrorName string
	textOverflow  bool

	onInteraction func(kind, partID string)
}

func newSubState(sessionID, promptID string, onInteraction func(kind, partID string)) *subState {
	return &subState{
		sessionID: sessionID, promptID: promptID,
		assistants:    map[string]*assistantState{},
		tools:         map[string]*toolPartState{},
		texts:         map[string]string{},
		interactions:  map[string]string{},
		onInteraction: onInteraction,
	}
}

func (s *subState) sessionMatches(sessionID string) bool {
	return sessionID != "" && sessionID == s.sessionID
}

// apply folds one frame into the state. Frames that fail to decode, target
// another session, or belong to another conversation turn are ignored.
func (s *subState) apply(frame eventFrame) {
	switch frame.Type {
	case "session.status":
		var p struct {
			SessionID string `json:"sessionID"`
			Status    struct {
				Type string `json:"type"`
			} `json:"status"`
		}
		if !decodeProperties(frame.Properties, &p) || !s.sessionMatches(p.SessionID) {
			return
		}
		s.statusType = p.Status.Type
		s.idle = p.Status.Type == "idle"
	case "session.idle":
		var p struct {
			SessionID string `json:"sessionID"`
		}
		if !decodeProperties(frame.Properties, &p) || !s.sessionMatches(p.SessionID) {
			return
		}
		s.idle = true
	case "session.error":
		var p struct {
			SessionID string `json:"sessionID"`
			Error     *struct {
				Name string `json:"name"`
			} `json:"error"`
		}
		if !decodeProperties(frame.Properties, &p) || !s.sessionMatches(p.SessionID) {
			return
		}
		if p.Error == nil || p.Error.Name == "" {
			return
		}
		if p.Error.Name == messageAbortedErrorName {
			s.aborted = true
			return
		}
		if s.liveErrorName == "" {
			s.liveErrorName = p.Error.Name
		}
	case "message.updated":
		var p struct {
			SessionID string `json:"sessionID"`
			Info      struct {
				ID       string `json:"id"`
				ParentID string `json:"parentID"`
				Role     string `json:"role"`
				Finish   string `json:"finish"`
				Time     struct {
					Completed int64 `json:"completed"`
				} `json:"time"`
				Error *struct {
					Name string `json:"name"`
				} `json:"error"`
			} `json:"info"`
		}
		if !decodeProperties(frame.Properties, &p) || !s.sessionMatches(p.SessionID) {
			return
		}
		if p.Info.Role != "assistant" || p.Info.ParentID != s.promptID || p.Info.ID == "" {
			return
		}
		state := s.assistants[p.Info.ID]
		if state == nil {
			if len(s.assistants) >= maxTrackedParts {
				return
			}
			state = &assistantState{id: p.Info.ID, parentID: p.Info.ParentID}
			s.assistants[p.Info.ID] = state
		}
		state.finish = p.Info.Finish
		state.completedAt = p.Info.Time.Completed
		if p.Info.Error != nil && p.Info.Error.Name != "" {
			state.errorName = p.Info.Error.Name
		}
	case "message.part.updated":
		var p struct {
			SessionID string   `json:"sessionID"`
			Part      partWire `json:"part"`
		}
		if !decodeProperties(frame.Properties, &p) || !s.sessionMatches(p.SessionID) {
			return
		}
		view, ok := decodePartView(json.RawMessage(mustEncodeJSON(p.Part)))
		if !ok || !s.assistantMessage(p.Part.MessageID) {
			return
		}
		s.applyPart(view)
	case "message.part.delta":
		var p struct {
			SessionID string `json:"sessionID"`
			MessageID string `json:"messageID"`
			PartID    string `json:"partID"`
			Field     string `json:"field"`
			Delta     string `json:"delta"`
		}
		if !decodeProperties(frame.Properties, &p) || !s.sessionMatches(p.SessionID) {
			return
		}
		if p.Field != "text" || !s.assistantMessage(p.MessageID) {
			return
		}
		s.appendDelta(p.PartID, p.Delta)
	}
}

func (s *subState) assistantMessage(messageID string) bool {
	_, ok := s.assistants[messageID]
	return ok
}

func (s *subState) applyPart(view partView) {
	switch partKind(view) {
	case "text":
		s.setText(view.id, view.text)
	case "tool":
		if _, tracked := s.tools[view.id]; !tracked && len(s.tools) >= maxTrackedParts {
			return
		}
		existing := s.tools[view.id]
		if existing == nil {
			existing = &toolPartState{partID: view.id, messageID: view.messageID}
			s.tools[view.id] = existing
			if state := s.assistants[view.messageID]; state != nil {
				state.partCount++
			}
		}
		existing.tool = view.tool
		existing.callID = view.callID
		if view.status != "" {
			existing.status = view.status
		}
	case "question", "permission":
		kind := partKind(view)
		if _, seen := s.interactions[view.id]; !seen {
			if len(s.interactions) >= maxTrackedParts {
				return
			}
			s.interactions[view.id] = kind
			if partPending(view) && s.onInteraction != nil {
				s.onInteraction(kind, view.id)
			}
		}
	}
}

func (s *subState) setText(partID, text string) {
	if partID == "" || text == "" {
		return
	}
	if len(text) > maxTextPartBytes {
		text = text[:maxTextPartBytes]
		s.textOverflow = true
	}
	if _, seen := s.texts[partID]; !seen {
		s.makeTextRoom(len(text))
		if len(s.textOrder) >= maxTrackedParts {
			return
		}
		s.textOrder = append(s.textOrder, partID)
	}
	s.textBytes += len(text) - len(s.texts[partID])
	s.texts[partID] = text
	s.trimMergedText()
}

func (s *subState) appendDelta(partID, delta string) {
	if partID == "" || delta == "" {
		return
	}
	current := s.texts[partID]
	if len(current)+len(delta) > maxTextPartBytes {
		remaining := maxTextPartBytes - len(current)
		if remaining > 0 {
			delta = delta[:remaining]
		} else {
			delta = ""
		}
		s.textOverflow = true
	}
	if delta == "" {
		return
	}
	if _, seen := s.texts[partID]; !seen {
		s.makeTextRoom(len(delta))
		s.textOrder = append(s.textOrder, partID)
	}
	s.textBytes += len(delta)
	s.texts[partID] = current + delta
	s.trimMergedText()
}

func (s *subState) makeTextRoom(needed int) {
	for s.textBytes+needed > maxMergedTextBytes && len(s.textOrder) > 1 {
		oldest := s.textOrder[0]
		s.textOrder = s.textOrder[1:]
		s.textBytes -= len(s.texts[oldest])
		delete(s.texts, oldest)
	}
}

func (s *subState) trimMergedText() {
	for s.textBytes > maxMergedTextBytes && len(s.textOrder) > 0 {
		oldest := s.textOrder[0]
		s.textOrder = s.textOrder[1:]
		s.textBytes -= len(s.texts[oldest])
		delete(s.texts, oldest)
	}
}

func (s *subState) mergedText() string {
	var builder strings.Builder
	for _, partID := range s.textOrder {
		text := s.texts[partID]
		if builder.Len()+len(text) > maxEmitTextBytes {
			remaining := maxEmitTextBytes - builder.Len()
			if remaining > 0 {
				builder.WriteString(text[:remaining])
			}
			break
		}
		builder.WriteString(text)
	}
	return builder.String()
}

func (s *subState) hasPendingTool() bool {
	for _, tool := range s.tools {
		if tool.status == "pending" || tool.status == "running" {
			return true
		}
	}
	return false
}

// observation projects the live state into an Observation. Event-derived
// completion is advisory only; the snapshot verification decides.
func (s *subState) observation() craft.Observation {
	obs := craft.Observation{SessionID: s.sessionID, PromptMessageID: s.promptID,
		Idle: s.idle, Aborted: s.aborted}
	if best, ok := chooseAssistant(s.assistantViews()); ok {
		obs.AssistantParentID = best.parentID
		obs.Finish = best.finish
		obs.Completed = best.completedAt > 0
		obs.Aborted = obs.Aborted || best.errorName == messageAbortedErrorName
	}
	obs.PendingTool = s.hasPendingTool()
	return obs
}

// terminalSignal reports when the live stream shows a decision-ready
// sub-execution: the session went idle after the assistant chain reached a
// terminal message state, or an abort landed.
func (s *subState) terminalSignal() bool {
	if s.aborted && s.idle {
		return true
	}
	if !s.idle {
		return false
	}
	for _, state := range s.assistants {
		if state.completedAt > 0 || state.finish != "" {
			return true
		}
	}
	return false
}

func (s *subState) assistantViews() []assistantView {
	views := make([]assistantView, 0, len(s.assistants))
	for _, state := range s.assistants {
		views = append(views, assistantView{id: state.id, parentID: state.parentID,
			finish: state.finish, errorName: state.errorName,
			completedAt: state.completedAt, partCount: state.partCount})
	}
	return views
}

func (s *subState) terminalTools() []toolPartState {
	partIDs := make([]string, 0, len(s.tools))
	for partID, tool := range s.tools {
		if tool.status == "completed" || tool.status == "error" {
			partIDs = append(partIDs, partID)
		}
	}
	sort.Strings(partIDs)
	tools := make([]toolPartState, 0, len(partIDs))
	for _, partID := range partIDs {
		tools = append(tools, *s.tools[partID])
	}
	return tools
}

// assistantView is the common projection used to choose the answering
// assistant message both from live events and from the message snapshot.
type assistantView struct {
	id, parentID, finish, errorName string
	completedAt                     int64
	parts                           []json.RawMessage
	partCount                       int
}

// betterAssistant orders two candidates of the same prompt chain. Selection
// prefers completion time, then a set finish flag, then richer content;
// never the message id and never array position, so a shuffled snapshot or
// an id wrap cannot change the outcome.
func betterAssistant(a, b assistantView) bool {
	switch {
	case a.completedAt != b.completedAt:
		return a.completedAt > b.completedAt
	case (a.finish != "") != (b.finish != ""):
		return a.finish != ""
	default:
		return viewPartCount(a) > viewPartCount(b)
	}
}

func viewPartCount(view assistantView) int {
	if view.parts != nil {
		return len(view.parts)
	}
	return view.partCount
}

func chooseAssistant(views []assistantView) (assistantView, bool) {
	if len(views) == 0 {
		return assistantView{}, false
	}
	best := views[0]
	for _, candidate := range views[1:] {
		if betterAssistant(candidate, best) {
			best = candidate
		}
	}
	return best, true
}

// snapshot is one verified read of the runtime: session status, the
// message list and their parts, plus the locked error projection when the
// answering assistant ended without a successful finish.
type snapshot struct {
	obs        craft.Observation
	status     string
	promptSeen bool
	assistant  *Message
	errorName  string
}

// snapshotObservation reads the runtime state and projects it into an
// Observation bound to the exact prompt message id. Any read failure
// returns an error: a snapshot that could not be collected proves nothing.
func snapshotObservation(ctx context.Context, client *Client, sessionID, promptID string) (snapshot, error) {
	status, err := client.Status(ctx, sessionID)
	if err != nil {
		return snapshot{}, err
	}
	messages, err := client.Messages(ctx, sessionID)
	if err != nil {
		return snapshot{}, err
	}
	snap := snapshot{status: status, obs: craft.Observation{
		SessionID: sessionID, PromptMessageID: promptID, Idle: status == "idle"}}
	var views []assistantView
	for i := range messages {
		message := messages[i]
		if message.ID == promptID {
			snap.promptSeen = true
		}
		if message.Role != "assistant" || message.ParentID != promptID {
			continue
		}
		views = append(views, assistantView{id: message.ID, parentID: message.ParentID,
			finish: message.Finish, completedAt: message.CompletedAt, parts: message.Parts})
	}
	for _, view := range views {
		for _, raw := range view.parts {
			part, ok := decodePartView(raw)
			if ok && partKind(part) == "tool" && partPending(part) {
				snap.obs.PendingTool = true
			}
		}
	}
	if best, ok := chooseAssistant(views); ok {
		snap.obs.AssistantParentID = best.parentID
		snap.obs.Finish = best.finish
		snap.obs.Completed = best.completedAt > 0
		if best.completedAt > 0 && best.finish != "stop" {
			names, err := client.messageErrorNames(ctx, sessionID)
			if err == nil {
				snap.errorName = names[best.id]
			}
		}
		if snap.errorName == messageAbortedErrorName {
			snap.obs.Aborted = true
		}
		for i := range messages {
			if messages[i].ID == best.id {
				snap.assistant = &messages[i]
				break
			}
		}
	}
	return snap, nil
}

// messageErrorNames fetches the locked per-message error projection
// (info.error.name). The pinned Message type deliberately narrows the
// message projection; abort classification still needs the exact
// MessageAbortedError name, so this reads the same route once more and
// keeps only id and error name. It is called only when the answering
// assistant ended without a successful finish.
func (c *Client) messageErrorNames(ctx context.Context, sessionID string) (map[string]string, error) {
	var wire []struct {
		Info struct {
			ID    string `json:"id"`
			Error *struct {
				Name string `json:"name"`
			} `json:"error"`
		} `json:"info"`
	}
	if err := c.jsonRequest(ctx, http.MethodGet, escapedPath("session", sessionID, "message"),
		nil, http.StatusOK, &wire); err != nil {
		return nil, err
	}
	names := make(map[string]string, len(wire))
	for _, item := range wire {
		if item.Info.ID != "" && item.Info.Error != nil && item.Info.Error.Name != "" {
			names[item.Info.ID] = item.Info.Error.Name
		}
	}
	return names, nil
}

func decodeProperties(raw json.RawMessage, target any) bool {
	return json.Unmarshal(raw, target) == nil
}

func mustEncodeJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		return []byte("{}")
	}
	return raw
}

func boundString(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit]
}
