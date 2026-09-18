# 2026-09-17 Round R450 — Preview fix live-verified, lite-mode gating, credential Replace/Remove (TDD, 4 parallel agents)

Round type: TDD round with 4 parallel agents — A1 isolated-context live verification closing the R447/R448
preview-fix loop, A2 platform settings residuals, A3 datasource credential-step closeout, A4 verifier.
Verdict: **all PASS, zero rework**; final gates: test:web 1615/1615 (+13), test:shared 604/604 (+1),
typecheck clean, build ✓.

## A1 — Live verification, loop closed (PASS)

- **Chunking preview fix FINAL VERIFICATION PASSED**: real click through settings → 分块设置 → 测试分块效果 →
  载入示例 → 运行预览, twice — HTTP 200, envelope unwrapped (`selected_tier=legacy`, `rejected:null`, 6 chunks,
  2177 chars), NO failure toast, stats row (98 行/2177 字符/13 标题/语言 en,de,zh) and 6 chunk cards render.
  The R447 envelope unwrap + R448 rejected:null normalization together resolve the live chain end-to-end.
- Platform menu regression: 空间设置 (R449 rename), 帮助与文档 (R447 translation) correct; 系统管理 correctly
  hidden for the parity account (`is_system_admin=false` — the gated landing could not be observed).
- Shared-KB banner regression: no 「查看权限」banner in the shared fixture KB (R448 removal verified live).
- join-requests naming: 「加入申请」tab + inner 「待审核申请」+ badge + empty state all render.
- A4 adjudication: 「全部设置」landing at `?section=general` is NOT a defect — the menu href matches Vue (no
  query); the query comes from SettingsPage's existing URL normalization (a750ea30), and Vue's Settings page
  also defaults to general and syncs `?section=`. Task-spec wording corrected.
- Environment note: a transient `isLiteEdition` TDZ white-screen on /platform/organizations was an HMR
  mid-edit state (A2's concurrent edit), gone after reload, no TDZ in the final code.

## A2 — Platform settings residuals (PASS)

- tenant section title: VERIFIED CONSISTENT with Vue (TenantInfo.vue `tenant.title` = 空间信息; nav
  `settings.tenantInfo` = 空间信息) — R449's record stands, closed as a no-op with the 5-locale values checked.
- isLiteMode gating: source is equivalent (`localStorage weknora_lite_mode`, not a Vue-specific build system);
  wired at the six Vue-cited points (space identity line + switch panel, 空间设置 shortcut, divider+logout,
  member/model/skill entries, sidebar organizations via liteHiddenPaths). 5 behavioral tests red→green;
  platform 195 pass; zero new i18n keys. R449 erratum: Vue's UserMenu has no "share item" — the gated entry is
  the 空间设置 shortcut.
Deferred: `getSystemInfo().edition==='lite'` probe has no React equivalent yet (first session before the key
is written stays out of lite gating — low risk, recorded).

## A3 — Datasource credential step closeout (PASS)

- Replace/Remove per Vue's three-state contract: `configured` (已配置 ✓ + 更换/移除) → `unconfigured` (未配置 +
  配置) → `inputs` (Replace mode editable + Cancel discards); Remove uses Vue's inline confirm (danger
  取消/确认移除), success returns to unconfigured with removedToast; `credentialsRequiredForValidation`'s
  replacementTyped now binds the explicit replace mode (Vue needsConnectionTest semantics — R447's exemption
  tests unchanged and green).
- prereq hints/placeholders aligned to Vue connectorDefs (baseUrl hints, rss feedUrls hint, lark placeholder
  corrected to https://open.feishu.cn, gitlab专用 keys); unmigrated resource-step UI (rss custom headers,
  Drive folder_token, GitLab projects) recorded.
- 10 tests red (state machine + 5 page contracts) → green; data-sources 36/36; i18n 66/66 with the guard
  synced 169→180 (+11 `dataSource.credential.*` keys ×5 locale, byte-exact from Vue).
Deferred: `removeCredentials` DELETE call goes through feature-detect (api-client frozen this round — land the
method and the call activates).

## Gates (final)

`pnpm test:web` 1615/1615 (+13 = A2 5 + A3 8), `pnpm test:shared` 604/604 (+1 byte assertion), typecheck clean,
build ✓ (A4 final run on the merged state; zero new failures). No Vue, mobile, or Go code modified. Per-agent
reports: .omc/state/r450/report-A{1,2,3}.md + report-A4-review.md (session artifacts).
