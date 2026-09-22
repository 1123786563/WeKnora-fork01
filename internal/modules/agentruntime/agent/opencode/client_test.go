package opencode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Mutation caught: accepting JSON null, a non-array, null array elements, or
// non-object elements (numbers, strings) as parts.
func TestDecodePartsRejectsInvalidShape(t *testing.T) {
	for _, raw := range []string{`null`, `[null]`, `{}`, `[1]`, `["text"]`} {
		if _, err := DecodeParts([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := DecodeParts([]byte(`[{"type":"text","text":"ok"}]`)); err != nil {
		t.Fatal(err)
	}
}

func TestClientUsesLockedRoutesBodiesAndStatuses(t *testing.T) {
	t.Helper()
	messageID := "msg_0019a468fdc1ABCDEFGHIJKLMN"
	var seen []string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "create")
		fmt.Fprint(w, `{"id":"ses_test"}`)
	})
	mux.HandleFunc("POST /session/ses_test/prompt_async", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "prompt")
		assertJSONBody(t, r, `{"messageID":"`+messageID+`","parts":[{"type":"text","text":"hello"}]}`)
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /event", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "events")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, ": heartbeat\n\nid: evt_1\ndata: {\"id\":\"evt_1\",\ndata: \"type\":\"session.status\",\"properties\":{}}\n\n")
	})
	mux.HandleFunc("GET /session/ses_test/message", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "messages")
		fmt.Fprint(w, `[{"info":{"id":"msg_user","sessionID":"ses_test","role":"user","time":{"created":1}},"parts":[{"type":"text","text":"hello"}]},{"info":{"id":"msg_assistant","sessionID":"ses_test","role":"assistant","parentID":"msg_user","finish":"stop","time":{"created":2,"completed":3}},"parts":[{"type":"text","text":"done"}]}]`)
	})
	mux.HandleFunc("POST /session/ses_test/abort", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "abort")
		fmt.Fprint(w, `true`)
	})
	mux.HandleFunc("GET /session/status", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "status")
		fmt.Fprint(w, `{"ses_test":{"type":"retry","attempt":1,"message":"later","next":4},"other":{"type":"busy"}}`)
	})
	mux.HandleFunc("POST /question/que%2F1/reply", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "question-reply")
		assertJSONBody(t, r, `{"answers":[["yes"],["a","b"]]}`)
		fmt.Fprint(w, `true`)
	})
	mux.HandleFunc("POST /question/que%2F1/reject", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "question-reject")
		fmt.Fprint(w, `true`)
	})
	mux.HandleFunc("POST /session/ses%2Ftest/permissions/per%2F1", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, "permission")
		assertJSONBody(t, r, `{"response":"once"}`)
		fmt.Fprint(w, `true`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx)
	if err != nil || sessionID != "ses_test" {
		t.Fatalf("CreateSession() = %q, %v", sessionID, err)
	}
	if err := client.Prompt(ctx, sessionID, messageID, "hello"); err != nil {
		t.Fatal(err)
	}
	events, err := client.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := io.ReadAll(events)
	events.Close()
	if err != nil || !strings.Contains(string(stream), "data: {\"id\":\"evt_1\",") {
		t.Fatalf("Events() = %q, %v", stream, err)
	}
	messages, err := client.Messages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 || messages[0].Role != "user" || messages[1].ParentID != "msg_user" || messages[1].Finish != "stop" || messages[1].CompletedAt != 3 || len(messages[1].Parts) != 1 {
		t.Fatalf("Messages() = %#v", messages)
	}
	if err := client.Abort(ctx, sessionID); err != nil {
		t.Fatal(err)
	}
	status, err := client.Status(ctx, sessionID)
	if err != nil || status != "retry" {
		t.Fatalf("Status() = %q, %v", status, err)
	}
	if err := client.ReplyQuestion(ctx, "que/1", [][]string{{"yes"}, {"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if err := client.RejectQuestion(ctx, "que/1"); err != nil {
		t.Fatal(err)
	}
	if err := client.ReplyPermission(ctx, "ses/test", "per/1", "once"); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 9 {
		t.Fatalf("called routes = %v", seen)
	}
}

func TestPromptRequires204AndBoundsErrorBodies(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int
		body string
	}{
		{name: "400", code: http.StatusBadRequest, body: "bad request"},
		{name: "500", code: http.StatusInternalServerError, body: "server error"},
		{name: "200 is not accepted", code: http.StatusOK, body: "true"},
		{name: "oversize", code: http.StatusBadRequest, body: strings.Repeat("x", maxBodyBytes+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			err = client.Prompt(context.Background(), "ses_test", "msg_0019a468fdc1ABCDEFGHIJKLMN", "hello")
			if err == nil {
				t.Fatal("Prompt accepted non-204 response")
			}
			if len(err.Error()) > 8192 {
				t.Fatalf("unbounded error length: %d", len(err.Error()))
			}
		})
	}
}

func TestClientReportsDisconnect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hijacker := w.(http.Hijacker)
		conn, _, err := hijacker.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		conn.Close()
	}))
	defer server.Close()
	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateSession(context.Background()); err == nil {
		t.Fatal("CreateSession accepted a disconnected response")
	}
}

func TestEventsAllowsHeartbeatAndMultilineDataButRejectsOversizeFrame(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		wantErr bool
	}{
		{name: "heartbeat and multiline", payload: ": heartbeat\n\ndata: {\"id\":\"evt_1\",\ndata: \"type\":\"session.status\",\"properties\":{}}\n\n"},

		{name: "oversize", payload: "data: " + strings.Repeat("x", maxEventBytes) + "\n\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, tc.payload)
			}))
			defer server.Close()
			client, err := NewClient(server.URL, server.Client())
			if err != nil {
				t.Fatal(err)
			}
			body, err := client.Events(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			_, readErr := io.ReadAll(body)
			body.Close()
			if (readErr != nil) != tc.wantErr {
				t.Fatalf("ReadAll error = %v, wantErr %v", readErr, tc.wantErr)
			}
		})
	}
}

func TestClientValidationAndRedirectBoundary(t *testing.T) {
	for _, raw := range []string{"", "://bad", "ftp://example.com", "http://", "http://example.com/path?query=1"} {
		if _, err := NewClient(raw, http.DefaultClient); err == nil {
			t.Fatalf("NewClient(%q) succeeded", raw)
		}
	}
	client, err := NewClient("http://example.com", http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.ReplyPermission(context.Background(), "ses", "per", "always"); err == nil {
		t.Fatal("ReplyPermission accepted unsupported reply")
	}
	if err := client.Prompt(context.Background(), "ses", "msg_random", "hello"); err == nil {
		t.Fatal("Prompt accepted an ID that only has the msg_ prefix")
	}

	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"id":"ses_wrong_host"}`)
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	redirectClient, err := NewClient(source.URL, source.Client())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := redirectClient.CreateSession(context.Background()); err == nil {
		t.Fatal("followed redirect to another host")
	}
}

func TestOrdinaryRequestsHaveThirtySecondDeadline(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok {
			t.Error("ordinary request has no deadline")
		} else if remaining := time.Until(deadline); remaining < 29*time.Second || remaining > 30*time.Second {
			t.Errorf("deadline remaining = %v", remaining)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"id":"ses_test"}`)),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}
	client, err := NewClient("http://example.com", httpClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateSession(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

func TestMessageIDsFollowPinnedAscendingAlgorithm(t *testing.T) {
	first, err := newMessageIDAt(1720000000123)
	if err != nil {
		t.Fatal(err)
	}
	second, err := newMessageIDAt(1720000000123)
	if err != nil {
		t.Fatal(err)
	}
	if !validMessageID(first) || !validMessageID(second) || first >= second {
		t.Fatalf("IDs are not valid ascending IDs: %q, %q", first, second)
	}
}

// --- Fixtures captured from the locked OpenCode 1.18.4 binary (see
// testdata/protocol-lock.json). IDs were replaced with shape-preserving
// placeholders; every other byte is the real server response.

var (
	sessionIDPattern = regexp.MustCompile("^ses_[0-9a-f]{12}[0-9A-Za-z]{14}$")
	eventIDPattern   = regexp.MustCompile("^evt_[0-9a-f]{12}[0-9A-Za-z]{14}$")
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestClientReplaysRealBinaryFixtures replays the captured 1.18.4 responses
// through the client: the session id format, the abort body, the empty and
// two-round message projections, and the real /event SSE frames.
func TestClientReplaysRealBinaryFixtures(t *testing.T) {
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(loadFixture(t, "fixture_session_created.json"), &created); err != nil {
		t.Fatal(err)
	}
	var messageCalls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, "fixture_session_created.json"))
	})
	mux.HandleFunc("GET /session/"+created.ID+"/message", func(w http.ResponseWriter, _ *http.Request) {
		name := "fixture_messages_after_exchange.json"
		if messageCalls.Add(1) == 1 {
			name = "fixture_messages_empty.json"
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(loadFixture(t, name))
	})
	mux.HandleFunc("POST /session/"+created.ID+"/abort", func(w http.ResponseWriter, _ *http.Request) {
		w.Write(loadFixture(t, "fixture_abort_response.json"))
	})
	mux.HandleFunc("GET /event", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write(loadFixture(t, "fixture_event_stream.txt"))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !sessionIDPattern.MatchString(sessionID) {
		t.Fatalf("CreateSession() = %q, want ses_ + 12 hex + 14 base62", sessionID)
	}
	empty, err := client.Messages(ctx, sessionID)
	if err != nil || len(empty) != 0 {
		t.Fatalf("Messages() on fresh session = %#v, %v", empty, err)
	}
	messages, err := client.Messages(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 4 {
		t.Fatalf("Messages() = %d messages, want 4", len(messages))
	}
	for i, wantRole := range []string{"user", "assistant", "user", "assistant"} {
		if messages[i].Role != wantRole {
			t.Fatalf("message %d role = %q, want %q", i, messages[i].Role, wantRole)
		}
	}
	for i := 1; i < len(messages); i += 2 {
		assistant := messages[i]
		if assistant.ParentID != messages[i-1].ID || assistant.Finish != "stop" || assistant.CompletedAt == 0 {
			t.Fatalf("assistant message %d = %#v", i, assistant)
		}
		var text string
		for _, part := range assistant.Parts {
			var probe struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(part, &probe) == nil && probe.Type == "text" {
				text = probe.Text
			}
		}
		if !strings.HasPrefix(text, "MOCK-REPLY-") {
			t.Fatalf("assistant %d text part = %q", i, text)
		}
	}
	if err := client.Abort(ctx, sessionID); err != nil {
		t.Fatalf("Abort() on the captured 'true' body = %v", err)
	}

	events, err := client.Events(ctx)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := io.ReadAll(events)
	events.Close()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	frames := 0
	for _, line := range strings.Split(string(stream), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var frame struct {
			ID         string          `json:"id"`
			Type       string          `json:"type"`
			Properties json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal([]byte(line[len("data: "):]), &frame); err != nil {
			t.Fatalf("captured SSE frame %q is not flat JSON: %v", line, err)
		}
		if !eventIDPattern.MatchString(frame.ID) {
			t.Fatalf("captured event id = %q, want evt_ + 12 hex + 14 base62", frame.ID)
		}
		if frame.Type == "" || frame.Properties == nil {
			t.Fatalf("captured frame missing type or properties: %q", line)
		}
		seen[frame.Type] = true
		frames++
	}
	if frames < 8 || !seen["server.connected"] || !seen["session.created"] || !seen["session.status"] || !seen["message.updated"] || !seen["message.part.delta"] {
		t.Fatalf("captured stream covered %d frames, types %v", frames, seen)
	}
}

// --- Live tests against the locked binary. They skip unless
// /Users/wuyongjun/.opencode/bin/opencode exists, prints the locked version,
// and hashes to the digest recorded in testdata/protocol-lock.json.

const lockedOpenCodeBinary = "/Users/wuyongjun/.opencode/bin/opencode"

type protocolLockFile struct {
	Version      string `json:"version"`
	BinarySHA256 string `json:"binary_sha256"`
}

func requireLockedBinary(t *testing.T) {
	t.Helper()
	info, err := os.Stat(lockedOpenCodeBinary)
	if err != nil || info.IsDir() {
		t.Skipf("locked OpenCode binary missing at %s: %v", lockedOpenCodeBinary, err)
	}
	var lock protocolLockFile
	raw, err := os.ReadFile(filepath.Join("testdata", "protocol-lock.json"))
	if err != nil {
		t.Fatalf("read protocol lock: %v", err)
	}
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("parse protocol lock: %v", err)
	}
	output, err := exec.Command(lockedOpenCodeBinary, "--version").Output()
	if err != nil {
		t.Skipf("locked binary --version failed: %v", err)
	}
	if version := strings.TrimSpace(string(output)); version != lock.Version {
		t.Skipf("locked binary version %q != locked %q", version, lock.Version)
	}
	file, err := os.Open(lockedOpenCodeBinary)
	if err != nil {
		t.Skipf("open locked binary: %v", err)
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		t.Skipf("hash locked binary: %v", err)
	}
	if got := hex.EncodeToString(digest.Sum(nil)); got != lock.BinarySHA256 {
		t.Skipf("locked binary digest %s != locked %s", got, lock.BinarySHA256)
	}
}

type cappedBuffer struct {
	buf bytes.Buffer
	max int
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	if c.buf.Len() < c.max {
		c.buf.Write(p[:min(len(p), c.max-c.buf.Len())])
	}
	return len(p), nil
}

func (c *cappedBuffer) String() string { return c.buf.String() }

// startLockedServe boots the pinned opencode serve with XDG-isolated
// config/data/state directories so the user's real configuration, API keys,
// and database are never touched. The returned cleanup kills exactly this
// child process.
func startLockedServe(t *testing.T, config map[string]any) *Client {
	t.Helper()
	requireLockedBinary(t)
	for attempt := 0; attempt < 3; attempt++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("reserve port: %v", err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		base := "http://127.0.0.1:" + strconv.Itoa(port)

		dataDir := t.TempDir()
		configDir := t.TempDir()
		stateDir := t.TempDir()
		workDir := t.TempDir()
		if config != nil {
			raw, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(configDir, "opencode"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(configDir, "opencode", "opencode.json"), raw, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		cmd := exec.Command(lockedOpenCodeBinary, "serve", "--port", strconv.Itoa(port), "--hostname", "127.0.0.1")
		cmd.Dir = workDir
		cmd.Env = append(os.Environ(),
			"XDG_DATA_HOME="+dataDir,
			"XDG_CONFIG_HOME="+configDir,
			"XDG_STATE_HOME="+stateDir,
		)
		logs := &cappedBuffer{max: 8 << 10}
		cmd.Stdout, cmd.Stderr = logs, logs
		if err := cmd.Start(); err != nil {
			t.Fatalf("start locked serve: %v", err)
		}
		t.Cleanup(func() {
			if cmd.Process != nil {
				cmd.Process.Kill()
				cmd.Wait()
			}
		})
		deadline := time.Now().Add(60 * time.Second)
		for time.Now().Before(deadline) {
			response, err := http.Get(base + "/doc")
			if err == nil {
				response.Body.Close()
				if response.StatusCode == http.StatusOK {
					client, err := NewClient(base, &http.Client{})
					if err != nil {
						t.Fatal(err)
					}
					return client
				}
			}
			if cmd.ProcessState != nil {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		t.Logf("serve attempt %d did not become ready; logs: %s", attempt, logs.String())
	}
	t.Fatal("locked OpenCode serve never became ready")
	return nil
}

func waitforMessages(t *testing.T, client *Client, sessionID string, want int, timeout time.Duration) []Message {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var messages []Message
	for time.Now().Before(deadline) {
		candidate, err := client.Messages(context.Background(), sessionID)
		// The assistant message is appended before its parts and finish
		// state arrive, so completion (not just count) must be awaited.
		if err == nil && len(candidate) >= want && candidate[want-1].Finish == "stop" && len(candidate[want-1].Parts) > 0 {
			messages = candidate
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if messages == nil {
		messages, _ = client.Messages(context.Background(), sessionID)
		t.Fatalf("session never reached %d messages; now %#v", want, messages)
	}
	return messages
}

// TestLiveLockedBinaryRoutesStatusAndAbort exercises the real locked binary:
// route shapes and status codes for CreateSession/Status/Messages/Abort.
func TestLiveLockedBinaryRoutesStatusAndAbort(t *testing.T) {
	client := startLockedServe(t, nil)
	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx)
	if err != nil {
		t.Fatalf("live CreateSession: %v", err)
	}
	if !sessionIDPattern.MatchString(sessionID) {
		t.Fatalf("live session id %q does not follow the pinned format", sessionID)
	}
	status, err := client.Status(ctx, sessionID)
	if err != nil || status != "idle" {
		t.Fatalf("live Status() = %q, %v; want idle", status, err)
	}
	messages, err := client.Messages(ctx, sessionID)
	if err != nil || len(messages) != 0 {
		t.Fatalf("live Messages() = %#v, %v; want empty", messages, err)
	}
	if err := client.Abort(ctx, sessionID); err != nil {
		t.Fatalf("live Abort(): %v", err)
	}
}

// mockModelProvider is a local OpenAI-compatible endpoint; it never touches
// any real provider or API key.
type mockModelProvider struct {
	hits atomic.Int32
}

func (m *mockModelProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/chat/completions") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	r.Body.Close()
	var request struct {
		Model string `json:"model"`
	}
	json.Unmarshal(raw, &request)
	n := m.hits.Add(1)
	w.Header().Set("Content-Type", "text/event-stream")
	flusher := w.(http.Flusher)
	chunks := []map[string]any{
		{"delta": map[string]any{"role": "assistant", "content": fmt.Sprintf("MOCK-REPLY-%d", n)}},
		{"delta": map[string]any{}, "finish_reason": "stop"},
	}
	for _, chunk := range chunks {
		choice := map[string]any{"index": 0}
		for key, value := range chunk {
			choice[key] = value
		}
		frame := map[string]any{
			"id": fmt.Sprintf("chatcmpl-mock-%d", n), "object": "chat.completion.chunk",
			"created": time.Now().Unix(), "model": request.Model, "choices": []any{choice},
		}
		data, _ := json.Marshal(frame)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// TestLiveLockedBinaryTwoConsecutiveRoundsWithLocalMock drives two full
// prompt rounds through the locked binary using only a local mock model
// provider configured in an isolated XDG config. Message IDs come from the
// pinned ascending algorithm (NewMessageID).
func TestLiveLockedBinaryTwoConsecutiveRoundsWithLocalMock(t *testing.T) {
	model := &mockModelProvider{}
	modelServer := httptest.NewServer(model)
	defer modelServer.Close()
	client := startLockedServe(t, map[string]any{
		"$schema": "https://opencode.ai/config.json",
		"model":   "mock/mock-model",
		"provider": map[string]any{
			"mock": map[string]any{
				"npm":  "@ai-sdk/openai-compatible",
				"name": "Local Mock",
				"options": map[string]any{
					"baseURL": modelServer.URL + "/v1",
					"apiKey":  "local-mock-not-a-real-key",
				},
				"models": map[string]any{
					"mock-model": map[string]any{"name": "Mock Model"},
				},
			},
		},
	})
	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx)
	if err != nil {
		t.Fatalf("live CreateSession: %v", err)
	}
	rounds := []struct {
		prompt   string
		messageI int
	}{
		{prompt: "round one", messageI: 0},
		{prompt: "round two", messageI: 2},
	}
	var firstMessageID string
	for index, round := range rounds {
		messageID, err := NewMessageID()
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			firstMessageID = messageID
		}
		if err := client.Prompt(ctx, sessionID, messageID, round.prompt); err != nil {
			t.Fatalf("live Prompt round %d: %v", index+1, err)
		}
		messages := waitforMessages(t, client, sessionID, round.messageI+2, 90*time.Second)
		user, assistant := messages[round.messageI], messages[round.messageI+1]
		if user.Role != "user" || assistant.Role != "assistant" {
			t.Fatalf("round %d roles = %q/%q", index+1, user.Role, assistant.Role)
		}
		if user.ID != messageID {
			t.Fatalf("round %d server did not adopt the client message id: got %q want %q", index+1, user.ID, messageID)
		}
		if assistant.ParentID != messageID || assistant.Finish != "stop" || assistant.CompletedAt == 0 {
			t.Fatalf("round %d assistant = %#v", index+1, assistant)
		}
		var reply string
		for _, part := range assistant.Parts {
			var probe struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}
			if json.Unmarshal(part, &probe) == nil && probe.Type == "text" {
				reply = probe.Text
			}
		}
		if !strings.HasPrefix(reply, "MOCK-REPLY-") {
			t.Fatalf("round %d assistant reply %q did not come from the local mock", index+1, reply)
		}
		if index > 0 && messageID <= firstMessageID {
			t.Fatalf("message ids are not ascending: %q then %q", firstMessageID, messageID)
		}
	}
	if err := client.Abort(ctx, sessionID); err != nil {
		t.Fatalf("live Abort(): %v", err)
	}
	if model.hits.Load() == 0 {
		t.Fatal("mock provider was never called")
	}
}

func assertJSONBody(t *testing.T, r *http.Request, want string) {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	if err := json.Unmarshal(raw, &gotValue); err != nil {
		t.Fatalf("invalid request JSON %q: %v", raw, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(gotValue)
	expected, _ := json.Marshal(wantValue)
	if string(got) != string(expected) {
		t.Fatalf("body = %s, want %s", got, expected)
	}
}
