# agentcatalog — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/agentcatalog.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/agent_marketplace.go` | Agent marketplace (application/repository) | `B-agentcatalog` |
| `internal/application/repository/agent_share.go` | Agent share (application/repository) | `B-agentcatalog` |
| `internal/application/repository/agent_version.go` | Agent version (application/repository) | `B-agentcatalog` |
| `internal/application/repository/custom_agent.go` | Custom agent (application/repository) | `B-agentcatalog` |
| `internal/application/repository/expert_install.go` | Expert install (application/repository) | `B-agentcatalog` |
| `internal/application/repository/published_expert.go` | Published expert (application/repository) | `B-agentcatalog` |
| `internal/application/repository/published_skill.go` | Published skill (application/repository) | `B-agentcatalog` |
| `internal/application/repository/tenant_disabled_shared_agent.go` | Tenant disabled shared agent (application/repository) | `B-agentcatalog` |
| `internal/application/repository/tenant_skill.go` | Tenant skill (application/repository) | `B-agentcatalog` |
| `internal/application/repository/tenant_subagent.go` | Tenant subagent (application/repository) | `B-agentcatalog` |
| `internal/application/repository/user_resource_favorite.go` | User resource favorite (application/repository) | `B-agentcatalog` |
| `internal/application/service/agent_browser_preferences.go` | Agent browser preferences (application/service) | `B-agentcatalog` |
| `internal/application/service/agent_marketplace.go` | Agent marketplace (application/service) | `B-agentcatalog` |
| `internal/application/service/agent_service.go` | Agent service (application/service) | `B-agentcatalog` |
| `internal/application/service/agent_share.go` | Agent share (application/service) | `B-agentcatalog` |
| `internal/application/service/agent_version.go` | Agent version (application/service) | `B-agentcatalog` |
| `internal/application/service/custom_agent.go` | Custom agent (application/service) | `B-agentcatalog` |
| `internal/application/service/expert_market_source.go` | Expert market source (application/service) | `B-agentcatalog` |
| `internal/application/service/expert_service.go` | Expert service (application/service) | `B-agentcatalog` |
| `internal/application/service/expert_skills.go` | Expert skills (application/service) | `B-agentcatalog` |
| `internal/application/service/skill_market_service.go` | Skill market service (application/service) | `B-agentcatalog` |
| `internal/application/service/subagent_service.go` | Subagent service (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_expert_market_service.go` | Tenant expert market service (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_admin.go` | Tenant skill admin (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_bundle.go` | Tenant skill bundle (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_catalog.go` | Tenant skill catalog (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_effective.go` | Tenant skill effective (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_env_declare.go` | Tenant skill env declare (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_files.go` | Tenant skill files (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_install.go` | Tenant skill install (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_market_service.go` | Tenant skill market service (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_progress.go` | Tenant skill progress (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_reaper.go` | Tenant skill reaper (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_remove.go` | Tenant skill remove (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_runtime_verify.go` | Tenant skill runtime verify (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_service.go` | Tenant skill service (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_source.go` | Tenant skill source (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_steer.go` | Tenant skill steer (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_stop.go` | Tenant skill stop (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_transcript.go` | Tenant skill transcript (application/service) | `B-agentcatalog` |
| `internal/application/service/tenant_skill_verify.go` | Tenant skill verify (application/service) | `B-agentcatalog` |
| `internal/application/service/user_resource_favorite.go` | User resource favorite (application/service) | `B-agentcatalog` |
| `internal/handler/agent_marketplace.go` | Agent marketplace (handler) | `B-agentcatalog` |
| `internal/handler/agent_version.go` | Agent version (handler) | `B-agentcatalog` |
| `internal/handler/custom_agent.go` | Custom agent (handler) | `B-agentcatalog` |
| `internal/handler/expert.go` | Expert (handler) | `B-agentcatalog` |
| `internal/handler/persona.go` | Persona (handler) | `B-agentcatalog` |
| `internal/handler/shared_agent_access.go` | Shared agent access (handler) | `B-agentcatalog` |
| `internal/handler/skill_catalog.go` | Skill catalog (handler) | `B-agentcatalog` |
| `internal/handler/skill_handler.go` | Skill handler (handler) | `B-agentcatalog` |
| `internal/handler/skill_market.go` | Skill market (handler) | `B-agentcatalog` |
| `internal/handler/subagent.go` | Subagent (handler) | `B-agentcatalog` |
| `internal/handler/tenant_expert_market.go` | Tenant expert market (handler) | `B-agentcatalog` |
| `internal/handler/tenant_skill_market.go` | Tenant skill market (handler) | `B-agentcatalog` |
| `internal/handler/user_resource_favorite.go` | User resource favorite (handler) | `B-agentcatalog` |
