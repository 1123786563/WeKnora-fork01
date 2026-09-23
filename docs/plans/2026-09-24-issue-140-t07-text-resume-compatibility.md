# T07 real DocReader text-resume compatibility fix

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make an advertised, validated `.txt` resume produce pending Career proposals when the configured production DocReader does not implement `txt`.

**Architecture:** The upload adapter already validates the `.txt` extension/MIME/content and stores scoped bytes before parsing. For validated UTF-8 plain text, decode those stored bytes directly as the extracted text; keep DocumentReader for `.pdf`, `.docx` and `.doc`. Continue using the same deterministic field extraction, source/receipt transaction, resource binding, cleanup and replay paths. Never make invalid UTF-8 or an unsupported extension successful.

**Tech Stack:** Go Career UploadAdapter, real DocReader protocol integration, SQLite Lite HTTP acceptance.

**Sources:** Issue #147 and Task 7; integrated backend code at `internal/modules/career/upload.go`; real production DocReader container `wechatopenai/weknora-docreader:latest` on 2026-09-24 logged `Unsupported file type: txt` for a 355-byte valid UTF-8 `.txt` resume while the same content packaged as `.docx` produced seven proposals. Browser UI advertises `.txt`; adapter accepts `.txt`. This is a concrete integration defect, not an assumed parser behavior.

## Global Constraints

- Own only `internal/modules/career/upload.go` and focused tests; no Web, router, migrations, catalog, or unrelated parser code change.
- Preserve private raw bytes/text scope and the current source/receipt transaction and cleanup semantics. No automatic fact confirmation.
- A plain text file still must pass existing safe-name, size, declared MIME, content signature and UTF-8 checks. Do not treat arbitrary binary as text.
- Local task commits authorized; no push/merge/deploy. Independent validator and reviewer are required before integration.

## Review Focus

- A real UTF-8 `.txt` upload yields exact text for deterministic extraction even when DocumentReader rejects `txt` or is unavailable; the parser is not called for that format.
- `.docx`/`.pdf` continue to use DocumentReader; their failure behavior remains visible and does not erase confirmed facts.
- The direct-text branch has no new authorization, storage, resource lifecycle, replay or privacy side effects.

---

### Task 1: Parse validated plain text locally

**Depends:** T07 reviewed backend and current integrated API/Web contracts. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/upload.go`, `internal/modules/career/upload_test.go` and at most one focused adjacent Career test. **Consumes:** validated/stored scoped resume bytes. **Produces:** existing `UploadResult` with extracted plain text and proposals through unchanged Office seam.

- [ ] RED: Add a test that sends a valid labeled UTF-8 `.txt` resume through UploadAdapter with a DocumentReader stub that errors if called. Assert six categories, exact evidence, missing graduation year and multiple-experience review flag. Add invalid UTF-8/MIME or binary rejection coverage if existing focused tests do not already prove it. Verify RED fails because the current adapter calls the unsupported reader.
- [ ] GREEN: After successful validation and storage readback, branch only `.txt` to `string(storedData)`; other formats call DocumentReader as before. Reuse the existing post-read nonempty/normalization/extractor path. Avoid copy/paste of success/cleanup logic.
- [ ] REFACTOR/VERIFY: Run `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/handler/... ./internal/container/...`, `git diff --check`; save code SHA/report. Root reruns a fresh isolated Lite HTTP upload using the real local DocReader container and both `.txt` and `.docx` fixtures. Independent validation and Spec/quality review gate integration.

## Shared-file and interface preflight

The Web file accept list and backend validation both already include `.txt`; no wire change is needed. The integration worktree owns plans/evidence while the backend implementation runs in a new isolated worktree. No other active task writes `upload.go`. T08 implementation waits for this fix and its integration.
