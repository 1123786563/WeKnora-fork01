package container

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	"github.com/stretchr/testify/require"
)

type publisherTestManifest struct {
	Kind       string                `json:"kind"`
	DataNotice string                `json:"data_notice"`
	Scope      publisherTestScope    `json:"scope"`
	Query      string                `json:"query"`
	Truncated  bool                  `json:"truncated"`
	Empty      bool                  `json:"empty"`
	Sources    []publisherTestSource `json:"sources"`
}

type publisherTestScope struct {
	TenantID  uint64 `json:"tenant_id"`
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
}

type publisherTestSource struct {
	CitationID      string    `json:"citation_id"`
	Ref             string    `json:"ref"`
	KnowledgeID     string    `json:"knowledge_id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	TenantID        uint64    `json:"tenant_id"`
	ExcerptBytes    int       `json:"excerpt_bytes"`
	Digest          string    `json:"digest"`
	AcquiredAt      time.Time `json:"acquired_at"`
	File            string    `json:"file"`
}

func publisherTestRoot(t *testing.T) string {
	t.Helper()
	tempRoot := t.TempDir()
	root, err := filepath.EvalSymlinks(tempRoot)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = filepath.Walk(root, func(name string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			mode := os.FileMode(0700)
			if !info.IsDir() {
				mode = 0600
			}
			_ = os.Chmod(name, mode)
			return nil
		})
	})
	return root
}

func publisherTestWorkspace() craft.Workspace {
	return craft.Workspace{ID: "workspace-test", Scope: craft.Scope{TenantID: 1, UserID: "owner", SessionID: "session"}}
}

func publisherTestPackage(t *testing.T, runID string, excerpts ...string) service.CraftKnowledgeMaterialPackage {
	t.Helper()
	directory := craft.KnowledgeRunDir(runID)
	require.NotEmpty(t, directory)
	manifest := publisherTestManifest{
		Kind: "craft.knowledge.manifest", DataNotice: craft.KnowledgeDataNotice,
		Scope: publisherTestScope{TenantID: 1, UserID: "owner", SessionID: "session"},
		Query: "test query", Empty: len(excerpts) == 0, Sources: []publisherTestSource{},
	}
	files := make(map[string][]byte, len(excerpts)+1)
	for i, excerpt := range excerpts {
		chunkID := fmt.Sprintf("chunk-%d", i)
		citationID := craft.KnowledgeCitationID("kb-a", "knowledge-a", chunkID)
		ref := craft.KnowledgeRef("kb-a", "knowledge-a", chunkID)
		file := directory + "/" + citationID + ".txt"
		sum := sha256.Sum256([]byte(excerpt))
		files[file] = []byte(craft.KnowledgeDataNotice + "\ncitation: " + citationID + "\nref: " + ref + "\n\n" + excerpt + "\n")
		manifest.Sources = append(manifest.Sources, publisherTestSource{
			CitationID: citationID, Ref: ref, KnowledgeID: "knowledge-a", KnowledgeBaseID: "kb-a",
			TenantID: 1, ExcerptBytes: len(excerpt), Digest: hex.EncodeToString(sum[:]),
			AcquiredAt: time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC), File: file,
		})
	}
	manifestBytes, err := json.Marshal(manifest)
	require.NoError(t, err)
	files[directory+"/manifest.json"] = manifestBytes
	return service.CraftKnowledgeMaterialPackage{
		RunID: runID, Directory: directory, Digest: publisherTestDigest(files), Files: files,
	}
}

func publisherTestDigest(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		_, _ = fmt.Fprintf(h, "%d:%s%d:", len(path), path, len(files[path]))
		_, _ = h.Write(files[path])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestCraftKnowledgePublisherPrepareIsPrivateAndResumable(t *testing.T) {
	root := publisherTestRoot(t)
	publisher, err := NewCraftKnowledgePackagePublisher(root, "run-private")
	require.NoError(t, err)
	pkg := publisherTestPackage(t, "run-private", "secret excerpt")
	workspace := publisherTestWorkspace()
	require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
	_, err = os.Stat(filepath.Join(root, filepath.FromSlash(pkg.Directory)))
	require.ErrorIs(t, err, os.ErrNotExist, "Prepare must not create a delegate-visible package")
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "private candidates must be stored outside the RunView root")

	restarted, err := NewCraftKnowledgePackagePublisher(root, "run-private")
	require.NoError(t, err)
	resumed, err := restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err, "a process restart must recover the exact sealed candidate")
	require.Equal(t, pkg, resumed)
}

func TestCraftKnowledgePublisherPublishesCompleteReadOnlyPackageAndConflictsOnDigestChange(t *testing.T) {
	root := publisherTestRoot(t)
	workspace := publisherTestWorkspace()
	publisher, err := NewCraftKnowledgePackagePublisher(root, "run-atomic")
	require.NoError(t, err)
	pkg := publisherTestPackage(t, "run-atomic", "selected excerpt")
	require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
	require.NoError(t, publisher.Publish(context.Background(), workspace, pkg))
	finalDir := filepath.Join(root, filepath.FromSlash(pkg.Directory))
	for name, expected := range pkg.Files {
		relative := strings.TrimPrefix(name, pkg.Directory+"/")
		actual, err := os.ReadFile(filepath.Join(finalDir, relative))
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		info, err := os.Stat(filepath.Join(finalDir, relative))
		require.NoError(t, err)
		require.Zero(t, info.Mode().Perm()&0222, "published material files must not be writable by a delegate")
	}
	dirInfo, err := os.Stat(finalDir)
	require.NoError(t, err)
	require.Zero(t, dirInfo.Mode().Perm()&0222, "published package directory must not be writable by a delegate")

	restarted, err := NewCraftKnowledgePackagePublisher(root, "run-atomic")
	require.NoError(t, err)
	resumed, err := restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err)
	require.Equal(t, pkg, resumed)
	require.NoError(t, restarted.Publish(context.Background(), workspace, pkg), "same digest replay is idempotent")

	changed := publisherTestPackage(t, pkg.RunID, "different excerpt")
	require.ErrorIs(t, restarted.Publish(context.Background(), workspace, changed), craft.ErrConflict)
	_, err = restarted.Resume(context.Background(), workspace, pkg.RunID, changed.Digest)
	require.ErrorIs(t, err, craft.ErrNotFound)
}

func TestCraftKnowledgePublisherFailedWriteAndRenameNeverExposePartialPackage(t *testing.T) {
	root := publisherTestRoot(t)
	workspace := publisherTestWorkspace()
	pkg := publisherTestPackage(t, "run-failure", "source text")
	publisher, err := NewCraftKnowledgePackagePublisher(root, "run-failure")
	require.NoError(t, err)
	publisher.writeFile = func(name string, data []byte, mode os.FileMode) error {
		if strings.HasSuffix(name, ".txt") {
			return errors.New("injected write failure")
		}
		return os.WriteFile(name, data, mode)
	}
	require.Error(t, publisher.Prepare(context.Background(), workspace, pkg))
	_, err = os.Stat(filepath.Join(root, filepath.FromSlash(pkg.Directory)))
	require.ErrorIs(t, err, os.ErrNotExist)
	runCandidateDir := filepath.Join(filepath.Dir(root), filepath.Base(root)+".craft-knowledge-candidates", pkg.RunID)
	entries, err := os.ReadDir(runCandidateDir)
	require.NoError(t, err)
	require.Empty(t, entries, "failed writes must clean their temporary candidate")

	publisher, err = NewCraftKnowledgePackagePublisher(root, "run-failure")
	require.NoError(t, err)
	require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
	standardRename := publisher.rename
	publisher.rename = func(oldPath, newPath string) error {
		if strings.HasPrefix(oldPath, filepath.Join(filepath.Dir(root), filepath.Base(root)+".craft-knowledge-candidates")) && newPath == filepath.Join(root, filepath.FromSlash(pkg.Directory)) {
			return errors.New("injected atomic rename failure")
		}
		return standardRename(oldPath, newPath)
	}
	require.Error(t, publisher.Publish(context.Background(), workspace, pkg))
	_, err = os.Stat(filepath.Join(root, filepath.FromSlash(pkg.Directory)))
	require.ErrorIs(t, err, os.ErrNotExist, "failed atomic rename must not expose a partial directory")
	resumed, err := publisher.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err, "the private accepted candidate must remain recoverable")
	require.Equal(t, pkg, resumed)
}

func TestCraftKnowledgePublisherRejectsSymlinkAndTraversalPaths(t *testing.T) {
	root := publisherTestRoot(t)
	publisher, err := NewCraftKnowledgePackagePublisher(root, "run-paths")
	require.NoError(t, err)
	workspace := publisherTestWorkspace()
	pkg := publisherTestPackage(t, "run-paths", "safe")
	traversal := pkg
	traversal.Directory = "../../outside"
	require.ErrorIs(t, publisher.Prepare(context.Background(), workspace, traversal), craft.ErrInvalidInput)
	traversal = publisherTestPackage(t, "run-paths", "safe")
	content := traversal.Files[traversal.Directory+"/manifest.json"]
	delete(traversal.Files, traversal.Directory+"/manifest.json")
	traversal.Files[traversal.Directory+"/../outside.txt"] = []byte("escape")
	traversal.Files[traversal.Directory+"/manifest.json"] = content
	traversal.Digest = publisherTestDigest(traversal.Files)
	require.ErrorIs(t, publisher.Prepare(context.Background(), workspace, traversal), craft.ErrInvalidInput)

	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, craft.KnowledgeDir)))
	safe := publisherTestPackage(t, "run-symlink", "secret")
	require.ErrorIs(t, publisher.Prepare(context.Background(), workspace, safe), craft.ErrInvalidInput)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries, "publisher must not traverse a symlink target")

	rootLink := filepath.Join(t.TempDir(), "root-link")
	require.NoError(t, os.Symlink(root, rootLink))
	_, err = NewCraftKnowledgePackagePublisher(rootLink, "run-paths")
	require.ErrorIs(t, err, craft.ErrInvalidInput)
}

func TestCraftKnowledgePublisherEnforcesMaterialCapsAndResumeNotFound(t *testing.T) {
	root := publisherTestRoot(t)
	publisher, err := NewCraftKnowledgePackagePublisher(root, "run-missing")
	require.NoError(t, err)
	workspace := publisherTestWorkspace()
	missing := publisherTestPackage(t, "run-missing", "not here")
	_, err = publisher.Resume(context.Background(), workspace, missing.RunID, missing.Digest)
	require.ErrorIs(t, err, craft.ErrNotFound)

	tooMany := make([]string, craft.MaxKnowledgeSources+1)
	for i := range tooMany {
		tooMany[i] = "x"
	}
	require.ErrorIs(t, publisher.Prepare(context.Background(), workspace, publisherTestPackage(t, "run-many-files", tooMany...)), craft.ErrInvalidInput)

	tooLarge := make([]string, 9)
	for i := range tooLarge {
		tooLarge[i] = strings.Repeat("x", craft.MaxKnowledgeExcerptBytes)
	}
	require.ErrorIs(t, publisher.Prepare(context.Background(), workspace, publisherTestPackage(t, "run-too-large", tooLarge...)), craft.ErrInvalidInput)
}

func TestCraftKnowledgePublisherDoesNotCopyEarlierRunIntoNewViewRoot(t *testing.T) {
	workspace := publisherTestWorkspace()
	rootA, rootB := publisherTestRoot(t), publisherTestRoot(t)
	publisherA, err := NewCraftKnowledgePackagePublisher(rootA, "run-a")
	require.NoError(t, err)
	packageA := publisherTestPackage(t, "run-a", "run A private secret")
	require.NoError(t, publisherA.Prepare(context.Background(), workspace, packageA))
	require.NoError(t, publisherA.Publish(context.Background(), workspace, packageA))

	publisherB, err := NewCraftKnowledgePackagePublisher(rootB, "run-b")
	require.NoError(t, err)
	packageB := publisherTestPackage(t, "run-b", "run B selected material")
	require.NoError(t, publisherB.Prepare(context.Background(), workspace, packageB))
	require.NoError(t, publisherB.Publish(context.Background(), workspace, packageB))
	for _, file := range packageB.Files {
		require.NotContains(t, string(file), "run A private secret")
	}
	entries, err := os.ReadDir(rootB)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, craft.KnowledgeDir, entries[0].Name())
}

func TestCraftKnowledgePublisherResumesCrashAfterRenameBeforeDirectorySeal(t *testing.T) {
	root := publisherTestRoot(t)
	workspace := publisherTestWorkspace()
	pkg := publisherTestPackage(t, "run-crash-unsealed", "accepted material")
	publisher, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
	require.NoError(t, err)
	require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
	finalPath := publisherSimulateCrashAfterRename(t, publisher, pkg)

	info, err := os.Stat(finalPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), info.Mode().Perm(), "fixture models the rename/chmod crash window")

	restarted, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
	require.NoError(t, err)
	resumed, err := restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err, "restart must validate and finalize the exact accepted bytes")
	require.Equal(t, pkg, resumed)
	info, err = os.Stat(finalPath)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0555), info.Mode().Perm())
	_, err = os.Stat(restarted.candidatePath(pkg.RunID, pkg.Digest))
	require.ErrorIs(t, err, os.ErrNotExist, "recovery removes the sealed private remainder")

	resumed, err = restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err, "recovery must be idempotent")
	require.Equal(t, pkg, resumed)
}

func TestCraftKnowledgePublisherRefusesAndPreservesCorruptCrashRemainder(t *testing.T) {
	for _, testCase := range []string{"corrupt-bytes", "symlink", "wrong-scope"} {
		t.Run(testCase, func(t *testing.T) {
			root := publisherTestRoot(t)
			workspace := publisherTestWorkspace()
			pkg := publisherTestPackage(t, "run-corrupt-remainder", "accepted material")
			publisher, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
			require.NoError(t, err)
			require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
			finalPath := publisherSimulateCrashAfterRename(t, publisher, pkg)
			materialPath := filepath.Join(finalPath, filepath.Base(pkg.Directory+"/"+citationFile(pkg)))
			outside := filepath.Join(t.TempDir(), "outside.txt")
			require.NoError(t, os.WriteFile(outside, []byte("outside sentinel"), 0600))
			resumeWorkspace := workspace
			switch testCase {
			case "corrupt-bytes":
				require.NoError(t, os.Chmod(materialPath, 0600))
				require.NoError(t, os.WriteFile(materialPath, []byte("tampered"), 0600))
			case "symlink":
				require.NoError(t, os.Chmod(finalPath, 0700))
				require.NoError(t, os.Remove(materialPath))
				require.NoError(t, os.Symlink(outside, materialPath))
			case "wrong-scope":
				resumeWorkspace.UserID = "different-user"
			}

			restarted, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
			require.NoError(t, err)
			_, err = restarted.Resume(context.Background(), resumeWorkspace, pkg.RunID, pkg.Digest)
			require.Error(t, err, "mismatched or symlinked final bytes must fail closed")
			_, err = os.Stat(finalPath)
			require.NoError(t, err, "refusal must preserve final evidence")
			_, err = os.Stat(restarted.candidatePath(pkg.RunID, pkg.Digest))
			require.NoError(t, err, "refusal must preserve candidate seal evidence")
			actualOutside, err := os.ReadFile(outside)
			require.NoError(t, err)
			require.Equal(t, []byte("outside sentinel"), actualOutside)
		})
	}
}

func TestCraftKnowledgePublisherFinalizesChmodCrashAndSyncsCandidateParent(t *testing.T) {
	root := publisherTestRoot(t)
	workspace := publisherTestWorkspace()
	pkg := publisherTestPackage(t, "run-crash-sealed", "accepted material")
	publisher, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
	require.NoError(t, err)
	require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
	finalPath := publisherSimulateCrashAfterRename(t, publisher, pkg)
	require.NoError(t, os.Chmod(finalPath, 0555), "fixture models chmod before parent sync/candidate cleanup")

	restarted, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
	require.NoError(t, err)
	var synced []string
	restarted.syncDir = func(directory string) error {
		synced = append(synced, directory)
		return syncCraftKnowledgeDir(directory)
	}
	resumed, err := restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err)
	require.Equal(t, pkg, resumed)
	candidatePath := restarted.candidatePath(pkg.RunID, pkg.Digest)
	_, err = os.Stat(candidatePath)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Contains(t, synced, filepath.Dir(candidatePath), "candidate deletion must sync the directory that contained its entry")
	resumed, err = restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
	require.NoError(t, err, "post-chmod cleanup recovery must be idempotent")
	require.Equal(t, pkg, resumed)
}

func publisherSimulateCrashAfterRename(t *testing.T, publisher *craftKnowledgeFilesystemPublisher, pkg service.CraftKnowledgeMaterialPackage) string {
	t.Helper()
	require.NoError(t, publisher.ensureRunParent(pkg.Directory))
	payloadPath := filepath.Join(publisher.candidatePath(pkg.RunID, pkg.Digest), craftKnowledgePublisherPayload)
	require.NoError(t, prepareCraftKnowledgePublishedModes(payloadPath, pkg.Files))
	finalPath := filepath.Join(publisher.root, filepath.FromSlash(pkg.Directory))
	require.NoError(t, os.Rename(payloadPath, finalPath))
	return finalPath
}

func citationFile(pkg service.CraftKnowledgeMaterialPackage) string {
	for file := range pkg.Files {
		if file != pkg.Directory+"/manifest.json" {
			return filepath.Base(file)
		}
	}
	return ""
}

func TestCraftKnowledgePublisherRecoverySyncsKnowledgeGrandparentLikePublish(t *testing.T) {
	// Durability parity with the Publish success path: a crash between the
	// atomic rename and the directory fsync chain may leave the runs/ entry —
	// or even the freshly created runs/ directory itself under knowledge/ —
	// undurable. Whichever recovery branch finalizes the package must therefore
	// fsync the same three levels Publish syncs: the sealed run directory, its
	// runs/ parent and the knowledge/ grandparent.
	for _, crashWindow := range []string{"unsealed-0700", "sealed-0555"} {
		t.Run(crashWindow, func(t *testing.T) {
			root := publisherTestRoot(t)
			workspace := publisherTestWorkspace()
			pkg := publisherTestPackage(t, "run-recovery-grandparent-"+crashWindow, "accepted material")
			publisher, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
			require.NoError(t, err)
			require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
			finalPath := publisherSimulateCrashAfterRename(t, publisher, pkg)
			if crashWindow == "sealed-0555" {
				require.NoError(t, os.Chmod(finalPath, 0555), "fixture models the crash window after the directory seal")
			}

			restarted, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
			require.NoError(t, err)
			var synced []string
			restarted.syncDir = func(directory string) error {
				synced = append(synced, directory)
				return syncCraftKnowledgeDir(directory)
			}
			resumed, err := restarted.Resume(context.Background(), workspace, pkg.RunID, pkg.Digest)
			require.NoError(t, err)
			require.Equal(t, pkg, resumed)
			require.Contains(t, synced, finalPath, "recovery must sync the sealed run directory itself")
			require.Contains(t, synced, filepath.Dir(finalPath), "recovery must sync the runs/ parent")
			require.Contains(t, synced, filepath.Dir(filepath.Dir(finalPath)), "recovery must sync the knowledge/ grandparent like the Publish success path")
		})
	}
}

func TestCraftKnowledgePublisherSyncsImmediateCandidateParentAfterRemoval(t *testing.T) {
	for _, operation := range []string{"publish", "discard"} {
		t.Run(operation, func(t *testing.T) {
			root := publisherTestRoot(t)
			workspace := publisherTestWorkspace()
			pkg := publisherTestPackage(t, "run-sync-parent", "accepted material")
			publisher, err := NewCraftKnowledgePackagePublisher(root, pkg.RunID)
			require.NoError(t, err)
			require.NoError(t, publisher.Prepare(context.Background(), workspace, pkg))
			var synced []string
			publisher.syncDir = func(directory string) error {
				synced = append(synced, directory)
				return syncCraftKnowledgeDir(directory)
			}
			if operation == "publish" {
				require.NoError(t, publisher.Publish(context.Background(), workspace, pkg))
			} else {
				require.NoError(t, publisher.Discard(context.Background(), workspace, pkg))
			}
			require.Contains(t, synced, filepath.Dir(publisher.candidatePath(pkg.RunID, pkg.Digest)), "sync the parent directory containing the removed candidate entry")
		})
	}
}
