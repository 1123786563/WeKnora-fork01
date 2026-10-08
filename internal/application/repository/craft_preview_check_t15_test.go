package repository

// T15 (#130) — the web-gate probe channel on real stores. Pins:
//   - preview_reachable and page_load are recorded independently
//   - page load cannot pass before reachability passed (no substitution)
//   - facts are immutable: identical rewrites are no-ops, different facts conflict
//   - the four-check promotion evidence persists through the real version
//     store and derives back identically
import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

func webCheckOf(t *testing.T, v craft.Version, name string) craft.Check {
	t.Helper()
	for _, c := range v.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("version %s has no %q check", v.ID, name)
	return craft.Check{}
}

// TestCraftPreviewCheckUpdateWebProbeRecordsIndependently pins the channel's
// core contract: one call records one fact, the other fact stays untouched.
func TestCraftPreviewCheckUpdateWebProbeRecordsIndependently(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			versions := NewCraftVersionStore(db)
			checks := NewCraftPreviewCheckStore(db).(*CraftPreviewCheckStore)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()
			files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}

			published := publishCraftPreviewVersion(t, versions, scope, ws.ID, "run-1", files)

			// Reachability passes first; the page-load fact is still absent
			// (not_run when derived).
			updated, err := checks.UpdateWebProbeCheck(context.Background(), scope, published.ID, craft.CheckPreviewReachable, craft.WebCheckPassed)
			require.NoError(t, err)
			require.Equal(t, craft.CheckPassed, webCheckOf(t, updated, craft.CheckPreviewReachable).Status)
			derived := craft.WebEvidenceFromChecks(updated.Checks)
			require.Equal(t, craft.WebCheckPassed, derived.PreviewReachable)
			require.Equal(t, craft.WebCheckNotRun, derived.PageLoaded, "recording reachability never records the load")

			// The page load then fails, independently: reachability stays passed.
			updated, err = checks.UpdateWebProbeCheck(context.Background(), scope, published.ID, craft.CheckPageLoad, craft.WebCheckFailed)
			require.NoError(t, err)
			require.Equal(t, craft.CheckFailed, webCheckOf(t, updated, craft.CheckPageLoad).Status)
			require.Equal(t, craft.CheckPassed, webCheckOf(t, updated, craft.CheckPreviewReachable).Status, "the load fact never rewrites reachability")

			// Build/entry checks and the manifest survive untouched.
			stored, err := versions.Get(context.Background(), scope, published.ID)
			require.NoError(t, err)
			require.Equal(t, webCheckOf(t, published, craft.CheckBuild), webCheckOf(t, stored, craft.CheckBuild))
			require.Equal(t, webCheckOf(t, published, craft.CheckEntry), webCheckOf(t, stored, craft.CheckEntry))
			require.Equal(t, published.Files, stored.Files)

			// Cross-tenant scope does not see the version at all.
			foreign := scope
			foreign.TenantID = scope.TenantID + 1
			_, err = checks.UpdateWebProbeCheck(context.Background(), foreign, published.ID, craft.CheckPageLoad, craft.WebCheckPassed)
			require.ErrorIs(t, err, craft.ErrNotFound)
		})
	}
}

// TestCraftPreviewCheckUpdateWebProbeRefusesSubstitutionAndRewrites pins the
// substitution and immutability rules.
func TestCraftPreviewCheckUpdateWebProbeRefusesSubstitutionAndRewrites(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			versions := NewCraftVersionStore(db)
			checks := NewCraftPreviewCheckStore(db).(*CraftPreviewCheckStore)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()
			files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}
			// A collected round with a successful build (T04 evidence): the
			// probe facts are the two missing pieces of the four-check gate.
			digest := mustDigest(t, files)
			published, err := versions.Publish(context.Background(), scope, craft.Version{
				ID: craft.VersionID(ws.ID, "run-1", digest), WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb,
				Files:  files,
				Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{ClaimedSuccess: true, BuildRan: true, BuildExitCode: 0}),
			})
			require.NoError(t, err)
			ctx := context.Background()

			// page_load=passed while reachability never passed: refused.
			_, perr := checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPageLoad, craft.WebCheckPassed)
			require.ErrorIs(t, perr, craft.ErrInvalidInput, "the load fact cannot precede reachability")

			// Reachability passes, then load passes — the honest sequence.
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPreviewReachable, craft.WebCheckPassed)
			require.NoError(t, err)
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPageLoad, craft.WebCheckPassed)
			require.NoError(t, err)

			// Identical rewrites are idempotent no-ops.
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPageLoad, craft.WebCheckPassed)
			require.NoError(t, err)

			// A different fact under the same name is a conflict: recorded
			// facts are immutable.
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPageLoad, craft.WebCheckFailed)
			require.ErrorIs(t, err, craft.ErrConflict)

			// Reachability cannot be withdrawn while a load passed.
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPreviewReachable, craft.WebCheckFailed)
			require.ErrorIs(t, err, craft.ErrConflict)

			// Unknown check names and outcomes are refused.
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPreview, craft.WebCheckPassed)
			require.ErrorIs(t, err, craft.ErrInvalidInput)
			_, err = checks.UpdateWebProbeCheck(ctx, scope, published.ID, craft.CheckPageLoad, craft.CheckOutcome("maybe"))
			require.ErrorIs(t, err, craft.ErrInvalidInput)

			// After the full pass sequence the recorded evidence is Ready.
			stored, err := versions.Get(ctx, scope, published.ID)
			require.NoError(t, err)
			require.True(t, craft.WebEvidenceFromChecks(stored.Checks).Ready(), "four passed facts derive a ready evidence")
			_, ok := craft.SelectDefaultVersion([]craft.Version{stored})
			require.True(t, ok, "the four-check ready version is selectable as the default")
		})
	}
}

// TestCraftWebPromotionEvidencePersistsThroughVersionStore round-trips the
// T15 promotion publish through the real version store: the four checks and
// their run/revision/version binding survive a reload, and the identical
// replay adopts the stored row (duplicate callbacks create no duplicates).
func TestCraftWebPromotionEvidencePersistsThroughVersionStore(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			versions := NewCraftVersionStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()
			files := []craft.File{craftTestFile(t, "index.html", "<h1>promo</h1>")}
			digest := mustDigest(t, files)

			record := craft.WebPromotionRecord{
				RunID: "run-1", Revision: 3,
				VersionID:        craft.VersionID(ws.ID, "run-1", digest),
				WebCheckEvidence: craft.WebCheckEvidence{Build: craft.WebCheckPassed, Entry: craft.WebCheckPassed, PreviewReachable: craft.WebCheckPassed, PageLoaded: craft.WebCheckPassed},
			}
			require.NoError(t, record.Validate())
			published, err := versions.Publish(context.Background(), scope, craft.Version{
				ID: record.VersionID, WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb,
				Files: files, Checks: craft.WebChecks(record),
			})
			require.NoError(t, err)

			stored, err := versions.Get(context.Background(), scope, published.ID)
			require.NoError(t, err)
			require.Equal(t, craft.WebChecks(record), stored.Checks, "the four bound checks persist verbatim")
			require.Equal(t, record.WebCheckEvidence, craft.WebEvidenceFromChecks(stored.Checks))
			for _, c := range stored.Checks {
				require.Contains(t, c.Detail, "run run-1")
				require.Contains(t, c.Detail, "revision 3")
				require.Contains(t, c.Detail, record.VersionID)
			}

			// The identical replay adopts the stored row — idempotent.
			replay, err := versions.Publish(context.Background(), scope, craft.Version{
				ID: record.VersionID, WorkspaceID: ws.ID, RunID: "run-1", Kind: craft.KindWeb,
				Files: files, Checks: craft.WebChecks(record),
			})
			require.NoError(t, err)
			require.Equal(t, stored.ID, replay.ID)
			list, err := versions.List(context.Background(), scope)
			require.NoError(t, err)
			require.Len(t, list, 1, "the replay created no duplicate version")
		})
	}
}
