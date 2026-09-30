# Task Brief: F10 corrupt gzip trailer regression

- **Issue/finding:** Craft #107 F10 / Task J; independent review LOW gap (no regression for corrupt gzip trailer after valid tar EOF).
- **Spec/context:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`; `CONTEXT.md`; Task J in `docs/plans/2026-09-28-craft-107-ocr-r2-plan.md`; current implementation evidence `docs/plans/2026-09-28-craft-107-ocr-r2-pax-report.md`; plan `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-plan.md`.
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/ocr-pax-budget/WeKnora-fork01` (existing isolated dirty task lane).
- **BASE:** `4dc97ff8a32bff93f9a97a0f6e0d87e071acb159`; no commits/staging.
- **Ownership:** only `internal/modules/craft/archive_test.go` and SDD plan/report/review-package artifacts for this repair. `archive.go` is read-only unless test proves behavior incorrect; any production edit must be minimal and documented.
- **Resources:** no Docker/network/database; Go package compile/test cache only.
- **Task:** add valid tar.gz fixture then flip a deterministic gzip CRC byte after tar termination; assert `ExtractArchive` returns `ErrInvalidInput`. Run focused test, complete archive selector, `git diff --check`.
- **Report:** `docs/plans/2026-09-29-craft-107-pax-gzip-trailer-fix-report.md`, with starting/final hashes, patch SHA and exact outputs.
