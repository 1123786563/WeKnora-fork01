package career

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testOffice(t *testing.T) (*Office, context.Context) {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, e)
	o, e := NewOffice(db)
	require.NoError(t, e)
	require.NoError(t, db.AutoMigrate(&types.StoredResource{}, &types.ResourceBinding{}))
	ctx := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 1})
	require.NoError(t, o.ClaimSpace(ctx))
	return o, ctx
}
func TestUnconfirmedProposalIsNotConfirmedFact(t *testing.T) {
	o, ctx := testOffice(t)
	r, e := o.Propose(ctx, "graduation_year", "2027", "req-1", 0, Source{Kind: "resume_extraction", ReferenceID: "file-1"})
	require.NoError(t, e)
	require.Equal(t, "proposed", r.Kind)
	require.NotEmpty(t, r.Proposal.ID)
	v, e := o.Open(ctx)
	require.NoError(t, e)
	require.Empty(t, v.Facts)
	require.Len(t, v.Proposals, 1)
}
func TestConfirmProposalResolvesAndRetainsConfirmationHistory(t *testing.T) {
	o, ctx := testOffice(t)
	p, e := o.Propose(ctx, "graduation_year", "2027", "p1", 0, Source{Kind: "resume_extraction", ReferenceID: "file-1"})
	require.NoError(t, e)
	confirmed, e := o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 1, Source{Kind: "user_confirmation"})
	require.NoError(t, e)
	require.Equal(t, "confirmed", confirmed.Kind)
	replayed, e := o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 1, Source{Kind: "user_confirmation"})
	require.NoError(t, e)
	confirmedJSON, err := json.Marshal(confirmed)
	require.NoError(t, err)
	replayedJSON, err := json.Marshal(replayed)
	require.NoError(t, err)
	require.JSONEq(t, string(confirmedJSON), string(replayedJSON))
	_, e = o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 1, Source{Kind: "edited_confirmation"})
	require.ErrorIs(t, e, ErrIdempotencyConflict)
	_, e = o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 2, Source{Kind: "user_confirmation"})
	require.ErrorIs(t, e, ErrIdempotencyConflict)
	view, e := o.Open(ctx)
	require.NoError(t, e)
	require.Len(t, view.Facts, 1)
	require.Empty(t, view.Proposals)
	require.Equal(t, "resume_extraction", view.Facts[0].Source.Kind)
	require.Equal(t, "u1", view.Facts[0].Confirmation.UserID)
	history, e := o.History(ctx, "graduation_year")
	require.NoError(t, e)
	require.Len(t, history, 1)
	require.Equal(t, "file-1", history[0].Source.ReferenceID)
	require.Equal(t, uint64(2), history[0].Revision)
	again, e := o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c2", 2, Source{Kind: "user_confirmation"})
	require.ErrorIs(t, e, ErrProposalResolved)
	_ = again
}
func TestDismissProposalAndChangesAreDurableOrderedEvents(t *testing.T) {
	o, ctx := testOffice(t)
	p, e := o.Propose(ctx, "degree", "bachelor", "p1", 0, Source{Kind: "interview"})
	require.NoError(t, e)
	_, e = o.Act(ctx, "dismiss", p.Proposal.ID, "", "", "d1", 1, Source{Kind: "user_confirmation"})
	require.NoError(t, e)
	set, e := o.Changes(ctx, 0)
	require.NoError(t, e)
	require.Equal(t, uint64(2), set.Revision)
	require.Len(t, set.Changes, 2)
	require.Equal(t, uint64(1), set.Changes[0].Revision)
	require.Equal(t, "proposed", set.Changes[0].Kind)
	require.Equal(t, "dismissed", set.Changes[1].Kind)
	require.Equal(t, "u1", set.Changes[1].Proposal.Confirmation.UserID)
	require.Equal(t, "user_confirmation", set.Changes[1].Proposal.ResolutionSource.Kind)
	require.Equal(t, set.Revision, set.Changes[len(set.Changes)-1].Revision)
	view, e := o.Open(ctx)
	require.NoError(t, e)
	require.Empty(t, view.Proposals)
}
func TestConfirmedFactVersionsAreImmutable(t *testing.T) {
	o, ctx := testOffice(t)
	_, e := o.Confirm(ctx, "graduation_year", "2027", "c1", 0, Source{Kind: "user"})
	require.NoError(t, e)
	_, e = o.Confirm(ctx, "graduation_year", "2028", "c2", 1, Source{Kind: "user"})
	require.NoError(t, e)
	history, e := o.History(ctx, "graduation_year")
	require.NoError(t, e)
	require.Len(t, history, 2)
	require.Equal(t, "2027", history[0].Value)
	require.Equal(t, "2028", history[1].Value)
	changes, err := o.Changes(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(2), changes.Revision)
	require.Len(t, changes.Changes, 2)
	secondClientChanges, err := o.Changes(ctx, changes.Changes[0].Revision)
	require.NoError(t, err)
	require.Len(t, secondClientChanges.Changes, 1)
	require.Equal(t, "2028", secondClientChanges.Changes[0].Fact.Value)
	require.Equal(t, secondClientChanges.Revision, secondClientChanges.Changes[0].Revision)
	_, err = o.Changes(ctx, 3)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, uint64(2), conflict.CurrentRevision)
}
func TestRevisionConflictHasCurrentValueAndIdempotency(t *testing.T) {
	o, ctx := testOffice(t)
	r, e := o.Confirm(ctx, "degree", "bachelor", "r1", 0, Source{Kind: "user"})
	require.NoError(t, e)
	again, e := o.Confirm(ctx, "degree", "bachelor", "r1", 0, Source{Kind: "user"})
	require.NoError(t, e)
	require.Equal(t, r.Kind, again.Kind)
	_, e = o.Confirm(ctx, "degree", "master", "r1", 0, Source{Kind: "user"})
	require.ErrorIs(t, e, ErrIdempotencyConflict)
	_, e = o.Confirm(ctx, "city", "Beijing", "r2", 0, Source{Kind: "user"})
	var conflict *RevisionConflictError
	require.ErrorAs(t, e, &conflict)
	require.Equal(t, uint64(1), conflict.CurrentRevision)
}

func TestPostConflictReceiptLookupDeadlineReturnsUnknownWireError(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "degree", "bachelor", "winner", 0, Source{Kind: "user"})
	require.NoError(t, err)
	entered := make(chan struct{})
	o.beforeReplayReceipt = func(lookupCtx context.Context) {
		close(entered)
		<-lookupCtx.Done()
	}
	deadlineCtx, cancel := context.WithTimeout(ctx, 80*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, mutationErr := o.Confirm(deadlineCtx, "city", "Beijing", "loser", 0, Source{Kind: "user"})
		result <- mutationErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("mutation did not reach post-conflict receipt lookup")
	}
	var mutationErr error
	select {
	case mutationErr = <-result:
	case <-time.After(time.Second):
		t.Fatal("post-conflict lookup did not obey deadline")
	}
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, mutationErr, &unknown)
	require.Equal(t, "loser", unknown.RequestID)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	writeError(c, mutationErr)
	require.Equal(t, 504, w.Code)
	require.JSONEq(t, `{"error":{"code":"outcome_unknown","message":"career request outcome unknown","requestId":"loser"}}`, w.Body.String())
}

func TestPostConflictReceiptLookupCancellationReturnsUnknown(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "degree", "bachelor", "winner", 0, Source{Kind: "user"})
	require.NoError(t, err)
	entered := make(chan struct{})
	o.beforeReplayReceipt = func(lookupCtx context.Context) {
		close(entered)
		<-lookupCtx.Done()
	}
	canceledCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, mutationErr := o.Confirm(canceledCtx, "city", "Beijing", "loser", 0, Source{Kind: "user"})
		result <- mutationErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("mutation did not reach post-conflict receipt lookup")
	}
	cancel()
	select {
	case mutationErr := <-result:
		var unknown *OutcomeUnknownError
		require.ErrorAs(t, mutationErr, &unknown)
		require.Equal(t, "loser", unknown.RequestID)
	case <-time.After(time.Second):
		t.Fatal("post-conflict lookup did not obey cancellation")
	}
}

func TestPostConflictReceiptLookupPreservesUnrelatedDatabaseError(t *testing.T) {
	o, ctx := testOffice(t)
	_, err := o.Confirm(ctx, "degree", "bachelor", "winner", 0, Source{Kind: "user"})
	require.NoError(t, err)
	var dropErr error
	o.beforeReplayReceipt = func(context.Context) {
		dropErr = o.db.Exec("DROP TABLE career_receipts").Error
	}
	_, err = o.Confirm(ctx, "city", "Beijing", "loser", 0, Source{Kind: "user"})
	require.NoError(t, dropErr)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrOutcomeUnknown)
	require.ErrorContains(t, err, "no such table")
}

func TestSlowConcurrentMutationAcrossSQLiteConnections(t *testing.T) {
	testSlowConcurrentMutation(t, func(t *testing.T) func() *gorm.DB {
		dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "career.db")) + "?_busy_timeout=5000&_journal_mode=WAL"
		return func() *gorm.DB {
			db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			return db
		}
	}, true)
}

func TestSlowConcurrentMutationAcrossPostgresConnections(t *testing.T) {
	dsn := os.Getenv("CAREER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("requires isolated CAREER_TEST_POSTGRES_DSN")
	}
	testSlowConcurrentMutation(t, func(t *testing.T) func() *gorm.DB {
		return func() *gorm.DB {
			db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			return db
		}
	}, false)
}

func testSlowConcurrentMutation(t *testing.T, makeOpen func(*testing.T) func() *gorm.DB, sqliteDB bool) {
	for index, scenario := range []struct {
		name, loserID, loserValue string
		deadline                  bool
	}{
		{"identical replay", "same", "bachelor", false},
		{"changed same ID", "same", "master", false},
		{"distinct ID stale revision", "other", "master", false},
		{"deadline then receipt", "same", "bachelor", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			open := makeOpen(t)
			db1 := open()
			o1, err := NewOffice(db1)
			require.NoError(t, err)
			tenantID := uint64(time.Now().UnixNano())
			ctx := WithScope(context.Background(), Scope{UserID: fmt.Sprintf("career-test-u%d-%d", index, tenantID), TenantID: tenantID})
			require.NoError(t, o1.ClaimSpace(ctx))
			db2 := open()
			o2, err := NewOffice(db2)
			require.NoError(t, err)
			t.Cleanup(func() {
				if raw, e := db1.DB(); e == nil {
					_ = raw.Close()
				}
				if raw, e := db2.DB(); e == nil {
					_ = raw.Close()
				}
			})

			winnerPersisted := make(chan struct{})
			releaseWinner := make(chan struct{})
			t.Cleanup(func() {
				select {
				case <-releaseWinner:
				default:
					close(releaseWinner)
				}
			})
			o1.afterReceiptPersist = func() { close(winnerPersisted); <-releaseWinner }
			loserStarted := make(chan struct{})
			var started sync.Once
			o2.beforeFirstWrite = func() { started.Do(func() { close(loserStarted) }) }
			type result struct {
				receipt Receipt
				err     error
			}
			winnerResult := make(chan result, 1)
			go func() {
				r, e := o1.Confirm(ctx, "degree", "bachelor", "same", 0, Source{Kind: "user"})
				winnerResult <- result{r, e}
			}()
			select {
			case <-winnerPersisted:
			case <-time.After(2 * time.Second):
				t.Fatal("winner did not persist receipt")
			}
			loserCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if scenario.deadline {
				cancel()
				loserCtx, cancel = context.WithTimeout(ctx, 80*time.Millisecond)
			}
			defer cancel()
			loserResult := make(chan result, 1)
			go func() {
				r, e := o2.Confirm(loserCtx, "degree", scenario.loserValue, scenario.loserID, 0, Source{Kind: "user"})
				loserResult <- result{r, e}
			}()
			select {
			case <-loserStarted:
			case <-time.After(2 * time.Second):
				t.Fatal("loser did not reach write gate")
			}
			if scenario.deadline {
				select {
				case got := <-loserResult:
					require.ErrorIs(t, got.err, ErrOutcomeUnknown)
				case <-time.After(time.Second):
					t.Fatal("deadline did not bound loser")
				}
			} else {
				select {
				case got := <-loserResult:
					t.Fatalf("loser completed before slow winner: %+v", got)
				case <-time.After(260 * time.Millisecond):
				}
			}
			close(releaseWinner)
			winner := <-winnerResult
			require.NoError(t, winner.err)
			if !scenario.deadline {
				loser := <-loserResult
				switch scenario.name {
				case "identical replay":
					require.NoError(t, loser.err)
					j1, _ := json.Marshal(winner.receipt)
					j2, _ := json.Marshal(loser.receipt)
					require.JSONEq(t, string(j1), string(j2))
				case "changed same ID":
					require.ErrorIs(t, loser.err, ErrIdempotencyConflict)
				case "distinct ID stale revision":
					var conflict *RevisionConflictError
					require.ErrorAs(t, loser.err, &conflict)
					require.Equal(t, uint64(1), conflict.CurrentRevision)
				}
			}
			stored, err := o2.Receipt(ctx, "same")
			require.NoError(t, err)
			j1, _ := json.Marshal(winner.receipt)
			j2, _ := json.Marshal(stored)
			require.JSONEq(t, string(j1), string(j2))
			view, err := o1.Open(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(1), view.Revision)
			history, err := o1.History(ctx, "degree")
			require.NoError(t, err)
			require.Len(t, history, 1)
			changes, err := o1.Changes(ctx, 0)
			require.NoError(t, err)
			require.Len(t, changes.Changes, 1)
			if sqliteDB {
				var busyTimeout int
				require.NoError(t, db2.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error)
				require.Equal(t, 5000, busyTimeout)
			}
		})
	}
}
func TestOfficeRejectsUnclaimedOrCrossOwnerCareerSpace(t *testing.T) {
	o, _ := testOffice(t)
	other := WithScope(context.Background(), Scope{UserID: "u2", TenantID: 1})
	_, e := o.Open(other)
	require.ErrorIs(t, e, ErrUnauthorized)
	require.ErrorIs(t, o.ClaimSpace(other), ErrUnauthorized)
	tenant := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 2})
	_, e = o.Open(tenant)
	require.ErrorIs(t, e, ErrUnauthorized)
}

func TestCareerFactsStayIsolatedAcrossUsersAndTenants(t *testing.T) {
	o, ctx := testOffice(t)
	other := WithScope(context.Background(), Scope{UserID: "u2", TenantID: 2})
	require.NoError(t, o.ClaimSpace(other))
	_, err := o.Confirm(other, "degree", "master", "other-fact", 0, Source{Kind: "user"})
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Empty(t, view.Facts)
	otherView, err := o.Open(other)
	require.NoError(t, err)
	require.Len(t, otherView.Facts, 1)
}

// Oversized keys and request IDs must be refused as invalid_request before
// any write: career_proposals.key, career_facts.key and
// career_receipts.request_id are VARCHAR(128), and an oversized value would
// otherwise surface as an untyped 500 on PostgreSQL while SQLite silently
// accepts it (dialect divergence, ocr1-145).
func TestProfileActionsRefuseOversizedKeysAndRequestIDs(t *testing.T) {
	o, ctx := testOffice(t)
	longKey := strings.Repeat("k", 129)
	longRequestID := strings.Repeat("r", 129)

	_, err := o.Propose(ctx, longKey, "Go", "len-1", 0, Source{Kind: "manual"})
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = o.Propose(ctx, "skill.go", "Go", longRequestID, 0, Source{Kind: "manual"})
	require.ErrorIs(t, err, ErrInvalidRequest)

	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, longKey, "Go", "len-2", view.Revision, Source{Kind: "manual"})
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = o.Confirm(ctx, "skill.go", "Go", longRequestID, view.Revision, Source{Kind: "manual"})
	require.ErrorIs(t, err, ErrInvalidRequest)

	// Boundary values stay accepted.
	edgeKey := strings.Repeat("k", 128)
	edgeRequestID := strings.Repeat("r", 128)
	_, err = o.Propose(ctx, edgeKey, "Go", edgeRequestID, 0, Source{Kind: "manual"})
	require.NoError(t, err)

	// Padded request IDs normalize to one durable receipt (TrimSpace throat).
	padded, err := o.Confirm(ctx, "skill.go", "Go", " padded-1 ", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	require.Equal(t, "padded-1", padded.RequestID)
	replayed, err := o.Confirm(ctx, "skill.go", "Go", "padded-1", 1, Source{Kind: "manual"})
	require.NoError(t, err)
	require.Equal(t, padded.Revision, replayed.Revision)
}
