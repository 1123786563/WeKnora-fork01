package subagents

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// These tests scan the real builtin role library that
// scripts/import_octop_subagents.py copies verbatim from Octop into
// <repo>/config/subagents/library. Expected counts are computed from the
// imported tree at test time (os.ReadDir) rather than hardcoded, so the
// tests stay correct if the upstream library grows.

// builtinLibraryRoot is <repo>/config/subagents/library, resolved relative
// to this package's directory (internal/agent/subagents).
func builtinLibraryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return filepath.Join(root, "config", "subagents", "library")
}

// importSkip lists library files (slash-separated, relative to the library
// root) that the import script deliberately drops: the supply-chain
// duplicate stem. The scanner errors on a slug appearing in two divisions
// of one locale, and en ships supply-chain-strategist only under
// specialized/, so the specialized/ zh copy wins.
var importSkip = map[string]bool{
	"zh/supply-chain/supply-chain-strategist.md": true,
}

// libraryStems returns the set of filename stems of
// <libraryRoot>/<locale>/<division>/*.md (skipping importSkip entries),
// mirroring the scanner's slug derivation.
func libraryStems(t *testing.T, libraryRoot, locale string) map[string]bool {
	t.Helper()
	localeDir := filepath.Join(libraryRoot, locale)
	divisionDirs, err := os.ReadDir(localeDir)
	if err != nil {
		t.Fatalf("read %s: %v", localeDir, err)
	}
	stems := make(map[string]bool)
	for _, division := range divisionDirs {
		if !division.IsDir() {
			continue // divisions.json and other root-level files
		}
		divisionDir := filepath.Join(localeDir, division.Name())
		files, err := os.ReadDir(divisionDir)
		if err != nil {
			t.Fatalf("read %s: %v", divisionDir, err)
		}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), mdSuffix) {
				continue
			}
			if importSkip[path.Join(locale, division.Name(), file.Name())] {
				continue
			}
			stems[strings.TrimSuffix(file.Name(), mdSuffix)] = true
		}
	}
	return stems
}

// libraryDivisionDirs returns the division directory names under
// <libraryRoot>/<locale>.
func libraryDivisionDirs(t *testing.T, libraryRoot, locale string) []string {
	t.Helper()
	localeDir := filepath.Join(libraryRoot, locale)
	entries, err := os.ReadDir(localeDir)
	if err != nil {
		t.Fatalf("read %s: %v", localeDir, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs
}

func TestScanBuiltinLibrary(t *testing.T) {
	root := builtinLibraryRoot(t)
	catalog, err := ScanSubagents(root)
	if err != nil {
		t.Fatalf("ScanSubagents failed — has scripts/import_octop_subagents.py been run? %v", err)
	}

	// The Octop library ships 19 divisions, and the catalog mirrors
	// divisions.json, which must also match the division dirs on disk.
	if len(catalog.Divisions) != 19 {
		t.Errorf("Divisions size = %d, want 19", len(catalog.Divisions))
	}
	zhDivDirs := libraryDivisionDirs(t, root, "zh")
	if len(zhDivDirs) != len(catalog.Divisions) {
		t.Errorf("zh division dirs on disk = %d, want %d (divisions.json)",
			len(zhDivDirs), len(catalog.Divisions))
	}

	// The skipped duplicate must actually be gone, otherwise the scan
	// above would have errored on the duplicate slug.
	if _, err := os.Stat(filepath.Join(root, "zh", "supply-chain", "supply-chain-strategist.md")); !os.IsNotExist(err) {
		t.Errorf("zh/supply-chain/supply-chain-strategist.md must not be imported (duplicate slug; err=%v)", err)
	}

	// Expected counts computed from the imported tree: every distinct stem
	// across locales is exactly one BySlug entry.
	zhStems := libraryStems(t, root, "zh")
	enStems := libraryStems(t, root, "en")
	union := make(map[string]bool, len(zhStems)+len(enStems))
	for stem := range zhStems {
		union[stem] = true
	}
	for stem := range enStems {
		union[stem] = true
	}
	if len(catalog.BySlug) != len(union) {
		t.Errorf("BySlug size = %d, want %d (distinct stems across zh/en)", len(catalog.BySlug), len(union))
	}

	// zh-only slugs: computed from the tree vs. entries lacking an En
	// definition.
	wantZhOnly := 0
	for stem := range zhStems {
		if !enStems[stem] {
			wantZhOnly++
		}
	}
	gotZhOnly := 0
	for _, entry := range catalog.BySlug {
		if entry.Zh != nil && entry.En == nil {
			gotZhOnly++
		}
	}
	if gotZhOnly != wantZhOnly {
		t.Errorf("zh-only entries = %d, want %d (computed from tree)", gotZhOnly, wantZhOnly)
	}

	// product-manager: paired across locales, in the product division,
	// with a real body and WebFetch among its tools.
	pm, ok := catalog.BySlug["product-manager"]
	if !ok {
		t.Fatal("BySlug missing product-manager")
	}
	if pm.Zh == nil {
		t.Fatal("product-manager.Zh is nil")
	}
	if pm.Zh.Division != "product" {
		t.Errorf("product-manager zh division = %q, want product", pm.Zh.Division)
	}
	if len(pm.Zh.Body) == 0 {
		t.Error("product-manager zh body is empty")
	}
	if !strings.Contains(pm.Zh.Frontmatter.ToolsRaw, "WebFetch") {
		t.Errorf("product-manager zh ToolsRaw = %q, want it to contain WebFetch", pm.Zh.Frontmatter.ToolsRaw)
	}

	// Locale resolution serves the en definition when requested.
	resolved := catalog.Resolve("product-manager", "en")
	if resolved == nil {
		t.Fatal("Resolve(product-manager, en) = nil")
	}
	if resolved.Locale != "en" || pm.En == nil {
		t.Errorf("Resolve(product-manager, en) locale = %q, entry.En = %v; want en definition", resolved.Locale, pm.En)
	}

	// The surviving supply-chain-strategist copy is the specialized one
	// (en ships it there too, so the slug stays paired).
	scs, ok := catalog.BySlug["supply-chain-strategist"]
	if !ok {
		t.Fatal("BySlug missing supply-chain-strategist")
	}
	if scs.Division != "specialized" {
		t.Errorf("supply-chain-strategist division = %q, want specialized", scs.Division)
	}
}

func TestBuiltinLibraryEntryInvariants(t *testing.T) {
	root := builtinLibraryRoot(t)
	catalog, err := ScanSubagents(root)
	if err != nil {
		t.Fatalf("ScanSubagents failed — has scripts/import_octop_subagents.py been run? %v", err)
	}

	knownDivisions := make(map[string]bool, len(catalog.Divisions))
	for _, division := range catalog.Divisions {
		knownDivisions[division.Slug] = true
	}

	for slug, entry := range catalog.BySlug {
		if entry.Zh == nil && entry.En == nil {
			t.Errorf("entry %q has neither locale", slug)
		}
		if entry.Slug != slug {
			t.Errorf("entry key %q holds slug %q", slug, entry.Slug)
		}
		if !knownDivisions[entry.Division] {
			t.Errorf("entry %q division %q is not in divisions.json", slug, entry.Division)
		}
		for name, def := range map[string]*SubagentDefinition{"zh": entry.Zh, "en": entry.En} {
			if def == nil {
				continue
			}
			if def.Slug != slug || def.Division != entry.Division {
				t.Errorf("entry %q: %s definition slug/division = %q/%q", slug, name, def.Slug, def.Division)
			}
			if def.Locale != name {
				t.Errorf("entry %q: %s definition stamped locale %q", slug, name, def.Locale)
			}
		}
	}
}
