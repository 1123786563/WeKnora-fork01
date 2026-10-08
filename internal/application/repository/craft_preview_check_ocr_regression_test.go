package repository

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

// TestCraftPreviewCheckConcurrentProbeFactsBothSurvive pins the row-lock
// fix: the two probe facts (reachability and page load) arrive from
// independent callbacks; under the old lock-free read-modify-write the
// second writer's unconditional update silently dropped the first's fact.
// With SELECT ... FOR UPDATE both facts survive concurrent recording, and
// a same-name different-outcome rewrite still conflicts (immutability).
func TestCraftPreviewCheckConcurrentProbeFactsBothSurvive(t *testing.T) {
	db := openCraftDB(t)
	versions := NewCraftVersionStore(db)
	checks := NewCraftPreviewCheckStore(db).(*CraftPreviewCheckStore)
	ws := putCraftWorkspace(t, NewCraftStore(db))
	scope := craftTestScope()
	files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}
	published := publishCraftPreviewVersion(t, versions, scope, ws.ID, "run-conc", files)

	// Seed reachability=passed first so the concurrent page-load PASS and a
	// second reachability observation are both admissible.
	_, err := checks.UpdateWebProbeCheck(context.Background(), scope, published.ID, craft.CheckPreviewReachable, craft.WebCheckPassed)
	require.NoError(t, err)

	const writers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			// Half record the page-load fact, half rewrite reachability
			// idempotently — every write must survive or be a no-op.
			// SQLite serializes writers with a whole-database lock, so a
			// concurrent deferred-transaction upgrade can surface a
			// transient "database is locked"; that is an honest refusal,
			// not a silent drop, and the busy_timeout/retry loop is the
			// documented production posture for SQLite deployments.
			name := craft.CheckPreviewReachable
			outcome := craft.WebCheckPassed
			if i%2 == 0 {
				name = craft.CheckPageLoad
			}
			for attempt := 0; attempt < 20; attempt++ {
				_, errs[i] = checks.UpdateWebProbeCheck(context.Background(), scope, published.ID, name, outcome)
				if errs[i] == nil || !strings.Contains(errs[i].Error(), "database is locked") {
					return
				}
				time.Sleep(25 * time.Millisecond)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "writer %d", i)
	}

	stored, err := versions.Get(context.Background(), scope, published.ID)
	require.NoError(t, err)
	evidence := craft.WebEvidenceFromChecks(stored.Checks)
	require.Equal(t, craft.WebCheckPassed, evidence.PreviewReachable, "the reachability fact survives every concurrent writer")
	require.Equal(t, craft.WebCheckPassed, evidence.PageLoaded, "the page-load fact survives every concurrent writer (no silent drop)")

	// Immutability under concurrency: a same-name DIFFERENT outcome is a
	// conflict, never an overwrite.
	_, err = checks.UpdateWebProbeCheck(context.Background(), scope, published.ID, craft.CheckPageLoad, craft.WebCheckFailed)
	require.ErrorIs(t, err, craft.ErrConflict, "a recorded fact cannot be rewritten by a late different outcome")
	storedAfter, err := versions.Get(context.Background(), scope, published.ID)
	require.NoError(t, err)
	require.Equal(t, craft.WebCheckPassed, craft.WebEvidenceFromChecks(storedAfter.Checks).PageLoaded, "the failed rewrite never landed")
}
