package experts

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// skillManifestFile is the one file every bundled skill directory must carry;
// the install path derives the skill's identity from it.
const skillManifestFile = "SKILL.md"

// maxSkillZipBytes caps the uncompressed size of one zipped skill directory.
// The upload path allows 512 MiB across a whole archive; a bundled expert
// skill is shipped with the platform and stays far below that, so a tighter
// cap here catches a runaway directory (a vendored build tree, a checked-in
// dataset) before it reaches the tenant skill catalog.
const maxSkillZipBytes = 64 << 20

// skillZipModTime stamps every entry so the same directory always zips to the
// same bytes regardless of checkout mtimes.
var skillZipModTime = time.Unix(0, 0).UTC()

// ZipSkillDir packs one bundled skill directory into the archive layout
// TenantSkillService.InstallSkill accepts: a single top-level
// <basename(dir)>/ directory holding SKILL.md plus the skill's files.
//
// The archive is deterministic — SKILL.md first, remaining entries sorted,
// fixed mod times — so identical trees produce identical bytes. Only regular
// files are packed: a symlink (or any other special file) is an error rather
// than a silent skip, because the install path would reject it later anyway
// and skipping it would ship a skill that differs from the author's tree.
// Total uncompressed size is capped at maxSkillZipBytes.
func ZipSkillDir(dir string) ([]byte, error) {
	if dir == "" {
		return nil, fmt.Errorf("experts: zip skill: directory is required")
	}
	cleaned := filepath.Clean(dir)
	slug := filepath.Base(cleaned)
	if slug == "." || slug == ".." || slug == string(filepath.Separator) {
		return nil, fmt.Errorf("experts: zip skill: %q is not a skill directory", dir)
	}
	info, err := os.Stat(cleaned)
	if err != nil {
		return nil, fmt.Errorf("experts: zip skill %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("experts: zip skill: %q is not a directory", dir)
	}

	files, err := collectSkillZipFiles(cleaned)
	if err != nil {
		return nil, err
	}
	if _, ok := files[skillManifestFile]; !ok {
		return nil, fmt.Errorf("experts: zip skill %q: %s is missing", dir, skillManifestFile)
	}

	// SKILL.md leads so the parser's skill-root discovery settles on the
	// first entry; everything else follows in sorted order.
	ordered := make([]string, 0, len(files))
	ordered = append(ordered, skillManifestFile)
	for _, rel := range sortedRelPaths(files) {
		if rel != skillManifestFile {
			ordered = append(ordered, rel)
		}
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	remaining := int64(maxSkillZipBytes)
	for _, rel := range ordered {
		content, err := os.ReadFile(filepath.Join(cleaned, filepath.FromSlash(rel)))
		if err != nil {
			return nil, fmt.Errorf("experts: zip skill %q: read %s: %w", dir, rel, err)
		}
		// The walk already checked declared sizes; this re-checks what is
		// actually read, so a file growing between stat and read cannot blow
		// the budget either.
		if int64(len(content)) > remaining {
			return nil, fmt.Errorf("experts: zip skill %q: contents exceed the %d byte limit",
				dir, int64(maxSkillZipBytes))
		}
		remaining -= int64(len(content))

		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name:     slug + "/" + rel,
			Method:   zip.Deflate,
			Modified: skillZipModTime,
		})
		if err != nil {
			return nil, fmt.Errorf("experts: zip skill %q: write %s: %w", dir, rel, err)
		}
		if _, err := entry.Write(content); err != nil {
			return nil, fmt.Errorf("experts: zip skill %q: write %s: %w", dir, rel, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("experts: zip skill %q: %w", dir, err)
	}
	return buf.Bytes(), nil
}

// collectSkillZipFiles walks root and indexes its regular files by
// root-relative slash path, rejecting symlinks and other special files and
// enforcing the total size cap from declared sizes (before any content is
// read, so an oversized tree fails cheaply).
func collectSkillZipFiles(root string) (map[string]int64, error) {
	files := make(map[string]int64)
	var total int64
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("experts: zip skill: %q is not a regular file (symlinks are not supported)", p)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = info.Size()
		total += info.Size()
		if total > maxSkillZipBytes {
			return fmt.Errorf("experts: zip skill: %q exceeds the %d byte limit",
				root, int64(maxSkillZipBytes))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func sortedRelPaths(files map[string]int64) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
