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

func TestNotionPartialSuccessDoesNotCreateAnotherPage(t *testing.T) {
	if NextNotionStep("page1", false) != "append_remaining" {
		t.Fatal("would duplicate page")
	}
	if NextNotionStep("page1", true) != "complete" {
		t.Fatal("completion lost")
	}
}

// ---- Planned cases ----

func TestNotionStepRoutingAndUnknownNeverRouted(t *testing.T) {
	if NextNotionStep("", false) != "create" {
		t.Fatal("empty pageID must create")
	}
	if NextNotionStep("", true) != "create" {
		t.Fatal("no persisted page means nothing completed")
	}
	// unknown_create must never be routed through NextNotionStep: an
	// unknown creation has no confirmed pageID to resume from.
	if CanRouteNotionStep(ActionUnknown) {
		t.Fatal("unknown must reconcile first, never route a step")
	}
	if !CanRouteNotionStep(ActionDispatched) || !CanRouteNotionStep(ActionSucceeded) {
		t.Fatal("confirmed states may route their next step")
	}
}

func TestNotionNO01ContractPinned(t *testing.T) {
	if NotionAPIHost != "api.notion.com" {
		t.Fatal(NotionAPIHost)
	}
	if NotionAPIVersion != "2022-06-28" {
		t.Fatal(NotionAPIVersion)
	}
	if NotionCreatePagePath != "/v1/pages" {
		t.Fatal(NotionCreatePagePath)
	}
	if NotionAppendChildrenFormat != "/v1/blocks/%s/children" {
		t.Fatalf("format constant drift: %q", NotionAppendChildrenFormat)
	}
	if NotionCapabilityInsert != "insert_content" {
		t.Fatal(NotionCapabilityInsert)
	}
	if NotionAppendBatchLimit != 100 {
		t.Fatal("append batch must respect the documented 100-block cap")
	}
}

// fakeNotion is a LOCAL contract double of the NO-01 endpoints. It is NOT
// NO-04: no real Notion acceptance is claimed through it.
type fakeNotion struct {
	mu sync.Mutex
	// pages ever CREATED remotely (hijacked responses still save).
	creates      int
	createParent map[string]string            // page id -> parent page id
	createTitle  map[string]string            // page id -> title
	children     map[string][]json.RawMessage // page id -> appended blocks
	// captured first create request (headers + body pieces).
	capturedNotionVersion string
	capturedAuth          string
	capturedParent        string
	capturedTitle         string
	appends               []string // raw children arrays of every append attempt
	appendedTotal         int
	searches              int
	wire                  int  // every request that reached the server
	appendFailNext        int  // fail the next N append requests (provider error)
	appendFailOnNth       int  // fail the Nth append request, counting every request (provider error)
	createDrop            bool // save the page, then lose the response
	decoyInSearch         bool // search returns a same-title page under the approved parent
}

func (f *fakeNotion) stats() (creates, appended, searches, wire int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.creates, f.appendedTotal, f.searches, f.wire
}

func (f *fakeNotion) pageChildren(id string) []json.RawMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]json.RawMessage(nil), f.children[id]...)
}

func (f *fakeNotion) writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"object":"error","status":%d,"code":%q,"message":%q}`, status, code, msg)
}

func (f *fakeNotion) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	pagesHandler := func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.wire++
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
		drop := f.createDrop
		var id string
		if r.Method == http.MethodPost {
			f.creates++
			id = fmt.Sprintf("real_page_%d", f.creates)
			if f.createParent == nil {
				f.createParent = map[string]string{}
				f.createTitle = map[string]string{}
				f.children = map[string][]json.RawMessage{}
			}
			f.createParent[id] = req.Parent.PageID
			f.createTitle[id] = ""
			if len(req.Properties.Title.Title) > 0 {
				f.createTitle[id] = req.Properties.Title.Title[0].Text.Content
			}
			if f.capturedParent == "" {
				f.capturedNotionVersion = r.Header.Get("Notion-Version")
				f.capturedAuth = r.Header.Get("Authorization")
				f.capturedParent = req.Parent.PageID
				f.capturedTitle = f.createTitle[id]
			}
		} else if r.Method == http.MethodGet {
			id = strings.TrimPrefix(r.URL.Path, "/v1/pages/")
		}
		parent := f.createParent[id]
		f.mu.Unlock()
		if drop {
			hj, ok := w.(http.Hijacker)
			if !ok {
				panic("fake notion: cannot hijack")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				panic("fake notion: hijack failed: " + err.Error())
			}
			_ = conn.Close() // remote saved the page; the response is lost
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"object":"page","id":%q,"parent":{"type":"page_id","page_id":%q}}`, id, parent)
	}
	// The exact pattern serves POST /v1/pages (create); the subtree
	// pattern serves the reliable read GET /v1/pages/{id} used by
	// reconciliation.
	mux.HandleFunc("/v1/pages", pagesHandler)
	mux.HandleFunc("/v1/pages/", pagesHandler)
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.wire++
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Children []json.RawMessage `json:"children"`
		}
		_ = json.Unmarshal(body, &req)
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/blocks/"), "/children")
		if r.Method == http.MethodGet {
			// Reliable read of a page's stored children (reconciliation
			// path); the write path below is unchanged.
			kids := append([]json.RawMessage(nil), f.children[id]...)
			f.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			out, _ := json.Marshal(struct {
				Object  string            `json:"object"`
				Results []json.RawMessage `json:"results"`
			}{"list", kids})
			_, _ = w.Write(out)
			return
		}
		f.appends = append(f.appends, string(body))
		fail := false
		if f.appendFailNext > 0 {
			f.appendFailNext--
			fail = true
		} else if f.appendFailOnNth > 0 && len(f.appends) == f.appendFailOnNth {
			fail = true
		}
		if fail {
			f.mu.Unlock()
			f.writeErr(w, http.StatusInternalServerError, "internal_server_error", "append rejected")
			return
		}
		f.children[id] = append(f.children[id], req.Children...)
		f.appendedTotal += len(req.Children)
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"object":"list","results":[]}`)
	})
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.wire++
		f.searches++
		decoy := f.decoyInSearch
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if decoy {
			// A page with the SAME title under the approved parent that
			// THIS action did not necessarily create.
			fmt.Fprint(w, `{"object":"list","results":[{"object":"page","id":"real_page_1","parent":{"type":"page_id","page_id":"p_approved"},"properties":{"title":{"title":[{"text":{"content":"Q3 Board Review"}}]}}}]}`)
			return
		}
		fmt.Fprint(w, `{"object":"list","results":[]}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func notionNorm(t *testing.T, raw string) json.RawMessage {
	t.Helper()
	norm, err := NormalizeArgs(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return norm
}

func notionBlocks(t *testing.T, n int) []json.RawMessage {
	t.Helper()
	out := make([]json.RawMessage, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, notionNorm(t, fmt.Sprintf(
			`{"object":"block","type":"paragraph","paragraph":{"rich_text":[{"type":"text","text":{"content":"line %d"}}]}}`, i)))
	}
	return out
}

func notionCreateAction(t *testing.T) Action {
	t.Helper()
	return Action{
		ID:           "act_notion_create_1",
		TenantID:     7,
		ActorID:      "user_1",
		ConnectionID: "conn_notion",
		Version:      "v1",
		Target:       "p_approved",
		Risk:         RiskWrite,
		Args: notionNorm(t, fmt.Sprintf(
			`{"parent":"p_approved","title":"Q3 Board Review","blocks":[{"object":"block","type":"paragraph","paragraph":{"rich_text":[{"type":"text","text":{"content":"line 1"}}]}},{"object":"block","type":"paragraph","paragraph":{"rich_text":[{"type":"text","text":{"content":"line 2"}}]}},{"object":"block","type":"paragraph","paragraph":{"rich_text":[{"type":"text","text":{"content":"line 3"}}]}}]}`)),
	}
}

func notionPolicy(host, port string) HTTPPolicy {
	_, loop, _ := net.ParseCIDR("127.0.0.0/8")
	return HTTPPolicy{
		Scheme:             "http",
		Host:               host,
		Port:               port,
		Methods:            []string{http.MethodPost, http.MethodPatch, http.MethodGet},
		PathPrefix:         "/v1/",
		AuthorizedNetworks: []*net.IPNet{loop},
		Resolver: IPResolverFunc(func(ctx context.Context, host string) ([]net.IPAddr, error) {
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}),
		Timeout: 5 * time.Second,
	}
}

type notionProgressStore struct {
	mu sync.Mutex
	p  NotionPageProgress
}

func (s *notionProgressStore) save(_ Action, p NotionPageProgress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.p = p
	return nil
}

func (s *notionProgressStore) load(_ Action) NotionPageProgress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.p
}

func newNotionAdapter(t *testing.T, f *fakeNotion, store *notionProgressStore) *NotionCreateAdapter {
	t.Helper()
	ts := f.server(t)
	port := strconv.Itoa(ts.Listener.Addr().(*net.TCPAddr).Port)
	ad := &NotionCreateAdapter{
		Policy:          notionPolicy("notion.test", port),
		Token:           func(ctx context.Context) (string, error) { return "secret_test_integration_token", nil },
		ApprovedParents: []string{"p_approved"},
		ConnectionCapabilities: func(ctx context.Context, a Action) ([]string, error) {
			return []string{NotionCapabilityInsert, "read_content"}, nil
		},
	}
	if store != nil {
		ad.LoadProgress = store.load
		ad.SaveProgress = store.save
	}
	return ad
}

func TestNotionParentOutOfScopeRejectedZeroWire(t *testing.T) {
	f := &fakeNotion{}
	ad := newNotionAdapter(t, f, nil)
	a := notionCreateAction(t)
	// The approved parent scope names a DIFFERENT page; the snapshot's
	// parent is out of the approval / pre-authorization scope.
	ad.ApprovedParents = []string{"p_other_approved"}
	out, err := ad.Execute(context.Background(), a)
	if !errors.Is(err, ErrNotionParentOutOfScope) {
		t.Fatalf("out-of-scope parent must be rejected, got %v", err)
	}
	if out.State != ActionFailed {
		t.Fatalf("state = %q", out.State)
	}
	if _, _, _, wire := f.stats(); wire != 0 {
		t.Fatalf("out-of-scope parent reached the wire: %d requests", wire)
	}
	// Empty scope list is fail-closed for every parent.
	ad.ApprovedParents = nil
	if _, err := ad.Execute(context.Background(), a); !errors.Is(err, ErrNotionParentOutOfScope) {
		t.Fatalf("empty scope must fail closed, got %v", err)
	}
}

func TestNotionApprovalRevokedBeforeExecuteBlocks(t *testing.T) {
	f := &fakeNotion{}
	ad := newNotionAdapter(t, f, nil)
	ad.Recheck = func(ctx context.Context, a Action) error {
		return errors.New("approval revoked by admin after authorization")
	}
	out, err := ad.Execute(context.Background(), notionCreateAction(t))
	if !errors.Is(err, ErrNotionApprovalRevoked) {
		t.Fatalf("revocation must surface, got %v", err)
	}
	if out.State != ActionAwaitingApproval {
		t.Fatalf("revoked action must stay parked awaiting approval, got %q", out.State)
	}
	if _, _, _, wire := f.stats(); wire != 0 {
		t.Fatalf("revoked approval still dispatched: %d requests", wire)
	}
}

func TestNotionMissingInsertCapabilityBlocks(t *testing.T) {
	f := &fakeNotion{}
	ad := newNotionAdapter(t, f, nil)
	ad.ConnectionCapabilities = func(ctx context.Context, a Action) ([]string, error) {
		return []string{"read_content"}, nil // no insert_content
	}
	out, err := ad.Execute(context.Background(), notionCreateAction(t))
	if !errors.Is(err, ErrNotionMissingCapability) {
		t.Fatalf("missing insert capability must block, got %v", err)
	}
	if out.State != ActionFailed {
		t.Fatalf("state = %q", out.State)
	}
	if _, _, _, wire := f.stats(); wire != 0 {
		t.Fatalf("uncapable connection reached the wire: %d requests", wire)
	}
}

func TestNotionSuccessSavesRealPageID(t *testing.T) {
	f := &fakeNotion{}
	ad := newNotionAdapter(t, f, nil)
	out, err := ad.Execute(context.Background(), notionCreateAction(t))
	if err != nil {
		t.Fatal(err)
	}
	if out.State != ActionSucceeded {
		t.Fatalf("state = %q", out.State)
	}
	if out.ExternalID != "real_page_1" {
		t.Fatalf("the REAL provider page id must be saved, got %q", out.ExternalID)
	}
	if !strings.Contains(string(out.Output), "real_page_1") {
		t.Fatalf("raw provider payload must be kept, got %s", out.Output)
	}
	creates, appended, _, _ := f.stats()
	if creates != 1 || appended != 3 {
		t.Fatalf("creates=%d appended=%d", creates, appended)
	}
}

func TestNotionSnapshotFieldByFieldPinned(t *testing.T) {
	f := &fakeNotion{}
	ad := newNotionAdapter(t, f, nil)
	if _, err := ad.Execute(context.Background(), notionCreateAction(t)); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.capturedNotionVersion != NotionAPIVersion {
		t.Fatalf("Notion-Version header = %q", f.capturedNotionVersion)
	}
	if f.capturedAuth != "Bearer secret_test_integration_token" {
		t.Fatalf("Authorization = %q", f.capturedAuth)
	}
	if f.capturedParent != "p_approved" {
		t.Fatalf("parent rewritten after approval: %q", f.capturedParent)
	}
	if f.capturedTitle != "Q3 Board Review" {
		t.Fatalf("title rewritten after approval: %q", f.capturedTitle)
	}
	want := notionBlocks(t, 3)
	if len(f.appends) == 0 {
		t.Fatal("no append request captured")
	}
	var sent struct {
		Children []json.RawMessage `json:"children"`
	}
	if err := json.Unmarshal([]byte(f.appends[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if len(sent.Children) != len(want) {
		t.Fatalf("append sent %d blocks, want %d", len(sent.Children), len(want))
	}
	for i, c := range sent.Children {
		got, err := NormalizeArgs(c)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want[i]) {
			t.Fatalf("block %d rewritten: %s", i, got)
		}
	}
	// The snapshot shape is fixed: an extra field is refused instead of
	// silently forwarded.
	if _, err := ParseNotionCreateSnapshot(notionNorm(t,
		`{"parent":"p_x","title":"t","blocks":[],"extra":1}`)); !errors.Is(err, ErrNotionSnapshotInvalid) {
		t.Fatalf("extra snapshot field must be rejected, got %v", err)
	}
	if _, err := ParseNotionCreateSnapshot(notionNorm(t,
		`{"parent":"","title":"t","blocks":[]}`)); !errors.Is(err, ErrNotionSnapshotInvalid) {
		t.Fatalf("empty parent must be rejected, got %v", err)
	}
}

func TestNotionCreateThenLocalCrashResumesWithoutDuplicate(t *testing.T) {
	f := &fakeNotion{appendFailNext: 1} // create OK, first append fails
	store := &notionProgressStore{}
	ad := newNotionAdapter(t, f, store)
	a := notionCreateAction(t)
	out, err := ad.Execute(context.Background(), a)
	if out.State != ActionFailed || err == nil {
		t.Fatalf("append failure after create must fail without re-create: %+v %v", out, err)
	}
	p := store.load(a)
	if p.PageID != "real_page_1" {
		t.Fatalf("page id must be persisted right after create, got %+v", p)
	}
	// Simulated local crash: a FRESH process recovers from the persisted
	// page id alone and resumes the remaining blocks.
	ad2 := newNotionAdapter(t, f, store)
	if NextNotionStep(p.PageID, false) != "append_remaining" {
		t.Fatal("recovery must resume, never re-create")
	}
	out2, err2 := ad2.Execute(context.Background(), a)
	if err2 != nil {
		t.Fatal(err2)
	}
	if out2.State != ActionSucceeded || out2.ExternalID != "real_page_1" {
		t.Fatalf("resumed execution must complete the SAME page: %+v", out2)
	}
	creates, appended, _, _ := f.stats()
	if creates != 1 {
		t.Fatalf("exactly one page must ever be created, got %d", creates)
	}
	if appended != 3 {
		t.Fatalf("all blocks appended exactly once, got %d", appended)
	}
}

func TestNotionPartialAppendSuccessResumesRemainingOnly(t *testing.T) {
	f := &fakeNotion{appendFailOnNth: 2}
	store := &notionProgressStore{}
	ad := newNotionAdapter(t, f, store)
	ad.MaxBatch = 2 // 3 blocks -> batch of 2 (succeeds) + batch of 1 (fails)
	a := notionCreateAction(t)
	out, err := ad.Execute(context.Background(), a)
	if out.State != ActionFailed || err == nil {
		t.Fatalf("partial append must surface failure, got %+v %v", out, err)
	}
	p := store.load(a)
	if p.PageID != "real_page_1" || p.BlocksDone != 2 {
		t.Fatalf("completed block range must be persisted after each step, got %+v", p)
	}
	if NextNotionStep(p.PageID, false) != "append_remaining" {
		t.Fatal("partial append must resume remaining blocks")
	}
	// Server is healthy again: only the remaining block is re-sent.
	f.mu.Lock()
	f.appendFailNext = 0
	f.mu.Unlock()
	out2, err2 := ad.Execute(context.Background(), a)
	if err2 != nil {
		t.Fatal(err2)
	}
	if out2.State != ActionSucceeded || out2.ExternalID != "real_page_1" {
		t.Fatalf("resumed execution must complete the SAME page: %+v", out2)
	}
	creates, appended, _, _ := f.stats()
	if creates != 1 || appended != 3 {
		t.Fatalf("duplicate page or blocks: creates=%d appended=%d", creates, appended)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	last := f.appends[len(f.appends)-1]
	var sent struct {
		Children []json.RawMessage `json:"children"`
	}
	_ = json.Unmarshal([]byte(last), &sent)
	if len(sent.Children) != 1 {
		t.Fatalf("resume must send ONLY the remaining block, got %d", len(sent.Children))
	}
	got, _ := NormalizeArgs(sent.Children[0])
	want := notionBlocks(t, 3)
	if string(got) != string(want[2]) {
		t.Fatalf("resumed block = %s, want the third snapshot block", got)
	}
}

func TestNotionUnknownCreateStaysUnknownTitleSearchNotProof(t *testing.T) {
	f := &fakeNotion{createDrop: true, decoyInSearch: true} // saved remotely, response lost
	store := &notionProgressStore{}
	ad := newNotionAdapter(t, f, store)
	a := notionCreateAction(t)
	out, err := ad.Execute(context.Background(), a)
	if out.State != ActionUnknown {
		t.Fatalf("lost create response must stay unknown, got %q", out.State)
	}
	if !errors.Is(err, ErrNotionOutcomeUnknown) {
		t.Fatalf("unknown must be explicit, got %v", err)
	}
	if p := store.load(a); p.PageID != "" {
		t.Fatalf("unknown create must not persist a fabricated page id: %+v", p)
	}
	if CanRouteNotionStep(out.State) {
		t.Fatal("unknown_create must never route through NextNotionStep")
	}
	// Reconciliation: search returns a page with the SAME title under the
	// SAME approved parent — still not proof THIS action created it.
	q, qerr := ad.Query(context.Background(), a)
	if q.State != ActionUnknown {
		t.Fatalf("title-search results are never sole proof: %+v", q)
	}
	if q.ExternalID != "" {
		t.Fatalf("no page id may be claimed from a title match: %+v", q)
	}
	_ = qerr
	creates, _, searches, _ := f.stats()
	if creates != 1 || searches != 1 {
		t.Fatalf("no re-create allowed after unknown: creates=%d searches=%d", creates, searches)
	}
}

func TestNotionQueryResolvesPersistedPageByReliableRead(t *testing.T) {
	f := &fakeNotion{}
	store := &notionProgressStore{}
	ad := newNotionAdapter(t, f, store)
	a := notionCreateAction(t)
	if _, err := ad.Execute(context.Background(), a); err != nil {
		t.Fatal(err)
	}
	q, qerr := ad.Query(context.Background(), a)
	if qerr != nil {
		t.Fatal(qerr)
	}
	if q.State != ActionSucceeded || q.ExternalID != "real_page_1" {
		t.Fatalf("reliable page read must resolve the persisted page id: %+v", q)
	}
	// A persisted page whose blocks are NOT all present yet stays unknown:
	// the honest report is "not provably complete", never a fabricated
	// success.
	f.mu.Lock()
	f.children["real_page_1"] = f.children["real_page_1"][:1]
	f.mu.Unlock()
	q2, _ := ad.Query(context.Background(), a)
	if q2.State != ActionUnknown {
		t.Fatalf("incomplete content must not report success: %+v", q2)
	}
}
