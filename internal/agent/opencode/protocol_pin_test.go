package opencode

// CFT-S02-T013: the pinned-protocol contract. OpenCode 1.18.4 is the locked
// dialect (sha256 9449af91…f98 in docker/craft/runtime-config.json); these
// tests pin the three acceptance assertions OFFLINE against fixtures, while
// the live two-turn run (live_test.go, CRAFT_LIVE=1) and the W06 real-mode
// e2e hold the against-the-real-serve evidence:
//  1. a version drift yields CLEAR failures, never guessed fields
//  2. the event/message/cancel protocol matches the pinned surface
//  3. the /doc OpenAPI version is NEVER treated as the binary version
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProtocolPinVersionDriftFailsClearly(t *testing.T) {
	// A drifted parts shape (v-next renames the array) must error, not
	// silently decode into an empty projection.
	for name, raw := range map[string]string{
		"object instead of array": `{"parts":[]}`,
		"null part":               `[null]`,
		"string part":             `["text"]`,
		"empty":                   ``,
	} {
		if _, err := DecodeParts([]byte(raw)); err == nil {
			t.Fatalf("%s: expected a clear decode error, got none", name)
		}
	}
	// A drifted message-id shape must be rejected by the prompt path, never
	// guessed into the pinned wire format.
	if validMessageID("msg_drifted-format") {
		t.Fatal("a non-pinned message id must not validate")
	}
	if validMessageID(NewMessageIDOrFatal(t)) != true {
		t.Fatal("the pinned generator's ids must validate against the same pattern")
	}
}

func TestProtocolPinEndpointSurface(t *testing.T) {
	// The pinned 1.18.4 HTTP/SSE surface, asserted through a fixture serve:
	// POST /session, POST /session/{id}/prompt_async (204),
	// POST /session/{id}/abort, GET /session/status, GET /event (SSE),
	// POST /question/{id}/reply.
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/session":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"ses_pin"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/prompt_async"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/abort"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`true`))
		case r.Method == http.MethodGet && r.URL.Path == "/session/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ses_pin":{"type":"idle"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/event":
			w.Header().Set("content-type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"type\":\"session.idle\",\"properties\":{\"sessionID\":\"ses_pin\"}}\n\n"))
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/question/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`true`))
		default:
			t.Errorf("unexpected pinned-surface call: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx := context.Background()
	session, err := client.CreateSession(ctx)
	if err != nil || session != "ses_pin" {
		t.Fatalf("create session: %v %q", err, session)
	}
	if err := client.Prompt(ctx, session, NewMessageIDOrFatal(t), "build the report"); err != nil {
		t.Fatalf("prompt: %v", err)
	}
	if err := client.Abort(ctx, session); err != nil {
		t.Fatalf("abort: %v", err)
	}
	status, err := client.Status(ctx, session)
	if err != nil || status != "idle" {
		t.Fatalf("status: %v %q", err, status)
	}
	stream, err := client.Events(ctx)
	if err != nil {
		t.Fatalf("events: %v", err)
	}
	defer stream.Close()
	if err := client.ReplyQuestion(ctx, "q_1", [][]string{{"a"}}); err != nil {
		t.Fatalf("reply question: %v", err)
	}
	for _, required := range []string{
		"POST /session", "/prompt_async", "/abort", "GET /session/status", "GET /event", "/question/",
	} {
		found := false
		for _, call := range seen {
			if strings.Contains(call, required) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("pinned surface call missing: %s (seen %v)", required, seen)
		}
	}
	// An unknown session status type is a CLEAR failure, never a guess.
	driftServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]struct {
			Type string `json:"type"`
		}{"ses_pin": {Type: "v2-drifted-state"}})
	}))
	defer driftServer.Close()
	driftClient, driftErr := NewClient(driftServer.URL, nil)
	if driftErr != nil {
		t.Fatalf("NewClient(drift): %v", driftErr)
	}
	if _, err := driftClient.Status(ctx, "ses_pin"); err == nil {
		t.Fatal("an unknown status type must fail clearly, not map to a guessed state")
	}
}

func TestProtocolPinBinaryVersionIsTheDigestNotTheOpenAPI(t *testing.T) {
	// The client NEVER probes /doc for a version: the binary identity is the
	// deployment's runtime digest (CRAFT_OPENCODE_RUNTIME_DIGEST, checked
	// against docker/craft/runtime-config.json at boot) — the OpenAPI doc
	// version is documentation, not the pinned binary.
	if _, err := NewClient("http://pin.example", nil); err != nil {
		t.Fatalf("client construction must not require a version probe: %v", err)
	}
	// and the generator pins the 1.18.4 message-id ordering itself:
	id := NewMessageIDOrFatal(t)
	if !strings.HasPrefix(id, "msg_") || len(id) != len("msg_")+12+14 {
		t.Fatalf("message id %q does not match the pinned 1.18.4 shape", id)
	}
}

func NewMessageIDOrFatal(t *testing.T) string {
	t.Helper()
	id, err := NewMessageID()
	if err != nil {
		t.Fatalf("NewMessageID: %v", err)
	}
	return id
}
