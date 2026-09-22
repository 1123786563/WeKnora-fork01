package subagents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"gopkg.in/yaml.v3"
)

const (
	// frontmatterFence delimits the YAML frontmatter block of a role file.
	frontmatterFence = "---"
	// divisionsRelPath is the authoritative division metadata location
	// inside the library (only the zh tree ships divisions.json).
	divisionsRelPath = "zh/divisions.json"
	mdSuffix         = ".md"

	// LocaleZh and LocaleEn are the only locale directories scanned; the
	// slug pairing across them keys on the filename stem.
	localeZh = "zh"
	localeEn = "en"
)

// ScanSubagents scans dir, the library root laid out as
// <dir>/{zh,en}/<division>/*.md plus <dir>/zh/divisions.json, and merges
// the per-locale role files into slug-keyed catalog entries.
//
// Contract:
//   - zh/divisions.json is required; a missing or division-less file is an
//     error. Extra keys alongside "divisions" (Octop ships a "_note") are
//     tolerated, and Divisions preserves the JSON document order.
//   - Division directories under a locale that divisions.json does not
//     list are an error; listed divisions without any .md file are fine.
//   - A role file must carry frontmatter with a non-empty name AND
//     description; violations are an error naming the file. Other
//     frontmatter keys and non-.md files are ignored.
//   - A slug (filename stem) appearing in multiple divisions within one
//     locale is an error; the same stem in different locales pairs.
//   - Entries at the library root other than the locale directories
//     (e.g. Octop's README.md) are ignored. en/ is optional; zh/ is
//     implied by the mandatory zh/divisions.json.
//
// Slugs are read from directory entries (never path input), so filenames
// cannot traverse outside the scanned tree. The scan is deterministic:
// locale, division, and file iteration follow os.ReadDir's sorted order.
func ScanSubagents(dir string) (*Catalog, error) {
	divisionsPath := filepath.Join(dir, filepath.FromSlash(divisionsRelPath))
	divisionsData, err := os.ReadFile(divisionsPath)
	if err != nil {
		return nil, fmt.Errorf("subagents: read %s: %w", divisionsPath, err)
	}
	divisions, err := parseDivisions(divisionsData)
	if err != nil {
		return nil, fmt.Errorf("subagents: parse %s: %w", divisionsPath, err)
	}
	knownDivisions := make(map[string]DivisionInfo, len(divisions))
	for _, d := range divisions {
		knownDivisions[d.Slug] = d
	}

	catalog := &Catalog{Divisions: divisions, BySlug: make(map[string]*CatalogEntry)}
	for _, locale := range []string{localeZh, localeEn} {
		if err := scanLocale(catalog, filepath.Join(dir, locale), locale, knownDivisions); err != nil {
			return nil, err
		}
	}
	return catalog, nil
}

// scanLocale merges one locale tree (<localeDir>/<division>/*.md) into
// catalog. A missing en/ directory is the "no English library" case and is
// skipped; zh/ cannot be missing because divisions.json lives inside it.
func scanLocale(catalog *Catalog, localeDir, locale string, knownDivisions map[string]DivisionInfo) error {
	entries, err := os.ReadDir(localeDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && locale == localeEn {
			return nil
		}
		return fmt.Errorf("subagents: scan %s: %w", localeDir, err)
	}

	slugDivisions := make(map[string]string) // slug → division, within this locale
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // divisions.json and any other files sit here unscanned
		}
		division := entry.Name()
		if _, ok := knownDivisions[division]; !ok {
			return fmt.Errorf("subagents: %s: division %q is not listed in %s",
				filepath.Join(localeDir, division), division, divisionsRelPath)
		}
		if err := scanDivision(catalog, filepath.Join(localeDir, division), division, locale, slugDivisions); err != nil {
			return err
		}
	}
	return nil
}

// scanDivision reads one <division>/*.md directory into catalog; files that
// are neither directories nor .md are ignored.
func scanDivision(catalog *Catalog, divisionDir, division, locale string, slugDivisions map[string]string) error {
	files, err := os.ReadDir(divisionDir)
	if err != nil {
		return fmt.Errorf("subagents: scan %s: %w", divisionDir, err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), mdSuffix) {
			continue
		}
		slug := strings.TrimSuffix(file.Name(), mdSuffix)
		path := filepath.Join(divisionDir, file.Name())
		def, err := parseSubagentFile(path, slug, division, locale)
		if err != nil {
			return err
		}
		if prev, dup := slugDivisions[slug]; dup {
			return fmt.Errorf("subagents: slug %q appears in divisions %q and %q in locale %s",
				slug, prev, division, locale)
		}
		slugDivisions[slug] = division

		entry := catalog.BySlug[slug]
		if entry == nil {
			entry = &CatalogEntry{Slug: slug, Division: division}
			catalog.BySlug[slug] = entry
		}
		switch locale {
		case localeZh:
			entry.Zh = def
		case localeEn:
			entry.En = def
		}
	}
	return nil
}

// parseSubagentFile loads one role markdown file and validates its
// frontmatter (name and description required).
func parseSubagentFile(path, slug, division, locale string) (*SubagentDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("subagents: read %s: %w", path, err)
	}
	fmSource, body, ok := splitFrontmatter(data)
	if !ok {
		return nil, fmt.Errorf("subagents: %s: missing --- frontmatter fence", path)
	}
	var fm SubagentFrontmatter
	if err := yaml.Unmarshal([]byte(fmSource), &fm); err != nil {
		return nil, fmt.Errorf("subagents: parse frontmatter in %s: %w", path, err)
	}
	if strings.TrimSpace(fm.Name) == "" {
		return nil, fmt.Errorf("subagents: %s: frontmatter name is empty", path)
	}
	if strings.TrimSpace(fm.Description) == "" {
		return nil, fmt.Errorf("subagents: %s: frontmatter description is empty", path)
	}
	return &SubagentDefinition{
		Slug:        slug,
		Division:    division,
		Locale:      locale,
		Frontmatter: fm,
		Body:        body,
	}, nil
}

// splitFrontmatter splits a role file into its YAML frontmatter source and
// the markdown body below the closing fence. The opening fence must be the
// file's first line; the closing fence is a line that is exactly "---"
// (a trailing \r is tolerated). body is the bytes after the closing fence
// line, verbatim. ok is false when no complete fence pair exists.
func splitFrontmatter(data []byte) (fm string, body []byte, ok bool) {
	const fence = frontmatterFence
	if !bytes.HasPrefix(data, []byte(fence)) {
		return "", nil, false
	}
	rest := data[len(fence):]
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 || len(bytes.TrimSuffix(rest[:nl], []byte("\r"))) != 0 {
		return "", nil, false // unterminated fence or a "----"-style line
	}
	rest = rest[nl+1:]

	off := 0
	for {
		idx := bytes.IndexByte(rest[off:], '\n')
		next := len(rest)
		var line []byte
		if idx >= 0 {
			line = rest[off : off+idx]
			next = off + idx + 1
		} else {
			line = rest[off:]
		}
		if string(bytes.TrimSuffix(line, []byte("\r"))) == fence {
			return string(rest[:off]), rest[next:], true
		}
		if idx < 0 {
			return "", nil, false // no closing fence
		}
		off = next
	}
}

// parseDivisions decodes divisions.json preserving the "divisions" object's
// document order (Go maps lose key order, and the catalog API is ordered).
// Keys other than "divisions" (Octop's "_note") are skipped. An empty label
// falls back to the division slug, matching Octop's reader.
func parseDivisions(data []byte) ([]DivisionInfo, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("expected a JSON object")
	}

	var divisions []DivisionInfo
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		if key != "divisions" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, err
			}
			continue
		}

		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if d, ok := tok.(json.Delim); !ok || d != '{' {
			return nil, fmt.Errorf(`"divisions" must be a JSON object`)
		}
		for dec.More() {
			slugTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			slug, _ := slugTok.(string)
			var fields struct {
				Label string `json:"label"`
				Icon  string `json:"icon"`
				Color string `json:"color"`
			}
			if err := dec.Decode(&fields); err != nil {
				return nil, fmt.Errorf("division %q: %w", slug, err)
			}
			if fields.Label == "" {
				fields.Label = slug
			}
			divisions = append(divisions, DivisionInfo{
				Slug:  slug,
				Label: fields.Label,
				Icon:  fields.Icon,
				Color: fields.Color,
			})
		}
		if _, err := dec.Token(); err != nil { // consume the closing '}'
			return nil, err
		}
	}
	if len(divisions) == 0 {
		return nil, fmt.Errorf(`no "divisions" object found`)
	}
	return divisions, nil
}

// Resolve returns the definition for slug following the locale fallback
// chain requested-locale → en → zh (Global Constraints: Octop ships 55
// zh-only roles, so en users still see them). It returns nil when the slug
// is unknown. A nil catalog resolves nothing.
func (c *Catalog) Resolve(slug, locale string) *SubagentDefinition {
	if c == nil {
		return nil
	}
	entry := c.BySlug[slug]
	if entry == nil {
		return nil
	}
	for _, candidate := range []string{locale, localeEn, localeZh} {
		switch candidate {
		case localeZh:
			if entry.Zh != nil {
				return entry.Zh
			}
		case localeEn:
			if entry.En != nil {
				return entry.En
			}
		}
	}
	return nil
}

// ParseRoleTools splits a role's tools frontmatter value into tool names.
// The library mixes ASCII commas ("WebFetch, WebSearch") and the CJK
// fullwidth separator 、("阅读、写作、编辑"); both split. Fragments are
// trimmed, empties dropped, and duplicates removed preserving the first
// occurrence.
func ParseRoleTools(raw string) []string {
	seen := make(map[string]struct{})
	var tools []string
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '、' }) {
		tool := strings.TrimSpace(field)
		if tool == "" {
			continue
		}
		if _, dup := seen[tool]; dup {
			continue
		}
		seen[tool] = struct{}{}
		tools = append(tools, tool)
	}
	return tools
}

var (
	builtinOnce    sync.Once
	builtinCatalog *Catalog
)

// LoadBuiltinSubagents returns the process-level cached scan of the builtin
// role library at <ConfigDir()>/subagents/library (the scan root IS the
// library containing zh/ and en/, matching ScanSubagents semantics). The
// scan runs on the first call, never at import time. A missing directory is
// the normal "no library shipped" case; any other scan error is logged and
// swallowed — the library is optional data and must never block startup
// (degraded mode: an empty, non-nil catalog).
//
// The returned catalog is shared; treat it as read-only.
func LoadBuiltinSubagents() *Catalog {
	builtinOnce.Do(func() {
		builtinCatalog = loadBuiltinSubagentsFrom(
			filepath.Join(config.ConfigDir(), "subagents", "library"))
	})
	return builtinCatalog
}

// loadBuiltinSubagentsFrom performs the degraded-mode load of dir used by
// LoadBuiltinSubagents (exposed separately for tests).
func loadBuiltinSubagentsFrom(dir string) *Catalog {
	catalog, err := ScanSubagents(dir)
	if err != nil {
		// Absence is normal; anything else is worth a warning but not fatal.
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Warnf(context.Background(),
				"[Subagents] builtin catalog unavailable (dir=%s): %v", dir, err)
		}
		return &Catalog{Divisions: []DivisionInfo{}, BySlug: map[string]*CatalogEntry{}}
	}
	return catalog
}
