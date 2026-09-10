#!/usr/bin/env python3
"""Generate deterministic T01 migration inventories from the current checkout.

This is intentionally a reporting generator, not a contract generator. Swagger
rows retain an explicit handler/schema review status because the running Gin
router and handler annotations are the contract authority.
"""

from __future__ import annotations

import csv
import hashlib
import json
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


HTTP_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
SOURCE_STATUSES = {"swagger", "registered-route", "client-used", "handler-reviewed", "special-route"}


def normalize_path(path: str) -> str:
    """Normalize Gin paths to the path form used by swagger.json."""
    path = re.sub(r"/+$", "", path) or "/"
    if path.startswith("/api/v1"):
        path = path[len("/api/v1") :] or "/"
    return path


def canonical_path(path: str) -> str:
    """Use one placeholder spelling when comparing Swagger and Gin paths."""
    return re.sub(r"\{([^}/]+)\}", r":\1", normalize_path(path))


def _join_route(prefix: str, suffix: str) -> str:
    if suffix.startswith("/api/v1"):
        return suffix
    if not suffix:
        return prefix or "/"
    return f"{prefix.rstrip('/')}/{suffix.lstrip('/')}" or "/"


def _router_sources() -> list[Path]:
    return sorted((ROOT / "internal/router").glob("*.go"))


def registered_routes() -> list[dict[str, str]]:
    """Extract route declarations from Gin registration code.

    This is deliberately a conservative source audit, not a Go compiler. It
    follows named RouterGroup assignments and direct/apiKeyRoute calls so the
    generated matrix can distinguish source registration from Swagger-only
    documentation. Handler DTO and permission semantics still require manual
    review and are never inferred here.
    """
    routes: list[dict[str, str]] = []
    for source in _router_sources():
        text = source.read_text(encoding="utf-8")
        # Register* functions receive the already-prefixed v1 group as `r`;
        # helper functions in router.go/files.go receive the root engine.
        prefixes: dict[str, str] = {
            "r": "/api/v1" if source.name.startswith("routes_") else "",
            "v1": "/api/v1",
        }
        # Resolve group assignments iteratively because groups are commonly
        # nested (for example tenantRoutes -> tenantByID).
        assignment = re.compile(
            r"(?m)^\s*(\w+)\s*:=\s*(?:[^\n]*?\b)?(\w+)\.Group\(\s*\"([^\"]*)\""
        )
        for _ in range(8):
            changed = False
            for match in assignment.finditer(text):
                name, parent, suffix = match.groups()
                if parent in prefixes:
                    value = _join_route(prefixes[parent], suffix)
                    if prefixes.get(name) != value:
                        prefixes[name] = value
                        changed = True
            if not changed:
                break

        def add(method: str, group: str, suffix: str, kind: str, line: int) -> None:
            if group not in prefixes:
                return
            path = _join_route(prefixes[group], suffix)
            if not (path.startswith("/api/v1") or path in {"/files", "/r/:token"}):
                return
            normalized = normalize_path(path)
            special = (
                path in {"/files", "/r/:token"}
                or "/embed/" in path
                or "/files" in path
                or "terminal" in path
                or "stream" in path
            )
            routes.append(
                {
                    "method": method,
                    "path": normalized,
                    "registered_path": path,
                    "source": source.relative_to(ROOT).as_posix(),
                    "line": str(line),
                    "source_status": "special-route" if special else "registered-route",
                    "route_kind": "special" if special else "api",
                }
            )

        # Ordinary RouterGroup calls, including the explicit WebSocket route.
        calls = re.compile(
            r"(?m)(\w+)\.(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\(\s*\"([^\"]*)\""
        )
        for match in calls.finditer(text):
            line = text.count("\n", 0, match.start()) + 1
            add(match.group(2), match.group(1), match.group(3), "group", line)

        # apiKeyRoute carries a net/http method constant and is frequently
        # split across lines, hence the DOTALL expression.
        api_key = re.compile(
            r"(?s)g\.apiKeyRoute\(\s*(\w+)\s*,\s*http\.Method(\w+)\s*,\s*\"([^\"]*)\""
        )
        for match in api_key.finditer(text):
            line = text.count("\n", 0, match.start()) + 1
            method = match.group(2).upper()
            if method in HTTP_METHODS:
                add(method, match.group(1), match.group(3), "api-key", line)

    # A declaration can be visible through both a wrapper and a direct call;
    # retaining one sorted row makes the artifact stable and auditable.
    unique: dict[tuple[str, str], dict[str, str]] = {}
    for route in routes:
        unique.setdefault((route["method"], route["path"]), route)
    return [unique[key] for key in sorted(unique)]


def client_used_paths() -> set[str]:
    paths: set[str] = set()
    api_root = ROOT / "frontend/src/api"
    for file in sorted(api_root.rglob("*.ts")):
        text = file.read_text(encoding="utf-8")
        for match in re.finditer(r"['\"](/api/v1/[^'\"]+|/api/v1)['\"]", text):
            paths.add(normalize_path(match.group(1)))
    return paths


def router_rows() -> list[list[str]]:
    rows = [
        ["/", "redirect", "public", "none", "redirect to /platform/knowledge-bases", "T04", "migrate", "frontend/src/router/index.ts", "client-used"],
        ["/login", "page", "public", "none", "login/OIDC/language selection", "T03", "migrate", "frontend/src/views/auth/Login.vue", "client-used"],
        ["/register", "page", "public", "none", "invite registration reuses login", "T03", "migrate", "frontend/src/views/auth/Login.vue", "client-used"],
        ["/onboarding/workspace", "page", "authenticated", "no tenant", "workspace onboarding", "T03/T04", "migrate", "frontend/src/views/auth/WorkspaceOnboarding.vue", "client-used"],
        ["/join", "redirect", "authenticated", "initialization", "organization invite code redirect", "T04/T16", "migrate", "frontend/src/router/index.ts", "client-used"],
        ["/knowledgeBase", "page", "authenticated", "initialization", "legacy knowledge-base URL", "T06", "migrate", "frontend/src/views/knowledge/KnowledgeBase.vue", "client-used"],
        ["/platform", "layout+redirect", "authenticated", "initialization", "platform shell", "T04/T05", "migrate", "frontend/src/views/platform/index.vue", "client-used"],
        ["/platform/settings", "page", "authenticated", "initialization", "settings section selected by query", "T05/T17", "migrate", "frontend/src/views/settings/Settings.vue", "client-used"],
        ["/platform/knowledge-bases", "page", "authenticated", "initialization", "knowledge-base list/search/create", "T06", "migrate", "frontend/src/views/knowledge/KnowledgeBaseList.vue", "client-used"],
        ["/platform/knowledge-bases/:kbId", "page", "authenticated", "initialization", "knowledge-base detail", "T06-T09", "migrate", "frontend/src/views/knowledge/KnowledgeBase.vue", "client-used"],
        ["/platform/knowledge-search", "redirect", "authenticated", "initialization", "legacy command-panel URL", "T04/T06", "migrate", "frontend/src/router/index.ts", "client-used"],
        ["/platform/agents", "page", "authenticated", "agents", "agent list", "T15", "migrate", "frontend/src/views/agent/AgentList.vue", "client-used"],
        ["/platform/integrations", "redirect", "authenticated", "initialization", "legacy settings integration URL", "T05/T18", "migrate", "frontend/src/router/index.ts", "client-used"],
        ["/platform/creatChat", "page", "authenticated", "initialization", "legacy spelling must remain", "T10/T11", "migrate", "frontend/src/views/creatChat/creatChat.vue", "client-used"],
        ["/platform/knowledge-bases/:kbId/creatChat", "page", "authenticated", "initialization", "knowledge-base chat entry", "T10/T11", "migrate", "frontend/src/views/creatChat/creatChat.vue", "client-used"],
        ["/platform/chat/:chatid", "page", "authenticated", "initialization", "chat history/resume", "T10-T14", "migrate", "frontend/src/views/chat/index.vue", "client-used"],
        ["/platform/organizations", "page", "authenticated", "organizations", "organization management", "T16", "migrate", "frontend/src/views/organization/OrganizationList.vue", "client-used"],
        ["/platform/system/*", "redirect", "authenticated", "system_admin", "system settings/queues compatibility URLs", "T16/T17", "migrate", "frontend/src/router/index.ts", "client-used"],
        ["/platform/dev/markdown", "dev-only", "public", "none", "development renderer fixture", "T13", "test-only", "frontend/src/views/dev/MarkdownTestPage.vue", "client-used"],
        ["embed.html", "independent-entry", "embed visitor", "embed", "token exchange/chat/postMessage bridge", "T18", "migrate", "frontend/embed.html; frontend/src/embed-main.ts", "special-route"],
    ]
    settings_dir = ROOT / "frontend/src/views/settings"
    for file in sorted(settings_dir.rglob("*.vue")):
        rel = file.relative_to(ROOT).as_posix()
        section = file.stem
        rows.append([f"settings:{section}", "settings-section", "tenant-scoped", "section access", "settings panel; verify save/error semantics", "T05/T17", "migrate", rel, "client-used"])
    rows.extend([
        ["/files", "special-file-route", "authenticated", "file access", "tenant-scoped storage proxy", "T07/T13", "migrate", "internal/router/files.go", "special-route"],
        ["/r/:token", "special-file-route", "anonymous", "signed resource", "short-lived resource grant", "T07/T13", "migrate", "internal/router/files.go", "special-route"],
        ["/api/v1/files/presigned", "special-file-route", "anonymous", "signed resource", "HMAC presigned GET/HEAD", "T07/T13", "migrate", "internal/router/files.go", "special-route"],
        ["/api/v1/knowledge-bases/:id/files", "special-file-route", "authenticated", "knowledge access", "KB-scoped file proxy", "T07/T13", "migrate", "internal/router/files.go", "special-route"],
        ["/api/v1/sessions/:id/messages/:message_id/files", "special-file-route", "authenticated", "message access", "message-scoped file proxy", "T07/T13", "migrate", "internal/router/files.go", "special-route"],
        ["/api/v1/sessions/:id/sandbox/terminal", "special-websocket", "authenticated ticket", "sandbox", "terminal WebSocket", "T14", "migrate", "internal/router/routes_chat.go", "special-route"],
        ["/api/v1/embed/:channel_id/files", "special-embed-route", "embed visitor", "embed", "publish-token file proxy", "T18", "migrate", "internal/router/routes_agent.go", "special-route"],
    ])
    return rows


def api_rows() -> list[list[str]]:
    rows = []
    registered = {(r["method"], canonical_path(r["path"])): r for r in registered_routes()}
    used_paths = client_used_paths()
    domain_map = {
        "/auth": "identity", "/me": "identity", "/users": "identity", "/tenants": "tenant",
        "/organizations": "organization", "/knowledge": "knowledge", "/knowledge-base": "knowledge",
        "/knowledge-bases": "knowledge", "/knowledge-chat": "chat", "/sessions": "chat", "/messages": "chat",
        "/models": "configuration", "/mcp": "configuration", "/skills": "configuration", "/memory": "configuration",
        "/sandbox": "sandbox", "/storage": "configuration", "/vector": "configuration", "/web-search": "configuration",
        "/system": "system", "/embed": "embed", "/im": "integration", "/files": "file",
    }
    swagger = json.loads((ROOT / "docs/swagger.json").read_text(encoding="utf-8"))
    for current, item in sorted(swagger["paths"].items()):
        for raw_method in sorted(item):
            if raw_method.upper() not in HTTP_METHODS:
                continue
            method = raw_method.upper()
            domain = next((v for k, v in domain_map.items() if current.startswith(k)), "other")
            kind = "mutation" if method in {"POST", "PUT", "PATCH", "DELETE"} else "read"
            if any(token in current for token in ("chat", "stream", "events")):
                kind = "sse-or-event" if method in {"GET", "POST"} else kind
            if any(token in current for token in ("download", "preview", "export", "files")):
                kind = "blob-or-file"
            task = {"identity": "T03", "tenant": "T04/T16", "organization": "T16", "knowledge": "T06-T09", "chat": "T10-T13", "configuration": "T15/T17", "sandbox": "T14", "system": "T16/T17", "embed": "T18", "integration": "T18", "file": "T07/T13"}.get(domain, "T01 follow-up")
            key = (method, canonical_path(current))
            route = registered.get(key)
            source_status = (
                route["source_status"]
                if route
                else ("client-used" if current in used_paths else "swagger")
            )
            comparison = "registered-route" if route else "swagger-only"
            current_source = route["source"] if route else "docs/swagger.json"
            rows.append([
                method,
                current,
                domain,
                kind,
                "swagger-2.0",
                "internal/router + handler DTO/permission/tests",
                source_status,
                comparison,
                task,
                current_source,
            ])

    # Keep implementation-only registrations visible. This is the explicit
    # guard against treating a Swagger inventory as proof of runtime wiring.
    swagger_keys = {(row[0], canonical_path(row[1])) for row in rows}
    for route in registered.values():
        key = (route["method"], canonical_path(route["path"]))
        if key in swagger_keys:
            continue
        path = route["path"]
        domain = next((v for k, v in domain_map.items() if path.startswith(k)), "other")
        kind = "websocket" if "terminal" in path else ("blob-or-file" if "files" in path or path == "/r/:token" else "special")
        task = {"embed": "T18", "file": "T07/T13", "sandbox": "T14"}.get(domain, "T01 follow-up")
        rows.append([
            route["method"],
            path,
            domain,
            kind,
            "not-in-swagger",
            f"{route['source']}:{route['line']} + handler registration",
            route["source_status"],
            "implementation-only",
            task,
            f"{route['source']}:{route['line']}",
        ])
    return sorted(rows, key=lambda row: (row[1], row[0], row[6], row[9]))


def main() -> None:
    OUT.mkdir(parents=True, exist_ok=True)
    write_csv(OUT / "route-parity.csv", ["route_or_entry", "kind", "identity", "capability", "interaction", "target_task", "migration_disposition", "current_source", "source_status"], router_rows())
    write_csv(OUT / "api-contract-matrix.csv", ["method", "path", "domain", "wire_kind", "document_source", "runtime_authority", "source_status", "comparison_status", "target_task", "current_source"], api_rows())
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
