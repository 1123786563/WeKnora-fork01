// Package subagents implements the builtin sub-agent role catalog: Octop
// role markdown files under config/subagents/library/{zh,en}/<division>/
// paired by filename stem across locales, plus the division metadata from
// zh/divisions.json.
//
// The package is pure logic and data: no import-time disk IO — scanning is
// explicit (ScanSubagents) or lazy on first use (LoadBuiltinSubagents),
// mirroring the experts package's loading model.
package subagents

// SubagentFrontmatter is the parsed YAML frontmatter of one role markdown
// file. ToolsRaw is the verbatim comma list (ASCII "," and/or CJK "、") —
// callers parse it lazily via ParseRoleTools so this package stores the
// library bytes without interpretation.
type SubagentFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Color       string `yaml:"color"`
	Emoji       string `yaml:"emoji"`
	Vibe        string `yaml:"vibe"`
	ToolsRaw    string `yaml:"tools"`
}

// SubagentDefinition is one role file: the zh or en variant of a slug.
// Body is the markdown below the frontmatter fence, bytes verbatim
// (first heading included) — it becomes the sub-run system prompt.
type SubagentDefinition struct {
	Slug        string // filename stem, shared across locales for pairing
	Division    string // division directory name under the locale root
	Locale      string // "zh" | "en"
	Frontmatter SubagentFrontmatter
	Body        []byte
}

// DivisionInfo describes one division from zh/divisions.json.
type DivisionInfo struct {
	Slug  string
	Label string // falls back to the slug when divisions.json omits it
	Icon  string // Lucide icon name, PascalCase
	Color string // hex brand color
}

// CatalogEntry is one slug with its per-locale definitions. At least one
// of Zh, En is non-nil (the scanner guarantees it: an entry exists only
// after a file was scanned).
type CatalogEntry struct {
	Slug     string
	Division string
	Zh       *SubagentDefinition
	En       *SubagentDefinition
}

// Catalog is the scanned role library. Divisions mirrors divisions.json in
// document order (not sorted); BySlug indexes every slug found in any
// locale. Values are shared process-wide once cached by
// LoadBuiltinSubagents — treat them as read-only.
type Catalog struct {
	Divisions []DivisionInfo
	BySlug    map[string]*CatalogEntry
}
