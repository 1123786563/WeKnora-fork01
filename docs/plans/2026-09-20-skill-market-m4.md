# Skill Market (M4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate Octop's skill-market capability: a SkillHub remote client (search/rankings/download with the full zip-validation port) bridged into WeKnora's existing catalog+install pipelines, skillset→expert materialization reusing M2's ExpertService, a tenant-internal market (published skills + published experts, install without content duplication), market APIs, a React skills-market page, and the experts page's two remaining tabs.

**Architecture:** Pure client package `internal/agent/skills/skillhub/` (SSRF-safe HTTP + zip validation + 300s singleflight cache with stale fallback) → service-layer bridges: skill installs go through the existing `RegisterCatalogFromArchive` + `InstallCatalogToConfigs` (no new install path); skillset installs materialize an expert directory into tenant storage + `expert_installs` row and merge into the injected ExpertService catalog → `published_skills`/`published_experts` tables model the tenant-internal market (install = read the source's catalog archive, register under the installer's tenant — content never copied) → routes under `/api/v1/skills/market/*`, `/api/v1/experts/market*`, `/api/v1/market/tenant/*` → React market page + experts tabs.

**Tech Stack:** Go 1.26 / gin / gorm (2 migration pairs) / archive/zip / React 19 (SkillSettingsPanel's SSE progress reused).

**Spec:** `docs/superpowers/specs/2026-09-19-octop-capabilities-migration-design.md` §6. Octop source of truth: `/Users/wuyongjun/trea/Octop/src/octop/infra/skills/skillhub_market.py` (single-skill client + zip port) and `experts/skillhub_market.py` (skillset index/materialization).

## Global Constraints

- Out-of-bound HTTP MUST use `utils.NewSSRFSafeHTTPClient` (internal/utils/security.go:798) with explicit timeout; config block `skillhub_market: {host, timeout_seconds}` following the WebSearchConfig precedent (internal/config/config.go:1545); host default `https://api.skillhub.cn`, overridable (env/config) — E2E depends on the override.
- Zip validation port is EXHAUSTIVE per Octop: ≤2000 entries; ≤64 MiB uncompressed total; compression-ratio ≤100 for entries >1 MiB; reject symlink/unsupported types/encrypted (flag 0x1)/duplicate names/NUL/backslash-normalized traversal/drive-letter/absolute paths; read-size == declared size; UTF-8 coercion; strip single wrapper dir; root SKILL.md required. Response caps: 32 MiB download, 4 MiB JSON.
- Errors: SkillHub unreachable → serve cached results with `stale: true` (cache TTL 300s, singleflight, stale-fallback per Octop) or 503 when no cache; install failures ride the existing install error flow (reaper included). No silent failures.
- NOT migrating (spec §6 exclusions, recorded): server-side curl|bash fallback; multi-source URL import (skills.sh/clawhub/github).
- Skill install path: download → validate → `RegisterCatalogFromArchive` (tenant_skill_catalog.go:169) → `InstallCatalogToConfigs` (:201) — reuse, never a parallel installer. 202 + progress subscription (existing SSE).
- Skillset materialization: fetch skillset zip + each skill zip per Octop's skillhub_market.py; produce a standard expert directory (manifest.yaml + persona md + skills/<slug>/) in tenant-scoped storage; record `expert_installs` row (tenant, slug, storage ref); ExpertService's injected catalog becomes builtin ∪ tenant-installed (merge preserving builtin precedence on id collision).
- Tenant-internal market: `published_skills` (tenant, catalog_id, published_by, timestamps) and `published_experts` (tenant, expert snapshot ref, name/desc at publish, published_by) — installs read the PUBLISHER's catalog archive via the existing bundle chain and register under the INSTALLER's tenant (no content copy); visibility tenant-wide by construction.
- Guards: market reads Viewer+ (apiKeyFullAccess read precedent); installs/publishes Contributor+/Admin per sibling semantics (skills install is Admin+ today — mirror `POST /skills/catalog/:id/install`'s guard); tenant from ctx always.
- E2E must use the host override with a local mock SkillHub server (real client → mock remote); one best-effort live probe of api.skillhub.cn recorded either way (reachability is not a gate).
- JSON snake_case; gofmt/vet; i18n three-point registration; frontend only apps/web + packages/api-client + packages/i18n.

---

### Task 1: skillhub client package — HTTP + cache + zip validation port

**Files:**
- Create: `internal/agent/skills/skillhub/client.go` (search/rankings/download + config struct)
- Create: `internal/agent/skills/skillhub/zip.go` (validation port + wrapper strip)
- Create: `internal/agent/skills/skillhub/cache.go` (TTL 300s singleflight stale-fallback)
- Create: `internal/config` block for skillhub_market (host/timeout) following WebSearchConfig wiring
- Test: `internal/agent/skills/skillhub/client_test.go`, `zip_test.go`

**Interfaces (Producing):**
- `type Client interface { Search(ctx, query string, limit int) ([]SkillSummary, error); Rankings(ctx, kind string) ([]SkillSummary, error); Download(ctx, slug string) ([]byte, error) }` with `SkillSummary{Slug, Name, Description, Version string}` (+raw passthrough map for display fields).
- `type ZipFiles = []struct{ Name string; Content []byte }`; `func ParsePackage(payload []byte) (ZipFiles, error)` — full validation port incl. wrapper strip + root SKILL.md.
- Cache wrapper: `NewCached(inner Client, ttl time.Duration)` — singleflight; stale served with marker; errors typed (`ErrUnreachable`, `ErrStaleOnly`).
- Rankings kinds: hot/featured/newest/recommended/trending/paid (validate enum).
- httptest-based tests incl. adversarial zips (zip-bomb ratio, symlink entry, encrypted flag, traversal names, duplicate, wrapper strip both ways, missing SKILL.md, size caps).

Commit: `feat(skillhub): remote client with cache and exhaustive zip validation port`

### Task 2: skillset index + expert materialization

**Files:**
- Create: `internal/agent/skills/skillhub/skillsets.go` (index client: list/detail, cached)
- Create: `internal/agent/experts/materialize.go` (skillset → expert directory builder: manifest.yaml + SOUL.md from skillset metadata + skills/<slug>/ from per-skill zips, reusing ParsePackage output; writes into a tenant storage dir)
- Create: `internal/types/expert_install.go` + migration pair `expert_installs` (000164 versioned / 000085 sqlite — VERIFY next free)
- Modify: ExpertService catalog injection site (container) — builtin ∪ installed with builtin precedence.
- Test: both packages.

Binding details: skillset DTO fields per Octop experts/skillhub_market.py (name/slug/description/skills[]); materialized manifest maps to M2's ExpertManifest (i18n label/description from skillset metadata, skills list = slugs, agent_config minimal smart-reasoning); `expert_installs`{ID, TenantID, Slug, StorageRef, SnapshotSHA256, CreatedBy, timestamps}; install = download skillset zips → materialize → upsert row → catalog merge.

Commit: `feat(skillhub): skillset index and expert materialization with install ledger`

### Task 3: market service + remote-consumption API

**Files:**
- Create: `internal/application/service/skill_market_service.go` (+ interface types/interfaces/skill_market.go)
- Create: `internal/handler/skill_market.go` + `internal/router/routes_skill_market.go` (+ router/container wiring)
- Test: handler + router coexistence + service (fake client).

Routes (binding):
- `GET /api/v1/skills/market/search?q=&limit=` (Viewer+) → data {results, stale bool}
- `GET /api/v1/skills/market/rankings/:kind` (Viewer+) → data {results, stale}
- `POST /api/v1/skills/market/install` (mirror `POST /skills/catalog/:id/install` guard) body `{slug, sandbox_config_ids: []}` strict → 202 {catalog_id, install_ids}; flow: Download → ParsePackage → repack to install archive format (reuse the zip layout the catalog path expects — verify what RegisterCatalogFromArchive accepts) → RegisterCatalogFromArchive → InstallCatalogToConfigs.
- `GET /api/v1/experts/market` (Viewer+) → data {skillsets, stale} (T2 index)
- `GET /api/v1/experts/market/:slug` (Viewer+) → detail
- `POST /api/v1/experts/market/:slug/install` (Contributor+, expert-instantiate precedent) body `{agent_name?, sandbox_config_id?}` → materialize + M2 Instantiate → 201 {agent, pending_skills, skill_install_ids}.

Commit: `feat(skillhub): market search/rankings/install and skillset-to-expert routes`

### Task 4: tenant-internal market — skills

**Files:**
- Migration pair `published_skills` (000165/000086 — VERIFY)
- `internal/types/published_skill.go` + repo
- Service additions + routes:
  - `POST /api/v1/skills/catalog/:id/publish` (Admin+, owner semantics per catalog) → row {tenant, catalog_id, published_by}
  - `DELETE .../publish` → unpublish
  - `GET /api/v1/market/tenant/skills` (Viewer+) → data {skills:[{catalog_id, name, description, version, publisher_name, installed bool}]}
  - `POST /api/v1/market/tenant/skills/:catalogId/install` (Admin+ mirror) body {sandbox_config_ids} → reads PUBLISHER archive via existing bundle chain → RegisterCatalogFromArchive under installer tenant → InstallCatalogToConfigs → 202. No content duplication.

Commit: `feat(market): tenant-internal skill publishing and cross-member install`

### Task 5: tenant-internal market — experts

**Files:**
- Migration pair `published_experts` (000166/000087 — VERIFY)
- `internal/types/published_expert.go` + repo
- Publish flow (from an existing agent): export per M2's whitelist rules (persona/system prompt + skills references + subagents list + starters — NO KB/model-key/sandbox bindings) into an immutable snapshot ref (tenant storage) + row {tenant, name/desc at publish, snapshot ref, published_by, agent_id}.
- Routes: `POST /api/v1/agents/:id/publish-expert` (OwnedAgentOrAdmin+Admin), `DELETE /api/v1/market/tenant/experts/:id`, `GET /api/v1/market/tenant/experts` (Viewer+), `POST /api/v1/market/tenant/experts/:id/install` (Contributor+) → seeds from snapshot → M2 Instantiate → 201.

Commit: `feat(market): tenant-internal expert publishing and snapshot install`

### Task 6: api-client market module

**Files:** packages/api-client/src/market.ts (+ mounts): types MarketSkillSummary, MarketSkillset, TenantPublishedSkill/Expert, InstallResult reuse; functions market.searchSkills(q, limit?), market.rankings(kind), market.installSkill(slug, sandboxConfigIds), market.skillsets(), market.skillset(slug), market.installSkillset(slug, body?), market.tenantSkills(), market.publishSkill(catalogId)/unpublish, market.tenantExperts(), market.publishAgentAsExpert(agentId), market.installTenantExpert(id, body?). Field parity vs T3-T5 DTOs verified in review. Gate: web build. Commit `feat(api-client): market module`.

### Task 7: React skills market page

**Files:** `apps/web/src/market/MarketPage.tsx` (+ tests); router/routes.tsx whitelist/PlatformShell nav (experts precedent); i18n via script (MARKET_VALUES).
Behavior: search box + rankings tabs (6 kinds) + result cards (name/desc/version) + install flow (sandbox config multi-select or default) wiring the existing SSE install-events progress (reuse SkillSettingsPanel's skill-install SDK + timeline reducer) and 202 handling; stale banner when data.stale; error/empty states. Tests: search render, rankings tab switch, install triggers progress consumption (mock SSE), stale banner. Gates: build + tests. Commit `feat(web): skills market page with install progress`.

### Task 8: experts page — remote + tenant tabs

**Files:** extend `apps/web/src/experts/ExpertsPage.tsx` + api glue + i18n.
Behavior: three tabs — 内置库 (existing), 远程市场 (skillsets grid from T3, detail, instantiate → M2 create-agent flow incl. pending toast), 秩序内发布 (tenant experts list, publish-from-agent entry (agent picker → confirm dialog listing what exports), install → create-agent flow). Tab state in URL query (?tab=). Tests per tab incl. publish dialog call. Gates: build + tests. Commit `feat(web): experts page remote and tenant-market tabs`.

### Task 9: E2E verification + domain docs

**Files:** evidence doc `docs/migrations/octop-m4-market/evidence/<date>-m4-verification.md`; domain.md Market section.
Must witness (M1-M3 playbook; mock SkillHub server via host override serving fixture search/rankings/download/skillsets from disk; one best-effort live probe recorded): search+rankings through real client; skill install 202 → SSE progress → catalog row (real pipeline); skillset install → agent created with expert_source + materialized skills pending; tenant publish → other member sees → install → catalog row (no content duplication evidenced by storage ref equality); expert publish snapshot → install → agent created; negatives (unreachable host → stale/503, invalid zip → 400, guards 401/403); frontend build + page tests. Gates with pre-existing reds root-caused. Cleanup; no secrets (LAN IP redaction per M3 lesson — write mock host as `<mock-host>`). Commit `docs(market): M4 capability documentation and verification evidence`.

---

## Self-Review Notes

- Spec §6 coverage: remote consumption (T1/T3), zip port (T1), bridge to existing install (T3 constraint), skillset→expert (T2/T3), tenant market skills (T4) + experts (T5), stale/503 semantics (T1/T3), exclusions recorded. M2's deferred market scope (expert_installs/published_experts tables) lands here per the milestone table.
- Deviations recorded: publish visibility is tenant-wide (no per-member ACL — spec silent, simplest correct); live-probe not a gate (host may be unreachable).
- Execution-time lookups flagged: RegisterCatalogFromArchive's accepted archive layout (T3 — may need repack), migration numbers (T2/T4/T5), skillset DTO fields (T2 vs Octop source), publish export whitelist exact fields (T5 — mirror M2 §5 rules).
- Type consistency: Client/SkillSummary/ZipFiles (T1) consumed T2/T3; materialized ExpertManifest (T2) flows into M2 Instantiate unchanged; market DTOs (T3-T5) ↔ api-client (T6) ↔ pages (T7/T8).
