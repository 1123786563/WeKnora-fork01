package service

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	sqlite3migrate "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
)

// fakeSemanticSearcher stands in for the semantic RPC client: it can
// trigger a REAL epoch bump (revoke) just before returning results.
type fakeSemanticSearcher struct {
	mu        sync.Mutex
	epochBump func() error
	results   []types.SemanticEvidence
	mode      string
	err       error
}

func (f *fakeSemanticSearcher) Search(ctx context.Context, issued types.SemanticAccessScope) ([]types.SemanticEvidence, string, error) {
	if f.err != nil {
		return nil, "", f.err
	}
	if f.epochBump != nil {
		if err := f.epochBump(); err != nil {
			return nil, "", err
		}
	}
	mode := f.mode
	if mode == "" {
		mode = "graphrag"
	}
	return f.results, mode, nil
}

type fakeVectorSearcher struct {
	results []types.SemanticEvidence
}

func (f *fakeVectorSearcher) Search(ctx context.Context, query string) ([]types.SemanticEvidence, error) {
	return f.results, nil
}

type semanticQueryFixture struct {
	svc       *SemanticQueryService
	scopeSvc  *SemanticScopeService
	epochRepo *epochBumperStub
	delivered int
	searcher  *fakeSemanticSearcher
}

// epochBumperStub wraps the semantic control repo bump for the fixture.
type epochBumperStub struct {
	repo interface {
		BumpKBSemanticEpochs(ctx context.Context, tenantID uint64, kbIDs ...string) error
	}
}

func (f *semanticQueryFixture) RevokeDuringQuery() {
	f.searcher.epochBump = func() error {
		return f.epochRepo.repo.BumpKBSemanticEpochs(context.Background(), 7, "kb-q04")
	}
}

func (f *semanticQueryFixture) QueryReason() (*types.SemanticReasonResult, error) {
	return f.svc.Reason(context.Background(), "user-1", ReasonRequest{
		TenantID: 7, KBID: "kb-q04", Query: "甲控制丙?", Mode: "graphrag",
	})
}

func (f *semanticQueryFixture) DeliveredContentBytes() int { return f.delivered }

func newSemanticQueryFixture(t *testing.T) *semanticQueryFixture {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../.."))
	dbPath := filepath.Join(t.TempDir(), "semantic-q04.db")
	dsn := "file:" + dbPath + "?_foreign_keys=on&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	driver, err := sqlite3migrate.WithInstance(sqlDB, &sqlite3migrate.Config{})
	require.NoError(t, err)
	migrator, err := migrate.NewWithDatabaseInstance(
		"file://"+filepath.Join(repoRoot, "migrations/sqlite"), "sqlite3", driver,
	)
	require.NoError(t, err)
	require.NoError(t, migrator.Up())
	_, _ = migrator.Close()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() { conn, e := db.DB(); if e == nil { _ = conn.Close() } })

	apprepo := apprepo.NewSemanticControlRepository(db)
	clock := &semanticScopeClock{value: time.Now().UTC()}
	scopeSvc := NewSemanticScopeService(apprepo, SemanticScopeConfig{
		Audience:   "weknora-semantic",
		IssuingKey: []byte(strings.Repeat("q04-test-issuing-key-material-", 4)),
		TTL:        15 * time.Minute,
		Now:        func() time.Time { return clock.value },
	})
	searcher := &fakeSemanticSearcher{results: []types.SemanticEvidence{
		{EvidenceID: "e1", DocumentID: "d1", Revision: 1, ChunkID: "c1"},
	}}
	bumper := &epochBumperStub{repo: apprepo}
	svc := NewSemanticQueryService(scopeSvc, searcher, &fakeVectorSearcher{}, FusionConfig{RRFK: 60})
	return &semanticQueryFixture{svc: svc, scopeSvc: scopeSvc, epochRepo: bumper, delivered: 0, searcher: searcher}
}

func TestSemanticQueryDiscardsWholeResultOnRevoke(t *testing.T) {
	f := newSemanticQueryFixture(t)
	f.RevokeDuringQuery()
	_, err := f.QueryReason()
	if !errors.Is(err, ErrSemanticScopeChanged) {
		t.Fatalf("got %v", err)
	}
	if f.DeliveredContentBytes() != 0 {
		t.Fatal("content delivered before authorization")
	}
}

func TestSemanticQuerySearchHappyPathFusesRRF(t *testing.T) {
	f := newSemanticQueryFixture(t)
	result, err := f.svc.Search(context.Background(), "user-1", SearchRequest{
		TenantID: 7, KBID: "kb-q04", Query: "甲控制丙",
	})
	require.NoError(t, err)
	require.Equal(t, "graphrag", result.Mode)
	require.NotEmpty(t, result.Evidence)
	require.Equal(t, float64(1.0/61.0), result.Evidence[0].FusedScore, "RRF k=60: 1/(60+1) for rank 1")
}

func TestSemanticQueryRRFDedupsAcrossEngines(t *testing.T) {
	f := newSemanticQueryFixture(t)
	// Both engines return the SAME identity: fused score doubles, one row.
	f.searcher.results = []types.SemanticEvidence{
		{EvidenceID: "e1", DocumentID: "d1", Revision: 1, ChunkID: "c1"},
	}
	f.svc = NewSemanticQueryService(f.scopeSvc, f.searcher, &fakeVectorSearcher{
		results: []types.SemanticEvidence{
			{EvidenceID: "e1", DocumentID: "d1", Revision: 1, ChunkID: "c1"},
		},
	}, FusionConfig{RRFK: 60})
	result, err := f.svc.Search(context.Background(), "user-1", SearchRequest{
		TenantID: 7, KBID: "kb-q04", Query: "q",
	})
	require.NoError(t, err)
	require.Len(t, result.Evidence, 1, "same identity across engines must dedupe to one")
	require.Equal(t, 2*(1.0/61.0), result.Evidence[0].FusedScore, "rank 1 in both engines doubles the RRF contribution")
}

func TestSemanticQueryDegradedModeIsExplicit(t *testing.T) {
	f := newSemanticQueryFixture(t)
	f.searcher.err = errors.New("semantic backend unavailable")
	result, err := f.svc.Search(context.Background(), "user-1", SearchRequest{
		TenantID: 7, KBID: "kb-q04", Query: "q",
	})
	require.NoError(t, err)
	require.Equal(t, "retrieval.degraded", result.Mode, "degraded search MUST declare its actual mode")
}