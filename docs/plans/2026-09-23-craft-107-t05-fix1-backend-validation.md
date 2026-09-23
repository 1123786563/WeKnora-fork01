# T05 Fix 1 Backend Validation

Status: DONE_WITH_CONCERNS  
Date: 2026-09-23  
Validated worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t05/WeKnora-fork01`  
Revision: `f11fb3e9831db6abd405f47e100e8c2a5f0cf94b` (HEAD unchanged; incremental checkpoint)  
Manifest: `.superpowers/sdd/2026-09-23-craft-107-implementation/t05-fix1-checkpoint-01/manifest.json`  
Manifest SHA-256: `8dbff4761bd1c650aee908b0d1194982a14a6ed2fb80f2ba26052f48d0146d88`

## Scope and acceptance

Validated the assigned T05 Fix 1 behavior only: TaskWrite side-effect guard, per-Run selected material isolation including revoked access, hard 8 KiB UTF-8 excerpt cap and truncation propagation, and record/store compatibility.

All six manifest file hashes matched the worktree at validation time:

```text
0aa560fc7307679704e89c26df13e8eee97154994e499cf60125c9e75ca88c12  internal/modules/craft/knowledge.go
2aec6b0a956dd16536c2bb128e53cd7304a3bf36ca278ba63877821b1b74f70e  internal/modules/craft/knowledge_test.go
4b232baac77d728e41f24d50abcbf676df090a8fe271fd13d3908b98866018b3  internal/application/service/craft_knowledge.go
4571e12acb187ade874f58f9dd4422fca0e182cea3063c5f02468692baf86c56  internal/application/service/craft_knowledge_test.go
370d9b6721f025c0d31a740d1c31c05b61b80cf808a574558a76ad4ee9ad3a94  internal/application/service/craft_knowledge_t05_test.go
d59847b5b53a9bf5c432becb8baa29857026883407a7343ac7b10e62ce53dba1  internal/application/service/craft_knowledge_tool_test.go
```

## Evidence and commands

Commands run at the validated revision/checkpoint:

```sh
go test ./internal/application/service -run 'TestCraftT05(BuildForRunRequiresTaskWriteBeforeSideEffects|SequentialRunsHaveIsolatedMaterial|ExcerptClippingIsBoundedAndDisclosed|Journey)' -count=1
# exit 0: service package passed

go test ./internal/modules/craft/... -run Knowledge -count=1
# exit 0: craft module passed

go test ./internal/application/service/... -run CraftKnowledge -count=1
# exit 0: service and service/file passed; file had no matching tests

go test ./internal/application/repository/... -run CraftKnowledgeRecordSQLite -count=1
# exit 0: repository package passed

git diff --check
# exit 0
```

The implementation report also records passing same-checkpoint focused tests for handler source access: `go test ./internal/handler/session/... -run 'Craft.*Source' -count=1` (exit 0). I did not rerun that broader handler check because this fix leaves handler/HTTP contracts untouched.

## Findings

- **TaskWrite denial:** the service enforces `TaskWrite` at the start of `BuildForRun`; the focused test uses a checker that allows read and rejects write, and asserts an error with zero material writer calls and zero record inserts.
- **Run isolation and revocation:** staging uses `knowledge/runs/<runID>/`. Tests perform sequential Runs with different selected knowledge IDs and inspect the generated manifests/material files; B contains no A excerpt. A revoked source is rejected before writing. Legacy `Build` retains the legacy `knowledge/` path.
- **8 KiB UTF-8 bound:** ASCII and multibyte clipping cases assert the excerpt is valid UTF-8 and no larger than `MaxKnowledgeExcerptBytes` (8192 bytes). Clipping sets bundle truncation and the persisted record and staged manifest expose it.
- **Record/store compatibility:** the fix adds no schema migration or public DTO/route change. The existing SQLite record persistence regression test passes. The service tests confirm truncated state and source excerpt byte count remain available to manifest serialization; existing knowledge tests cover the immutable record shape.
- **Authentication / authorization:** no auth entry point changed. Authorization is checked before retrieval/staging/insertion in this service seam. Broader handler access behavior has same-checkpoint passing evidence as noted above.
- **Cancellation / errors:** the service continues passing `ctx` through access checking, retrieval, writer, and record insertion; errors from those operations return to the caller. No new cancellation-specific behavior was introduced by this fix; cancellation-under-write was not separately exercised.
- **Migrations:** none introduced or needed by the checkpoint.

## Acceptance gap and risk

Backend seam acceptance passes. Full vertical #124 acceptance remains open pending T20 central integration: the Run admission path must bind the authoritative server Run ID to the Task and pass exactly `craft.KnowledgeRunDir(serverRunID)` as the delegate material boundary, using that Run's `manifest.json`. These backend checks cannot prove the caller does so, nor prove end-to-end delegate isolation. Preserve this dependency in the final status.

Runtime model and reasoning effort were not exposed. No source or test files were modified by validation. This report is the only validation artifact written.
