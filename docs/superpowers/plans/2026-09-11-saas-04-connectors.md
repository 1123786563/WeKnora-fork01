# SaaS 04 App 与 Connector Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一空间安装、授权、同步与两个首批外部写操作。

**Architecture:** 保留现有DataSource/MCP适配，新增持久化安装、连接和Action执行层；审批与预算同时满足才派发。

**Tech Stack:** 当前仓库 Go 1.26、Gin、GORM、PostgreSQL／SQLite 版本化迁移；React 19、TypeScript 6、pnpm 10.28.2；官方 OpenMeter 固定版本；优先复用现有依赖。

**Spec:** [完整规格](../specs/2026-09-10-saas-billing-connectors-design.md)；[接口验证清单](../specs/2026-09-10-saas-billing-connectors-interface-verification.md)。执行者必须同时阅读本计划引用的规格章节与上下游契约。

## Global Constraints

- 每个空间始终独立购买、付款并承担费用；共享组织不改变归属，不共享余额或套餐。
- 费用预算批准和外部写入批准是独立条件，两者都满足才能执行。
- 首期不包含：跨空间合并付款或余额池、自动扣款、独立席位与存储加购包、嵌入式第三方 Web 应用、公开开发者上架与交易分成。
- 固定候选基线：`v1.0.0-beta.232`，提交 `887e0cac903ccd06e74d61ed23c651651d10c7a9`。
- 未在验证清单取得真实通过证据的外部能力，不得标记为生产可用；不使用旧本地 OpenMeter fork 接口。
- 保留已有 React、Craft、移动端和其他任务的工作；仅提交本任务明确拥有的文件，不使用 `git add .`。
- 下列内部接口是计划新增契约，不声称当前已存在；外部请求字段以 P00 的已验证契约为准。
- 时区、精度与分段报价采用完整规格的推荐默认值；外部模型无法无损表达时在 P00 停止依赖任务并修正规格，不能自行换商业语义。

---

## 依赖与执行边界

A01/A02可在商业接口实验期间实现；A03及后续收费执行依赖U05。真实FS/NO写入需单独具备测试目标和授权，设计或计划批准不自动等于当前外部发送授权。

本文件仅编写计划，不表示已实现或已测试。执行时先使用 using-git-worktrees 建立隔离工作区；所有命令从仓库根执行。每一步只做所列一个动作；较长代码修改按列出的文件／函数逐项执行，完成该任务的 RED→GREEN 与独立 review 后再进入下一任务。

## 文件结构与职责

appconnector：纯契约与适配；repository/service/appconnector：持久权限、intent、恢复；原datasource接口保持兼容；原approval Gate变为产品持久审批的交互适配。

### Task A01: 空间安装、版本与两类连接

**Files:**
- Create: `internal/appconnector/model.go`、`internal/appconnector/access.go`、`internal/appconnector/access_test.go`
- Create: `internal/application/repository/appconnector/install.go`
- Create: `migrations/versioned/000117_app_installations.up.sql`、`migrations/versioned/000117_app_installations.down.sql`
- Create: `migrations/sqlite/000037_app_installations.up.sql`、`migrations/sqlite/000037_app_installations.down.sql`

**Interfaces:**
- Consumes: 现有TenantMember与DataSourceConfig；F02空间权限。
- Produces: `Installation{ID,AppID,Version,State string; TenantID uint64}`；`Connection{ID,InstallationID,Kind,OwnerID,CredentialRef,State string; TenantID uint64; AuthVersion int64}`；`CanUseConnection(Connection,tenant uint64,actor string,spaceGrant bool) bool`。

**行为与边界：** 同 App 多空间独立、同安装多连接、个人连接隔离、空间连接显式授权、升级scope扩大、停用后新调用拒绝；安装权限不等于商业购买权限。

**验收映射：** B08/B09；CON-01/02；AC-15

- [ ] **Step 1: 在 `internal/appconnector/access_test.go` 写入以下失败断言。**

```go
package appconnector
import "testing"
func TestPersonalConnectionCannotBeSharedByInstallation(t *testing.T) {
    c:=Connection{TenantID:7,Kind:"personal",OwnerID:"u1",State:"active"}
    if CanUseConnection(c,7,"u2",true) { t.Fatal("personal token shared") }
    if CanUseConnection(c,8,"u1",true) { t.Fatal("cross-space token used") }
    if !CanUseConnection(c,7,"u1",false) { t.Fatal("owner rejected") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector -run "Test(PersonalConnection|Installation)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanUseConnection(c Connection,tenant uint64,actor string,spaceGrant bool) bool {
 if c.TenantID!=tenant || c.State!="active" { return false }
 if c.Kind=="personal" { return c.OwnerID==actor }
 return c.Kind=="space" && spaceGrant
}
```

迁移保存 app_versions（app/version唯一、schema_json、risk_json）、installations（tenant/app唯一、version/state/config_json）、connections（tenant/id唯一、installation、kind、owner、credential_ref、auth_version），credentials 不放 config_json。Create/Upgrade/Disable 的 repository 方法固定签名 `ApplyInstallation(ctx context.Context, i appconnector.Installation, expected int64) error`；更新 CAS 后查目标版本。新scope需要 reauthorization 状态，不自动active。Owner/Admin才能安装，普通成员仅申请。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector -run "Test(PersonalConnection|Installation)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/model.go internal/appconnector/access.go internal/appconnector/access_test.go internal/application/repository/appconnector/install.go migrations/versioned/000117_app_installations.up.sql migrations/versioned/000117_app_installations.down.sql migrations/sqlite/000037_app_installations.up.sql migrations/sqlite/000037_app_installations.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: add space installations and scoped connections"
```

### Task A02: OAuth绑定、凭据引用和撤销接线

**Files:**
- Create: `internal/appconnector/oauth.go`、`internal/appconnector/oauth_test.go`
- Create: `internal/application/service/appconnector/credentials.go`
- Modify: `internal/application/repository/mcp_oauth.go`、`internal/application/service/tenant_member.go`
- Create: `internal/handler/app_connections.go`

**Interfaces:**
- Consumes: A01 Connection.AuthVersion；现有MCPOAuthRepository.TryAcquireTokenRefreshLease、SaveTokenForPrincipal、internal/utils/crypto.go。
- Produces: `OAuthBinding{State,InstallationID,ActorID string; TenantID uint64; ExpiresAt time.Time; Used bool}`；`ValidateOAuthBinding(OAuthBinding,tenant uint64,actor string,now time.Time) error`；`CredentialResolver.Resolve(ctx context.Context,connectionID string,expectedVersion int64)([]byte,error)`，内部接口禁止作为HTTP结果返回。

**行为与边界：** 一次性state、过期、错tenant/actor、并发refresh、撤销和输出脱敏均测试；不能把oauth结果完整对象直接塞模型上下文。

**验收映射：** B09/B10；CON-03/04；AC-15

- [ ] **Step 1: 在 `internal/appconnector/oauth_test.go` 写入以下失败断言。**

```go
package appconnector
import("testing";"time")
func TestOAuthCallbackCannotRebindSpace(t *testing.T) {
    b:=OAuthBinding{State:"random",TenantID:7,ActorID:"u1",ExpiresAt:time.Now().Add(time.Minute)}
    if ValidateOAuthBinding(b,8,"u1",time.Now())==nil { t.Fatal("cross-space OAuth accepted") }
    b.Used=true
    if ValidateOAuthBinding(b,7,"u1",time.Now())==nil { t.Fatal("state replay accepted") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector ./internal/application/repository -run "Test(OAuth|MCPOAuth)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func ValidateOAuthBinding(b OAuthBinding,tenant uint64,actor string,now time.Time) error {
 if b.State=="" || b.TenantID!=tenant || b.ActorID!=actor || b.Used || !now.Before(b.ExpiresAt) {
   return errors.New("oauth_binding_invalid")
 }
 return nil
}
```

callback 在单事务消费 state，绑定 Installation 与发起人的当前 membership；仅按提供方核验过的回调／PKCE规则交换。凭据用现有加密封装保存，Connection 仅存引用，refresh 复用已有租约并校验 auth_version。成员离开把个人连接状态改 revoked 并递增版本；空间Connection不因一个管理员离开而默认换成个人连接。Resolve 在派发前检查当前成员与连接版本，撤销后不再刷新／派发。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector ./internal/application/repository -run "Test(OAuth|MCPOAuth)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/oauth.go internal/appconnector/oauth_test.go internal/application/service/appconnector/credentials.go internal/application/repository/mcp_oauth.go internal/application/service/tenant_member.go internal/handler/app_connections.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: bind connector credentials to revocable space principals"
```

### Task A03: 持久化Action审批与未知结果状态

**Files:**
- Create: `internal/appconnector/action.go`、`internal/appconnector/action_test.go`
- Create: `internal/application/repository/appconnector/action.go`
- Create: `internal/application/service/appconnector/action.go`
- Create: `migrations/versioned/000118_app_actions.up.sql`、`migrations/versioned/000118_app_actions.down.sql`
- Create: `migrations/sqlite/000038_app_actions.up.sql`、`migrations/sqlite/000038_app_actions.down.sql`
- Modify: `internal/agent/approval/gate.go`

**Interfaces:**
- Consumes: A01/A02权限版本；U05 ExecutionGate；现有approval.PendingRequest/Decision作为交互适配，不能用内存pending当持久权威。
- Produces: `Action{ID string; TenantID uint64; ActorID,ConnectionID,Version,Target,Risk string; Args json.RawMessage; AuthVersion int64}`；`ActionDigest(Action)(string,error)`；`NeedsExplicitApproval(risk string,generalWriteGrant bool) bool`；`ActionService.Prepare(context.Context,Action)(string,error)`、`Approve(context.Context,id,actor,digest string) error`、`Execute(context.Context,id string) error`。

**行为与边界：** 测试digest改变、权限撤销、并发授权次数、重启保留pending、费用批准不等于写批准；类别明确的特殊发送预授权作为独立scope匹配，不复用generalWriteGrant。

**验收映射：** B12/B13/B14；CON-05/06；AC-17/18

- [ ] **Step 1: 在 `internal/appconnector/action_test.go` 写入以下失败断言。**

```go
package appconnector
import "testing"
func TestGeneralWriteGrantDoesNotAuthorizeSending(t *testing.T) {
    if !NeedsExplicitApproval("send",true) { t.Fatal("send bypassed") }
    if NeedsExplicitApproval("write",true) { t.Fatal("bounded write grant ignored") }
    if !NeedsExplicitApproval("delete",true) { t.Fatal("delete bypassed") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector ./internal/agent/approval -run "Test(GeneralWrite|Action|Approval)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func NeedsExplicitApproval(risk string,generalWriteGrant bool) bool {
 switch risk {
 case "read": return false
 case "write": return !generalWriteGrant
 default: return true
 }
}
```

ActionDigest 对绑定身份、连接权限版本、App版本、目标和规范化参数做 SHA-256；JSON Decoder.UseNumber 后 Marshal，保存同一规范化 bytes并实际发送，不审批后再让模型重写。action表保存 snapshot/digest/state/provider_key/provider_result/fence，approval表绑定digest/actor/expiry/remaining次数；预授权表另含允许的风险类别、连接、目标范围、期限、预算上限。

Prepare→awaiting_approval/authorized→queued→dispatched→succeeded/failed/unknown，每次执行重新检查A02和U05。消耗审批次数与写入派发intent同事务；崩溃后unknown走提供方查询，不重排普通队列。现有Gate只负责展示／唤醒，ModifiedArgs改变时创建新快照并批准新digest，不能批准旧参数后执行新参数。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector ./internal/agent/approval -run "Test(GeneralWrite|Action|Approval)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/action.go internal/appconnector/action_test.go internal/application/repository/appconnector/action.go internal/application/service/appconnector/action.go migrations/versioned/000118_app_actions.up.sql migrations/versioned/000118_app_actions.down.sql migrations/sqlite/000038_app_actions.up.sql migrations/sqlite/000038_app_actions.down.sql internal/agent/approval/gate.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: persist exact connector approvals and dispatch intent"
```

### Task A04: MCP和受控HTTP统一执行适配

**Files:**
- Create: `internal/appconnector/adapter.go`、`internal/appconnector/http_policy.go`、`internal/appconnector/http_policy_test.go`
- Create: `internal/appconnector/mcp_adapter.go`
- Modify: `internal/agent/approval/tool_policy.go`

**Interfaces:**
- Consumes: A03 Action与执行状态；A02CredentialResolver；U05ExecutionGate；现有MCP工具策略与HTTP出站策略。
- Produces: `ActionResult{State,ExternalID string; Output json.RawMessage}`；`Adapter.Execute(context.Context,Action)(ActionResult,error)`、`Query(context.Context,Action)(ActionResult,error)`；`PublicAddress(net.IP) bool`。

**行为与边界：** 反例覆盖重定向、DNS rebinding、IPv6、认证头注入、未列主机、工具schema改动和MCP只读伪造；公网IP本身不等于已授权。

**验收映射：** B11；CON-07/08；AC-19

- [ ] **Step 1: 在 `internal/appconnector/http_policy_test.go` 写入以下失败断言。**

```go
package appconnector
import("testing";"net")
func TestHTTPRejectsInternalDestinations(t *testing.T) {
    for _,s:=range []string{"127.0.0.1","10.0.0.1","169.254.169.254","::1"} {
        if PublicAddress(net.ParseIP(s)) { t.Fatal(s) }
    }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector ./internal/agent/approval -run "Test(HTTP|MCP|Action)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func PublicAddress(ip net.IP) bool {
 return ip!=nil && !ip.IsLoopback() && !ip.IsPrivate() &&
   !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() &&
   !ip.IsUnspecified() && !ip.IsMulticast()
}
```

公共地址检查之外，HTTP必须匹配管理员审核的scheme/host/port/method和目标资源规则；DNS解析的每个地址都校验，DialContext实际连接已校验IP并保留TLS服务器名；每次redirect重验，认证头不跨host传递。对企业确需内网连接使用单独明确的管理员网络范围授权，不能由模型参数自动解禁。

MCP工具列表/输入schema/风险版本固定；外部readOnly提示只供参考，风险变更使installation待审核。把真实Execute包在A03 intent与U05预算之间。Query不支持时返回 ActionResult{State:"unknown"}，不能假装失败从而重发。stdio执行只在受控隔离中，拒绝由远端描述生成宿主命令。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector ./internal/agent/approval -run "Test(HTTP|MCP|Action)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/adapter.go internal/appconnector/http_policy.go internal/appconnector/http_policy_test.go internal/appconnector/mcp_adapter.go internal/agent/approval/tool_policy.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: constrain connector network and MCP execution"
```

### Task A05: 飞书发送消息的审批和结果核对

**Files:**
- Create: `internal/appconnector/feishu_send.go`、`internal/appconnector/feishu_send_test.go`

**Interfaces:**
- Consumes: A04 Adapter/ActionResult；A03 Action；现有 oapi-sdk-go/v3 v3.9.7；FS-01验证的消息API。
- Produces: `FeishuSendAdapter` 实现 Adapter；`CanRetryProviderWrite(hasGuaranteedKey bool,withinWindow bool) bool`。

**行为与边界：** httptest测错目标、审批后撤销、响应丢失、窗口过期；FS-04使用用户明确指定测试会话与内容做真实发送，缺授权保持blocked-env。

**验收映射：** FS-01 至 FS-04；AC-17；B25

- [ ] **Step 1: 在 `internal/appconnector/feishu_send_test.go` 写入以下失败断言。**

```go
package appconnector
import "testing"
func TestFeishuTimeoutOutsideDedupWindowDoesNotRetry(t *testing.T) {
    if CanRetryProviderWrite(true,false) { t.Fatal("expired dedup key retried") }
    if CanRetryProviderWrite(false,true) { t.Fatal("no provider guarantee") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector -run TestFeishu -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanRetryProviderWrite(hasGuaranteedKey,withinWindow bool) bool {
 return hasGuaranteedKey && withinWindow
}
```

先执行 FS-01 固定令牌类别、会话范围、消息类型、请求路径与权限；从A03快照构建实际请求，目标和内容逐字段相等。成功保存真实消息ID。只有官方验证的幂等字段和时间窗口均有效时可重试原key，否则保持unknown并展示核对入口；不可生成新UUID重发未知消息。现有SDK可复用，但必须以已验证方法为契约，不能只凭SDK编译宣布外部成功。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector -run TestFeishu -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/feishu_send.go internal/appconnector/feishu_send_test.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: send approved Feishu messages with recoverable results"
```

### Task A06: Notion页面创建与部分完成恢复

**Files:**
- Create: `internal/appconnector/notion_create.go`、`internal/appconnector/notion_create_test.go`

**Interfaces:**
- Consumes: A04Adapter/ActionResult；A03Action；NO-01固定的Notion-Version与页面接口。
- Produces: `NotionCreateAdapter` 实现 Adapter；`NextNotionStep(pageID string,contentDone bool) string`。

**行为与边界：** 测试父页面越界、授权撤销、创建成功后本地崩溃、追加部分成功、未知创建；真实NO-04仅对指定测试父页面写入。

**验收映射：** NO-01 至 NO-04；AC-18；B25

- [ ] **Step 1: 在 `internal/appconnector/notion_create_test.go` 写入以下失败断言。**

```go
package appconnector
import "testing"
func TestNotionPartialSuccessDoesNotCreateAnotherPage(t *testing.T) {
    if NextNotionStep("page1",false)!="append_remaining" { t.Fatal("would duplicate page") }
    if NextNotionStep("page1",true)!="complete" { t.Fatal("completion lost") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector -run TestNotion -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func NextNotionStep(pageID string,contentDone bool) string {
 if pageID=="" { return "create" }
 if !contentDone { return "append_remaining" }
 return "complete"
}
```

NextNotionStep仅适用于已确认状态；unknown_create不得调用该函数，先查询／人工核对。创建请求固定父页面、标题、内容和Notion-Version；连接需目标insert权限，父页面必须在审批／预授权范围。多步写入把page_id及已完成内容块区间持久化；已创建页面后追加失败不重建。若API无法对未知创建可靠查询，不把title搜索结果当唯一证明。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector -run TestNotion -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/notion_create.go internal/appconnector/notion_create_test.go docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: resume approved Notion page creation without duplicates"
```

### Task A07: 飞书和Notion同步迁移、游标与暂停

**Files:**
- Create: `internal/appconnector/sync.go`、`internal/appconnector/sync_test.go`
- Modify: `internal/application/service/datasource_service.go`、`internal/datasource/scheduler.go`
- Modify: `internal/application/repository/datasource_repo.go`
- Create: `internal/application/service/appconnector/sync_test.go`
- Create: `migrations/versioned/000119_app_datasource_bindings.up.sql`、`migrations/versioned/000119_app_datasource_bindings.down.sql`
- Create: `migrations/sqlite/000039_app_datasource_bindings.up.sql`、`migrations/sqlite/000039_app_datasource_bindings.down.sql`

**Interfaces:**
- Consumes: A01/A02空间连接；U05ExecutionGate；现有 datasource.Connector / StreamingConnector / StreamHandler.Checkpoint。
- Produces: `CanAdvanceCheckpoint(contentCommitted bool,currentFence,workerFence int64) bool`；`SyncBinding{TenantID uint64; InstallationID,ConnectionID,DataSourceID string; AuthVersion int64}`；`ResolveSyncBinding(context.Context,uint64,string)(SyncBinding,error)`。

**行为与边界：** 飞书和Notion各自全量/增量/断点恢复；重复运行不重复建条目；源失败不误删；套餐到期停新同步并允许已持久化结算；执行写入与调度使用空间连接。

**验收映射：** SYNC-01 至 SYNC-05；B24；AC-16

- [ ] **Step 1: 在 `internal/appconnector/sync_test.go` 写入以下失败断言。**

```go
package appconnector
import "testing"
func TestSyncCheckpointWaitsForDurableContentAndFence(t *testing.T) {
    if CanAdvanceCheckpoint(false,2,2) { t.Fatal("lost uncommitted content") }
    if CanAdvanceCheckpoint(true,2,1) { t.Fatal("stale worker advanced cursor") }
    if !CanAdvanceCheckpoint(true,2,2) { t.Fatal("valid checkpoint refused") }
}
```

- [ ] **Step 2: 执行定向测试，确认因待实现行为失败。**

```sh
go test ./internal/appconnector ./internal/application/service ./internal/datasource -run "Test(Sync|DataSource)" -count=1
```

预期：新增接口未实现或具体业务断言失败；依赖安装、数据库不可达或语法错误不算有效 RED。若代码已由并行任务实现，核对其行为后补充缺失回归用例，不能为制造失败删除他人代码。

- [ ] **Step 3: 按下面接口与算法实现本任务文件。**

```go
func CanAdvanceCheckpoint(contentCommitted bool,currentFence,workerFence int64) bool {
 return contentCommitted && currentFence==workerFence
}
```

保持现有 Connector 协议不变，服务层在调度前解析空间连接与当前预算，实例游标绑定版本和租约fence。内容与索引任务持久化交接后才能推进 checkpoint；同源ID/版本使用现有知识映射去重。暂停原因分别为budget/plan/permission/quota/provider_limit，恢复沿用游标。

数据源迁移映射放 A01 的关联表 `app_datasource_bindings(tenant_id,datasource_id,installation_id,connection_id,auth_version)`；该表迁移应由本任务独立新增 PG000119/SQLite000039 up/down，不能改已应用A01迁移。旧凭据无法证明个人/空间归属标记requires_reauthorization。其他数据源继续旧路径但不能绕过已启用空间的预算；源端404/权限拒绝/删除分开，默认保留本地副本。

- [ ] **Step 4: 加入本任务“行为与边界”列明的反例，并执行同一定向命令。**

```sh
go test ./internal/appconnector ./internal/application/service ./internal/datasource -run "Test(Sync|DataSource)" -count=1
```

预期：所有本任务断言通过；外部真实测试缺少环境时报告 blocked-env，不能将 skip 作为 pass。

- [ ] **Step 5: 校验本任务文件、更新执行台账中的证据与差异。**

```sh
git diff --check
```

在 `docs/superpowers/plans/saas-billing-connectors-progress.md` 记录任务 ID、实现提交、测试命令与结果、对应接口检查状态。不得把尚未运行的检查改为 pass。

- [ ] **Step 6: 只暂存上述本任务文件与台账，并提交独立变更。**

```sh
git add internal/appconnector/sync.go internal/appconnector/sync_test.go internal/application/service/datasource_service.go internal/datasource/scheduler.go internal/application/repository/datasource_repo.go internal/application/service/appconnector/sync_test.go migrations/versioned/000119_app_datasource_bindings.up.sql migrations/versioned/000119_app_datasource_bindings.down.sql migrations/sqlite/000039_app_datasource_bindings.up.sql migrations/sqlite/000039_app_datasource_bindings.down.sql docs/superpowers/plans/saas-billing-connectors-progress.md
git commit -m "feat: migrate team sync to scoped connector execution"
```

