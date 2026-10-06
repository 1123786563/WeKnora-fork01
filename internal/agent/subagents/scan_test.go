package subagents

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fixtureRoot is the happy-path library fixture: zh + en locales, a slug in
// both locales (product-manager), a zh-only slug (researcher), an empty
// division directory (engineering, kept alive by a non-md file), a stray
// README.md at the library root, and a divisions.json whose "_note" key and
// non-alphabetical key order must both be tolerated.
func fixtureRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("testdata", "subagents")
}

// copyFixture copies the fixture library into a writable temp dir that
// negative tests can corrupt.
func copyFixture(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := fixtureRoot(t)
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copy fixture: %v", err)
	}
	return dst
}

func TestScanSubagentsHappyPath(t *testing.T) {
	catalog, err := ScanSubagents(fixtureRoot(t))
	if err != nil {
		t.Fatalf("ScanSubagents: %v", err)
	}

	// Divisions mirror divisions.json in document order (fixture order is
	// deliberately non-alphabetical) with label/icon/color intact.
	wantDivisions := []DivisionInfo{
		{Slug: "product", Label: "产品", Icon: "Box", Color: "#D946EF"},
		{Slug: "academic", Label: "学术", Icon: "GraduationCap", Color: "#8B5CF6"},
		{Slug: "engineering", Label: "工程", Icon: "Code", Color: "#3B82F6"},
	}
	if !reflect.DeepEqual(catalog.Divisions, wantDivisions) {
		t.Errorf("Divisions = %+v, want %+v", catalog.Divisions, wantDivisions)
	}

	// Two slugs total: product-manager (zh+en) and researcher (zh only).
	if len(catalog.BySlug) != 2 {
		t.Fatalf("BySlug size = %d, want 2: %v", len(catalog.BySlug), catalog.BySlug)
	}

	pm, ok := catalog.BySlug["product-manager"]
	if !ok {
		t.Fatal("BySlug missing product-manager")
	}
	if pm.Division != "product" || pm.Slug != "product-manager" {
		t.Errorf("entry slug/division = %q/%q", pm.Slug, pm.Division)
	}
	if pm.Zh == nil || pm.En == nil {
		t.Fatalf("product-manager must pair zh+en, got zh=%v en=%v", pm.Zh, pm.En)
	}
	if pm.Zh.Frontmatter.Name != "产品经理" {
		t.Errorf("zh name = %q, want 产品经理", pm.Zh.Frontmatter.Name)
	}
	if pm.En.Frontmatter.Name != "Product Manager" {
		t.Errorf("en name = %q, want Product Manager", pm.En.Frontmatter.Name)
	}
	if pm.Zh.Locale != "zh" || pm.En.Locale != "en" {
		t.Errorf("locale stamps = %q/%q, want zh/en", pm.Zh.Locale, pm.En.Locale)
	}
	if pm.Zh.Division != "product" || pm.En.Division != "product" {
		t.Errorf("definition divisions = %q/%q", pm.Zh.Division, pm.En.Division)
	}

	// Frontmatter quirks: CJK fullwidth 、 in ToolsRaw stays verbatim.
	if pm.Zh.Frontmatter.ToolsRaw != "WebFetch, WebSearch, Read, Write, Edit" {
		t.Errorf("zh ToolsRaw = %q", pm.Zh.Frontmatter.ToolsRaw)
	}
	if pm.Zh.Frontmatter.Emoji != "🧭" || pm.Zh.Frontmatter.Color != "blue" {
		t.Errorf("zh emoji/color = %q/%q", pm.Zh.Frontmatter.Emoji, pm.Zh.Frontmatter.Color)
	}
	if pm.Zh.Frontmatter.Vibe != "交付正确的东西，而不仅仅是下一个东西。" {
		t.Errorf("zh vibe = %q", pm.Zh.Frontmatter.Vibe)
	}

	// Body is the markdown below the frontmatter fence line, bytes
	// verbatim — the fixture's blank line after the fence survives.
	wantZhBody := "\n# 🧭 产品经理代理\n\n你是 Alex，一位经验丰富的产品经理。\n"
	if string(pm.Zh.Body) != wantZhBody {
		t.Errorf("zh body = %q, want %q", pm.Zh.Body, wantZhBody)
	}
	wantEnBody := "\n# 🧭 Product Manager Agent\n\nYou are Alex, a seasoned product manager.\n"
	if string(pm.En.Body) != wantEnBody {
		t.Errorf("en body = %q, want %q", pm.En.Body, wantEnBody)
	}

	// zh-only slug: En pointer nil, division carried from zh.
	res, ok := catalog.BySlug["researcher"]
	if !ok {
		t.Fatal("BySlug missing researcher")
	}
	if res.En != nil {
		t.Errorf("researcher.En = %+v, want nil", res.En)
	}
	if res.Zh == nil {
		t.Fatal("researcher.Zh is nil")
	}
	if res.Zh.Frontmatter.Name != "用户研究员" {
		t.Errorf("researcher zh name = %q", res.Zh.Frontmatter.Name)
	}
	if res.Zh.Frontmatter.ToolsRaw != "阅读、写作、编辑" {
		t.Errorf("researcher ToolsRaw = %q, want 阅读、写作、编辑", res.Zh.Frontmatter.ToolsRaw)
	}
	if res.Division != "product" {
		t.Errorf("researcher division = %q, want product", res.Division)
	}
}

func TestCatalogResolveFallbackChain(t *testing.T) {
	catalog, err := ScanSubagents(fixtureRoot(t))
	if err != nil {
		t.Fatalf("ScanSubagents: %v", err)
	}

	// Requested locale wins when present.
	if got := catalog.Resolve("product-manager", "zh"); got.Frontmatter.Name != "产品经理" {
		t.Errorf("Resolve(zh) name = %q, want 产品经理", got.Frontmatter.Name)
	}
	if got := catalog.Resolve("product-manager", "en"); got.Frontmatter.Name != "Product Manager" {
		t.Errorf("Resolve(en) name = %q, want Product Manager", got.Frontmatter.Name)
	}

	// Fallback chain: requested → en → zh. A zh-only slug requested as en
	// resolves to its zh definition; an unknown locale falls through en.
	if got := catalog.Resolve("researcher", "en"); got == nil || got.Locale != "zh" {
		t.Errorf("Resolve(researcher, en) = %+v, want the zh definition", got)
	}
	if got := catalog.Resolve("researcher", "fr"); got == nil || got.Locale != "zh" {
		t.Errorf("Resolve(researcher, fr) = %+v, want the zh definition", got)
	}
	if got := catalog.Resolve("product-manager", "fr"); got == nil || got.Locale != "en" {
		t.Errorf("Resolve(product-manager, fr) = %+v, want the en definition", got)
	}

	// Unknown slug resolves to nil.
	if got := catalog.Resolve("no-such-role", "zh"); got != nil {
		t.Errorf("Resolve(unknown slug) = %+v, want nil", got)
	}
}

func TestParseRoleTools(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{"WebFetch, WebSearch", []string{"WebFetch", "WebSearch"}},
		{"阅读、写作、编辑", []string{"阅读", "写作", "编辑"}},
		// Both separators mixed, stray spaces trimmed, dedupe keeps first.
		{"Read、Write, Edit , Read", []string{"Read", "Write", "Edit"}},
		{"", nil},
		{" , 、 ", nil},
		{"WebFetch", []string{"WebFetch"}},
	}
	for _, tc := range cases {
		if got := ParseRoleTools(tc.raw); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseRoleTools(%q) = %#v, want %#v", tc.raw, got, tc.want)
		}
	}
}

func TestScanSubagentsMissingFrontmatter(t *testing.T) {
	root := copyFixture(t)
	broken := filepath.Join(root, "zh", "product", "broken.md")
	if err := os.WriteFile(broken, []byte("# No frontmatter here\n\nJust a heading.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ScanSubagents(root)
	if err == nil {
		t.Fatal("want error for md without frontmatter, got nil")
	}
	if !strings.Contains(err.Error(), "broken.md") {
		t.Errorf("error must name the file, got: %v", err)
	}
}

func TestScanSubagentsMissingName(t *testing.T) {
	root := copyFixture(t)
	broken := filepath.Join(root, "zh", "product", "broken.md")
	md := "---\ndescription: 有描述但没有名字。\n---\n\n# 缺名字\n"
	if err := os.WriteFile(broken, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ScanSubagents(root)
	if err == nil {
		t.Fatal("want error for missing frontmatter name, got nil")
	}
	if !strings.Contains(err.Error(), "broken.md") || !strings.Contains(err.Error(), "name") {
		t.Errorf("error must name file and field, got: %v", err)
	}
}

func TestScanSubagentsMissingDescription(t *testing.T) {
	root := copyFixture(t)
	broken := filepath.Join(root, "zh", "product", "broken.md")
	md := "---\nname: 有名字\n---\n\n# 缺描述\n"
	if err := os.WriteFile(broken, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ScanSubagents(root)
	if err == nil {
		t.Fatal("want error for missing frontmatter description, got nil")
	}
	if !strings.Contains(err.Error(), "broken.md") || !strings.Contains(err.Error(), "description") {
		t.Errorf("error must name file and field, got: %v", err)
	}
}

func TestScanSubagentsUnknownDivision(t *testing.T) {
	root := copyFixture(t)
	dir := filepath.Join(root, "zh", "mystery")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: X\ndescription: Y\n---\n\n# X\n"
	if err := os.WriteFile(filepath.Join(dir, "ghost.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ScanSubagents(root)
	if err == nil {
		t.Fatal("want error for division not in divisions.json, got nil")
	}
	if !strings.Contains(err.Error(), "mystery") {
		t.Errorf("error must name the division, got: %v", err)
	}
}

func TestScanSubagentsDivisionsJSONMissing(t *testing.T) {
	root := copyFixture(t)
	if err := os.Remove(filepath.Join(root, "zh", "divisions.json")); err != nil {
		t.Fatal(err)
	}
	_, err := ScanSubagents(root)
	if err == nil {
		t.Fatal("want error for missing divisions.json, got nil")
	}
}

func TestScanSubagentsMissingDir(t *testing.T) {
	_, err := ScanSubagents(filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("want error for missing library dir, got nil")
	}
}

func TestScanSubagentsDuplicateSlugAcrossDivisions(t *testing.T) {
	root := copyFixture(t)
	// product-manager already lives in zh/product; duplicating the slug in
	// another zh division must be an error even with valid frontmatter.
	dir := filepath.Join(root, "zh", "academic")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nname: 学术产品经理\ndescription: 另一个部门的同名角色。\n---\n\n# 冲突\n"
	if err := os.WriteFile(filepath.Join(dir, "product-manager.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ScanSubagents(root)
	if err == nil {
		t.Fatal("want error for slug in multiple divisions within one locale, got nil")
	}
	if !strings.Contains(err.Error(), "product-manager") {
		t.Errorf("error must name the slug, got: %v", err)
	}
}

func TestSplitFrontmatterEdges(t *testing.T) {
	cases := []struct {
		name  string
		data  string
		fm    string
		body  string
		found bool
	}{
		{"typical", "---\nname: X\n---\n# H\n", "name: X\n", "# H\n", true},
		{"closing fence without trailing newline", "---\nname: X\n---", "name: X\n", "", true},
		{"no opening fence", "# H\n", "", "", false},
		{"opening fence only", "---\nname: X", "", "", false},
		{"dash-run line is not a fence", "----\nname: X\n---\n", "", "", false},
		{"crlf fences", "---\r\nname: X\r\n---\r\n# H\r\n", "name: X\r\n", "# H\r\n", true},
	}
	for _, tc := range cases {
		fm, body, ok := splitFrontmatter([]byte(tc.data))
		if ok != tc.found || string(fm) != tc.fm || string(body) != tc.body {
			t.Errorf("%s: splitFrontmatter(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tc.name, tc.data, fm, body, ok, tc.fm, tc.body, tc.found)
		}
	}
}

func TestLoadBuiltinSubagentsFromDegradedEmpty(t *testing.T) {
	// Absent library directory is the normal "not shipped" case: a
	// non-nil, empty catalog — never an error, never nil.
	catalog := loadBuiltinSubagentsFrom(filepath.Join(t.TempDir(), "no-such-library"))
	if catalog == nil {
		t.Fatal("catalog must be non-nil in degraded mode")
	}
	if len(catalog.BySlug) != 0 || len(catalog.Divisions) != 0 {
		t.Errorf("degraded catalog must be empty, got %+v", catalog)
	}
	if catalog.Resolve("product-manager", "zh") != nil {
		t.Error("degraded catalog must resolve nothing")
	}
}

func TestLoadBuiltinSubagentsFromFixture(t *testing.T) {
	catalog := loadBuiltinSubagentsFrom(fixtureRoot(t))
	if catalog == nil {
		t.Fatal("catalog is nil")
	}
	if len(catalog.BySlug) != 2 {
		t.Errorf("BySlug size = %d, want 2", len(catalog.BySlug))
	}
	if got := catalog.Resolve("researcher", "en"); got == nil || got.Locale != "zh" {
		t.Errorf("Resolve(researcher, en) = %+v, want zh definition", got)
	}
}
