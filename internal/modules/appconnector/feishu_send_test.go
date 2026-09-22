package appconnector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- Step 1 (brief, verbatim) ----

func TestFeishuTimeoutOutsideDedupWindowDoesNotRetry(t *testing.T) {
	if CanRetryProviderWrite(true, false) {
		t.Fatal("expired dedup key retried")
	}
	if CanRetryProviderWrite(false, true) {
		t.Fatal("no provider guarantee")
	}
}

// ---- Planned cases ----

func TestFeishuRetryNeedsGuaranteedKeyAndWindow(t *testing.T) {
	if !CanRetryProviderWrite(true, true) {
		t.Fatal("guaranteed key within window must allow the retry")
	}
}

func TestFeishuFS01ContractPinned(t *testing.T) {
	if FeishuAPIHost != "open.feishu.cn" {
		t.Fatal(FeishuAPIHost)
	}
	if FeishuSendMessagePath != "/open-apis/im/v1/messages" {
		t.Fatal(FeishuSendMessagePath)
	}
	if FeishuTokenCategory != "tenant_access_token" {
		t.Fatal(FeishuTokenCategory)
	}
	if FeishuReceiveIDType != "chat_id" {
		t.Fatal(FeishuReceiveIDType)
	}
	if FeishuPermissionSendMessage != "im:message:send_as_message" {
		t.Fatal(FeishuPermissionSendMessage)
	}
	if !FeishuReviewedMsgTypes["text"] || FeishuReviewedMsgTypes["bogus"] {
		t.Fatal("reviewed message-type set is wrong")
	}
	// Official create-message doc: "Requests with the same uuid can
	// successfully send at most one message within 1 hour."
	if FeishuDedupWindow != time.Hour {
		t.Fatal("official dedup window is 1 hour")
	}
}

// fakeFeishu is a LOCAL contract double of the FS-01 endpoints. It records
// every create-message POST, models "saved remotely then response lost"
// (hijack + close), and serves the chat history used for reconciliation.
// It is NOT FS-04: no real Feishu acceptance is claimed through it.
type fakeFeishu struct {
	mu            sync.Mutex
	sends         []capturedFeishuSend // saved messages (deduped by uuid)
	attemptUUIDs  []string             // uuid of every POST attempt
	attempts      int
	lists         int
	dropsResponse bool
}

type capturedFeishuSend struct {
	UUID      string
	ReceiveID string
	MsgType   string
	Content   string
	Auth      string
	Query     string
}

func (f *fakeFeishu) stats() (attempts, saved, lists int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts, len(f.sends), f.lists
}

func (f *fakeFeishu) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/open-apis/im/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req feishuCreateMessageRequest
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.attempts++
		f.attemptUUIDs = append(f.attemptUUIDs, req.UUID)
		id := ""
		for i, s := range f.sends {
			// Provider contract: same uuid sends at most one message.
			if s.UUID == req.UUID {
				id = fmt.Sprintf("om_saved_%d", i+1)
				break
			}
		}
		if id == "" {
			f.sends = append(f.sends, capturedFeishuSend{
				UUID:      req.UUID,
				ReceiveID: req.ReceiveID,
				MsgType:   req.MsgType,
				Content:   req.Content,
				Auth:      r.Header.Get("Authorization"),
				Query:     r.URL.RawQuery,
			})
			id = fmt.Sprintf("om_saved_%d", len(f.sends))
		}
		drop := f.dropsResponse
		f.mu.Unlock()
		if drop {
			hj, ok := w.(http.Hijacker)
			if !ok {
				panic("fake feishu: cannot hijack connection")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				panic("fake feishu: hijack failed: " + err.Error())
			}
			_ = conn.Close() // remote saved the message; the response is lost
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"code\":0,\"msg\":\"success\",\"data\":{\"message_id\":%q}}", id)
	})
	mux.HandleFunc("/open-apis/im/v1/chats/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lists++
		sends := append([]capturedFeishuSend(nil), f.sends...)
		f.mu.Unlock()
		var b strings.Builder
		b.WriteString("{\"code\":0,\"data\":{\"items\":[")
		for i, s := range sends {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, "{\"message_id\":\"om_saved_%d\",\"chat_id\":%q,\"msg_type\":%q,\"body\":{\"content\":%s}}",
				i+1, s.ReceiveID, s.MsgType, strconv.Quote(s.Content))
		}
		b.WriteString("]}}")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, b.String())
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func feishuNorm(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	norm, err := NormalizeArgs(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return norm
}

func feishuSendAction(t *testing.T) Action {
	t.Helper()
	return Action{
		ID:           "act_feishu_send_1",
		TenantID:     7,
		ActorID:      "user_1",
		ConnectionID: "conn_feishu",
		Version:      "v1",
		Target:       "oc_reviewed_chat",
		Risk:         RiskSend,
		Args: feishuNorm(t,
			"{\"chat_id\":\"oc_reviewed_chat\",\"msg_type\":\"text\",\"content\":{\"text\":\"Q3 board pack is ready for review\"}}"),
	}
}

func feishuPolicy(host, port string) HTTPPolicy {
	_, loop, _ := net.ParseCIDR("127.0.0.0/8")
	return HTTPPolicy{
		Scheme:             "http",
		Host:               host,
		Port:               port,
		Methods:            []string{http.MethodPost, http.MethodGet},
		PathPrefix:         "/open-apis/",
		AuthorizedNetworks: []*net.IPNet{loop},
		Resolver: IPResolverFunc(func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}),
		Timeout: 5 * time.Second,
	}
}

func newFeishuAdapter(t *testing.T, drops bool) (*FeishuSendAdapter, *fakeFeishu) {
	t.Helper()
	f := &fakeFeishu{dropsResponse: drops}
	ts := f.server(t)
	port := strconv.Itoa(ts.Listener.Addr().(*net.TCPAddr).Port)
	return &FeishuSendAdapter{
		Policy:         feishuPolicy("feishu.test", port),
		Token:          func(ctx context.Context) (string, error) { return "t-g104_test_tenant_token", nil },
		KeyGuaranteed:  true,
		DedupWindow:    FeishuDedupWindow,
		FirstAttemptAt: func(a Action) time.Time { return time.Unix(1700000000, 0) },
		Now:            func() time.Time { return time.Unix(1700000000, 0) },
	}, f
}

func TestFeishuWrongTargetRejectedBeforeRequest(t *testing.T) {
	ad, f := newFeishuAdapter(t, false)
	// The connection's reviewed policy names a DIFFERENT destination
	// resource; the send path is unlisted for it.
	ad.Policy.Host = "other.reviewed.host"
	ad.Policy.PathPrefix = "/open-apis/bisheng/v1"
	out, err := ad.Execute(context.Background(), feishuSendAction(t))
	if !errors.Is(err, ErrUnlistedDestination) {
		t.Fatalf("unlisted destination must be rejected, got %v", err)
	}
	if out.State != ActionFailed {
		t.Fatalf("policy denial state = %q", out.State)
	}
	if attempts, _, _ := f.stats(); attempts != 0 {
		t.Fatalf("unlisted target reached the wire: %d attempts", attempts)
	}
}

func TestFeishuApprovalRevokedBeforeExecuteBlocks(t *testing.T) {
	ad, f := newFeishuAdapter(t, false)
	ad.Recheck = func(ctx context.Context, a Action) error {
		return errors.New("approval revoked by admin after authorization")
	}
	out, err := ad.Execute(context.Background(), feishuSendAction(t))
	if !errors.Is(err, ErrFeishuApprovalRevoked) {
		t.Fatalf("revocation must surface, got %v", err)
	}
	if out.State != ActionAwaitingApproval {
		t.Fatalf("revoked action must stay parked awaiting approval, got %q", out.State)
	}
	if attempts, _, _ := f.stats(); attempts != 0 {
		t.Fatalf("revoked approval still dispatched: %d attempts", attempts)
	}
}

func TestFeishuSuccessSavesRealMessageID(t *testing.T) {
	ad, _ := newFeishuAdapter(t, false)
	out, err := ad.Execute(context.Background(), feishuSendAction(t))
	if err != nil {
		t.Fatal(err)
	}
	if out.State != ActionSucceeded {
		t.Fatalf("state = %q", out.State)
	}
	if out.ExternalID != "om_saved_1" {
		t.Fatalf("the REAL provider message id must be saved, got %q", out.ExternalID)
	}
	if !strings.Contains(string(out.Output), "om_saved_1") {
		t.Fatalf("raw provider payload must be kept, got %s", out.Output)
	}
}

func TestFeishuSnapshotFieldByFieldEquality(t *testing.T) {
	ad, f := newFeishuAdapter(t, false)
	a := feishuSendAction(t)
	key, err := ad.IdempotencyKey(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(key) == 0 || len(key) > 50 {
		t.Fatalf("uuid must fit the documented 50-char limit: %q", key)
	}
	if _, err := ad.Execute(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	if _, err := ad.Execute(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	attempts, saved, _ := f.stats()
	if attempts != 2 || saved != 1 {
		t.Fatalf("same-key attempts=%d saved=%d", attempts, saved)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, uuid := range f.attemptUUIDs {
		if uuid != key {
			t.Fatalf("attempt %d used %q, want the SAME stable key %q (never a fresh UUID)", i+1, uuid, key)
		}
	}
	s := f.sends[0]
	if s.ReceiveID != "oc_reviewed_chat" {
		t.Fatalf("target rewritten after approval: %q", s.ReceiveID)
	}
	if s.MsgType != "text" {
		t.Fatalf("msg_type rewritten after approval: %q", s.MsgType)
	}
	if s.Content != "{\"text\":\"Q3 board pack is ready for review\"}" {
		t.Fatalf("content rewritten after approval: %s", s.Content)
	}
	if s.Query != "receive_id_type=chat_id" {
		t.Fatalf("conversation scope (receive_id_type) wrong: %q", s.Query)
	}
	if s.Auth != "Bearer t-g104_test_tenant_token" {
		t.Fatalf("FS-01 tenant token category wrong: %q", s.Auth)
	}
	// The snapshot shape is fixed: extra fields or unreviewed message types
	// are rejected instead of silently forwarded.
	if _, err := ParseFeishuSendSnapshot(feishuNorm(t,
		"{\"chat_id\":\"oc_x\",\"msg_type\":\"text\",\"content\":{\"text\":\"a\"},\"extra\":1}")); !errors.Is(err, ErrFeishuSnapshotInvalid) {
		t.Fatalf("extra snapshot field must be rejected, got %v", err)
	}
	if _, err := ParseFeishuSendSnapshot(feishuNorm(t,
		"{\"chat_id\":\"oc_x\",\"msg_type\":\"executable\",\"content\":{\"text\":\"a\"}}")); !errors.Is(err, ErrFeishuSnapshotInvalid) {
		t.Fatalf("unreviewed msg_type must be rejected, got %v", err)
	}
}

func TestFeishuResponseLostStaysUnknownThenReconciles(t *testing.T) {
	ad, f := newFeishuAdapter(t, true) // server SAVES, then drops the response
	a := feishuSendAction(t)
	out, err := ad.Execute(context.Background(), a)
	if out.State != ActionUnknown {
		t.Fatalf("lost response must stay unknown, got %q", out.State)
	}
	if !errors.Is(err, ErrFeishuOutcomeUnknown) {
		t.Fatalf("unknown must be explicit, got %v", err)
	}
	if attempts, _, _ := f.stats(); attempts != 1 {
		t.Fatalf("attempts = %d", attempts)
	}
	// unknown NEVER auto-resolves into a re-send: Query is the entry point.
	q, qerr := ad.Query(context.Background(), a)
	if qerr != nil {
		t.Fatal(qerr)
	}
	if q.State != ActionSucceeded || q.ExternalID != "om_saved_1" {
		t.Fatalf("reconciliation must surface the REAL saved id, got %+v", q)
	}
	attempts, saved, lists := f.stats()
	if attempts != 1 || saved != 1 || lists != 1 {
		t.Fatalf("duplicate send after lost response: attempts=%d saved=%d lists=%d", attempts, saved, lists)
	}
	q2, qerr2 := ad.Query(context.Background(), a)
	if qerr2 != nil || q2.ExternalID != "om_saved_1" {
		t.Fatalf("reconciliation must be idempotent: %+v %v", q2, qerr2)
	}
}

func TestFeishuDedupWindowExpiredNoRetrySameKeyOnly(t *testing.T) {
	ctx := context.Background()
	ad, f := newFeishuAdapter(t, true) // first attempt: saved, response lost
	a := feishuSendAction(t)
	first, ferr := ad.Execute(ctx, a)
	if first.State != ActionUnknown || !errors.Is(ferr, ErrFeishuOutcomeUnknown) {
		t.Fatalf("first attempt must be unknown, got %+v %v", first, ferr)
	}
	base := time.Unix(1700000000, 0)
	ad.Now = func() time.Time { return base.Add(FeishuDedupWindow + time.Minute) }
	if ad.CanRetry(a) {
		t.Fatal("expired dedup window must forbid the retry end-to-end")
	}
	// The unknown stays parked: only the provider query may resolve it.
	q, qerr := ad.Query(ctx, a)
	if qerr != nil || q.State != ActionSucceeded || q.ExternalID != "om_saved_1" {
		t.Fatalf("expired-window resolution via Query failed: %+v %v", q, qerr)
	}
	if attempts, saved, _ := f.stats(); attempts != 1 || saved != 1 {
		t.Fatalf("window-expired unknown was re-sent: attempts=%d saved=%d", attempts, saved)
	}

	// In-window WITH the officially guaranteed key: retry is allowed and
	// reuses the SAME key (provider dedups it to the one saved message).
	ad2, f2 := newFeishuAdapter(t, true)
	a2 := feishuSendAction(t)
	if _, err := ad2.Execute(ctx, a2); !errors.Is(err, ErrFeishuOutcomeUnknown) {
		t.Fatal(err)
	}
	if !ad2.CanRetry(a2) {
		t.Fatal("guaranteed key within window must allow retrying the same key")
	}
	f2.mu.Lock()
	f2.dropsResponse = false
	f2.mu.Unlock()
	r2, rerr := ad2.Execute(ctx, a2) // SAME key retried
	if rerr != nil {
		t.Fatal(rerr)
	}
	attempts2, saved2, _ := f2.stats()
	if r2.State != ActionSucceeded || attempts2 != 2 || saved2 != 1 {
		t.Fatalf("same-key retry must dedup to one message: %+v attempts=%d saved=%d", r2, attempts2, saved2)
	}
	if r2.ExternalID != "om_saved_1" {
		t.Fatalf("same-key retry must surface the original message id, got %q", r2.ExternalID)
	}
	f2.mu.Lock()
	uuids := append([]string(nil), f2.attemptUUIDs...)
	f2.mu.Unlock()
	if len(uuids) != 2 || uuids[0] != uuids[1] || uuids[0] == "" {
		t.Fatalf("retry must reuse the SAME uuid, never a new one: %v", uuids)
	}
	// No provider guarantee → never a retry, even inside the window.
	ad2.KeyGuaranteed = false
	if ad2.CanRetry(a2) {
		t.Fatal("no provider guarantee must forbid the retry")
	}
}
