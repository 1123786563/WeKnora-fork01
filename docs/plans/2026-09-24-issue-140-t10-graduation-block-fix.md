# T10 Graduation requirement block review fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop false hard rejection when a graduation-year alternative or negation continues onto a neighboring line.

**Architecture:** A single regex hit is only a candidate. Before comparing the confirmed graduation year, classify the surrounding logical graduation-requirement block across line breaks. If neighboring text may qualify, negate, broaden or contradict the candidate, return `unknown` with no authoritative hard conclusion. A clearly separate skill/project section can end the block so an explicit 2027-only condition still yields ineligible for a confirmed 2026 graduate.

**Tech Stack:** Go Career deterministic parser and Office tests.

**Spec:** #150 and approved Job Search Spec §3/§6.1; T10 initial code `d170cc02e`, first fix `ec8f0f2f60e0041b47ea1113f12d5729f3c8c299`, second independent review high finding at `evaluation.go:367-375`, read-only architect diagnosis in controller Ledger.

## Root-cause investigation

The first repair inspected only the physical line containing `仅限2027届`. The JD `仅限2027届\n或2026届` has one regex hit on a clean first line; the next line is a continuation that changes the condition's meaning. The parser therefore compares the profile year before it has established a complete requirement. The hypothesis is that a conservative block scan (including adjacent continuation/graduation lines) prevents this class of false rejection without suppressing a separate skill line. This is a parsing boundary defect, not a profile-version or persistence defect.

## Global Constraints

- Existing isolated T10 backend worktree only. Own `internal/modules/career/evaluation.go` and `evaluation_test.go`; no migrations, route, Web or guard edits.
- Local code commit authorized; no push/merge/deploy. Preserve exact immutable source span when a rule is recognized; ambiguous text remains unknown and cannot influence an ineligible result.
- If the block cannot be segmented with evidence, prefer unknown. Do not add a permissive parser that makes more hard conclusions from unreviewed text.

## Review Focus

- Confirmed 2026 + standalone `仅限2027届` => ineligible; with a separate `技能：Go` line still ineligible.
- `仅限2027届\n或2026届`, `2026届或\n仅限2027届`, `仅限2027届\n2026届亦可`, `非仅限2027届`, `仅限2027届、2028届`, and CRLF equivalents => unknown.
- A neighboring graduation lexeme, range, alternative or negation is not treated as an unrelated skill section. A clearly labeled skill/project section may terminate the graduation block.
- The prior no-citation, soft token-boundary and changed-intent 409 fixes remain intact.

---

### Task 1: Parse a conservative graduation block

**Depends:** first T10 review fix. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/evaluation.go`, `evaluation_test.go`. **Consumes:** raw immutable JD and candidate `仅限 YYYY 届` span. **Produces:** recognized unambiguous clause or unknown.

- [ ] RED: Add table-driven Office tests from the review matrix above, including LF/CRLF and a clearly separate skill line. Confirm the current line-only parser falsely rejects the wrapped alternative.
- [ ] GREEN: Normalize line boundaries for classification without changing raw bytes or citation offsets. Inspect adjacent logical lines that begin/continue alternatives, contain graduation lexemes or contradict the candidate. Return unknown before profile comparison if the block is ambiguous. Accept a separate labeled skill/project line only when the boundary is clear.
- [ ] VERIFY: Run focused matrix repeatedly, Career `-race` relevant tests, broader Career/router/database/handler/container suites, SQLite migration test unchanged, and `git diff --check`; report exact code SHA and parser limits. Independent Spec/quality re-review and validator are required before integration.

## Shared-file preflight

Only the backend implementer writes Career parser/tests now. The initial and first repair commits remain audit checkpoints. T10 guard and Web implementation are paused until the backend's final reviewed interface is integrated.
