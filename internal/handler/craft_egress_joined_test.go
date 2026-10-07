package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craftegress"
	"github.com/stretchr/testify/require"
)

// lostResponseTransport performs the real round trip — the gateway finishes
// its side and the physical call is durably attributed — and then loses the
// response before the adapter can observe even the status line. This is the
// unknown-outcome window under every adapter ruling: no status, no envelope,
// nothing to classify, so the identity stays parked and the same-fingerprint
// retry reuses it against the gateway's fail-closed 409 without a second
// physical send.
type lostResponseTransport struct {
	inner    http.RoundTripper
	failNext bool
}

func (t *lostResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if t.failNext {
		t.failNext = false
		_ = resp.Body.Close()
		return nil, errors.New("connection lost after the gateway finished its side")
	}
	return resp, nil
}

// brokenBodyTransport performs the real round trip and then truncates the
// response body after a prefix. Per the craft-107 OCR R2 F28 ruling
// (docs/plans/2026-09-28-craft-107-ocr-r2-egress-report.md) a torn 2xx whose
// status line arrived intact is a TERMINAL outcome: the journal resolves the
// attempt, and the retry legitimately mints a new identity for a new physical
// send. A torn 5xx/409 stays parked by the status-only fallback, so the
// double-billing protection covers the windows that are actually unknown.
type brokenBodyTransport struct {
	inner    http.RoundTripper
	failNext bool
}

func (t *brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if t.failNext {
		t.failNext = false
		resp.Body = &prefixBrokenBody{inner: resp.Body}
	}
	return resp, nil
}

type prefixBrokenBody struct {
	inner io.ReadCloser
	done  bool
}

func (b *prefixBrokenBody) Read(p []byte) (int, error) {
	if b.done {
		return 0, errors.New("response stream lost mid-body")
	}
	b.done = true
	// Deliver a strict prefix without EOF so the reader must come back and
	// then hit the lost-stream error.
	if len(p) > 4 {
		p = p[:4]
	}
	return b.inner.Read(p)
}

func (b *prefixBrokenBody) Close() error { return b.inner.Close() }

// TestCraftEgressAdapterJoinedWithRealGateway proves the runtime-side
// producer against the production gateway contract: the adapter-minted
// X-Craft-Activity-ID is accepted by the real Forward path, an unknown
// outcome (response lost before any status is observable) reuses the identity
// and the gateway answers 409 fail-closed with exactly one physical upstream
// call and one usage record for the window, while a terminally-resolved torn
// 2xx retry (F28 ruling) mints a new identity for its own physical send.
func TestCraftEgressAdapterJoinedWithRealGateway(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	gatewayServer := httptest.NewServer(env.router)
	defer gatewayServer.Close()

	journalPath := filepath.Join(t.TempDir(), "attempts.jsonl")
	// The joined gateway is an httptest server on 127.0.0.1: this fixture is
	// exactly the adapter's documented "local deployment" case, so it takes
	// the explicit private-target opt-in. Without it the production transport
	// re-validates every dialed IP against the loopback/private/reserved
	// ranges at connect time and refuses the forward ([T08] dial guard).
	adapter, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL:     gatewayServer.URL + "/craft/model-gateway",
		Credential:         credential,
		JournalPath:        journalPath,
		AllowPrivateTarget: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, adapter.Close()) }()

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		adapter.ServeHTTP(rec, req)
		return rec
	}
	chatBody := `{"model":"m1","messages":[{"role":"user","content":"joined"}]}`

	happy := post(chatBody)
	require.Equal(t, http.StatusOK, happy.Code, happy.Body.String())
	require.NotEmpty(t, happy.Header().Get("X-Craft-Activity-ID"), "the adapter surfaces its minted identity")
	recorded := env.recorder.recorded()
	require.Len(t, recorded, 1, "one physical call is durably attributed")
	require.Equal(t, "Bearer sk-platform-managed", env.upstreamAuth(), "the gateway injected the managed upstream credential")

	// Unknown outcome: the gateway finishes its side (the round trip really
	// completes and the usage fact is durably recorded), and the adapter then
	// loses the response before any status is observable. The retry must
	// reuse the SAME identity, and the real gateway must refuse a second
	// physical send with 409.
	unknownBody := `{"model":"m1","messages":[{"role":"user","content":"unknown window"}]}`
	adapter2, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL: gatewayServer.URL + "/craft/model-gateway",
		Credential:     credential,
		JournalPath:    filepath.Join(t.TempDir(), "attempts2.jsonl"),
		Transport:      &lostResponseTransport{inner: http.DefaultTransport, failNext: true},
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, adapter2.Close()) }()
	post2 := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		adapter2.ServeHTTP(rec, req)
		return rec
	}
	lost := post2(unknownBody)
	require.Equal(t, http.StatusBadGateway, lost.Code, "a lost gateway response is an explicit unknown")
	parkedID := lost.Header().Get("X-Craft-Activity-ID")
	require.NotEmpty(t, parkedID)
	retry := post2(unknownBody)
	require.Equal(t, http.StatusConflict, retry.Code, "the real gateway fail-closes an unresolved activity: %s", retry.Body.String())
	require.Equal(t, parkedID, retry.Header().Get("X-Craft-Activity-ID"), "the parked identity is reused, never re-minted")
	require.Len(t, env.recorder.recorded(), 2, "exactly two physical calls total: one happy, one unknown-window")

	// Terminally-resolved torn 2xx (F28 ruling): the gateway's 200 arrives
	// but the body tears mid-stream. The intact status line resolves the
	// attempt terminally, so the same-fingerprint retry is a NEW physical
	// send with a NEW identity — not a reuse of the resolved one.
	tornBody := `{"model":"m1","messages":[{"role":"user","content":"torn window"}]}`
	adapter3, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL: gatewayServer.URL + "/craft/model-gateway",
		Credential:     credential,
		JournalPath:    filepath.Join(t.TempDir(), "attempts3.jsonl"),
		Transport:      &brokenBodyTransport{inner: http.DefaultTransport, failNext: true},
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, adapter3.Close()) }()
	post3 := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		adapter3.ServeHTTP(rec, req)
		return rec
	}
	torn := post3(tornBody)
	require.Equal(t, http.StatusBadGateway, torn.Code, "a torn gateway body is surfaced as an explicit incomplete response")
	tornID := torn.Header().Get("X-Craft-Activity-ID")
	require.NotEmpty(t, tornID)
	tornRetry := post3(tornBody)
	require.Equal(t, http.StatusOK, tornRetry.Code, "a terminally-resolved torn 2xx retry is a new physical send: %s", tornRetry.Body.String())
	require.NotEqual(t, tornID, tornRetry.Header().Get("X-Craft-Activity-ID"), "a terminal outcome never reuses the resolved identity")
	require.Len(t, env.recorder.recorded(), 4, "four physical calls total: happy, unknown-window, torn send, torn retry")

	// A deliberately new logical attempt after a RESOLVED one mints a new
	// identity and succeeds through the real gateway again.
	next := post(chatBody)
	require.Equal(t, http.StatusOK, next.Code, next.Body.String())
	require.NotEqual(t, happy.Header().Get("X-Craft-Activity-ID"), next.Header().Get("X-Craft-Activity-ID"),
		"a resolved activity followed by a new send must mint a new identity")
	require.Len(t, env.recorder.recorded(), 5, "five durable attributions for five physical sends")

	// Minted identities satisfy the gateway charset contract end to end.
	for _, id := range []string{happy.Header().Get("X-Craft-Activity-ID"), parkedID, tornID, next.Header().Get("X-Craft-Activity-ID")} {
		require.NotEmpty(t, id)
		require.LessOrEqual(t, len(id), 128)
		for _, r := range id {
			require.True(t, r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r),
				"id %q violates the gateway charset", id)
		}
	}
}
