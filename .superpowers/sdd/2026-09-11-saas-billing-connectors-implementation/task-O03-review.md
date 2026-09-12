---
status: final
verdict: PASS
reviewer: independent-review-subagent (O03, 按证据收口全部接口与产品联验; did NOT write this code)
commit-under-review: f053e3a2ce26677d3ebf080f4b611d17e5aa86d4 (implementation, 5 files +428/−1) + b1f44cb5b151ea594f282481271ab07ceb0ed89c (ledger), worktree saas-billing-connectors
files-reviewed: full diff f053e3a (scripts/saas/release_gate.py 126ln, release_gate_test.py 155ln, docs/superpowers/plans/saas-billing-connectors-acceptance.md 131ln, specs/2026-09-10-saas-billing-connectors-interface-verification.md +15ln, plans/saas-billing-connectors-progress.md 2 +−); ledger diff b1f44cb; brief task-O03-brief.md; precedent scripts/saas/gate.py (60ln, V03) and task-O02-review.md. Runtime evidence re-verified: artifacts/saas-acceptance/report.json parsed (86 records), all 26 cited SHAs git cat-file -t checked, 12 cited test-command patterns grepped against the ledger, commit-vs-disk drift checked empty.
method: source-level evidence via git show + post-change reads/greps with exact line numbers; BOTH mandated commands re-run by reviewer with true exit codes captured on separate lines; parse_required_ids independently re-executed against the real spec/design files; report.json independently parsed and cross-checked row-by-row against acceptance tables + ledger + git object database.
confirming-command: python3 -m unittest scripts.saas.release_gate_test -v -> Ran 16 tests in 0.004s, OK, echo $? = 0 (16 test names match the ledger claim, incl. brief Step-1 verbatim pair). python3 scripts/saas/release_gate.py --report artifacts/saas-acceptance/report.json -> "gate blocked: missing passing evidence: NO-04" + exactly 40 unfinished rows (AC-01..20 blocked-env, ALI-05/FS-04/NO-04/WX-05 blocked-env reason=real provider evidence required, OM-01..10 blocked-env, ALI-01/03/04 + WX-01/03/04 pending), echo $? = 1.
---

# Task O03 Independent Review — 按证据收口全部接口与产品联验

## AF1 — Brief compliance: file set exact, Step-3 byte-equivalent, Step-1 verbatim, CLI --report — PASS

- **File set exact.** `git show f053e3a --stat`: exactly the brief's Files + Step-6 add list — creates scripts/saas/release_gate.py (126ln), release_gate_test.py (155ln), docs/superpowers/plans/saas-billing-connectors-acceptance.md (131ln); modifies docs/superpowers/specs/2026-09-10-saas-billing-connectors-interface-verification.md (+15) and docs/superpowers/plans/saas-billing-connectors-progress.md (2 +−). 5 files, +428/−1 — matches the claimed stat; no 6th file, no generator script.
- **require_pass byte-equivalent to brief Step-3.** release_gate.py:41-48 reproduces the brief snippet exactly: same signature `require_pass(records: dict, required: set[str]) -> None`, local `provider_cases = {"WX-05", "ALI-05", "FS-04", "NO-04"}`, same two raise conditions in the same order with identical message strings (`missing passing evidence: {key}`, `real provider evidence required: {key}`). Nothing strengthened or weakened.
- **Step-1 tests verbatim.** release_gate_test.py:27-29 is the brief's `test_mock_cannot_prove_provider_acceptance` verbatim (same WX-05 mock-level record, same assertRaises(ValueError)); :31-32 is `test_missing_case_is_failure` verbatim (empty records, required {'AC-01'}). No assertion removed.
- **CLI --report per brief.** :90-91 `--report` is a required Path arg; the acceptance doc's 复核命令 (:130-131) lists exactly `python3 scripts/saas/release_gate.py --report artifacts/saas-acceptance/report.json` from the brief; defaults point at the real spec/design (:37-38).

## AF2 — Gate strictness: every substitute rejected, provider level enforced — PASS

Verified code enforces what tests claim, not just that tests pass:
- **skip rejected**: require_pass release_gate.py:45 (`row.get("status") != "pass"`) — test :34-36 feeds status=skip and asserts ValueError.
- **blocked-env rejected**: same :45 — test :38-43.
- **pass without evidence rejected**: :45 `or not row.get("evidence")` — test :45-47 (evidence key absent).
- **empty evidence list rejected**: `[]` is falsy at :45 — test :49-51.
- **missing key rejected**: :44 `records.get(key, {})` → status None → :45 raises — test :31-32 (brief Step-1).
- **Provider-level enforcement for WX-05/ALI-05/FS-04/NO-04**: :47-48 requires `level == "provider"`. Test :53-56 loops ALL four non-provider levels (unit/integration/browser/mock) across ALL four provider IDs via PROVIDER_CASES — every combination raises; :58-61 pins the error message; :68-69 confirms the positive (provider level passes). The enforcement set in code (:42 local, :15 module constant) is exactly the brief's four IDs.
- **No weakening anywhere**: no status aliasing, no level whitelist for non-provider IDs beyond the brief's own rule (unit/integration accepted for non-provider cases, test :63-66 — same as brief semantics which only constrains provider cases).

## AF3 — Spec parsing: exactly 66 interface IDs + AC-01..20 from the REAL files — PASS

- **Reviewer re-ran the parser independently**: `python3 -c` importing parse_required_ids against the real spec + design → total 86 = 66 interface IDs with group counts {'ALI': 5, 'BUD': 8, 'COM': 7, 'CON': 8, 'FS': 4, 'NO': 4, 'OM': 10, 'OPS': 4, 'SYNC': 5, 'USE': 6, 'WX': 5} + AC-01..AC-20 complete (missing_acs = []). Matches the coordinator-verified distribution 5/8/7/8/4/4/10/4/5/6/5 and the ledger claim exactly.
- **The test does this against real files too**: ParseRequiredIdsTests.test_real_spec_yields_sixty_six_interface_ids_plus_twenty_acs (:73-82) reads SPEC/DESIGN resolved from repo root (:17-19) and asserts 66 + per-group counts + the exact AC set — a genuine regression pin on the actual spec tables, not a fixture tautology. It passed in my 16/16 run.
- **Regex robustness pinned**: :84-97 synthetic test proves prose rows (`G0 环境与契约 | OM-01、OM-02`) do NOT yield IDs and non-AC table keys (`B01 | 旧表`) don't match — the anchored `^| ID |` patterns (:32-35) cannot inflate the required set from narrative text.

## AF4 — Real-report integrity: 11+ pass rows spot-checked, all SHAs exist, commands match ledger — PASS

- **Report structure independently parsed**: artifacts/saas-acceptance/report.json = 86 records — 46 pass / 34 blocked-env / 6 pending; zero pass rows lacking evidence; levels among pass rows: 13 unit + 33 integration (no provider/browser inflation); 20 browser (ACs) + 20 provider (OM-01..10, WX-01/03/04/05, ALI-01/03/04/05, FS-04, NO-04) among non-pass. All counts match the acceptance doc :8 and the ledger row.
- **All 26 cited SHAs exist in git** (`git cat-file -t` = commit for every one): C01 19e64ef, C02 e269415, C03 187570b9, C04 1e7de9a, C05 d22aad3, F01 5a1111e, F02 bb7e8c6, F03 2f4462a, F04 af899ce, F05 195cf6e, U01 5449a1c, U02 0cfa73e, U03 3ef3742, U04 a137c79, U05 0a38092, A01 a798e77, A02 1b5f149, A03 02cc527, A04 21bb6cd, A05 0888fac, A06 1cfc957, A07 0e30173, O01 bf5ab4d, O02 4e99dc9, W02 7adec10, V03 f84b98b.
- **Commands match ledger entries**: all 12 cited directed-command patterns found in the progress ledger (Test(BillingAccess|Account), TestAlipay, Test(Available|Budget) with -race, TestFeishu, TestNotion-implied A06 row, pnpm --filter @weknora/web test, Test(Sync|DataSource), Test(BYOK|Usage), TestWechat, Test(MonthBoundary|ParseCredits|CreditsString), Test(ExpiredLease|BudgetRecovery|BudgetExtend), Test(Fulfillment|ApplyBenefit), Test(Monthly|Lifecycle)); task→primary-SHA columns in the ledger match the citations for F02, C03, A05, W02, A07, U01, C02, F01, F04, U03, U04, U05, O01, O02.
- **Spot-checked 11 pass rows across all segments F/C/U/A/W/O** (report.json + acceptance doc + ledger + git): COM-01 (F02, 4/4 BillingAccess|Account), COM-03 (C01, 12/12 Payment|Order|Outbox), ALI-02 (C03, 14/14 TestAlipay), BUD-01..03 (U02, 10/10 -race), USE-01 (U01, 14/14 BYOK|Usage), CON-01 (A01, 7/7), FS-01 (A05, 9/9 TestFeishu), OPS-02 (O02, 9/9 TestMigration), OPS-04 (W02, 9/9 pnpm web), SYNC-01 (A07, Test(Sync|DataSource)). Result counts in report.json match the ledger rows verbatim. **No fabricated or inflated evidence found.**
- **One nuance (non-blocking)**: U02 rows cite fix commit 0cfa73ec while the ledger's U02 primary column is a89920d（初版）+ 修复 0cfa73ec. Both exist; citing the fix (the final code state whose tests the ledger records as GREEN 10/10 -race) is defensible — see Improvement #1.

## AF5 — Honest outcome: CLI exit 1 verified live, full unfinished listing, no weakened required set — PASS

- **Exit 1 verified by reviewer**: `python3 scripts/saas/release_gate.py --report artifacts/saas-acceptance/report.json; echo $?` → output `gate blocked: missing passing evidence: NO-04` + `CLI_EXIT=1` (no pipe before capture).
- **Unfinished scope fully listed**: my run printed exactly 40 unfinished rows — AC-01..20 (blocked-env), ALI-01/03/04 + WX-01/03/04 (pending), ALI-05/FS-04/NO-04/WX-05 (blocked-env, reason=real provider evidence required — the provider-specific reason from :78-80), OM-01..10 (blocked-env). Identical 40-ID set to acceptance doc :115.
- **No weakened required set**: required = parse_required_ids on the real files (86 IDs), `--without` defaults to [] (:94); nothing silently excluded. The gate refuses the honest report rather than bending.
- **--without exclusion semantics**: excluded IDs stay listed as unfinished with reason=excluded (:72-74, :112-113, :115-119); pinned by tests :101-109 (exclusion + unfinished listing) and :142-151 (main exits 0 WITH exclusion but OM-01 + "excluded" still printed). Exit-0-with-exclusions message explicitly names the excluded scope (:119).

## AF6 — Acceptance doc quality: per-AC + per-66-ID tables, honest statement, complete unfinished list — PASS

- **Per-AC table**: :15-36 — every AC-01..AC-20 row has 层级 (browser), 状态 (blocked-env), and 说明 carrying "full-chain success NOT claimed".
- **Explicit no-full-chain statement**: §明确声明 :5-9 — "不宣称全链路成功", exit 1 declared the correct honest result, report construction documented (86 records mapped row-by-row from these tables), and the pass-semantics note (:9) ties pass to unit/integration levels only with require_pass named as the enforcement point.
- **Per-66-ID table**: :40-107 — each of the 66 IDs has 层级/状态/证据 citing ledger task + full commit SHA + directed command + result counts, or an explicit pending/blocked-env reason.
- **Unfinished list complete**: :109-115 — provider blocked-env (OM-01..10, WX-05, ALI-05, FS-04, NO-04), pending (WX-01/03/04, ALI-01/03/04), browser (AC-01..20, plus OPS-04's un-run browser layer noted separately — correct, since the OPS-04 interface check itself is unit-level). 合计 40 enumerated at :115 and equals my live CLI listing exactly.
- **G0–G5 verdicts**: :119-126 with honest 未达/部分（unit/integration）/按能力区分 verdicts; G3 and G5 explicitly 未达.

## AF7 — Spec modification honesty: +15 hunk purely additive, no check flipped — PASS

- The interface-verification.md diff in f053e3a is ONE hunk: +15 lines inserting §1A (单元／集成证据状态汇总) after the execution-record preamble; **zero deletions**, no other hunks — original check tables (e.g., OM-01 :105, WX-05 :142) untouched.
- §1A only records "当前可主张的最高层级" per group and states 门槛仍以 runtime 联验为准 + "没有任何 pending/blocked-env 被改为 pass"; every provider/browser item stays blocked-env/pending in it (:26-30 grep confirms pass mentions appear only inside the new summary lines).
- Cross-proof: the release gate on the real report still exits 1 with all provider cases unfinished — the summary section authorizes nothing mechanically.

## AF8 — No fabricated artifacts: report untracked, only 5 files committed, no generators — PASS

- `git status --porcelain`: `?? artifacts/` — untracked, exactly as the ledger and acceptance doc :8 disclose; no report JSON or builder script inside either commit (`git show --stat` re-checked: 5 files only; b1f44cb touches only progress.md).
- Reproducibility documented: ledger records "report.json 由验收文档 86 行表格逐行映射构建可复现" — and my independent parse of report.json matches the acceptance tables row-for-row on every spot-checked entry.
- No commit-vs-disk drift: `git diff f053e3a..HEAD` empty on the four non-ledger files; worktree copies of gate/test/acceptance/spec match HEAD.

## AF9 — Ledger honesty: O03 row matches reality, no premature PASS — PASS

- progress.md:41 — status done, commit f053e3a, final column **"独立复核（不写 PASS）"**: no PASS or review-outcome pre-written (O01 lesson applied, consistent with O02's precedent of awaiting this review).
- Every factual claim in the row re-verified true by me: 16/16 tests (my run), ModuleNotFoundError-RED consistent with the module being created in this commit (single-commit RED not independently replayable — same accepted disclosure pattern as prior tasks; coordinator re-verified), 86 records 46/34/6 (my parse), exit 1 (my run), artifacts untracked (my git status), git diff --check clean (coordinator; worktree now clean for these files).
- Two-step ledger write disclosed by history itself: f053e3a staged the row with placeholder 见实现提交, b1f44cb filled the real SHA — honest sequencing, no content change besides the SHA.

## AF10 — Code quality: gate.py precedent style, clear errors, meaningful tests — PASS

- **Style mirrors scripts/saas/gate.py (V03)**: policy module docstring (:1-7 vs gate.py:1-5), `from __future__ import annotations` (:8), argparse with required `--report` Path (:90-91 vs gate.py:48), `main(argv) -> int` returning typed exit codes (:86), `gate blocked: {exc}` prefix (:104, :111 vs gate.py:53), `raise SystemExit(main())` (:126 vs gate.py:60). Adds only stdlib re/typing — no dependencies.
- **Clear errors**: per-ID messages name the exact failure ("missing passing evidence: X", "real provider evidence required: X"); unfinished rows carry id/status/reason so operators can act; blocked file/JSON errors exit 2 distinctly from evidence failure exit 1.
- **Tests meaningful, not tautological**: real-spec parse test pins actual repository documents; main() tests exercise the full CLI path with captured stdout and both exit codes; provider-level test covers all four wrong levels × all four IDs; exclusion tests assert BOTH the removal from required and the retention in listing. 16/16 pass in 0.004s.
- Minor style notes are non-blocking (Improvements #2/#4 below).

## Verdict — PASS

No blocking finding. This closing task does exactly what the plan demanded of it: a gate that structurally refuses every substitute for runtime provider evidence (mock/skip/blocked-env/pending/missing/empty-evidence, and any level below provider for WX-05/ALI-05/FS-04/NO-04), a required set parsed from the real spec documents (66 IDs + AC-01..20, reviewer-verified), a real 86-record report whose every spot-checked pass row traces to an existing commit and a ledger-recorded command, an honest CLI exit 1 with the complete 40-item unfinished listing, an acceptance document that tables all 86 checks with level/status/evidence and explicitly refuses to claim full-chain success, a purely additive spec summary that flips nothing to pass, and a ledger row that claims no verdict before this review. The branch closes with its evidence boundary stated rather than papered over.

### Improvements (non-blocking)

1. **U02 citation cites only the fix commit** (acceptance :47-49, report.json BUD-01..03 → 0cfa73ec; ledger U02 primary is a89920d（初版）+ 0cfa73ec（修复）). Both are real and the fix is the final state, but citing both (初版+修复) — as the ledger does — would make the evidence chain unambiguous for future audits.
2. **Two sources of truth for the provider set**: require_pass's local `provider_cases` (:42, kept for brief byte-equivalence) duplicates module `PROVIDER_CASES` (:15, used by unfinished_scope :78). Derive one from the other or add a comment tying them, so a future edit cannot drift.
3. **Evidence truthiness is loose**: :45 accepts any truthy value (a bare string "x" would pass); validating evidence as a non-empty list of non-empty strings would tighten the contract slightly.
4. **Inconsistent unfinished labels between paths**: excluded rows print as `unfinished: ... reason=excluded` on the failure path (:113) but `unfinished (excluded scope): ...` on the success path (:117); unify the prefix so log parsers treat them identically.
5. **capability_prefixes raises bare KeyError** for an unknown capability (:62) — argparse `choices` (:94-95) guards the CLI, but a programmatic caller gets an unhelpful traceback; a ValueError naming valid capabilities would be kinder.
6. **AC rows are boilerplate-heavy**: :17-36 repeat one identical English sentence per row; a single shared declaration plus short per-AC specifics would read better without losing honesty.
