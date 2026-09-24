//go:build linux || darwin

package container

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestRenameRunViewInputNoReplaceMovesTemporaryEntry(t *testing.T) {
	dir := t.TempDir()
	dirFD, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(dirFD)
	if err := os.WriteFile(filepath.Join(dir, "temporary"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renameRunViewInputNoReplace(dirFD, "temporary", "target"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "temporary")); !os.IsNotExist(err) {
		t.Fatalf("atomic rename must consume its source name, got %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(dir, "target")); err != nil || string(content) != "source" {
		t.Fatalf("published target = %q, %v", content, err)
	}
}

func TestRenameRunViewInputNoReplacePreservesExistingDestinationKinds(t *testing.T) {
	for _, kind := range []string{"file", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			dirFD, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer unix.Close(dirFD)
			if err := os.WriteFile(filepath.Join(dir, "temporary"), []byte("source"), 0o644); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(dir, "target")
			switch kind {
			case "file":
				if err := os.WriteFile(target, []byte("existing"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink("outside", target); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(target, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			err = renameRunViewInputNoReplace(dirFD, "temporary", "target")
			if !errors.Is(err, unix.EEXIST) {
				t.Fatalf("existing destination error = %v, want EEXIST", err)
			}
			if content, readErr := os.ReadFile(filepath.Join(dir, "temporary")); readErr != nil || string(content) != "source" {
				t.Fatalf("source changed after refused publish: %q, %v", content, readErr)
			}
			info, statErr := os.Lstat(target)
			if statErr != nil {
				t.Fatalf("existing target disappeared: %v", statErr)
			}
			switch kind {
			case "file":
				content, readErr := os.ReadFile(target)
				if readErr != nil || string(content) != "existing" {
					t.Fatalf("existing file changed: %q, %v", content, readErr)
				}
			case "symlink":
				if info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("existing symlink replaced by mode %v", info.Mode())
				}
			case "directory":
				if !info.IsDir() {
					t.Fatalf("existing directory replaced by mode %v", info.Mode())
				}
			}
		})
	}
}
