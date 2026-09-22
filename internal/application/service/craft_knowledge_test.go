package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// The CraftKnowledgeService.Build seam reuses the two REAL existing ACL
// entrances the knowledge product already runs on:
//
//   - knowledge resolution goes through the real
//     knowledgeService.GetKnowledgeBatchWithSharedAccess (sqlite-backed
//     repository + access.KBPermissions via the share lookup), the same
//     entry buildSearchTargets uses for @mentioned documents;
//   - per-KB retrieval goes through the real knowledgeBaseService
//     authorizeKBAccess guard that HybridSearch runs before any fan-out.
//     The sqlite test cannot run the vector-store fan-out, so the bound
//     search port performs the real authorization and then serves fixture
//     results for the authorized KBs — production wiring binds the full
//     HybridSearch via BindCraftKnowledgeSearch.

// -----------------------------------------------------------------------------
// fixtures
// -----------------------------------------------------------------------------

func craftKnowledgeScope() craft.Scope {
	return craft.Scope{TenantID: 1, UserID: "u-craft", SessionID: "s-craft"}
}

func craftKnowledgeCtx(scope craft.Scope) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, scope.TenantID)
	ctx = context.WithValue(ctx, types.UserIDContextKey, scope.UserID)
	return ctx
}

// knowledgeWorkspaceStore answers Build's workspace resolution for one bound
// workspace. Only GetWorkspace is expected on this path.
type knowledgeWorkspaceStore struct {
	craft.Store
	ws craft.Workspace
}

func (s *knowledgeWorkspaceStore) GetWorkspace(context.Context, craft.Scope) (craft.Workspace, error) {
	return s.ws, nil
}

type knowledgeFixture struct {
	db      *gorm.DB
	shares  *fakeKBShareService
	writer  *recordingWriter
	results map[string][]*types.SearchResult
}

// newKnowledgeFixture builds the sqlite-backed real-ACL fixture: one shared
// share-lookup fake for both entrances and per-KB search fixtures.
func newKnowledgeFixture(t *testing.T, allowedKBs map[string]bool) *knowledgeFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&types.KnowledgeBase{}, &types.Knowledge{}))
	return &knowledgeFixture{
		db:      db,
		shares:  &fakeKBShareService{allowedKBs: allowedKBs},
		writer:  &recordingWriter{},
		results: map[string][]*types.SearchResult{},
	}
}

func (f *knowledgeFixture) seedKB(t *testing.T, kbID string, tenantID uint64) {
	t.Helper()
	require.NoError(t, f.db.Create(&types.KnowledgeBase{
		ID: kbID, TenantID: tenantID, Name: "kb " + kbID, Type: "document",
	}).Error)
}

func (f *knowledgeFixture) seedKnowledge(t *testing.T, id, kbID string, tenantID uint64, title string) {
	t.Helper()
	now := time.Now().UTC()
	require.NoError(t, f.db.Create(&types.Knowledge{
		ID: id, TenantID: tenantID, KnowledgeBaseID: kbID, Type: "file",
		Title: title, FileName: title + ".txt", FileType: "txt",
		ParseStatus: types.ParseStatusCompleted, EnableStatus: "enabled",
		CreatedAt: now, UpdatedAt: now,
	}).Error)
}

func (f *knowledgeFixture) seedChunk(kbID, knowledgeID, chunkID, content string) {
	f.results[kbID] = append(f.results[kbID], &types.SearchResult{
		ID: chunkID, KnowledgeID: knowledgeID, KnowledgeBaseID: kbID,
		Content: content, KnowledgeTitle: "doc-" + knowledgeID,
	})
}

// service assembles the CraftKnowledgeService under test. searchShares may
// differ from the access share lookup to simulate a revocation landing
// between the two entrances.
func (f *knowledgeFixture) service(t *testing.T, searchShares *fakeKBShareService) *CraftKnowledgeService {
	t.Helper()
	if searchShares == nil {
		searchShares = f.shares
	}
	accessSvc := &knowledgeService{
		repo:           repository.NewKnowledgeRepository(f.db),
		kbShareService: f.shares,
	}
	kbSvc := &knowledgeBaseService{
		repo:           repository.NewKnowledgeBaseRepository(f.db),
		kgRepo:         repository.NewKnowledgeRepository(f.db),
		kbShareService: searchShares,
	}
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		kbs, err := kbSvc.repo.GetKnowledgeBaseByIDs(ctx, []string{kbID})
		if err != nil {
			return nil, err
		}
		// REAL ACL entry: the exact authorizeKBAccess HybridSearch runs.
		if err := kbSvc.authorizeKBAccess(ctx, kbs); err != nil {
			return nil, err
		}
		wanted := make(map[string]bool, len(params.KnowledgeIDs))
		for _, id := range params.KnowledgeIDs {
			wanted[id] = true
		}
		var out []*types.SearchResult
		for _, r := range f.results[kbID] {
			if r.KnowledgeID == "" || wanted[r.KnowledgeID] {
				out = append(out, r)
			}
		}
		return out, nil
	}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store:  &knowledgeWorkspaceStore{ws: craft.Workspace{ID: "ws-knowledge", Scope: craftKnowledgeScope()}},
		Access: BindCraftKnowledgeAccess(accessSvc),
		Search: search,
		Writer: f.writer.write,
	})
	require.NoError(t, err)
	return svc
}

func writePathMap(w *recordingWriter) map[string][]byte {
	out := map[string][]byte{}
	for _, wr := range w.writes {
		out[wr.path] = wr.content
	}
	return out
}

func manifestOf(t *testing.T, w *recordingWriter) map[string]any {
	t.Helper()
	raw, ok := writePathMap(w)["knowledge/manifest.json"]
	require.True(t, ok, "knowledge manifest must be staged")
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func manifestSources(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	raw, ok := m["sources"].([]any)
	require.True(t, ok, "manifest sources missing")
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		row, ok := item.(map[string]any)
		require.True(t, ok)
		out = append(out, row)
	}
	return out
}

// -----------------------------------------------------------------------------
// scenarios
// -----------------------------------------------------------------------------

// Same-user authorized library plus an organization-shared library in another
// tenant: both become bounded sources, the shared source keeps the OWNER
// tenant (never the caller's tenant), and the material manifest is staged
// with stable citation IDs, refs and digests.
func TestCraftKnowledgeBuildStagesAuthorizedSources(t *testing.T) {
	f := newKnowledgeFixture(t, map[string]bool{"kb-shared": true})
	f.seedKB(t, "kb-own", 1)
	f.seedKB(t, "kb-shared", 2)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	f.seedKnowledge(t, "k-shared", "kb-shared", 2, "Shared Doc")
	f.seedChunk("kb-own", "k-own", "c-own-1", "own excerpt")
	f.seedChunk("kb-shared", "k-shared", "c-shared-1", "shared excerpt")

	scope := craftKnowledgeScope()
	bundle, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-own", "k-shared"})
	require.NoError(t, err)
	require.False(t, bundle.Truncated)
	require.Len(t, bundle.Sources, 2)

	own, shared := bundle.Sources[0], bundle.Sources[1]
	require.Equal(t, "kb-own/k-own/c-own-1", ownRefCoords(own.Ref))
	require.Equal(t, uint64(1), own.TenantID)
	require.Equal(t, "own excerpt", own.Excerpt)
	require.Equal(t, shaHex([]byte("own excerpt")), own.Digest)
	require.Equal(t, craft.KnowledgeCitationID("kb-own", "k-own", "c-own-1"), own.ID)

	// The shared library lives in another tenant: the source records the
	// owner tenant, not the caller's workspace.
	require.Equal(t, "kb-shared/k-shared/c-shared-1", ownRefCoords(shared.Ref))
	require.Equal(t, uint64(2), shared.TenantID)
	require.Equal(t, "shared excerpt", shared.Excerpt)

	files := writePathMap(f.writer)
	require.Len(t, files, 3, "two material files plus the manifest")
	for _, s := range bundle.Sources {
		raw, ok := files["knowledge/"+s.ID+".txt"]
		require.True(t, ok, "material file for %s missing", s.ID)
		require.Contains(t, string(raw), craft.KnowledgeDataNotice)
		require.Contains(t, string(raw), s.ID)
		require.Contains(t, string(raw), s.Excerpt)
	}
	m := manifestOf(t, f.writer)
	require.Equal(t, "craft.knowledge.manifest", m["kind"])
	require.Equal(t, craft.KnowledgeDataNotice, m["data_notice"])
	require.Equal(t, false, m["truncated"])
	require.Equal(t, "report", m["query"])
	rows := manifestSources(t, m)
	require.Len(t, rows, 2)
	require.Equal(t, own.ID, rows[0]["citation_id"])
	require.Equal(t, own.Ref, rows[0]["ref"])
	require.Equal(t, "k-own", rows[0]["knowledge_id"])
	require.Equal(t, "kb-own", rows[0]["knowledge_base_id"])
	require.Equal(t, float64(1), rows[0]["tenant_id"])
	for _, wr := range f.writer.writes {
		require.Equal(t, "ws-knowledge", wr.workspace.ID)
	}
}

// ownRefCoords extracts the kb/knowledge/chunk coordinates from a
// craft.KnowledgeRef for readable assertions.
func ownRefCoords(ref string) string {
	const prefix = "craftkb://kb/"
	if !strings.HasPrefix(ref, prefix) {
		return "<unexpected ref " + ref + ">"
	}
	rest := strings.TrimPrefix(ref, prefix)
	rest = strings.ReplaceAll(rest, "/knowledge/", "/")
	return strings.ReplaceAll(rest, "/chunk/", "/")
}

// A requested document from a library the caller cannot read (no share, no
// ownership) fails the WHOLE request before anything is staged.
func TestCraftKnowledgeBuildRejectsUnauthorizedKnowledge(t *testing.T) {
	f := newKnowledgeFixture(t, map[string]bool{})
	f.seedKB(t, "kb-own", 1)
	f.seedKB(t, "kb-foreign", 2)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	f.seedKnowledge(t, "k-foreign", "kb-foreign", 2, "Foreign Doc")
	f.seedChunk("kb-own", "k-own", "c-own-1", "own excerpt")

	scope := craftKnowledgeScope()
	_, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-own", "k-foreign"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, f.writer.writes, "unauthorized resources must never reach the writer")
}

// A share revoked between knowledge resolution and per-KB retrieval (torn
// state) still fails the whole request fail-closed.
func TestCraftKnowledgeBuildFailsWhenSearchACLDenies(t *testing.T) {
	f := newKnowledgeFixture(t, map[string]bool{"kb-shared": true})
	f.seedKB(t, "kb-shared", 2)
	f.seedKnowledge(t, "k-shared", "kb-shared", 2, "Shared Doc")
	f.seedChunk("kb-shared", "k-shared", "c-shared-1", "shared excerpt")

	scope := craftKnowledgeScope()
	_, err := f.service(t, &fakeKBShareService{allowedKBs: map[string]bool{}}).
		Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-shared"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, f.writer.writes, "a denied retrieval must not stage anything")
}

// Revoking the share after one successful build removes access immediately:
// the second build re-resolves live permissions and fails — no cached
// permission snapshot keeps the old access alive.
func TestCraftKnowledgeBuildFailsAfterShareRevoked(t *testing.T) {
	shares := map[string]bool{"kb-shared": true}
	f := newKnowledgeFixture(t, shares)
	f.seedKB(t, "kb-shared", 2)
	f.seedKnowledge(t, "k-shared", "kb-shared", 2, "Shared Doc")
	f.seedChunk("kb-shared", "k-shared", "c-shared-1", "shared excerpt")

	scope := craftKnowledgeScope()
	svc := f.service(t, nil)
	bundle, err := svc.Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-shared"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	writesAfterFirst := len(f.writer.writes)

	delete(shares, "kb-shared")
	_, err = svc.Build(craftKnowledgeCtx(scope), scope, "report again", []string{"k-shared"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Len(t, f.writer.writes, writesAfterFirst, "revoked share must not leave a stale permission snapshot")
}

// Material larger than the bundle budget is bounded: the byte cap keeps only
// the sources that fit (truncated), and the count cap keeps at most
// MaxKnowledgeSources entries.
func TestCraftKnowledgeBuildBoundsOversizedMaterial(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-big", 1)
	f.seedKnowledge(t, "k-big", "kb-big", 1, "Big Doc")
	bigExcerpt := strings.Repeat("b", 4<<10) // 4 KiB each
	for i := 0; i < 30; i++ {                // 120 KiB total > 64 KiB bundle cap
		f.seedChunk("kb-big", "k-big", "c-big-"+string(rune('a'+i)), bigExcerpt)
	}

	scope := craftKnowledgeScope()
	bundle, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-big"})
	require.NoError(t, err)
	require.True(t, bundle.Truncated)
	var used int
	for _, s := range bundle.Sources {
		used += len(s.Excerpt)
		require.LessOrEqual(t, len(s.Excerpt), craft.MaxKnowledgeExcerptBytes)
	}
	require.LessOrEqual(t, used, craft.MaxKnowledgeBundleBytes)
	// 4 KiB excerpts fill the 64 KiB budget with exactly 16 sources.
	require.Equal(t, craft.MaxKnowledgeBundleBytes/(4<<10), len(bundle.Sources))
	require.Equal(t, true, manifestOf(t, f.writer)["truncated"])
}

// More matching chunks than the per-bundle count cap: at most twenty sources
// survive even when the byte budget is far from exhausted.
func TestCraftKnowledgeBuildCapsSourceCount(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-many", 1)
	f.seedKnowledge(t, "k-many", "kb-many", 1, "Many Doc")
	for i := 0; i < craft.MaxKnowledgeSources+9; i++ {
		f.seedChunk("kb-many", "k-many", "c-many-"+string(rune('a'+i)), "tiny")
	}

	scope := craftKnowledgeScope()
	bundle, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-many"})
	require.NoError(t, err)
	require.True(t, bundle.Truncated)
	require.Len(t, bundle.Sources, craft.MaxKnowledgeSources)
}

// One huge chunk is cut to the per-source excerpt cap before bounding.
func TestCraftKnowledgeBuildCapsSingleExcerpt(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-huge", 1)
	f.seedKnowledge(t, "k-huge", "kb-huge", 1, "Huge Doc")
	f.seedChunk("kb-huge", "k-huge", "c-huge-1", strings.Repeat("x", 20<<10))

	scope := craftKnowledgeScope()
	bundle, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-huge"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	// The cap bounds the content; the truncation marker may add one rune.
	require.LessOrEqual(t, len(bundle.Sources[0].Excerpt), craft.MaxKnowledgeExcerptBytes+3)
	require.Equal(t, shaHex([]byte(bundle.Sources[0].Excerpt)), bundle.Sources[0].Digest)
}

// Authorized library with no matching chunks: an honest empty bundle plus an
// empty manifest — not an error, not a silent scope expansion.
func TestCraftKnowledgeBuildEmptyRetrievalReturnsEmptyBundle(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-quiet", 1)
	f.seedKnowledge(t, "k-quiet", "kb-quiet", 1, "Quiet Doc")

	scope := craftKnowledgeScope()
	bundle, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-quiet"})
	require.NoError(t, err)
	require.NotNil(t, bundle.Sources)
	require.Empty(t, bundle.Sources)
	require.False(t, bundle.Truncated)
	rows := manifestSources(t, manifestOf(t, f.writer))
	require.Empty(t, rows)
}

// Retrieval rows outside the requested documents (or web-search noise) are
// never packaged as knowledge sources: the model cannot smuggle unrequested
// citations into the material manifest.
func TestCraftKnowledgeBuildDropsUnrequestedResults(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-own", 1)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	f.seedKnowledge(t, "k-other", "kb-own", 1, "Other Doc")
	f.seedChunk("kb-own", "k-own", "c-own-1", "own excerpt")
	f.seedChunk("kb-own", "k-other", "c-other-1", "other excerpt")
	f.results["kb-own"] = append(f.results["kb-own"], &types.SearchResult{
		ID: "w-1", KnowledgeBaseID: "kb-own", Content: "web noise",
		KnowledgeSource: "web_search",
	})

	scope := craftKnowledgeScope()
	bundle, err := f.service(t, nil).Build(craftKnowledgeCtx(scope), scope, "report", []string{"k-own"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	require.Equal(t, "own excerpt", bundle.Sources[0].Excerpt)
	_, staged := writePathMap(f.writer)["knowledge/"+bundle.Sources[0].ID+".txt"]
	require.True(t, staged)
	require.Len(t, manifestSources(t, manifestOf(t, f.writer)), 1)
}

// Malformed requests and identity mismatches fail before any ACL or staging
// work: incomplete scope, empty query, empty or oversized selection, and a
// context caller that does not match the server-derived scope.
func TestCraftKnowledgeBuildRejectsMalformedRequests(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-own", 1)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	f.seedChunk("kb-own", "k-own", "c-own-1", "own excerpt")
	svc := f.service(t, nil)
	scope := craftKnowledgeScope()

	cases := []struct {
		name  string
		scope craft.Scope
		query string
		ids   []string
		want  error
	}{
		{"incomplete scope", craft.Scope{TenantID: 1, UserID: "u-craft"}, "q", []string{"k-own"}, craft.ErrInvalidInput},
		{"empty query", scope, "   ", []string{"k-own"}, craft.ErrInvalidInput},
		{"no knowledge", scope, "q", nil, craft.ErrInvalidInput},
		{"oversized selection", scope, "q", make([]string, craft.MaxKnowledgeSources+1), craft.ErrInvalidInput},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Build(craftKnowledgeCtx(scope), tc.scope, tc.query, tc.ids)
			require.ErrorIs(t, err, tc.want)
		})
	}

	// The context caller must match the server-derived scope: Build never
	// fabricates or upgrades identity.
	otherTenant := craftKnowledgeCtx(craft.Scope{TenantID: 2, UserID: "u-craft", SessionID: "s-craft"})
	_, err := svc.Build(otherTenant, scope, "q", []string{"k-own"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, f.writer.writes)
}

// A scope with no bound workspace refuses to build: material never lands in
// an unbound sandbox.
func TestCraftKnowledgeBuildRequiresBoundWorkspace(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-own", 1)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	f.seedChunk("kb-own", "k-own", "c-own-1", "own excerpt")

	accessSvc := &knowledgeService{
		repo:           repository.NewKnowledgeRepository(f.db),
		kbShareService: f.shares,
	}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store:  &knowledgeWorkspaceStore{craft.Store(nil), craft.Workspace{}},
		Access: BindCraftKnowledgeAccess(accessSvc),
		Search: func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
			return nil, nil
		},
		Writer: f.writer.write,
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	_, err = svc.Build(craftKnowledgeCtx(scope), scope, "q", []string{"k-own"})
	require.ErrorIs(t, err, craft.ErrNotFound)
	require.Empty(t, f.writer.writes)
}

// The assembly rejects a config missing any of its required ports.
func TestCraftKnowledgeServiceRequiresAllPorts(t *testing.T) {
	_, err := NewCraftKnowledgeService(CraftKnowledgeConfig{})
	require.Error(t, err)
}

// BindCraftKnowledgeSearch pins the production binding: the bound port calls
// the existing HybridSearch ACL-guarded entry.
func TestCraftKnowledgeSearchBindsHybridSearch(t *testing.T) {
	calls := 0
	svc := &hybridSearchProbe{onSearch: func() { calls++ }}
	port := BindCraftKnowledgeSearch(svc)
	_, err := port(context.Background(), "kb-1", types.SearchParams{QueryText: "q"})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

type hybridSearchProbe struct {
	onSearch func()
}

func (p *hybridSearchProbe) HybridSearch(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
	p.onSearch()
	return nil, nil
}

// Silent-scope-expansion guard: an error from the access entrance aborts the
// whole build instead of degrading to searching every library.
func TestCraftKnowledgeBuildPropagatesAccessErrors(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-own", 1)
	f.seedKnowledge(t, "k-own", "kb-own", 1, "Own Doc")
	searched := 0
	accessSvc := &knowledgeService{
		repo:           repository.NewKnowledgeRepository(f.db),
		kbShareService: f.shares,
	}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: &knowledgeWorkspaceStore{ws: craft.Workspace{ID: "ws-knowledge", Scope: craftKnowledgeScope()}},
		Access: func(context.Context, uint64, []string) ([]*types.Knowledge, error) {
			return nil, errors.New("share lookup unavailable")
		},
		Search: func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
			searched++
			return nil, nil
		},
		Writer: f.writer.write,
	})
	require.NoError(t, err)
	_ = accessSvc
	scope := craftKnowledgeScope()
	_, err = svc.Build(craftKnowledgeCtx(scope), scope, "q", []string{"k-own"})
	require.Error(t, err)
	require.Equal(t, 0, searched, "an access failure must abort before any per-KB search")
	require.Empty(t, f.writer.writes)
}
