# identity — legacy 遗留文件索引（横向包内归属本模块的文件）

与 `docs/architecture/moves/identity.yaml` 的 `legacy_files` 逐条镜像（同路径、同 Pass B 任务）。
非测试 `.go` 文件；同目录 `_test.go` 随主题文件一并搬迁。

| 文件 | 导航标签 | Pass B 任务 |
|---|---|---|
| `internal/application/repository/audit_log.go` | Audit log (application/repository) | `B-identity` |
| `internal/application/repository/mobile_auth_exchange.go` | Mobile auth exchange (application/repository) | `B-identity` |
| `internal/application/repository/organization.go` | Organization (application/repository) | `B-identity` |
| `internal/application/repository/tenant.go` | Tenant (application/repository) | `B-identity` |
| `internal/application/repository/tenant_api_key.go` | Tenant api key (application/repository) | `B-identity` |
| `internal/application/repository/tenant_invitation.go` | Tenant invitation (application/repository) | `B-identity` |
| `internal/application/repository/tenant_member.go` | Tenant member (application/repository) | `B-identity` |
| `internal/application/repository/user.go` | User (application/repository) | `B-identity` |
| `internal/application/service/audit_log.go` | Audit log (application/service) | `B-identity` |
| `internal/application/service/audit_log_retention.go` | Audit log retention (application/service) | `B-identity` |
| `internal/application/service/organization.go` | Organization (application/service) | `B-identity` |
| `internal/application/service/password_policy.go` | Password policy (application/service) | `B-identity` |
| `internal/application/service/tenant.go` | Tenant (application/service) | `B-identity` |
| `internal/application/service/tenant_api_key.go` | Tenant api key (application/service) | `B-identity` |
| `internal/application/service/tenant_invitation.go` | Tenant invitation (application/service) | `B-identity` |
| `internal/application/service/tenant_member.go` | Tenant member (application/service) | `B-identity` |
| `internal/application/service/user.go` | User (application/service) | `B-identity` |
| `internal/handler/audit_log.go` | Audit log (handler) | `B-identity` |
| `internal/handler/auth.go` | Auth (handler) | `B-identity` |
| `internal/handler/auth_register_by_invite.go` | Auth register by invite (handler) | `B-identity` |
| `internal/handler/organization.go` | Organization (handler) | `B-identity` |
| `internal/handler/rbac_lookups.go` | Rbac lookups (handler) | `B-identity` |
| `internal/handler/tenant.go` | Tenant (handler) | `B-identity` |
| `internal/handler/tenant_invitation.go` | Tenant invitation (handler) | `B-identity` |
| `internal/handler/tenant_invite_link.go` | Tenant invite link (handler) | `B-identity` |
| `internal/handler/tenant_member.go` | Tenant member (handler) | `B-identity` |
| `internal/handler/tenant_policy.go` | Tenant policy (handler) | `B-identity` |
