package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/experts"
	"github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/skills/skillhub"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// fakeMarketClient is the skillhub.Client seam: scripted answers plus a
// recording of every Download call and an optional per-slug gate that holds a
// download until the test releases it (install-serialization probes).
// Search/Rankings return (value, err) together so the cached wrapper's stale
// contract — last good value alongside the ErrStaleOnly marker — is
// expressible.
type fakeMarketClient struct {
	mu        sync.Mutex
	search    []skillhub.SkillSummary
	searchErr error
	rankings  map[string][]skillhub.SkillSummary
	rankErr   error
	packages  map[string][]byte
	downErr   map[string]error
	downloads []string
	started   int
	gate      map[string]chan struct{}
}

func (f *fakeMarketClient) Search(_ context.Context, _ string, _ int) ([]skillhub.SkillSummary, error) {
	return f.search, f.searchErr
}

func (f *fakeMarketClient) Rankings(_ context.Context, kind string) ([]skillhub.SkillSummary, error) {
	if results, ok := f.rankings[kind]; ok {
		return results, f.rankErr
	}
	return nil, fmt.Errorf("%w: %q", skillhub.ErrUnsupportedRanking, kind)
}

func (f *fakeMarketClient) Download(_ context.Context, slug string) ([]byte, error) {
	f.mu.Lock()
	f.started++
	f.mu.Unlock()
	if gate := f.gate[slug]; gate != nil {
		<-gate
	}
	f.mu.Lock()
	f.downloads = append(f.downloads, slug)
	f.mu.Unlock()
	if err := f.downErr[slug]; err != nil {
		return nil, err
	}
	payload, ok := f.packages[slug]
	if !ok {
		return nil, fmt.Errorf("%w: HTTP 404", skillhub.ErrMarket)
	}
	return payload, nil
}

func (f *fakeMarketClient) downloadLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.downloads...)
}

func (f *fakeMarketClient) startedDownloads() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

// fakeMarketSkillsets is the skillhub.SkillsetClient seam.
type fakeMarketSkillsets struct {
	index    []skillhub.SkillsetSummary
	indexErr error
	detail   map[string]skillhub.SkillsetSummary
	detailFn func(slug string) (skillhub.SkillsetSummary, error)
}

func (f *fakeMarketSkillsets) SkillsetIndex(context.Context) ([]skillhub.SkillsetSummary, error) {
	return f.index, f.indexErr
}

func (f *fakeMarketSkillsets) SkillsetDetail(_ context.Context, slug string) (skillhub.SkillsetSummary, error) {
	// Mirror the real client: the slug is validated before any lookup.
	if _, err := skillhub.ValidateSkillsetSlug(slug); err != nil {
		return skillhub.SkillsetSummary{}, err
	}
	if f.detailFn != nil {
		return f.detailFn(slug)
	}
	summary, ok := f.detail[slug]
	if !ok {
		return skillhub.SkillsetSummary{}, fmt.Errorf("%w: %q", skillhub.ErrSkillsetNotFound, slug)
	}
	return summary, nil
}

// fakeMarketCatalog is the tenant-catalog seam (Register + InstallToConfigs).
type fakeMarketCatalog struct {
	mu                 sync.Mutex
	registeredArchives [][]byte
	registerNames      []string
	registerErr        error
	catalogID          string
	installCalls       []installCall
	installResult      *CatalogInstallResult
	installErr         error
}

type installCall struct {
	catalogID string
	configIDs []string
}

func (f *fakeMarketCatalog) RegisterCatalogFromArchive(
	_ context.Context, _ uint64, archive []byte,
) (*types.TenantSkillCatalogEntity, error) {
	bundle, err := ParseSkillBundle(archive)
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.registeredArchives = append(f.registeredArchives, archive)
	f.registerNames = append(f.registerNames, bundle.Name)
	f.mu.Unlock()
	id := f.catalogID
	if id == "" {
		id = "cat-" + bundle.Name
	}
	if f.registerErr != nil {
		return nil, f.registerErr
	}
	return &types.TenantSkillCatalogEntity{ID: id, Name: bundle.Name}, nil
}

func (f *fakeMarketCatalog) InstallCatalogToConfigs(
	_ context.Context, _ uint64, catalogID string, configIDs []string,
) (*CatalogInstallResult, error) {
	f.mu.Lock()
	f.installCalls = append(f.installCalls, installCall{catalogID: catalogID, configIDs: append([]string(nil), configIDs...)})
	f.mu.Unlock()
	if f.installErr != nil {
		return nil, f.installErr
	}
	if f.installResult != nil {
		return f.installResult, nil
	}
	installs := make(map[string]string, len(configIDs))
	for _, id := range configIDs {
		installs[id] = "skill-for-" + id
	}
	return &CatalogInstallResult{Installs: installs}, nil
}

// fakeMarketInstantiator is the M2 ExpertService.Instantiate seam.
type fakeMarketInstantiator struct {
	mu     sync.Mutex
	calls  []marketInstantiateCall
	answer func(call marketInstantiateCall) (*interfaces.InstantiateResult, error)
}

type marketInstantiateCall struct {
	tenantID uint64
	expertID string
	req      interfaces.InstantiateRequest
}

func (f *fakeMarketInstantiator) Instantiate(
	_ context.Context, tenantID uint64, expertID string, req interfaces.InstantiateRequest,
) (*interfaces.InstantiateResult, error) {
	call := marketInstantiateCall{tenantID: tenantID, expertID: expertID, req: req}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()
	if f.answer != nil {
		return f.answer(call)
	}
	return &interfaces.InstantiateResult{
		Agent:           &types.CustomAgent{ID: "agent-1", Name: "materialized"},
		PendingSkills:   []string{"pdf-extract"},
		SkillInstallIDs: []string{"inst-1"},
	}, nil
}

// fakeExpertInstallStore is the repository.ExpertInstallRepository seam.
type fakeExpertInstallStore struct {
	mu        sync.Mutex
	rows      map[string]*types.ExpertInstallEntity // key: slug
	listErr   error
	upsertErr error
}

func (f *fakeExpertInstallStore) Upsert(_ context.Context, e *types.ExpertInstallEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.rows == nil {
		f.rows = map[string]*types.ExpertInstallEntity{}
	}
	f.rows[e.Slug] = e
	return nil
}

func (f *fakeExpertInstallStore) ListByTenant(context.Context, uint64) ([]types.ExpertInstallEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]types.ExpertInstallEntity, 0, len(f.rows))
	for _, row := range f.rows {
		out = append(out, *row)
	}
	return out, nil
}

func (f *fakeExpertInstallStore) GetByTenantAndSlug(_ context.Context, _ uint64, slug string) (*types.ExpertInstallEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.rows[slug]
	if row == nil {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

func (f *fakeExpertInstallStore) Delete(context.Context, uint64, string) error { return nil }

// ---------------------------------------------------------------------------
// fixtures
// ---------------------------------------------------------------------------

// marketSkillZip builds a package zip whose payload lives under dir (an empty
// dir means root-level files), the way the registry wraps skills.
func marketSkillZip(t *testing.T, dir, skillName string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	manifest := fmt.Sprintf("---\nname: %s\ndescription: market skill %s\n---\n\n# %s\n", skillName, skillName, skillName)
	entries := map[string]string{
		"SKILL.md":  manifest,
		"helper.py": "print('hi')\n",
	}
	for name, content := range entries {
		path := name
		if dir != "" {
			path = dir + "/" + name
		}
		entry, err := writer.Create(path)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return buf.Bytes()
}

func newTestMarketService(
	client skillhub.Client,
	skillsets skillhub.SkillsetClient,
	catalog SkillMarketCatalog,
	instantiate MarketExpertInstantiator,
	installs *fakeExpertInstallStore,
	dataRoot string,
) *SkillMarketService {
	if installs == nil {
		installs = &fakeExpertInstallStore{}
	}
	return NewSkillMarketService(client, skillsets, catalog, instantiate, installs, dataRoot)
}

func requireAppErrorStatus(t *testing.T, err error, wantHTTP int) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.AppError
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, wantHTTP, appErr.HTTPCode, "error: %v", err)
}

// ---------------------------------------------------------------------------
// search / rankings
// ---------------------------------------------------------------------------

func TestSearchMarketSkillsServesFreshResults(t *testing.T) {
	client := &fakeMarketClient{search: []skillhub.SkillSummary{{
		Slug: "pdf", Name: "PDF", Description: "pdf tools", Version: "1.2",
		Raw: map[string]string{"slug": "pdf"},
	}}}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	listing, err := svc.SearchMarketSkills(context.Background(), "pdf", 10)
	require.NoError(t, err)
	require.False(t, listing.Stale)
	require.Len(t, listing.Results, 1)
	require.Equal(t, "pdf", listing.Results[0].Slug)
	require.Equal(t, "1.2", listing.Results[0].Version)
}

func TestSearchMarketSkillsServesStaleWithFlag(t *testing.T) {
	// The cached wrapper's stale contract: (value, ErrStaleOnly-wrapped error).
	client := &fakeMarketClient{search: []skillhub.SkillSummary{{Slug: "pdf", Name: "PDF"}}}
	client.searchErr = fmt.Errorf("%w: upstream 500", skillhub.ErrStaleOnly)
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	listing, err := svc.SearchMarketSkills(context.Background(), "pdf", 10)
	require.NoError(t, err, "stale-but-cached answers 200 with the flag, not an error")
	require.True(t, listing.Stale)
	require.Len(t, listing.Results, 1)
}

func TestSearchMarketSkillsUnreachableWithoutCacheIs503(t *testing.T) {
	client := &fakeMarketClient{}
	client.searchErr = fmt.Errorf("%w: connection refused", skillhub.ErrUnreachable)
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	_, err := svc.SearchMarketSkills(context.Background(), "pdf", 10)
	requireAppErrorStatus(t, err, http.StatusServiceUnavailable)
}

func TestMarketSkillRankingsValidatesKind(t *testing.T) {
	svc := newTestMarketService(&fakeMarketClient{}, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	_, err := svc.MarketSkillRankings(context.Background(), "bogus")
	requireAppErrorStatus(t, err, http.StatusBadRequest)

	client := &fakeMarketClient{rankings: map[string][]skillhub.SkillSummary{
		"hot": {{Slug: "pdf", Name: "PDF"}},
	}}
	svc = newTestMarketService(client, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())
	listing, err := svc.MarketSkillRankings(context.Background(), "hot")
	require.NoError(t, err)
	require.False(t, listing.Stale)
	require.Equal(t, "pdf", listing.Results[0].Slug)
}

// ---------------------------------------------------------------------------
// skill install
// ---------------------------------------------------------------------------

func TestInstallMarketSkillDownloadsParsesAndRepacks(t *testing.T) {
	// The package arrives wrapped in a top-level directory; the catalog
	// registration must receive an archive ParseSkillBundle accepts (the
	// repack decision the hand-off flagged as job one).
	client := &fakeMarketClient{packages: map[string][]byte{
		"pdf-extract": marketSkillZip(t, "pdf-extract", "pdf-extract"),
	}}
	catalog := &fakeMarketCatalog{installResult: &CatalogInstallResult{
		Installs: map[string]string{"cfg-1": "sk-1", "cfg-2": "sk-2"},
	}}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, catalog, nil, nil, t.TempDir())

	result, err := svc.InstallMarketSkill(context.Background(), 7, "pdf-extract", []string{"cfg-1", "cfg-2"})
	require.NoError(t, err)
	require.Equal(t, "cat-pdf-extract", result.CatalogID)
	require.Equal(t, []string{"sk-1", "sk-2"}, result.InstallIDs, "install ids follow the requested config order")
	require.Empty(t, result.Errors)

	require.Len(t, catalog.registeredArchives, 1)
	bundle, err := ParseSkillBundle(catalog.registeredArchives[0])
	require.NoError(t, err, "the repacked archive must satisfy RegisterCatalogFromArchive's parser")
	require.Equal(t, "pdf-extract", bundle.Name)
	require.Equal(t, "market skill pdf-extract", bundle.Description)
	_, ok := bundle.Files["helper.py"]
	require.True(t, ok, "payload members survive the repack")
	_, ok = bundle.Files["pdf-extract/SKILL.md"]
	require.False(t, ok, "the wrapper directory is re-rooted away")
}

func TestInstallMarketSkillRejectsInvalidPackageAs400(t *testing.T) {
	client := &fakeMarketClient{packages: map[string][]byte{"broken": []byte("not a zip")}}
	catalog := &fakeMarketCatalog{}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, catalog, nil, nil, t.TempDir())

	_, err := svc.InstallMarketSkill(context.Background(), 7, "broken", []string{"cfg-1"})
	requireAppErrorStatus(t, err, http.StatusBadRequest)
	require.Empty(t, catalog.registeredArchives)
}

func TestInstallMarketSkillUnreachableIs503(t *testing.T) {
	client := &fakeMarketClient{downErr: map[string]error{
		"gone": fmt.Errorf("%w: HTTP 404", skillhub.ErrMarket),
	}}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	_, err := svc.InstallMarketSkill(context.Background(), 7, "gone", []string{"cfg-1"})
	requireAppErrorStatus(t, err, http.StatusServiceUnavailable)
}

func TestInstallMarketSkillValidatesInput(t *testing.T) {
	svc := newTestMarketService(&fakeMarketClient{}, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	_, err := svc.InstallMarketSkill(context.Background(), 7, "  ", []string{"cfg-1"})
	requireAppErrorStatus(t, err, http.StatusBadRequest)

	_, err = svc.InstallMarketSkill(context.Background(), 7, "pdf", nil)
	requireAppErrorStatus(t, err, http.StatusBadRequest)
}

func TestInstallMarketSkillMirrorsPartialInstallErrors(t *testing.T) {
	client := &fakeMarketClient{packages: map[string][]byte{
		"pdf-extract": marketSkillZip(t, "", "pdf-extract"),
	}}
	catalog := &fakeMarketCatalog{installResult: &CatalogInstallResult{
		Installs: map[string]string{"cfg-1": "sk-1"},
		Errors:   map[string]string{"cfg-2": "sandbox config not found"},
	}}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, catalog, nil, nil, t.TempDir())

	result, err := svc.InstallMarketSkill(context.Background(), 7, "pdf-extract", []string{"cfg-1", "cfg-2"})
	require.NoError(t, err, "partial success is a 202 answer with per-config errors")
	require.Equal(t, "cat-pdf-extract", result.CatalogID)
	require.Equal(t, []string{"sk-1"}, result.InstallIDs)
	require.Equal(t, map[string]string{"cfg-2": "sandbox config not found"}, result.Errors)
}

func TestInstallMarketSkillSerializesSameSlug(t *testing.T) {
	release := make(chan struct{})
	client := &fakeMarketClient{
		packages: map[string][]byte{"pdf-extract": marketSkillZip(t, "", "pdf-extract")},
		gate:     map[string]chan struct{}{"pdf-extract": release},
	}
	catalog := &fakeMarketCatalog{}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, catalog, nil, nil, t.TempDir())

	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.InstallMarketSkill(context.Background(), 7, "pdf-extract", []string{"cfg-1"})
			done <- err
		}()
	}

	// The first install holds the gate (it has started its download); the
	// second must NOT have started one — the per-slug lock serializes it.
	require.Eventually(t, func() bool {
		return client.startedDownloads() == 1
	}, 2*time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 1, client.startedDownloads(), "the second install waits on the per-slug lock before downloading")

	close(release)
	for i := 0; i < 2; i++ {
		require.NoError(t, <-done)
	}
	require.Len(t, client.downloadLog(), 2, "both installs ran after serialization")
	require.Len(t, catalog.registeredArchives, 2)
}

func TestInstallMarketSkillDifferentSlugsRunConcurrently(t *testing.T) {
	client := &fakeMarketClient{packages: map[string][]byte{
		"skill-a": marketSkillZip(t, "", "skill-a"),
		"skill-b": marketSkillZip(t, "", "skill-b"),
	}}
	svc := newTestMarketService(client, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	var wg sync.WaitGroup
	for _, slug := range []string{"skill-a", "skill-b"} {
		wg.Add(1)
		go func(slug string) {
			defer wg.Done()
			_, err := svc.InstallMarketSkill(context.Background(), 7, slug, []string{"cfg-1"})
			require.NoError(t, err)
		}(slug)
	}
	wg.Wait()
	require.Len(t, client.downloadLog(), 2)
}

// ---------------------------------------------------------------------------
// skillset index / detail
// ---------------------------------------------------------------------------

func TestListMarketSkillsetsServesIndexWithInstalledFlags(t *testing.T) {
	skillsets := &fakeMarketSkillsets{index: []skillhub.SkillsetSummary{
		{Slug: "pdf-tools", Name: "PDF 工具箱", SkillSlugs: []string{"pdf-extract"}},
		{Slug: "web-tools", Name: "Web 工具箱", SkillSlugs: []string{"web-fetch"}},
	}}
	installs := &fakeExpertInstallStore{rows: map[string]*types.ExpertInstallEntity{
		"pdf-tools": {Slug: "pdf-tools"},
	}}
	svc := newTestMarketService(&fakeMarketClient{}, skillsets, &fakeMarketCatalog{}, nil, installs, t.TempDir())

	index, err := svc.ListMarketSkillsets(context.Background(), 7)
	require.NoError(t, err)
	require.False(t, index.Stale)
	require.Len(t, index.Skillsets, 2)
	require.True(t, index.Skillsets[0].Installed)
	require.False(t, index.Skillsets[1].Installed)
	require.Equal(t, []string{"pdf-extract"}, index.Skillsets[0].SkillSlugs)
}

func TestListMarketSkillsetsStaleServesFlag(t *testing.T) {
	skillsets := &fakeMarketSkillsets{index: []skillhub.SkillsetSummary{{Slug: "pdf-tools", Name: "PDF"}}}
	skillsets.indexErr = fmt.Errorf("%w: timeout", skillhub.ErrStaleOnly)
	svc := newTestMarketService(&fakeMarketClient{}, skillsets, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	index, err := svc.ListMarketSkillsets(context.Background(), 7)
	require.NoError(t, err)
	require.True(t, index.Stale)
	require.Len(t, index.Skillsets, 1)
}

func TestGetMarketSkillsetServesDetail(t *testing.T) {
	skillsets := &fakeMarketSkillsets{detail: map[string]skillhub.SkillsetSummary{
		"pdf-tools": {
			Slug: "pdf-tools", Name: "PDF 工具箱", SkillSlugs: []string{"pdf-extract", "pdf-merge"},
			Raw: map[string]any{"displayNameEn": "PDF Toolkit", "summaryEn": "English summary"},
		},
	}}
	installs := &fakeExpertInstallStore{rows: map[string]*types.ExpertInstallEntity{
		"pdf-tools": {Slug: "pdf-tools"},
	}}
	svc := newTestMarketService(&fakeMarketClient{}, skillsets, &fakeMarketCatalog{}, nil, installs, t.TempDir())

	detail, err := svc.GetMarketSkillset(context.Background(), 7, "pdf-tools")
	require.NoError(t, err)
	require.Equal(t, "pdf-tools", detail.Slug)
	require.Equal(t, "PDF Toolkit", detail.NameEn)
	require.Equal(t, "English summary", detail.DescriptionEn)
	require.True(t, detail.Installed)
}

func TestGetMarketSkillsetNotFoundIs404(t *testing.T) {
	svc := newTestMarketService(&fakeMarketClient{}, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	_, err := svc.GetMarketSkillset(context.Background(), 7, "nope")
	requireAppErrorStatus(t, err, http.StatusNotFound)
}

func TestGetMarketSkillsetInvalidSlugIs400(t *testing.T) {
	svc := newTestMarketService(&fakeMarketClient{}, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, nil, nil, t.TempDir())

	_, err := svc.GetMarketSkillset(context.Background(), 7, "../escape")
	requireAppErrorStatus(t, err, http.StatusBadRequest)
}

// ---------------------------------------------------------------------------
// skillset (expert) install
// ---------------------------------------------------------------------------

func TestInstallMarketSkillsetMaterializesAndInstantiates(t *testing.T) {
	skillsets := &fakeMarketSkillsets{detail: map[string]skillhub.SkillsetSummary{
		"pdf-tools": {
			Slug: "pdf-tools", Name: "PDF 工具箱",
			SkillSlugs: []string{"pdf-extract", "pdf-merge"},
		},
	}}
	client := &fakeMarketClient{packages: map[string][]byte{
		"pdf-extract": marketSkillZip(t, "", "pdf-extract"),
		"pdf-merge":   marketSkillZip(t, "wrap", "pdf-merge"),
	}}
	installs := &fakeExpertInstallStore{}
	instantiate := &fakeMarketInstantiator{}
	dataRoot := t.TempDir()
	svc := newTestMarketService(client, skillsets, &fakeMarketCatalog{}, instantiate, installs, dataRoot)

	result, err := svc.InstallMarketSkillset(tenantCtx(7), 7, "pdf-tools", interfaces.InstantiateRequest{
		AgentName:       "我的 PDF 专家",
		SandboxConfigID: "cfg-1",
	})
	require.NoError(t, err)
	require.Equal(t, "agent-1", result.Agent.ID)
	require.Equal(t, "skillhub-skillset-pdf-tools", result.ExpertID)
	require.NotEmpty(t, result.SnapshotSHA256)

	// The instantiator saw the materialized expert's manifest ID with the
	// caller's request verbatim.
	require.Len(t, instantiate.calls, 1)
	require.Equal(t, uint64(7), instantiate.calls[0].tenantID)
	require.Equal(t, "skillhub-skillset-pdf-tools", instantiate.calls[0].expertID)
	require.Equal(t, "我的 PDF 专家", instantiate.calls[0].req.AgentName)
	require.Equal(t, "cfg-1", instantiate.calls[0].req.SandboxConfigID)

	// The ledger row records slug + storage ref + the materialized digest.
	row := installs.rows["pdf-tools"]
	require.NotNil(t, row)
	require.Equal(t, uint64(7), row.TenantID)
	require.Equal(t, experts.MarketInstallDir(dataRoot, 7, "pdf-tools"), row.StorageRef)
	require.Equal(t, result.SnapshotSHA256, row.SnapshotSHA256)

	// Composition evidence: the tree the catalog merge scans now carries the
	// expert Instantiate looked up (InstalledExperts is the exact path
	// MarketExpertSource serves GetExpert from).
	installed := experts.InstalledExperts(dataRoot, 7)
	require.Len(t, installed, 1)
	require.Equal(t, "skillhub-skillset-pdf-tools", installed[0].Manifest.ID)
	require.Equal(t, []string{"pdf-extract", "pdf-merge"}, installed[0].Manifest.Skills)
}

func TestInstallMarketSkillsetInstallsPerSkillPackages(t *testing.T) {
	skillsets := &fakeMarketSkillsets{detail: map[string]skillhub.SkillsetSummary{
		"pdf-tools": {Slug: "pdf-tools", Name: "PDF", SkillSlugs: []string{"pdf-extract"}},
	}}
	client := &fakeMarketClient{packages: map[string][]byte{
		"pdf-extract": marketSkillZip(t, "", "pdf-extract"),
	}}
	svc := newTestMarketService(client, skillsets, &fakeMarketCatalog{}, &fakeMarketInstantiator{}, nil, t.TempDir())

	_, err := svc.InstallMarketSkillset(context.Background(), 7, "pdf-tools", interfaces.InstantiateRequest{})
	require.NoError(t, err)
	require.Equal(t, []string{"pdf-extract"}, client.downloadLog())
}

func TestInstallMarketSkillsetUnknownSlugIs404(t *testing.T) {
	svc := newTestMarketService(&fakeMarketClient{}, &fakeMarketSkillsets{}, &fakeMarketCatalog{}, &fakeMarketInstantiator{}, nil, t.TempDir())

	_, err := svc.InstallMarketSkillset(context.Background(), 7, "nope", interfaces.InstantiateRequest{})
	requireAppErrorStatus(t, err, http.StatusNotFound)
}

func TestInstallMarketSkillsetFailingSkillDownloadPropagates(t *testing.T) {
	skillsets := &fakeMarketSkillsets{detail: map[string]skillhub.SkillsetSummary{
		"pdf-tools": {Slug: "pdf-tools", Name: "PDF", SkillSlugs: []string{"gone"}},
	}}
	client := &fakeMarketClient{downErr: map[string]error{
		"gone": fmt.Errorf("%w: HTTP 500", skillhub.ErrMarket),
	}}
	instantiate := &fakeMarketInstantiator{}
	svc := newTestMarketService(client, skillsets, &fakeMarketCatalog{}, instantiate, nil, t.TempDir())

	_, err := svc.InstallMarketSkillset(context.Background(), 7, "pdf-tools", interfaces.InstantiateRequest{})
	require.Error(t, err)
	require.Empty(t, instantiate.calls, "no agent is created when a bundled skill cannot be fetched")
}

func TestInstallMarketSkillsetSerializesSameSlug(t *testing.T) {
	skillsets := &fakeMarketSkillsets{detail: map[string]skillhub.SkillsetSummary{
		"pdf-tools": {Slug: "pdf-tools", Name: "PDF", SkillSlugs: []string{"pdf-extract"}},
	}}
	release := make(chan struct{})
	client := &fakeMarketClient{
		packages: map[string][]byte{"pdf-extract": marketSkillZip(t, "", "pdf-extract")},
		gate:     map[string]chan struct{}{"pdf-extract": release},
	}
	svc := newTestMarketService(client, skillsets, &fakeMarketCatalog{}, &fakeMarketInstantiator{}, nil, t.TempDir())

	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := svc.InstallMarketSkillset(context.Background(), 7, "pdf-tools", interfaces.InstantiateRequest{})
			done <- err
		}()
	}

	require.Eventually(t, func() bool {
		return client.startedDownloads() == 1
	}, 2*time.Second, 10*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, 1, client.startedDownloads(), "the second skillset install waits on the per-slug lock")

	close(release)
	for i := 0; i < 2; i++ {
		require.NoError(t, <-done)
	}
	require.Len(t, client.downloadLog(), 2)
}
