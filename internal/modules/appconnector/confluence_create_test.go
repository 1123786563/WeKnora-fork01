package appconnector

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeConfluence is a LOCAL contract double of the official Confluence
// REST wire for BOTH editions (developer.atlassian.com): Cloud v2
// GET/POST/PUT {base}/api/v2/pages and Server/DC GET/POST/PUT
// {base}/rest/api/content, with HTTP Basic auth and the monotonic
// version.number gate a real instance enforces on update. It is NOT the
// real-provider acceptance — that stays CONFLUENCE_*-gated (Task 9,
// confluence_publish_real_test.go).
type fakeConfluence struct {
	mu       sync.Mutex
	username string
	secret   string
	pages    map[string]*cfPage
	nextID   int
	// counters
	postCalls, putCalls, pageGets int
	// knob: apply the write effect, then lose the reply (unknown outcome).
	dropNextWrite bool
	// knob: serve page reads with HTML entities decoded to raw characters
	// — the real Confluence serializer writes back bare quotes/apostrophes
	// where Go's html.EscapeString emitted &#39;/&#34; (B5-F71).
	reserialize bool
	// lastCreateRaw captures the raw create request body for wire-shape
	// assertions (B5-F43).
	lastCreateRaw string
}

type cfPage struct {
	id, spaceID, spaceKey, title, storage string
	version                               int
}

func newFakeConfluence(username, secret string) *fakeConfluence {
	return &fakeConfluence{username: username, secret: secret, pages: map[string]*cfPage{}}
}

func (f *fakeConfluence) addPage(id, spaceID, spaceKey, title string, version int) *cfPage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &cfPage{id: id, spaceID: spaceID, spaceKey: spaceKey, title: title, version: version}
	f.pages[id] = p
	return p
}

// bump applies an EXTERNAL collaborator's edit: the version advances and
// (when storage is non-empty) the content changes.
func (f *fakeConfluence) bump(id, storage string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.version++
		if storage != "" {
			p.storage = storage
		}
	}
}

func (f *fakeConfluence) stats() (posts, puts, gets int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.postCalls, f.putCalls, f.pageGets
}

func cfAuthorized(r *http.Request, username, secret string) bool {
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+secret))
	return r.Header.Get("Authorization") == want
}

func cfCloudJSON(p *cfPage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"status":"current","title":%q,"spaceId":%q,"version":{"number":%d,"createdAt":"2026-09-25T08:00:00Z"}%s}`,
		p.id, p.title, p.spaceID, p.version, storage)
}

func cfServerJSON(p *cfPage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s,"representation":"storage"}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"type":"page","title":%q,"space":{"key":%q},"version":{"number":%d}%s}`,
		p.id, p.title, p.spaceKey, p.version, storage)
}

func (f *fakeConfluence) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	writeJSON := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !cfAuthorized(r, f.username, f.secret) {
			writeJSON(w, 401, `{"message":"unauthorized"}`)
			return
		}
		// The /wiki context path (Atlassian Cloud) is stripped before
		// routing so both bare and /wiki-prefixed doubles share one mux.
		path := strings.TrimPrefix(r.URL.Path, "/wiki")
		switch {
		case path == "/api/v2/pages" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			// The double decodes the OFFICIAL Cloud v2 contract — camelCase
			// spaceId/parentId keys only (B5-F43: an isomorphic snake_case
			// decode would mask a wire-contract regression).
			var req struct {
				SpaceID  string `json:"spaceId"`
				ParentID string `json:"parentId"`
				Status   string `json:"status"`
				Title    string `json:"title"`
				Body     struct {
					Representation string `json:"representation"`
					Value          string `json:"value"`
				} `json:"body"`
			}
			_ = json.Unmarshal(body, &req)
			f.mu.Lock()
			f.lastCreateRaw = string(body)
			parent, ok := f.pages[req.ParentID]
			f.mu.Unlock()
			if !ok || req.SpaceID == "" || req.Title == "" || req.Body.Value == "" {
				writeJSON(w, 400, `{"errors":[{"title":"invalid create request"}]}`)
				return
			}
			f.mu.Lock()
			f.nextID++
			f.postCalls++
			np := &cfPage{id: fmt.Sprintf("cf-%d", f.nextID), spaceID: req.SpaceID, spaceKey: parent.spaceKey,
				title: req.Title, storage: req.Body.Value, version: 1}
			f.pages[np.id] = np
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfCloudJSON(np, false))
		case strings.HasPrefix(path, "/api/v2/pages/") && r.Method == http.MethodGet:
			id := strings.TrimPrefix(path, "/api/v2/pages/")
			f.mu.Lock()
			f.pageGets++
			p, ok := f.pages[id]
			reserialize := f.reserialize
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"errors":[{"title":"not found"}]}`)
				return
			}
			if reserialize {
				served := *p
				served.storage = html.UnescapeString(p.storage)
				served.storage = strings.ReplaceAll(served.storage, "\r\n", "\n")
				writeJSON(w, 200, cfCloudJSON(&served, true))
				return
			}
			writeJSON(w, 200, cfCloudJSON(p, true))
		case strings.HasPrefix(path, "/api/v2/pages/") && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				ID     string `json:"id"`
				Status string `json:"status"`
				Title  string `json:"title"`
				Body   struct {
					Representation string `json:"representation"`
					Value          string `json:"value"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			id := strings.TrimPrefix(path, "/api/v2/pages/")
			f.mu.Lock()
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"errors":[{"title":"not found"}]}`)
				return
			}
			if req.Version.Number != p.version+1 {
				writeJSON(w, 409, `{"errors":[{"title":"version conflict"}]}`)
				return
			}
			f.mu.Lock()
			f.putCalls++
			p.title, p.storage, p.version = req.Title, req.Body.Value, req.Version.Number
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfCloudJSON(p, false))
		case path == "/rest/api/content" && r.Method == http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Type  string `json:"type"`
				Space struct {
					Key string `json:"key"`
				} `json:"space"`
				Ancestors []struct {
					ID string `json:"id"`
				} `json:"ancestors"`
				Title string `json:"title"`
				Body  struct {
					Storage struct {
						Value          string `json:"value"`
						Representation string `json:"representation"`
					} `json:"storage"`
				} `json:"body"`
			}
			_ = json.Unmarshal(body, &req)
			parentID := ""
			if len(req.Ancestors) > 0 {
				parentID = req.Ancestors[0].ID
			}
			f.mu.Lock()
			parent, ok := f.pages[parentID]
			f.mu.Unlock()
			if !ok || req.Space.Key == "" || req.Title == "" || req.Body.Storage.Value == "" {
				writeJSON(w, 400, `{"message":"invalid create request"}`)
				return
			}
			f.mu.Lock()
			f.nextID++
			f.postCalls++
			np := &cfPage{id: fmt.Sprintf("cf-%d", f.nextID), spaceID: parent.spaceID, spaceKey: req.Space.Key,
				title: req.Title, storage: req.Body.Storage.Value, version: 1}
			f.pages[np.id] = np
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfServerJSON(np, false))
		case strings.HasPrefix(path, "/rest/api/content/") && r.Method == http.MethodGet:
			id := strings.TrimPrefix(path, "/rest/api/content/")
			f.mu.Lock()
			f.pageGets++
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"message":"not found"}`)
				return
			}
			writeJSON(w, 200, cfServerJSON(p, true))
		case strings.HasPrefix(path, "/rest/api/content/") && r.Method == http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Type  string `json:"type"`
				ID    string `json:"id"`
				Title string `json:"title"`
				Body  struct {
					Storage struct {
						Value          string `json:"value"`
						Representation string `json:"representation"`
					} `json:"storage"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			id := strings.TrimPrefix(path, "/rest/api/content/")
			f.mu.Lock()
			p, ok := f.pages[id]
			f.mu.Unlock()
			if !ok {
				writeJSON(w, 404, `{"message":"not found"}`)
				return
			}
			if req.Version.Number != p.version+1 {
				writeJSON(w, 409, `{"message":"version conflict"}`)
				return
			}
			f.mu.Lock()
			f.putCalls++
			p.title, p.storage, p.version = req.Title, req.Body.Storage.Value, req.Version.Number
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			writeJSON(w, 200, cfServerJSON(p, false))
		default:
			writeJSON(w, 404, `{"message":"no route"}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// cfPolicyOf mirrors the documented loopback test hook (http_policy.go:63-66).
// It takes no *testing.T on purpose: several tests build adapters through
// helpers that have no t in scope.
func cfPolicyOf(srv *httptest.Server, apiBasePath string) HTTPPolicy {
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	prefix := "/"
	if apiBasePath != "" {
		prefix = apiBasePath + "/"
	}
	return HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PUT"}, PathPrefix: prefix,
		AuthorizedNetworks: []*net.IPNet{network}, Timeout: 10 * time.Second,
	}
}

func cfCredential(ctx context.Context) (ConfluenceCredential, error) {
	return ConfluenceCredential{Username: "cf-user@example.test", Secret: "secret_cf_token"}, nil
}

func cfCaps(caps ...string) func(ctx context.Context, a Action) ([]string, error) {
	return func(ctx context.Context, a Action) ([]string, error) { return caps, nil }
}

func cfCreateAdapter(srv *httptest.Server, edition, apiBasePath string) *ConfluenceCreateAdapter {
	return &ConfluenceCreateAdapter{
		Policy:                 cfPolicyOf(srv, apiBasePath),
		Credential:             cfCredential,
		Edition:                edition,
		APIBasePath:            apiBasePath,
		ApprovedParents:        []string{"parent-1"},
		ConnectionCapabilities: cfCaps(ConfluenceCapabilityWrite),
	}
}

func cfCreateArgs(parent, title, storage string) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"parent": parent, "title": title, "storage": storage})
	return raw
}

func cfCreateAction(args json.RawMessage) Action {
	return Action{ID: "act_cf_c1", TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		Version: "confluence/v1", Target: "parent-1", Risk: RiskWrite, Args: args}
}

func TestParseConfluenceCreateSnapshot(t *testing.T) {
	snap, err := ParseConfluenceCreateSnapshot(cfCreateArgs("parent-1", "T", "<p>x</p>"))
	if err != nil || snap.Parent != "parent-1" || snap.Title != "T" || snap.Storage != "<p>x</p>" {
		t.Fatalf("snapshot fields drift: %+v %v", snap, err)
	}
	raw := append([]byte{}, cfCreateArgs("p", "T", "s")...)
	raw = raw[:len(raw)-1]
	raw = append(raw, []byte(`,"evil":"x"}`)...)
	if _, err := ParseConfluenceCreateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
		t.Fatalf("approve-then-rewrite must be refused, got %v", err)
	}
	for name, raw := range map[string]json.RawMessage{
		"missing parent": []byte(`{"title":"T","storage":"s"}`),
		"missing title":  []byte(`{"parent":"p","storage":"s"}`),
		"empty storage":  cfCreateArgs("p", "T", ""),
		"empty parent":   cfCreateArgs("", "T", "s"),
		"two fields":     []byte(`{"parent":"p","title":"T"}`),
		"not an object":  []byte(`["parent"]`),
	} {
		if _, err := ParseConfluenceCreateSnapshot(raw); !errors.Is(err, ErrConfluenceSnapshotInvalid) {
			t.Fatalf("%s: want ErrConfluenceSnapshotInvalid, got %v", name, err)
		}
	}
}

func TestIsConfluenceCreateArgs(t *testing.T) {
	if !IsConfluenceCreateArgs(cfCreateArgs("p", "T", "s")) {
		t.Fatal("create-shaped args must be detected")
	}
	update, _ := json.Marshal(map[string]any{"page_id": "p", "expected_version": "3", "title": "T", "storage": "s"})
	if IsConfluenceCreateArgs(update) {
		t.Fatal("update-shaped args must not be treated as create")
	}
	if IsConfluenceCreateArgs([]byte(`not json`)) {
		t.Fatal("garbage must not be create")
	}
}

func TestConfluenceCreateCloudHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>hello</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("create: state=%s err=%v", out.State, err)
	}
	if out.ExternalID == "" || out.ExternalID == "parent-1" {
		t.Fatalf("external id must be the NEW page: %+v", out)
	}
	rcpt, rerr := ParseConfluencePageReceipt(out.Output)
	if rerr != nil || rcpt.ExternalID != out.ExternalID || rcpt.ExternalVersion != "1" {
		t.Fatalf("output must be a receipt-bearing page payload: %+v %v", rcpt, rerr)
	}
	posts, puts, gets := fake.stats()
	if posts != 1 || puts != 0 || gets != 1 {
		t.Fatalf("wire counts: posts=%d puts=%d gets=%d (the parent read must run)", posts, puts, gets)
	}
	fake.mu.Lock()
	created := fake.pages[out.ExternalID]
	fake.mu.Unlock()
	if created == nil || created.spaceID != "sp-1" || created.title != "Report" || created.storage != "<p>hello</p>" {
		t.Fatalf("created page drift: %+v", created)
	}
}

func TestConfluenceCreateCloudWikiContextPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "/wiki")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>x</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("create under /wiki context path: state=%s err=%v", out.State, err)
	}
	posts, _, _ := fake.stats()
	if posts != 1 {
		t.Fatalf("posts=%d", posts)
	}
}

func TestConfluenceCreateServerHappyPath(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "42", "ENG", "Parent", 3)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionServer, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>srv</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("server create: state=%s err=%v", out.State, err)
	}
	posts, puts, gets := fake.stats()
	if posts != 1 || puts != 0 || gets != 1 {
		t.Fatalf("wire counts: posts=%d puts=%d gets=%d", posts, puts, gets)
	}
	fake.mu.Lock()
	created := fake.pages[out.ExternalID]
	fake.mu.Unlock()
	if created == nil || created.spaceKey != "ENG" || created.storage != "<p>srv</p>" {
		t.Fatalf("created page drift: %+v", created)
	}
}

func TestConfluenceCreateParentOutOfScope(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-unlisted", "sp-9", "OPS", "Other", 1)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-unlisted", "T", "s")))
	if !errors.Is(err, ErrConfluenceParentOutOfScope) || out.State != ActionFailed {
		t.Fatalf("unlisted parent must fail closed: state=%s err=%v", out.State, err)
	}
	posts, _, _ := fake.stats()
	if posts != 0 {
		t.Fatalf("zero writes after scope refusal, got %d posts", posts)
	}
}

func TestConfluenceCreateCapabilityMissing(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")
	ad.ConnectionCapabilities = cfCaps()

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if !errors.Is(err, ErrConfluenceMissingCapability) || out.State != ActionFailed {
		t.Fatalf("missing capability must fail closed: state=%s err=%v", out.State, err)
	}
	posts, _, gets := fake.stats()
	if posts != 0 || gets != 0 {
		t.Fatalf("no request may leave without the capability: posts=%d gets=%d", posts, gets)
	}
}

func TestConfluenceCreateUnreadableParentIsFailedNotUnknown(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-missing", "T", "s")))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("missing parent (a GET that never wrote) must fail definitively: state=%s err=%v", out.State, err)
	}
	posts, _, _ := fake.stats()
	if posts != 0 {
		t.Fatalf("no write may follow an unreadable parent: %d", posts)
	}
}

func TestConfluenceCreateUnknownOnLostCreateReply(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if out.State != ActionUnknown || !errors.Is(err, ErrConfluenceOutcomeUnknown) {
		t.Fatalf("lost create reply must park unknown, got state=%s err=%v", out.State, err)
	}
	// The effect DID land (the fake applies then drops): a blind second
	// create would duplicate the page — which is exactly why this state is
	// unknown and resolves only via human reconciliation.
	posts, _, _ := fake.stats()
	if posts != 1 {
		t.Fatalf("the dropped post counted once: %d", posts)
	}
}

func TestConfluenceCreateQueryStaysUnknown(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	q, _ := ad.Query(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if q.State != ActionUnknown {
		t.Fatalf("a create without a confirmed page id can never be proven — must stay unknown, got %s", q.State)
	}
}

func TestConfluenceCreateRejectsBadEdition(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, "gopher", "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "T", "s")))
	if out.State != ActionFailed || err == nil {
		t.Fatalf("unknown edition must fail closed pre-send: state=%s err=%v", out.State, err)
	}
	posts, _, gets := fake.stats()
	if posts != 0 || gets != 0 {
		t.Fatalf("no request may leave: posts=%d gets=%d", posts, gets)
	}
}

// TestConfluenceCreateWireUsesOfficialCamelCaseKeys pins B5-F43: the Cloud
// v2 create wire must carry the official camelCase spaceId/parentId keys —
// a snake_case body is a first-call 400 on the real Confluence Cloud (the
// fake now decodes the official contract, so the isomorphic-mask cannot
// recur) and the captured raw body proves the wire shape.
func TestConfluenceCreateWireUsesOfficialCamelCaseKeys(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>hello</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("create against the official-contract double: state=%s err=%v", out.State, err)
	}
	fake.mu.Lock()
	raw := fake.lastCreateRaw
	fake.mu.Unlock()
	if !strings.Contains(raw, `"spaceId"`) || !strings.Contains(raw, `"parentId"`) {
		t.Fatalf("create wire must use official camelCase keys (spaceId/parentId), got: %s", raw)
	}
	if strings.Contains(raw, `"space_id"`) || strings.Contains(raw, `"parent_id"`) {
		t.Fatalf("create wire must not carry snake_case keys, got: %s", raw)
	}
}

// TestConfluenceDoRejectsBodyOverCap pins R5-F6: a response body larger than
// the read cap must surface as an ErrConfluenceOutcomeUnknown-wrapped ERROR
// naming the cap — never as silently truncated bytes. A truncated write
// reply proves nothing, and a truncated read breaks reconciliation forever.
func TestConfluenceDoRejectsBodyOverCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		chunk := make([]byte, 64*1024)
		remaining := maxConfluenceBodyBytes + 1
		for remaining > 0 {
			n := len(chunk)
			if remaining < n {
				n = remaining
			}
			if _, err := w.Write(chunk[:n]); err != nil {
				return
			}
			remaining -= n
		}
	}))
	t.Cleanup(srv.Close)
	pol := cfPolicyOf(srv, "")
	u := confluenceTargetURL(pol, "", ConfluenceCloudPagesPath+"/parent-1", "body-format=storage")

	status, raw, err := confluenceDo(context.Background(), pol, cfCredential, http.MethodGet, u, nil)
	if err == nil || !errors.Is(err, ErrConfluenceOutcomeUnknown) {
		t.Fatalf("an over-cap body must be an unknown-outcome error, got %v", err)
	}
	if status != http.StatusOK || raw != nil {
		t.Fatalf("no truncated bytes may be returned: status=%d raw=%v", status, raw)
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("the error must name the cap, got: %v", err)
	}
}

// TestConfluenceDoAcceptsMultiMegabyteStorageEcho pins R5-F6's ceiling side:
// the cap must cover legal payloads. A ~3MiB storage echo (within the
// publish.MaxPublishArtifactBytes 1MiB artifact → ~5x escaped storage
// envelope) must read and decode normally, not be refused as over-cap.
func TestConfluenceDoAcceptsMultiMegabyteStorageEcho(t *testing.T) {
	fake := newFakeConfluence("cf-user@example.test", "secret_cf_token")
	parent := fake.addPage("parent-1", "sp-1", "ENG", "Parent", 7)
	fake.mu.Lock()
	parent.storage = "<p>" + strings.Repeat("x", 3<<20) + "</p>"
	fake.mu.Unlock()
	srv := fake.server(t)
	ad := cfCreateAdapter(srv, EditionCloud, "")

	// Execute's parent read (GET ?body-format=storage) carries the ~3MiB
	// echo; a create must still succeed against it.
	out, err := ad.Execute(context.Background(), cfCreateAction(cfCreateArgs("parent-1", "Report", "<p>hello</p>")))
	if err != nil || out.State != ActionSucceeded {
		t.Fatalf("a legal multi-megabyte storage echo must not be refused: state=%s err=%v", out.State, err)
	}
}
