package container

// O03/O04 integration wiring (coordinator-assigned): the craft lifecycle
// service, its guards, the session-deletion tombstone entry, the periodic
// reclamation sweep and the usage view assembly. Everything here is glue the
// service tasks deliberately left to the integrator — the services and their
// tests already existed (O03, O01); this file only assembles them.

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// craftLocalSandboxDeleter is the honest sandbox deleter of the local
// single-serve craft runtime: the craft "sandbox" is the shared OpenCode
// serve, so per-session reclamation has no remote instance to delete and an
// empty sandbox id clears the way for the sweep's binding/workspace cleanup.
// A binding that names a NON-empty remote sandbox has no deleter in this
// deployment: the delete fails visible (the sweep keeps its retry record)
// instead of pretending the resource is gone.
type craftLocalSandboxDeleter struct{}

func (craftLocalSandboxDeleter) Delete(_ context.Context, _ uint64, sandboxID string) error {
	if strings.TrimSpace(sandboxID) == "" {
		return nil
	}
	return fmt.Errorf("craft: no remote sandbox deleter is wired in the local runtime (sandbox %q)", sandboxID)
}

// newCraftLifecycleService assembles the O03 lifecycle service over the real
// ports: the craft store, the SHARED session sandbox binding store singleton
// (the same instance the tenant resolver writes and the preview no-egress
// checker reads — a separately built memory store in Lite mode would make
// this sweeper blind to the resolver's bindings and its lifecycle locks
// non-mutual), the real active-run query, the soft-delete session existence
// check, the local sandbox deleter, the run canceler and the file-backed
// object deleter. Quota and TTL sources stay unwired until their
// specialties own them.
func newCraftLifecycleService(
	db *gorm.DB,
	store craft.Store,
	bindings sandbox.SessionSandboxBindingStore,
	files interfaces.FileService,
	activeRuns service.CraftRunActivity,
) (*service.CraftLifecycle, error) {
	if bindings == nil {
		// NewCraftLifecycle's own validation below REFUSES a nil binding
		// store and aborts assembly — say so, instead of implying the
		// sweeper degrades to inert-but-running.
		logger.Warnf(context.Background(),
			"[CraftLifecycle] shared binding store unavailable; lifecycle assembly will be refused (startup fails closed)")
	}
	var runCanceler service.CraftSessionRunCanceler
	if runs := service.RegisteredAgentRunService(); runs != nil {
		runCanceler = service.NewCraftSessionRunCanceler(db, runs)
	}
	lifecycle, lerr := service.NewCraftLifecycle(service.CraftLifecycleConfig{
		DB: db, Store: store, Bindings: bindings,
		ActiveRuns: activeRuns, SessionExists: service.NewCraftSessionExistence(db),
		SandboxDeleter: craftLocalSandboxDeleter{},
		RunCanceler:    runCanceler,
		ObjectDeleter:  service.CraftObjectDeleterFromFileService(files),
	})
	if lerr != nil {
		return nil, lerr
	}
	logger.Infof(context.Background(), "[CraftLifecycle] service assembled (shared binding store)")
	return lifecycle, nil
}

// newCraftUsageService assembles the O01 usage ledger service over the
// durable store (the recording side is the O02 model gateway; this provider
// is the read side O04 composes from).
func newCraftUsageService(db *gorm.DB) *service.CraftUsageService {
	return service.NewCraftUsageService(repository.NewCraftUsageStore(db))
}

// newCraftUsageViewService assembles the O04 usage view: the session read
// ACL, the usage ledger, the lifecycle residency and the version checks.
func newCraftUsageViewService(
	db *gorm.DB,
	sessions interfaces.SessionService,
	usage *service.CraftUsageService,
	lifecycle *service.CraftLifecycle,
	versions craft.VersionStore,
) (*service.CraftUsageViewService, error) {
	return service.NewCraftUsageViewService(service.CraftUsageViewConfig{
		DB: db, Sessions: sessions, Usage: usage,
		Residency: lifecycle, Versions: versions,
	})
}

// registerCraftUsageHTTPHandlers installs the O04 usage view handler for
// route mounting (the route itself mounts with the craft session table).
func registerCraftUsageHTTPHandlers(view *service.CraftUsageViewService) {
	if view == nil {
		return
	}
	session.RegisterCraftUsageHandler(view)
}

// wireCraftLifecycleIntegration lands the O03 hard wiring checklist:
//
//  1. GuardDispatch guards every NEW delegation dispatch;
//  2. GuardRestore guards every snapshot restore;
//  3. the session-deletion entrance tombstones the session before its row
//     goes away (see Handler.DeleteSession);
//  4. the periodic reclamation sweep starts with the container (default ON;
//     CRAFT_LIFECYCLE_SWEEP_DISABLED=true turns it off).
func wireCraftLifecycleIntegration(
	delegation *service.CraftDelegation,
	snapshots *service.CraftSnapshotService,
	lifecycle *service.CraftLifecycle,
	cleaner interfaces.ResourceCleaner,
) {
	if lifecycle == nil {
		return
	}
	if delegation != nil && delegation.Delegate != nil {
		delegation.Delegate.SetDispatchGuard(lifecycle)
	}
	if snapshots != nil {
		snapshots.SetRestoreGuard(lifecycle)
	}
	StartCraftLifecycleSweep(lifecycle, cleaner)
}

// wireCraftSessionTombstone installs the lifecycle tombstone at the session
// deletion entrance.
func wireCraftSessionTombstone(handler *session.Handler, lifecycle *service.CraftLifecycle) {
	if handler == nil || lifecycle == nil {
		return
	}
	handler.SetCraftTombstoner(lifecycle)
}
