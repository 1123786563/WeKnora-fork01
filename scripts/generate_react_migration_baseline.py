#!/usr/bin/env python3
"""Generate deterministic T01 migration inventories from the current checkout.

This is intentionally a reporting generator, not a contract generator. Swagger
rows retain an explicit handler/schema review status because the running Gin
router and handler annotations are the contract authority.
"""

from __future__ import annotations

import csv
import hashlib
import re
import subprocess
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "docs/migrations/react"


def run(*args: str, cwd: Path = ROOT) -> str:
    return subprocess.check_output(args, cwd=cwd, text=True).strip()


def sha(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_csv(path: Path, header: list[str], rows: list[list[str]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", newline="", encoding="utf-8") as fh:
        writer = csv.writer(fh)
        writer.writerow(header)
        writer.writerows(rows)


def router_rows() -> list[list[str]]:
    rows = [
        ["/", "redirect", "public", "none", "redirect to /platform/knowledge-bases", "T04", "migrate", "frontend/src/router/index.ts"],
        ["/login", "page", "public", "none", "login/OIDC/language selection", "T03", "migrate", "frontend/src/views/auth/Login.vue"],
        ["/register", "page", "public", "none", "invite registration reuses login", "T03", "migrate", "frontend/src/views/auth/Login.vue"],
        ["/onboarding/workspace", "page", "authenticated", "no tenant", "workspace onboarding", "T03/T04", "migrate", "frontend/src/views/auth/WorkspaceOnboarding.vue"],
        ["/join", "redirect", "authenticated", "initialization", "organization invite code redirect", "T04/T16", "migrate", "frontend/src/router/index.ts"],
        ["/knowledgeBase", "page", "authenticated", "initialization", "legacy knowledge-base URL", "T06", "migrate", "frontend/src/views/knowledge/KnowledgeBase.vue"],
        ["/platform", "layout+redirect", "authenticated", "initialization", "platform shell", "T04/T05", "migrate", "frontend/src/views/platform/index.vue"],
        ["/platform/settings", "page", "authenticated", "initialization", "settings section selected by query", "T05/T17", "migrate", "frontend/src/views/settings/Settings.vue"],
        ["/platform/knowledge-bases", "page", "authenticated", "initialization", "knowledge-base list/search/create", "T06", "migrate", "frontend/src/views/knowledge/KnowledgeBaseList.vue"],
        ["/platform/knowledge-bases/:kbId", "page", "authenticated", "initialization", "knowledge-base detail", "T06-T09", "migrate", "frontend/src/views/knowledge/KnowledgeBase.vue"],
        ["/platform/knowledge-search", "redirect", "authenticated", "initialization", "legacy command-panel URL", "T04/T06", "migrate", "frontend/src/router/index.ts"],
        ["/platform/agents", "page", "authenticated", "agents", "agent list", "T15", "migrate", "frontend/src/views/agent/AgentList.vue"],
        ["/platform/integrations", "redirect", "authenticated", "initialization", "legacy settings integration URL", "T05/T18", "migrate", "frontend/src/router/index.ts"],
        ["/platform/creatChat", "page", "authenticated", "initialization", "legacy spelling must remain", "T10/T11", "migrate", "frontend/src/views/creatChat/creatChat.vue"],
        ["/platform/knowledge-bases/:kbId/creatChat", "page", "authenticated", "initialization", "knowledge-base chat entry", "T10/T11", "migrate", "frontend/src/views/creatChat/creatChat.vue"],
        ["/platform/chat/:chatid", "page", "authenticated", "initialization", "chat history/resume", "T10-T14", "migrate", "frontend/src/views/chat/index.vue"],
        ["/platform/organizations", "page", "authenticated", "organizations", "organization management", "T16", "migrate", "frontend/src/views/organization/OrganizationList.vue"],
        ["/platform/system/*", "redirect", "authenticated", "system_admin", "system settings/queues compatibility URLs", "T16/T17", "migrate", "frontend/src/router/index.ts"],
        ["/platform/dev/markdown", "dev-only", "public", "none", "development renderer fixture", "T13", "test-only", "frontend/src/views/dev/MarkdownTestPage.vue"],
        ["embed.html", "independent-entry", "embed visitor", "embed", "token exchange/chat/postMessage bridge", "T18", "migrate", "frontend/embed.html; frontend/src/embed-main.ts"],
    ]
    settings_dir = ROOT / "frontend/src/views/settings"
    for file in sorted(settings_dir.rglob("*.vue")):
        rel = file.relative_to(ROOT).as_posix()
        section = file.stem
        rows.append([f"settings:{section}", "settings-section", "tenant-scoped", "section access", "settings panel; verify save/error semantics", "T05/T17", "migrate", rel])
    return rows


def api_rows() -> list[list[str]]:
    path_re = re.compile(r"^  (/[^:]+):\s*$")
    method_re = re.compile(r"^    (get|post|put|patch|delete|head|options):\s*$", re.I)
    current = None
    rows = []
    domain_map = {
        "/auth": "identity", "/me": "identity", "/users": "identity", "/tenants": "tenant",
        "/organizations": "organization", "/knowledge": "knowledge", "/knowledge-base": "knowledge",
        "/knowledge-bases": "knowledge", "/knowledge-chat": "chat", "/sessions": "chat", "/messages": "chat",
        "/models": "configuration", "/mcp": "configuration", "/skills": "configuration", "/memory": "configuration",
        "/sandbox": "sandbox", "/storage": "configuration", "/vector": "configuration", "/web-search": "configuration",
        "/system": "system", "/embed": "embed", "/im": "integration", "/files": "file",
    }
    swagger = ROOT / "docs/swagger.yaml"
    for line in swagger.read_text(encoding="utf-8").splitlines():
        path_match = path_re.match(line)
        if path_match:
            current = path_match.group(1)
            continue
        method_match = method_re.match(line)
        if not method_match or current is None:
            continue
        method = method_match.group(1).upper()
        domain = next((v for k, v in domain_map.items() if current.startswith(k)), "other")
        kind = "mutation" if method in {"POST", "PUT", "PATCH", "DELETE"} else "read"
        if any(token in current for token in ("chat", "stream", "events")):
            kind = "sse-or-event" if method in {"GET", "POST"} else kind
        if any(token in current for token in ("download", "preview", "export", "files")):
            kind = "blob-or-file"
        task = {"identity": "T03", "tenant": "T04/T16", "organization": "T16", "knowledge": "T06-T09", "chat": "T10-T13", "configuration": "T15/T17", "sandbox": "T14", "system": "T16/T17", "embed": "T18", "integration": "T18", "file": "T07/T13"}.get(domain, "T01 follow-up")
        rows.append([method, current, domain, kind, "swagger-2.0", "internal/router/routes_*.go + handler annotation", "requires-handler-dto-permission-review", task])
    return rows


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    write_csv(OUT / "route-parity.csv", ["route_or_entry", "kind", "identity", "capability", "interaction", "target_task", "migration_disposition", "current_source"], router_rows())
    write_csv(OUT / "api-contract-matrix.csv", ["method", "path", "domain", "wire_kind", "document_source", "runtime_authority", "comparison_status", "target_task"], api_rows())
    write_csv(OUT / "reuse-manifest.csv", ["source_kind", "source", "source_sha", "license", "reuse_mode", "target_boundary", "decision", "reason"], [
        ["weknora-owned", "frontend/src/api + request.ts", run("git", "rev-parse", "HEAD"), "MIT plus third-party notices", "behavioral extraction", "packages/api-client", "approved", "preserve WeKnora API/auth semantics; rewrite without Vue runtime"],
        ["weknora-owned", "frontend/src/router + stores", run("git", "rev-parse", "HEAD"), "MIT plus third-party notices", "behavioral extraction", "packages/domain + apps/* adapters", "approved", "preserve URL/scope/capability behavior"],
        ["weknora-owned", "cmd/desktop", run("git", "rev-parse", "HEAD"), "MIT plus Wails notices", "direct runtime preservation", "apps/desktop adapter", "approved", "retain Wails/Go/Lite lifecycle"],
        ["reference-pattern", "/Users/wuyongjun/trea/multica/packages/core", run("git", "-C", "/Users/wuyongjun/trea/multica", "rev-parse", "HEAD"), "Multica License + additional conditions", "clean-room architecture only", "packages/contracts/domain", "no source copy", "business/API/UI semantics are not equivalent and license is not pure Apache-2.0"],
        ["reference-pattern", "/Users/wuyongjun/trea/multica/packages/ui + packages/views", run("git", "-C", "/Users/wuyongjun/trea/multica", "rev-parse", "HEAD"), "Multica License + additional conditions", "clean-room architecture only", "packages/ui/views", "no source copy", "do not copy UI, brand, text, CSS or product-domain code"],
        ["reference-pattern", "/Users/wuyongjun/trea/multica/apps/mobile", run("git", "-C", "/Users/wuyongjun/trea/multica", "rev-parse", "HEAD"), "Multica License + additional conditions", "version/reference only", "apps/mobile", "verify independently", "Expo/RN versions are an input, not a source implementation"],
    ])
    version = f"""# React 多端迁移版本与兼容矩阵\n\n## Frozen repository inputs\n\n- WeKnora: `{run('git', 'rev-parse', 'HEAD')}`\n- Multica (read-only): `{run('git', '-C', '/Users/wuyongjun/trea/multica', 'rev-parse', 'HEAD')}`\n- WeKnora Go module: `{(ROOT / 'go.mod').read_text(encoding='utf-8').splitlines()[0]}`; Wails requirement is `v2.12.0` (`go.mod`), while `cmd/desktop/wails.json` uses schema v2.\n- Existing frontend: Vue 3.5, Vite 7, TypeScript 6, Pinia 3, npm lockfile; no React workspace exists yet.\n- Existing API description: Swagger/OpenAPI 2.0, 282 paths and 361 operations.\n\n## Reference-only mobile input\n\n- Multica manifest: Expo `~55.0.23`, React `19.2.0`, React Native `0.83.6`.\n- `apps/mobile/CLAUDE.md` is stale (describes RN 0.82/React 19.1); manifest/lockfile wins.\n- WeKnora must independently lock an Expo-compatible set during T20; no dependency is installed or accepted by this T01 artifact.\n\n## Swagger 2.0 generation trial\n\n- Candidate input: `docs/swagger.yaml`; authority remains `internal/router/routes_*.go`, handler DTOs, middleware and tests.\n- Required trial: pin OpenAPI Generator `typescript-fetch` and record Java/runtime versions before T02.\n- Current result: not executed in T01 because no generator/toolchain was present in the checkout; therefore generated-client compatibility is **unverified**, not passed.\n- Required fixtures: knowledge-base list/create and login, including `null`, omitted fields, unknown enum, `uint64` string precision, 204, 413, non-JSON error and mutation failure.\n\n## Compatibility decisions\n\n| Area | Frozen boundary | Evidence status | Follow-up |\n|---|---|---|---|\n| Web | React + TypeScript + Vite SPA | decision approved; implementation absent | T02 |\n| Desktop | Existing Wails + Go/Lite lifecycle; React renderer later | current Wails path verified | T06/T19 |\n| Mobile | Expo + React Native, native UI | no host exists | T20 |\n| Backend | Gin REST + SSE + terminal WS unchanged | route registration exists; behavior matrix pending | T01/T02 |\n| Embed | separate entry and credential profile | Vue entry exists; React entry absent | T18 |\n| Vue retirement | only after T24 acceptance | not eligible | T25 |\n"""
    (OUT / "version-matrix.md").write_text(version, encoding="utf-8")
    baseline = f"""# T01 运行基线\n\n- Captured at: 2026-09-10\n- WeKnora HEAD: `{run('git', 'rev-parse', 'HEAD')}`\n- Multica HEAD (read-only): `{run('git', '-C', '/Users/wuyongjun/trea/multica', 'rev-parse', 'HEAD')}`\n- Vue SFC count: `{len(list((ROOT / 'frontend/src').rglob('*.vue')))}` (plan/inventory said 199; current is 200)\n- API TS count under `frontend/src/api`: `{len(list((ROOT / 'frontend/src/api').rglob('*.ts')))}`\n- Locales: `zh-CN`, `en-US`, `ja-JP`, `ko-KR`, `ru-RU`, plus embed locale resources\n- Legacy URL sources: `frontend/src/router/index.ts`; route matrix records the retained entries\n- Desktop data/runtime sources: `cmd/desktop/main.go`, `cmd/desktop/prefs.go`, `cmd/desktop/update.go`; Wails frontend is currently `../../frontend`\n- API description: Swagger 2.0, 282 paths/361 operations\n\n## Evidence status\n\n| Layer | Result | Evidence |\n|---|---|---|\n| Static inventory | collected | route/API/reuse/version matrices in this directory |\n| Mock transport | not applicable to T01 | T02 |\n| Existing frontend build/test | not run in T01 | dependency installation and baseline command are still pending |\n| Real backend smoke | not run | requires a safe isolated backend and credentials; no production data used |\n| Wails installed package | not run | T19; browser/WebView preview is not package evidence |\n| iOS/Android native | not run | T20-T23; no Expo host exists yet |\n| Core screenshots/performance | not collected | requires running frontend and a stable fixture/backend |\n\n## Known baseline blockers\n\n1. `frontend/node_modules` and root React workspace are absent; no build claim is made.\n2. The three authoritative input documents are currently untracked user files; this ledger must not overwrite or clean them.\n3. Swagger is a 2.0 document inventory, not proof of handler behavior; every generated row remains explicitly marked for handler/DTO/permission review.\n4. Real smoke and native/Wails package evidence require environment inputs not present in this T01 run.\n"""
    (OUT / "runtime-baseline.md").write_text(baseline, encoding="utf-8")


if __name__ == "__main__":
    main()
