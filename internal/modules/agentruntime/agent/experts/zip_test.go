package experts

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeSkillFixture creates <tmp>/<slug> with SKILL.md plus two nested files
// and returns the skill directory path.
func writeSkillFixture(t *testing.T, slug string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), slug)
	for _, sub := range []string{"", "assets", "scripts"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"SKILL.md":        "---\nname: stock-quote\ndescription: Fixture skill for tests\n---\n\nBody.\n",
		"assets/data.txt": "data",
		"scripts/tool.py": "print('hi')\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func readZipEntries(t *testing.T, archive []byte) []*zip.File {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open zipped skill: %v", err)
	}
	return reader.File
}

// TestZipSkillDirGoldenStructure pins the archive layout InstallSkill's
// ParseSkillBundle consumes: a single top-level <slug>/ directory, SKILL.md as
// the first entry, remaining files sorted, directory entries absent.
func TestZipSkillDirGoldenStructure(t *testing.T) {
	dir := writeSkillFixture(t, "sample-skill")

	archive, err := ZipSkillDir(dir)
	if err != nil {
		t.Fatalf("ZipSkillDir: %v", err)
	}
	entries := readZipEntries(t, archive)
	wantNames := []string{
		"sample-skill/SKILL.md",
		"sample-skill/assets/data.txt",
		"sample-skill/scripts/tool.py",
	}
	if len(entries) != len(wantNames) {
		t.Fatalf("got %d entries %v, want %d", len(entries), entryNames(entries), len(wantNames))
	}
	for i, want := range wantNames {
		if entries[i].Name != want {
			t.Errorf("entry %d = %q, want %q (order: SKILL.md first, rest sorted)", i, entries[i].Name, want)
		}
		if entries[i].FileInfo().IsDir() {
			t.Errorf("entry %q must not be a directory", entries[i].Name)
		}
	}
	rc, err := entries[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "name: stock-quote") {
		t.Errorf("SKILL.md content = %q", string(manifest))
	}
}

func entryNames(files []*zip.File) []string {
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	return names
}

// TestZipSkillDirDeterministic zips the same tree twice — after shifting every
// mtime — and requires byte-identical output.
func TestZipSkillDirDeterministic(t *testing.T) {
	dir := writeSkillFixture(t, "sample-skill")

	first, err := ZipSkillDir(dir)
	if err != nil {
		t.Fatalf("first zip: %v", err)
	}
	shift := time.Now().Add(3 * time.Hour)
	if err := os.Chtimes(dir, shift, shift); err != nil {
		t.Fatal(err)
	}
	err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(p, shift, shift)
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := ZipSkillDir(dir)
	if err != nil {
		t.Fatalf("second zip: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("zipping the same tree twice produced different bytes")
	}
}

// TestZipSkillDirRejectsSymlink refuses anything that is not a regular file:
// the install path rejects symlink entries, and silently skipping one would
// ship a skill that differs from the template author's tree.
func TestZipSkillDirRejectsSymlink(t *testing.T) {
	dir := writeSkillFixture(t, "sample-skill")
	if err := os.Symlink("SKILL.md", filepath.Join(dir, "alias.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := ZipSkillDir(dir); err == nil {
		t.Fatal("ZipSkillDir accepted a symlink, want error")
	} else if !strings.Contains(err.Error(), "alias.md") {
		t.Errorf("error should name the offending entry, got: %v", err)
	}
}

func TestZipSkillDirRequiresManifest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "empty-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("no manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ZipSkillDir(dir)
	if err == nil || !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("err = %v, want missing-SKILL.md error", err)
	}
}

func TestZipSkillDirMissingDirectory(t *testing.T) {
	if _, err := ZipSkillDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("ZipSkillDir accepted a missing directory, want error")
	}
}

// TestZipSkillDirSizeGuard asserts the guard constant and the oversize
// rejection without materializing 64 MiB of data: the walk reads declared
// sizes, so a sparse file over the cap fails before any content is read.
func TestZipSkillDirSizeGuard(t *testing.T) {
	if maxSkillZipBytes != 64<<20 {
		t.Errorf("maxSkillZipBytes = %d, want %d", maxSkillZipBytes, 64<<20)
	}
	dir := writeSkillFixture(t, "sample-skill")
	big, err := os.Create(filepath.Join(dir, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(maxSkillZipBytes + 1); err != nil {
		t.Fatal(err)
	}
	big.Close()
	if _, err := ZipSkillDir(dir); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err = %v, want oversize error", err)
	}
}
