package career

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	appfile "github.com/Tencent/WeKnora/internal/application/service/file"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLifecycleGateCoordinatesSeparateOfficesAndNeverExpiresClaims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gate.db")
	db1, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o1, err := NewOffice(db1)
	require.NoError(t, err)
	db2, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o2, err := NewOffice(db2)
	require.NoError(t, err)
	scope := Scope{TenantID: 7, UserID: "owner"}
	ctx := WithScope(context.Background(), scope)
	require.NoError(t, o1.ClaimSpace(ctx))
	require.NoError(t, o1.admitLifecycleClaim(ctx, scope, "application_link", "req-1"))
	require.ErrorIs(t, o2.beginLifecycleDeletion(ctx, scope, "delete-1", "delete-fp"), ErrCareerOperationsBusy)
	require.NoError(t, o2.admitLifecycleClaim(ctx, scope, "application_link", "req-1"), "original request must be able to reconcile while deletion drains")
	require.ErrorIs(t, o2.admitLifecycleClaim(ctx, scope, "application_link", "late-request"), ErrCareerDeleting)
	// A claim has no lease and remains visible to another independently
	// constructed Office until same-request reconciliation resolves it.
	var count int64
	require.NoError(t, db2.Model(&lifecycleClaim{}).Where("tenant_id=? AND user_id=?", scope.TenantID, scope.UserID).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, o1.resolveLifecycleClaim(ctx, scope, "application_link", "req-1"))
	require.NoError(t, o2.beginLifecycleDeletion(ctx, scope, "delete-1", "delete-fp"))
	require.ErrorIs(t, o2.beginLifecycleDeletion(ctx, scope, "delete-1", "different-fp"), ErrIdempotencyConflict)
	require.ErrorIs(t, o2.beginLifecycleDeletion(ctx, scope, "delete-2", "other-fp"), ErrCareerDeleting)
	require.ErrorIs(t, o1.admitLifecycleClaim(ctx, scope, "material_publish", "req-2"), ErrCareerDeleting)
}

func TestLifecycleClaimBindsRequestIDToOriginalIntent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claim-fingerprint.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o, err := NewOffice(db)
	require.NoError(t, err)
	scope := Scope{TenantID: 8, UserID: "owner"}
	ctx := WithScope(context.Background(), scope)
	require.NoError(t, o.ClaimSpace(ctx))
	require.NoError(t, o.admitLifecycleClaim(ctx, scope, "material_publish", "same-request", "fingerprint-a"))
	require.NoError(t, o.admitLifecycleClaim(ctx, scope, "material_publish", "same-request", "fingerprint-a"))
	require.ErrorIs(t, o.admitLifecycleClaim(ctx, scope, "material_publish", "same-request", "fingerprint-b"), ErrIdempotencyConflict)
	require.NoError(t, o.resolveLifecycleClaim(context.Background(), scope, "material_publish", "same-request"))
}

func TestLifecycleClaimReleaseRequiresCurrentAttemptOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attempt-owner.db")
	db1, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o1, err := NewOffice(db1)
	require.NoError(t, err)
	db2, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o2, err := NewOffice(db2)
	require.NoError(t, err)
	defer func() { raw, _ := db2.DB(); _ = raw.Close() }()
	scope := Scope{TenantID: 81, UserID: "owner"}
	ctx := WithScope(context.Background(), scope)
	first, releaseFirst, err := o1.acquireLifecycleClaim(ctx, scope, "material_publish", "owner-token-request", "fingerprint")
	require.NoError(t, err)
	require.NoError(t, o1.resolveLifecycleClaimOwned(ctx, scope, "material_publish", "owner-token-request", first))
	releaseFirst()
	second, releaseSecond, err := o2.acquireLifecycleClaim(ctx, scope, "material_publish", "owner-token-request", "fingerprint")
	require.NoError(t, err)
	defer releaseSecond()
	require.NotEqual(t, first, second)
	require.ErrorIs(t, o1.resolveLifecycleClaimOwned(ctx, scope, "material_publish", "owner-token-request", first), ErrUploadClaimLost)
	var count int64
	require.NoError(t, db1.Model(&lifecycleClaim{}).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", scope.TenantID, scope.UserID, "material_publish", "owner-token-request").Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.NoError(t, o2.resolveLifecycleClaimOwned(ctx, scope, "material_publish", "owner-token-request", second))
}

type pausedCareerLinker struct {
	entered chan struct{}
	release chan struct{}
}

func (p *pausedCareerLinker) EnsureCareerApplicationTask(_ context.Context, _ uint64, _ string, intent interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error) {
	close(p.entered)
	<-p.release
	return interfaces.CareerApplicationTaskLink{TaskID: "task-" + intent.RequestID, ApplicationID: intent.ApplicationID}, nil
}
func (p *pausedCareerLinker) FindCareerApplicationTask(context.Context, uint64, string, string) (interfaces.CareerApplicationTaskLink, error) {
	return interfaces.CareerApplicationTaskLink{}, interfaces.ErrCareerApplicationTaskNotFound
}

type pausedFindCareerLinker struct {
	entered chan struct{}
	release chan struct{}
}

func (*pausedFindCareerLinker) EnsureCareerApplicationTask(context.Context, uint64, string, interfaces.CareerApplicationTaskIntent) (interfaces.CareerApplicationTaskLink, error) {
	return interfaces.CareerApplicationTaskLink{}, errors.New("link result unknown")
}
func (p *pausedFindCareerLinker) FindCareerApplicationTask(context.Context, uint64, string, string) (interfaces.CareerApplicationTaskLink, error) {
	close(p.entered)
	<-p.release
	return interfaces.CareerApplicationTaskLink{TaskID: "recovered-task"}, nil
}

func openSecondOffice(t *testing.T, path string) (*Office, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o, err := NewOffice(db)
	require.NoError(t, err)
	return o, db
}

func TestTwoOfficesDeletionWaitsForPausedApplicationLink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application-gate.db")
	db1, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o1, err := NewOffice(db1)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{TenantID: 721, UserID: "owner"})
	require.NoError(t, o1.ClaimSpace(ctx))
	seed := seedApplicationEvaluation(t, o1, ctx, "Backend engineer", "2027", "gate-app")
	view, err := o1.Open(ctx)
	require.NoError(t, err)
	blocker := &pausedCareerLinker{entered: make(chan struct{}), release: make(chan struct{})}
	o1.SetApplicationTaskLinker(blocker)
	o2, db2 := openSecondOffice(t, path)
	defer func() {
		if raw, e := db2.DB(); e == nil {
			_ = raw.Close()
		}
	}()
	result := make(chan error, 1)
	go func() {
		_, e := o1.CreateApplication(ctx, CreateApplicationInput{RequestID: "gate-app-link", OpportunityID: seed.OpportunityID, SnapshotID: seed.SnapshotID, EvaluationID: seed.EvaluationID, BatchIdentity: "fall", ExpectedRevision: view.Revision})
		result <- e
	}()
	select {
	case <-blocker.entered:
	case err = <-result:
		t.Fatalf("application ended before linker admission: %v", err)
	}
	_, err = o2.DeleteCareer(ctx, CareerDeletionInput{RequestID: "gate-app-delete", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrCareerOperationsBusy)
	close(blocker.release)
	require.NoError(t, <-result)
	o2.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	receipt, err := o2.DeleteCareer(ctx, CareerDeletionInput{RequestID: "gate-app-delete", ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
}

func TestReconcileApplicationLinkKeepsClaimUntilFoundReceiptAndThenUnblocksDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "application-reconcile-gate.db")
	db1, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o1, err := NewOffice(db1)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{TenantID: 723, UserID: "owner"})
	require.NoError(t, o1.ClaimSpace(ctx))
	seed := seedApplicationEvaluation(t, o1, ctx, "Backend engineer", "2027", "gate-reconcile")
	view, err := o1.Open(ctx)
	require.NoError(t, err)
	linker := &pausedFindCareerLinker{entered: make(chan struct{}), release: make(chan struct{})}
	o1.SetApplicationTaskLinker(linker)
	_, err = o1.CreateApplication(ctx, CreateApplicationInput{RequestID: "gate-reconcile-link", OpportunityID: seed.OpportunityID, SnapshotID: seed.SnapshotID, EvaluationID: seed.EvaluationID, BatchIdentity: "fall", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	o2, db2 := openSecondOffice(t, path)
	defer func() {
		if raw, e := db2.DB(); e == nil {
			_ = raw.Close()
		}
	}()
	o2.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	result := make(chan error, 1)
	go func() { _, e := o1.ReconcileApplicationLink(ctx, "gate-reconcile-link"); result <- e }()
	select {
	case <-linker.entered:
	case e := <-result:
		t.Fatalf("reconcile ended before lookup: %v", e)
	}
	_, err = o2.DeleteCareer(ctx, CareerDeletionInput{RequestID: "gate-reconcile-delete", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrCareerOperationsBusy)
	close(linker.release)
	require.NoError(t, <-result)
	receipt, err := o2.DeleteCareer(ctx, CareerDeletionInput{RequestID: "gate-reconcile-delete", ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
}

type pausingExportStorage struct {
	inner   *mapExportStorage
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

type writeThenErrorStorage struct {
	inner     *mapExportStorage
	mu        sync.Mutex
	names     []string
	failFirst bool
}

func (s *writeThenErrorStorage) SaveExport(ctx context.Context, tenant uint64, name string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.names = append(s.names, name)
	key, err := s.inner.SaveExport(ctx, tenant, name, data)
	if err != nil {
		return key, err
	}
	if s.failFirst {
		s.failFirst = false
		return key, errors.New("response lost after durable write")
	}
	return key, nil
}
func (s *writeThenErrorStorage) ReadExport(ctx context.Context, key string) ([]byte, error) {
	return s.inner.ReadExport(ctx, key)
}
func (s *writeThenErrorStorage) DeleteExport(ctx context.Context, key string) error {
	return s.inner.DeleteExport(ctx, key)
}

type loseFirstFileServiceResponse struct {
	inner materialExportStorage
	keys  []string
}

func (s *loseFirstFileServiceResponse) SaveExport(ctx context.Context, tenant uint64, name string, data []byte) (string, error) {
	key, err := s.inner.SaveExport(ctx, tenant, name, data)
	if err != nil {
		return "", err
	}
	s.keys = append(s.keys, key)
	if len(s.keys) == 1 {
		return "", errors.New("storage response lost after object write")
	}
	return key, nil
}
func (s *loseFirstFileServiceResponse) ReadExport(ctx context.Context, key string) ([]byte, error) {
	return s.inner.ReadExport(ctx, key)
}
func (s *loseFirstFileServiceResponse) DeleteExport(ctx context.Context, key string) error {
	return s.inner.DeleteExport(ctx, key)
}

func TestFileServiceStorageRetryAfterLostResponseUsesSamePhysicalObject(t *testing.T) {
	files := appfile.NewLocalFileService(t.TempDir(), "")
	storage := &loseFirstFileServiceResponse{inner: newFileExportStorage(files)}
	name := "career_export_retry-stable.pdf"
	_, err := storage.SaveExport(context.Background(), 31, name, []byte("first write"))
	require.ErrorContains(t, err, "response lost")
	key, err := storage.SaveExport(context.Background(), 31, name, []byte("retried write"))
	require.NoError(t, err)
	require.Len(t, storage.keys, 2)
	require.Equal(t, storage.keys[0], storage.keys[1])
	require.Equal(t, key, storage.keys[0])
	data, err := storage.ReadExport(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, []byte("retried write"), data)
}

func (p *pausingExportStorage) SaveExport(ctx context.Context, tenant uint64, name string, data []byte) (string, error) {
	p.once.Do(func() { close(p.entered); <-p.release })
	return p.inner.SaveExport(ctx, tenant, name, data)
}
func (p *pausingExportStorage) ReadExport(ctx context.Context, key string) ([]byte, error) {
	return p.inner.ReadExport(ctx, key)
}
func (p *pausingExportStorage) DeleteExport(ctx context.Context, key string) error {
	return p.inner.DeleteExport(ctx, key)
}

func TestTwoOfficesDeletionWaitsForMaterialPublishAndReplayUsesStableKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "material-gate.db")
	db1, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o1, err := NewOffice(db1)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{TenantID: 722, UserID: "owner"})
	require.NoError(t, o1.ClaimSpace(ctx))
	seed := seedMaterialEvidence(t, o1, ctx, "2027", "gate-pub")
	edit, err := o1.EditMaterial(ctx, editMaterialInput(seed, "gate-pub-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o1.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "gate-pub-confirm", MaterialID: edit.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	storage := &pausingExportStorage{inner: newMapExportStorage(), entered: make(chan struct{}), release: make(chan struct{})}
	o1.SetExportStorage(storage)
	o1.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	o2, db2 := openSecondOffice(t, path)
	defer func() {
		if raw, e := db2.DB(); e == nil {
			_ = raw.Close()
		}
	}()
	o2.SetExportStorage(storage)
	o2.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	o2.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	published := make(chan ExportReceipt, 1)
	publishErr := make(chan error, 1)
	go func() {
		r, e := o1.PublishMaterial(ctx, PublishMaterialInput{RequestID: "gate-publish", MaterialID: edit.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
		published <- r
		publishErr <- e
	}()
	<-storage.entered
	// A second Office using the exact same public request is not an owner and
	// must not enter storage or release the first attempt's lifecycle claim.
	_, retryErr := o2.PublishMaterial(ctx, PublishMaterialInput{RequestID: "gate-publish", MaterialID: edit.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.ErrorIs(t, retryErr, ErrCareerOperationsBusy)
	_, err = o2.DeleteCareer(ctx, CareerDeletionInput{RequestID: "gate-material-delete", ExpectedRevision: seed.Revision})
	require.ErrorIs(t, err, ErrCareerOperationsBusy)
	close(storage.release)
	require.NoError(t, <-publishErr)
	first := <-published
	// Exact replay returns the same receipt/key pair and resolves a claim left
	// behind by a process restart after the first external effect.
	second, err := o2.PublishMaterial(ctx, PublishMaterialInput{RequestID: "gate-publish", MaterialID: edit.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, first.ExportID, second.ExportID)
	require.Equal(t, first.Files, second.Files)
	// Simulate a crash after the receipt transaction committed but before the
	// lifecycle claim release, then verify exact replay reconciles the claim.
	fingerprint, err := materialFingerprint(materialFingerprintPublish, "gate-publish", edit.MaterialID, uint64(1), seed.Revision)
	require.NoError(t, err)
	require.NoError(t, db1.Create(&lifecycleClaim{TenantID: 722, UserID: "owner", Operation: "material_publish", RequestID: "gate-publish", Fingerprint: fingerprint}).Error)
	_, err = o2.PublishMaterial(ctx, PublishMaterialInput{RequestID: "gate-publish", MaterialID: edit.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	var claimCount int64
	require.NoError(t, db1.Model(&lifecycleClaim{}).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", 722, "owner", "material_publish", "gate-publish").Count(&claimCount).Error)
	require.Zero(t, claimCount)
	deletion, err := o2.DeleteCareer(ctx, CareerDeletionInput{RequestID: "gate-material-delete", ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, deletion.Status)
	require.Empty(t, storage.inner.files, fmt.Sprintf("deletion must remove every material object: %v", storage.inner.files))
}

func TestMaterialPublishReplaysSameObjectsAfterUnknownStorageOutcome(t *testing.T) {
	path := filepath.Join(t.TempDir(), "material-replay-gate.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	o, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{TenantID: 724, UserID: "owner"})
	require.NoError(t, o.ClaimSpace(ctx))
	seed := seedMaterialEvidence(t, o, ctx, "2027", "publish-retry")
	edit, err := o.EditMaterial(ctx, editMaterialInput(seed, "publish-retry-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "publish-retry-confirm", MaterialID: edit.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	storage := &writeThenErrorStorage{inner: newMapExportStorage(), failFirst: true}
	o.SetExportStorage(storage)
	o.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	input := PublishMaterialInput{RequestID: "publish-retry-request", MaterialID: edit.MaterialID, Version: 1, ExpectedRevision: seed.Revision}
	_, err = o.PublishMaterial(ctx, input)
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	require.Len(t, storage.inner.files, 1)
	var claims int64
	require.NoError(t, db.Model(&lifecycleClaim{}).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", 724, "owner", "material_publish", input.RequestID).Count(&claims).Error)
	require.EqualValues(t, 1, claims)
	receipt, err := o.PublishMaterial(ctx, input)
	require.NoError(t, err)
	require.Len(t, receipt.Files, 2)
	require.Len(t, storage.inner.files, 2)
	require.Len(t, storage.names, 3)
	require.Equal(t, storage.names[0], storage.names[1], "replay must overwrite the exact request-derived first object")
	require.NoError(t, db.Model(&lifecycleClaim{}).Where("tenant_id=? AND user_id=? AND operation=? AND request_id=?", 724, "owner", "material_publish", input.RequestID).Count(&claims).Error)
	require.Zero(t, claims)
}
