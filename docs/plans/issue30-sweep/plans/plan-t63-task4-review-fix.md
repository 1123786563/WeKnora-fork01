# T63 Task4 Review Fix — bind lifecycle service interface in container

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow repair to T63 Task4 commit `750e2777148669c70ad4d0b8e85940ab13b5fbc4`.

**Goal:** Ensure the production Dig container can resolve the lifecycle handler and router after Task4 wiring.

**Finding:** HIGH T63-T4-R1: container provides concrete `*AgentMarketplaceLifecycleService`, but handler constructor requests `interfaces.AgentMarketplaceLifecycleService`; Dig does not bind concrete to interface implicitly, no interface provider exists, and `RouterParams` requires the handler. Router construction will fail at startup.

## Constraints
- Continue in existing T63 worktree on Task4 implementation branch; only handler/service/container/router/tests directly required for explicit binding and container resolution coverage.
- Preserve established service API and route security. No unrelated T63 Task5 admission files.
- Local commit authorized, no external access.

## Task 1 — explicit interface binding and startup test
**Role:** backend_implementer; independent review + validation follow.
1. RED: add test resolving actual app container/router (or narrow Dig graph) and prove current Task4 graph fails due missing interface; ensure test invokes route registration/container assembly rather than only constructors.
2. GREEN: provide explicit adapter/provider whose output type is `interfaces.AgentMarketplaceLifecycleService` backed by concrete implementation, or revise handler constructor type if compatible with plan and architecture. Prefer explicit interface provider. Add container-resolution assertion that `NewRouter` succeeds and lifecycle routes mount.
3. Run targeted container/router tests, build, diff-check; commit exact owned files and report.

**Review focus:** actual production DI graph resolves; no cycle/duplicate provider; all four routes still protected and present.
