# T08/#146 final task gate

Date: 2026-09-24 (Asia/Shanghai). Integration branch `codex/issue-140-integration`; base before T08 backend `11d90ce674fe84186797a4bcbd4bbe0a96ea9071`. The final T08 code came through reviewed backend, guard, typed Web client and UI commits. Local commits only; nothing was pushed, merged, deployed, or posted to the Issue.

## Acceptance mapping

| #146 criterion | Evidence |
| --- | --- |
| JD raw text separate from extracted fields; missing fields unknown | Career Office/HTTP tests and live SQLite browser evidence: exact original JD persisted and reopened, all five extracted fields explicitly unknown under `needs_review` because no production extractor is approved. |
| Same request ID replay returns same opportunity or receipt | Backend Office/HTTP replay/conflict, concurrency and ambiguous-outcome tests; reviewed typed receipt recovery. The Web saved state prevents an accidental second POST until a deliberate new draft. |
| Instructions in JD cannot obtain Agent tool permission | Backend import performs no Agent/tool/grant/URL fetch; source reference is inert metadata. Browser rendered the synthetic instruction line literally in the raw evidence section. |
| Evidence page opens from conversation result | Live in-app browser: paste in conversation panel → typed result link with opportunity and snapshot IDs → evidence page → reload showing identical text, source, time and unknown fields. Sign-out redirected fixed evidence URL to `/login`. |

Independent backend final review Spec/quality PASS after three repair increments; backend validator PASS. Guard review PASS/PASS, guard validator focused checks PASS, implementation full Go suite PASS. Web contract final review PASS/PASS and validator PASS. Web UI final review PASS/PASS and validator focused/full suite/typecheck/build PASS; live browser test closed its E2E and visual-layout concerns. Exact reports, repair plans and source SHAs are in this directory.

At final integrated code, `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/... ./tools/architectureguard/...` passed. `pnpm typecheck:web`, focused Opportunity/chat/router 36/36, and `pnpm build:web` passed. `git diff --check 11d90ce674fe84186797a4bcbd4bbe0a96ea9071..HEAD` passed after correcting whitespace in the read-only T09 research memo. The full Web suite passed 2329/2329 at the same UI source revision before the final docs-only commits. The Go suite emitted an existing duplicate `-lc++` linker warning; Vite emitted existing PostCSS import-order, invalid `calc()`, and large-chunk warnings.

The PostgreSQL versioned migration was written/reviewed but not executed because no test DSN was available. SQLite migration up/down/up and live SQLite persistence passed. No production JD field extractor is configured, so a successful import conservatively remains `needs_review` with unknown fields. Parent #140 full OCR and its other DAG nodes remain outstanding; this T08 gate does not claim parent completion.
