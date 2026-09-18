# 2026-09-18 Round R473 — Fixture library rebuilt (tenant 10002), queued-steer chips landed, questions pairing partial (4 parallel agents)

Round type: recovery + TDD round with 4 parallel agents (A1 fixture rebuild, A2 queued-steer chips, A3
questions Vue-side pairing + skipped capture, A4 verifier). Verdict: A1 DELIVERED (its report landed after
A4's check deadline — the fixture table below is from A1's final state); A2 PASS (+18 tests, 1826/1826);
A3 partial through environment breaks with high-value captures; gates: test:web 1826/1826, test:shared
795/795, typecheck 0, build ✓, integrity 1 P0 (unstaged new file — staging clears).

## A1 — Fixture library rebuilt (DELIVERED; new infrastructure for all future browser rounds)

Diagnosis: the WeKnora DB was a COMPLETE empty schema (154 tables absent — the post-wipe backend wasn't
running migrations); two backend restarts + renaming two stale commercial unique constraints restored it.
**New fixture table (tenant 10001 is gone — taken over by a parallel agent):**

| Fixture | New id | Notes |
|---|---|---|
| parity account | tenant **10002** | same credentials |
| Parity KB Demo | KB `76b81ceb` | docs `29df436f` (smoke) + `55a1ed54` (mermaid block) |
| wiki-fixture | KB `09d5ab6b` | 4 wiki pages (incl. concept/source-doc-traceability → doc `ca54abb4`), graph links live |
| shared fixture | KB `165f9c5f` (owner shared_fixture, tenant 10007) | org `013ae9d3`, share `95bc3f21` (viewer) |
| 「知识库检索讨论」session | `faeb6017` | builtin engine |

Key operational findings: embedding is unreachable (DNS-polluted + SSRF guard) — the working strategy is
**wiki-only indexing** (passes the ≥1-strategy validation, skips NeedsEmbeddingModel, chunks generate);
documents rest at finalizing (mock summary unavailable) but chunks are usable; uploads write the worktree
`.local-data` while reparse reads the main repo's (delete+re-upload is more reliable than reparse); React :5181
listens IPv6-only. Env fixes recorded for code follow-up (the two commercial AutoMigrate constraints).

## A2 — Queued-steer chips (PASS)

Vue contract: chips render inside the composer top (delivery==='after' only; inject rides the message
bubbles) — clock icon + truncated text (title=full), per-state actions (failed→retry, pending→loading,
normal→promote/remove); promote is NOT reorder — it converts after→inject (POST /steer/:id/inject, read at
the next round boundary); remove cancels (DELETE); consumption sends ONE awaiting steer per turn (chained),
follow-up runs attach via list-resync; stop clears the whole queue; session switch clears; resume rehydrates.
React: steer-queue.ts pure model (settle/rebase/hydrate) + ChatRoutePage handlers with two stale-closure
fixes; chips UI in the composer; 4 new keys ×5 locale byte-exact. +18 tests (red evidenced); 1826/1826.
A4 additionally found N1 (a narrow race: enqueue-while-idle returning new_run drops the message without a
send fallback — the promote path handles it correctly) — queued R474. Also deferred: ⌘Enter promote-first
shortcut, awaitingIdleSend mention re-injection, a promoting-specific disabled state.

## A3 — Questions pairing partial; pending-span diffs captured (environment-blocked quality)

A3's window overlapped A1's rebuild and hit the broken pipeline (docreader couldn't read local-storage files;
vectorstore container absent → chunks not persisting). Delivered: a full CODE-level questions diff (Vue's
t-popup bottom-right floating form with popconfirm vs React's inline expansion panel — a form-factor gap
queued), a possible missing legacy-question guard in React (P2 review queued), and a NEW live capture — the
same pending span renders React「进行中」 vs Vue Trace「—」 with no status text (plus Vue-only LIVE badge /
attempt switch / stage n/5 / 停止解析 controls). 6 screenshots; all self-created fixtures cleaned.

## Gates (final)

test:web 1826/1826 (+18), test:shared 795/795, typecheck 0, build ✓, integrity 1 P0 (the unstaged
steer-queue.ts — staging clears). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r473/report-A{1,2,3}.md + report-A4-review.md.
