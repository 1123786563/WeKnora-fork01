---
status: final
verdict: PASS
reviewer: independent-reviewer (A06, connectors segment; did NOT write this code)
commit-under-review: 1cfc957 (implementation) + e81d0da (ledger row), branch codex/saas-billing-connectors
files-reviewed: internal/appconnector/notion_create.go (609), notion_create_test.go (607), adapter.go, action.go, http_policy.go (A03/A04 contracts), task-A06-brief.md, plan A06 section, sibling feishu_send.go, ledger row
method: source-level evidence + independently re-run command suite (test/vet/build/gofmt/diff-check, -run TestNotion -v, -race) + two empirical re-derivations of the coordinator's disclosed repairs (scratch edits reverted, working tree restored to HEAD)
---

# Task A06 Independent Review — Notion page creation + partial-completion recovery

## W1 — Brief compliance: files, interfaces, NextNotionStep semantics, routing guards — PASS
- `git show --stat 1cfc957`: exactly the two brief-named files — `notion_create.go` (+609), `notion_create_test.go` (+607); no other file touched. Commit message is the brief Step 6 string verbatim.
- `NextNotionStep` (notion_create.go:74-82) is exactly the brief Step 3 algorithm, with the literals `"create"`/`"append_remaining"`/`"complete"` expressed as named constants of identical values (lines 64-68): empty pageID → create; !contentDone → append_remaining; else complete. Step constants pinned by value in TestNotionNO01ContractPinned + TestNotionStepRoutingAndUnknownNeverRouted (""/false and ""/true both → "create").
- unknown_create never routed: `CanRouteNotionStep` (91-98) allows only `ActionDispatched/ActionSucceeded/ActionFailed` (confirmed, post-dispatch states); `ActionUnknown` falls to default → false. Tested at test:41-43 and end-to-end at test:563-565. `NextNotionStep` itself is never called with an unconfirmed id anywhere in Execute/Query: the only call sites outside tests are… none — production routing goes through the persisted-progress gate, and the tests call it only with persisted ids. A failed append routes (failed + persisted page id = the NO-03 resume case); a pre-network failure has no page id → "create", correct since nothing was created.
- `NotionCreateAdapter` implements A04 `Adapter` — compile-time `var _ Adapter = (*NotionCreateAdapter)(nil)` (272) with exact `Execute(ctx, Action) (ActionResult, error)` (493) and `Query` (570) signatures; consumes A03 `Action`/lifecycle states and A04 `HTTPPolicy`/ActionResult per the brief's Consumes list.

## W2 — Step 1 verbatim test present and unmodified — PASS
- test:21-28 `TestNotionPartialSuccessDoesNotCreateAnotherPage`: same name, same two calls `NextNotionStep("page1", false)`/`("page1", true)`, same expectations, same fatal messages "would duplicate page" / "completion lost" — character-identical to brief Step 1 modulo gofmt spacing (the same standard accepted in the A05 review V1). Passes (targeted run).

## W3 — NO-01 pinned contract; nothing derived from model output — PASS
- Reviewed constants (notion_create.go:50-61): host `api.notion.com`, `Notion-Version 2022-06-28`, create path `/v1/pages`, page read format, append format `/v1/blocks/%s/children`, search path, capability `insert_content`, append batch cap 100. All pinned by TestNotionNO01ContractPinned (test:49-68).
- Wire verification, not just constants: Notion-Version header and Bearer token captured on the wire and asserted byte-exact (test:150, 418-423); create parent is `{"page_id":…}` (parent TYPE = page) and is not rewritten (424-426); title not rewritten (427-429); append children byte-equal to the normalized approved blocks (430-451). A parent of type database_id would leave `capturedParent` empty and fail the test — parent type is effectively pinned.
- Create body (168-191) carries ONLY parent + title property, official nested shape `properties.title.title[].text.content` — blocks travel exclusively via append-children so each batch is a separately recoverable step (a deliberate NO-03 structure). File imports are stdlib-only (3-13); zero model/LLM/prompt references (grep: only a comment saying "nothing here is derived from model output").

## W4 — NO-02 authorization rejections with ZERO network writes — PASS
- Code order in Execute (493-511): config check → A03 Recheck → snapshot validation → parent-scope check → capability check — ALL before the first `do()`. Every rejection returns before any request object exists.
- Tests count wire requests for real: the fake increments `f.wire` in every handler (pages, blocks, search), and `stats()` returns it. Out-of-scope parent → `ErrNotionParentOutOfScope`, wire==0, plus empty-scope fail-closed (test:328-350); revoked approval (Recheck) → `ErrNotionApprovalRevoked`, state `ActionAwaitingApproval` (parked), wire==0 (352-368); missing insert capability → `ErrNotionMissingCapability`, wire==0 (370-386).
- Snapshot rewriting: extra field, empty parent, empty title, non-JSON block rejected by `ParseNotionCreateSnapshot` (exactly 3 keys, 126-162) — pinned directly at test:454-461 (extra field, empty parent) and indirectly by the A05-style field-by-field test. These parse rejections are direct-call tests, so no wire counting applies by construction; the Execute-path zero-wire property for them follows from the verified ordering and is the same pre-network position proven by the three wire-counted tests. Content change invalidating old authorization is A03 digest territory (already covered by A03's suite).

## W5 — NO-03 recovery discipline — PASS
- Page id persisted immediately after create BEFORE content steps: notion_create.go:527-535 — `storeProgress(PageID, BlocksDone:0)` runs before the append loop; test:473-476 asserts the id is in the store even when the very first append fails.
- BlocksDone persisted after each successful batch, and only after: 542-557 (store runs after `done = end`; on append error the store still holds the last successful range). Test:509-512 asserts BlocksDone==2 after batch-1-of-2 succeeds and batch-2-of-1 fails.
- Append failure never re-creates: once pageID is persisted, the create branch (`if pageID == ""`, 516) is structurally unreachable; failures return `ExternalID: pageID`. Crash-resume test (464-497): fresh adapter + same store → completes the SAME page, creates==1, appended==3. Partial-append test (499-546): resume sends ONLY the remaining block (last append has exactly 1 child == snapshot block 3), creates==1, appended==3.
- Unknown create → `ActionUnknown` (519-525) with NO page id persisted (return happens before any store) — test:560-562 asserts the store stays empty (no fabricated id). Query claims a page id ONLY via reliable `GET /v1/pages/{persisted id}` + parent re-check + children read + field-by-field block comparison (585-607); with no persisted id, Query runs the advisory title search and ALWAYS returns ActionUnknown with empty ExternalID (579-584) — the search response body is discarded at the transport call itself (`_, _, err = m.do(...)`, 484), so a title match is structurally incapable of becoming proof. Tested against a same-title/same-parent decoy (test:548-580): q.State==unknown, q.ExternalID=="", creates==1, searches==1.
- Partial display: persisted PageID + BlocksDone + NextNotionStep names the remaining step — the NO-03 "show created page and remaining steps" contract at the adapter level.

## W6 — A04 policy on every request; no raw client — PASS
- `do()` (349-380): `m.Policy.ValidateRequest(method, u)` FIRST (351) — before the token fetch, before the request is built, before any dial; then `m.Policy.NewClient().Do(req)` (370), the A04 validating client (per-hop re-validation, redirect checks, DNS-validated dial). `grep 'http.Client|http.Transport'` over notion_create.go: only a doc comment — no raw client constructed. All five endpoints (create/append/get page/get children/search) go through this single `do()`.
- Policy denial classification is correct and sharp: ValidateRequest errors are returned raw (not wrapped unknown) → Execute classifies them ActionFailed — a denial definitively did not leave the machine, so failed (not unknown) is honest; transport/read failures ARE wrapped ErrNotionOutcomeUnknown (372, 377) → ActionUnknown. Zero-wire tests (W4) prove denials fire before the wire.

## W7 — Error semantics — PASS
- 5xx create → `ErrNotionOutcomeUnknown` → ActionUnknown (397-402): the page may exist — the ambiguous case. 4xx create → plain `notion_provider_error` → ActionFailed (403): a definitive provider no. Reply without a REAL page id → unknown (409-411).
- Provider definitive append errors (any 4xx/5xx answered) → failed (429-432) with progress retained → resume from BlocksDone, exactly the brief's "append failure after created page must not rebuild".
- Progress-save failure → `ErrNotionOutcomeUnknown` → ActionUnknown, never failed (318-329, 533-535, 555-557), with the justification written in-code: a fabricated failure is what triggers duplicate creations; losing the resume record makes the outcome genuinely ambiguous. Justified. (Coverage gap noted non-blocking.)
- Query unverifiable anything (non-200, unparseable, parent mismatch, missing/short/divergent children) → ActionUnknown, never fabricated success (442-451, 585-606); tested for short children (test:597-606).

## W8 — Honesty: NO-04 not claimed, stub labeled, blocked-env explicit — PASS
- Test file labels the double explicitly: "fakeNotion is a LOCAL contract double of the NO-01 endpoints. It is NOT NO-04: no real Notion acceptance is claimed through it" (test:70-71).
- Ledger row (e81d0da): 12/12 GREEN matches my independent count of 12 `func TestNotion`; RED recorded as build failure with the test file only (valid RED per the plan's own rule and A05 precedent); gates recorded as run; NO-04 declared blocked-env — "无用户指定测试父页面与集成凭据，未伪造真实写入，不标 pass". Nothing unrun is marked pass; the assembly history (two timed-out subagents) and the three coordinator repairs are disclosed IN the ledger itself. NO-01–NO-03 covered by stub evidence, honestly labeled.
- Scope hygiene: zero Notion references leaked into the Feishu files (grep), no Feishu references in the Notion files.

## W9 — The 3 coordinator repairs: legitimate completion, not weakening — PASS (both re-derivations run empirically)
- (a) Output evidence: Execute keeps the CREATE response as `Output` (output captured at 529, append replies discarded via `_ = raw` at 554, returned at 559). The implementer's own test already asserted `out.Output` contains the real page id (test:401-403) — only the create reply carries it, so without the fix the assertion fails for any implementation. Implementation-side repair satisfying an existing honest assertion; no assertion touched. Same evidence-preservation principle as A05. Legitimate.
- (b) Title-parse nesting: re-derived with a standalone program — the official body `{"properties":{"title":{"title":[{"text":{"content":"Q3 Board Review"}}]}}}` into a flat `Properties.Title string` yields `Title=""` + unmarshal error, so the fixture could never read the title of ANY implementation sending the official shape; satisfying a flat parser would require the non-official body `{"properties":{"title":"Q3 Board Review"}}`, violating NO-01. The fixture's own search-decoy JSON (test:231) already used the official nested shape. The repair added one nesting level to the fixture's parse struct only — implementation unchanged, assertion (`capturedTitle == "Q3 Board Review"`) unchanged. Legitimate instrument repair.
- (c) appendFailOnNth:2: re-derived by temporarily restoring `appendFailNext: 1` in the partial-append test and running it — FAIL at test:511 "completed block range must be persisted after each step, got {PageID:real_page_1 BlocksDone:0}". With MaxBatch=2 the FIRST append IS the batch of 2; failing it leaves BlocksDone=0, so the test's own stated expectation (batch of 2 succeeds, batch of 1 fails, BlocksDone==2) is unsatisfiable for ANY implementation — the only way to "pass" would be treating a provider 500 as success, which the crash-resume test (same primitive appendFailNext:1, expecting ActionFailed + resume) then forbids. Intrinsic contradiction. The new fixture field fails exactly the Nth request (test:202-209); the partial test's assertions are all unchanged in strength (BlocksDone==2, creates==1, appended==3, resume sends exactly 1 block == snapshot block 3). Working tree restored to HEAD after the experiment. Legitimate completion — arguably the only correct fix.

## W10 — Code quality, race safety, package health — PASS
- Race safety: fake server guards all state with a mutex (stats/pageChildren/handlers lock correctly, writeErr called after unlock); progress store mutex-guarded; adapter holds only injection fields and mutates nothing per-call. `go test -run TestNotion -race` → ok.
- Resources: `defer resp.Body.Close()`, bodies read via `io.LimitReader(…, 1<<20)` (truncation fails toward unknown/unverifiable — the safe direction). Empty-blocks edge (title-only page) returns Succeeded with progress {pageID, 0}. Consistent with sibling feishu_send.go (same configError/do/targetURL/Recheck shape — no divergent idiom introduced).
- Nothing breaks the package: full `go test ./internal/appconnector ./internal/agent/approval -count=1` ok/ok, vet clean, build clean, gofmt empty, `git diff --check` clean.

## Non-blocking notes
1. Commit shape deviates from brief Step 6: implementation (1cfc957) and ledger row (e81d0da) are two commits instead of one staged unit — same deviation-then-record pattern the coordinator disclosed; ledger content is complete and honest. Process note only.
2. A RESUMED Execute (page id already persisted) returns `Output: nil` — the create-response evidence is only attached on the execution that performed the create. ExternalID + progress + Query still identify the page; a follow-up could re-read the page for the payload. Cosmetic.
3. Query compares blocks as exact normalized bytes; real Notion returns enriched block objects (ids, timestamps, has_children), so against the real provider Query would conservatively stay unknown rather than confirm. That is the SAFE direction (never fabricated success) but means NO-04 real testing will need a semantic comparison (e.g. type + text content). Flag for the NO-04/A07 follow-up.
4. A 5xx on append is treated as a definitive failure and resume re-sends from BlocksDone; if a provider actually applied a 5xx'd batch, the same page could carry duplicated blocks. Page-level non-duplication (the brief's hard guarantee, NO-03/AC-18) is unaffected; block-level idempotency of append is not offered by the API and not required by the brief.
5. The SaveProgress-failure → ActionUnknown path (318-329) is implemented with an in-code justification but has no test. Worth one regression case.
6. NO-01 mentions content size limits; the adapter enforces the 100-block request cap client-side and delegates per-block size validation to the provider (a 4xx validation answer is a definitive failed — safe). Acceptable; noted.
7. Fixture nit: `createDrop` also hijacks GET /v1/pages (the drop flag is read for every method). Harmless for all current tests; would surprise a future test that GETs while the flag is set.
8. The second Execute in TestNotionParentOutOfScopeRejectedZeroWire (empty-scope case) does not re-assert wire==0 — the first assertion plus the verified ordering cover it; assertion-completeness nit only.

## Blocking violations found
None. W1–W10 all pass on direct source evidence, independent command runs, and two empirical re-derivations of the disclosed repairs.

## Commands and outputs (independently run in the worktree)

```
$ go test ./internal/appconnector -run TestNotion -count=1 -v
--- PASS: TestNotionPartialSuccessDoesNotCreateAnotherPage (0.00s)
--- PASS: TestNotionStepRoutingAndUnknownNeverRouted (0.00s)
--- PASS: TestNotionNO01ContractPinned (0.00s)
--- PASS: TestNotionParentOutOfScopeRejectedZeroWire (0.00s)
--- PASS: TestNotionApprovalRevokedBeforeExecuteBlocks (0.00s)
--- PASS: TestNotionMissingInsertCapabilityBlocks (0.00s)
--- PASS: TestNotionSuccessSavesRealPageID (0.00s)
--- PASS: TestNotionSnapshotFieldByFieldPinned (0.00s)
--- PASS: TestNotionCreateThenLocalCrashResumesWithoutDuplicate (0.00s)
--- PASS: TestNotionPartialAppendSuccessResumesRemainingOnly (0.00s)
--- PASS: TestNotionUnknownCreateStaysUnknownTitleSearchNotProof (0.00s)
--- PASS: TestNotionQueryResolvesPersistedPageByReliableRead (0.00s)
PASS
ok  github.com/Tencent/WeKnora/internal/appconnector 5.649s     # 12/12, matches ledger

$ go test ./internal/appconnector ./internal/agent/approval -count=1
ok  github.com/Tencent/WeKnora/internal/appconnector  0.804s
ok  github.com/Tencent/WeKnora/internal/agent/approval 5.918s

$ go vet ./internal/appconnector        # exit 0, silent
$ go build ./internal/appconnector      # exit 0, silent
$ gofmt -l internal/appconnector/       # empty output (clean)
$ git diff --check                      # exit 0, clean

$ go test ./internal/appconnector -run TestNotion -count=1 -race   # extra
ok  github.com/Tencent/WeKnora/internal/appconnector 2.070s

$ git show --stat 1cfc957
 internal/appconnector/notion_create.go      | 609 ++++++++++++++++++++++++++++
 internal/appconnector/notion_create_test.go | 607 ++++++++++++++++++++++++++++
 2 files changed, 1216 insertions(+)

# Review experiment 1 (repair c re-derivation; scratch edit, then git checkout restore):
#   partial-append test with the original trigger appendFailNext:1 →
#   --- FAIL: TestNotionPartialAppendSuccessResumesRemainingOnly
#       notion_create_test.go:511: completed block range must be persisted after each step,
#       got {PageID:real_page_1 BlocksDone:0}        # contradiction proven; tree restored to HEAD

# Review experiment 2 (repair b re-derivation; standalone program, deleted after):
#   flat parse   of official body: Title=""  err=cannot unmarshal object into ...type string
#   nested parse of official body: Title="Q3 Board Review" err=<nil>
#   flat-shape wire parses only with the NON-official body {"properties":{"title":"..."}}
```

## Verdict
VERDICT: PASS
