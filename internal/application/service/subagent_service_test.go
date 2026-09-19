package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/subagents"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// fakeSubagentRepo stands in for repository.TenantSubagentRepository: an
// in-memory row store the install path upserts into.
type fakeSubagentRepo struct {
	rows    []types.TenantSubagentEntity
	upserts int
}

func (r *fakeSubagentRepo) Upsert(_ context.Context, e *types.TenantSubagentEntity) error {
	r.upserts++
	for i := range r.rows {
		if r.rows[i].TenantID == e.TenantID && r.rows[i].Slug == e.Slug && r.rows[i].Locale == e.Locale {
			r.rows[i] = *e
			return nil
		}
	}
	r.rows = append(r.rows, *e)
	return nil
}

func (r *fakeSubagentRepo) ListByTenant(_ context.Context, tenantID uint64) ([]types.TenantSubagentEntity, error) {
	var out []types.TenantSubagentEntity
	for _, row := range r.rows {
		if row.TenantID == tenantID {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *fakeSubagentRepo) Delete(_ context.Context, tenantID uint64, slug string) error {
	kept := r.rows[:0]
	for _, row := range r.rows {
		if row.TenantID != tenantID || row.Slug != slug {
			kept = append(kept, row)
		}
	}
	r.rows = kept
	return nil
}

// fakeSubagentAgentService stands in for interfaces.CustomAgentService: only
// GetAgentByID/UpdateAgent are implemented (the persona Apply pattern).
type fakeSubagentAgentService struct {
	interfaces.CustomAgentService
	agents  map[string]*types.CustomAgent
	updated []*types.CustomAgent
}

func (s *fakeSubagentAgentService) GetAgentByID(_ context.Context, id string) (*types.CustomAgent, error) {
	if a, ok := s.agents[id]; ok {
		return a, nil
	}
	return nil, ErrAgentNotFound
}

func (s *fakeSubagentAgentService) UpdateAgent(_ context.Context, agent *types.CustomAgent) (*types.CustomAgent, error) {
	s.updated = append(s.updated, agent)
	return agent, nil
}

// newSubagentServiceForTest wires the real subagentService over the fixture
// library (the subagents package's testdata tree) and in-memory fakes.
func newSubagentServiceForTest(t *testing.T) (interfaces.SubagentService, *fakeSubagentRepo, *fakeSubagentAgentService) {
	t.Helper()
	catalog, err := subagents.ScanSubagents("../../agent/subagents/testdata/subagents")
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeSubagentRepo{}
	agents := &fakeSubagentAgentService{agents: map[string]*types.CustomAgent{
		"a1": {ID: "a1", TenantID: 7, Name: "Agent One"},
	}}
	svc := NewSubagentService(func() *subagents.Catalog { return catalog }, agents, repo)
	return svc, repo, agents
}

// TestSubagentListCatalogInstallsFromRows pins the list projection over the
// fixture library: sorted entries, per-locale name presence, division counts
// in divisions.json order, total, and installed flags from tenant rows.
func TestSubagentListCatalogInstallsFromRows(t *testing.T) {
	svc, repo, _ := newSubagentServiceForTest(t)
	repo.rows = []types.TenantSubagentEntity{
		{TenantID: 7, Slug: "product-manager", Locale: "zh"},
		{TenantID: 9, Slug: "researcher", Locale: "zh"}, // other tenant: no flag
	}

	out, err := svc.ListCatalog(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 {
		t.Fatalf("total = %d, want 2", out.Total)
	}
	if len(out.Entries) != 2 || out.Entries[0].Slug != "product-manager" || out.Entries[1].Slug != "researcher" {
		t.Fatalf("entries = %+v, want sorted [product-manager researcher]", out.Entries)
	}
	pm := out.Entries[0]
	if !pm.Installed || pm.NameEn != "Product Manager" || pm.NameZh != "产品经理" || pm.Division != "product" {
		t.Fatalf("product-manager = %+v", pm)
	}
	res := out.Entries[1]
	if res.Installed || res.NameEn != "" {
		t.Fatalf("researcher = %+v, want not installed and empty en name (zh-only role)", res)
	}
	if len(out.Divisions) != 3 {
		t.Fatalf("divisions = %d, want 3", len(out.Divisions))
	}
	if out.Divisions[0].Slug != "product" || out.Divisions[0].Count != 2 || out.Divisions[0].Label != "产品" {
		t.Fatalf("product division = %+v, want count 2 in divisions.json order", out.Divisions[0])
	}
	if out.Divisions[1].Slug != "academic" || out.Divisions[1].Count != 0 || out.Divisions[2].Slug != "engineering" {
		t.Fatalf("divisions = %+v, want academic(0) then engineering(0)", out.Divisions)
	}
}

// TestSubagentGetCatalogEntry pins the detail projection: both-locale bodies
// present-or-empty, frontmatter scalars, and the unknown-slug sentinel.
func TestSubagentGetCatalogEntry(t *testing.T) {
	svc, _, _ := newSubagentServiceForTest(t)

	d, err := svc.GetCatalogEntry(context.Background(), 7, "product-manager")
	if err != nil {
		t.Fatal(err)
	}
	if d.Slug != "product-manager" || d.Division != "product" {
		t.Fatalf("detail = %+v", d)
	}
	if !strings.Contains(d.BodyZh, "# 🧭 产品经理代理") || !strings.Contains(d.BodyEn, "You are Alex") {
		t.Fatalf("bodies = zh:%q en:%q", d.BodyZh, d.BodyEn)
	}
	if d.ToolsRaw == "" || d.Vibe == "" || d.Emoji != "🧭" || d.Color != "blue" {
		t.Fatalf("frontmatter scalars = %+v", d)
	}
	if d.Installed {
		t.Fatal("installed should be false without tenant rows")
	}

	d, err = svc.GetCatalogEntry(context.Background(), 7, "researcher")
	if err != nil {
		t.Fatal(err)
	}
	if d.NameEn != "" || d.BodyEn != "" || d.NameZh != "用户研究员" {
		t.Fatalf("zh-only detail = %+v, want empty en fields", d)
	}

	if _, err = svc.GetCatalogEntry(context.Background(), 7, "ghost"); err != ErrSubagentNotFound {
		t.Fatalf("unknown slug err = %v, want ErrSubagentNotFound", err)
	}
}

// TestSubagentInstallForAgent pins the install semantics: resolved-role
// upsert (fresh UUID, verbatim markdown that round-trips through
// ParseRoleMarkdown, resolved locale, builtin source), config append with
// order preserved, idempotent reinstall, and the 404 sentinels.
func TestSubagentInstallForAgent(t *testing.T) {
	ctx := context.Background()
	svc, repo, agents := newSubagentServiceForTest(t)
	agents.agents["a1"].Config.Subagents = []string{"researcher"}

	// Explicit zh locale resolves the zh definition.
	got, err := svc.InstallForAgent(ctx, 7, "a1", "product-manager", "zh-CN")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "researcher" || got[1] != "product-manager" {
		t.Fatalf("subagents = %v, want [researcher product-manager]", got)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("tenant rows = %d, want 1", len(repo.rows))
	}
	row := repo.rows[0]
	if row.TenantID != 7 || row.Slug != "product-manager" || row.Locale != "zh" ||
		row.Division != "product" || row.Source != types.SubagentSourceBuiltin {
		t.Fatalf("row = %+v", row)
	}
	if row.ID == "" || len(row.ID) > 36 {
		t.Fatalf("row ID = %q, want a fresh UUID (<=36 chars)", row.ID)
	}
	if !strings.HasPrefix(row.Content, "---\n") || !strings.Contains(row.Content, "name: 产品经理") {
		t.Fatalf("content missing verbatim frontmatter: %q", row.Content)
	}
	fm, body, err := subagents.ParseRoleMarkdown(row.Content)
	if err != nil {
		t.Fatalf("stored content does not round-trip: %v", err)
	}
	if fm.Name != "产品经理" || !strings.Contains(body, "你是 Alex，一位经验丰富的产品经理。") {
		t.Fatalf("round-trip = %+v body %q", fm, body)
	}
	if len(agents.updated) != 1 {
		t.Fatalf("UpdateAgent called %d times, want 1", len(agents.updated))
	}

	// Empty locale falls back to the request language (no ctx language here →
	// the zh-CN default → "zh") and the zh-only researcher resolves its zh
	// definition. The slug is ALREADY configured, so this exercises the
	// refresh-only path: row upserted, config untouched.
	got, err = svc.InstallForAgent(ctx, 7, "a1", "researcher", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "researcher" || got[1] != "product-manager" {
		t.Fatalf("subagents = %v, want unchanged [researcher product-manager]", got)
	}
	researcherRow := repo.rows[1]
	if researcherRow.Slug != "researcher" || researcherRow.Locale != "zh" ||
		!strings.Contains(researcherRow.Content, "用户研究员") {
		t.Fatalf("researcher row = %+v, want the zh definition (en absent in fixture, en→zh fallback)", researcherRow)
	}
	if researcherRow.ID == row.ID {
		t.Fatal("each upserted slot must mint its own fresh UUID")
	}

	// Explicit en locale resolves the en definition of the bilingual slug into
	// its own (slug, locale) slot.
	got, err = svc.InstallForAgent(ctx, 7, "a1", "product-manager", "en-US")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("subagents = %v, want unchanged (already configured)", got)
	}
	enRow := repo.rows[2]
	if enRow.Slug != "product-manager" || enRow.Locale != "en" ||
		!strings.Contains(enRow.Content, "You are Alex") {
		t.Fatalf("en row = %+v, want the en definition under its own locale slot", enRow)
	}

	// Idempotent reinstall: same list, row re-upserted under the same key
	// (rows stay 3), no config change.
	got, err = svc.InstallForAgent(ctx, 7, "a1", "product-manager", "zh")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("reinstall subagents = %v, want unchanged", got)
	}
	if repo.upserts != 4 || len(repo.rows) != 3 {
		t.Fatalf("upserts = %d rows = %d, want 4 upserts over 3 slots", repo.upserts, len(repo.rows))
	}
	if repo.rows[0].Locale != "zh" || !strings.Contains(repo.rows[0].Content, "产品经理") {
		t.Fatalf("zh slot after reinstall = %+v, want refreshed in place", repo.rows[0])
	}
	if len(agents.updated) != 1 {
		t.Fatalf("UpdateAgent called %d times, want 1 (reinstalls must not rewrite config)", len(agents.updated))
	}

	// Unknown slug / unknown agent → sentinels, nothing written.
	if _, err = svc.InstallForAgent(ctx, 7, "a1", "ghost", "zh"); err != ErrSubagentNotFound {
		t.Fatalf("unknown slug err = %v, want ErrSubagentNotFound", err)
	}
	if _, err = svc.InstallForAgent(ctx, 7, "nope", "researcher", "zh"); err != ErrAgentNotFound {
		t.Fatalf("unknown agent err = %v, want ErrAgentNotFound", err)
	}
	if repo.upserts != 4 || len(agents.updated) != 1 {
		t.Fatalf("failed installs must not write: upserts = %d updates = %d", repo.upserts, len(agents.updated))
	}
}

// TestSubagentListAndRemoveFromAgent pins the agent-config endpoints:
// config-order listing, removal (absent → idempotent no-op) and the tenant
// rows surviving removal (shared across agents).
func TestSubagentListAndRemoveFromAgent(t *testing.T) {
	ctx := context.Background()
	svc, repo, agents := newSubagentServiceForTest(t)
	agents.agents["a1"].Config.Subagents = []string{"researcher", "product-manager"}
	repo.rows = []types.TenantSubagentEntity{{TenantID: 7, Slug: "researcher", Locale: "zh"}}

	got, err := svc.ListAgentSubagents(ctx, "a1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "researcher" {
		t.Fatalf("list = %v, want config order preserved", got)
	}

	got, err = svc.RemoveFromAgent(ctx, "a1", "researcher")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "product-manager" {
		t.Fatalf("after remove = %v, want [product-manager]", got)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("tenant rows = %d, want 1 (rows are shared across agents and kept)", len(repo.rows))
	}

	// Absent slug: 200-idempotent no-op — no UpdateAgent, same list.
	got, err = svc.RemoveFromAgent(ctx, "a1", "researcher")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("idempotent remove = %v, want unchanged", got)
	}
	if len(agents.updated) != 1 {
		t.Fatalf("UpdateAgent called %d times, want 1", len(agents.updated))
	}

	if _, err = svc.ListAgentSubagents(ctx, "nope"); err != ErrAgentNotFound {
		t.Fatalf("unknown agent err = %v, want ErrAgentNotFound", err)
	}
	if _, err = svc.RemoveFromAgent(ctx, "nope", "researcher"); err != ErrAgentNotFound {
		t.Fatalf("unknown agent err = %v, want ErrAgentNotFound", err)
	}
}
