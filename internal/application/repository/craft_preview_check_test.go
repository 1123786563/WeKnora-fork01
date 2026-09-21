package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/craft"
	"github.com/stretchr/testify/require"
)

// publishCraftPreviewVersion publishes a web version whose checks come from
// the real collector path (BuildChecks with zero evidence → preview not_run).
func publishCraftPreviewVersion(t *testing.T, store craft.VersionStore, scope craft.Scope, wsID, runID string, files []craft.File) craft.Version {
	t.Helper()
	digest := mustDigest(t, files)
	v, err := store.Publish(context.Background(), scope, craft.Version{
		ID:          craft.VersionID(wsID, runID, digest),
		WorkspaceID: wsID, RunID: runID, Kind: craft.KindWeb,
		Files:  files,
		Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{}),
	})
	require.NoError(t, err)
	return v
}

// previewCheckOf extracts one named check from a version.
func previewCheckOf(t *testing.T, v craft.Version, name string) craft.Check {
	t.Helper()
	for _, c := range v.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("version %s has no %q check", v.ID, name)
	return craft.Check{}
}

// TestCraftPreviewCheckUpdateRewritesOnlyPreview pins the W02 update channel:
// the recorded preview verdict replaces exactly the preview entry, under the
// same scope ACL as a version read.
func TestCraftPreviewCheckUpdateRewritesOnlyPreview(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			versions := NewCraftVersionStore(db)
			checks := NewCraftPreviewCheckStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()
			files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}

			published := publishCraftPreviewVersion(t, versions, scope, ws.ID, "run-1", files)
			require.Equal(t, craft.CheckNotRun, previewCheckOf(t, published, craft.CheckPreview).Status)

			updated, err := checks.UpdatePreviewCheck(context.Background(), scope, published.ID,
				craft.PreviewCheckFromEvidence(true, true))
			require.NoError(t, err)
			require.Equal(t, craft.CheckPassed, previewCheckOf(t, updated, craft.CheckPreview).Status)

			// Build/entry checks and the manifest survive untouched.
			stored, err := versions.Get(context.Background(), scope, published.ID)
			require.NoError(t, err)
			require.Equal(t, previewCheckOf(t, published, craft.CheckBuild), previewCheckOf(t, stored, craft.CheckBuild))
			require.Equal(t, previewCheckOf(t, published, craft.CheckEntry), previewCheckOf(t, stored, craft.CheckEntry))
			require.Equal(t, published.Files, stored.Files)
			require.Equal(t, published.ID, stored.ID)

			// Cross-tenant scope does not see the version at all.
			foreign := scope
			foreign.TenantID = scope.TenantID + 1
			_, err = checks.UpdatePreviewCheck(context.Background(), foreign, published.ID,
				craft.PreviewCheckFromEvidence(true, true))
			require.ErrorIs(t, err, craft.ErrNotFound)

			// The channel refuses to touch any other check.
			_, err = checks.UpdatePreviewCheck(context.Background(), scope, published.ID,
				craft.Check{Name: craft.CheckBuild, Status: craft.CheckFailed})
			require.ErrorIs(t, err, craft.ErrInvalidInput)
		})
	}
}

// TestCraftPreviewCheckUpdateKeepsPublishIdempotent pins the two-channel
// consistency W01's review demanded: after the preview verdict is recorded,
// re-collecting the SAME content through the evidence-injected collector
// derives the same checks and Publish adopts the stored row instead of
// conflicting.
func TestCraftPreviewCheckUpdateKeepsPublishIdempotent(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			db := openCraftDB(t)
			versions := NewCraftVersionStore(db)
			checks := NewCraftPreviewCheckStore(db)
			ws := putCraftWorkspace(t, NewCraftStore(db))
			scope := craftTestScope()
			files := []craft.File{craftTestFile(t, "index.html", "<h1>v1</h1>")}

			published := publishCraftPreviewVersion(t, versions, scope, ws.ID, "run-1", files)
			_, err := checks.UpdatePreviewCheck(context.Background(), scope, published.ID,
				craft.PreviewCheckFromEvidence(true, true))
			require.NoError(t, err)

			// A nil-evidence replay would conflict — that is W01's design.
			digest := mustDigest(t, files)
			_, err = versions.Publish(context.Background(), scope, craft.Version{
				ID: craft.VersionID(ws.ID, "run-1", digest), WorkspaceID: ws.ID,
				RunID: "run-1", Kind: craft.KindWeb, Files: files,
				Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{}),
			})
			require.ErrorIs(t, err, craft.ErrConflict)

			// The evidence-fed replay (PreviewRan/PreviewPassed carried over)
			// adopts the stored row verbatim.
			replayed, err := versions.Publish(context.Background(), scope, craft.Version{
				ID: craft.VersionID(ws.ID, "run-1", digest), WorkspaceID: ws.ID,
				RunID: "run-1", Kind: craft.KindWeb, Files: files,
				Checks: craft.BuildChecks(craft.KindWeb, files, craft.ArtifactEvidence{PreviewRan: true, PreviewPassed: true}),
			})
			require.NoError(t, err)
			require.Equal(t, published.ID, replayed.ID)
			require.Equal(t, craft.CheckPassed, previewCheckOf(t, replayed, craft.CheckPreview).Status)

			list, err := versions.List(context.Background(), scope)
			require.NoError(t, err)
			require.Len(t, list, 1)

			// Unknown version id answers not found, not a silent no-op.
			_, err = checks.UpdatePreviewCheck(context.Background(), scope,
				craft.VersionID(ws.ID, "run-x", digest), craft.PreviewCheckFromEvidence(true, true))
			require.True(t, errors.Is(err, craft.ErrNotFound))
		})
	}
}
