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

// brokenBodyTransport performs the real round trip and then loses the response
// body after a prefix — exactly the unknown-outcome window the protocol must
// survive without double charging.
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
// outcome reuses the identity and the gateway answers 409 fail-closed, and
// exactly one physical upstream call plus one usage record exist afterwards.
func TestCraftEgressAdapterJoinedWithRealGateway(t *testing.T) {
	env := newCraftGatewayTestEnv(t)
	credential, _ := issueCredentialOn(t, env, craftGatewayIssueBody)

	gatewayServer := httptest.NewServer(env.router)
	defer gatewayServer.Close()

	journalPath := filepath.Join(t.TempDir(), "attempts.jsonl")
	adapter, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL: gatewayServer.URL + "/craft/model-gateway",
		Credential:     credential,
		JournalPath:    journalPath,
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

	// Unknown outcome: the gateway finishes its side, the adapter loses the
	// response stream. The retry must reuse the SAME identity, and the real
	// gateway must refuse a second physical send with 409.
	unknownBody := `{"model":"m1","messages":[{"role":"user","content":"unknown window"}]}`
	transport := &brokenBodyTransport{inner: http.DefaultTransport, failNext: true}
	adapter2, err := craftegress.NewCraftEgressAdapter(craftegress.CraftEgressAdapterConfig{
		GatewayBaseURL: gatewayServer.URL + "/craft/model-gateway",
		Credential:     credential,
		JournalPath:    filepath.Join(t.TempDir(), "attempts2.jsonl"),
		Transport:      transport,
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

	// A deliberately new logical attempt after a RESOLVED one mints a new
	// identity and succeeds through the real gateway again.
	next := post(chatBody)
	require.Equal(t, http.StatusOK, next.Code, next.Body.String())
	require.NotEqual(t, happy.Header().Get("X-Craft-Activity-ID"), next.Header().Get("X-Craft-Activity-ID"),
		"a resolved activity followed by a new send must mint a new identity")
	require.Len(t, env.recorder.recorded(), 3, "three durable attributions for three physical sends")

	// Minted identities satisfy the gateway charset contract end to end.
	for _, id := range []string{happy.Header().Get("X-Craft-Activity-ID"), parkedID, next.Header().Get("X-Craft-Activity-ID")} {
		require.NotEmpty(t, id)
		require.LessOrEqual(t, len(id), 128)
		for _, r := range id {
			require.True(t, r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._:-", r),
				"id %q violates the gateway charset", id)
		}
	}
}
