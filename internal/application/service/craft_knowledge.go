package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// CraftKnowledgeConfig assembles the knowledge service. Every port is
// required; a nil port refuses assembly instead of degrading to a wider
// scope.
type CraftKnowledgeConfig struct {
	// Store resolves the scope-bound workspace (R02 contract).
	Store craft.Store
	// Access is the existing shared-aware knowledge ACL read.
	Access CraftKnowledgeAccess
	// Search is the existing per-library ACL-guarded retrieval.
	Search CraftKnowledgeSearch
	// Writer stages material files into the bound workspace (R03 writer).
	Writer WorkspaceFileWriter
}

// CraftKnowledgeService builds bounded knowledge material packages for
// craft runs.
type CraftKnowledgeService struct {
	store  craft.Store
	access CraftKnowledgeAccess
	search CraftKnowledgeSearch
	writer WorkspaceFileWriter
}

// NewCraftKnowledgeService validates the assembly.
func NewCraftKnowledgeService(cfg CraftKnowledgeConfig) (*CraftKnowledgeService, error) {
	if cfg.Store == nil || cfg.Access == nil || cfg.Search == nil || cfg.Writer == nil {
		return nil, errors.New("craft: knowledge service requires store, access, search and writer")
	}
	return &CraftKnowledgeService{
		store: cfg.Store, access: cfg.Access, search: cfg.Search, writer: cfg.Writer,
	}, nil
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
	Sources    []craftKnowledgeManifestSource `json:"sources"`
}

type craftKnowledgeManifestScope struct {
	TenantID  uint64 `json:"tenant_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

type craftKnowledgeManifestSource struct {
	CitationID      string `json:"citation_id"`
	Ref             string `json:"ref"`
	KnowledgeID     string `json:"knowledge_id"`
	KnowledgeBaseID string `json:"knowledge_base_id"`
	TenantID        uint64 `json:"tenant_id"`
	Title           string `json:"title,omitempty"`
	ExcerptBytes    int    `json:"excerpt_bytes"`
	Digest          string `json:"digest"`
	File            string `json:"file"`
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
	if s == nil || s.store == nil || s.access == nil || s.search == nil || s.writer == nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: knowledge service is not assembled", craft.ErrInvalidInput)
	}
	if scope.TenantID == 0 || scope.UserID == "" || scope.SessionID == "" {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: build requires a complete scope", craft.ErrInvalidInput)
	}
	if len(knowledgeIDs) > craft.MaxKnowledgeSources {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: %d knowledge selections exceed the %d source cap",
			craft.ErrInvalidInput, len(knowledgeIDs), craft.MaxKnowledgeSources)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: build requires a retrieval query", craft.ErrInvalidInput)
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
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: build requires at least one knowledge selection", craft.ErrInvalidInput)
	}

	// The context caller must match the server-derived scope: Build never
	// fabricates or upgrades identity for the ACL entrances below.
	caller := types.CallerFromContext(ctx)
	if caller.TenantID != scope.TenantID || caller.UserID != scope.UserID {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: build caller does not match the session scope", craft.ErrForbidden)
	}

	workspace, err := s.store.GetWorkspace(ctx, scope)
	if err != nil {
		return craft.KnowledgeBundle{}, err
	}
	if workspace.ID == "" {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: session %s has no craft workspace", craft.ErrNotFound, scope.SessionID)
	}
	if !craft.SameScope(scope, workspace.Scope) {
		return craft.KnowledgeBundle{}, fmt.Errorf("%w: workspace %s is bound to another scope", craft.ErrForbidden, workspace.ID)
	}

	// Existing ACL entrance #1: shared-aware document resolution. Rows that
	// return are exactly the documents this caller may read.
	rows, err := s.access(ctx, scope.TenantID, ids)
	if err != nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: resolve knowledge selections: %w", err)
	}
	rowsByID := make(map[string]*types.Knowledge, len(rows))
	for _, row := range rows {
		if row != nil && row.ID != "" {
			rowsByID[row.ID] = row
		}
	}
	for _, id := range ids {
		if rowsByID[id] == nil {
			return craft.KnowledgeBundle{}, fmt.Errorf(
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
				return craft.KnowledgeBundle{}, fmt.Errorf("%w: library %s denied the retrieval", craft.ErrForbidden, kbID)
			}
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: search library %s: %w", kbID, err)
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
			sources = append(sources, craft.Source{
				ID:       craft.KnowledgeCitationID(kbID, result.KnowledgeID, result.ID),
				Ref:      craft.KnowledgeRef(kbID, result.KnowledgeID, result.ID),
				Excerpt:  excerpt,
				Digest:   craftKnowledgeDigest(excerpt),
				TenantID: kbTenant[kbID],
			})
		}
	}

	bundle := craft.BoundSources(sources, craft.MaxKnowledgeBundleBytes)

	// Stage material files then the manifest; the sub-execution receives
	// only this bounded, data-marked package.
	manifest := craftKnowledgeManifest{
		Kind:       "craft.knowledge.manifest",
		DataNotice: craft.KnowledgeDataNotice,
		Scope: craftKnowledgeManifestScope{
			TenantID: scope.TenantID, UserID: scope.UserID, SessionID: scope.SessionID,
		},
		Query:     query,
		Truncated: bundle.Truncated,
		Sources:   make([]craftKnowledgeManifestSource, 0, len(bundle.Sources)),
	}
	for _, source := range bundle.Sources {
		knowledgeID := craftKnowledgeIDOfRef(source.Ref)
		file := craft.KnowledgeDir + "/" + source.ID + ".txt"
		material := strings.Join([]string{
			craft.KnowledgeDataNotice,
			"citation: " + source.ID,
			"ref: " + source.Ref,
			"",
			source.Excerpt,
		}, "\n") + "\n"
		if err := s.writer(ctx, workspace, file, []byte(material)); err != nil {
			return craft.KnowledgeBundle{}, fmt.Errorf("craft: stage knowledge material %s: %w", source.ID, err)
		}
		manifest.Sources = append(manifest.Sources, craftKnowledgeManifestSource{
			CitationID:      source.ID,
			Ref:             source.Ref,
			KnowledgeID:     knowledgeID,
			KnowledgeBaseID: craftKnowledgeBaseOfRef(source.Ref),
			TenantID:        source.TenantID,
			Title:           craft.ExcerptOf(titleOf[knowledgeID], 256),
			ExcerptBytes:    len(source.Excerpt),
			Digest:          source.Digest,
			File:            file,
		})
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: encode knowledge manifest: %w", err)
	}
	if err := s.writer(ctx, workspace, craft.KnowledgeManifestPath, encoded); err != nil {
		return craft.KnowledgeBundle{}, fmt.Errorf("craft: stage knowledge manifest: %w", err)
	}
	return bundle, nil
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
