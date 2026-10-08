package publish

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/appconnector/service/appconnector"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// cfWireFake is the Cloud-v2-only wire double for bridge/service tests
// (the both-editions double lives in the appconnector package tests; this
// one stays minimal because routing/edition logic is already covered
// there). It is NOT the real-provider acceptance (Task 9).
type cfWireFake struct {
	mu       sync.Mutex
	username string
	secret   string
	pages    map[string]*cfWirePage
	nextID   int
	posts    int
	puts     int
	gets     int
	// knob: apply the write effect, then lose the reply.
	dropNextWrite bool
}

type cfWirePage struct {
	id, spaceID, title, storage string
	version                     int
}

func newCfWireFake(username, secret string) *cfWireFake {
	return &cfWireFake{username: username, secret: secret, pages: map[string]*cfWirePage{}}
}

func (f *cfWireFake) addPage(id, spaceID, title string, version int) *cfWirePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := &cfWirePage{id: id, spaceID: spaceID, title: title, version: version}
	f.pages[id] = p
	return p
}

func (f *cfWireFake) bump(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.pages[id]; ok {
		p.version++
	}
}

func (f *cfWireFake) page(id string) *cfWirePage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pages[id]
}

func cfWireJSON(p *cfWirePage, withBody bool) string {
	storage := ""
	if withBody {
		raw, _ := json.Marshal(p.storage)
		storage = fmt.Sprintf(`,"body":{"storage":{"value":%s}}`, raw)
	}
	return fmt.Sprintf(`{"id":%q,"status":"current","title":%q,"spaceId":%q,"version":{"number":%d,"createdAt":"2026-09-25T08:00:00Z"}%s}`,
		p.id, p.title, p.spaceID, p.version, storage)
}

func (f *cfWireFake) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}
	auth := func(r *http.Request) bool {
		want := "Basic " + base64.StdEncoding.EncodeToString([]byte(f.username+":"+f.secret))
		return r.Header.Get("Authorization") == want
	}
	mux.HandleFunc("/api/v2/pages", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, 401, `{}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		// The double decodes the OFFICIAL Cloud v2 contract — camelCase
		// spaceId/parentId keys only (R5-F5: an isomorphic snake_case decode
		// here would mask a wire-contract regression exactly as it did on
		// the real provider).
		var req struct {
			SpaceID  string `json:"spaceId"`
			ParentID string `json:"parentId"`
			Title    string `json:"title"`
			Body     struct {
				Value string `json:"value"`
			} `json:"body"`
		}
		_ = json.Unmarshal(body, &req)
		f.mu.Lock()
		_, ok := f.pages[req.ParentID] // plan erratum: existence check only; the parent value itself is never read
		f.mu.Unlock()
		if !ok || req.SpaceID == "" || req.Title == "" || req.Body.Value == "" {
			write(w, 400, `{"errors":[{"title":"invalid"}]}`)
			return
		}
		f.mu.Lock()
		f.nextID++
		f.posts++
		np := &cfWirePage{id: fmt.Sprintf("cf-%d", f.nextID), spaceID: req.SpaceID, title: req.Title, storage: req.Body.Value, version: 1}
		f.pages[np.id] = np
		drop := f.dropNextWrite
		if drop {
			f.dropNextWrite = false
		}
		f.mu.Unlock()
		if drop {
			panic(http.ErrAbortHandler)
		}
		write(w, 200, cfWireJSON(np, false))
	})
	mux.HandleFunc("/api/v2/pages/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(r) {
			write(w, 401, `{}`)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v2/pages/")
		f.mu.Lock()
		p, ok := f.pages[id]
		f.mu.Unlock()
		if !ok {
			write(w, 404, `{"errors":[{"title":"not found"}]}`)
			return
		}
		switch r.Method {
		case http.MethodGet:
			f.mu.Lock()
			f.gets++
			f.mu.Unlock()
			write(w, 200, cfWireJSON(p, true))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Title string `json:"title"`
				Body  struct {
					Value string `json:"value"`
				} `json:"body"`
				Version struct {
					Number int `json:"number"`
				} `json:"version"`
			}
			_ = json.Unmarshal(body, &req)
			if req.Version.Number != p.version+1 {
				write(w, 409, `{"errors":[{"title":"version conflict"}]}`)
				return
			}
			f.mu.Lock()
			f.puts++
			p.title, p.storage, p.version = req.Title, req.Body.Value, req.Version.Number
			drop := f.dropNextWrite
			if drop {
				f.dropNextWrite = false
			}
			f.mu.Unlock()
			if drop {
				panic(http.ErrAbortHandler)
			}
			write(w, 200, cfWireJSON(p, false))
		default:
			write(w, 405, `{}`)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func cfWirePolicy(t *testing.T, srv *httptest.Server) appconn.HTTPPolicy {
	t.Helper()
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	return appconn.HTTPPolicy{
		Scheme: "http", Host: host, Port: port,
		Methods: []string{"GET", "POST", "PUT"}, PathPrefix: "/",
		AuthorizedNetworks: []*net.IPNet{network},
	}
}

// ---- port fakes ----

type cfFakeScopes struct{ scope ConfluenceConnectionScope }

func (f *cfFakeScopes) ConfluenceScope(ctx context.Context, connectionID string) (ConfluenceConnectionScope, error) {
	return f.scope, nil
}

type cfFakeTokens struct{ cred appconn.ConfluenceCredential }

func (f *cfFakeTokens) Token(ctx context.Context, connectionID string, expectedVersion int64) (appconn.ConfluenceCredential, error) {
	return f.cred, nil
}

func cfScope(policy appconn.HTTPPolicy) ConfluenceConnectionScope {
	return ConfluenceConnectionScope{
		AppID: "confluence", ConnectionKind: "personal", OwnerID: "u1", AuthVersion: 1,
		ApprovedParents: []string{"parent-1"}, WriteCapability: true,
		Scheme: policy.Scheme, Host: policy.Host, Port: policy.Port,
		Edition: "cloud",
	}
}

func cfCred() appconn.ConfluenceCredential {
	return appconn.ConfluenceCredential{Username: "cf-user@example.test", Secret: "secret_cf_token"}
}

func cfSnapshot(id string, args json.RawMessage) appconnectorsvc.ActionSnapshot {
	return appconnectorsvc.ActionSnapshot{ID: id, TenantID: 7, ActorID: "u1", ConnectionID: "conn-cf",
		Version: "confluence/v1", Target: "parent-1", Risk: appconn.RiskWrite, Args: args, AuthVersion: 1}
}

func TestConfluenceBridgeDispatchesCreateAndUpdate(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	createArgs, _ := json.Marshal(map[string]any{"parent": "parent-1", "title": "Report", "storage": "<p>x</p>"})
	out, err := bridge.Dispatch(context.Background(), cfSnapshot("act-1", createArgs), "conn-cf")
	if err != nil || out.Status != appconn.ActionSucceeded {
		t.Fatalf("create dispatch: %+v %v", out, err)
	}
	pageID := out.ExecutionID
	if pageID == "" {
		t.Fatalf("execution id must carry the new page: %+v", out)
	}
	updateArgs, _ := json.Marshal(map[string]any{"page_id": pageID, "expected_version": "1", "title": "Report v2", "storage": "<p>y</p>"})
	out, err = bridge.Dispatch(context.Background(), cfSnapshot("act-2", updateArgs), "conn-cf")
	if err != nil || out.Status != appconn.ActionSucceeded {
		t.Fatalf("update dispatch: %+v %v", out, err)
	}
	fake.mu.Lock()
	p := fake.pages[pageID]
	fake.mu.Unlock()
	if p.version != 2 || p.storage != "<p>y</p>" {
		t.Fatalf("remote page drift: %+v", p)
	}
}

func TestConfluenceBridgeConflictPrefix(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	fake.addPage("page-9", "sp-1", "Old", 3)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	fake.bump("page-9") // external edit: version 3 → 4
	args, _ := json.Marshal(map[string]any{"page_id": "page-9", "expected_version": "3", "title": "T", "storage": "<p>s</p>"})
	out, err := bridge.Dispatch(context.Background(), cfSnapshot("act-3", args), "conn-cf")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != appconn.ActionFailed || !strings.HasPrefix(out.ProviderResult, ConfluenceVersionConflictResult) {
		t.Fatalf("conflict must fail with the confluence prefix: %+v", out)
	}
	_, puts, _ := func() (int, int, int) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.posts, fake.puts, fake.gets
	}()
	if puts != 0 {
		t.Fatalf("zero writes on conflict, got %d", puts)
	}
}

func TestConfluenceBridgeRefusesNonConfluenceConnection(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	srv := fake.server(t)
	scope := cfScope(cfWirePolicy(t, srv))
	scope.AppID = "notion"
	bridge := NewConfluenceBridge(&cfFakeScopes{scope: scope}, &fakePolicies{pol: cfWirePolicy(t, srv)}, &cfFakeTokens{cred: cfCred()})
	args, _ := json.Marshal(map[string]any{"parent": "parent-1", "title": "T", "storage": "s"})
	_, err := bridge.Dispatch(context.Background(), cfSnapshot("act-4", args), "conn-cf")
	if !errors.Is(err, appconnectorsvc.ErrDispatchNotStarted) {
		t.Fatalf("a non-confluence connection must be a provable pre-send refusal, got %v", err)
	}
	posts, puts, gets := func() (int, int, int) {
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return fake.posts, fake.puts, fake.gets
	}()
	if posts != 0 || puts != 0 || gets != 0 {
		t.Fatalf("no request may leave: %d/%d/%d", posts, puts, gets)
	}
}

func TestConfluenceBridgeQueryProviderReadsRemoteFirst(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 7)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	// A lost update reply after the effect applied.
	updateArgs, _ := json.Marshal(map[string]any{"page_id": "page-9", "expected_version": "3", "title": "T2", "storage": "<p>q</p>"})
	fake.addPage("page-9", "sp-1", "Old", 3)
	fake.mu.Lock()
	fake.dropNextWrite = true
	fake.mu.Unlock()
	out, err := bridge.Dispatch(context.Background(), cfSnapshot("act-5", updateArgs), "conn-cf")
	if err != nil || out.Status != appconn.ActionUnknown {
		t.Fatalf("lost reply must park unknown: %+v %v", out, err)
	}
	fake.mu.Lock()
	putsBefore := fake.puts
	fake.mu.Unlock()
	resolved, err := bridge.QueryProvider(context.Background(), cfSnapshot("act-5", updateArgs), "conn-cf")
	if err != nil || resolved.Status != appconn.ActionSucceeded {
		t.Fatalf("provider query must resolve from the remote: %+v %v", resolved, err)
	}
	fake.mu.Lock()
	putsAfter := fake.puts
	fake.mu.Unlock()
	if putsAfter != putsBefore {
		t.Fatalf("query must not re-send: %d -> %d", putsBefore, putsAfter)
	}
}

func TestConfluenceBridgeReadPageVersion(t *testing.T) {
	fake := newCfWireFake("cf-user@example.test", "secret_cf_token")
	fake.addPage("parent-1", "sp-1", "Parent", 9)
	srv := fake.server(t)
	bridge := NewConfluenceBridge(
		&cfFakeScopes{scope: cfScope(cfWirePolicy(t, srv))},
		&fakePolicies{pol: cfWirePolicy(t, srv)},
		&cfFakeTokens{cred: cfCred()},
	)
	v, err := bridge.ReadConfluencePageVersion(context.Background(), "conn-cf", "parent-1")
	if err != nil || v != "9" {
		t.Fatalf("plan-time pre-read drift: %q %v", v, err)
	}
}

func TestDBConfluenceScopeSource(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repoappconn.ConnectionRow{}, &repoappconn.InstallationRow{}, &repoappconn.AppVersion{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.InstallationRow{ID: "inst-cf", TenantID: 7, AppID: "confluence", AppVersion: "v1", State: "active",
		ConfigJSON: `{"base_url":"https://acme.atlassian.net"}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO app_versions (app_id, version, schema_json, risk_json) VALUES ('confluence', 'v1', '{"scopes":["write_content"],"approved_parents":["parent-1"]}', '{}')`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.ConnectionRow{TenantID: 7, ID: "conn-cf", InstallationID: "inst-cf", Kind: "personal",
		OwnerID: "u1", State: "active", AuthVersion: 3}).Error; err != nil {
		t.Fatal(err)
	}
	src := NewDBConfluenceScopeSource(db)
	scope, err := src.ConfluenceScope(context.Background(), "conn-cf")
	if err != nil {
		t.Fatal(err)
	}
	if scope.AppID != "confluence" || !scope.WriteCapability || scope.AuthVersion != 3 ||
		len(scope.ApprovedParents) != 1 || scope.ApprovedParents[0] != "parent-1" ||
		scope.Edition != "cloud" || scope.APIBasePath != "/wiki" || scope.Scheme != "https" || scope.Host != "acme.atlassian.net" {
		t.Fatalf("production scope drift: %+v", scope)
	}
	// Missing base_url fails closed. Plan erratum: the fixture must use a
	// different tenant — installations carries unique (tenant_id, app_id),
	// so a second tenant-7 confluence installation cannot exist.
	if err := db.Create(&repoappconn.InstallationRow{ID: "inst-bad", TenantID: 8, AppID: "confluence", AppVersion: "v1", State: "active", ConfigJSON: `{}`}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&repoappconn.ConnectionRow{TenantID: 8, ID: "conn-bad", InstallationID: "inst-bad", Kind: "personal", OwnerID: "u1", State: "active", AuthVersion: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := src.ConfluenceScope(context.Background(), "conn-bad"); err == nil {
		t.Fatal("a connection without a reviewed base_url must fail scope resolution")
	}
	// Unknown connection is a lookup miss.
	if _, err := src.ConfluenceScope(context.Background(), "conn-nope"); err == nil {
		t.Fatal("unknown connection must error")
	}
}

func TestConfluencePolicyProviderFromScope(t *testing.T) {
	scope := ConfluenceConnectionScope{Scheme: "https", Host: "acme.atlassian.net", APIBasePath: "/wiki"}
	pol, err := NewConfluencePolicyProvider(&cfFakeScopes{scope: scope}).PolicyFor(context.Background(), "conn-cf")
	if err != nil {
		t.Fatal(err)
	}
	if pol.Scheme != "https" || pol.Host != "acme.atlassian.net" || pol.PathPrefix != "/wiki/" {
		t.Fatalf("policy drift: %+v", pol)
	}
	if len(pol.Methods) != 3 || pol.Methods[0] != "GET" || pol.Methods[1] != "POST" || pol.Methods[2] != "PUT" {
		t.Fatalf("policy methods drift: %+v", pol.Methods)
	}
	pol, err = NewConfluencePolicyProvider(&cfFakeScopes{scope: ConfluenceConnectionScope{Scheme: "https", Host: "h"}}).PolicyFor(context.Background(), "c")
	if err != nil || pol.PathPrefix != "/" {
		t.Fatalf("root prefix drift: %+v %v", pol, err)
	}
}
