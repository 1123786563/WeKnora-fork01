package container

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/application/service"
	agentruntime "github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// CraftKnowledgeRunViewConfig contains only the application ports needed to
// build a knowledge package. Publisher and workspace ownership are bound per
// admitted Run by CraftKnowledgeRunViewBuilder.
type CraftKnowledgeRunViewConfig struct {
	Store      craft.Store
	Access     service.CraftKnowledgeAccess
	Search     service.CraftKnowledgeSearch
	TaskAccess craft.TaskAccessChecker
	Records    service.CraftKnowledgeRecordStore
}

// CraftKnowledgeRunViewAcceptance is the server-only identity of a package
// accepted for a Run. It intentionally contains no filesystem paths or bytes.
type CraftKnowledgeRunViewAcceptance struct {
	RunID         string
	PackageDigest string
}

// CraftKnowledgeRunViewBuilder constructs a fresh T05 service and publisher
// for each verified RunView material handle. No publisher or selected scope is
// shared between Runs.
type CraftKnowledgeRunViewBuilder struct {
	store      craft.Store
	access     service.CraftKnowledgeAccess
	search     service.CraftKnowledgeSearch
	taskAccess craft.TaskAccessChecker
	records    service.CraftKnowledgeRecordStore
}

// NewCraftKnowledgeRunViewBuilder validates the immutable dependencies used
// to build packages inside verified RunView generations.
func NewCraftKnowledgeRunViewBuilder(cfg CraftKnowledgeRunViewConfig) (*CraftKnowledgeRunViewBuilder, error) {
	if cfg.Store == nil || cfg.Access == nil || cfg.Search == nil || cfg.TaskAccess == nil || cfg.Records == nil {
		return nil, fmt.Errorf("craft: RunView knowledge builder requires store, access, search, task access and records")
	}
	return &CraftKnowledgeRunViewBuilder{
		store: cfg.Store, access: cfg.Access, search: cfg.Search,
		taskAccess: cfg.TaskAccess, records: cfg.Records,
	}, nil
}

// BuildForRun consumes only an admitted Run's strict durable snapshot and a
// capability issued by MaterialHandle. It binds ACL and record scope to the
// immutable actor, while resolving the existing workspace through the Run
// owner's persisted workspace binding.
func (b *CraftKnowledgeRunViewBuilder) BuildForRun(
	ctx context.Context,
	run agentruntime.Run,
	material CraftRunViewMaterialHandle,
) (CraftKnowledgeRunViewAcceptance, error) {
	if b == nil || b.store == nil || b.access == nil || b.search == nil || b.taskAccess == nil || b.records == nil {
		return CraftKnowledgeRunViewAcceptance{}, craft.ErrForbidden
	}
	scope, ownerScope, selection, err := craftKnowledgeRunIdentity(run)
	if err != nil {
		return CraftKnowledgeRunViewAcceptance{}, err
	}
	caller := types.CallerFromContext(ctx)
	if (caller.TenantID != 0 && caller.TenantID != scope.TenantID) || (caller.UserID != "" && caller.UserID != scope.UserID) {
		return CraftKnowledgeRunViewAcceptance{}, craft.ErrForbidden
	}
	ctx = types.WithCaller(ctx, types.Caller{TenantID: scope.TenantID, UserID: scope.UserID, Role: caller.Role})

	store := &craftKnowledgeRunViewStore{Store: b.store, actorScope: scope, ownerScope: ownerScope}
	if err := validateCraftKnowledgeRunViewHandle(material, run); err != nil {
		return CraftKnowledgeRunViewAcceptance{}, err
	}
	publisher, err := NewCraftKnowledgePackagePublisher(material.root, run.Key.RunID)
	if err != nil {
		return CraftKnowledgeRunViewAcceptance{}, fmt.Errorf("craft: bind RunView knowledge publisher: %w", err)
	}
	knowledge, err := service.NewCraftKnowledgeService(service.CraftKnowledgeConfig{
		Store: store, Access: b.access, Search: b.search, TaskAccess: b.taskAccess,
		Records: b.records, Publisher: publisher,
		// BuildForKnowledgeBases returns package bytes for atomic publication.
		// The legacy writer is required by the service constructor but must not
		// become a fallback material writer in this RunView path.
		Writer: func(context.Context, craft.Workspace, string, []byte) error {
			return errors.New("craft: direct workspace knowledge writes are disabled for RunView packages")
		},
	})
	if err != nil {
		return CraftKnowledgeRunViewAcceptance{}, fmt.Errorf("craft: assemble RunView knowledge service: %w", err)
	}
	if _, err := knowledge.BuildForKnowledgeBases(ctx, scope, run.Key.RunID, selection.Query, selection.KnowledgeBaseIDs); err != nil {
		return CraftKnowledgeRunViewAcceptance{}, err
	}
	record, err := b.records.Load(ctx, scope, run.Key.RunID)
	if err != nil {
		return CraftKnowledgeRunViewAcceptance{}, fmt.Errorf("craft: load accepted RunView knowledge record: %w", err)
	}
	if !craft.SameScope(scope, record.Scope) || record.RunID != run.Key.RunID ||
		record.PublicationState != craft.KnowledgePublicationPublished ||
		!validCraftKnowledgeRunViewDigest(record.PackageDigest) || (len(record.Sources) == 0) != record.Empty {
		return CraftKnowledgeRunViewAcceptance{}, craft.ErrConflict
	}
	workspace, err := store.GetWorkspace(ctx, scope)
	if err != nil {
		return CraftKnowledgeRunViewAcceptance{}, fmt.Errorf("craft: resolve RunView knowledge workspace: %w", err)
	}
	pkg, err := publisher.Resume(ctx, workspace, run.Key.RunID, record.PackageDigest)
	if err != nil {
		return CraftKnowledgeRunViewAcceptance{}, fmt.Errorf("craft: verify accepted RunView package: %w", err)
	}
	if pkg.RunID != run.Key.RunID || pkg.Directory != craft.KnowledgeRunDir(run.Key.RunID) || pkg.Digest != record.PackageDigest {
		return CraftKnowledgeRunViewAcceptance{}, craft.ErrConflict
	}
	return CraftKnowledgeRunViewAcceptance{RunID: record.RunID, PackageDigest: record.PackageDigest}, nil
}

// VerifyAcceptedForRun performs the final read-only verification for an
// already accepted Run package. It reloads the actor-scoped published record
// and resumes the exact accepted digest from the provider-issued generation;
// it does not search, build, publish, or mutate durable records.
func (b *CraftKnowledgeRunViewBuilder) VerifyAcceptedForRun(
	ctx context.Context,
	run agentruntime.Run,
	material CraftRunViewMaterialHandle,
	accepted CraftKnowledgeRunViewAcceptance,
) error {
	if b == nil || b.store == nil || b.records == nil || accepted.RunID != run.Key.RunID ||
		!validCraftKnowledgeRunViewDigest(accepted.PackageDigest) {
		return craft.ErrConflict
	}
	scope, ownerScope, _, err := craftKnowledgeRunIdentity(run)
	if err != nil {
		return err
	}
	caller := types.CallerFromContext(ctx)
	if (caller.TenantID != 0 && caller.TenantID != scope.TenantID) || (caller.UserID != "" && caller.UserID != scope.UserID) {
		return craft.ErrForbidden
	}
	ctx = types.WithCaller(ctx, types.Caller{TenantID: scope.TenantID, UserID: scope.UserID, Role: caller.Role})
	if err := validateCraftKnowledgeRunViewHandle(material, run); err != nil {
		return err
	}
	record, err := b.records.Load(ctx, scope, run.Key.RunID)
	if err != nil {
		return fmt.Errorf("craft: load accepted RunView knowledge record: %w", err)
	}
	if !craft.SameScope(scope, record.Scope) || record.RunID != run.Key.RunID ||
		record.PublicationState != craft.KnowledgePublicationPublished || record.PackageDigest != accepted.PackageDigest ||
		!validCraftKnowledgeRunViewDigest(record.PackageDigest) || (len(record.Sources) == 0) != record.Empty {
		return craft.ErrConflict
	}
	store := &craftKnowledgeRunViewStore{Store: b.store, actorScope: scope, ownerScope: ownerScope}
	workspace, err := store.GetWorkspace(ctx, scope)
	if err != nil {
		return fmt.Errorf("craft: resolve accepted RunView knowledge workspace: %w", err)
	}
	publisher, err := NewCraftKnowledgePackagePublisher(material.root, run.Key.RunID)
	if err != nil {
		return fmt.Errorf("craft: bind accepted RunView knowledge publisher: %w", err)
	}
	publishedPath := filepath.Join(material.root, filepath.FromSlash(craft.KnowledgeRunDir(run.Key.RunID)))
	publishedInfo, err := os.Lstat(publishedPath)
	if err != nil || publishedInfo.Mode()&os.ModeSymlink != 0 || !publishedInfo.IsDir() || publishedInfo.Mode().Perm() != 0555 {
		return craft.ErrConflict
	}
	if _, candidateErr := os.Lstat(publisher.candidatePath(run.Key.RunID, accepted.PackageDigest)); candidateErr == nil || !errors.Is(candidateErr, os.ErrNotExist) {
		return craft.ErrConflict
	}
	pkg, err := publisher.Resume(ctx, workspace, run.Key.RunID, accepted.PackageDigest)
	if err != nil {
		return fmt.Errorf("craft: verify accepted RunView knowledge package: %w", err)
	}
	if pkg.RunID != run.Key.RunID || pkg.Directory != craft.KnowledgeRunDir(run.Key.RunID) ||
		pkg.Digest != accepted.PackageDigest || len(pkg.Files) == 0 {
		return craft.ErrConflict
	}
	return ctx.Err()
}

type craftKnowledgeRunViewStore struct {
	craft.Store
	actorScope craft.Scope
	ownerScope craft.Scope
}

func (s *craftKnowledgeRunViewStore) GetWorkspace(ctx context.Context, scope craft.Scope) (craft.Workspace, error) {
	if s == nil || s.Store == nil || !craft.SameScope(scope, s.actorScope) {
		return craft.Workspace{}, craft.ErrForbidden
	}
	workspace, err := s.Store.GetWorkspace(ctx, s.ownerScope)
	if err != nil {
		return craft.Workspace{}, err
	}
	if workspace.ID == "" || !craft.SameScope(workspace.Scope, s.ownerScope) {
		return craft.Workspace{}, craft.ErrForbidden
	}
	// T05 uses one scope for authorization and package metadata. The persisted
	// Workspace is owner-bound, so adapt only its in-memory scope after the
	// owner lookup has proved the admitted Run's storage binding.
	workspace.Scope = s.actorScope
	return workspace, nil
}

func craftKnowledgeRunIdentity(run agentruntime.Run) (craft.Scope, craft.Scope, service.CraftKnowledgeSelectionSnapshot, error) {
	if run.Key.TenantID == 0 || !craftKnowledgeRunToken(run.Key.RunID) || len(run.Key.RunID) > 64 ||
		!craftKnowledgeRunToken(run.SessionID) || !craftKnowledgeRunToken(run.UserID) ||
		!craftKnowledgeRunToken(run.ActorUserID) || strings.TrimSpace(run.Key.RunID) != run.Key.RunID ||
		strings.TrimSpace(run.SessionID) != run.SessionID || strings.TrimSpace(run.UserID) != run.UserID ||
		strings.TrimSpace(run.ActorUserID) != run.ActorUserID {
		return craft.Scope{}, craft.Scope{}, service.CraftKnowledgeSelectionSnapshot{}, craft.ErrForbidden
	}
	snapshot, err := service.ParseDurableRunSnapshot(run.Snapshot)
	if err != nil || snapshot.CraftInputManifest == nil || snapshot.CraftKnowledgeSelection == nil {
		return craft.Scope{}, craft.Scope{}, service.CraftKnowledgeSelectionSnapshot{}, craft.ErrForbidden
	}
	actorScope := craft.Scope{TenantID: run.Key.TenantID, UserID: run.ActorUserID, SessionID: run.SessionID}
	ownerScope := craft.Scope{TenantID: run.Key.TenantID, UserID: run.UserID, SessionID: run.SessionID}
	return actorScope, ownerScope, *snapshot.CraftKnowledgeSelection, nil
}

func validateCraftKnowledgeRunViewHandle(handle CraftRunViewMaterialHandle, run agentruntime.Run) error {
	expected := craft.RunViewKey{TenantID: run.Key.TenantID, OwnerID: run.UserID, SessionID: run.SessionID, RunID: run.Key.RunID}
	if handle.key != expected || handle.provider == nil {
		return craft.ErrForbidden
	}
	if err := handle.provider.verifyMaterialHandle(handle); err != nil {
		return craft.ErrForbidden
	}
	return nil
}

func craftKnowledgeRunToken(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func validCraftKnowledgeRunViewDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
