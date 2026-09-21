package service

// CFT-S03-T019: the immutable-publish chain pins. The collection rules
// (traversal/symlink/credential/oversize abort with ZERO publishes) and the
// v1-hash-unchanged-after-v2 fact are pinned by TestCraftArtifactCollect
// RejectsHostileOutput and PinsImmutableVersions; these suites add the two
// missing links:
//   1. a FAILED admission check (a document round without its manifest.json)
//      publishes no version at all — the gate fires before any upload
//   2. objects uploaded SUCCESSFULLY whose publish transaction then fails
//      leave clean staging (reclaimable by O03) and no visible version
import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/modules/execution/sandbox"
	"github.com/stretchr/testify/require"
)

// A round whose kind demands a manifest.json but ships none fails admission
// BEFORE any upload or publish: no version, and nothing staged.
func TestCraftArtifactPublishAdmissionFailurePublishesNothing(t *testing.T) {
	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	content := []byte("<h1>doc round without manifest</h1>")
	source := craftSourceWith(
		[]sandbox.RemoteDirEntry{craftEntry(t, "report.md", string(content))},
		map[string][]byte{craftTestOutputDir + "/report.md": content},
	)
	svc := newCraftArtifactService(source, files, versions)

	// The SESSION kind drives admission: report.md alone is not a valid
	// document deliverable (no manifest.json, no exported docx path).
	task := craftArtifactTask("s1", "ws-doc", "run-doc")
	_, err := svc.CollectForKind(context.Background(), task, "document")
	require.Error(t, err, "document admission must reject a manifest-less round")
	require.Zero(t, versions.publishes, "a failed check publishes no version")
	require.Zero(t, files.saves, "admission fires before any upload — nothing to clean")
	list, err := versions.List(context.Background(), task.Scope)
	require.NoError(t, err)
	require.Empty(t, list)
}

// Objects that uploaded fine but whose PUBLISH then fails leave staging
// objects (reclaimable by the O03 deferred sweep) and no visible version —
// never a half-published round.
func TestCraftArtifactPublishTxnFailureLeavesStagingNotVersion(t *testing.T) {
	files := newDirBackedFileService(t)
	versions := newMemVersionStore()
	versions.failPublish = true
	content := []byte("<h1>txn doomed</h1>")
	source := craftSourceWith(
		[]sandbox.RemoteDirEntry{craftEntry(t, "index.html", string(content))},
		map[string][]byte{craftTestOutputDir + "/index.html": content},
	)
	svc := newCraftArtifactService(source, files, versions)

	task := craftArtifactTask("s1", "ws-txn", "run-txn")
	_, err := svc.Collect(context.Background(), task)
	require.Error(t, err, "the publish transaction failed")
	require.Equal(t, 1, files.saves, "the object DID upload — clean staging exists for reclamation")
	require.Zero(t, versions.publishes, "no version became visible")
	list, err := versions.List(context.Background(), task.Scope)
	require.NoError(t, err)
	require.Empty(t, list, "the half-finished round is not listable")

	// The staged object is content-addressed and reusable: the retry of the
	// SAME content publishes cleanly once the store accepts again.
	versions.failPublish = false
	published, err := svc.Collect(context.Background(), task)
	require.NoError(t, err)
	require.NotEmpty(t, published.ID)
}
