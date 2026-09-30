# Craft #107 ownership and integration locks

Source: approved Craft web-artifact Spec, #119–#139 DAG, and implementation plan. Task ID remains the existing Session ID; one Run represents one execution. This matrix assigns writes; dependency order remains the DAG.

## Central seams

| Files | Owner | Rule |
| --- | --- | --- |
| `internal/modules/craft/contracts.go`, `web_contracts.go`, public HTTP DTOs in `internal/handler/session/craft.go`, `packages/contracts/src/craft/index.ts`, `web-artifact.ts` | T00; T20 for integration corrections | Feature lanes consume the frozen names and request a contract amendment through the controller; they do not edit these files. |
| `internal/handler/session/craft_features.go`, `internal/container/craft_features.go`, `packages/views/src/craft/workbench-features.tsx`, `workbench.tsx` | T00; T20 for final composition | Lane handlers register by unique name before router build. Web panels use unique names and one typed slot; T20 passes the immutable feature list. |
| `internal/router/routes_chat.go`, `internal/container/container.go`, `apps/web/src/features/craft/routes.tsx` | T20 | Lane packages expose constructors/adapters; T20 wires them after reviewed checkpoints are integrated. |
| `internal/database/migration.go`, migration numbers and migration files | T20 | Lanes submit schema requirements without reserving a number. T20 allocates ordered numbers at integration. |

## Feature lanes

| Task | Exclusive extension files or area |
| --- | --- |
| T01 | `internal/modules/craft/input.go`, `internal/application/service/craft_inputs.go`, `packages/views/src/craft/files.tsx` |
| T02 | `internal/modules/craft/archive.go`, `internal/application/service/craft_archive.go`, archive fixtures |
| T03 | `internal/modules/craft/input_code.go`, `internal/application/service/craft_delegate.go`, sandbox material policy adapter |
| T04 | Pinned web template/runtime image and lock, OpenCode web skill, `internal/container/craft_runtime.go` |
| T05 | `internal/modules/craft/knowledge.go`, `internal/application/service/craft_knowledge.go`, `packages/views/src/craft/sources.tsx` |
| T06 | `internal/modules/craft/citation.go`, `internal/application/service/craft_citations.go`, generated web citation view |
| T07 | `internal/modules/craft/version.go`, `internal/application/repository/craft_version.go`; `craft_artifacts.go` requires controller lock |
| T08 | `internal/modules/craft/access.go`, `internal/application/service/craft_access.go`, `internal/handler/session/craft_access.go`, `packages/views/src/craft/access.tsx`; supplies `craft.TaskAccessChecker` |
| T09 | `internal/application/service/craft_collaborator_run.go`, `packages/views/src/craft/workbench-edit.tsx` |
| T10 | `internal/application/service/craft_source_open.go`, `internal/handler/session/artifact_reference.go` |
| T11 | `internal/modules/craft/share.go`, `internal/application/service/craft_share.go`, `internal/handler/session/craft_share.go`, `packages/views/src/craft/share.tsx` |
| T12 | `internal/modules/craft/export_manifest.go`, `internal/application/service/craft_export.go`, `internal/handler/session/artifact_download.go` |
| T13 | `internal/modules/craft/export_consent.go`, `internal/application/service/craft_export_consent.go`, `internal/handler/session/craft_export_consent.go`, `packages/views/src/craft/export.tsx` |
| T14 | `internal/modules/craft/preview.go`, `internal/application/service/craft_preview.go`, `internal/handler/session/craft_preview.go`, preview network policy |
| T15 | `internal/modules/craft/release.go`, `internal/application/repository/craft_preview_check.go`; `craft_artifacts.go` requires controller lock |
| T16 | `internal/modules/craft/lifecycle.go`, `internal/application/repository/craft_workspace.go`, `internal/application/service/craft_workspace.go` |
| T17 | `internal/application/service/craft_control.go`, `packages/views/src/craft/status-notice.tsx`; `lifecycle.go` requires controller lock |
| T18 | `internal/modules/craft/recovery.go`, `internal/application/service/craft_recovery.go`, `packages/domain/src/craft/reconnect.ts` |
| T19 | `internal/modules/craft/budget.go`, `internal/application/service/craft_budget.go`, `packages/views/src/craft/usage.tsx` |
| T20 | Final central registration, route composition, migration allocation, web adapter, browser journey and acceptance evidence |

Focused tests follow the owning production file. The controller grants an exclusive lock before any lane edits a shared extension file named above. Backend tests using one database, migrations, fixed ports, image builds, and generated assets must be serialized or given isolated resources. No lane uses a shared stash or mutable test database to coordinate another lane.

The frozen Task-access injection contract is `craft.TaskAccessChecker.CheckTaskAccess(context.Context, craft.Scope, craft.TaskAction) error`. T08 owns the implementation; T05 and T14 consume it for fresh authorization. `craft.RequireTaskAccess` fails closed when the checker or authenticated scope is absent. Feature route registration supplies only the already guarded session group; the handler's service must still enforce Task authorization.
