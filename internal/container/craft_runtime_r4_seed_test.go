package container

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
	"golang.org/x/sys/unix"
)

func TestCraftRunViewR4SeedRejectsHeadChangedAfterAdmission(t *testing.T) {
	seed := craftRunViewWorkspaceSeed{WorkspaceID: "ws-1", State: craft.DraftHeadSelected, DraftRevision: 1, SourceRunID: "run-a", ManifestDigest: "digest-a"}
	head := craft.DraftHead{WorkspaceID: "ws-1", Revision: 2, State: craft.DraftHeadSelected, SourceRunID: "run-b", ManifestDigest: "digest-b"}
	if err := validateCraftRunViewWorkspaceSeed(seed, head); err == nil {
		t.Fatal("changed Workspace head was accepted for the frozen predecessor")
	}
}

func TestCraftRunViewR4SeedAcceptsExplicitEmptyPredecessor(t *testing.T) {
	seed := craftRunViewWorkspaceSeed{WorkspaceID: "ws-1", State: craft.DraftHeadEmpty, DraftRevision: 0}
	head := craft.DraftHead{WorkspaceID: "ws-1", Revision: 0, State: craft.DraftHeadEmpty}
	if err := validateCraftRunViewWorkspaceSeed(seed, head); err != nil {
		t.Fatalf("explicit empty predecessor rejected: %v", err)
	}
}

func TestCraftRunViewR4SeedAcceptsFrozenSelectedRevisionAfterHeadMoves(t *testing.T) {
	files := []craft.File{{Path: "index.html", Ref: "tenant-1/object-1", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Bytes: 1}}
	digest, err := craft.ManifestDigest(files)
	if err != nil {
		t.Fatal(err)
	}
	seed := craftRunViewWorkspaceSeed{WorkspaceID: "ws-1", State: craft.DraftHeadSelected, DraftRevision: 1, SourceRunID: "run-a", ManifestDigest: digest}
	frozen := craft.DraftHead{WorkspaceID: "ws-1", Revision: 1, State: craft.DraftHeadSelected, SourceRunID: "run-a", ManifestDigest: digest, Files: files}
	if err := validateCraftRunViewWorkspaceSeed(seed, frozen); err != nil {
		t.Fatalf("frozen predecessor rejected after current head moved: %v", err)
	}
}

func TestCraftRunViewR4SeedRejectsUnknownSeedState(t *testing.T) {
	seed := craftRunViewWorkspaceSeed{WorkspaceID: "ws-1", State: "unknown", DraftRevision: 0}
	head := craft.DraftHead{WorkspaceID: "ws-1", Revision: 0, State: craft.DraftHeadEmpty}
	if err := validateCraftRunViewWorkspaceSeed(seed, head); err == nil {
		t.Fatal("unknown predecessor state was accepted")
	}
}

func TestCraftRunViewR4OutputManifestRejectsFileDirectoryCollision(t *testing.T) {
	if err := validateRunViewDraftOutputPaths(map[string]craft.File{
		"bundle":            {Path: "bundle"},
		"bundle/index.html": {Path: "bundle/index.html"},
	}); err == nil {
		t.Fatal("file and child path collision was accepted")
	}
}

func TestCraftRunViewR4OutputTreeAcceptsOnlyExactRetryBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	content := []byte("frozen D1")
	sum := sha256.Sum256(content)
	files := map[string]craft.File{"nested/index.html": {Path: "nested/index.html", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(content))}}
	if present, err := inspectRunViewDraftOutput(fd, "", files); err != nil || len(present) != 0 {
		t.Fatalf("initial empty private output inspect = %v, %v", present, err)
	}
	nestedFD, err := openOrCreateRunViewDraftParent(fd, "nested")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeRunViewInputAtomic(nestedFD, "index.html", content); err != nil {
		_ = unix.Close(nestedFD)
		t.Fatal(err)
	}
	_ = unix.Close(nestedFD)
	present, err := inspectRunViewDraftOutput(fd, "", files)
	if err != nil || len(present) != 1 || !present["nested/index.html"] {
		t.Fatalf("identical retry inspect = %v, %v", present, err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "index.html"), []byte("mutated"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := inspectRunViewDraftOutput(fd, "", files); err == nil {
		t.Fatal("changed predecessor bytes were accepted on retry")
	}
}

func TestCraftRunViewR4OutputTreeRejectsSymlinkAndExtras(t *testing.T) {
	for _, setup := range []struct {
		name  string
		build func(string) error
	}{
		{name: "symlink", build: func(root string) error { return os.Symlink("/etc/passwd", filepath.Join(root, "index.html")) }},
		{name: "extra", build: func(root string) error {
			return os.WriteFile(filepath.Join(root, "unexpected.txt"), []byte("x"), 0o644)
		}},
	} {
		t.Run(setup.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := setup.build(root); err != nil {
				t.Fatal(err)
			}
			fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(fd)
			if _, err := inspectRunViewDraftOutput(fd, "", map[string]craft.File{}); err == nil {
				t.Fatal("unsafe or unpinned output entry was accepted")
			}
		})
	}
}

func TestCraftRunViewR4OutputTreeRejectsSameByteHardLinkReplacementAfterPreflight(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("same bytes")
	target := filepath.Join(root, "index.html")
	object := filepath.Join(root, "sealed-object")
	if err := os.WriteFile(target, content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(object, content, 0o644); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	sum := sha256.Sum256(content)
	files := map[string]craft.File{"index.html": {Path: "index.html", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(content))}}
	replaced := false
	_, err = inspectRunViewDraftOutputWithOps(fd, "", files, runViewDraftOutputInspectOps{beforeFileOpen: func(_ int, name string) error {
		if replaced || name != "index.html" {
			return nil
		}
		replaced = true
		if err := os.Remove(target); err != nil {
			return err
		}
		return os.Link(object, target)
	}})
	if err == nil {
		t.Fatal("same-byte hard-link replacement after preflight was accepted")
	}
}

func TestCraftRunViewR4OutputTreeRejectsDirectoryReplacementAfterPreflight(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("same bytes")
	oldDir := filepath.Join(root, "bundle")
	if err := os.Mkdir(oldDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDir, "index.html"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	sum := sha256.Sum256(content)
	files := map[string]craft.File{"bundle/index.html": {Path: "bundle/index.html", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(content))}}
	replaced := false
	_, err = inspectRunViewDraftOutputWithOps(fd, "", files, runViewDraftOutputInspectOps{beforeDirOpen: func(_ int, name string) error {
		if replaced || name != "bundle" {
			return nil
		}
		replaced = true
		if err := os.Rename(oldDir, oldDir+"-old"); err != nil {
			return err
		}
		if err := os.Mkdir(oldDir, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(oldDir, "index.html"), content, 0o644)
	}})
	if err == nil {
		t.Fatal("directory replacement after preflight was accepted")
	}
}
