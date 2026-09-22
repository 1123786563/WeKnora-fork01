package appconnector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// Feishu send rejections.
var (
	// ErrFeishuApprovalRevoked: the A03 approval was revoked (or its A02
	// permission version invalidated) between approval and execute; the
	// action stays parked awaiting approval and nothing is dispatched.
	ErrFeishuApprovalRevoked = errors.New("feishu_approval_revoked")
	// ErrFeishuSnapshotInvalid: the action arguments are not exactly the
	// approved three-field send snapshot (chat_id, msg_type, content) —
	// an extra field, a missing field, an unreviewed message type or
	// non-JSON content is refused rather than silently forwarded.
	ErrFeishuSnapshotInvalid = errors.New("feishu_snapshot_invalid")
	// ErrFeishuOutcomeUnknown: the request may or may not have produced
	// its remote effect (response lost, unparseable reply, or a provider
	// "success" without a real message id). The outcome stays unknown and
	// resolves ONLY via Query or a CanRetryProviderWrite-approved retry.
	ErrFeishuOutcomeUnknown = errors.New("feishu_outcome_unknown")
	// ErrFeishuNotConfigured: the adapter is missing a reviewed outbound
	// policy or a token source — fail closed, never dial by invention.
	ErrFeishuNotConfigured = errors.New("feishu_adapter_not_configured")
)

// FS-01 fixed contract, validated against the official "create message"
// document (open.larksuite.com /document/server-docs/im-v1/message/create):
// token category, conversation scope, reviewed message types, request path
// and permission. These are the reviewed constants the adapter is allowed
// to use; nothing here is derived from model output.
const (
	FeishuAPIHost               = "open.feishu.cn"
	FeishuTokenCategory         = "tenant_access_token" // Authorization: Bearer t-...
	FeishuSendMessagePath       = "/open-apis/im/v1/messages"
	FeishuChatMessagesFormat    = "/open-apis/im/v1/chats/%s/messages"
	FeishuReceiveIDType         = "chat_id" // conversation scope
	FeishuPermissionSendMessage = "im:message:send_as_message"
	// FeishuDedupWindow is the officially verified idempotency guarantee:
	// "Requests with the same uuid can successfully send at most one
	// message within 1 hour."
	FeishuDedupWindow = time.Hour
	// FeishuUUIDMaxLen: documented maximum length of the uuid field.
	FeishuUUIDMaxLen = 50
)

// FeishuReviewedMsgTypes is the FS-01 reviewed message-type set. A snapshot
// naming anything else is rejected before any request is built.
var FeishuReviewedMsgTypes = map[string]bool{
	"text":        true,
	"post":        true,
	"image":       true,
	"interactive": true,
}

// FeishuSendSnapshot is the A03-approved argument snapshot for one Feishu
// send: exactly chat_id (conversation scope), msg_type and content. The
// dispatched request is built FROM these bytes field-by-field; nothing is
// added, rewritten or re-derived after approval.
type FeishuSendSnapshot struct {
	ChatID  string
	MsgType string
	Content json.RawMessage
}

// feishuCreateMessageRequest is the wire body of the create-message call.
type feishuCreateMessageRequest struct {
	ReceiveID string `json:"receive_id"`
	MsgType   string `json:"msg_type"`
	Content   string `json:"content"`
	UUID      string `json:"uuid"`
}

type feishuMessageItem struct {
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
	MsgType   string `json:"msg_type"`
	Body      struct {
		Content string `json:"content"`
	} `json:"body"`
}

type feishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		MessageID string              `json:"message_id"`
		Items     []feishuMessageItem `json:"items"`
	} `json:"data"`
}

// ParseFeishuSendSnapshot validates that args are EXACTLY the approved
// three-field send snapshot: no extra fields (approve-then-rewrite), no
// missing fields, a reviewed message type, a non-empty chat and JSON
// content.
func ParseFeishuSendSnapshot(args json.RawMessage) (FeishuSendSnapshot, error) {
	var s FeishuSendSnapshot
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.UseNumber()
	var raw map[string]json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return s, fmt.Errorf("%w: %v", ErrFeishuSnapshotInvalid, err)
	}
	if len(raw) != 3 {
		return s, fmt.Errorf("%w: snapshot must be exactly chat_id, msg_type, content", ErrFeishuSnapshotInvalid)
	}
	if err := json.Unmarshal(raw["chat_id"], &s.ChatID); err != nil {
		return s, fmt.Errorf("%w: chat_id: %v", ErrFeishuSnapshotInvalid, err)
	}
	if err := json.Unmarshal(raw["msg_type"], &s.MsgType); err != nil {
		return s, fmt.Errorf("%w: msg_type: %v", ErrFeishuSnapshotInvalid, err)
	}
	if s.ChatID == "" {
		return s, fmt.Errorf("%w: empty chat_id", ErrFeishuSnapshotInvalid)
	}
	if !FeishuReviewedMsgTypes[s.MsgType] {
		return s, fmt.Errorf("%w: msg_type %q not reviewed", ErrFeishuSnapshotInvalid, s.MsgType)
	}
	s.Content = raw["content"]
	if _, err := NormalizeArgs(s.Content); err != nil {
		return s, fmt.Errorf("%w: content: %v", ErrFeishuSnapshotInvalid, err)
	}
	return s, nil
}

// CanRetryProviderWrite decides whether a provider write whose outcome is
// unknown may be retried: ONLY when the provider OFFICIALLY GUARANTEES
// idempotency for the key we sent AND the first attempt is still inside
// the provider's dedup window. Any other unknown stays unknown and is
// resolved by a provider query or human reconciliation — never by a fresh
// send.
func CanRetryProviderWrite(hasGuaranteedKey, withinWindow bool) bool {
	return hasGuaranteedKey && withinWindow
}

// FeishuSendAdapter is the Feishu member of the A04 Adapter family. It
// executes ONE approved send (A03 snapshot) against the FS-01 reviewed
// create-message contract, routed through the A04 outbound policy. Real
// transport is policy.NewClient(); the oapi-sdk-go client is NOT used
// here because its transport would bypass the reviewed dial/redirect
// validation — the validated method contract is pinned by the constants
// above instead of trusting SDK compilation as external success.
type FeishuSendAdapter struct {
	// Policy is the admin-reviewed outbound contract (A04). Every request
	// and redirect hop is validated by it before any dial.
	Policy HTTPPolicy
	// Token returns the tenant_access_token (FS-01 token category).
	Token func(ctx context.Context) (string, error)
	// Recheck re-validates the A03 approval right before the outbound
	// call; a revocation between approval and execute blocks the send.
	Recheck func(ctx context.Context, a Action) error
	// KeyGuaranteed marks that the officially verified idempotency field
	// (uuid, same-uuid dedup within 1h) is part of the used contract.
	KeyGuaranteed bool
	// DedupWindow overrides FeishuDedupWindow (test hook; 0 = official).
	DedupWindow time.Duration
	// FirstAttemptAt reports the persisted first-dispatch time of the
	// action (nil or zero time fails the window check closed).
	FirstAttemptAt func(a Action) time.Time
	// Now is the clock used for the dedup window (test hook).
	Now func() time.Time
}

var _ Adapter = (*FeishuSendAdapter)(nil)

// IdempotencyKey derives the provider dedup key (uuid) for one action. It
// is DETERMINISTIC — a pure function of the approved action identity — so
// every retry and every reconciliation of the SAME action carries the
// IDENTICAL key; generating a fresh UUID for an unknown message is
// structurally impossible here.
func (m *FeishuSendAdapter) IdempotencyKey(a Action) (string, error) {
	digest, err := ActionDigest(a)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte("feishu_send_uuid:" + digest))
	return hex.EncodeToString(sum[:])[:FeishuUUIDMaxLen], nil
}

func (m *FeishuSendAdapter) dedupWindow() time.Duration {
	if m.DedupWindow > 0 {
		return m.DedupWindow
	}
	return FeishuDedupWindow
}

func (m *FeishuSendAdapter) withinDedupWindow(a Action) bool {
	if m.FirstAttemptAt == nil {
		return false
	}
	first := m.FirstAttemptAt(a)
	if first.IsZero() {
		return false
	}
	now := m.Now
	if now == nil {
		now = time.Now
	}
	return !now().After(first.Add(m.dedupWindow()))
}

// CanRetry applies the retry contract end-to-end for one action: the
// officially guaranteed key AND the dedup window must both hold.
func (m *FeishuSendAdapter) CanRetry(a Action) bool {
	return CanRetryProviderWrite(m.KeyGuaranteed, m.withinDedupWindow(a))
}

func (m *FeishuSendAdapter) targetURL(path string, query url.Values) *url.URL {
	host := m.Policy.Host
	if m.Policy.Port != "" {
		host = net.JoinHostPort(host, m.Policy.Port)
	}
	u := &url.URL{Scheme: m.Policy.Scheme, Host: host, Path: path}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	return u
}

func (m *FeishuSendAdapter) configError() error {
	if m.Policy.Host == "" || m.Policy.Scheme == "" {
		return fmt.Errorf("%w: no reviewed outbound policy", ErrFeishuNotConfigured)
	}
	if m.Token == nil {
		return fmt.Errorf("%w: no %s token source", ErrFeishuNotConfigured, FeishuTokenCategory)
	}
	return nil
}

func (m *FeishuSendAdapter) do(ctx context.Context, method string, u *url.URL, contentType string, body []byte, out *feishuResponse) (raw []byte, err error) {
	if err := m.Policy.ValidateRequest(method, u); err != nil {
		return nil, err // policy denial: the request never leaves
	}
	tok, err := m.Token(ctx)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := m.Policy.NewClient().Do(req)
	if err != nil {
		// The effect may already exist remotely: unknown, never failed.
		return nil, fmt.Errorf("%w: %v", ErrFeishuOutcomeUnknown, err)
	}
	defer resp.Body.Close()
	raw, err = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuOutcomeUnknown, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFeishuOutcomeUnknown, err)
	}
	return raw, nil
}

// Execute performs the approved send. Order: A03 recheck, snapshot
// validation, A04 policy validation, then — only then — the real call.
func (m *FeishuSendAdapter) Execute(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	if m.Recheck != nil {
		if err := m.Recheck(ctx, a); err != nil {
			return ActionResult{State: ActionAwaitingApproval}, fmt.Errorf("%w: %v", ErrFeishuApprovalRevoked, err)
		}
	}
	snap, err := ParseFeishuSendSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	key, err := m.IdempotencyKey(a)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	// The wire body is built FROM the snapshot, field-by-field: receive_id
	// = approved chat, msg_type = approved type, content = the approved
	// canonical content bytes, uuid = the deterministic same-key.
	content, err := NormalizeArgs(snap.Content)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	body, err := json.Marshal(feishuCreateMessageRequest{
		ReceiveID: snap.ChatID,
		MsgType:   snap.MsgType,
		Content:   string(content),
		UUID:      key,
	})
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	u := m.targetURL(FeishuSendMessagePath, url.Values{"receive_id_type": {FeishuReceiveIDType}})
	var pr feishuResponse
	raw, err := m.do(ctx, http.MethodPost, u, "application/json; charset=utf-8", body, &pr)
	if err != nil {
		state := ActionFailed
		if errors.Is(err, ErrFeishuOutcomeUnknown) {
			state = ActionUnknown
		}
		return ActionResult{State: state}, err
	}
	if pr.Code != 0 {
		return ActionResult{State: ActionFailed}, fmt.Errorf("feishu_provider_error: code=%d msg=%s", pr.Code, pr.Msg)
	}
	if pr.Data.MessageID == "" {
		// A provider confirmation without a REAL message id proves nothing.
		return ActionResult{State: ActionUnknown}, fmt.Errorf("%w: no real message id in provider reply", ErrFeishuOutcomeUnknown)
	}
	return ActionResult{State: ActionSucceeded, ExternalID: pr.Data.MessageID, Output: json.RawMessage(raw)}, nil
}

// Query is the reconciliation entry point for an unknown outcome: it reads
// the conversation history (chat_id scope) and matches the saved message
// FIELD-BY-FIELD against the approved snapshot. A match settles succeeded
// with the REAL message id — no re-send happened. No match keeps the
// honest unknown; a fabricated failure is never returned because it would
// trigger a re-send of an effect that may already exist.
func (m *FeishuSendAdapter) Query(ctx context.Context, a Action) (ActionResult, error) {
	if err := m.configError(); err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	snap, err := ParseFeishuSendSnapshot(a.Args)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	approved, err := NormalizeArgs(snap.Content)
	if err != nil {
		return ActionResult{State: ActionFailed}, err
	}
	u := m.targetURL(fmt.Sprintf(FeishuChatMessagesFormat, url.PathEscape(snap.ChatID)), url.Values{"page_size": {"20"}})
	var pr feishuResponse
	raw, err := m.do(ctx, http.MethodGet, u, "", nil, &pr)
	if err != nil {
		state := ActionFailed
		if errors.Is(err, ErrFeishuOutcomeUnknown) {
			state = ActionUnknown
		}
		return ActionResult{State: state}, err
	}
	if pr.Code != 0 {
		return ActionResult{State: ActionUnknown}, fmt.Errorf("feishu_provider_error: code=%d msg=%s", pr.Code, pr.Msg)
	}
	for _, it := range pr.Data.Items {
		if it.ChatID != snap.ChatID || it.MsgType != snap.MsgType {
			continue
		}
		item, nerr := NormalizeArgs(json.RawMessage(it.Body.Content))
		if nerr != nil || !bytes.Equal(item, approved) {
			continue
		}
		return ActionResult{State: ActionSucceeded, ExternalID: it.MessageID, Output: json.RawMessage(raw)}, nil
	}
	return ActionResult{State: ActionUnknown}, nil
}
