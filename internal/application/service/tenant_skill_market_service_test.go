package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// ---------------------------------------------------------------------------
// fakes
// ---------------------------------------------------------------------------

// fakePublishedStore is the repository.PublishedSkillRepository seam.
type fakePublishedStore struct {
	mu   sync.Mutex
	rows map[string]*types.PublishedSkillEntity // key: "<tenant>/<catalog>"
}

func publishedStoreKey(tenantID uint64, catalogID string) string {
	return fmt.Sprintf("%d/%s", tenantID, catalogID)
}

func (f *fakePublishedStore) Upsert(_ context.Context, e *types.PublishedSkillEntity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.rows == nil {
		f.rows = map[string]*types.PublishedSkillEntity{}
	}
	k := publishedStoreKey(e.TenantID, e.CatalogID)
	if existing, ok := f.rows[k]; ok {
		existing.PublishedBy = e.PublishedBy
		existing.UpdatedAt = e.UpdatedAt
		e = existing
		return nil
	}
	f.rows[k] = e
	return nil
}

func (f *fakePublishedStore) ListByTenant(_ context.Context, tenantID uint64) ([]types.PublishedSkillEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]types.PublishedSkillEntity, 0)
	for _, row := range f.rows {
		if row.TenantID == tenantID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (f *fakePublishedStore) GetByTenantAndCatalog(_ context.Context, tenantID uint64, catalogID string) (*types.PublishedSkillEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	row := f.rows[publishedStoreKey(tenantID, catalogID)]
	if row == nil {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

func (f *fakePublishedStore) Delete(_ context.Context, tenantID uint64, catalogID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.rows, publishedStoreKey(tenantID, catalogID))
	return nil
}

// fakeTenantCatalogStore is the catalog-read seam (the slice of
// repository.TenantSkillRepository the tenant market reads).
type fakeTenantCatalogStore struct {
	mu       sync.Mutex
	catalogs map[uint64]map[string]*types.TenantSkillCatalogEntity
	installs map[uint64][]*types.TenantSkillEntity
}

func (f *fakeTenantCatalogStore) GetCatalog(_ context.Context, tenantID uint64, catalogID string) (*types.TenantSkillCatalogEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	byID := f.catalogs[tenantID]
	if byID == nil {
		return nil, nil
	}
	row := byID[catalogID]
	if row == nil {
		return nil, nil
	}
	copy := *row
	return &copy, nil
}

func (f *fakeTenantCatalogStore) ListCatalogsByTenant(_ context.Context, tenantID uint64) ([]*types.TenantSkillCatalogEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	byID := f.catalogs[tenantID]
	out := make([]*types.TenantSkillCatalogEntity, 0, len(byID))
	for _, row := range byID {
		copy := *row
		out = append(out, &copy)
	}
	return out, nil
}

func (f *fakeTenantCatalogStore) ListSkillsByTenant(_ context.Context, tenantID uint64) ([]*types.TenantSkillEntity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*types.TenantSkillEntity(nil), f.installs[tenantID]...), nil
}

// fakeTenantInstaller is the InstallCatalogToConfigs seam (*TenantSkillService).
type fakeTenantInstaller struct {
	mu     sync.Mutex
	calls  []tenantInstallCall
	answer func(call tenantInstallCall) (*CatalogInstallResult, error)
}

type tenantInstallCall struct {
	tenantID  uint64
	catalogID string
	configIDs []string
}

func (f *fakeTenantInstaller) InstallCatalogToConfigs(
	_ context.Context, tenantID uint64, catalogID string, configIDs []string,
) (*CatalogInstallResult, error) {
	call := tenantInstallCall{tenantID: tenantID, catalogID: catalogID, configIDs: append([]string(nil), configIDs...)}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	answer := f.answer
	f.mu.Unlock()
	if answer != nil {
		return answer(call)
	}
	installs := make(map[string]string, len(configIDs))
	for _, id := range configIDs {
		installs[id] = "skill-for-" + id
	}
	return &CatalogInstallResult{Installs: installs}, nil
}

// fakePublisherNames is the GetUsersByIDs seam (interfaces.UserRepository).
type fakePublisherNames struct {
	users map[string]*types.User
	err   error
}

func (f *fakePublisherNames) GetUsersByIDs(_ context.Context, ids []string) (map[string]*types.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	out := make(map[string]*types.User, len(ids))
	for _, id := range ids {
		if u := f.users[id]; u != nil {
			out[id] = u
		}
	}
	return out, nil
}

// newTenantMarketForTest wires the service with a movable clock so the
// idempotent re-publish timestamp refresh is observable.
func newTenantMarketForTest(
	published *fakePublishedStore,
	store *fakeTenantCatalogStore,
	installer *fakeTenantInstaller,
	names interfaces.TenantSkillPublisherNames,
) (*TenantSkillMarketService, func(time.Time)) {
	current := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	svc := NewTenantSkillMarketService(published, store, installer, names).
		withClock(func() time.Time { return current })
	return svc, func(t time.Time) { current = t }
}

func tenantCatalog(tenantID uint64, id, name string) *types.TenantSkillCatalogEntity {
	return &types.TenantSkillCatalogEntity{
		ID: id, TenantID: tenantID, Name: name, Version: "1.0", Description: "skill " + name,
	}
}

// ---------------------------------------------------------------------------
// publish / unpublish
// ---------------------------------------------------------------------------

func TestTenantMarketPublishCreatesRowAndRefreshesOnRepublish(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-a": tenantCatalog(7, "cat-a", "pdf-extract")},
	}}
	svc, setClock := newTenantMarketForTest(published, store, &fakeTenantInstaller{}, nil)

	row, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)
	require.NotNil(t, row)
	require.Equal(t, "cat-a", row.CatalogID)
	require.Equal(t, "user-1", row.PublishedBy)
	firstStamp := row.PublishedAt
	require.False(t, firstStamp.IsZero())

	// Re-publish by another member of the same tenant: the same row comes
	// back with the new publisher and a refreshed timestamp. PublishedAt is
	// the row's created_at, so it staying fixed proves the slot was reused
	// rather than re-inserted.
	setClock(firstStamp.Add(time.Hour))
	row2, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-2")
	require.NoError(t, err)
	require.Equal(t, row.PublishedAt, row2.PublishedAt, "re-publish must reuse the publish row")
	require.Equal(t, "user-2", row2.PublishedBy)
	require.True(t, row2.UpdatedAt.After(row.UpdatedAt), "re-publish must refresh the timestamp")

	// The published skill is now visible in the listing.
	index, err := svc.ListPublishedSkills(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, index.Skills, 1)
	require.Equal(t, "cat-a", index.Skills[0].CatalogID)
}

func TestTenantMarketPublishUnknownOrWrongTenantCatalogIs404(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		9: {"cat-b": tenantCatalog(9, "cat-b", "ocr")},
	}}
	svc, _ := newTenantMarketForTest(published, store, &fakeTenantInstaller{}, nil)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-missing", "user-1")
	requireAppErrorStatus(t, err, 404)

	// cat-b exists — but in tenant 9, so tenant 7 must read it as absent.
	_, err = svc.PublishSkill(context.Background(), 7, "cat-b", "user-1")
	requireAppErrorStatus(t, err, 404)
	require.Empty(t, published.rows, "no publish row may be written for another tenant's catalog")
}

func TestTenantMarketUnpublishSoftDeletesAndIsIdempotent(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-a": tenantCatalog(7, "cat-a", "pdf-extract")},
	}}
	svc, _ := newTenantMarketForTest(published, store, &fakeTenantInstaller{}, nil)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)

	require.NoError(t, svc.UnpublishSkill(context.Background(), 7, "cat-a"))
	index, err := svc.ListPublishedSkills(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, index.Skills)

	// Unpublishing again is an idempotent no-op while the catalog exists.
	require.NoError(t, svc.UnpublishSkill(context.Background(), 7, "cat-a"))
	// Unpublishing a catalog this tenant never had is a 404.
	requireAppErrorStatus(t, svc.UnpublishSkill(context.Background(), 7, "nope"), 404)
	requireAppErrorStatus(t, svc.UnpublishSkill(context.Background(), 9, "cat-a"), 404)
}

func TestTenantMarketUnpublishClearsOrphanedRowAfterCatalogDelete(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-a": tenantCatalog(7, "cat-a", "pdf-extract")},
	}}
	svc, _ := newTenantMarketForTest(published, store, &fakeTenantInstaller{}, nil)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)

	// The catalog row is deleted out from under the publish row
	// (DeleteCatalog knows nothing about the market).
	delete(store.catalogs[7], "cat-a")

	// The orphan neither lists...
	index, err := svc.ListPublishedSkills(context.Background(), 7)
	require.NoError(t, err)
	require.Empty(t, index.Skills, "an orphaned publish row must not list")
	// ...nor blocks the unpublish that cleans it up.
	require.NoError(t, svc.UnpublishSkill(context.Background(), 7, "cat-a"))
	row, err := published.GetByTenantAndCatalog(context.Background(), 7, "cat-a")
	require.NoError(t, err)
	require.Nil(t, row)
}

// ---------------------------------------------------------------------------
// list
// ---------------------------------------------------------------------------

func TestTenantMarketListJoinsCatalogPublisherAndInstallState(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{
		catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
			7: {
				"cat-a": tenantCatalog(7, "cat-a", "pdf-extract"),
				"cat-b": tenantCatalog(7, "cat-b", "ocr"),
			},
		},
		// cat-a is installed on a sandbox config; cat-b is not.
		installs: map[uint64][]*types.TenantSkillEntity{
			7: {{ID: "sk-1", TenantID: 7, SandboxConfigID: "cfg-1", CatalogID: "cat-a", Name: "pdf-extract"}},
		},
	}
	names := &fakePublisherNames{users: map[string]*types.User{
		"user-1": {ID: "user-1", Username: "alice"},
	}}
	svc, _ := newTenantMarketForTest(published, store, &fakeTenantInstaller{}, names)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)
	_, err = svc.PublishSkill(context.Background(), 7, "cat-b", "user-2")
	require.NoError(t, err)

	index, err := svc.ListPublishedSkills(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, index.Skills, 2)

	byCatalog := map[string]interfaces.PublishedSkillEntry{}
	for _, entry := range index.Skills {
		byCatalog[entry.CatalogID] = entry
	}
	a := byCatalog["cat-a"]
	require.Equal(t, "pdf-extract", a.Name)
	require.Equal(t, "1.0", a.Version)
	require.Equal(t, "skill pdf-extract", a.Description)
	require.Equal(t, "alice", a.PublisherName)
	require.True(t, a.Installed, "cat-a has a live install row")

	b := byCatalog["cat-b"]
	require.False(t, b.Installed)
	// An unresolvable publisher id falls back to the stored id rather than
	// vanishing (user-2 has no User row in the fake).
	require.Equal(t, "user-2", b.PublisherName)

	// Another tenant sees none of this.
	other, err := svc.ListPublishedSkills(context.Background(), 9)
	require.NoError(t, err)
	require.Empty(t, other.Skills)
}

func TestTenantMarketListSkipsRowsWithoutLiveCatalog(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-live": tenantCatalog(7, "cat-live", "ocr")},
	}}
	svc, _ := newTenantMarketForTest(published, store, &fakeTenantInstaller{}, nil)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-live", "user-1")
	require.NoError(t, err)
	// A publish row whose catalog row is gone (deleted catalog).
	require.NoError(t, published.Upsert(context.Background(), &types.PublishedSkillEntity{
		ID: "pub-gone", TenantID: 7, CatalogID: "cat-gone", PublishedBy: "user-1",
	}))

	index, err := svc.ListPublishedSkills(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, index.Skills, 1)
	require.Equal(t, "cat-live", index.Skills[0].CatalogID)
}

// ---------------------------------------------------------------------------
// install
// ---------------------------------------------------------------------------

func TestTenantMarketInstallDelegatesToCatalogPipeline(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-a": tenantCatalog(7, "cat-a", "pdf-extract")},
	}}
	installer := &fakeTenantInstaller{}
	svc, _ := newTenantMarketForTest(published, store, installer, nil)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)

	result, err := svc.InstallPublishedSkill(context.Background(), 7, "cat-a", []string{"cfg-1", "cfg-2"})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, installer.calls, 1)
	require.Equal(t, uint64(7), installer.calls[0].tenantID)
	require.Equal(t, "cat-a", installer.calls[0].catalogID)
	require.Equal(t, []string{"cfg-1", "cfg-2"}, installer.calls[0].configIDs)
	require.Equal(t, "skill-for-cfg-1", result.Installs["cfg-1"])
}

func TestTenantMarketInstallRequiresPublishedCatalog(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-a": tenantCatalog(7, "cat-a", "pdf-extract")},
		9: {"cat-b": tenantCatalog(9, "cat-b", "ocr")},
	}}
	installer := &fakeTenantInstaller{}
	svc, _ := newTenantMarketForTest(published, store, installer, nil)

	// Never published in this tenant.
	_, err := svc.InstallPublishedSkill(context.Background(), 7, "cat-a", []string{"cfg-1"})
	requireAppErrorStatus(t, err, 404)
	// Published only in another tenant.
	_, err = svc.PublishSkill(context.Background(), 9, "cat-b", "user-9")
	require.NoError(t, err)
	_, err = svc.InstallPublishedSkill(context.Background(), 7, "cat-b", []string{"cfg-1"})
	requireAppErrorStatus(t, err, 404)
	require.Empty(t, installer.calls, "no install may start for an unpublished skill")

	// Unpublished skills stop being installable through the market.
	_, err = svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)
	require.NoError(t, svc.UnpublishSkill(context.Background(), 7, "cat-a"))
	_, err = svc.InstallPublishedSkill(context.Background(), 7, "cat-a", []string{"cfg-1"})
	requireAppErrorStatus(t, err, 404)
	require.Empty(t, installer.calls)
}

func TestTenantMarketInstallMapsPartialFailures(t *testing.T) {
	published := &fakePublishedStore{}
	store := &fakeTenantCatalogStore{catalogs: map[uint64]map[string]*types.TenantSkillCatalogEntity{
		7: {"cat-a": tenantCatalog(7, "cat-a", "pdf-extract")},
	}}
	installer := &fakeTenantInstaller{answer: func(call tenantInstallCall) (*CatalogInstallResult, error) {
		return &CatalogInstallResult{
			Installs: map[string]string{"cfg-1": "sk-1"},
			Errors:   map[string]string{"cfg-2": apperrors.NewBadRequestError("missing config").Error()},
		}, nil
	}}
	svc, _ := newTenantMarketForTest(published, store, installer, nil)

	_, err := svc.PublishSkill(context.Background(), 7, "cat-a", "user-1")
	require.NoError(t, err)

	result, err := svc.InstallPublishedSkill(context.Background(), 7, "cat-a", []string{"cfg-1", "cfg-2"})
	require.NoError(t, err, "partial per-config failures ride the 202, mirroring the catalog install")
	require.Equal(t, "sk-1", result.Installs["cfg-1"])
	require.Contains(t, result.Errors, "cfg-2")
}
