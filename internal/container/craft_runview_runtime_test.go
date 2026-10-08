package container

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
)

type fakeCraftRunViewStore struct {
	mu      sync.Mutex
	view    craft.RunView
	allowed craft.RunViewKey
}

func newFakeCraftRunViewStore(key craft.RunViewKey) *fakeCraftRunViewStore {
	return &fakeCraftRunViewStore{allowed: key, view: craft.RunView{
		Key: key, Generation: "rv_test_generation", State: craft.RunViewStateAllocating,
	}}
}

func (s *fakeCraftRunViewStore) scope(key craft.RunViewKey) error {
	if key.TenantID != s.allowed.TenantID || key.RunID != s.allowed.RunID || key.SessionID != s.allowed.SessionID {
		return craft.ErrNotFound
	}
	if key.OwnerID != s.allowed.OwnerID {
		return craft.ErrForbidden
	}
	return nil
}

func (s *fakeCraftRunViewStore) Allocate(_ context.Context, key craft.RunViewKey) (craft.RunView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.scope(key); err != nil {
		return craft.RunView{}, err
	}
	return s.view, nil
}

func (s *fakeCraftRunViewStore) Load(_ context.Context, key craft.RunViewKey) (craft.RunView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.scope(key); err != nil {
		return craft.RunView{}, err
	}
	return s.view, nil
}

func (s *fakeCraftRunViewStore) BeginSessionCreate(_ context.Context, key craft.RunViewKey, generation string) (craft.RunView, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.scope(key); err != nil {
		return craft.RunView{}, false, err
	}
	if generation != s.view.Generation {
		return craft.RunView{}, false, craft.ErrConflict
	}
	if s.view.State != craft.RunViewStateAllocating || s.view.SessionCreateIntentAt != nil {
		return s.view, false, nil
	}
	now := time.Now().UTC()
	s.view.SessionCreateIntentAt = &now
	return s.view, true, nil
}

func (s *fakeCraftRunViewStore) BindRuntime(_ context.Context, key craft.RunViewKey, generation string, runtime craft.RunViewRuntime) (craft.RunView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.scope(key); err != nil {
		return craft.RunView{}, err
	}
	if generation != s.view.Generation {
		return craft.RunView{}, craft.ErrConflict
	}
	if s.view.SessionCreateIntentAt == nil {
		return craft.RunView{}, craft.ErrConflict
	}
	if s.view.State == craft.RunViewStateBound {
		if s.view.Runtime != runtime {
			return craft.RunView{}, craft.ErrConflict
		}
		return s.view, nil
	}
	s.view.Runtime = runtime
	s.view.State = craft.RunViewStateBound
	return s.view, nil
}

type fakeCraftRunViewProvider struct {
	mu sync.Mutex

	containerOverride *CraftRunViewRuntimeContainer
	inventoryOverride *CraftRunViewRuntimeInventory
	sessions          []CraftRunViewRuntimeSession
	createError       error
	persistOnError    bool
	createCalls       int
	createDirectory   string
	containerSpecs    []CraftRunViewRuntimeContainerSpec
}

func (p *fakeCraftRunViewProvider) InspectOrCreateContainer(_ context.Context, spec CraftRunViewRuntimeContainerSpec) (CraftRunViewRuntimeContainer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.containerSpecs = append(p.containerSpecs, spec)
	if p.containerOverride != nil {
		return *p.containerOverride, nil
	}
	return CraftRunViewRuntimeContainer{
		Generation: spec.Generation, RuntimeID: spec.RuntimeID, ContainerID: spec.ContainerID,
		Directory: spec.Directory, ProjectID: "project_test",
		IdentityVerified: true, DedicatedForRun: true, DirectoryCanonical: true,
	}, nil
}

func (p *fakeCraftRunViewProvider) FindSessions(_ context.Context, container CraftRunViewRuntimeContainer) (CraftRunViewRuntimeInventory, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.inventoryOverride != nil {
		return *p.inventoryOverride, nil
	}
	containerProject := "project_test"
	if len(p.sessions) > 0 && p.sessions[0].ProjectID != "" {
		containerProject = p.sessions[0].ProjectID
	}
	return CraftRunViewRuntimeInventory{
		Authoritative: true, Complete: true,
		Directory: container.Directory,
		ProjectID: containerProject,
		Sessions:  append([]CraftRunViewRuntimeSession(nil), p.sessions...),
	}, nil
}

func (p *fakeCraftRunViewProvider) CreateSession(_ context.Context, container CraftRunViewRuntimeContainer, directory string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.createCalls++
	p.createDirectory = directory
	if p.persistOnError || p.createError == nil {
		p.sessions = append(p.sessions, CraftRunViewRuntimeSession{
			ID: "oc_test_session", ProjectID: container.ProjectID, Directory: directory,
		})
	}
	return p.createError
}

func TestCraftRunViewRuntimeCoordinatorCreatesAndBindsVerifiedSession(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{}
	coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatalf("NewCraftRunViewRuntimeCoordinator() error = %v", err)
	}

	handle, err := coordinator.Resolve(context.Background(), key)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if handle.View.State != craft.RunViewStateBound || handle.View.Runtime.OpenCodeSessionID != "oc_test_session" {
		t.Fatalf("Resolve() returned unbound or unexpected view: %+v", handle.View)
	}
	if want := provider.containerSpecs[0].Directory; handle.Directory != want {
		t.Fatalf("handle directory = %q, want server-derived %q", handle.Directory, want)
	}
	if provider.createCalls != 1 || provider.createDirectory != handle.Directory {
		t.Fatalf("CreateSession calls/directory = %d/%q, want one call to %q", provider.createCalls, provider.createDirectory, handle.Directory)
	}
}

func TestCraftRunViewRuntimeCoordinatorRecoversLostCreateResponseWithoutResending(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{createError: errors.New("response lost"), persistOnError: true}
	first, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
		t.Fatalf("first Resolve() error = %v, want unresolved after lost create response", err)
	}

	provider.createError = nil
	// A new coordinator represents process restart; durable store/provider state
	// persists, but coordinator memory is intentionally empty.
	restarted, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatal(err)
	}
	handle, err := restarted.Resolve(context.Background(), key)
	if err != nil {
		t.Fatalf("restart Resolve() error = %v", err)
	}
	if handle.View.Runtime.OpenCodeSessionID != "oc_test_session" || provider.createCalls != 1 {
		t.Fatalf("restart session/call count = %q/%d, want existing session and no resend", handle.View.Runtime.OpenCodeSessionID, provider.createCalls)
	}
}

func TestCraftRunViewRuntimeCoordinatorRevalidatesBoundSessionBeforeReturningHandle(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{}
	coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Resolve(context.Background(), key); err != nil {
		t.Fatalf("initial Resolve() error = %v", err)
	}

	provider.sessions[0].ID = "oc_replaced"
	if _, err := coordinator.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
		t.Fatalf("Resolve() with replaced session error = %v, want unresolved", err)
	}
	if provider.createCalls != 1 {
		t.Fatalf("bound session mismatch triggered a replacement create: calls=%d", provider.createCalls)
	}

	provider.sessions[0].ID = "oc_test_session"
	if _, err := coordinator.Resolve(context.Background(), key); err != nil {
		t.Fatalf("Resolve() with exact bound session error = %v", err)
	}
}

func TestCraftRunViewRuntimeCoordinatorNeverRetriesUnobservedCreate(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{createError: errors.New("response lost")}
	coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
		t.Fatalf("first Resolve() error = %v, want unresolved", err)
	}
	provider.createError = nil
	if _, err := coordinator.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
		t.Fatalf("retry Resolve() error = %v, want unresolved with empty inventory", err)
	}
	if provider.createCalls != 1 {
		t.Fatalf("CreateSession call count = %d, want one after unknown result", provider.createCalls)
	}
}

func TestCraftRunViewRuntimeCoordinatorFailsClosedOnUnprovenInventory(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{inventoryOverride: &CraftRunViewRuntimeInventory{Authoritative: false}}
	coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
		t.Fatalf("Resolve() error = %v, want unresolved without authoritative inventory", err)
	}
	if provider.createCalls != 0 || store.view.SessionCreateIntentAt != nil {
		t.Fatalf("unproven inventory caused create/intent: calls=%d intent=%v", provider.createCalls, store.view.SessionCreateIntentAt)
	}
}

func TestCraftRunViewRuntimeCoordinatorRejectsForeignSessionsAndDirectories(t *testing.T) {
	for name, mutate := range map[string]func(*CraftRunViewRuntimeSession){
		"wrong project":   func(session *CraftRunViewRuntimeSession) { session.ProjectID = "project_foreign" },
		"wrong directory": func(session *CraftRunViewRuntimeSession) { session.Directory = "/srv/craft/runviews/other" },
	} {
		t.Run(name, func(t *testing.T) {
			key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
			store := newFakeCraftRunViewStore(key)
			directory := testCraftRunViewContainerSpec(store.view.Generation).Directory
			session := CraftRunViewRuntimeSession{
				ID: "oc_foreign", ProjectID: "project_test", Directory: directory,
			}
			mutate(&session)
			provider := &fakeCraftRunViewProvider{sessions: []CraftRunViewRuntimeSession{session}}
			coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := coordinator.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
				t.Fatalf("Resolve() error = %v, want unresolved for foreign session evidence", err)
			}
			if provider.createCalls != 0 || store.view.State != craft.RunViewStateAllocating || store.view.SessionCreateIntentAt != nil {
				t.Fatalf("foreign evidence changed state: createCalls=%d view=%+v", provider.createCalls, store.view)
			}
		})
	}
}

func TestCraftRunViewRuntimeCoordinatorRejectsMultipleSessionsAndProviderIdentityMismatch(t *testing.T) {
	for name, setup := range map[string]func(*fakeCraftRunViewStore, *fakeCraftRunViewProvider){
		"multiple sessions": func(store *fakeCraftRunViewStore, provider *fakeCraftRunViewProvider) {
			provider.sessions = []CraftRunViewRuntimeSession{
				{ID: "oc-1", ProjectID: "project_test", Directory: testCraftRunViewContainerSpec(store.view.Generation).Directory},
				{ID: "oc-2", ProjectID: "project_test", Directory: testCraftRunViewContainerSpec(store.view.Generation).Directory},
			}
		},
		"wrong container generation": func(store *fakeCraftRunViewStore, provider *fakeCraftRunViewProvider) {
			container := CraftRunViewRuntimeContainer{Generation: "rv_foreign", RuntimeID: "runtime_test", ContainerID: "container_test", Directory: testCraftRunViewContainerSpec(store.view.Generation).Directory}
			provider.containerOverride = &container
		},
		"wrong root": func(store *fakeCraftRunViewStore, provider *fakeCraftRunViewProvider) {
			container := CraftRunViewRuntimeContainer{Generation: store.view.Generation, RuntimeID: "runtime_test", ContainerID: "container_test", Directory: "/srv/craft/runviews/foreign"}
			provider.containerOverride = &container
		},
		"unverified dedicated root": func(store *fakeCraftRunViewStore, provider *fakeCraftRunViewProvider) {
			container := CraftRunViewRuntimeContainer{
				Generation: store.view.Generation, RuntimeID: "runtime_test", ContainerID: "container_test",
				Directory: testCraftRunViewContainerSpec(store.view.Generation).Directory,
			}
			provider.containerOverride = &container
		},
	} {
		t.Run(name, func(t *testing.T) {
			key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
			store := newFakeCraftRunViewStore(key)
			provider := &fakeCraftRunViewProvider{}
			setup(store, provider)
			coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := coordinator.Resolve(context.Background(), key); !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
				t.Fatalf("Resolve() error = %v, want unresolved", err)
			}
			if provider.createCalls != 0 {
				t.Fatalf("CreateSession calls = %d, want none", provider.createCalls)
			}
		})
	}
}

func testCraftRunViewContainerSpec(generation string) CraftRunViewRuntimeContainerSpec {
	coordinator := &CraftRunViewRuntimeCoordinator{privateRootBase: "/srv/craft/runviews"}
	return coordinator.containerSpec(generation)
}

func TestCraftRunViewRuntimeCoordinatorForeignStoreScopeAndInvalidRootFailClosed(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{}
	coordinator, err := NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
	if err != nil {
		t.Fatal(err)
	}
	wrongOwner := key
	wrongOwner.OwnerID = "owner-foreign"
	if _, err := coordinator.Resolve(context.Background(), wrongOwner); !errors.Is(err, craft.ErrForbidden) {
		t.Fatalf("foreign owner error = %v, want forbidden", err)
	}
	if len(provider.containerSpecs) != 0 {
		t.Fatalf("foreign scope reached provider: %+v", provider.containerSpecs)
	}
	for _, root := range []string{"", "relative/runviews", "/srv/craft/../outside", "/"} {
		if _, err := NewCraftRunViewRuntimeCoordinator(store, provider, root); err == nil {
			t.Fatalf("coordinator accepted invalid root %q", root)
		}
	}
}

func TestCraftRunViewRuntimeCoordinatorConcurrentResolveUsesOneSessionCreate(t *testing.T) {
	key := craft.RunViewKey{TenantID: 9, OwnerID: "owner-9", SessionID: "task-9", RunID: "run-9"}
	store := newFakeCraftRunViewStore(key)
	provider := &fakeCraftRunViewProvider{}
	coordinators := make([]*CraftRunViewRuntimeCoordinator, 2)
	for i := range coordinators {
		var err error
		coordinators[i], err = NewCraftRunViewRuntimeCoordinator(store, provider, "/srv/craft/runviews")
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	handles := make([]CraftRunViewRuntimeHandle, len(coordinators))
	errs := make([]error, len(coordinators))
	var wg sync.WaitGroup
	for i := range coordinators {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			handles[i], errs[i] = coordinators[i].Resolve(context.Background(), key)
		}(i)
	}
	close(start)
	wg.Wait()

	if provider.createCalls != 1 {
		t.Fatalf("concurrent CreateSession calls = %d, want exactly one", provider.createCalls)
	}
	if len(provider.containerSpecs) != 2 || provider.containerSpecs[0] != provider.containerSpecs[1] {
		t.Fatalf("concurrent container specs differ: %+v", provider.containerSpecs)
	}
	if store.view.State != craft.RunViewStateBound || store.view.Runtime.OpenCodeSessionID != "oc_test_session" {
		t.Fatalf("final stored view = %+v, want one verified bound session", store.view)
	}
	for i, err := range errs {
		if err != nil && !errors.Is(err, ErrCraftRunViewRuntimeUnresolved) {
			t.Fatalf("Resolve[%d] error = %v, want success or fail-closed unresolved", i, err)
		}
		if err == nil && handles[i].View.State != craft.RunViewStateBound {
			t.Fatalf("Resolve[%d] returned unbound handle: %+v", i, handles[i])
		}
	}
}
