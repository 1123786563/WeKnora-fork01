package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
)

// C01: the Craft knowledge material package. Build runs the main agent's
// knowledge retrieval through the EXISTING user/space/shared-library ACL
// entrances — it never re-implements permissions and never expands scope on
// failure. The sub-execution receives only the bounded material files and
// the manifest; database, vector-store and full-library credentials stay
// with the main agent.

// CraftKnowledgeAccess resolves the selected knowledge rows through the
// existing shared-aware read ACL (own workspace rows plus organization-
// shared libraries, which legitimately live in other tenants). Production
// binds knowledgeService.GetKnowledgeBatchWithSharedAccess — the same entry
// buildSearchTargets uses for @mentioned documents.
type CraftKnowledgeAccess func(ctx context.Context, tenantID uint64, knowledgeIDs []string) ([]*types.Knowledge, error)

// CraftKnowledgeSearch performs ONE library's ACL-guarded retrieval.
// Production binds knowledgeBaseService.HybridSearch, which authorizes every
// KB for the original caller (authorizeKBAccess) before any fan-out and
// fails closed on unauthorized libraries.
type CraftKnowledgeSearch func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error)

// sharedKnowledgeReader is the narrow existing surface the access port
// binds to; satisfied by the production knowledge service.
type sharedKnowledgeReader interface {
	GetKnowledgeBatchWithSharedAccess(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error)
}

// hybridKnowledgeSearcher is the narrow existing surface the search port
// binds to; satisfied by the production knowledge-base service.
type hybridKnowledgeSearcher interface {
	HybridSearch(ctx context.Context, id string, params types.SearchParams) ([]*types.SearchResult, error)
}

// BindCraftKnowledgeAccess binds the existing shared-aware knowledge read
// to the Craft access port.
func BindCraftKnowledgeAccess(svc sharedKnowledgeReader) CraftKnowledgeAccess {
	return func(ctx context.Context, tenantID uint64, knowledgeIDs []string) ([]*types.Knowledge, error) {
		return svc.GetKnowledgeBatchWithSharedAccess(ctx, tenantID, knowledgeIDs)
	}
}

// BindCraftKnowledgeSearch binds the existing ACL-guarded per-library
// retrieval (HybridSearch) to the Craft search port.
func BindCraftKnowledgeSearch(svc hybridKnowledgeSearcher) CraftKnowledgeSearch {
	return func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		return svc.HybridSearch(ctx, kbID, params)
	}
}

// CraftKnowledgeConfig assembles the knowledge service. Legacy Build ports
// are required. BuildForRun additionally requires the atomic Publisher and
// refuses use when that port is missing.
type CraftKnowledgeConfig struct {
	// Store resolves the scope-bound workspace (R02 contract).
	Store craft.Store
	// Access is the existing shared-aware knowledge ACL read.
	Access CraftKnowledgeAccess
	// Search is the existing per-library ACL-guarded retrieval.
	Search CraftKnowledgeSearch
	// Writer stages material files into the bound workspace (R03 writer).
	Writer WorkspaceFileWriter
	// TaskAccess is the current Task membership authority supplied by T08.
	TaskAccess craft.TaskAccessChecker
	// Records durably inserts and reads immutable per-Run source observations.
	Records CraftKnowledgeRecordStore
	// Publisher atomically publishes complete Run material packages. BuildForRun
	// fails closed when this port is not assembled.
	Publisher CraftKnowledgePackagePublisher
	Now       func() time.Time
}

// CraftKnowledgeRecordStore must insert a Run record once and reject a
// different payload for an existing Run ID. Load must scope tenant/session.
type CraftKnowledgeRecordStore interface {
	Save(context.Context, craft.KnowledgeRecord) error
	Load(context.Context, craft.Scope, string) (craft.KnowledgeRecord, error)
	MarkPublished(context.Context, craft.Scope, string, string) error
}

// CraftKnowledgeMaterialPackage is the complete immutable byte set for one
// Run. Every workspace-relative target path in Files must remain beneath
// Directory.
type CraftKnowledgeMaterialPackage struct {
	RunID, Directory, Digest string
	Files                    map[string][]byte
}

// CraftKnowledgePackagePublisher must prepare packages privately and publish
// them atomically only when the whole package is complete. Prepare must not
// change visible files; Publish must verify Digest and atomically expose the
// complete package at Directory. Prepare and Publish are idempotent for an
// identical RunID+Digest; a different digest for an existing Run must return
// craft.ErrConflict without changing visible files. Discard removes only the
// private candidate for this exact package. A failed operation must never
// expose a partial package at Directory.
type CraftKnowledgePackagePublisher interface {
	Prepare(context.Context, craft.Workspace, CraftKnowledgeMaterialPackage) error
	Publish(context.Context, craft.Workspace, CraftKnowledgeMaterialPackage) error
	Discard(context.Context, craft.Workspace, CraftKnowledgeMaterialPackage) error
}

// CraftKnowledgePackageResumer exposes recovery of an exact accepted package.
// It is separate from the base publisher so existing adapters remain usable
// for new Runs; retries fail closed unless this capability is assembled.
type CraftKnowledgePackageResumer interface {
	// Resume returns the exact already prepared or published package accepted
	// for runID and acceptedDigest. It must not regenerate from live retrieval.
	Resume(context.Context, craft.Workspace, string, string) (CraftKnowledgeMaterialPackage, error)
}

// CraftKnowledgeService builds bounded knowledge material packages for
// craft runs.
type CraftKnowledgeService struct {
	store      craft.Store
	access     CraftKnowledgeAccess
	search     CraftKnowledgeSearch
	writer     WorkspaceFileWriter
	taskAccess craft.TaskAccessChecker
	records    CraftKnowledgeRecordStore
	publisher  CraftKnowledgePackagePublisher
	now        func() time.Time
}

// NewCraftKnowledgeService validates the assembly.
func NewCraftKnowledgeService(cfg CraftKnowledgeConfig) (*CraftKnowledgeService, error) {
	if cfg.Store == nil || cfg.Access == nil || cfg.Search == nil || cfg.Writer == nil {
		return nil, errors.New("craft: knowledge service requires store, access, search and writer")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &CraftKnowledgeService{
		store: cfg.Store, access: cfg.Access, search: cfg.Search, writer: cfg.Writer,
		taskAccess: cfg.TaskAccess, records: cfg.Records, publisher: cfg.Publisher, now: now,
	}, nil
}

// BuildForRun is the authoritative Craft entry point. Its selections are
// server-stored document IDs; it checks current Task access, then the existing
// knowledge ACL, persists an immutable package identity, and publishes only a
// complete package for this Run.
func (s *CraftKnowledgeService) BuildForRun(ctx context.Context, scope craft.Scope, runID, query string, selectedKnowledgeIDs []string) (craft.KnowledgeBundle, error) {
	return s.buildForRun(ctx, scope, runID, craftKnowledgeRequestDigest(query, selectedKnowledgeIDs),
		func(ctx context.Context, materialDir string, acquiredAt map[string]time.Time) (craft.Workspace, craft.KnowledgeBundle, map[string][]byte, error) {
			return s.buildPackage(ctx, scope, query, selectedKnowledgeIDs, materialDir, acquiredAt)
		}, nil)
}

// BuildForKnowledgeBases is the authoritative Craft entry point for the
// approved user selection contract. It takes knowledge-base IDs (never
// document IDs), searches each selected KB through its existing ACL entrance,
// and intersects results with the shared-aware document access port.
func (s *CraftKnowledgeService) BuildForKnowledgeBases(ctx context.Context, scope craft.Scope, runID, query string, selectedKnowledgeBaseIDs []string) (craft.KnowledgeBundle, error) {
	ids, err := canonicalCraftKnowledgeBaseIDs(selectedKnowledgeBaseIDs)
	if err != nil {
		return craft.KnowledgeBundle{}, err
	}
	return s.buildForRun(ctx, scope, runID, craftKnowledgeBaseRequestDigest(query, ids),
		func(ctx context.Context, materialDir string, acquiredAt map[string]time.Time) (craft.Workspace, craft.KnowledgeBundle, map[string][]byte, error) {
			return s.buildPackageForKnowledgeBases(ctx, scope, query, ids, materialDir, acquiredAt)
		}, func(ctx context.Context, record craft.KnowledgeRecord) error {
			return s.reauthorizeSelectedKnowledgeBases(ctx, query, ids, record)
		})
}

type craftKnowledgePackageBuilder func(context.Context, string, map[string]time.Time) (craft.Workspace, craft.KnowledgeBundle, map[string][]byte, error)
type craftKnowledgeReplayAuthorizer func(context.Context, craft.KnowledgeRecord) error

func (s *CraftKnowledgeService) buildForRun(ctx context.Context, scope craft.Scope, runID, requestDigest string, build craftKnowledgePackageBuilder, authorizeReplay craftKnowledgeReplayAuthorizer) (craft.KnowledgeBundle, error) {
	materialDir := craft.KnowledgeRunDir(runID)
	if s == nil || s.records == nil || s.publisher == nil || materialDir == "" {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: run material publication is unavailable", craft.ErrInvalidInput)
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskWrite); err != nil {
		return craft.KnowledgeBundle{}, err
	}
	existing, loadErr := s.records.Load(ctx, scope, runID)
	hasExisting := loadErr == nil
	if loadErr != nil && !errors.Is(loadErr, craft.ErrNotFound) {
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: load prior Run source record: %w", loadErr)
	}
	if hasExisting && !craft.SameScope(scope, existing.Scope) {
		return craft.KnowledgeBundle{}, craft.ErrForbidden
	}
	if hasExisting && existing.RequestDigest != requestDigest {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: knowledge request for Run %s changed", craft.ErrConflict, runID)
	}
	if hasExisting {
		if existing.PublicationState != "" && existing.PublicationState != craft.KnowledgePublicationPrepared && existing.PublicationState != craft.KnowledgePublicationPublished {
			return craft.KnowledgeBundle{}, fmt.Errorf("%w: unknown publication state %q", craft.ErrConflict, existing.PublicationState)
		}
		workspace, err := s.workspaceForScope(ctx, scope)
		if err != nil {
			return craft.KnowledgeBundle{}, err
		}
		resumer, ok := s.publisher.(CraftKnowledgePackageResumer)
		if !ok {
			return craft.KnowledgeBundle{}, fmt.Errorf("%w: knowledge package recovery is unavailable", craft.ErrInvalidInput)
		}
		pkg, err := resumer.Resume(ctx, workspace, runID, existing.PackageDigest)
		if err != nil {
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: resume accepted knowledge package: %w", err)
		}
		bundle, manifest, err := craftKnowledgeBundleFromPackage(scope, existing, materialDir, pkg)
		if err != nil {
			return craft.KnowledgeBundle{}, err
		}
		if err := s.reauthorizeKnowledgePackage(ctx, scope, existing, manifest); err != nil {
			return craft.KnowledgeBundle{}, err
		}
		if authorizeReplay != nil {
			if err := authorizeReplay(ctx, existing); err != nil {
				return craft.KnowledgeBundle{}, err
			}
		}
		if err := s.publisher.Publish(ctx, workspace, pkg); err != nil {
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: publish resumed knowledge package: %w", err)
		}
		if existing.PublicationState != craft.KnowledgePublicationPublished {
			if err := s.records.MarkPublished(ctx, scope, runID, existing.PackageDigest); err != nil {
				return craft.KnowledgeBundle{}, fmt.Errorf("craft: mark knowledge package published: %w", err)
			}
		}
		return bundle, nil
	}
	workspace, bundle, files, err := build(ctx, materialDir, nil)
	if err != nil {
		return craft.KnowledgeBundle{}, err
	}
	packageDigest := craftKnowledgePackageDigest(files)
	record := craft.KnowledgeRecord{
		Scope: scope, RunID: runID, RequestDigest: requestDigest, PackageDigest: packageDigest,
		PublicationState: craft.KnowledgePublicationPrepared,
		Empty:            bundle.Empty, Truncated: bundle.Truncated,
		Sources: make([]craft.KnowledgeSourceRecord, 0, len(bundle.Sources)),
	}
	for _, source := range bundle.Sources {
		record.Sources = append(record.Sources, craft.KnowledgeSourceRecord{ID: source.ID, Ref: source.Ref, Digest: source.Digest, TenantID: source.TenantID, AcquiredAt: source.AcquiredAt, ExcerptBytes: len(source.Excerpt)})
	}
	pkg := CraftKnowledgeMaterialPackage{RunID: runID, Directory: materialDir, Digest: packageDigest, Files: files}
	if err := s.publisher.Prepare(ctx, workspace, pkg); err != nil {
		discardErr := s.publisher.Discard(ctx, workspace, pkg)
		if discardErr != nil {
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: prepare private knowledge package: %w (private package cleanup failed: %v)", err, discardErr)
		}
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: prepare private knowledge package: %w", err)
	}
	if err := s.records.Save(ctx, record); err != nil {
		discardErr := s.publisher.Discard(ctx, workspace, pkg)
		if discardErr != nil {
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: persist actual sources: %w (private package cleanup failed: %v)", err, discardErr)
		}
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: persist actual sources: %w", err)
	}
	if err := s.publisher.Publish(ctx, workspace, pkg); err != nil {
		// The candidate remains private and can be retried with the same
		// immutable record; it is never read by a delegate before Publish.
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: publish knowledge package: %w", err)
	}
	if err := s.records.MarkPublished(ctx, scope, runID, packageDigest); err != nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: mark knowledge package published: %w", err)
	}
	return bundle, nil
}

// AuthorizeSourceOpen returns only a recorded stable ref after fresh Task and
// underlying knowledge checks. A caller must resolve the ref through the
// existing source service; this method never follows a model-provided URL.
func (s *CraftKnowledgeService) AuthorizeSourceOpen(ctx context.Context, scope craft.Scope, runID, citationID string) (string, error) {
	if s == nil || s.records == nil || s.access == nil {
		return "", craft.ErrForbidden
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskOpenSource); err != nil {
		return "", err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return "", craft.ErrForbidden
	}
	record, err := s.records.Load(ctx, scope, runID)
	if err != nil {
		return "", err
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != runID {
		return "", craft.ErrForbidden
	}
	if record.PublicationState != craft.KnowledgePublicationPublished {
		return "", craft.ErrConflict
	}
	for _, source := range record.Sources {
		if source.ID != citationID {
			continue
		}
		knowledgeID := craftKnowledgeIDOfRef(source.Ref)
		kbID := craftKnowledgeBaseOfRef(source.Ref)
		if knowledgeID == "" || kbID == "" {
			return "", craft.ErrNotFound
		}
		rows, err := s.access(ctx, scope.TenantID, []string{knowledgeID})
		if err != nil {
			return "", err
		}
		for _, row := range rows {
			if row != nil && row.ID == knowledgeID && row.KnowledgeBaseID == kbID && row.TenantID == source.TenantID {
				return source.Ref, nil
			}
		}
		return "", craft.ErrForbidden
	}
	return "", craft.ErrNotFound
}

// Sources projects a historical Run's facts only to a current Task reader.
// Historical facts remain immutable; opening the underlying document has a
// separate fresh authorization step.
func (s *CraftKnowledgeService) Sources(ctx context.Context, scope craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	if s == nil || s.records == nil {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskRead); err != nil {
		return craft.KnowledgeRecord{}, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	record, err := s.records.Load(ctx, scope, runID)
	if err != nil {
		return craft.KnowledgeRecord{}, err
	}
	if record.Scope.TenantID != scope.TenantID || record.Scope.SessionID != scope.SessionID || record.RunID != runID {
		return craft.KnowledgeRecord{}, craft.ErrForbidden
	}
	if record.PublicationState != craft.KnowledgePublicationPublished {
		return craft.KnowledgeRecord{}, craft.ErrConflict
	}
	return record, nil
}

// RevalidateForDispatch is the read-only authority gate for a future Craft
// delegate dispatch. It accepts only the exact published Run record, checks
// the authenticated actor and current TaskWrite grant, and re-resolves every
// recorded source through the shared-aware document/KB access port. It never
// searches, rebuilds, publishes, or changes the accepted record.
func (s *CraftKnowledgeService) RevalidateForDispatch(ctx context.Context, scope craft.Scope, runID string) error {
	if s == nil || s.records == nil || s.access == nil || s.search == nil || craft.KnowledgeRunDir(runID) == "" {
		return craft.ErrForbidden
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.ErrForbidden
	}
	if err := craft.RequireTaskAccess(ctx, s.taskAccess, scope, craft.TaskWrite); err != nil {
		return err
	}
	record, err := s.records.Load(ctx, scope, runID)
	if err != nil {
		return err
	}
	if !craft.SameScope(scope, record.Scope) {
		return craft.ErrForbidden
	}
	if record.RunID != runID {
		return fmt.Errorf("%w: accepted knowledge record Run does not match dispatch", craft.ErrConflict)
	}
	if record.PublicationState != craft.KnowledgePublicationPublished {
		return fmt.Errorf("%w: accepted knowledge package is not published", craft.ErrConflict)
	}
	if record.RequestDigest == "" || record.PackageDigest == "" {
		return fmt.Errorf("%w: accepted knowledge record has no immutable digest", craft.ErrConflict)
	}
	if (len(record.Sources) == 0) != record.Empty {
		return fmt.Errorf("%w: accepted knowledge record empty flag conflicts with its sources", craft.ErrConflict)
	}
	return s.reauthorizeKnowledgeRecordSources(ctx, scope, record)
}

// reauthorizeKnowledgeRecordSources re-resolves immutable source coordinates
// through CraftKnowledgeAccess. The production adapter is the shared-aware
// GetKnowledgeBatchWithSharedAccess path, which checks current document
// visibility and current owning-KB sharing grants. Search output and the
// Publisher are deliberately absent from this dispatch-time read check.
func (s *CraftKnowledgeService) reauthorizeKnowledgeRecordSources(ctx context.Context, scope craft.Scope, record craft.KnowledgeRecord) error {
	type coordinates struct {
		knowledgeBaseID string
		tenantID        uint64
	}
	expected := make(map[string]coordinates, len(record.Sources))
	for _, source := range record.Sources {
		knowledgeID := craftKnowledgeIDOfRef(source.Ref)
		knowledgeBaseID := craftKnowledgeBaseOfRef(source.Ref)
		if knowledgeID == "" || knowledgeBaseID == "" || source.TenantID == 0 {
			return fmt.Errorf("%w: accepted knowledge source has incomplete authorization coordinates", craft.ErrConflict)
		}
		if previous, exists := expected[knowledgeID]; exists {
			if previous.knowledgeBaseID != knowledgeBaseID || previous.tenantID != source.TenantID {
				return fmt.Errorf("%w: accepted knowledge source coordinates conflict", craft.ErrConflict)
			}
			continue
		}
		expected[knowledgeID] = coordinates{knowledgeBaseID: knowledgeBaseID, tenantID: source.TenantID}
	}
	if len(expected) == 0 {
		return nil
	}
	ids := make([]string, 0, len(expected))
	for id := range expected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	rows, err := s.access(ctx, scope.TenantID, ids)
	if err != nil {
		if errors.Is(err, craft.ErrForbidden) || craftKnowledgeAccessDenied(err) {
			return craft.ErrForbidden
		}
		return fmt.Errorf("craft: revalidate accepted knowledge sources: %w", err)
	}
	rowsByID := make(map[string]*types.Knowledge, len(rows))
	for _, row := range rows {
		if row == nil || row.ID == "" {
			continue
		}
		if prior, duplicate := rowsByID[row.ID]; duplicate && (prior.KnowledgeBaseID != row.KnowledgeBaseID || prior.TenantID != row.TenantID) {
			return fmt.Errorf("%w: duplicate knowledge authorization rows conflict", craft.ErrConflict)
		}
		rowsByID[row.ID] = row
	}
	for id, coordinates := range expected {
		row := rowsByID[id]
		if row == nil || row.ID != id || row.KnowledgeBaseID != coordinates.knowledgeBaseID || row.TenantID != coordinates.tenantID {
			return craft.ErrForbidden
		}
	}
	return nil
}

func (s *CraftKnowledgeService) workspaceForScope(ctx context.Context, scope craft.Scope) (craft.Workspace, error) {
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.Workspace{}, fmt.Errorf("%w: build caller does not match the session scope", craft.ErrForbidden)
	}
	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return craft.Workspace{}, err
	}
	if workspace.ID == "" {
		return craft.Workspace{}, fmt.Errorf("%w: session %s has no craft workspace", craft.ErrNotFound, scope.SessionID)
	}
	if !craft.SameScope(scope, workspace.Scope) {
		return craft.Workspace{}, fmt.Errorf("%w: workspace %s is bound to another scope", craft.ErrForbidden, workspace.ID)
	}
	return workspace, nil
}

// reauthorizeKnowledgePackage checks current authority for every immutable
// source before a resumed package is published. Search results are discarded:
// this is an ACL check, never a rebuild from mutable retrieval output.
func (s *CraftKnowledgeService) reauthorizeKnowledgePackage(ctx context.Context, scope craft.Scope, record craft.KnowledgeRecord, manifest craftKnowledgeManifest) error {
	type authorizedDocument struct {
		knowledgeBaseID string
		tenantID        uint64
	}
	documents := make(map[string]authorizedDocument, len(record.Sources))
	documentsByKB := make(map[string][]string)
	for _, source := range record.Sources {
		knowledgeID := craftKnowledgeIDOfRef(source.Ref)
		knowledgeBaseID := craftKnowledgeBaseOfRef(source.Ref)
		if knowledgeID == "" || knowledgeBaseID == "" || source.TenantID == 0 {
			return fmt.Errorf("%w: accepted knowledge source has incomplete authorization coordinates", craft.ErrConflict)
		}
		if prior, exists := documents[knowledgeID]; exists {
			if prior.knowledgeBaseID != knowledgeBaseID || prior.tenantID != source.TenantID {
				return fmt.Errorf("%w: accepted knowledge source coordinates conflict", craft.ErrConflict)
			}
			continue
		}
		documents[knowledgeID] = authorizedDocument{knowledgeBaseID: knowledgeBaseID, tenantID: source.TenantID}
		documentsByKB[knowledgeBaseID] = append(documentsByKB[knowledgeBaseID], knowledgeID)
	}
	if len(documents) == 0 {
		return nil
	}
	if strings.TrimSpace(manifest.Query) == "" {
		return fmt.Errorf("%w: accepted knowledge package has no authorization query", craft.ErrConflict)
	}
	documentIDs := make([]string, 0, len(documents))
	for id := range documents {
		documentIDs = append(documentIDs, id)
	}
	sort.Strings(documentIDs)
	rows, err := s.access(ctx, scope.TenantID, documentIDs)
	if err != nil {
		if errors.Is(err, craft.ErrForbidden) || craftKnowledgeAccessDenied(err) {
			return craft.ErrForbidden
		}
		return fmt.Errorf("craft: reauthorize accepted knowledge documents: %w", err)
	}
	rowsByID := make(map[string]*types.Knowledge, len(rows))
	for _, row := range rows {
		if row == nil || row.ID == "" {
			continue
		}
		if prior, duplicate := rowsByID[row.ID]; duplicate && (prior.KnowledgeBaseID != row.KnowledgeBaseID || prior.TenantID != row.TenantID) {
			return fmt.Errorf("%w: duplicate knowledge authorization rows conflict", craft.ErrConflict)
		}
		rowsByID[row.ID] = row
	}
	for id, expected := range documents {
		row := rowsByID[id]
		if row == nil || row.ID != id || row.KnowledgeBaseID != expected.knowledgeBaseID || row.TenantID != expected.tenantID {
			return craft.ErrForbidden
		}
	}
	kbIDs := make([]string, 0, len(documentsByKB))
	for id := range documentsByKB {
		kbIDs = append(kbIDs, id)
		sort.Strings(documentsByKB[id])
	}
	sort.Strings(kbIDs)
	for _, kbID := range kbIDs {
		// HybridSearch is the existing guarded ACL entrance. Its result bytes
		// are intentionally ignored; only successful authorization permits
		// publication of the already accepted package.
		_, err := s.search(ctx, kbID, types.SearchParams{
			QueryText: manifest.Query, KnowledgeIDs: documentsByKB[kbID],
			MatchCount: craft.MaxKnowledgeSources, SkipContextEnrichment: true,
		})
		if err != nil {
			if errors.Is(err, craft.ErrForbidden) || craftKnowledgeAccessDenied(err) {
				return craft.ErrForbidden
			}
			return fmt.Errorf("craft: reauthorize accepted knowledge library %s: %w", kbID, err)
		}
	}
	return nil
}

// craftKnowledgeManifest is the staged material manifest: the sub-execution's
// only map of what the material is and where each citation resolves. It
// carries excerpt digests, never full text, and marks every byte as data.
type craftKnowledgeManifest struct {
	Kind       string                         `json:"kind"`
	DataNotice string                         `json:"data_notice"`
	Scope      craftKnowledgeManifestScope    `json:"scope"`
	Query      string                         `json:"query"`
	Truncated  bool                           `json:"truncated"`
	Empty      bool                           `json:"empty"`
	Sources    []craftKnowledgeManifestSource `json:"sources"`
}

type craftKnowledgeManifestScope struct {
	TenantID  uint64 `json:"tenant_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

type craftKnowledgeManifestSource struct {
	CitationID      string    `json:"citation_id"`
	Ref             string    `json:"ref"`
	KnowledgeID     string    `json:"knowledge_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	TenantID        uint64    `json:"tenant_id"`
	Title           string    `json:"title,omitempty"`
	ExcerptBytes    int       `json:"excerpt_bytes"`
	Digest          string    `json:"digest"`
	AcquiredAt      time.Time `json:"acquired_at"`
	File            string    `json:"file"`
	Truncated       bool      `json:"truncated,omitempty"`
}

func craftKnowledgeBundleFromPackage(scope craft.Scope, record craft.KnowledgeRecord, materialDir string, pkg CraftKnowledgeMaterialPackage) (craft.KnowledgeBundle, craftKnowledgeManifest, error) {
	conflict := func(message string) (craft.KnowledgeBundle, craftKnowledgeManifest, error) {
		return craft.KnowledgeBundle{}, craftKnowledgeManifest{}, fmt.Errorf("%w: %s", craft.ErrConflict, message)
	}
	if pkg.RunID != record.RunID || pkg.Directory != materialDir || pkg.Digest == "" || pkg.Digest != record.PackageDigest || craftKnowledgePackageDigest(pkg.Files) != record.PackageDigest {
		return conflict("resumed knowledge package does not match the accepted digest")
	}
	manifestPath := materialDir + "/manifest.json"
	manifestBytes, ok := pkg.Files[manifestPath]
	if !ok {
		return conflict("resumed knowledge package has no manifest")
	}
	var manifest craftKnowledgeManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return conflict("resumed knowledge manifest is invalid")
	}
	if manifest.Kind != "craft.knowledge.manifest" || manifest.DataNotice != craft.KnowledgeDataNotice ||
		manifest.Scope.TenantID != scope.TenantID || manifest.Scope.UserID != scope.UserID || manifest.Scope.SessionID != scope.SessionID ||
		manifest.Empty != record.Empty || manifest.Truncated != record.Truncated || len(manifest.Sources) != len(record.Sources) ||
		len(pkg.Files) != len(manifest.Sources)+1 {
		return conflict("resumed knowledge manifest does not match the accepted source record")
	}
	byID := make(map[string]craft.KnowledgeSourceRecord, len(record.Sources))
	for _, source := range record.Sources {
		if source.ID == "" || byID[source.ID].ID != "" {
			return conflict("accepted knowledge record has duplicate or empty citation IDs")
		}
		byID[source.ID] = source
	}
	bundle := craft.KnowledgeBundle{Empty: manifest.Empty, Truncated: manifest.Truncated, Sources: make([]craft.Source, 0, len(manifest.Sources))}
	for _, entry := range manifest.Sources {
		recorded, exists := byID[entry.CitationID]
		if !exists || entry.CitationID == "" || entry.Ref != recorded.Ref || entry.Digest != recorded.Digest ||
			entry.TenantID != recorded.TenantID || !entry.AcquiredAt.Equal(recorded.AcquiredAt) || entry.ExcerptBytes != recorded.ExcerptBytes ||
			entry.KnowledgeID != craftKnowledgeIDOfRef(entry.Ref) || entry.KnowledgeBaseID != craftKnowledgeBaseOfRef(entry.Ref) {
			return conflict("resumed knowledge source does not match its accepted record")
		}
		expectedPath := materialDir + "/" + entry.CitationID + ".txt"
		if entry.File != expectedPath {
			return conflict("resumed knowledge source path is invalid")
		}
		material, ok := pkg.Files[entry.File]
		if !ok {
			return conflict("resumed knowledge source file is missing")
		}
		prefix := craft.KnowledgeDataNotice + "\ncitation: " + entry.CitationID + "\nref: " + entry.Ref + "\n\n"
		content := string(material)
		if !strings.HasPrefix(content, prefix) || !strings.HasSuffix(content, "\n") || len(content) < len(prefix)+1 {
			return conflict("resumed knowledge source file has an invalid envelope")
		}
		excerpt := strings.TrimSuffix(strings.TrimPrefix(content, prefix), "\n")
		if len(excerpt) != entry.ExcerptBytes || craftKnowledgeDigest(excerpt) != entry.Digest {
			return conflict("resumed knowledge excerpt does not match its accepted digest")
		}
		bundle.Sources = append(bundle.Sources, craft.Source{
			ID: entry.CitationID, Ref: entry.Ref, Excerpt: excerpt, Digest: entry.Digest,
			TenantID: entry.TenantID, AcquiredAt: entry.AcquiredAt, Truncated: entry.Truncated,
		})
		delete(byID, entry.CitationID)
	}
	if len(byID) != 0 || bundle.Empty != (len(bundle.Sources) == 0) {
		return conflict("resumed knowledge package source set is incomplete")
	}
	return bundle, manifest, nil
}

// Build assembles one knowledge material package for the scope's bound
// workspace:
//
//  1. request shape and caller identity are validated — the context caller
//     must match the server-derived scope; Build never fabricates identity;
//  2. the selected documents are resolved through the existing shared-aware
//     read ACL. A document that does not come back authorized fails the
//     WHOLE request: no silent drop, no fallback to wider libraries;
//  3. each owning library is retrieved through the existing per-library
//     ACL-guarded search (HybridSearch semantics), one call per library,
//     restricted to that library's authorized documents;
//  4. results outside the requested documents (or web-search noise) are
//     never packaged — the manifest cannot smuggle unrequested citations;
//  5. excerpts are bounded per source, the bundle is bounded to twenty
//     sources / 64 KiB, and only then are material files + manifest staged
//     into the workspace. A shared library's source records the OWNER
//     tenant, never the caller's.
//
// The returned bundle keeps every citation ID, ref and digest so the main
// agent retains citations and retrieval purpose while the sub-execution
// receives only the staged data.
func (s *CraftKnowledgeService) Build(
	ctx context.Context,
	scope craft.Scope,
	query string,
	knowledgeIDs []string,
) (craft.KnowledgeBundle, error) {
	workspace, bundle, files, err := s.buildPackage(ctx, scope, query, knowledgeIDs, craft.KnowledgeDir, nil)
	if err != nil {
		return craft.KnowledgeBundle{}, err
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if err := s.writer(ctx, workspace, path, files[path]); err != nil {
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: stage knowledge material %s: %w", path, err)
		}
	}
	return bundle, nil
}

func (s *CraftKnowledgeService) buildPackage(
	ctx context.Context,
	scope craft.Scope,
	query string,
	knowledgeIDs []string,
	materialDir string,
	acquiredAt map[string]time.Time,
) (craft.Workspace, craft.KnowledgeBundle, map[string][]byte, error) {
	if s == nil || s.store == nil || s.access == nil || s.search == nil || s.writer == nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: knowledge service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build requires a complete scope", craft.ErrInvalidInput)
	}
	if len(knowledgeIDs) > craft.MaxKnowledgeSources {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: %d knowledge selections exceed the %d source cap",
			craft.ErrInvalidInput, len(knowledgeIDs), craft.MaxKnowledgeSources)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build requires a retrieval query", craft.ErrInvalidInput)
	}
	ids := make([]string, 0, len(knowledgeIDs))
	seenIDs := make(map[string]bool, len(knowledgeIDs))
	for _, id := range knowledgeIDs {
		id = strings.TrimSpace(id)
		if id == "" || seenIDs[id] {
			continue
		}
		seenIDs[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build requires at least one knowledge selection", craft.ErrInvalidInput)
	}

	// The context caller must match the server-derived scope: Build never
	// fabricates or upgrades identity for the ACL entrances below.
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build caller does not match the session scope", craft.ErrForbidden)
	}

	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, err
	}
	if workspace.ID == "" {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: session %s has no craft workspace", craft.ErrNotFound, scope.SessionID)
	}
	if !craft.SameScope(scope, workspace.Scope) {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: workspace %s is bound to another scope", craft.ErrForbidden, workspace.ID)
	}

	// Existing ACL entrance #1: shared-aware document resolution. Rows that
	// return are exactly the documents this caller may read.
	rows, err := s.access(ctx, scope.TenantID, ids)
	if err != nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("craft: resolve knowledge selections: %w", err)
	}
	rowsByID := make(map[string]*types.Knowledge, len(rows))
	for _, row := range rows {
		if row != nil && row.ID != "" {
			rowsByID[row.ID] = row
		}
	}
	for _, id := range ids {
		if rowsByID[id] == nil {
			return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf(
				"%w: knowledge %s is outside the caller's authorized libraries", craft.ErrForbidden, id)
		}
	}

	// Group the authorized documents by owning library, preserving request
	// order for deterministic bundles.
	kbOrder := make([]string, 0, len(rows))
	docsInKB := make(map[string][]string)
	kbTenant := make(map[string]uint64)
	titleOf := make(map[string]string)
	for _, id := range ids {
		row := rowsByID[id]
		if _, ok := docsInKB[row.KnowledgeBaseID]; !ok {
			kbOrder = append(kbOrder, row.KnowledgeBaseID)
			kbTenant[row.KnowledgeBaseID] = row.TenantID
		}
		docsInKB[row.KnowledgeBaseID] = append(docsInKB[row.KnowledgeBaseID], row.ID)
		title := row.Title
		if title == "" {
			title = row.FileName
		}
		titleOf[row.ID] = title
	}

	// Existing ACL entrance #2: one retrieval per owning library, restricted
	// to that library's authorized documents.
	var sources []craft.Source
	seenChunks := make(map[string]bool)
	for _, kbID := range kbOrder {
		results, err := s.search(ctx, kbID, types.SearchParams{
			QueryText:             query,
			KnowledgeIDs:          docsInKB[kbID],
			MatchCount:            craft.MaxKnowledgeSources,
			SkipContextEnrichment: true,
		})
		if err != nil {
			if craftKnowledgeAccessDenied(err) {
				return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: library %s denied the retrieval", craft.ErrForbidden, kbID)
			}
			return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("craft: search library %s: %w", kbID, err)
		}
		authorized := make(map[string]bool, len(docsInKB[kbID]))
		for _, id := range docsInKB[kbID] {
			authorized[id] = true
		}
		for _, result := range results {
			if result == nil || result.ID == "" || seenChunks[result.ID] {
				continue
			}
			// Only rows belonging to the authorized documents of THIS
			// library are packaged; web-search noise is not knowledge
			// material.
			if !authorized[result.KnowledgeID] || isCraftWebReference(result) {
				continue
			}
			seenChunks[result.ID] = true
			excerpt := craft.ExcerptOf(result.Content, craft.MaxKnowledgeExcerptBytes)
			acquired := s.now().UTC()
			citationID := craft.KnowledgeCitationID(kbID, result.KnowledgeID, result.ID)
			if prior, ok := acquiredAt[citationID]; ok && !prior.IsZero() {
				acquired = prior
			}
			sources = append(sources, craft.Source{
				ID:         citationID,
				Ref:        craft.KnowledgeRef(kbID, result.KnowledgeID, result.ID),
				Excerpt:    excerpt,
				Digest:     craftKnowledgeDigest(excerpt),
				TenantID:   kbTenant[kbID],
				AcquiredAt: acquired,
				Truncated:  len(excerpt) < len(result.Content),
			})
		}
	}

	bundle := craft.BoundSources(sources, craft.MaxKnowledgeBundleBytes)
	files, err := craftKnowledgePackageFiles(scope, query, materialDir, bundle, titleOf)
	if err != nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, err
	}
	return workspace, bundle, files, nil
}

func craftKnowledgePackageFiles(scope craft.Scope, query, materialDir string, bundle craft.KnowledgeBundle, titleOf map[string]string) (map[string][]byte, error) {
	// Build a complete package in memory. The caller hands it to the atomic
	// publisher only after its digest-bound source record is durable.
	files := make(map[string][]byte, len(bundle.Sources)+1)
	manifest := craftKnowledgeManifest{
		Kind:       "craft.knowledge.manifest",
		DataNotice: craft.KnowledgeDataNotice,
		Scope: craftKnowledgeManifestScope{
			TenantID: scope.TenantID, UserID: scope.UserID, SessionID: scope.SessionID,
		},
		Query:     query,
		Truncated: bundle.Truncated,
		Empty:     bundle.Empty,
		Sources:   make([]craftKnowledgeManifestSource, 0, len(bundle.Sources)),
	}
	for _, source := range bundle.Sources {
		knowledgeID := craftKnowledgeIDOfRef(source.Ref)
		file := materialDir + "/" + source.ID + ".txt"
		material := strings.Join([]string{
			craft.KnowledgeDataNotice,
			"citation: " + source.ID,
			"ref: " + source.Ref,
			"",
			source.Excerpt,
		}, "\n") + "\n"
		files[file] = []byte(material)
		manifest.Sources = append(manifest.Sources, craftKnowledgeManifestSource{
			CitationID:      source.ID,
			Ref:             source.Ref,
			KnowledgeID:     knowledgeID,
			KnowledgeBaseID: craftKnowledgeBaseOfRef(source.Ref),
			TenantID:        source.TenantID,
			Title:           craft.ExcerptOf(titleOf[knowledgeID], 256),
			ExcerptBytes:    len(source.Excerpt),
			Digest:          source.Digest,
			AcquiredAt:      source.AcquiredAt,
			File:            file,
			Truncated:       source.Truncated,
		})
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("craft: encode knowledge manifest: %w", err)
	}
	manifestPath := materialDir + "/manifest.json"
	if materialDir == craft.KnowledgeDir {
		manifestPath = craft.KnowledgeManifestPath
	}
	files[manifestPath] = encoded
	return files, nil
}

func (s *CraftKnowledgeService) buildPackageForKnowledgeBases(
	ctx context.Context,
	scope craft.Scope,
	query string,
	knowledgeBaseIDs []string,
	materialDir string,
	acquiredAt map[string]time.Time,
) (craft.Workspace, craft.KnowledgeBundle, map[string][]byte, error) {
	if s == nil || s.store == nil || s.access == nil || s.search == nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: knowledge service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build requires a complete scope", craft.ErrInvalidInput)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build requires a retrieval query", craft.ErrInvalidInput)
	}
	ids, err := canonicalCraftKnowledgeBaseIDs(knowledgeBaseIDs)
	if err != nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, err
	}
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: build caller does not match the session scope", craft.ErrForbidden)
	}
	workspace, err := s.workspaceForScope(ctx, scope)
	if err != nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, err
	}

	type candidate struct {
		kbID   string
		result *types.SearchResult
	}
	candidates := make([]candidate, 0, len(ids)*craft.MaxKnowledgeSources)
	documentIDs := make([]string, 0, len(ids)*craft.MaxKnowledgeSources)
	seenDocuments := make(map[string]bool)
	retrievalSaturated := false
	searchLimit := craft.MaxKnowledgeSources + 1 // one bounded sentinel beyond the package source cap
	for _, kbID := range ids {
		// HybridSearch checks the caller's current access to this exact KB.
		// A single-KB filter is also explicit in params to prevent broadening
		// when the backend supports multi-KB SearchParams.
		results, searchErr := s.search(ctx, kbID, types.SearchParams{
			QueryText:             query,
			KnowledgeBaseIDs:      []string{kbID},
			MatchCount:            searchLimit,
			SkipContextEnrichment: true,
		})
		if searchErr != nil {
			if craftKnowledgeAccessDenied(searchErr) {
				return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("%w: library %s denied the retrieval", craft.ErrForbidden, kbID)
			}
			return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("craft: search selected library %s: %w", kbID, searchErr)
		}
		if len(results) >= searchLimit {
			// HybridSearch returns at most MatchCount. A full sentinel cap
			// cannot prove that later eligible results do not exist, especially
			// when document ACL filtering removes returned candidates.
			retrievalSaturated = true
		}
		count := 0
		for _, result := range results {
			if result == nil || result.ID == "" || result.KnowledgeID == "" || isCraftWebReference(result) {
				continue
			}
			if count >= craft.MaxKnowledgeSources+1 {
				break
			}
			count++
			candidates = append(candidates, candidate{kbID: kbID, result: result})
			if !seenDocuments[result.KnowledgeID] {
				seenDocuments[result.KnowledgeID] = true
				documentIDs = append(documentIDs, result.KnowledgeID)
			}
		}
	}

	rowsByID := make(map[string]*types.Knowledge)
	if len(documentIDs) > 0 {
		rows, accessErr := s.access(ctx, scope.TenantID, documentIDs)
		if accessErr != nil {
			if errors.Is(accessErr, craft.ErrForbidden) || craftKnowledgeAccessDenied(accessErr) {
				return craft.Workspace{}, craft.KnowledgeBundle{}, nil, craft.ErrForbidden
			}
			return craft.Workspace{}, craft.KnowledgeBundle{}, nil, fmt.Errorf("craft: resolve selected knowledge documents: %w", accessErr)
		}
		for _, row := range rows {
			if row != nil && row.ID != "" {
				rowsByID[row.ID] = row
			}
		}
	}

	sources := make([]craft.Source, 0, len(candidates))
	titleOf := make(map[string]string, len(rowsByID))
	seenChunks := make(map[string]bool)
	kbTenant := make(map[string]uint64)
	for _, candidate := range candidates {
		result := candidate.result
		row := rowsByID[result.KnowledgeID]
		// HybridSearch is scoped to the requested KB, but intersect its output
		// with the independent shared-aware document ACL and owning KB. This
		// drops denied documents and any cross-KB/noisy result fail-closed.
		if row == nil || row.KnowledgeBaseID != candidate.kbID || row.TenantID == 0 || seenChunks[result.ID] {
			continue
		}
		seenChunks[result.ID] = true
		kbTenant[candidate.kbID] = row.TenantID
		title := row.Title
		if title == "" {
			title = row.FileName
		}
		titleOf[row.ID] = title
		excerpt := craft.ExcerptOf(result.Content, craft.MaxKnowledgeExcerptBytes)
		acquired := s.now().UTC()
		citationID := craft.KnowledgeCitationID(candidate.kbID, row.ID, result.ID)
		if prior, ok := acquiredAt[citationID]; ok && !prior.IsZero() {
			acquired = prior
		}
		sources = append(sources, craft.Source{
			ID: citationID, Ref: craft.KnowledgeRef(candidate.kbID, row.ID, result.ID),
			Excerpt: excerpt, Digest: craftKnowledgeDigest(excerpt), TenantID: kbTenant[candidate.kbID],
			AcquiredAt: acquired, Truncated: len(excerpt) < len(result.Content),
		})
	}
	bundle := craft.BoundSources(sources, craft.MaxKnowledgeBundleBytes)
	bundle.Truncated = bundle.Truncated || retrievalSaturated
	files, err := craftKnowledgePackageFiles(scope, query, materialDir, bundle, titleOf)
	if err != nil {
		return craft.Workspace{}, craft.KnowledgeBundle{}, nil, err
	}
	return workspace, bundle, files, nil
}

func canonicalCraftKnowledgeBaseIDs(selected []string) ([]string, error) {
	if len(selected) == 0 || len(selected) > craft.MaxKnowledgeSources {
		return nil, fmt.Errorf("%w: knowledge-base selection must contain 1 to %d IDs", craft.ErrInvalidInput, craft.MaxKnowledgeSources)
	}
	ids := make([]string, 0, len(selected))
	seen := make(map[string]bool, len(selected))
	for _, raw := range selected {
		id := strings.TrimSpace(raw)
		if id == "" || len(id) > 128 || strings.ContainsAny(id, "\x00\r\n") {
			return nil, fmt.Errorf("%w: invalid selected knowledge-base ID", craft.ErrInvalidInput)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: duplicate selected knowledge-base ID", craft.ErrInvalidInput)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

func craftKnowledgeBaseRequestDigest(query string, knowledgeBaseIDs []string) string {
	data, _ := json.Marshal(struct {
		Kind             string   `json:"kind"`
		Query            string   `json:"query"`
		KnowledgeBaseIDs []string `json:"knowledge_base_ids"`
	}{Kind: "knowledge_base_selection", Query: strings.TrimSpace(query), KnowledgeBaseIDs: knowledgeBaseIDs})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *CraftKnowledgeService) reauthorizeSelectedKnowledgeBases(ctx context.Context, query string, selected []string, record craft.KnowledgeRecord) error {
	recorded := make(map[string]bool)
	for _, source := range record.Sources {
		recorded[craftKnowledgeBaseOfRef(source.Ref)] = true
	}
	for _, kbID := range selected {
		if recorded[kbID] {
			// reauthorizeKnowledgePackage already passed this KB and its exact
			// document IDs through the guarded Search and document ACL ports.
			continue
		}
		// Empty/filtered KBs have no source coordinates in the durable record.
		// Re-enter the existing guarded HybridSearch port to prove current KB
		// access; discard its output and never rebuild the accepted package.
		_, err := s.search(ctx, kbID, types.SearchParams{
			QueryText: query, KnowledgeBaseIDs: []string{kbID},
			MatchCount: craft.MaxKnowledgeSources, SkipContextEnrichment: true,
		})
		if err != nil {
			if craftKnowledgeAccessDenied(err) {
				return fmt.Errorf("%w: selected library %s access was revoked", craft.ErrForbidden, kbID)
			}
			return fmt.Errorf("craft: reauthorize selected library %s: %w", kbID, err)
		}
	}
	return nil
}

func craftKnowledgeRequestDigest(query string, selectedKnowledgeIDs []string) string {
	selected := make([]string, 0, len(selectedKnowledgeIDs))
	seen := make(map[string]bool, len(selectedKnowledgeIDs))
	for _, id := range selectedKnowledgeIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		selected = append(selected, id)
	}
	data, _ := json.Marshal(struct {
		Query    string   `json:"query"`
		Selected []string `json:"selected"`
	}{Query: strings.TrimSpace(query), Selected: selected})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func craftKnowledgePackageDigest(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		_, _ = fmt.Fprintf(h, "%d:%s%d:", len(path), path, len(files[path]))
		_, _ = h.Write(files[path])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func cloneCraftKnowledgePackage(pkg CraftKnowledgeMaterialPackage) CraftKnowledgeMaterialPackage {
	copyFiles := make(map[string][]byte, len(pkg.Files))
	for path, content := range pkg.Files {
		copyFiles[path] = append([]byte(nil), content...)
	}
	pkg.Files = copyFiles
	return pkg
}

// isCraftWebReference reports retrieval noise that is not knowledge-library
// material.
func isCraftWebReference(result *types.SearchResult) bool {
	if result == nil {
		return true
	}
	return strings.EqualFold(result.KnowledgeSource, "web_search") ||
		strings.EqualFold(result.ChunkType, string(types.ChunkTypeWebSearch))
}

// craftKnowledgeAccessDenied classifies the existing entrances' fail-closed
// permission errors (forbidden / not-found from authorizeKBAccess) so craft
// callers see craft.ErrForbidden without losing the original error.
func craftKnowledgeAccessDenied(err error) bool {
	appErr, ok := apperrors.IsAppError(err)
	if !ok {
		return false
	}
	return appErr.Code == apperrors.ErrForbidden || appErr.Code == apperrors.ErrNotFound
}

// craftKnowledgeDigest is the canonical excerpt digest written to sources
// and the manifest.
func craftKnowledgeDigest(excerpt string) string {
	sum := sha256.Sum256([]byte(excerpt))
	return hex.EncodeToString(sum[:])
}

// craftKnowledgeIDOfRef extracts the knowledge coordinate from a
// craft.KnowledgeRef for manifest rows.
func craftKnowledgeIDOfRef(ref string) string {
	const marker = "/knowledge/"
	at := strings.Index(ref, marker)
	if at < 0 {
		return ""
	}
	rest := ref[at+len(marker):]
	if end := strings.Index(rest, "/chunk/"); end >= 0 {
		return rest[:end]
	}
	return rest
}

// craftKnowledgeBaseOfRef extracts the library coordinate from a
// craft.KnowledgeRef for manifest rows.
func craftKnowledgeBaseOfRef(ref string) string {
	const prefix = "craftkb://kb/"
	if !strings.HasPrefix(ref, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(ref, prefix)
	if end := strings.Index(rest, "/knowledge/"); end >= 0 {
		return rest[:end]
	}
	return rest
}
