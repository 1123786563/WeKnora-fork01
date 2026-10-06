# Craft T20 post-F08 live browser result — verified blocker

## Run and cleanup

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-execution/WeKnora-fork01`
- Git HEAD: `6e1b2072a13a798e7ec25be44784e5242bd2f178` (detached `main`).
- Command: `CRAFT_STACK_TAG=craft107-final-20261006-rerun1 bash apps/web/e2e/craft-stack.sh up mock`; then `... run mock`; then `... down mock`.
- Results: `up=0`; Playwright `run=1`; precise `down=0`.
- Playwright: 1 of 6 scenarios failed; five were skipped after serial scenario 01 failed. Scenario 01 reached the parent-run completed UI but had no published version; `craft-version` input stayed empty at `apps/web/e2e/craft-report.spec.ts:125`.
- Cleanup stopped the exact PID groups recorded by the harness. Ports `41871–41878` and `41883` had no listener before launch and were checked free after teardown. Existing unrelated Docker containers were left untouched.
- Sanitized command output, source hashes and exit codes are in `docs/testing/craft/t20/2026-10-06-post-f08/`; the screenshot and Playwright error context are copied under its `artifacts/` subdirectory. The original private temp run was `/tmp/craft-craft107-final-20261006-rerun1-mock.Hwmdv9`.

## Reproduced failure and persisted state

Playwright page context showed the main Craft Run status `已完成`, no previewable artifact, and a selected-version field with only `暂无版本`.

The isolated SQLite state was queried before discarding the test DB:

- `agent_runs`: the parent run ended `succeeded`.
- `agent_run_events`: the model emitted `craft_delegate`; the run completion text then claimed delegation completed.
- `agent_tool_calls`: `craft_delegate` status was `failed`; its structured tool result was `craft delegation failed: craft RunView runtime unresolved: verified RunView material resolver is not assembled`.
- `craft_delegations`: row remained `prepared`, revision `0`, with no result.
- `craft_versions`: no version was published.

The model fixture returns a canned success sentence after any tool result, including a failed result. The empty version and persisted failed tool outcome are the acceptance facts; the UI completion label is not counted as success.

## Cause and why no local pin substitution was made

`internal/container/container.go:3164-3180` requires all six explicit RunView deployment inputs: `CRAFT_RUNVIEW_SANDBOX_ROOT`, `CRAFT_RUNVIEW_IMAGE_REFERENCE`, `CRAFT_RUNVIEW_IMAGE_DIGEST`, `CRAFT_RUNVIEW_RUNTIME_CONFIG_SHA256`, `CRAFT_RUNVIEW_PROJECT_ID`, and `CRAFT_RUNVIEW_DOCKER_ENDPOINT`. They were all unset in this run. `provideCraftRunViewProductionAssembly` therefore remains unavailable; its `ResolveMaterial` is nil, and the delegated executor refuses before writing any output. The harness script does not supply these pins.

`docker/craft/opencode.lock.json` records version `1.18.4` and the local image ID but explicitly leaves `container_digest` null/unpublished. `CRAFT_RUNVIEW_IMAGE_DIGEST` is a deployment-owned immutable pin; source comments/tests explicitly prohibit inferring it from local Docker image IDs or daemon RepoDigests. The available local tag `weknora-craft-runtime:1.18.4` and its local Docker metadata therefore do not establish an authorized production pin. The repository lock says the matching `docker/craft/runtime-config.json` SHA-256 is `8c706eaa33be38d6c6b586f18a5999cbfa3a56d3b6b474aef7d6126d40e79f10`; this value alone cannot satisfy the missing image digest or the other deployment identities.

Even with the RunView pins, promotion remains fail-closed: `RegisterCraftWebPageLoadProbe` exists only as an unregistered slot (`internal/container/craft_run_capture_promotion.go`), and current production search finds no implementation registered. T14 #129's live Issue body requires an actual browser to load local HTML/CSS/JS/assets and browser-level denied-egress evidence. The prior T20 report records the same prerequisites as deployment/T14 gates. No image pin or browser proof was fabricated.

## Ruling

T20 is not verified. Its end-to-end acceptance remains blocked on an authoritative immutable RunView image/pin configuration and integrated T14 browser-load/no-egress probe evidence. T14 #129 is also not verified against its live Issue acceptance. Focused code changes for the newly discovered webhook container and auth wiring have separate SDD reports; they do not clear these independent security gates.

Safe continuation requires a deployment-owned RunView pin manifest (especially the authoritative immutable container digest, plus the remaining environment identities) and the T14 real-browser probe implementation/evidence. After these arrive, rerun the same official harness with those explicit pins, verify run capture/promotion/page-load evidence, and update the DAG. Do not set the two nodes verified before that run.

## Evidence handling

The repository retains command logs, exit codes, screenshot, and error context. Raw trace, storage-state files, credentials, SQLite DB/WAL files, and server logs were not copied into the repository because they may contain bearer tokens or generated credentials. They were kept only in the run's private temp directory pending precise cleanup.
