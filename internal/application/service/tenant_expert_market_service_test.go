package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/agent/experts"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// fakePublishedExpertStore is the repository.PublishedExpertRepository seam.
type fakePublishedExpertStore struct {
	mu   sync.Mutex
	rows map[string]*types.PublishedExpertEntity // key: "<tenant>/<agent>"
}

func publishedExpertKey(tenantID uint64, agentID string) string {
	return fmt.Sprintf("%d/%s", tenantID, agentID)
}

func (f *fakePublishedExpertStore) Upsert(_ context.Context, e *types.PublishedExpertEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]*types.PublishedExpertEntity{}
	}
	k := publishedExpertKey(e.TenantID, e.AgentID)
	if existing, ok := f.rows[k]; ok {
		existing.Name = e.Name
		existing.Description = e.Description
		existing.SnapshotRef = e.SnapshotRef
		existing.SnapshotSHA256 = e.SnapshotSHA256
		existing.PublishedBy = e.PublishedBy
		existing.UpdatedAt = e.UpdatedAt
		return nil
	}
	f.rows[k] = e
	return nil
}

func (f *fakePublishedExpertStore) ListByTenant(_ context.Context, tenantID uint64) ([]types.PublishedExpertEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]types.PublishedExpertEntity, 0)
	for _, row := range f.rows {
		if row.TenantID == tenantID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (f *fakePublishedExpertStore) GetByTenantAndAgent(_ context.Context, tenantID uint64, agentID string) (*types.PublishedExpertEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.rows[publishedExpertKey(tenantID, agentID)]
	if row == nil {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

func (f *fakePublishedExpertStore) GetByTenantAndID(_ context.Context, tenantID uint64, id string) (*types.PublishedExpertEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, row := range f.rows {
		if row.TenantID == tenantID && row.ID == id {
			copy := *row
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakePublishedExpertStore) Delete(_ context.Context, tenantID uint64, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k, row := range f.rows {
		if row.TenantID == tenantID && row.ID == id {
			delete(f.rows, k)
			return nil
		}
	}
	return nil
}

// fakeExpertInstalls is the repository.ExpertInstallRepository seam.
type fakeExpertInstalls struct {
	mu   sync.Mutex
	rows map[string]*types.ExpertInstallEntity // key: "<tenant>/<slug>"
}

func expertInstallKey(tenantID uint64, slug string) string {
	return fmt.Sprintf("%d/%s", tenantID, slug)
}

func (f *fakeExpertInstalls) Upsert(_ context.Context, e *types.ExpertInstallEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]*types.ExpertInstallEntity{}
	}
	k := expertInstallKey(e.TenantID, e.Slug)
	if existing, ok := f.rows[k]; ok {
		existing.StorageRef = e.StorageRef
		existing.SnapshotSHA256 = e.SnapshotSHA256
		existing.CreatedBy = e.CreatedBy
		existing.UpdatedAt = e.UpdatedAt
		return nil
	}
	f.rows[k] = e
	return nil
}

func (f *fakeExpertInstalls) ListByTenant(_ context.Context, tenantID uint64) ([]types.ExpertInstallEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]types.ExpertInstallEntity, 0)
	for _, row := range f.rows {
		if row.TenantID == tenantID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (f *fakeExpertInstalls) GetByTenantAndSlug(_ context.Context, tenantID uint64, slug string) (*types.ExpertInstallEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.rows[expertInstallKey(tenantID, slug)]
	if row == nil {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

func (f *fakeExpertInstalls) Delete(_ context.Context, tenantID uint64, slug string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rows, expertInstallKey(tenantID, slug))
	return nil
}

// fakeExpertAgentSource is the agent-read seam (the GetAgentByIDAndTenant
// slice of interfaces.CustomAgentService).
type fakeExpertAgentSource struct {
	agents map[string]*types.CustomAgent // key: "<tenant>/<id>"
}

func (f *fakeExpertAgentSource) GetAgentByIDAndTenant(_ context.Context, id string, tenantID uint64) (*types.CustomAgent, error) {
	agent := f.agents[publishedExpertKey(tenantID, id)]
	if agent == nil {
		return nil, ErrAgentNotFound
	}
	copy := *agent
	return &copy, nil
}

// fakeTenantExpertInstantiator is the M2 Instantiate seam
// (MarketExpertInstantiator).
type fakeTenantExpertInstantiator struct {
	mu    sync.Mutex
	calls []tenantExpertInstantiateCall
}

type tenantExpertInstantiateCall struct {
	tenantID uint64
	expertID string
	req      interfaces.InstantiateRequest
}

func (f *fakeTenantExpertInstantiator) Instantiate(
	_ context.Context, tenantID uint64, expertID string, req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, tenantExpertInstantiateCall{tenantID: tenantID, expertID: expertID, req: req})
	f.mu.Unlock()
	return &interfaces.InstantiateResult{
		Agent:           &types.CustomAgent{ID: "agent-copy", Name: req.AgentName},
		PendingSkills:   []string{"pdf-extract"},
		SkillInstallIDs: []string{},
	}, nil
}

// tenantExpertAgentForTest mirrors the whitelist fixture from the experts
// package tests: every exportable field plus the excluded classes.
func tenantExpertAgentForTest(id string) *types.CustomAgent {
	memory := true
	return &types.CustomAgent{
		ID:          id,
		TenantID:    7,
		Name:        "合同审查助手",
		Description: "审查合同条款并提示风险",
		Config: types.CustomAgentConfig{
			SystemPrompt:        "你是一名资深法务。",
			PersonaMBTI:         "INTJ",
			SkillsSelectionMode: "selected",
			SelectedSkills:      []string{"pdf-extract", "doc-render"},
			Subagents:           []string{"code-reviewer"},
			QuestionSuggestions: &types.QuestionSuggestionConfig{
				Starters: types.StarterSuggestionConfig{
					Enabled: true, Mode: types.SuggestionModeCurated,
					Items: []string{"帮我审查这份 NDA"}, Count: 1,
				},
			},
			// Excluded classes.
			KBSelectionMode:     "selected",
			KnowledgeBases:      []string{"kb-1"},
			ModelID:             "model-secret-1",
			SandboxConfigID:     "sbx-1",
			MemoryEnabled:       &memory,
			WebSearchProviderID: "wsp-1",
		},
	}
}

// newTenantExpertMarketForTest wires the service over temp roots.
func newTenantExpertMarketForTest(
	agents *fakeExpertAgentSource,
	published *fakePublishedExpertStore,
	installs *fakeExpertInstalls,
	instantiate *fakeTenantExpertInstantiator,
	names interfaces.TenantSkillPublisherNames,
) (*TenantExpertMarketService, string, string) {
	publishedRoot := tTempDir()
	marketRoot := tTempDir()
	svc := NewTenantExpertMarketService(agents, published, installs, instantiate, names, publishedRoot, marketRoot)
	return svc, publishedRoot, marketRoot
}

func tTempDir() string {
	dir, err := os.MkdirTemp("", "tenant-expert-market-*")
	if err != nil {
		panic(err)
	}
	return dir
}

// ---------------------------------------------------------------------------
// publish
// ---------------------------------------------------------------------------

func TestTenantExpertPublishWritesParseableSnapshotAndRow(t *testing.T) {
	agent := tenantExpertAgentForTest("0b9f6a1e-6c96-4a8e-b7a5-3f2d1c0a9b8d")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agent.ID): agent,
	}}
	svc, publishedRoot, _ := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, &fakeExpertInstalls{}, &fakeTenantExpertInstantiator{}, nil)

	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "user-1")
	view, err := svc.PublishAgentExpert(ctx, 7, agent.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, agent.ID, view.AgentID)
	require.Equal(t, "合同审查助手", view.Name)
	require.NotEmpty(t, view.ID)
	require.Equal(t, "user-1", view.PublishedBy, "publisher comes from the ctx user")
	require.False(t, view.PublishedAt.IsZero())

	// The snapshot lands in the tenant-scoped published root and parses with
	// the same scanner the catalog uses.
	expertID := experts.TenantExpertID(agent.ID)
	snapshotDir := experts.PublishedSnapshotDir(publishedRoot, 7, expertID)
	scanned, err := experts.ScanExperts(experts.PublishedTenantDir(publishedRoot, 7))
	require.NoError(t, err)
	require.Len(t, scanned, 1)
	require.Equal(t, expertID, scanned[0].Manifest.ID)
	require.FileExists(t, filepath.Join(snapshotDir, "manifest.yaml"))
	require.FileExists(t, filepath.Join(snapshotDir, "SOUL.md"))

	// The row records the snapshot pointer and its digest, and the digest
	// matches a rebuild of the scanned tree.
	row, err := svc.published.GetByTenantAndAgent(context.Background(), 7, agent.ID)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, snapshotDir, row.SnapshotRef)
	require.NotEmpty(t, row.SnapshotSHA256)
	rebuilt := &experts.MaterializedExpert{Manifest: scanned[0].Manifest, PersonaFiles: scanned[0].PersonaFiles}
	require.Equal(t, row.SnapshotSHA256, rebuilt.SnapshotSHA256())

	// Re-publishing under a new name refreshes the SAME row.
	agent2 := tenantExpertAgentForTest(agent.ID)
	agent2.Name = "改名后的助手"
	agents.agents[publishedExpertKey(7, agent.ID)] = agent2
	view2, err := svc.PublishAgentExpert(context.Background(), 7, agent.ID, "user-2",
		interfaces.PublishAgentExpertRequest{Name: "市场名"})
	require.NoError(t, err)
	require.Equal(t, view.ID, view2.ID, "re-publish must reuse the publish row")
	require.Equal(t, "市场名", view2.Name)
}

func TestTenantExpertPublishUnknownAgentIs404(t *testing.T) {
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(9, "agent-9"): tenantExpertAgentForTest("agent-9"),
	}}
	published := &fakePublishedExpertStore{}
	svc, _, _ := newTenantExpertMarketForTest(agents, published, &fakeExpertInstalls{}, &fakeTenantExpertInstantiator{}, nil)

	_, err := svc.PublishAgentExpert(context.Background(), 7, "missing", "user-1", interfaces.PublishAgentExpertRequest{})
	requireAppErrorStatus(t, err, 404)

	// agent-9 exists — but in tenant 9, so tenant 7 must read it as absent.
	_, err = svc.PublishAgentExpert(context.Background(), 7, "agent-9", "user-1", interfaces.PublishAgentExpertRequest{})
	requireAppErrorStatus(t, err, 404)
	require.Empty(t, published.rows, "no publish row may be written for another tenant's agent")
}

// TestTenantExpertPublishExcludesNonWhitelistedConfig is the binding M4 §5
// assertion: an agent WITH KB bindings, model keys, a sandbox binding and
// memory exports NONE of them into the snapshot.
func TestTenantExpertPublishExcludesNonWhitelistedConfig(t *testing.T) {
	agent := tenantExpertAgentForTest("aaaabbbb-cccc-dddd-eeee-ffff00001111")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agent.ID): agent,
	}}
	svc, publishedRoot, _ := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, &fakeExpertInstalls{}, &fakeTenantExpertInstantiator{}, nil)

	_, err := svc.PublishAgentExpert(context.Background(), 7, agent.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)

	snapshotDir := experts.PublishedSnapshotDir(publishedRoot, 7, experts.TenantExpertID(agent.ID))
	manifestYAML, err := os.ReadFile(filepath.Join(snapshotDir, "manifest.yaml"))
	require.NoError(t, err)
	soul, err := os.ReadFile(filepath.Join(snapshotDir, "SOUL.md"))
	require.NoError(t, err)

	for _, secret := range []string{"kb-1", "model-secret-1", "sbx-1", "wsp-1", "knowledge_bases", "model_id", "sandbox_config_id", "memory"} {
		require.NotContains(t, string(manifestYAML), secret, "excluded config leaked into manifest.yaml")
		require.NotContains(t, string(soul), secret, "excluded config leaked into SOUL.md")
	}
	// The whitelisted exports DID land.
	require.Contains(t, string(manifestYAML), "pdf-extract")
	require.Contains(t, string(manifestYAML), "code-reviewer")
	require.Contains(t, string(manifestYAML), "帮我审查这份 NDA")
	require.Contains(t, string(soul), "你是一名资深法务。")
}

// ---------------------------------------------------------------------------
// list / unpublish
// ---------------------------------------------------------------------------

func TestTenantExpertListJoinsPublisherAndInstalledState(t *testing.T) {
	agentA := tenantExpertAgentForTest("11111111-2222-3333-4444-555555555555")
	agentB := tenantExpertAgentForTest("66666666-7777-8888-9999-000000000000")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agentA.ID): agentA,
		publishedExpertKey(7, agentB.ID): agentB,
	}}
	installs := &fakeExpertInstalls{}
	names := &fakePublisherNames{users: map[string]*types.User{
		"user-1": {ID: "user-1", Username: "alice"},
	}}
	svc, _, _ := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, installs, &fakeTenantExpertInstantiator{}, names)

	viewA, err := svc.PublishAgentExpert(context.Background(), 7, agentA.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)
	viewB, err := svc.PublishAgentExpert(context.Background(), 7, agentB.ID, "user-2", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)

	// agentA's expert is installed (ledger row with its manifest slug).
	require.NoError(t, installs.Upsert(context.Background(), &types.ExpertInstallEntity{
		ID: "ei-1", TenantID: 7, Slug: experts.TenantExpertID(agentA.ID),
		StorageRef: "/x", SnapshotSHA256: "sha",
	}))

	index, err := svc.ListPublishedExperts(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, index.Experts, 2)
	byID := map[string]interfaces.PublishedExpertEntry{}
	for _, entry := range index.Experts {
		byID[entry.ID] = entry
	}
	entryA := byID[viewA.ID]
	require.Equal(t, "合同审查助手", entryA.Name)
	require.Equal(t, "alice", entryA.PublisherName)
	require.True(t, entryA.Installed, "an expert_installs row on the manifest slug marks the expert installed")
	require.False(t, entryA.CreatedAt.IsZero())
	entryB := byID[viewB.ID]
	require.Equal(t, "user-2", entryB.PublisherName, "unresolved publisher ids degrade to the raw id")
	require.False(t, entryB.Installed)

	// Another tenant sees nothing.
	empty, err := svc.ListPublishedExperts(context.Background(), 9)
	require.NoError(t, err)
	require.Empty(t, empty.Experts)
}

func TestTenantExpertUnpublishSoftDeletes(t *testing.T) {
	agent := tenantExpertAgentForTest("12345678-1234-1234-1234-123456789012")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agent.ID): agent,
	}}
	svc, _, _ := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, &fakeExpertInstalls{}, &fakeTenantExpertInstantiator{}, nil)

	view, err := svc.PublishAgentExpert(context.Background(), 7, agent.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)

	require.NoError(t, svc.UnpublishExpert(context.Background(), 7, view.ID))
	index, err := svc.ListPublishedExperts(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, index.Experts)

	// Unpublishing again is a 404 (the row reads as gone), and so is a
	// foreign tenant's id.
	requireAppErrorStatus(t, svc.UnpublishExpert(context.Background(), 7, view.ID), 404)
	requireAppErrorStatus(t, svc.UnpublishExpert(context.Background(), 9, view.ID), 404)
}

// ---------------------------------------------------------------------------
// install
// ---------------------------------------------------------------------------

func TestTenantExpertInstallSeedsSnapshotAndInstantiates(t *testing.T) {
	agent := tenantExpertAgentForTest("abcdefab-cdef-abcd-efab-cdefabcdefab")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agent.ID): agent,
	}}
	installs := &fakeExpertInstalls{}
	instantiate := &fakeTenantExpertInstantiator{}
	svc, _, marketRoot := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, installs, instantiate, nil)

	view, err := svc.PublishAgentExpert(context.Background(), 7, agent.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)

	ctx := context.WithValue(context.Background(), types.UserIDContextKey, "user-9")
	result, err := svc.InstallPublishedExpert(ctx, 7, view.ID, interfaces.InstantiateRequest{AgentName: "副本"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "副本", result.Agent.Name)

	// The snapshot was copied into the tenant's installed-experts root —
	// the same tree the M2 ExpertSource scans.
	expertID := experts.TenantExpertID(agent.ID)
	installDir := experts.MarketInstallDir(marketRoot, 7, expertID)
	scanned, err := experts.ScanExperts(experts.MarketTenantDir(marketRoot, 7))
	require.NoError(t, err)
	require.Len(t, scanned, 1)
	require.Equal(t, expertID, scanned[0].Manifest.ID)
	require.FileExists(t, filepath.Join(installDir, "manifest.yaml"))

	// The install ledger row ties the install back to the published expert.
	row, err := installs.GetByTenantAndSlug(ctx, 7, expertID)
	require.NoError(t, err)
	require.NotNil(t, row, "an expert_installs row on the manifest slug records the install")
	require.Equal(t, installDir, row.StorageRef)
	require.Equal(t, "user-9", row.CreatedBy)
	require.Equal(t, view.SnapshotSHA256, row.SnapshotSHA256, "the ledger digest matches the published snapshot")

	// Instantiate ran against the seeded expert with the caller's request.
	require.Len(t, instantiate.calls, 1)
	require.Equal(t, uint64(7), instantiate.calls[0].tenantID)
	require.Equal(t, expertID, instantiate.calls[0].expertID)
	require.Equal(t, "副本", instantiate.calls[0].req.AgentName)

	// The listing now flags the expert installed.
	index, err := svc.ListPublishedExperts(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, index.Experts[0].Installed)
}

func TestTenantExpertInstallGatesOnThePublishRow(t *testing.T) {
	agent := tenantExpertAgentForTest("fedcbafe-dcba-fedc-bafe-dcbafebabcba")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agent.ID): agent,
	}}
	installs := &fakeExpertInstalls{}
	instantiate := &fakeTenantExpertInstantiator{}
	svc, _, _ := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, installs, instantiate, nil)

	view, err := svc.PublishAgentExpert(context.Background(), 7, agent.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)

	// Unknown and cross-tenant ids read as absent (the wrong-tenant 404).
	_, err = svc.InstallPublishedExpert(context.Background(), 7, "missing", interfaces.InstantiateRequest{})
	requireAppErrorStatus(t, err, 404)
	_, err = svc.InstallPublishedExpert(context.Background(), 9, view.ID, interfaces.InstantiateRequest{})
	requireAppErrorStatus(t, err, 404)

	// Unpublishing closes the install surface for the same id.
	require.NoError(t, svc.UnpublishExpert(context.Background(), 7, view.ID))
	_, err = svc.InstallPublishedExpert(context.Background(), 7, view.ID, interfaces.InstantiateRequest{})
	requireAppErrorStatus(t, err, 404)
	require.Empty(t, instantiate.calls)
	require.Empty(t, installs.rows)
}

func TestTenantExpertInstallRejectsTamperedSnapshots(t *testing.T) {
	agent := tenantExpertAgentForTest("eeddccbb-aabb-0011-2233-445566778899")
	agents := &fakeExpertAgentSource{agents: map[string]*types.CustomAgent{
		publishedExpertKey(7, agent.ID): agent,
	}}
	svc, publishedRoot, marketRoot := newTenantExpertMarketForTest(agents, &fakePublishedExpertStore{}, &fakeExpertInstalls{}, &fakeTenantExpertInstantiator{}, nil)

	view, err := svc.PublishAgentExpert(context.Background(), 7, agent.ID, "user-1", interfaces.PublishAgentExpertRequest{})
	require.NoError(t, err)

	// Rewrite the persona document: the tree no longer matches the digest
	// the row recorded, so the immutable-snapshot contract fails the install.
	soul := filepath.Join(experts.PublishedSnapshotDir(publishedRoot, 7, experts.TenantExpertID(agent.ID)), "SOUL.md")
	require.NoError(t, os.WriteFile(soul, []byte("# tampered\n\nevil"), 0o644))
	_, err = svc.InstallPublishedExpert(context.Background(), 7, view.ID, interfaces.InstantiateRequest{})
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, 500, appErr.HTTPCode)

	// A deleted snapshot is equally fatal — and neither case seeded an
	// install directory.
	require.NoError(t, os.RemoveAll(experts.PublishedTenantDir(publishedRoot, 7)))
	_, err = svc.InstallPublishedExpert(context.Background(), 7, view.ID, interfaces.InstantiateRequest{})
	require.Error(t, err)
	require.NoError(t, os.MkdirAll(experts.MarketTenantDir(marketRoot, 7), 0o755))
	entries, err := os.ReadDir(experts.MarketTenantDir(marketRoot, 7))
	require.NoError(t, err)
	require.Empty(t, entries, "a failed install must leave the installed-experts root untouched")
}

// The service's own clock override keeps the idempotent re-publish observable
// without sleeping (the tenant skill market precedent).
func (s *TenantExpertMarketService) withExpertClock(now func() time.Time) *TenantExpertMarketService {
	s.now = now
	return s
}
