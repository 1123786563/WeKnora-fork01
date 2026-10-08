package publish

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---- in-memory ports (module-level scenario doubles for the seams the
// production wiring supplies from the database / credential store) ----

type fakeScopes struct {
	scope NotionConnectionScope
	err   error
	calls int
}

func (f *fakeScopes) NotionScope(ctx context.Context, connectionID string) (NotionConnectionScope, error) {
	f.calls++
	return f.scope, f.err
}

type fakePolicies struct{ pol appconn.HTTPPolicy }

func (f *fakePolicies) PolicyFor(ctx context.Context, connectionID string) (appconn.HTTPPolicy, error) {
	return f.pol, nil
}

type fakeTokens struct{ tok string }

func (f *fakeTokens) Token(ctx context.Context, connectionID string, expectedVersion int64) (string, error) {
	return f.tok, nil
}

func openBridgeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.PublicationRow{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func bridgeTestStack(t *testing.T, fake *bridgeFakeNotion) (*NotionBridge, *repoappconn.PublicationStore) {
	t.Helper()
	db := openBridgeDB(t)
	pubs := repoappconn.NewPublicationStore(db)
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
	bridge := NewNotionBridge(
		&fakeScopes{scope: NotionConnectionScope{
			AppID: "notion", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
			ApprovedParents: []string{"parent-1"}, InsertCapability: true,
		}},
		&fakePolicies{pol: pol},
		&fakeTokens{tok: "secret_test_token"},
		pubs,
	)
	return bridge, pubs
}

func createSnapshotArgs(parent, title string) appconnectorsvc.ActionSnapshot {
	blocks, _ := json.Marshal([]any{map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "hello"},
		}}},
	}})
	args, _ := json.Marshal(map[string]any{"parent": parent, "title": title, "blocks": []any{json.RawMessage(blocks)}})
	return appconnectorsvc.ActionSnapshot{
		ID: "act-b1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-notion",
		Version: "notion/v1", Target: parent, Risk: "write",
		AuthVersion: 1, Args: args,
	}
}

func TestBridgeDispatchCreateRoutesToCreateAdapter(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	bridge, pubs := bridgeTestStack(t, fake)
	if err := pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-b1", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "create", Destination: "parent-1", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	out, err := bridge.Dispatch(context.Background(), createSnapshotArgs("parent-1", "T"), "conn-notion")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != appconn.ActionSucceeded || out.ExecutionID == "" {
		t.Fatalf("create dispatch outcome: %+v", out)
	}
	rcpt, rerr := appconn.ParseNotionPageReceipt([]byte(out.ProviderResult))
	if rerr != nil || rcpt.ExternalID != out.ExecutionID || rcpt.ExternalVersion == "" {
		t.Fatalf("ProviderResult must be the receipt-bearing page payload: %+v %v", rcpt, rerr)
	}
	row, perr := pubs.FindByAction(context.Background(), 7, "act-b1")
	if perr != nil || row.ProgressJSON == "" {
		t.Fatalf("progress must be durably recorded on the publication row: %+v %v", row, perr)
	}
}

func TestBridgeDispatchUpdateConflictIsDefinitiveFailure(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	fake.touch("page-9", "2026-09-24T10:30:00.000Z") // external drift
	bridge, pubs := bridgeTestStack(t, fake)
	if err := pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-b2", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	blocks, _ := json.Marshal([]any{map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "hello"},
		}}},
	}})
	args, _ := json.Marshal(map[string]any{
		"page_id": "page-9", "expected_version": "2026-09-24T08:00:00.000Z",
		"title": "T", "blocks": []any{json.RawMessage(blocks)},
	})
	snap := appconnectorsvc.ActionSnapshot{ID: "act-b2", TenantID: 7, ActorID: "u1",
		ConnectionID: "conn-notion", Version: "notion/v1", Target: "page-9", Risk: "write",
		AuthVersion: 1, Args: args}
	out, err := bridge.Dispatch(context.Background(), snap, "conn-notion")
	if err != nil {
		t.Fatalf("a definitive provider refusal must be an outcome, not an error: %v", err)
	}
	if out.Status != appconn.ActionFailed || !strings.HasPrefix(out.ProviderResult, PublishVersionConflictResult) {
		t.Fatalf("conflict must map to a failed outcome with the conflict marker: %+v", out)
	}
	if fake.patchCalls != 0 {
		t.Fatalf("conflict must write nothing, got %d patches", fake.patchCalls)
	}
}

func TestBridgeDispatchUnknownOutcomeParks(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	fake.mu.Lock()
	fake.dropNextAppend = true
	fake.mu.Unlock()
	bridge, pubs := bridgeTestStack(t, fake)
	if err := pubs.CreatePublication(context.Background(), repoappconn.PublicationRow{
		TenantID: 7, ActionID: "act-b3", ConnectionID: "conn-notion", Provider: "notion",
		Mode: "update", Destination: "page-9", State: repoappconn.PublicationPlanned,
	}); err != nil {
		t.Fatal(err)
	}
	blocks, _ := json.Marshal([]any{map[string]any{
		"object": "block", "type": "paragraph",
		"paragraph": map[string]any{"rich_text": []any{map[string]any{
			"type": "text", "text": map[string]string{"content": "hello"},
		}}},
	}})
	args, _ := json.Marshal(map[string]any{
		"page_id": "page-9", "expected_version": "2026-09-24T08:00:00.000Z",
		"title": "T", "blocks": []any{json.RawMessage(blocks)},
	})
	snap := appconnectorsvc.ActionSnapshot{ID: "act-b3", TenantID: 7, ActorID: "u1",
		ConnectionID: "conn-notion", Version: "notion/v1", Target: "page-9", Risk: "write",
		AuthVersion: 1, Args: args}
	out, err := bridge.Dispatch(context.Background(), snap, "conn-notion")
	if err != nil || out.Status != appconn.ActionUnknown {
		t.Fatalf("unobservable outcome must park unknown: %+v %v", out, err)
	}
	// AC2: reconcile by remote query FIRST — the effect applied, so the
	// query confirms; no re-dispatch ever happens here.
	q, qerr := bridge.QueryProvider(context.Background(), snap, "conn-notion")
	if qerr != nil || q.Status != appconn.ActionSucceeded {
		t.Fatalf("query must resolve from the remote state: %+v %v", q, qerr)
	}
	if fake.appendCalls != 1 {
		t.Fatalf("no re-dispatch: appends=%d", fake.appendCalls)
	}
}

func TestBridgeDispatchNonNotionConnectionFailsClosedPreSend(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	bridge, _ := bridgeTestStackWithScopes(t, fake, &fakeScopes{scope: NotionConnectionScope{AppID: "feishu"}})
	out, err := bridge.Dispatch(context.Background(), createSnapshotArgs("parent-1", "T"), "conn-feishu")
	if err == nil || out.Status != "" {
		t.Fatalf("non-notion connection must be a pre-send refusal, got %+v %v", out, err)
	}
	if !strings.Contains(err.Error(), "not a notion connection") {
		t.Fatalf("refusal must name the wiring gap: %v", err)
	}
	if fake.creates != 0 {
		t.Fatalf("nothing may be sent: creates=%d", fake.creates)
	}
}

func TestBridgeReadPageVersion(t *testing.T) {
	fake := newBridgeFakeNotion("secret_test_token")
	fake.addPage("page-9", "parent-1", "old")
	bridge, _ := bridgeTestStack(t, fake)
	v, err := bridge.ReadPageVersion(context.Background(), "conn-notion", "page-9")
	if err != nil || v != "2026-09-24T08:00:00.000Z" {
		t.Fatalf("version pre-read: %q %v", v, err)
	}
}

// bridgeFakeNotion: the same contract double as Task 2's fakeNotionDocs,
// duplicated here so the publish package's tests stay self-contained.
type bridgeFakeNotion struct {
	mu             sync.Mutex
	token          string
	nextID         int
	creates        int
	patchCalls     int
	appendCalls    int
	pages          map[string]*bridgeFakePage
	dropNextAppend bool
}

type bridgeFakePage struct {
	id, parent, title, lastEdited string
	children                      []json.RawMessage
}

func newBridgeFakeNotion(token string) *bridgeFakeNotion {
	return &bridgeFakeNotion{token: token, pages: map[string]*bridgeFakePage{}}
}

func (f *bridgeFakeNotion) addPage(id, parent, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[id] = &bridgeFakePage{id: id, parent: parent, title: title, lastEdited: "2026-09-24T08:00:00.000Z"}
}

func (f *bridgeFakeNotion) touch(id, when string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.lastEdited = when
	}
}

func (f *bridgeFakeNotion) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool { return r.Header.Get("Authorization") == "Bearer "+f.token }
	mux.HandleFunc("/v1/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
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
		f.creates++
		f.nextID++
		id := fmt.Sprintf("page-%d", f.nextID)
		f.pages[id] = &bridgeFakePage{id: id, parent: req.Parent.PageID, title: req.Properties.Title.Title[0].Text.Content, lastEdited: "2026-09-24T09:00:00.000Z"}
		f.mu.Unlock()
		writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":"2026-09-24T09:00:00.000Z","parent":{"type":"page_id","page_id":%q}}`, id, req.Parent.PageID))
	})
	mux.HandleFunc("/v1/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
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
			le := p.lastEdited
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
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
			p.title = req.Properties.Title.Title[0].Text.Content
			p.lastEdited = "2026-09-24T11:00:00.000Z"
			f.mu.Unlock()
			f.mu.Lock()
			le := p.lastEdited
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"page","id":%q,"last_edited_time":%q,"parent":{"type":"page_id","page_id":%q}}`, p.id, le, p.parent))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	mux.HandleFunc("/v1/blocks/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			writeJSON(w, 401, `{"object":"error","status":401,"code":"unauthorized","message":"x"}`)
			return
		}
		rest := r.URL.Path[len("/v1/blocks/"):]
		var id string
		if suffix := "/children"; strings.HasSuffix(rest, suffix) {
			id = strings.TrimSuffix(rest, suffix)
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
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinBridgeRaw(results)))
		case http.MethodGet:
			f.mu.Lock()
			results := append([]json.RawMessage(nil), p.children...)
			f.mu.Unlock()
			writeJSON(w, 200, fmt.Sprintf(`{"object":"list","results":[%s]}`, joinBridgeRaw(results)))
		default:
			writeJSON(w, 405, `{"object":"error","status":405,"code":"method_not_allowed","message":"x"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func joinBridgeRaw(items []json.RawMessage) string {
	out := ""
	for i, it := range items {
		if i > 0 {
			out += ","
		}
		out += string(it)
	}
	return out
}

// bridgeTestStackWithScopes lets a test inject its own scope source.
func bridgeTestStackWithScopes(t *testing.T, fake *bridgeFakeNotion, scopes NotionScopeSource) (*NotionBridge, *repoappconn.PublicationStore) {
	t.Helper()
	db := openBridgeDB(t)
	pubs := repoappconn.NewPublicationStore(db)
	srv := fake.server(t)
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	pol := appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PATCH"}, PathPrefix: "/v1/",
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
	return NewNotionBridge(scopes, &fakePolicies{pol: pol}, &fakeTokens{tok: "secret_test_token"}, pubs), pubs
}
