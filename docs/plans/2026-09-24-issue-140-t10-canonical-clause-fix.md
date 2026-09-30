# T10 canonical graduation clause repair

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent false hard rejection from a question or any unparsed qualifier anywhere in a JD.

**Architecture:** The hard `ineligible` and rule-level `eligible` graduation comparison applies only when the **entire JD** is one canonical, affirmative `仅限 YYYY 届` assertion, aside from surrounding whitespace and an optional declarative full stop. Any other text, second line, soft field, question mark, or unrecognized punctuation yields `unknown` for this hard rule. Keep raw citation offsets and immutable evaluation output. Soft literal evidence may still be shown separately and cannot change hard unknown.

**Sources:** #150 and approved Job Search Spec; T10 initial plan and prior three parser repair plans; independent third review at code `85ee5cac2` finding high `技能：Go，其他批次均可` and medium `仅限2027届？` false hard rejection.

## Root cause and decision

The previous plans allowed unrelated soft fields and attempted to reject qualifiers with a denylist. Natural language can express exceptions outside any small denylist. The earlier line/block/full-JD repairs each missed a counterexample. A positive hard conclusion therefore requires a tiny whole-input grammar. #150 requires an unambiguous 2027-only JD versus confirmed 2026 to be `ineligible`, and missing graduation to be `unknown`; it does not require interpreting multi-line JDs with other prose. This conservative rule deliberately returns `unknown` for mixed JDs pending later human review.

## Global Constraints

- Work only in T10 isolated backend worktree, `internal/modules/career/evaluation.go` and `evaluation_test.go`. No route, migration, Web, guard, or data-shape changes. Preserve others' edits.
- Local commit authorized; no push, merge, deploy. The JD is inert data. Maintain exact original citation span for accepted canonical text.
- Fourth parser repair needs a stronger implementation review context: previous backend_implementer attempts passed tests but repeatedly missed semantic exceptions. Controller assigns a fresh `default` agent on Sol for this bounded code edit, with backend ownership/constraints; this is the AGENTS.md fix-round upgrade, not a parallel implementation stream.

## Review Focus

- Confirmed 2026 + `仅限2027届` (surrounding whitespace, optional `。` or `.`) => ineligible; confirmed 2027 may satisfy this rule. Missing/unconfirmed graduation => unknown.
- `仅限2027届？`, `仅限2027届?`, `仅限2027届\n技能：Go`, `仅限2027届\n技能：Go，其他批次均可`, `仅限2027届\n2026届亦可`, and any extra nonblank content => unknown for the graduation rule. Include LF/CRLF and raw citation checks.
- Preserve previous no fabricated citation, soft `Go`/`Google` boundary, changed-intent 409, owner/Tenant scope and immutable replay behavior.

---

### Task 1: Replace permissive text classification with canonical whole-input match

**Depends:** T10 backend checkpoint `85ee5cac2`. **Owner/validator:** upgraded backend implementer / backend_validator. **Files:** `evaluation.go`, `evaluation_test.go`. **Consumes:** immutable JD raw text and pinned confirmed profile facts. **Produces:** canonical clause comparison or unknown.

- [ ] RED: Add the two reviewer counterexamples and matrix above; demonstrate false hard results under current code.
- [ ] GREEN: Remove the line and soft-field acceptance path from hard graduation parsing. Match one whole-input affirmative clause only; do not use a growing exception denylist. Only declarative terminal punctuation can be accepted. Preserve source offsets from raw text.
- [ ] REFACTOR: Keep code small and document the intentional unknown behavior. Do not alter soft matching, persistence, or route semantics.
- [ ] VERIFY: Focused matrix repeated, race tests including changed-intent, broader Career/router/database/handler/container suites, `git diff --check`. Report code SHA and limits. Separate independent reviewer and validator check this checkpoint before integration.

## Shared-file and interface preflight

Only this upgraded implementer writes the two parser files in the existing isolated worktree. Reviewers/validators are read-only. Go API and migrations stay frozen, so downstream architectureguard and Web tasks wait for reviewed integration.
