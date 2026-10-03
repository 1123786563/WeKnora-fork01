# Delivery 面验证与修复报告（#31 残留处置）· 2026-10-03

分支 main @ 1dfcc88ad；后端 :8084（进程 `/tmp/t40r/server-e0d63bae1`，连 127.0.0.1:5432/WeKnora）。认证：`POST /api/v1/auth/login`（parity LOGIN-RECIPE，env-only 密码）→ Bearer。

## 端点结果（验证前 → 验证后）

| 面 | 前 | 后 |
|---|---|---|
| `GET /api/v1/workbench/inbox` | 200（表已存在） | 200；seed 一行 fixture 后 items/unread=1 可见、`POST /inbox/read` 置读 200（已清理） |
| `GET /api/v1/workbench/executions/{run}/delivery` | **500** `code_delivery_failed`（真实 run 上复现；不存在的 run 是前置 404 run_not_found，掩盖了 500） | **404** `code_delivery_not_found`（干净映射，mobile 口径=absent） |
| `POST /api/v1/workbench/inbox/devices` | **500** `relation "workbench_device_registrations" does not exist` | **200** registered（重复注册 upsert 200，fixture 已清理） |

## 归因（修正先前轮口径）

先前「/delivery 500 = workbench_notifications 迁移未跑」**归因不准**：delivery 读面走 `code_deliveries`（`codedelivery.LatestForRun`），与 notifications 无关。真实根因两条：

1. **环境漂移**：dev WeKnora 库 schema_migrations=271（latest-pointer 单行）但 000196 从未执行——`code_deliveries` 缺表。force-skip 类漂移（task-3-review F3 所述无启动期检测），全量审计共 **16 张迁移应建表缺失**（见附录）。
2. **代码缺口**：`workbench_device_registrations` 无任何生产建表方（模型 DeviceRegistrationRow 仅测试 AutoMigrate；postgres/sqlite 迁移链均无）——inbox devices 面 500 是真 bug，非环境。

## 修复

- `migrations/versioned/000272_workbench_device_registrations.{up,down}.sql` + `migrations/sqlite/000191_...`（双链，列集对齐 DeviceRegistrationRow，唯一索引 tenant+owner+device）
- TDD：`workbench_device_registrations_migration_test.go` 先红后绿（镜像 000209/notifications 测试模式）
- dev 库处置：000196 up.sql 直灌（文件幂等）+ `migrate up` 正规应用 000272（账本 271→272, dirty=f）
- 探针行（sessions/agent_runs/device_registrations/notifications fixture）全部清理，表归零

## 写入管线（#69 轮遗留确认）

`workbench_notifications` 仍**无生产写入方**：inbox handler 仅写 device_registrations、overview 只读计数。接线缺口维持「记录不建设」口径。

## 附录：dev 库剩余漂移（15 表，记录不修——属 F3 启动期检测轮次的系统处置）

agent_licenses, agent_upgrade_proposals, app_action_plan_items, app_action_plans, app_publications, app_space_connection_grants, organization_members, plugin_installations, plugin_previews, task_artifact_annotations, task_compliance_access, task_grants, task_research_delegations, tenant_task_policies, wiki_log_entries

## Concerns

- :8084 仍跑 e0d63bae1 旧二进制（非 1dfcc88ad）；本修复为迁移层，不依赖二进制版本，但下轮重启时应从当前 HEAD 编译。
- 15 表漂移意味着 granted-read（task_grants）等面在 dev 库同样可能 500——任何后续 500 排查先对附录清单。
