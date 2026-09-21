package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/opencode"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
)

// pinnedCraftRuntimeDigest mirrors the R01 lock digest for the 1.18.4 binary.
const pinnedCraftRuntimeDigest = "sha256:9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98"

// fakeWorkspaceStore is an in-memory craft.Store faithfully reproducing the
// R02 semantics that matter to Resolve: scope-filtered reads with the
// owner-mismatch read guard (disableable to prove Resolve does not depend on
// it), create-if-absent, and the CAS update path that accepts a changed
// owner_id — the exact hole R02 review nit-2 flagged.
type fakeWorkspaceStore struct {
	mu             sync.Mutex
	row            *craft.Workspace
	permissiveRead bool
	puts           int
}

func (s *fakeWorkspaceStore) GetWorkspace(_ context.Context, scope craft.Scope) (craft.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.row == nil {
		return craft.Workspace{}, craft.ErrNotFound
	}
	if !s.permissiveRead {
		if s.row.Scope.TenantID != scope.TenantID || s.row.Scope.SessionID != scope.SessionID {
			return craft.Workspace{}, craft.ErrNotFound
		}
		if s.row.Scope.UserID != scope.UserID {
			return craft.Workspace{}, fmt.Errorf("%w: workspace owned by %s", craft.ErrForbidden, s.row.Scope.UserID)
		}
	}
	return *s.row, nil
}

func (s *fakeWorkspaceStore) PutWorkspace(_ context.Context, in craft.Workspace, expected int64) (craft.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.puts++
	if expected == 0 {
		if s.row != nil {
			return craft.Workspace{}, fmt.Errorf("%w: session already has a workspace binding", craft.ErrConflict)
		}
		if in.ID == "" {
			in.ID = "ws-fake"
		}
		in.Revision = 1
		s.row = &in
		return in, nil
	}
	if s.row == nil || s.row.ID != in.ID || s.row.Revision != expected {
		return craft.Workspace{}, fmt.Errorf("%w: workspace revision changed", craft.ErrConflict)
	}
	// Faithful to the real CAS update: owner_id is overwritten from the input.
	in.Revision = s.row.Revision + 1
	s.row = &in
	return in, nil
}

func (s *fakeWorkspaceStore) snapshot() *craft.Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.row == nil {
		return nil
	}
	copy := *s.row
	return &copy
}

func (s *fakeWorkspaceStore) PrepareTask(context.Context, craft.Task) (craft.Task, error) {
	panic("unused by Resolve")
}
func (s *fakeWorkspaceStore) GetTask(context.Context, craft.Scope, string) (craft.Task, error) {
	panic("unused by Resolve")
}
func (s *fakeWorkspaceStore) SaveResult(context.Context, agentruntime.Fence, craft.Result) error {
	panic("unused by Resolve")
}
func (s *fakeWorkspaceStore) GetResult(context.Context, craft.Scope, string) (craft.Result, error) {
	panic("unused by Resolve")
}

// lockRecordingBindings records whether binding reads happen under the
// session's lifecycle lock.
type lockRecordingBindings struct {
	sandbox.SessionSandboxBindingStore
	mu             sync.Mutex
	locked         bool
	readsUnderLock int
}

func (b *lockRecordingBindings) WithLifecycleLock(ctx context.Context, key sandbox.SessionSandboxKey, fn func(context.Context) error) error {
	b.mu.Lock()
	b.locked = true
	b.mu.Unlock()
	err := b.SessionSandboxBindingStore.WithLifecycleLock(ctx, key, fn)
	b.mu.Lock()
	b.locked = false
	b.mu.Unlock()
	return err
}

func (b *lockRecordingBindings) Get(ctx context.Context, key sandbox.SessionSandboxKey) (*sandbox.SessionSandboxBinding, error) {
	b.mu.Lock()
	underLock := b.locked
	if underLock {
		b.readsUnderLock++
	}
	b.mu.Unlock()
	return b.SessionSandboxBindingStore.Get(ctx, key)
}

func (b *lockRecordingBindings) isLocked() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.locked
}

// openCodeStub is a pinned-protocol OpenCode server: POST /session either
// answers a sequential session id or loses the response after counting the
// create, letting tests replay a lost-create-response recovery.
type openCodeStub struct {
	server  *httptest.Server
	creates int32
	lose    bool
}

func newOpenCodeStub(lose bool) *openCodeStub {
	stub := &openCodeStub{lose: lose}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /session", func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&stub.creates, 1)
		if stub.lose {
			// Truncated JSON: the session exists server-side, the client
			// cannot decode the id — a lost create response.
			fmt.Fprint(w, `{"id":"oc-lost`)
			return
		}
		fmt.Fprintf(w, `{"id":"oc-%d"}`, n)
	})
	stub.server = httptest.NewServer(mux)
	return stub
}

type workspaceRig struct {
	scope    craft.Scope
	key      sandbox.SessionSandboxKey
	store    *fakeWorkspaceStore
	bindings *lockRecordingBindings
	memory   *sandbox.MemorySessionSandboxBindingStore
	stub     *openCodeStub
	svc      *CraftWorkspaceService
	active   int32
	activeN  int32
	dials    int32
}

func newWorkspaceRig(t *testing.T) *workspaceRig {
	t.Helper()
	stub := newOpenCodeStub(false)
	t.Cleanup(stub.server.Close)
	return rigWithStub(t, stub)
}

func rigWithStub(t *testing.T, stub *openCodeStub) *workspaceRig {
	t.Helper()
	r := &workspaceRig{
		scope:  craft.Scope{TenantID: 1, UserID: "u-ws", SessionID: "s-ws"},
		stub:   stub,
		memory: sandbox.NewMemorySessionSandboxBindingStore(),
		store:  &fakeWorkspaceStore{},
	}
	r.key = sandbox.SessionSandboxKey{TenantID: r.scope.TenantID, SessionID: r.scope.SessionID}
	r.seedBinding("sbx-1", "g1")
	r.bindings = &lockRecordingBindings{SessionSandboxBindingStore: r.memory}

	client, err := opencode.NewClient(stub.server.URL, stub.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewCraftWorkspaceService(CraftWorkspaceConfig{
		Store:    r.store,
		Bindings: r.bindings,
		ActiveRuns: func(context.Context, craft.Scope) (bool, error) {
			atomic.AddInt32(&r.activeN, 1)
			if r.bindings.isLocked() {
				atomic.AddInt32(&r.active, 1)
			}
			return false, nil
		},
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			atomic.AddInt32(&r.dials, 1)
			return client, nil
		},
		RuntimeDigest: pinnedCraftRuntimeDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	r.svc = svc
	return r
}

func (r *workspaceRig) seedBinding(sandboxID, generation string) {
	binding := sandbox.SessionSandboxBinding{
		Version:    sandbox.SessionSandboxBindingVersion,
		Provider:   sandbox.SandboxTypeDocker,
		TenantID:   r.scope.TenantID,
		SessionID:  r.scope.SessionID,
		SandboxID:  sandboxID,
		TemplateID: "craft-opencode-1.18.4",
		Generation: generation,
		CreatedAt:  time.Now().UTC(),
	}
	if old, err := r.memory.Get(context.Background(), r.key); err == nil && old != nil {
		if _, err := r.memory.DeleteIfMatch(context.Background(), r.key, old.Provider, old.SandboxID); err != nil {
			panic(err)
		}
	}
	if _, err := r.memory.Create(context.Background(), r.key, binding); err != nil {
		panic(err)
	}
}

func TestResolveCreatesAndPersistsWorkspace(t *testing.T) {
	r := newWorkspaceRig(t)
	ctx := context.Background()
	ws, err := r.svc.Resolve(ctx, r.scope)
	if err != nil {
		t.Fatal(err)
	}
	if ws.SandboxID != "sbx-1" || ws.Generation != "g1" || ws.OpenCodeSessionID != "oc-1" {
		t.Fatalf("workspace = %#v", ws)
	}
	if ws.RuntimeDigest != pinnedCraftRuntimeDigest || ws.Revision != 1 || ws.Scope != r.scope {
		t.Fatalf("workspace = %#v", ws)
	}
	if got := atomic.LoadInt32(&r.stub.creates); got != 1 {
		t.Fatalf("opencode creates = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&r.dials); got != 1 {
		t.Fatalf("dials = %d, want 1", got)
	}
	if stored := r.store.snapshot(); stored == nil || stored.OpenCodeSessionID != "oc-1" {
		t.Fatalf("store row = %#v", stored)
	}
	// The binding read and the database active-run check both ran under the
	// session's existing lifecycle lock.
	r.bindings.mu.Lock()
	reads := r.bindings.readsUnderLock
	r.bindings.mu.Unlock()
	if reads == 0 {
		t.Fatal("sandbox binding was read outside the lifecycle lock")
	}
	if atomic.LoadInt32(&r.active) == 0 {
		t.Fatal("active-run database check ran outside the lifecycle lock")
	}
}

func TestResolveReusesMappingWithoutNewSession(t *testing.T) {
	r := newWorkspaceRig(t)
	ctx := context.Background()
	first, err := r.svc.Resolve(ctx, r.scope)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.svc.Resolve(ctx, r.scope)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("second round = %#v, want the same reused mapping %#v", second, first)
	}
	if got := atomic.LoadInt32(&r.stub.creates); got != 1 {
		t.Fatalf("opencode creates = %d, want 1: reuse must not CreateSession every round", got)
	}
	if got := atomic.LoadInt32(&r.dials); got != 1 {
		t.Fatalf("dials = %d, want 1 on the reuse path", got)
	}
}

func TestResolveRebindsWhenGenerationMoves(t *testing.T) {
	r := newWorkspaceRig(t)
	ctx := context.Background()
	first, err := r.svc.Resolve(ctx, r.scope)
	if err != nil {
		t.Fatal(err)
	}
	r.seedBinding("sbx-2", "g2")
	second, err := r.svc.Resolve(ctx, r.scope)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("rebind must keep workspace id %s, got %s", first.ID, second.ID)
	}
	if second.SandboxID != "sbx-2" || second.Generation != "g2" || second.OpenCodeSessionID != "oc-2" {
		t.Fatalf("rebound workspace = %#v", second)
	}
	if second.Revision != 2 {
		t.Fatalf("rebind revision = %d, want 2 (CAS update)", second.Revision)
	}
	if got := atomic.LoadInt32(&r.stub.creates); got != 2 {
		t.Fatalf("opencode creates = %d, want 2", got)
	}
}

func TestResolveRefusesRebindWhileRunActive(t *testing.T) {
	r := newWorkspaceRig(t)
	ctx := context.Background()
	if _, err := r.svc.Resolve(ctx, r.scope); err != nil {
		t.Fatal(err)
	}
	before := r.store.snapshot()
	r.seedBinding("sbx-2", "g2")
	// Turn the active-run database check positive.
	svc, err := NewCraftWorkspaceService(CraftWorkspaceConfig{
		Store:      r.store,
		Bindings:   r.bindings,
		ActiveRuns: func(context.Context, craft.Scope) (bool, error) { return true, nil },
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			atomic.AddInt32(&r.dials, 1)
			client, err := opencode.NewClient(r.stub.server.URL, r.stub.server.Client())
			if err != nil {
				t.Fatal(err)
			}
			return client, nil
		},
		RuntimeDigest: pinnedCraftRuntimeDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Resolve(ctx, r.scope)
	if !errors.Is(err, craft.ErrBusy) {
		t.Fatalf("rebind under an active run error = %v, want craft.ErrBusy", err)
	}
	if got := atomic.LoadInt32(&r.stub.creates); got != 1 {
		t.Fatalf("opencode creates = %d, want 1 (no new session on refusal)", got)
	}
	if after := r.store.snapshot(); after.Revision != before.Revision || after.OpenCodeSessionID != before.OpenCodeSessionID {
		t.Fatalf("workspace changed under refusal: %#v -> %#v", before, after)
	}
}

func TestResolveLostCreateResponseIsUnknownAndNeverAdopts(t *testing.T) {
	stub := newOpenCodeStub(true)
	r := rigWithStub(t, stub)
	defer stub.server.Close()
	ctx := context.Background()
	_, err := r.svc.Resolve(ctx, r.scope)
	if !errors.Is(err, craft.ErrUnknown) {
		t.Fatalf("lost create response error = %v, want craft.ErrUnknown", err)
	}
	if got := atomic.LoadInt32(&stub.creates); got != 1 {
		t.Fatalf("opencode creates = %d, want exactly one unconfirmed attempt", got)
	}
	if r.store.snapshot() != nil {
		t.Fatal("an unconfirmed create must not persist a workspace mapping")
	}
}

func TestResolveAdoptsRaceWinnerOnLostCreateResponse(t *testing.T) {
	stub := newOpenCodeStub(true)
	r := rigWithStub(t, stub)
	defer stub.server.Close()
	ctx := context.Background()
	client, err := opencode.NewClient(stub.server.URL, stub.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewCraftWorkspaceService(CraftWorkspaceConfig{
		Store:      r.store,
		Bindings:   r.bindings,
		ActiveRuns: func(context.Context, craft.Scope) (bool, error) { return false, nil },
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			// A cross-process winner persisted the controlled mapping while
			// our own create response was being lost.
			winner := craft.Workspace{
				Scope: r.scope, SandboxID: "sbx-1", Generation: "g1",
				OpenCodeSessionID: "oc-winner", RuntimeDigest: pinnedCraftRuntimeDigest,
			}
			if _, err := r.store.PutWorkspace(ctx, winner, 0); err != nil {
				t.Fatal(err)
			}
			return client, nil
		},
		RuntimeDigest: pinnedCraftRuntimeDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	ws, err := svc.Resolve(ctx, r.scope)
	if err != nil {
		t.Fatal(err)
	}
	if ws.OpenCodeSessionID != "oc-winner" {
		t.Fatalf("reconciled workspace = %#v, want the recorded winner mapping", ws)
	}
}

// TestResolveGuardsOwnerItself proves the nit-2 fix: with a store whose read
// side has no owner guard at all, Resolve still refuses to touch a workspace
// owned by another user before any dial or PutWorkspace.
func TestResolveGuardsOwnerItself(t *testing.T) {
	r := newWorkspaceRig(t)
	r.store.permissiveRead = true
	foreign := r.scope
	foreign.UserID = "u-other"
	row := craft.Workspace{
		ID: "ws-foreign", Scope: foreign, SandboxID: "sbx-1", Generation: "g1",
		OpenCodeSessionID: "oc-foreign", RuntimeDigest: pinnedCraftRuntimeDigest, Revision: 4,
	}
	if _, err := r.store.PutWorkspace(context.Background(), row, 0); err != nil {
		t.Fatal(err)
	}
	r.store.mu.Lock()
	r.store.puts = 0
	r.store.mu.Unlock()
	_, err := r.svc.Resolve(context.Background(), r.scope)
	if !errors.Is(err, craft.ErrForbidden) {
		t.Fatalf("foreign-owned workspace error = %v, want craft.ErrForbidden", err)
	}
	if got := atomic.LoadInt32(&r.dials); got != 0 {
		t.Fatalf("dials = %d, want 0: refuse before creating any session", got)
	}
	r.store.mu.Lock()
	puts := r.store.puts
	r.store.mu.Unlock()
	if puts != 0 {
		t.Fatalf("PutWorkspace calls = %d, want 0: the CAS owner-overwrite hole must stay unreachable", puts)
	}
}

func TestResolveRequiresBoundSandbox(t *testing.T) {
	r := newWorkspaceRig(t)
	empty := sandbox.NewMemorySessionSandboxBindingStore()
	svc, err := NewCraftWorkspaceService(CraftWorkspaceConfig{
		Store:      r.store,
		Bindings:   empty,
		ActiveRuns: func(context.Context, craft.Scope) (bool, error) { return false, nil },
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			return nil, errors.New("unreachable")
		},
		RuntimeDigest: pinnedCraftRuntimeDigest,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Resolve(context.Background(), r.scope)
	if !errors.Is(err, craft.ErrUnsupported) {
		t.Fatalf("missing sandbox binding error = %v, want craft.ErrUnsupported", err)
	}
}

func TestResolveValidatesScope(t *testing.T) {
	r := newWorkspaceRig(t)
	for _, scope := range []craft.Scope{
		{UserID: "u", SessionID: "s"},
		{TenantID: 1, SessionID: "s"},
		{TenantID: 1, UserID: "u"},
	} {
		if _, err := r.svc.Resolve(context.Background(), scope); !errors.Is(err, craft.ErrInvalidInput) {
			t.Fatalf("scope %#v error = %v, want craft.ErrInvalidInput", scope, err)
		}
	}
}

func TestNewCraftWorkspaceServiceRequiresFullAssembly(t *testing.T) {
	working := CraftWorkspaceConfig{
		Store:      &fakeWorkspaceStore{},
		Bindings:   sandbox.NewMemorySessionSandboxBindingStore(),
		ActiveRuns: func(context.Context, craft.Scope) (bool, error) { return false, nil },
		Dial: func(context.Context, sandbox.SessionSandboxBinding) (*opencode.Client, error) {
			return nil, errors.New("unused")
		},
		RuntimeDigest: pinnedCraftRuntimeDigest,
	}
	if _, err := NewCraftWorkspaceService(working); err != nil {
		t.Fatal(err)
	}
	noDigest := working
	noDigest.RuntimeDigest = ""
	if _, err := NewCraftWorkspaceService(noDigest); err == nil {
		t.Fatal("an unpinned runtime digest must be rejected")
	}
	noStore := working
	noStore.Store = nil
	if _, err := NewCraftWorkspaceService(noStore); err == nil {
		t.Fatal("a missing store must be rejected")
	}
}
