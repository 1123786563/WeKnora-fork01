# Sub-agents (M3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate Octop's sub-agent capability into WeKnora: the 272/217 zh/en role library as data under config/subagents/, a tenant install model (tenant_subagents table + per-agent Subagents reference list), a `subagent_delegate` tool on the main agent (strict {slug, goal, input_refs} surface, role tools ∩ agent AllowedTools whitelist, independent trpc graph execution with round/char budgets, 8 KiB summary back to the main context, full trace on session events), catalog/install APIs, and an agent-editor management tab.

**Architecture:** Pure-data catalog package `internal/agent/subagents` (scan + locale merge, no import-time IO) → `tenant_subagents` table (SQL migration pair) + repository → `CustomAgentConfig.Subagents []string` drives conditional registration of one `subagent_delegate` tool in the existing registerTools chain → executor runs a lightweight trpc graph (pattern: internal/agent/trpc/compatibility_probe.go) with the role markdown as system prompt, an intersected tool registry, a fresh session id (noop session service = natural history isolation), budget caps, and event emission → handler/service expose catalog + install endpoints → api-client module + editor tab.

**Tech Stack:** Go 1.26 / gin / gorm (SQL-migration-only table) / trpc-agent-go graph (SDK-level, not the durable GraphRunner) / React 19 editor section (M1 personalization precedent).

**Spec:** `docs/superpowers/specs/2026-09-19-octop-capabilities-migration-design.md` §7 (Sub-agents). Octop source: `/Users/wuyongjun/trea/Octop/src/octop/infra/agents/subagents/` (catalog.py + library/{zh,en}/).

## Global Constraints

- Assets are DATA under `config/subagents/library/{zh,en}/<division>/*.md` verbatim from Octop + `zh/divisions.json`; frontmatter `name/description/color/emoji/vibe/tools`; slug = filename stem (zh/en pairing); no import-time IO (explicit scan, sync.Once cached, absence → degraded empty).
- Locale policy (data-reality-driven, supersedes spec's one-step fallback): request locale → en → zh. Octop ships zh 272 / en 217 — en lacks 55 roles, so a second fallback to zh is REQUIRED or those roles are uninstallable for en users. Recorded as a spec deviation with rationale.
- Tool-name mapping (role frontmatter → WeKnora registry names): the library's `tools` values are Claude-Code-style (`WebFetch, WebSearch, Read, Write, Edit, Bash`) or Chinese (`阅读、写作、编辑`). Map: WebSearch/搜索→`web_search`, WebFetch/网页→`web_fetch`, Read/阅读→`read_file`, Write/写作→`write_file`, Edit/编辑→`edit_file`, Bash/终端→`shell_command` — VERIFY each target name against `tools.DefaultAllowedTools()` (internal/agent/tools/definitions.go:116) at implementation; any role tool that maps to nothing WeKnora has, or that the main agent's AllowedTools excludes, is dropped from the sub-run with a warn log (never blocks delegation). The intersection is roleTools(mapped) ∩ mainAllowedTools.
- Delegate tool surface: args strictly `{slug string, goal string, input_refs []string}` — DisallowUnknownFields, reject trailing input, reject blank goal (craft_delegate.go:44 precedent); ALL authority (tenant/user/session/tool whitelist/budgets) server-assembled from the run context; model never sees an authority field. `input_refs` V1 semantics: plain strings appended as an "Additional context" block after the goal (no file-system authorization — that is Craft's mechanism, deliberately not reused here).
- Execution: independent lightweight trpc graph (NewStateGraph + LLM node + tools node, runner with noop session service, fresh sessionID string — pattern internal/agent/trpc/compatibility_probe.go:83-125); role markdown body = system prompt; budgets: rounds ≤ 8 (const subagentMaxRounds, per-role override via frontmatter not in V1) and cumulative output chars ≤ 32 KiB (const subagentMaxOutputChars — token proxy; tokenizer-free); overrun → stop, return partial with an "[budget exceeded]" suffix note.
- Result: summary returned to the main agent capped at 8 KiB (craftDelegateSummaryLimit precedent, tail-preserving truncation); full execution trace emitted as session events (EventAgentToolCall/EventAgentToolResult on the MAIN session id) so the transcript shows the sub-run.
- Failure semantics: unknown slug / not installed for agent → tool error returned to model (visible, not silent); executor error → error text returned as tool result (model can react); never panics the run.
- DB: `tenant_subagents` table via migrations/versioned/000163 + migrations/sqlite/000084 pair (next free numbers at plan time — VERIFY at implementation, main moves; template migrations/versioned/000162_native_agent_schema.up.sql). No GORM AutoMigrate — entity + TableName() only.
- API guards: catalog reads Viewer+ (apiKeyFullAccess precedent), agent-scoped install/remove via the existing agentsWrite guard family (M2 expert routes precedent); tenant from ctx always.
- JSON snake_case; gofmt/vet clean; frontend only in apps/web + packages/api-client + i18n three-point registration (REACT_SIDE_SOURCES precedent).
- Frontend: no hardcoded role data — catalog from API; editor section follows M1 Personalization conventions; section gated to agent-mode ONLY if delegation is agent-mode-only (it is — the tool registers on the ReAct/trpc engine path), mirroring the M1 gating precedent.

---

### Task 1: subagents catalog package — types + scanner + locale merge

**Files:**
- Create: `internal/agent/subagents/subagent.go` (types)
- Create: `internal/agent/subagents/scan.go` (scanner + divisions + locale merge + LoadBuiltinSubagents)
- Create: `internal/agent/subagents/testdata/subagents/{zh,en}/product/product-manager.md` (+ a zh-only slug fixture + divisions.json fixture)
- Test: `internal/agent/subagents/scan_test.go`

**Interfaces (Producing):**
- `type SubagentFrontmatter struct{ Name, Description, Color, Emoji, Vibe, ToolsRaw string }` (ToolsRaw = the comma list, parsed lazily)
- `type SubagentDefinition struct{ Slug, Division string; Locale string /* "zh"|"en" */; Frontmatter SubagentFrontmatter; Body []byte /* markdown below frontmatter */ }`
- `type DivisionInfo struct{ Slug, Label, Icon, Color string }`
- `type Catalog struct{ Divisions []DivisionInfo /* ordered by divisions.json key order */; BySlug map[string]*CatalogEntry }`
- `type CatalogEntry struct{ Slug, Division string; Zh, En *SubagentDefinition /* at least one non-nil */ }`
- `func ScanSubagents(dir string) (*Catalog, error)` — dir = the library root containing zh/ and en/ + zh/divisions.json; md files without frontmatter name+description → error naming the file; division dirs not in divisions.json → error; divisions.json missing → error; frontmatter parse per Octop format (yaml fence, tolerating the CJK-fullwidth commas 、in ToolsRaw).
- `func (c *Catalog) Resolve(slug, locale string) *SubagentDefinition` — locale→en→zh fallback (Global Constraints); nil when absent.
- `func ParseRoleTools(raw string) []string` — splits on both `,` and `、`, trims, dedupes preserving order.
- `func LoadBuiltinSubagents() *Catalog` — sync.Once over `<ConfigDir()>/subagents`, degraded-empty on error (experts precedent).

- [ ] Step 1: Failing tests — scan fixture (2 locales × product-manager, 1 zh-only slug, divisions fixture): pairing, division order, Resolve fallback chain (zh→en for zh-only slug requested as en... wait — fallback is request→en→zh, so a zh-only slug requested as "en" resolves to zh; a both-locale slug requested "en" resolves en), ParseRoleTools on `WebFetch, WebSearch` and `阅读、写作、编辑`, missing-frontmatter error, unknown-division error. Red.
- [ ] Step 2: Implement. Green + gofmt/vet.
- [ ] Step 3: Commit `feat(subagents): catalog types, scanner, and locale resolution`

### Task 2: Role library migration (272 zh + 217 en + divisions)

**Files:**
- Create: `scripts/import_octop_subagents.py`
- Create: `config/subagents/library/{zh,en}/...` + `config/subagents/library/zh/divisions.json` (generated)
- Test: `internal/agent/subagents/library_test.go`

**Import script essentials:** copy every `<locale>/<division>/*.md` verbatim (shutil.copytree of zh/ and en/ including zh/divisions.json); print counts; re-runnable (idempotent copy). NO content transformation — frontmatter stays Octop-native (including the CJK quirks the T1 report noted in Octop data).

**Library test (binding):** ScanSubagents(`<repo>/config/subagents/library`) succeeds; 19 divisions in divisions.json all present; BySlug size == distinct stems across locales (expect ≥272, compute expected from both dirs at test time via os.ReadDir rather than hardcoding); zh product-manager has non-empty Body and ToolsRaw contains "WebFetch"; every entry has at least one locale; Resolve("product-manager","en") non-nil. TDD red (dir absent) → run script → green.

Commit: `feat(subagents): import Octop role library (272 zh / 217 en, verbatim)`

### Task 3: tenant_subagents table + repository + config field

**Files:**
- Create: `migrations/versioned/000163_tenant_subagents.up.sql` + `.down.sql` (VERIFY next free number)
- Create: `migrations/sqlite/000084_tenant_subagents.up.sql` + `.down.sql` (VERIFY)
- Modify: `internal/types/tenant_subagent.go` (new): `TenantSubagentEntity{ID varchar(36) PK; TenantID uint64 PK-composite; Slug varchar(255); Locale varchar(8); Content text /* full md */; Division varchar(64); Source varchar(32) /* "builtin" */; CreatedAt/UpdatedAt; DeletedAt}` + `TableName() "tenant_subagents"`; unique index (tenant_id, slug, locale).
- Modify: `internal/types/custom_agent.go` — after Subagents-ready block placement: `Subagents []string `yaml:"subagents" json:"subagents,omitempty"`` (comment: delegation disabled when empty).
- Create: `internal/application/repository/tenant_subagent.go` — `TenantSubagentRepository` interface (Upsert(ctx, *TenantSubagentEntity), ListByTenant(ctx, tenantID) ([]TenantSubagentEntity, error), Delete(ctx, tenantID, slug) error) + gorm impl following tenant_skill.go conventions; wire into repository constructor/container per its pattern.
- Test: `internal/types/tenant_subagent_test.go` (JSON round-trip, Subagents round-trip + EnsureDefaults no-invent) + `internal/application/repository/tenant_subagent_test.go` (sqlite in-memory per repo test conventions — follow tenant_skill repo tests; upsert idempotent, list tenant-scoped, delete soft).

Commit: `feat(subagents): tenant_subagents table, entity, repository, agent config field`

### Task 4: Executor — independent trpc sub-run with budgets + trace

**Files:**
- Create: `internal/agent/subagents/executor.go`
- Test: `internal/agent/subagents/executor_test.go`

**Interfaces:**
- Consumes: `chat.Chat` (the run's model, passed in), `tools.ToolRegistry`-derived function definitions (passed in already intersected by the caller), event bus emit func, role definition.
- Produces:
```go
type ExecuteRequest struct {
    SessionID   string            // MAIN session id (for events + isolation naming)
    RunLabel    string            // e.g. "subagent:product-manager" (fresh trpc sessionID = SessionID + ":" + slug)
    SystemPrompt string           // role markdown body
    Goal        string
    InputRefs   []string
    Model       chat.Chat         // reused main-run model client
    Tools       []tools.Tool      // INTERSECTED set (caller-computed)
    Emit        func(ctx context.Context, e event.Event) // main-run event bus
}
type ExecuteResult struct {
    Summary   string // capped 8 KiB, tail-preserving; "[budget exceeded]" suffix note when overrun
    Rounds    int
    Overrun   bool
}
func Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error)
```
- Mechanics: lightweight graph per compatibility_probe.go:83-125 (NewStateGraph; LLM node with SystemPrompt+goal(+input-refs block); tools node; loop with round counter ≤ subagentMaxRounds=8; cumulative output chars ≤ subagentMaxOutputChars=32<<10); runner with `noop.NewService()` session service and the fresh sessionID (history isolation); every tool call/result ALSO emitted via Emit as EventAgentToolCall/EventAgentToolResult on the main SessionID with a `subagent:<slug>` tool-name prefix (e.g. `subagent:product-manager:web_search`) so the transcript attributes them; final assistant text = Summary source.
- Error contract: model/stream error → (ExecuteResult{}, err) with wrapped context; budget overrun is NOT an error (Overrun=true, partial returned); no tools → single LLM call (still valid delegation).

**Tests (fake chat.Chat per repo's existing fake model patterns — find one in internal/models or service tests):** single-round no-tools run returns text + Round=1; two-round tool loop (fake model emits tool call then final) executes the fake tool, emits both events with prefixed names, returns final; round overrun stops at 8 with Overrun; char overrun truncates with suffix; stream error propagates. TDD red→green.

Commit: `feat(subagents): budgeted trpc sub-run executor with event attribution`

### Task 5: subagent_delegate tool + tool-name mapping + registration wiring

**Files:**
- Create: `internal/agent/subagents/tools.go` (mapping + delegate tool)
- Modify: `internal/application/service/agent_service.go` registerTools chain (~:1136 area, craft_delegate precedent): when `config.Subagents` non-empty AND engine is ReAct/trpc path AND tenant installed-subagent lookup yields ≥1 match → register `subagent_delegate` via a `registerSubagentDelegateTool` helper in a new `internal/application/service/subagent_delegate.go`.
- Create: `internal/application/service/subagent_delegate.go` (assembly: resolve role md from tenant_subagents (installed) via catalog Resolve for the locale, compute intersection, build ExecuteRequest)
- Test: `internal/agent/subagents/tools_test.go`, `internal/application/service/subagent_delegate_test.go`

**Interfaces:**
- `func MapRoleTools(raw string) []string` — ParseRoleTools + name mapping table (Global Constraints; unmapped names dropped); table lives here as `var roleToolNameMap = map[string]string{...}` with the verified WeKnora names.
- Tool: `NewSubagentDelegateTool(cfg SubagentDelegateToolConfig)` — config carries Scope (tenant/user/session), AllowedTools []string (main agent's effective allowlist), Lookup func(slug) (ResolvedRole, error) (md body + mapped tools + label), Exec func(ctx, subagents.ExecuteRequest)(ExecuteResult, error). `ParseDelegateArgs` strict (DisallowUnknownFields/trailing/blank-goal, craft_delegate.go:44 pattern). Execute: resolve slug (unknown/not-installed → tool error "subagent not installed: <slug>"), intersect (empty intersection → run with no tools + note in summary header), call Exec, return summary as tool result text.
- Service assembly: locale from ctx; role md from TenantSubagentRepository row (Content) — the installed row IS the source (not re-resolved from catalog, so tenants pin what they installed); sub-run model = the same chat.Chat the main run uses (pass through from capabilities).
- Empty Subagents config → tool NOT registered (delegation off — spec).

**Tests:** mapping table (each source name + unknown dropped + 、-splitting); ParseDelegateArgs rejections; tool happy path (fake Lookup/Exec: result text passthrough, slug error path, empty-intersection note); registration condition test (with/without Subagents config; craft_delegate-style registration helper unit-tested in service package with fake repo). TDD.

Commit: `feat(subagents): subagent_delegate tool with intersected whitelist and conditional registration`

### Task 6: API — catalog + install/remove

**Files:**
- Create: `internal/handler/subagent.go`
- Create: `internal/router/routes_subagent.go` (+ router.go wiring, container construction per expert routes precedent)
- Create: `internal/application/service/subagent_service.go` (+ interface in types/interfaces/subagent.go)
- Test: `internal/handler/subagent_test.go`, `internal/router/routes_subagent_test.go`

**Routes (binding):**
- `GET /api/v1/subagent-catalog` (Viewer+): data {divisions:[DivisionInfo+count], total} — locale-filtered presentation is client-side (entries carry zh+en presence flags: `{slug, division, name_zh, name_en, emoji, color, installed bool}` — installed = tenant has row).
- `GET /api/v1/subagent-catalog/:slug` (Viewer+): both-locale bodies + frontmatter + installed state.
- `GET /api/v1/agents/:id/subagents` (OwnedAgentOrAdmin): data {subagents: [slugs]} from config.
- `POST /api/v1/agents/:id/subagents` (same guard): body `{slug, locale?}` strict-decoded; resolves via catalog (locale→en→zh), upserts tenant row (Content = resolved md VERBATIM incl. frontmatter), appends slug to agent config.Subagents (idempotent — duplicate install = no-op 200), returns {subagents: [...]}.
- `DELETE /api/v1/agents/:id/subagents/:slug`: removes from config list (tenant row kept — shared across agents; row deletion is tenant-admin scope, NOT this endpoint; document).
- 404s for unknown agent/slug; 400 unknown body fields.

Commit: `feat(subagents): catalog and agent install/remove routes`

### Task 7: api-client subagents module

**Files:** packages/api-client/src/subagents.ts (+ client.ts/index.ts mount, mbti/experts precedent).
Types: `SubagentCatalogEntry{slug,division,name_zh,name_en,emoji,color,installed}`, `SubagentDivision{slug,label,icon,color,count}`, `SubagentDetail` (+body_zh/body_en, vibe, tools_raw), functions `subagents.catalog()`, `subagents.get(slug)`, `subagents.listAgent(agentId)`, `subagents.install(agentId, slug, locale?)`, `subagents.remove(agentId, slug)`. Field parity vs T6 DTOs verified in review. Gate: web build. Commit `feat(api-client): subagents module`.

### Task 8: Editor tab — subagent management

**Files:**
- Create: `apps/web/src/agents/SubagentsSection.tsx`
- Modify: `agent-editor.ts` (AgentSectionKey 'subagents'; defaultAgentConfig `subagents: []`; buildNavGroups gated isAgentMode — M1 personalization precedent), `AgentEditorModal.tsx` (section case + valid list), `agents/route.ts` whitelist, i18n three-point registration (SUBAGENTS_VALUES block).
- Test: `agent-editor.test.tsx` additions.

**Behavior:** section shows installed list (config.subagents, removable) + catalog browser (division-grouped collapsible or filter chips + search box client-side) + install button per entry (calls install → refresh config list); entries display name at app locale (zh name when zh locale else en name with fallback) + emoji + division color dot; empty-catalog/loading/error states per conventions; delegation-off hint when list empty. Tests: section render from mocked client, install flow patches config, remove flow, division grouping. Gates: build + agents tests. Commit `feat(web): agent editor subagents management tab`.

### Task 9: E2E verification + domain docs

**Files:** evidence doc `docs/migrations/octop-m3-subagents/evidence/<date>-m3-verification.md`; `docs/agents/domain.md` Sub-agents section.

**Must witness live (M1/M2 playbook: fresh port, isolated sqlite, register+login JWT, mock LLM per M1 evidence doc):** catalog 19 divisions + ≥272 entries; install product-manager on an agent → config.subagents updated + tenant row exists; chat turn with a goal designed to trigger delegation (mock model scripted to call subagent_delegate first — the mock/dump mechanism from M1/M2 evidence supports scripted turns; if scripted tool-calling is not feasible with the mock, substitute with: handler-level install/remove E2E + executor integration test running the REAL trpc graph against the fake chat.Chat + delegate tool unit E2E — honestly labeled); transcript events carry `subagent:` prefixed tool calls; unknown slug → visible tool error; budget overrun path via unit evidence. Frontend build + tab test evidence. Gates with pre-existing reds root-caused (M1/M2 lists). Cleanup; no secrets. Commit `docs(subagents): M3 capability documentation and verification evidence`.

---

## Self-Review Notes

- Spec §7 coverage: asset port (T1/T2), install model + Subagents field (T3), delegate tool strict surface + intersection whitelist (T5), independent trpc session execution + budgets + 8 KiB summary + event trace (T4), conditional registration (T5), catalog/install APIs (T6), editor tab (T8), E2E (T9). DAG orchestration explicitly YAGNI'd per spec §7 (interface presence: delegate returns full summary enabling serial relay — recorded).
- Spec deviations recorded: locale fallback gains a zh second step (data reality: en lacks 55 roles); input_refs are plain context strings (no fs authorization); char-based token proxy; tenant row kept on agent-remove (shared resource semantics).
- Execution-time lookups flagged: next migration numbers (T3), WeKnora tool registry canonical names for the mapping table (T5 — definitions.go), fake chat.Chat pattern location (T4), scripted-tool-call feasibility in the mock (T9 fallback defined).
- Type consistency: Catalog/Resolve/ParseRoleTools (T1) consumed in T2/T5/T6; ExecuteRequest/Result (T4) consumed by T5 verbatim; repository interface (T3) consumed by T5/T6.
