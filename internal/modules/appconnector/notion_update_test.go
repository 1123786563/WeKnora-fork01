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
	"sync"
	"testing"
	"time"
)

func updateArgs(pageID, version, title string, blocks []any) json.RawMessage {
	if blocks == nil {
		blocks = []any{paragraphBlock("hello")}
	}
	raw, _ := json.Marshal(map[string]any{
		"page_id":          pageID,
		"expected_version": version,
		"title":            title,
		"blocks":           blocks,
	})
	return raw
}

func paragraphBlock(text string) map[string]any {
	return map[string]any{
		"object": "block",
		"type":   "paragraph",
		"paragraph": map[string]any{
			"rich_text": []any{map[string]any{
				"type": "text",
				"text": map[string]string{"content": text},
			}},
		},
	}
}

func TestParseNotionUpdateSnapshotAcceptsExactFourFields(t *testing.T) {
	snap, err := ParseNotionUpdateSnapshot(updateArgs("page-1", "2026-09-24T10:00:00.000Z", "T", nil))
	if err != nil {
		t.Fatal(err)
	}
	if snap.PageID != "page-1" || snap.ExpectedVersion != "2026-09-24T10:00:00.000Z" || snap.Title != "T" || len(snap.Blocks) != 1 {
		t.Fatalf("snapshot fields drift: %+v", snap)
	}
}

func TestParseNotionUpdateSnapshotRejectsExtraField(t *testing.T) {
	raw := append([]byte{}, updateArgs("page-1", "v", "T", nil)...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseNotionUpdateSnapshot(raw); !errors.Is(err, ErrNotionSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
}

func TestParseNotionUpdateSnapshotRejectsMissingOrEmpty(t *testing.T) {
	cases := map[string]json.RawMessage{
		"missing page":    []byte(`{"expected_version":"v","title":"T","blocks":[]}`),
		"missing version": []byte(`{"page_id":"p","title":"T","blocks":[]}`),
		"empty page":      updateArgs("", "v", "T", nil),
		"empty version":   updateArgs("p", "", "T", nil),
		"empty title":     updateArgs("p", "v", "", nil),
		"not an object":   []byte(`["page_id"]`),
		"three fields":    []byte(`{"page_id":"p","title":"T","blocks":[]}`),
	}
	for name, raw := range cases {
		if _, err := ParseNotionUpdateSnapshot(raw); !errors.Is(err, ErrNotionSnapshotInvalid) {
			t.Fatalf("%s: want ErrNotionSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsNotionUpdateArgs(t *testing.T) {
	if !IsNotionUpdateArgs(updateArgs("p", "v", "T", nil)) {
		t.Fatal("update-shaped args must be detected")
	}
	create, _ := json.Marshal(map[string]any{"parent": "pp", "title": "T", "blocks": []any{}})
	if IsNotionUpdateArgs(create) {
		t.Fatal("create-shaped args must not be treated as update")
	}
	if IsNotionUpdateArgs([]byte(`not json`)) {
		t.Fatal("garbage must not be update")
	}
}

func TestDetectNotionVersionConflict(t *testing.T) {
	if err := DetectNotionVersionConflict("v1", "v1"); err != nil {
		t.Fatalf("matching version must pass: %v", err)
	}
	if err := DetectNotionVersionConflict("v1", "v2"); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("drift must conflict, got %v", err)
	}
	if err := DetectNotionVersionConflict("", "v2"); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("empty expected must fail closed, got %v", err)
	}
	if err := DetectNotionVersionConflict("v1", ""); !errors.Is(err, ErrNotionVersionConflict) {
		t.Fatalf("unreadable remote must fail closed, got %v", err)
	}
}

func TestParseNotionPageVersion(t *testing.T) {
	v, err := ParseNotionPageVersion([]byte(`{"object":"page","id":"p1","last_edited_time":"2026-09-24T10:00:00.000Z","parent":{"type":"page_id","page_id":"pp"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if v.PageID != "p1" || v.LastEditedTime != "2026-09-24T10:00:00.000Z" {
		t.Fatalf("version fields drift: %+v", v)
	}
	if _, err := ParseNotionPageVersion([]byte(`{"object":"page","id":"p1"}`)); err == nil {
		t.Fatal("missing last_edited_time must be an error, never a fabricated version")
	}
	if _, err := ParseNotionPageVersion([]byte(`{"object":"page","last_edited_time":"v"}`)); err == nil {
		t.Fatal("missing id must be an error")
	}
}

func TestParseNotionPageReceipt(t *testing.T) {
	r, err := ParseNotionPageReceipt([]byte(`{"object":"page","id":"p1","last_edited_time":"v9"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r.ExternalID != "p1" || r.ExternalVersion != "v9" {
		t.Fatalf("receipt fields drift: %+v", r)
	}
	if _, err := ParseNotionPageReceipt([]byte(`{"object":"page","id":""}`)); err == nil {
		t.Fatal("no real page id must refuse a receipt")
	}
}

// fakeNotionDocs is a LOCAL contract double of the official Notion page
// endpoints (developers.notion.com): POST/GET/PATCH /v1/pages and
// PATCH/GET /v1/blocks/{id}/children, carrying last_edited_time exactly
// like the real page object. It is NOT the real-provider acceptance — that
// stays NOTION_TOKEN-gated (Task 9, notion_publish_real_test.go).
type fakeNotionDocs struct {
	mu           sync.Mutex
	token        string
	nextID       int
	pages        map[string]*fakeNotionPage
	patchCalls   int // page PATCHes (title updates)
	appendCalls  int
	pageGets     int
	titlePatches []string
	// knobs: apply the effect then lose the reply (unknown outcome).
	dropNextTitlePatch bool
	dropNextAppend     bool
	// dropVersionGetFrom: once pageGets reaches this count, page GET replies
	// are lost (0 = off). The pre-read is GET #1; the final read-back is
	// GET #2 — set 2 to lose only the read-back after the writes landed.
	dropVersionGetFrom int
}

type fakeNotionPage struct {
	id, parent, title, lastEdited string
	children                      []json.RawMessage
}

func newFakeNotionDocs(token string) *fakeNotionDocs {
	return &fakeNotionDocs{token: token, pages: map[string]*fakeNotionPage{}}
}

func (f *fakeNotionDocs) addPage(id, parent, title string) *fakeNotionPage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &fakeNotionPage{id: id, parent: parent, title: title, lastEdited: "2026-09-24T08:00:00.000Z"}
	f.pages[id] = p
	return p
}

// touch bumps a page's last_edited_time — an external collaborator's edit.
func (f *fakeNotionDocs) touch(id, when string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.lastEdited = when
	}
}

func (f *fakeNotionDocs) stats() (patch, appends, gets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.patchCalls, f.appendCalls, f.pageGets
}

func (f *fakeNotionDocs) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	mux.HandleFunc("/v1/pages", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		if r.Method != http.MethodPost {
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Parent struct {
				PageID string `json:"page_id"`
			} `json:"parent"`
			Properties struct {
				Title struct {
					Title []struct {
						Text struct {
							Content string `json:"content"`
						} `json:"text"`
					} `json:"title"`
				} `json:"title"`
			} `json:"properties"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		f.nextID++
		id := fmt.Sprintf("page-%d", f.nextID)
		f.pages[id] = &fakeNotionPage{id: id, parent: req.Parent.PageID, title: req.Properties.Title.Title[0].Text.Content, lastEdited: "2026-09-24T09:00:00.000Z"}
		f.mu.Unlock()
		writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":"2026-09-24T09:00:00.000Z","parent":{"type":"page_id","page_id":%q}}`, id, req.Parent.PageID))
	})
	mux.HandleFunc("/v1/pages/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		id := r.URL.Path[len("/v1/pages/"):]
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			f.pageGets++
			drop := f.dropVersionGetFrom > 0 && f.pageGets >= f.dropVersionGetFrom
			edited := p.lastEdited
			f.mu.Unlock()
			if drop {
				// A read has no side effect to apply, but losing its reply
				// still leaves the client unable to observe the outcome —
				// exactly the final read-back case under test.
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, edited, p.parent))
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Properties struct {
					Title struct {
						Title []struct {
							Text struct {
								Content string `json:"content"`
							} `json:"text"`
						} `json:"title"`
					} `json:"title"`
				} `json:"properties"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.patchCalls++
			f.titlePatches = append(f.titlePatches, req.Properties.Title.Title[0].Text.Content)
			p.title = req.Properties.Title.Title[0].Text.Content
			p.lastEdited = "2026-09-24T11:00:00.000Z"
			drop := f.dropNextTitlePatch
			if drop {
				f.dropNextTitlePatch = false
			}
			f.mu.Unlock()
			if drop {
				// Effect applied, response lost: the client cannot observe
				// the outcome.
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, p.lastEdited, p.parent))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+f.token {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		rest := r.URL.Path[len("/v1/blocks/"):]
		var id string
		if suffix := "/children"; len(rest) > len(suffix) && rest[len(rest)-len(suffix):] == suffix {
			id = rest[:len(rest)-len(suffix)]
		} else {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			writeJSON(w, 404, `{"object":"error","status":404,"code":"object_not_found","message":"x"}`)
			return
		}
		switch r.Method {
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Children []json.RawMessage `json:"children"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.appendCalls++
			p.children = append(p.children, req.Children...)
			p.lastEdited = "2026-09-24T12:00:00.000Z"
			drop := f.dropNextAppend
			if drop {
				f.dropNextAppend = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			f.mu.Lock()
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinRaw(results)))
		case http.MethodGet:
			f.mu.Lock()
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinRaw(results)))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func joinRaw(items []json.RawMessage) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += string(it)
	}
	return out
}

// loopbackPolicy mirrors the documented test hook (http_policy.go:63-66):
// the reviewed policy shape for the Notion contract, with 127.0.0.0/8
// authorized so the httptest double is reachable.
func loopbackPolicy(host, port string) HTTPPolicy {
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	return HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network},
		Timeout:            10 * time.Second,
	}
}

func policyOfServer(srv *httptest.Server) HTTPPolicy {
	u := srv.URL // http://127.0.0.1:port
	host, port, _ := net.SplitHostPort(u[len("http://"):])
	return loopbackPolicy(host, port)
}

func newUpdateAdapter(srv *httptest.Server, progress map[string]NotionPageProgress) *NotionUpdateAdapter {
	return &NotionUpdateAdapter{
		Policy:                 policyOfServer(srv),
		Token:                  func(ctx context.Context) (string, error) { return "secret_test_token", nil },
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) { return []string{NotionCapabilityInsert}, nil },
		LoadProgress:           func(a Action) NotionPageProgress { return progress[a.ID] },
		SaveProgress:           func(a Action, p NotionPageProgress) error { progress[a.ID] = p; return nil },
	}
}

func updateAction(args json.RawMessage) Action {
	return Action{ID: "act_upd_1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-notion",
		Version: "notion/v1", Target: "page-9", Risk: RiskWrite, Args: args}
}

func TestNotionUpdatePublishesTitleAndBlocksHappyPath(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	progress := map[string]NotionPageProgress{}
	ad := newUpdateAdapter(srv, progress)

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("update: state=%s err=%v", out.State, err)
	}
	if out.ExternalID != "page-9" {
		t.Fatalf("external id: %+v", out)
	}
	rcpt, rerr := ParseNotionPageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != "page-9" || rcpt.ExternalVersion == "" {
		t.Fatalf("output must be a receipt-bearing page payload: %+v %v", rcpt, rerr)
	}
	patch, appends, _ := fake.stats()
	if patch != 1 || appends != 1 {
		t.Fatalf("wire counts: patch=%d appends=%d", patch, appends)
	}
	if progress["act_upd_1"].PageID != "page-9" || progress["act_upd_1"].BlocksDone != 1 {
		t.Fatalf("progress must record the resume point: %+v", progress["act_upd_1"])
	}
}

func TestNotionUpdateConflictZeroWrites(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	// External collaborator edited AFTER the plan was formed.
	fake.touch("page-9", "2026-09-24T10:30:00.000Z")

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if !errors.Is(err, ErrNotionVersionConflict) || out.State != ActionFailed {
		t.Fatalf("version drift must fail definitively: state=%s err=%v", out.State, err)
	}
	patch, appends, gets := fake.stats()
	if patch != 0 || appends != 0 {
		t.Fatalf("conflict must leave ZERO write requests, got patch=%d appends=%d", patch, appends)
	}
	if gets == 0 {
		t.Fatal("the pre-read version GET must have run")
	}
}

func TestNotionUpdatePreReadFailureIsFailedNotUnknown(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	// page-missing: the pre-read 404s — nothing was written, so this is a
	// definitive failure, never an unknown.
	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-missing", "v", "T", nil)))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("missing page must fail definitively: state=%s err=%v", out.State, err)
	}
	patch, appends, _ := fake.stats()
	if patch != 0 || appends != 0 {
		t.Fatalf("no write may follow an unreadable pre-read: %d/%d", patch, appends)
	}
}

func TestNotionUpdateUnknownOnLostWriteReply(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	fake.mu.Lock()
	fake.dropNextTitlePatch = true
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if out.State != ActionUnknown || !errors.Is(err, ErrNotionOutcomeUnknown) {
		t.Fatalf("lost write reply must park unknown, got state=%s err=%v", out.State, err)
	}
}

// TestNotionUpdateUnknownOnLostFinalReadBack pins the reliable read-back
// contract after the writes landed: a lost reply on the FINAL page version
// read parks unknown (the effect exists — never failed, AC2) and keeps the
// external id so Query can reconcile.
func TestNotionUpdateUnknownOnLostFinalReadBack(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	fake.dropVersionGetFrom = 2 // pre-read (GET #1) passes; the final read-back (GET #2) is lost
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})

	out, err := ad.Execute(context.Background(), updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil)))
	if out.State != ActionUnknown || !errors.Is(err, ErrNotionOutcomeUnknown) {
		t.Fatalf("lost final read-back must park unknown (never failed), got state=%s err=%v", out.State, err)
	}
	if out.ExternalID != "page-9" {
		t.Fatalf("external id must survive a lost read-back for reconcile, got %q", out.ExternalID)
	}
}

func TestNotionUpdateQueryResolvesAfterDroppedAppend(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	progress := map[string]NotionPageProgress{}
	ad := newUpdateAdapter(srv, progress)
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()

	act := updateAction(updateArgs("page-9", "2026-09-24T08:00:00.000Z", "new title", nil))
	out, err := ad.Execute(context.Background(), act)
	if out.State != ActionUnknown {
		t.Fatalf("dropped append reply must park unknown, got %s (%v)", out.State, err)
	}
	// AC2: reconcile by READING the remote first — the effect applied, so
	// the query must confirm success without any new write.
	_, appendsBefore, _ := fake.stats()
	q, qerr := ad.Query(context.Background(), act)
	if qerr != nil || q.State != ActionSucceeded || q.ExternalID != "page-9" {
		t.Fatalf("query must resolve the unknown from the remote state: %+v %v", q, qerr)
	}
	_, appendsAfter, _ := fake.stats()
	if appendsAfter != appendsBefore {
		t.Fatalf("query must not re-send: appends %d -> %d", appendsBefore, appendsAfter)
	}
}

func TestNotionUpdateQueryStaysUnknownWhenBlocksMissing(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	// The page still has ZERO children: not provably complete — honest
	// unknown, never fabricated success or failure.
	q, _ := ad.Query(context.Background(), updateAction(updateArgs("page-9", "v", "T", nil)))
	if q.State != ActionUnknown {
		t.Fatalf("unprovable state must stay unknown, got %s", q.State)
	}
}

func TestNotionUpdateQueryVerifiesContainedRun(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	srv := fake.server(t)
	// Earlier foreign blocks precede ours: our approved blocks must be
	// found as a contiguous run, not only as the tail.
	fake.mu.Lock()
	if p := fake.pages["page-9"]; p != nil {
		p.children = []json.RawMessage{mustJSONBlock("earlier foreign"), mustJSONBlock("hello")}
	}
	fake.mu.Unlock()
	ad := newUpdateAdapter(srv, map[string]NotionPageProgress{})
	q, err := ad.Query(context.Background(), updateAction(updateArgs("page-9", "v", "T", nil)))
	if err != nil || q.State != ActionSucceeded {
		t.Fatalf("contained run must verify: %+v %v", q, err)
	}
}

func mustJSONBlock(text string) json.RawMessage {
	b, _ := json.Marshal(paragraphBlock(text))
	return b
}

func TestNotionUpdateResumesOnlyRemainingBlocks(t *testing.T) {
	fake := newFakeNotionDocs("secret_test_token")
	fake.addPage("page-9", "parent-1", "old title")
	// Faithful crash simulation: the FIRST block was delivered to the
	// provider before the crash (progress persisted BlocksDone=1 AFTER a
	// successful append), so the remote page already carries it.
	fake.mu.Lock()
	fake.pages["page-9"].children = []json.RawMessage{mustJSONBlock("one")}
	fake.mu.Unlock()
	srv := fake.server(t)
	progress := map[string]NotionPageProgress{"act_upd_1": {PageID: "page-9", BlocksDone: 1}}
	ad := newUpdateAdapter(srv, progress)
	args, _ := json.Marshal(map[string]any{
		"page_id": "page-9", "expected_version": "2026-09-24T08:00:00.000Z",
		"title": "T", "blocks": []any{paragraphBlock("one"), paragraphBlock("two")},
	})
	out, err := ad.Execute(context.Background(), updateAction(args))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("resume: state=%s err=%v", out.State, err)
	}
	fake.mu.Lock()
	appended := len(fake.pages["page-9"].children)
	fake.mu.Unlock()
	// Correct resume appends ONLY blocks[1:2] → children = {one, two} = 2.
	// A restart-from-zero bug re-appends both → 3; a no-op resume → 1.
	if appended != 2 {
		t.Fatalf("resume must not duplicate the first block: children=%d", appended)
	}
}
