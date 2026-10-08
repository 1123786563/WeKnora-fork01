package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type t05Checker struct {
	allowed     bool
	rejectWrite bool
}

func (c *t05Checker) CheckTaskAccess(_ context.Context, _ craft.Scope, action craft.TaskAction) error {
	if !c.allowed || (c.rejectWrite && action == craft.TaskWrite) {
		return craft.ErrForbidden
	}
	return nil
}

type t05Records struct {
	byRun map[string]craft.KnowledgeRecord
}

type t05FailSaveStore struct {
	CraftKnowledgeRecordStore
	Err error
}

func (s t05FailSaveStore) Save(context.Context, craft.KnowledgeRecord) error { return s.Err }

func (r *t05Records) Save(_ context.Context, rec craft.KnowledgeRecord) error {
	if r.byRun == nil {
		r.byRun = map[string]craft.KnowledgeRecord{}
	}
	if old, ok := r.byRun[rec.RunID]; ok {
		if reflect.DeepEqual(old, rec) {
			return nil
		}
		return craft.ErrConflict
	}
	r.byRun[rec.RunID] = rec
	return nil
}

func TestCraftT05SameRunChangedRequestPreservesPackage(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKB(t, "kb-b", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedKnowledge(t, "k-b", "kb-b", 1, "B")
	f.seedChunk("kb-a", "k-a", "c-a", "original package")
	f.seedChunk("kb-b", "k-b", "c-b", "changed package")
	base := f.service(t, nil)
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())
	_, err = svc.BuildForRun(ctx, scope, "run-immutable", "sales", []string{"k-a"})
	require.NoError(t, err)
	filesBefore := writePathMap(f.writer)
	recordBefore := records.byRun["run-immutable"]

	_, err = svc.BuildForRun(ctx, scope, "run-immutable", "support", []string{"k-b"})
	require.ErrorIs(t, err, craft.ErrConflict)
	require.Equal(t, filesBefore, writePathMap(f.writer), "changed replay must preserve published bytes")
	require.Equal(t, recordBefore, records.byRun["run-immutable"], "changed replay must preserve immutable record")
	_, err = svc.BuildForRun(ctx, scope, "run-immutable", "sales", []string{"k-b"})
	require.ErrorIs(t, err, craft.ErrConflict, "changed selected scope must be rejected")
	require.Equal(t, filesBefore, writePathMap(f.writer))
	require.Equal(t, recordBefore, records.byRun["run-immutable"])
}

func TestCraftT05WriterFailureDoesNotLeavePublishedPackage(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "source excerpt")
	base := f.service(t, nil)
	publisher := newT05Publisher(f.writer)
	publisher.prepareErr = errors.New("simulated private package write failure")
	records := &t05Records{}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	_, err = svc.BuildForRun(craftKnowledgeCtx(craftKnowledgeScope()), craftKnowledgeScope(), "run-write-fail", "sales", []string{"k-a"})
	require.Error(t, err)
	require.Empty(t, writePathMap(f.writer), "private package failure must leave no published material")
	require.Empty(t, publisher.visible)
	require.Empty(t, publisher.candidates, "partial private staging must be discarded")
	require.Empty(t, records.byRun, "failed package must not have an immutable source record")
}

func TestCraftT05BuildForRunFailsClosedWithoutPublisher(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "source excerpt")
	base := f.service(t, nil)
	records := &t05Records{}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records,
	})
	require.NoError(t, err)
	_, err = svc.BuildForRun(craftKnowledgeCtx(craftKnowledgeScope()), craftKnowledgeScope(), "run-no-publisher", "sales", []string{"k-a"})
	require.ErrorIs(t, err, craft.ErrInvalidInput)
	require.Empty(t, f.writer.writes)
	require.Empty(t, records.byRun)
}

func TestCraftT05RecordFailureDoesNotLeavePublishedPackage(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "source excerpt")
	base := f.service(t, nil)
	require.NoError(t, f.db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}))
	store := repository.NewCraftKnowledgeRecordRepository(f.db)
	records := t05FailSaveStore{CraftKnowledgeRecordStore: store, Err: errors.New("injected record save failure")}
	publisher := newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	_, err = svc.BuildForRun(craftKnowledgeCtx(craftKnowledgeScope()), craftKnowledgeScope(), "run-record-fail", "sales", []string{"k-a"})
	require.Error(t, err)
	require.Empty(t, writePathMap(f.writer), "record failure must not leave visible material")
	require.Empty(t, publisher.visible)
	require.Empty(t, publisher.candidates, "failed record save must discard only its private candidate")
	_, loadErr := store.Load(context.Background(), craftKnowledgeScope(), "run-record-fail")
	require.ErrorIs(t, loadErr, craft.ErrNotFound)
}

func TestCraftT05PublishFailureRemainsPrivateAndRetryPublishesSamePackage(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "source excerpt")
	base := f.service(t, nil)
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	publisher.publishErr = errors.New("atomic publish failed")
	searchCalls := 0
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
			searchCalls++
			return base.search(ctx, kbID, params)
		}, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())
	_, err = svc.BuildForRun(ctx, scope, "run-publish-retry", "sales", []string{"k-a"})
	require.Error(t, err)
	require.Empty(t, publisher.visible, "failed atomic publish must leave no visible package")
	require.Empty(t, f.writer.writes)
	require.Contains(t, records.byRun, "run-publish-retry", "the accepted digest permits an exact retry")
	require.NotEmpty(t, publisher.candidates)
	_, err = svc.Sources(ctx, scope, "run-publish-retry")
	require.ErrorIs(t, err, craft.ErrConflict, "an unpublished record must not project as delivered actual sources")
	_, err = svc.AuthorizeSourceOpen(ctx, scope, "run-publish-retry", records.byRun["run-publish-retry"].Sources[0].ID)
	require.ErrorIs(t, err, craft.ErrConflict, "an unpublished record must not authorize source projection")

	publisher.publishErr = nil
	f.results["kb-a"][0].Content = "retrieval changed after publication failure"
	bundle, err := svc.BuildForRun(ctx, scope, "run-publish-retry", "sales", []string{"k-a"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	require.Equal(t, "source excerpt", bundle.Sources[0].Excerpt, "retry must publish the original sealed bytes")
	require.Equal(t, 2, searchCalls, "retry rechecks KB authority, but uses the accepted package rather than the search result")
	require.Len(t, publisher.visible, 1)
	filesBeforeReplay := writePathMap(f.writer)
	committedBeforeReplay := publisher.commitFiles
	_, err = svc.BuildForRun(ctx, scope, "run-publish-retry", "sales", []string{"k-a"})
	require.NoError(t, err)
	require.Equal(t, filesBeforeReplay, writePathMap(f.writer))
	require.Equal(t, committedBeforeReplay, publisher.commitFiles, "identical replay must not rewrite the stable package")
}

func TestCraftT05RetryFailsClosedWhenSealedCandidateIsAbsent(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "original bytes")
	base := f.service(t, nil)
	searchCalls := 0
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return base.search(ctx, kbID, params)
	}
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	publisher.publishErr = errors.New("temporary publish failure")
	cfg := CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	}
	svc, err := NewCraftKnowledgeService(cfg)
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())
	_, err = svc.BuildForRun(ctx, scope, "run-candidate-lost", "sales", []string{"k-a"})
	require.Error(t, err)
	require.Equal(t, 1, searchCalls)
	publisher.candidates = map[string]CraftKnowledgeMaterialPackage{} // simulate lost private store after restart
	f.results["kb-a"][0].Content = "different live retrieval"

	restarted, err := NewCraftKnowledgeService(cfg)
	require.NoError(t, err)
	_, err = restarted.BuildForRun(ctx, scope, "run-candidate-lost", "sales", []string{"k-a"})
	require.ErrorIs(t, err, craft.ErrNotFound, "missing accepted candidate must not be regenerated from live search")
	require.Equal(t, 1, searchCalls, "recovery must consult the accepted candidate before live retrieval")
	require.Empty(t, publisher.visible)
	_, err = restarted.Sources(ctx, scope, "run-candidate-lost")
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftT05PreparedRetryRejectsRevokedDocumentGrant(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "private source bytes")
	base := f.service(t, nil)
	accessCalls, searchCalls := 0, 0
	documentAllowed := true
	access := func(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
		accessCalls++
		if !documentAllowed {
			return nil, nil
		}
		return base.access(ctx, tenantID, ids)
	}
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return base.search(ctx, kbID, params)
	}
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	publisher.publishErr = errors.New("temporary publication failure")
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())
	_, err = svc.BuildForRun(ctx, scope, "run-document-revoked", "sales", []string{"k-a"})
	require.Error(t, err)
	require.Equal(t, 1, searchCalls)
	require.Equal(t, craft.KnowledgePublicationPrepared, records.byRun["run-document-revoked"].PublicationState)
	require.NotEmpty(t, publisher.candidates)
	require.Empty(t, publisher.visible)
	publisher.publishErr = nil
	documentAllowed = false
	accessBeforeRetry := accessCalls
	_, err = svc.BuildForRun(ctx, scope, "run-document-revoked", "sales", []string{"k-a"})
	require.ErrorIs(t, err, craft.ErrForbidden, "current document access is required before retry publication")
	require.Equal(t, accessBeforeRetry+1, accessCalls, "retry must re-resolve the accepted document IDs")
	require.Equal(t, 1, searchCalls, "denied documents must be rejected before KB search")
	require.Equal(t, 1, publisher.publishCalls, "revocation must prevent a second Publish")
	require.Empty(t, publisher.visible)
	require.NotEmpty(t, publisher.candidates, "denial leaves the exact candidate private for a later authorized retry")
	require.Equal(t, craft.KnowledgePublicationPrepared, records.byRun["run-document-revoked"].PublicationState)
	_, err = svc.Sources(ctx, scope, "run-document-revoked")
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftT05PreparedRetryRejectsRevokedKnowledgeBaseGrant(t *testing.T) {
	f := newKnowledgeFixture(t, map[string]bool{"kb-shared": true})
	f.seedKB(t, "kb-shared", 2)
	f.seedKnowledge(t, "k-shared", "kb-shared", 2, "Shared")
	f.seedChunk("kb-shared", "k-shared", "c-shared", "private shared bytes")
	searchACL := &fakeKBShareService{allowedKBs: map[string]bool{"kb-shared": true}}
	base := f.service(t, searchACL)
	accessCalls, searchCalls := 0, 0
	access := func(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
		accessCalls++
		return base.access(ctx, tenantID, ids)
	}
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return base.search(ctx, kbID, params)
	}
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	publisher.publishErr = errors.New("temporary publication failure")
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())
	_, err = svc.BuildForRun(ctx, scope, "run-kb-revoked", "sales", []string{"k-shared"})
	require.Error(t, err)
	require.Equal(t, craft.KnowledgePublicationPrepared, records.byRun["run-kb-revoked"].PublicationState)
	require.Empty(t, publisher.visible)
	publisher.publishErr = nil
	searchACL.allowedKBs["kb-shared"] = false
	accessBeforeRetry := accessCalls
	_, err = svc.BuildForRun(ctx, scope, "run-kb-revoked", "sales", []string{"k-shared"})
	require.ErrorIs(t, err, craft.ErrForbidden, "current owning-KB permission is required before retry publication")
	require.Equal(t, accessBeforeRetry+1, accessCalls, "retry must first reauthorize accepted documents")
	require.Equal(t, 2, searchCalls, "retry must enter the guarded KB authorization seam")
	require.Equal(t, 1, publisher.publishCalls, "revocation must prevent a second Publish")
	require.Empty(t, publisher.visible)
	require.NotEmpty(t, publisher.candidates)
	require.Equal(t, craft.KnowledgePublicationPrepared, records.byRun["run-kb-revoked"].PublicationState)
	_, err = svc.Sources(ctx, scope, "run-kb-revoked")
	require.ErrorIs(t, err, craft.ErrConflict)
}

func TestCraftT05SQLiteRecordProtectsSameRunMaterialDigest(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "original source bytes")
	require.NoError(t, f.db.AutoMigrate(&repository.CraftKnowledgeRecordRow{}))
	base := f.service(t, nil)
	publisher := newT05Publisher(f.writer)
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: repository.NewCraftKnowledgeRecordRepository(f.db),
		Publisher: publisher, Now: func() time.Time { return now },
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())
	first, err := svc.BuildForRun(ctx, scope, "run-real-record", "sales", []string{"k-a"})
	require.NoError(t, err)
	recordBefore, err := svc.records.Load(ctx, scope, "run-real-record")
	require.NoError(t, err)
	require.NotEmpty(t, recordBefore.RequestDigest)
	require.NotEmpty(t, recordBefore.PackageDigest)
	filesBefore := writePathMap(f.writer)
	committedBeforeReplay := publisher.commitFiles

	now = now.Add(time.Hour)
	replay, err := svc.BuildForRun(ctx, scope, "run-real-record", "sales", []string{"k-a"})
	require.NoError(t, err)
	require.Equal(t, first.Sources, replay.Sources, "same Run replay retains original acquisition facts")
	require.Equal(t, filesBefore, writePathMap(f.writer))
	require.Equal(t, committedBeforeReplay, publisher.commitFiles)

	f.results["kb-a"][0].Content = "changed source bytes"
	replay, err = svc.BuildForRun(ctx, scope, "run-real-record", "sales", []string{"k-a"})
	require.NoError(t, err, "published Runs resume the accepted package instead of rebuilding from changed retrieval")
	require.Equal(t, first.Sources, replay.Sources)
	recordAfter, loadErr := svc.records.Load(ctx, scope, "run-real-record")
	require.NoError(t, loadErr)
	require.Equal(t, recordBefore, recordAfter)
	require.Equal(t, filesBefore, writePathMap(f.writer))
}

func (r *t05Records) Load(_ context.Context, _ craft.Scope, runID string) (craft.KnowledgeRecord, error) {
	rec, ok := r.byRun[runID]
	if !ok {
		return craft.KnowledgeRecord{}, craft.ErrNotFound
	}
	return rec, nil
}

func (r *t05Records) MarkPublished(_ context.Context, scope craft.Scope, runID, packageDigest string) error {
	rec, ok := r.byRun[runID]
	if !ok {
		return craft.ErrNotFound
	}
	if !craft.SameScope(scope, rec.Scope) || rec.PackageDigest != packageDigest {
		return craft.ErrConflict
	}
	if rec.PublicationState == craft.KnowledgePublicationPublished {
		return nil
	}
	if rec.PublicationState != craft.KnowledgePublicationPrepared && rec.PublicationState != "" {
		return craft.ErrConflict
	}
	rec.PublicationState = craft.KnowledgePublicationPublished
	r.byRun[runID] = rec
	return nil
}

type t05Publisher struct {
	visible      map[string]CraftKnowledgeMaterialPackage
	candidates   map[string]CraftKnowledgeMaterialPackage
	writer       *recordingWriter
	prepareErr   error
	publishErr   error
	discardErr   error
	commitFiles  int
	publishCalls int
}

func newT05Publisher(writer *recordingWriter) *t05Publisher {
	return &t05Publisher{visible: map[string]CraftKnowledgeMaterialPackage{}, candidates: map[string]CraftKnowledgeMaterialPackage{}, writer: writer}
}

func packageKey(pkg CraftKnowledgeMaterialPackage) string { return pkg.RunID + ":" + pkg.Digest }

func (p *t05Publisher) Prepare(_ context.Context, _ craft.Workspace, pkg CraftKnowledgeMaterialPackage) error {
	p.candidates[packageKey(pkg)] = cloneCraftKnowledgePackage(pkg)
	if p.prepareErr != nil {
		return p.prepareErr
	}
	if prior, ok := p.visible[pkg.RunID]; ok {
		if prior.Digest == pkg.Digest {
			return nil
		}
		return craft.ErrConflict
	}
	return nil
}

func (p *t05Publisher) Publish(_ context.Context, _ craft.Workspace, pkg CraftKnowledgeMaterialPackage) error {
	p.publishCalls++
	if prior, ok := p.visible[pkg.RunID]; ok {
		if prior.Digest == pkg.Digest {
			return nil
		}
		return craft.ErrConflict
	}
	if p.publishErr != nil {
		return p.publishErr
	}
	candidate, ok := p.candidates[packageKey(pkg)]
	if !ok {
		return errors.New("private package candidate missing")
	}
	p.visible[pkg.RunID] = cloneCraftKnowledgePackage(candidate)
	delete(p.candidates, packageKey(pkg))
	paths := make([]string, 0, len(candidate.Files))
	for path := range candidate.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		p.writer.writes = append(p.writer.writes, stagedWrite{path: path, content: append([]byte(nil), candidate.Files[path]...)})
		p.commitFiles++
	}
	return nil
}

func (p *t05Publisher) Resume(_ context.Context, _ craft.Workspace, runID, acceptedDigest string) (CraftKnowledgeMaterialPackage, error) {
	if pkg, ok := p.visible[runID]; ok && pkg.Digest == acceptedDigest {
		return cloneCraftKnowledgePackage(pkg), nil
	}
	if pkg, ok := p.candidates[runID+":"+acceptedDigest]; ok {
		return cloneCraftKnowledgePackage(pkg), nil
	}
	return CraftKnowledgeMaterialPackage{}, craft.ErrNotFound
}

func (p *t05Publisher) Discard(_ context.Context, _ craft.Workspace, pkg CraftKnowledgeMaterialPackage) error {
	if p.discardErr != nil {
		return p.discardErr
	}
	delete(p.candidates, packageKey(pkg))
	return nil
}

func TestCraftT05Journey(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKB(t, "kb-b", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedKnowledge(t, "k-b", "kb-b", 1, "B")
	f.seedChunk("kb-a", "k-a", "c-a", "selected excerpt")
	f.seedChunk("kb-b", "k-b", "c-b", "unselected secret")
	checker := &t05Checker{allowed: true}
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	fixed := time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC)
	old := f.service(t, nil)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{Store: old.store, Access: old.access, Search: old.search, Writer: f.writer.write, TaskAccess: checker, Records: records, Publisher: publisher, Now: func() time.Time { return fixed }})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	ctx := craftKnowledgeCtx(scope)
	bundle, err := svc.BuildForRun(ctx, scope, "run-1", "sales", []string{"k-a"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, 1)
	require.Len(t, manifestSources(t, manifestOf(t, f.writer)), 1)
	rec := records.byRun["run-1"]
	require.Equal(t, "run-1", rec.RunID)
	require.Equal(t, craft.KnowledgePublicationPublished, rec.PublicationState)
	require.Len(t, rec.Sources, 1)
	require.Equal(t, fixed, rec.Sources[0].AcquiredAt)
	require.Equal(t, bundle.Sources[0].Ref, rec.Sources[0].Ref)
	require.Equal(t, bundle.Sources[0].Digest, rec.Sources[0].Digest)
	require.LessOrEqual(t, rec.Sources[0].ExcerptBytes, craft.MaxKnowledgeExcerptBytes)
	ref, err := svc.AuthorizeSourceOpen(ctx, scope, "run-1", rec.Sources[0].ID)
	require.NoError(t, err)
	require.Equal(t, rec.Sources[0].Ref, ref)
	checker.allowed = false
	_, err = svc.AuthorizeSourceOpen(ctx, scope, "run-1", rec.Sources[0].ID)
	require.ErrorIs(t, err, craft.ErrForbidden)
	checker.allowed = true
	// A later resource revocation is independently checked, even with Task access.
	// Empty results still have a durable, visible disclosure.
	f.results["kb-a"] = nil
	empty, err := svc.BuildForRun(ctx, scope, "run-empty", "none", []string{"k-a"})
	require.NoError(t, err)
	require.True(t, empty.Empty)
	require.True(t, records.byRun["run-empty"].Empty)
	require.Equal(t, true, manifestOf(t, f.writer)["empty"])
	for i := 0; i < craft.MaxKnowledgeSources+1; i++ {
		f.seedChunk("kb-a", "k-a", fmt.Sprintf("c-%d", i), "excerpt")
	}
	truncated, err := svc.BuildForRun(ctx, scope, "run-truncated", "many", []string{"k-a"})
	require.NoError(t, err)
	require.True(t, truncated.Truncated)
	require.True(t, records.byRun["run-truncated"].Truncated)
}

func TestCraftT05BuildForRunRequiresTaskWriteBeforeSideEffects(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
	f.seedChunk("kb-a", "k-a", "c-a", "secret excerpt")
	base := f.service(t, nil)
	checker := &t05Checker{allowed: true, rejectWrite: true}
	records := &t05Records{}
	publisher := newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: checker, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)

	_, err = svc.BuildForRun(craftKnowledgeCtx(craftKnowledgeScope()), craftKnowledgeScope(), "run-viewer", "sales", []string{"k-a"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Empty(t, f.writer.writes, "read-only caller must not stage material")
	require.Empty(t, records.byRun, "read-only caller must not create a Run source record")
}

func TestCraftT05BuildForKnowledgeBasesScopesSearchAndIntersectsDocumentAccess(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKB(t, "kb-b", 1)
	f.seedKnowledge(t, "doc-a", "kb-a", 1, "A")
	f.seedKnowledge(t, "doc-denied", "kb-a", 1, "Denied")
	f.seedKnowledge(t, "doc-b", "kb-b", 1, "B")
	f.seedChunk("kb-a", "doc-a", "chunk-a", "selected source")
	f.seedChunk("kb-a", "doc-denied", "chunk-denied", "denied source")
	// A compromised/noisy result from selected KB A that actually belongs to
	// unselected KB B must be rejected after shared-aware document resolution.
	f.seedChunk("kb-a", "doc-b", "chunk-wrong-kb", "unselected secret")
	f.seedChunk("kb-b", "doc-b", "chunk-b", "unselected secret")
	base := f.service(t, nil)
	var searchCalls []struct {
		kbID   string
		params types.SearchParams
	}
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls = append(searchCalls, struct {
			kbID   string
			params types.SearchParams
		}{kbID: kbID, params: params})
		return base.search(ctx, kbID, params)
	}
	access := func(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
		rows, err := base.access(ctx, tenantID, ids)
		if err != nil {
			return nil, err
		}
		allowed := rows[:0]
		for _, row := range rows {
			if row.ID != "doc-denied" {
				allowed = append(allowed, row)
			}
		}
		return allowed, nil
	}
	writer := f.writer
	records, publisher := &t05Records{}, newT05Publisher(writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: access, Search: search, Writer: writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)

	scope := craftKnowledgeScope()
	bundle, err := svc.BuildForKnowledgeBases(craftKnowledgeCtx(scope), scope, "run-kb-selection", "sales", []string{"kb-a"})
	require.NoError(t, err)
	require.Len(t, searchCalls, 1, "only the selected KB should be searched")
	require.Equal(t, "kb-a", searchCalls[0].kbID)
	require.Equal(t, []string{"kb-a"}, searchCalls[0].params.KnowledgeBaseIDs)
	require.Empty(t, searchCalls[0].params.KnowledgeIDs, "KB selection must not be passed as document IDs")
	require.Equal(t, "sales", searchCalls[0].params.QueryText)
	require.Equal(t, craft.MaxKnowledgeSources+1, searchCalls[0].params.MatchCount)
	require.Len(t, bundle.Sources, 1)
	require.Contains(t, bundle.Sources[0].Ref, "doc-a")
	require.NotContains(t, bundle.Sources[0].Ref, "doc-denied")
	require.NotContains(t, bundle.Sources[0].Ref, "doc-b")
	require.Equal(t, craft.KnowledgePublicationPublished, records.byRun["run-kb-selection"].PublicationState)
	files := writePathMap(writer)
	for _, content := range files {
		require.NotContains(t, string(content), "denied source")
		require.NotContains(t, string(content), "unselected secret")
	}
}

func TestCraftT05BuildForKnowledgeBasesEmptyTruncatedAndRevokedReplay(t *testing.T) {
	f := newKnowledgeFixture(t, map[string]bool{"kb-shared": true})
	f.seedKB(t, "kb-shared", 2)
	f.seedKnowledge(t, "doc-shared", "kb-shared", 2, "Shared")
	base := f.service(t, nil)
	searchCalls := 0
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return base.search(ctx, kbID, params)
	}
	records, publisher := &t05Records{}, newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())

	empty, err := svc.BuildForKnowledgeBases(ctx, scope, "run-empty-kb", "no matches", []string{"kb-shared"})
	require.NoError(t, err, "an authorized selected KB may legitimately return no documents")
	require.True(t, empty.Empty)
	require.True(t, records.byRun["run-empty-kb"].Empty)
	require.Equal(t, craft.KnowledgePublicationPublished, records.byRun["run-empty-kb"].PublicationState)

	for i := 0; i < craft.MaxKnowledgeSources+1; i++ {
		f.seedChunk("kb-shared", "doc-shared", fmt.Sprintf("chunk-%02d", i), "excerpt")
	}
	truncated, err := svc.BuildForKnowledgeBases(ctx, scope, "run-truncated-kb", "many", []string{"kb-shared"})
	require.NoError(t, err)
	require.True(t, truncated.Truncated)
	require.True(t, records.byRun["run-truncated-kb"].Truncated)

	// Replaying an empty-source Run still re-enters the guarded KB search port,
	// because the immutable record has no source rows from which to infer KB ACL.
	shareCallsBeforeReplay := searchCalls
	publishCallsBeforeReplay := publisher.publishCalls
	f.shares.allowedKBs["kb-shared"] = false
	_, err = svc.BuildForKnowledgeBases(ctx, scope, "run-empty-kb", "no matches", []string{"kb-shared"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	require.Greater(t, searchCalls, shareCallsBeforeReplay, "empty record replay must reauthorize its selected KB")
	require.Equal(t, publishCallsBeforeReplay, publisher.publishCalls, "revoked empty Run must not be republished")
}

func TestCraftT05BuildForKnowledgeBasesExplicitEmptySelectionPublishesWithoutSearch(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	base := f.service(t, nil)
	searchCalls := 0
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return base.search(ctx, kbID, params)
	}
	checker := &t05Checker{allowed: true}
	records, publisher := &t05Records{}, newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: search, Writer: f.writer.write,
		TaskAccess: checker, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx, runID := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope()), "run-no-selected-kb"

	bundle, err := svc.BuildForKnowledgeBases(ctx, scope, runID, "original query", []string{})
	require.NoError(t, err)
	require.True(t, bundle.Empty)
	require.Empty(t, bundle.Sources)
	require.Zero(t, searchCalls, "an explicit empty selection must never search every KB")
	record := records.byRun[runID]
	require.Equal(t, scope, record.Scope)
	require.Equal(t, runID, record.RunID)
	require.Equal(t, craft.KnowledgePublicationPublished, record.PublicationState)
	require.True(t, record.Empty)
	require.NotEmpty(t, record.PackageDigest)
	pkg := publisher.visible[runID]
	require.Equal(t, record.PackageDigest, pkg.Digest)
	require.Len(t, pkg.Files, 1, "the sealed empty package contains only its manifest")
	require.NotEmpty(t, pkg.Files[craft.KnowledgeRunDir(runID)+"/manifest.json"])

	replay, err := svc.BuildForKnowledgeBases(ctx, scope, runID, "original query", nil)
	require.NoError(t, err)
	require.True(t, replay.Empty)
	require.Equal(t, 0, searchCalls, "replay of an empty selection must not search")
	require.Equal(t, record.PackageDigest, records.byRun[runID].PackageDigest)

	_, err = svc.BuildForKnowledgeBases(ctx, scope, runID, "changed query", []string{})
	require.ErrorIs(t, err, craft.ErrConflict, "a Run cannot change its accepted query on replay")
	require.Equal(t, 0, searchCalls)
	require.Equal(t, record.PackageDigest, publisher.visible[runID].Digest)

	checker.allowed = false
	_, err = svc.BuildForKnowledgeBases(ctx, scope, runID, "original query", []string{})
	require.ErrorIs(t, err, craft.ErrForbidden, "a previously accepted empty package still requires current TaskWrite")
	require.Equal(t, 0, searchCalls)
}

func TestCraftT05BuildForKnowledgeBasesReplayUsesAcceptedPackageAndSelectionDigest(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKB(t, "kb-b", 1)
	f.seedKnowledge(t, "doc-a", "kb-a", 1, "A")
	f.seedKnowledge(t, "doc-b", "kb-b", 1, "B")
	f.seedChunk("kb-a", "doc-a", "chunk-a", "original accepted excerpt")
	f.seedChunk("kb-b", "doc-b", "chunk-b", "other selection")
	base := f.service(t, nil)
	searchCalls := 0
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return base.search(ctx, kbID, params)
	}
	records, publisher := &t05Records{}, newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	scope, ctx := craftKnowledgeScope(), craftKnowledgeCtx(craftKnowledgeScope())

	first, err := svc.BuildForKnowledgeBases(ctx, scope, "run-kb-replay", "sales", []string{"kb-a"})
	require.NoError(t, err)
	firstSearchCalls := searchCalls
	f.results["kb-a"][0].Content = "changed live retrieval"
	replay, err := svc.BuildForKnowledgeBases(ctx, scope, "run-kb-replay", "sales", []string{"kb-a"})
	require.NoError(t, err)
	require.Equal(t, first.Sources, replay.Sources, "replay returns sealed source facts, not changed live search output")
	require.Equal(t, "original accepted excerpt", replay.Sources[0].Excerpt)
	require.Greater(t, searchCalls, firstSearchCalls, "current KB authority is checked on replay")
	filesBefore := writePathMap(f.writer)
	recordBefore := records.byRun["run-kb-replay"]
	_, err = svc.BuildForKnowledgeBases(ctx, scope, "run-kb-replay", "sales", []string{"kb-b"})
	require.ErrorIs(t, err, craft.ErrConflict, "same Run cannot change its typed KB selection")
	require.Equal(t, filesBefore, writePathMap(f.writer))
	require.Equal(t, recordBefore, records.byRun["run-kb-replay"])
}

func TestCraftT05KnowledgeBaseSearchUsesBoundedSentinelForTruncation(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	for i := 0; i < craft.MaxKnowledgeSources+1; i++ {
		docID := fmt.Sprintf("doc-%02d", i)
		f.seedKnowledge(t, docID, "kb-a", 1, docID)
		f.seedChunk("kb-a", docID, fmt.Sprintf("chunk-%02d", i), fmt.Sprintf("excerpt-%02d", i))
	}
	base := f.service(t, nil)
	var matchCounts []int
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		matchCounts = append(matchCounts, params.MatchCount)
		results, err := base.search(ctx, kbID, params)
		if err != nil {
			return nil, err
		}
		// Production HybridSearch clips to MatchCount before returning.
		if len(results) > params.MatchCount {
			results = results[:params.MatchCount]
		}
		return results, nil
	}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: &t05Records{}, Publisher: newT05Publisher(f.writer),
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	bundle, err := svc.BuildForKnowledgeBases(craftKnowledgeCtx(scope), scope, "run-sentinel", "sales", []string{"kb-a"})
	require.NoError(t, err)
	require.Equal(t, []int{craft.MaxKnowledgeSources + 1}, matchCounts, "request exactly one bounded sentinel beyond the material cap")
	require.Len(t, bundle.Sources, craft.MaxKnowledgeSources)
	require.True(t, bundle.Truncated, "the authorized sentinel proves an eligible source was omitted")
}

func TestCraftT05KnowledgeBaseSearchSaturatedNoiseIsConservativelyTruncated(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKB(t, "kb-b", 1)
	f.seedKB(t, "kb-c", 1)
	for i := 0; i < craft.MaxKnowledgeSources-1; i++ {
		docID := fmt.Sprintf("doc-a-%02d", i)
		f.seedKnowledge(t, docID, "kb-a", 1, docID)
		f.seedChunk("kb-a", docID, fmt.Sprintf("chunk-a-%02d", i), "authorized A")
	}
	f.seedKnowledge(t, "doc-denied", "kb-a", 1, "Denied")
	f.seedChunk("kb-a", "doc-denied", "chunk-denied", "denied secret")
	f.seedKnowledge(t, "doc-b", "kb-b", 1, "B")
	f.seedKnowledge(t, "doc-c", "kb-c", 1, "C")
	// The selected A search returns a cross-KB document as noisy backend output.
	f.seedChunk("kb-a", "doc-c", "chunk-cross-kb", "unselected secret")
	f.seedChunk("kb-b", "doc-b", "chunk-b", "authorized B")
	base := f.service(t, nil)
	var searchedKBs [][]string
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		searchedKBs = append(searchedKBs, append([]string(nil), params.KnowledgeBaseIDs...))
		results, err := base.search(ctx, kbID, params)
		if err != nil {
			return nil, err
		}
		if len(results) > params.MatchCount {
			results = results[:params.MatchCount]
		}
		return results, nil
	}
	access := func(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
		rows, err := base.access(ctx, tenantID, ids)
		if err != nil {
			return nil, err
		}
		allowed := rows[:0]
		for _, row := range rows {
			if row.ID != "doc-denied" {
				allowed = append(allowed, row)
			}
		}
		return allowed, nil
	}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: &t05Records{}, Publisher: newT05Publisher(f.writer),
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	bundle, err := svc.BuildForKnowledgeBases(craftKnowledgeCtx(scope), scope, "run-saturated-noise", "sales", []string{"kb-b", "kb-a"})
	require.NoError(t, err)
	require.Equal(t, [][]string{{"kb-a"}, {"kb-b"}}, searchedKBs, "each KB is an independent bounded search")
	require.True(t, bundle.Truncated, "a full search limit with filtered noise cannot prove eligible results are exhausted")
	require.NotContains(t, fmt.Sprint(bundle.Sources), "doc-denied")
	require.NotContains(t, fmt.Sprint(bundle.Sources), "doc-c")
	for _, write := range f.writer.writes {
		require.NotContains(t, string(write.content), "denied secret")
		require.NotContains(t, string(write.content), "unselected secret")
	}
}

func TestCraftT05KnowledgeBaseSearchBelowSentinelReportsComplete(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	for i := 0; i < craft.MaxKnowledgeSources-1; i++ {
		docID := fmt.Sprintf("doc-%02d", i)
		f.seedKnowledge(t, docID, "kb-a", 1, docID)
		f.seedChunk("kb-a", docID, fmt.Sprintf("chunk-%02d", i), "excerpt")
	}
	base := f.service(t, nil)
	search := func(ctx context.Context, kbID string, params types.SearchParams) ([]*types.SearchResult, error) {
		results, err := base.search(ctx, kbID, params)
		if err != nil {
			return nil, err
		}
		if len(results) > params.MatchCount {
			results = results[:params.MatchCount]
		}
		return results, nil
	}
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: &t05Records{}, Publisher: newT05Publisher(f.writer),
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	bundle, err := svc.BuildForKnowledgeBases(craftKnowledgeCtx(scope), scope, "run-below-sentinel", "sales", []string{"kb-a"})
	require.NoError(t, err)
	require.Len(t, bundle.Sources, craft.MaxKnowledgeSources-1)
	require.False(t, bundle.Truncated)
}

func TestCraftT05RevalidateForDispatchChecksPublishedRecordAndCurrentDocumentAccess(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "doc-a", "kb-a", 1, "A")
	scope, runID := craftKnowledgeScope(), "run-dispatch-valid"
	record := t05PublishedKnowledgeRecord(scope, runID, craft.KnowledgeSourceRecord{
		ID:  craft.KnowledgeCitationID("kb-a", "doc-a", "chunk-a"),
		Ref: craft.KnowledgeRef("kb-a", "doc-a", "chunk-a"), TenantID: 1,
	})
	base := f.service(t, nil)
	accessCalls, searchCalls := 0, 0
	access := func(ctx context.Context, tenantID uint64, ids []string) ([]*types.Knowledge, error) {
		accessCalls++
		return base.access(ctx, tenantID, ids)
	}
	search := func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
		searchCalls++
		return nil, errors.New("dispatch revalidation must not search")
	}
	svc, records, publisher := t05DispatchRevalidator(t, f, &t05Checker{allowed: true}, record, access, search)
	require.NoError(t, svc.RevalidateForDispatch(craftKnowledgeCtx(scope), scope, runID))
	require.Equal(t, 1, accessCalls, "every recorded source must be resolved through current shared-aware access")
	require.Zero(t, searchCalls, "dispatch revalidation must not search or rebuild")
	require.Zero(t, publisher.publishCalls)
	require.Empty(t, publisher.candidates)
	require.Equal(t, record, records.byRun[runID], "read-only revalidation must not mutate the record")
}

func TestCraftT05RevalidateForDispatchRequiresCurrentTaskAndResourceGrants(t *testing.T) {
	for _, tc := range []struct {
		name       string
		shared     bool
		rejectTask bool
		revokeDoc  bool
		revokeKB   bool
	}{
		{name: "task write revoked", rejectTask: true},
		{name: "document revoked", revokeDoc: true},
		{name: "owning kb grant revoked", shared: true, revokeKB: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allowedKBs := map[string]bool(nil)
			kbID, docID := "kb-a", "doc-a"
			tenantID := uint64(1)
			if tc.shared {
				allowedKBs = map[string]bool{"kb-shared": true}
				kbID, docID, tenantID = "kb-shared", "doc-shared", 2
			}
			f := newKnowledgeFixture(t, allowedKBs)
			f.seedKB(t, kbID, tenantID)
			f.seedKnowledge(t, docID, kbID, tenantID, "Source")
			scope, runID := craftKnowledgeScope(), "run-dispatch-"+strings.ReplaceAll(tc.name, " ", "-")
			record := t05PublishedKnowledgeRecord(scope, runID, craft.KnowledgeSourceRecord{
				ID:  craft.KnowledgeCitationID(kbID, docID, "chunk-a"),
				Ref: craft.KnowledgeRef(kbID, docID, "chunk-a"), TenantID: tenantID,
			})
			base := f.service(t, nil)
			access := base.access
			if tc.revokeDoc {
				access = func(context.Context, uint64, []string) ([]*types.Knowledge, error) { return nil, nil }
			}
			if tc.revokeKB {
				f.shares.allowedKBs[kbID] = false
			}
			svc, _, publisher := t05DispatchRevalidator(t, f, &t05Checker{allowed: true, rejectWrite: tc.rejectTask}, record, access, nil)
			err := svc.RevalidateForDispatch(craftKnowledgeCtx(scope), scope, runID)
			require.ErrorIs(t, err, craft.ErrForbidden)
			require.Zero(t, publisher.publishCalls)
			require.Empty(t, publisher.candidates)
		})
	}
}

func TestCraftT05RevalidateForDispatchRejectsForgedActorScopeAndUnpublishedRecord(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	f.seedKB(t, "kb-a", 1)
	f.seedKnowledge(t, "doc-a", "kb-a", 1, "A")
	scope, runID := craftKnowledgeScope(), "run-dispatch-invalid"
	base := f.service(t, nil)
	validSource := craft.KnowledgeSourceRecord{
		ID:  craft.KnowledgeCitationID("kb-a", "doc-a", "chunk-a"),
		Ref: craft.KnowledgeRef("kb-a", "doc-a", "chunk-a"), TenantID: 1,
	}
	for _, tc := range []struct {
		name       string
		ctx        context.Context
		record     craft.KnowledgeRecord
		requestRun string
	}{
		{name: "authenticated actor mismatch", ctx: craftKnowledgeCtx(craft.Scope{TenantID: 1, UserID: "forged", SessionID: scope.SessionID}), record: t05PublishedKnowledgeRecord(scope, runID, validSource), requestRun: runID},
		{name: "record scope mismatch", ctx: craftKnowledgeCtx(scope), record: t05PublishedKnowledgeRecord(craft.Scope{TenantID: 1, UserID: scope.UserID, SessionID: "other-session"}, runID, validSource), requestRun: runID},
		{name: "record tenant mismatch", ctx: craftKnowledgeCtx(scope), record: t05PublishedKnowledgeRecord(craft.Scope{TenantID: 2, UserID: scope.UserID, SessionID: scope.SessionID}, runID, validSource), requestRun: runID},
		{name: "record run mismatch", ctx: craftKnowledgeCtx(scope), record: t05PublishedKnowledgeRecord(scope, "different-run", validSource), requestRun: runID},
		{name: "prepared not published", ctx: craftKnowledgeCtx(scope), record: func() craft.KnowledgeRecord {
			r := t05PublishedKnowledgeRecord(scope, runID, validSource)
			r.PublicationState = craft.KnowledgePublicationPrepared
			return r
		}(), requestRun: runID},
		{name: "unknown publication state", ctx: craftKnowledgeCtx(scope), record: func() craft.KnowledgeRecord {
			r := t05PublishedKnowledgeRecord(scope, runID, validSource)
			r.PublicationState = "unknown"
			return r
		}(), requestRun: runID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, records, publisher := t05DispatchRevalidator(t, f, &t05Checker{allowed: true}, tc.record, base.access, nil)
			if tc.name == "record run mismatch" {
				records.byRun[tc.requestRun] = tc.record
			}
			err := svc.RevalidateForDispatch(tc.ctx, scope, tc.requestRun)
			if tc.name == "prepared not published" || tc.name == "unknown publication state" || tc.name == "record run mismatch" {
				require.ErrorIs(t, err, craft.ErrConflict)
			} else {
				require.ErrorIs(t, err, craft.ErrForbidden)
			}
			require.Zero(t, publisher.publishCalls)
		})
	}
}

func TestCraftT05RevalidateForDispatchEmptyRecordStillRequiresTaskWrite(t *testing.T) {
	f := newKnowledgeFixture(t, nil)
	scope, runID := craftKnowledgeScope(), "run-dispatch-empty"
	record := t05PublishedKnowledgeRecord(scope, runID)
	record.Empty = true
	svc, _, _ := t05DispatchRevalidator(t, f, &t05Checker{allowed: true}, record, nil, nil)
	require.NoError(t, svc.RevalidateForDispatch(craftKnowledgeCtx(scope), scope, runID))
	viewer, _, _ := t05DispatchRevalidator(t, f, &t05Checker{allowed: true, rejectWrite: true}, record, nil, nil)
	require.ErrorIs(t, viewer.RevalidateForDispatch(craftKnowledgeCtx(scope), scope, runID), craft.ErrForbidden)
}

func TestCraftT05RevalidateForDispatchFailsClosedWhenAuthorityPortsMissing(t *testing.T) {
	scope, runID := craftKnowledgeScope(), "run-dispatch-no-ports"
	for _, tc := range []struct {
		name string
		svc  *CraftKnowledgeService
	}{
		{name: "task access", svc: &CraftKnowledgeService{records: &t05Records{}, access: func(context.Context, uint64, []string) ([]*types.Knowledge, error) { return nil, nil }, search: func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) { return nil, nil }}},
		{name: "records", svc: &CraftKnowledgeService{taskAccess: &t05Checker{allowed: true}, access: func(context.Context, uint64, []string) ([]*types.Knowledge, error) { return nil, nil }, search: func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) { return nil, nil }}},
		{name: "document access", svc: &CraftKnowledgeService{taskAccess: &t05Checker{allowed: true}, records: &t05Records{}, search: func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) { return nil, nil }}},
		{name: "kb search port", svc: &CraftKnowledgeService{taskAccess: &t05Checker{allowed: true}, records: &t05Records{}, access: func(context.Context, uint64, []string) ([]*types.Knowledge, error) { return nil, nil }}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.svc.RevalidateForDispatch(craftKnowledgeCtx(scope), scope, runID), craft.ErrForbidden)
		})
	}
}

func t05PublishedKnowledgeRecord(scope craft.Scope, runID string, sources ...craft.KnowledgeSourceRecord) craft.KnowledgeRecord {
	return craft.KnowledgeRecord{
		Scope: scope, RunID: runID, RequestDigest: "accepted-request-digest", PackageDigest: "accepted-package-digest",
		PublicationState: craft.KnowledgePublicationPublished, Sources: append([]craft.KnowledgeSourceRecord(nil), sources...),
	}
}

func t05DispatchRevalidator(t *testing.T, f *knowledgeFixture, checker craft.TaskAccessChecker, record craft.KnowledgeRecord, access CraftKnowledgeAccess, search CraftKnowledgeSearch) (*CraftKnowledgeService, *t05Records, *t05Publisher) {
	t.Helper()
	base := f.service(t, nil)
	if access == nil {
		access = base.access
	}
	if search == nil {
		search = func(context.Context, string, types.SearchParams) ([]*types.SearchResult, error) {
			return nil, errors.New("dispatch revalidation must not search")
		}
	}
	records := &t05Records{byRun: map[string]craft.KnowledgeRecord{record.RunID: record}}
	publisher := newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: access, Search: search, Writer: f.writer.write,
		TaskAccess: checker, Records: records, Publisher: publisher,
	})
	require.NoError(t, err)
	return svc, records, publisher
}

func TestCraftT05SequentialRunsHaveIsolatedMaterial(t *testing.T) {
	f := newKnowledgeFixture(t, map[string]bool{"kb-a": true})
	f.seedKB(t, "kb-a", 2)
	f.seedKB(t, "kb-b", 1)
	f.seedKnowledge(t, "k-a", "kb-a", 2, "A")
	f.seedKnowledge(t, "k-b", "kb-b", 1, "B")
	f.seedChunk("kb-a", "k-a", "c-a", "run A secret")
	f.seedChunk("kb-b", "k-b", "c-b", "run B material")
	base := f.service(t, nil)
	publisher := newT05Publisher(f.writer)
	svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
		Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
		TaskAccess: &t05Checker{allowed: true}, Records: &t05Records{}, Publisher: publisher,
	})
	require.NoError(t, err)
	scope := craftKnowledgeScope()
	ctx := craftKnowledgeCtx(scope)

	runA, err := svc.BuildForRun(ctx, scope, "run-a", "sales", []string{"k-a"})
	require.NoError(t, err)
	f.shares.allowedKBs["kb-a"] = false // revoke the shared source after Run A
	_, err = svc.BuildForRun(ctx, scope, "run-a-revoked", "sales", []string{"k-a"})
	require.ErrorIs(t, err, craft.ErrForbidden)
	runB, err := svc.BuildForRun(ctx, scope, "run-b", "sales", []string{"k-b"})
	require.NoError(t, err)
	require.Len(t, runA.Sources, 1)
	require.Len(t, runB.Sources, 1)

	files := writePathMap(f.writer)
	runBDir := "knowledge/runs/run-b"
	for path := range files {
		require.NotContains(t, path, "run-a-revoked", "revoked material must not be staged")
	}
	require.Contains(t, string(files[runBDir+"/"+runB.Sources[0].ID+".txt"]), "run B material")
	require.NotContains(t, string(files[runBDir+"/"+runB.Sources[0].ID+".txt"]), "run A secret")
	require.NotContains(t, runBDir, runA.Sources[0].ID)
	for path, content := range files {
		if strings.HasPrefix(path, runBDir+"/") {
			require.NotContains(t, string(content), "run A secret")
		}
	}
	manifestRaw, ok := files[runBDir+"/manifest.json"]
	require.True(t, ok, "the delegate should receive exactly this Run directory")
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(manifestRaw, &manifest))
	require.Len(t, manifestSources(t, manifest), 1)
	require.NotContains(t, string(manifestRaw), "run A secret")
}

func TestCraftT05ExcerptClippingIsBoundedAndDisclosed(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "ascii", content: strings.Repeat("a", craft.MaxKnowledgeExcerptBytes+20)},
		{name: "multibyte", content: strings.Repeat("界", craft.MaxKnowledgeExcerptBytes)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newKnowledgeFixture(t, nil)
			f.seedKB(t, "kb-a", 1)
			f.seedKnowledge(t, "k-a", "kb-a", 1, "A")
			f.seedChunk("kb-a", "k-a", "chunk", tc.content)
			base := f.service(t, nil)
			records := &t05Records{}
			publisher := newT05Publisher(f.writer)
			svc, err := NewCraftKnowledgeService(CraftKnowledgeConfig{
				Store: base.store, Access: base.access, Search: base.search, Writer: f.writer.write,
				TaskAccess: &t05Checker{allowed: true}, Records: records, Publisher: publisher,
			})
			require.NoError(t, err)
			bundle, err := svc.BuildForRun(craftKnowledgeCtx(craftKnowledgeScope()), craftKnowledgeScope(), "run-truncated", "sales", []string{"k-a"})
			require.NoError(t, err)
			require.True(t, bundle.Truncated, "per-source clipping must be disclosed")
			require.LessOrEqual(t, len(bundle.Sources[0].Excerpt), craft.MaxKnowledgeExcerptBytes)
			require.True(t, utf8.ValidString(bundle.Sources[0].Excerpt))
			require.True(t, records.byRun["run-truncated"].Truncated)
			manifest := manifestOf(t, f.writer)
			require.Equal(t, true, manifest["truncated"])
			source := manifestSources(t, manifest)[0]
			require.LessOrEqual(t, int(source["excerpt_bytes"].(float64)), craft.MaxKnowledgeExcerptBytes)
		})
	}
}
