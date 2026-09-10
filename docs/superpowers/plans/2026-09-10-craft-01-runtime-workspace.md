# Craft 01 执行与工作区 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 主 Runtime 能向持久化工作区内的 OpenCode 委派任务并获得可核对的结果。

**Architecture:** 复用 tRPC 的工具持久化与 fencing；新增 OpenCode 协议适配和工作区映射。Go 不替代 OpenCode 内部编码循环，也不改变 builtin 会话。

**Tech Stack:** Go/Gin/GORM、现有 Sandbox/对象存储、tRPC-Agent-Go/OpenCode、React/assistant-ui、TypeScript/pnpm；具体依赖沿专项锁定版本。

**Spec:** [产品方案](../specs/2026-09-10-onyx-craft-product-proposal.md)；必读 [总计划及共享契约](2026-09-10-craft-product-implementation.md)。

## Global Constraints

- “首版同一工作区串行修改。”
- “主 Run、ToolCall 和恢复调度复用现有 tRPC 恢复方案。”
- “关闭或刷新页面只影响订阅，不自动重新提交任务。”
- “历史版本引用保持原始内容，不能静默重定向到最新文件。”
- “基础权限与取消安全是每个可运行版本的前提”。
- “本方案不设定套餐、价格或充值规则，商业权威保持在现有专项方案。”
- 本文所有步骤为待执行计划。继承总计划全部门禁、错误类型、身份/版本约束，不把测试替身结果当成上线证据。

---

## 文件结构与执行约定

文件所有权在各任务 Files 中列出；接口定义只在对应责任模块维护。按任务依赖执行，修改共享入口前核对其他任务变更。Go 示例的测试文件使用被测包及 `testing`，生产代码按代码块使用的标准库导入；外部 craft 类型从 `github.com/Tencent/WeKnora/internal/craft` 导入。TypeScript 测试使用 `node:test` 和 `node:assert/strict`；UI 浏览器测试另用 Playwright。最小代码展示核心规则，后续集成动作和验收表同属必做内容，不能只实现纯函数就勾选整个任务。

### Task R01: 固定 OpenCode 协议与 HTTP 客户端

**依赖与用户结果：** 无依赖；形成可运行的协议兼容测试，先证明固定二进制行为。

**Files:**
- Create: `internal/agent/opencode/client.go`
- Create: `internal/agent/opencode/protocol.go`
- Test: `internal/agent/opencode/client_test.go`
- Create: `internal/agent/opencode/testdata/protocol-lock.json`
- Create: `docker/craft/opencode.lock.json`

**Interfaces:**
- Consumes：标准库 `*http.Client`；wanwu 参考实现只读，不导入 Eino。
- Produces：`NewClient(baseURL string, h *http.Client) (*Client,error)`；`Client.CreateSession(ctx context.Context) (string,error)`；`Client.Prompt(ctx context.Context,sessionID,messageID,prompt string) error`；`Client.Events(ctx context.Context) (io.ReadCloser,error)`；`Client.Messages(ctx context.Context,sessionID string) ([]Message,error)`；`Client.Abort(ctx context.Context,sessionID string) error`；`Client.Status(ctx context.Context,sessionID string) (string,error)`（idle/busy/retry）；`Client.ReplyQuestion(ctx context.Context,requestID string,answers [][]string) error`；`Client.RejectQuestion(ctx context.Context,requestID string) error`；`Client.ReplyPermission(ctx context.Context,sessionID,requestID,reply string) error`（once/reject）。
- `Message { ID, ParentID, Role, Finish string; CompletedAt int64; Parts []json.RawMessage }`；`DecodeParts([]byte)([]json.RawMessage,error)`；`Client` 字段 `base *url.URL; http *http.Client`。鉴权由服务端 transport 注入，不能成为工具参数。Status从锁定版session状态查询中取对应session；question/permission的路径、响应码与请求body从同一锁定版/doc导出并保存fixture，不跨版本拼接。R06/C02只能通过这些client方法回复。

- [ ] **Step 1：先写失败测试。**

```go
func TestDecodePartsRejectsInvalidShape(t *testing.T) {
    for _, raw := range []string{`null`, `[null]`, `{}`} {
        if _, err := DecodeParts([]byte(raw)); err == nil { t.Fatalf("accepted %s", raw) }
    }
    if _, err := DecodeParts([]byte(`[{"type":"text","text":"ok"}]`)); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/agent/opencode -run TestDecodeParts -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func DecodeParts(raw []byte) ([]json.RawMessage, error) {
    var parts []json.RawMessage
    if err := json.Unmarshal(raw, &parts); err != nil { return nil, err }
    if parts == nil { return nil, errors.New("parts must be an array") }
    for _, p := range parts {
        var object map[string]json.RawMessage
        if json.Unmarshal(p, &object) != nil || object == nil { return nil, errors.New("part must be object") }
    }
    return parts, nil
}
```

- [ ] **Step 4：** 把探测基线 OpenCode `1.18.4`、源码 `49c69c5ed3ccf706b61b3febb43c8aaff7f8325e`、已探测本机二进制 SHA256 `9449af91f517eacc2b0742fa93ae0da64fa6e5db7b714e30c62edea2a8de3f98` 记录到 lock，注明 OS/arch；容器二进制必须另测摘要。镜像按 digest 固定，健康接口 runtime version 与摘要一起校验，`/doc info.version=1.0.0` 不能代替运行版本。

- [ ] **Step 5：** 用 `httptest.NewServer` 分别注册 POST `/session`、POST `/session/{id}/prompt_async`、GET `/event`、GET `/session/{id}/message`、POST `/session/{id}/abort`；断言 prompt 恰为204才算已受理，其余响应读取有界错误。事件采用 `{id,type,properties}` 扁平结构；路径段编码，不把未经确认的 global envelope 强套到 `/event`。

- [ ] **Step 6：** 为 POST session/abort 的实际返回体、message+parts、SSE 多行 data/heartbeat/超长帧分别保存脱敏 fixture；HTTP body 上限8MiB、单事件1MiB、普通请求30s，长流用独立 context；每条错误路径关闭 body。禁用跨 host redirect，服务端绑定解析后才能构造 client。

- [ ] **Step 7：** 用锁定二进制和本机受控模型 fixture 验证 API，消息 ID 使用该版本官方 ID 算法生成，复制算法时记录来源/许可及排序规则；验证连续两轮，不接受只有 `msg_` 前缀的随机 ID。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/agent/opencode -run TestDecodeParts -count=1
```

预期 PASS。还必须逐项确认：

- 204/400/500、断连、超大事件、非法 parts 都有协议测试。
- 固定版本实际运行通过才开放 R04；不下载浮动 latest 或声称支持所有 OpenCode 版本。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/agent/opencode/client.go internal/agent/opencode/protocol.go internal/agent/opencode/client_test.go internal/agent/opencode/testdata/protocol-lock.json docker/craft/opencode.lock.json
git diff --cached --check
git commit -m "feat(craft): r01 固定 OpenCode 协议与 HTTP 客户端"
```

### Task R02: 工作区与子执行持久化

**依赖与用户结果：** 依赖 G1 契约；用户同一作品两轮复用工作区，竞争创建只产生一个绑定。

**Files:**
- Create: `internal/craft/contracts.go`
- Create: `internal/craft/scope.go`
- Test: `internal/craft/scope_test.go`
- Create: `internal/application/repository/craft_workspace.go`
- Test: `internal/application/repository/craft_workspace_test.go`
- Create: `migrations/versioned/000094_craft.up.sql`
- Create: `migrations/versioned/000094_craft.down.sql`
- Create: `migrations/sqlite/000015_craft.up.sql`
- Create: `migrations/sqlite/000015_craft.down.sql`

**Interfaces:**
- Consumes：总计划 `Scope/Workspace/Task/Store` 全部定义及 `runtime.Fence`。
- Produces：`SameScope(a,b Scope) bool`；`NewCraftStore(db *gorm.DB) craft.Store`，在 repository 定义私有实体。错误与共享结构按总计划创建。

- [ ] **Step 1：先写失败测试。**

```go
func TestSameScopeIncludesUserAndTenant(t *testing.T) {
    a := Scope{TenantID:1,UserID:"u1",SessionID:"s1"}
    b := a; b.TenantID = 2
    if SameScope(a,b) { t.Fatal("cross tenant accepted") }
    b = a; b.UserID = "u2"
    if SameScope(a,b) { t.Fatal("cross owner accepted") }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestSameScope -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func SameScope(a, b Scope) bool {
    return a.TenantID != 0 && a.UserID != "" && a.SessionID != "" &&
        a.TenantID == b.TenantID && a.UserID == b.UserID && a.SessionID == b.SessionID
}
```

- [ ] **Step 4：** 按总计划创建契约文件。`SameScope` 用于执行绑定身份相等，不替代共享会话 read ACL；共享查看仍交给现有权限服务。

- [ ] **Step 5：** 迁移建立 craft_workspaces：id主键、tenant_id/session_id唯一、owner_id、sandbox_id/generation/oc_session_id/runtime_digest/revision、created_at/updated_at；craft_delegations：id主键、tenant_id/run_id/tool_call_id唯一、workspace_id、prompt_message_id、request_hash、task_json、status、result_json、revision、timestamps。引用已有 sessions/runs/tool_calls，删除遵从会话保留策略。

- [ ] **Step 6：** `PutWorkspace` 用 WHERE revision=expected 的更新，首次 expected=0 用唯一插入；`PrepareTask` 在真实 Run 锁下验证权限与活跃 ToolCall，将 promptID/摘要写入后返回，同调用异参 ErrConflict；`SaveResult` 同事务校验 live fence 并且重复相同结果返回原结果，不同结果拒绝。不得用 AutoMigrate 替代迁移测试。

- [ ] **Step 7：** 在 repository 测试文件创建 `openCraftDB(t *testing.T) *gorm.DB`，执行真实 SQLite 迁移，seed 满足 tRPC 外键的 tenant/user/session/run/tool_call；并发2次 PutWorkspace 仅一个创建成功、重开DB GetWorkspace仍存在、过期epoch SaveResult失败、重复PrepareTask返回相同promptID。PostgreSQL用隔离schema执行同一断言；down 只清理Craft表。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestSameScope -count=1
```

预期 PASS。还必须逐项确认：

- 运行 `go test ./internal/application/repository -run TestCraft -count=1`；两个数据库结果分列。
- 不创建竞争的主 Run 表；基础设施上旧会话查询仍正常。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/contracts.go internal/craft/scope.go internal/craft/scope_test.go internal/application/repository/craft_workspace.go internal/application/repository/craft_workspace_test.go migrations/versioned/000094_craft.up.sql migrations/versioned/000094_craft.down.sql migrations/sqlite/000015_craft.up.sql migrations/sqlite/000015_craft.down.sql
git diff --cached --check
git commit -m "feat(craft): r02 工作区与子执行持久化"
```

### Task R03: 授权材料与可复现 Sandbox 配置

**依赖与用户结果：** 依赖 R02；上传文件以受控只读材料进入工作区，不把账号密钥传入模型。

**Files:**
- Create: `internal/craft/input.go`
- Test: `internal/craft/input_test.go`
- Create: `internal/application/service/craft_inputs.go`
- Create: `internal/application/service/craft_workspace.go`
- Test: `internal/application/service/craft_workspace_test.go`
- Test: `internal/application/service/craft_inputs_test.go`
- Create: `docker/craft/Dockerfile`
- Create: `docker/craft/runtime-config.json`

**Interfaces:**
- Consumes：`craft.Scope/Input/Workspace`、现有资源读取权限与 SessionSandboxBinding。
- Produces：`ValidateInputName(name string) error`；`CraftInputService.Stage(ctx context.Context,scope craft.Scope,workspace craft.Workspace,inputs []craft.Input) ([]craft.Input,error)`；构造器 `NewCraftInputService(load func(context.Context,craft.Scope,string)([]byte,error), write func(context.Context,craft.Workspace,string,[]byte)error) *CraftInputService`。load 必须是授权资源读取，write 必须是绑定工作区文件写入。
- `CraftWorkspaceService.Resolve(ctx context.Context,scope craft.Scope) (craft.Workspace,error)`：持生命周期锁读取现有Sandbox绑定和Craft工作区；首次创建OC session并持久化，已有绑定验证generation/digest后重连，不每轮CreateSession。

- [ ] **Step 1：先写失败测试。**

```go
func TestInputPathCannotEscape(t *testing.T) {
    for _, p := range []string{"../secret", "/etc/passwd", `a\b`, "", "a/b"} {
        if ValidateInputName(p) == nil { t.Fatalf("accepted %q",p) }
    }
    if err := ValidateInputName("sales.csv"); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestInputPath -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func ValidateInputName(name string) error {
    if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
        return ErrInvalidInput
    }
    return nil
}
```

- [ ] **Step 4：** 写 Stage 测试：伪授权loader遇其他tenant返回 ErrForbidden且writer调用0次；实际bytes摘要/大小与输入声明不符拒绝。限制每文件20MiB、每轮20个、总量100MiB，超限在读入前和有界读取后双重验证；路径用输入摘要加规范文件名，内容进入 `/workspace/inputs/`，引用清单保留来源。

- [ ] **Step 5：** 在新增craft_workspace.go实现Resolve：持现有生命周期锁并在数据库检查活跃Run，读取GetWorkspace；无映射时在绑定Sandbox创建OC session并CAS写Workspace，有映射直接校验复用。创建响应丢失时先按受控会话元数据核对，无法确认则ErrUnknown，不把新建空会话作为恢复。然后准备非root镜像： `/workspace/inputs`、`/workspace/output`、持久 OpenCode data/config 路径。使用R01锁定二进制；工作区不能挂宿主home或Docker socket。输入只读、output可写；封禁危险 symlink 跟随。模型配置仅允许服务端提供的gateway或已授权BYOK入口；平台长效key不能落工作区。

- [ ] **Step 6：** 生成版本化配置与技能摘要，固定shell输出/执行时限；网络允许规则由现有沙箱配置解析。所有未列明的权限采用 ask/deny，不能设置宽泛 always。R06处理 ask；G4之前使用仅内部试验的限额配置，不开放收费执行。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestInputPath -count=1
```

预期 PASS。还必须逐项确认：

- 恶意文件名、摘要篡改、跨空间引用、超额文件均在委派前失败。
- 镜像启动可读输入、可写output，退出重启持久data仍在；日志/任务JSON不含凭据。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/input.go internal/craft/input_test.go internal/application/service/craft_inputs.go internal/application/service/craft_workspace.go internal/application/service/craft_workspace_test.go internal/application/service/craft_inputs_test.go docker/craft/Dockerfile docker/craft/runtime-config.json
git diff --cached --check
git commit -m "feat(craft): r03 授权材料与可复现 Sandbox 配置"
```

### Task R04: 可核对的提交与子事件归一化

**依赖与用户结果：** 依赖 R01–R03、G1；执行中断不会被误报为已完成。

**Files:**
- Create: `internal/agent/opencode/executor.go`
- Create: `internal/agent/opencode/normalizer.go`
- Test: `internal/agent/opencode/normalizer_test.go`
- Test: `internal/agent/opencode/executor_test.go`

**Interfaces:**
- Consumes：`craft.Task/Result/Observation/Store/Executor`、R01 Client。
- Produces：`Completed(o craft.Observation) bool`；`NewExecutor(client *Client,store craft.Store,emit func(context.Context,craft.Task,string,json.RawMessage)error) craft.Executor`。emit 将 payload 写入已有 RunEvent，不直接发浏览器。

- [ ] **Step 1：先写失败测试。**

```go
func TestCompletedRequiresExactPrompt(t *testing.T) {
    o := craft.Observation{SessionID:"s",PromptMessageID:"p2",AssistantParentID:"p1",Completed:true,Idle:true,Finish:"stop"}
    if Completed(o) { t.Fatal("old turn accepted") }
    o.AssistantParentID="p2"; o.PendingTool=true
    if Completed(o) { t.Fatal("pending tool accepted") }
    o.PendingTool=false
    if !Completed(o) { t.Fatal("valid terminal rejected") }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/agent/opencode -run TestCompleted -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func Completed(o craft.Observation) bool {
    return o.SessionID != "" && o.PromptMessageID != "" &&
        o.AssistantParentID == o.PromptMessageID && o.Completed && o.Idle &&
        o.Finish == "stop" && !o.PendingTool && !o.Aborted
}
```

- [ ] **Step 4：** 首次准备promptID→PrepareTask持久化→订阅/event→向绑定session提交prompt；不能先执行再补记录。httptest记录事件订阅时间早于prompt时间。收到204只标记已受理；POST超时转Observe，不能自动POST第二次。

- [ ] **Step 5：** Observe同时读当前session状态、message列表和parts；只采用匹配PromptMessageID的assistant链，按完成时间/创建时间选择，不能依赖数组尾部。文字/工具变更绑定message/part ID去重，工具终态、question与permission分别保留；子session其他轮消息不污染当前任务。

- [ ] **Step 6：** EOF、transport错误、session.idle单独出现都触发快照核对。失败/超时返回unknown或明确failed；只有完成谓词+成功采集结果才能SaveResult succeeded。abort错误按锁定版本 `MessageAbortedError` 识别，不用名称包含cancel的启发式。事件过载优先保留状态与工具终态，文本可有界合并。

- [ ] **Step 7：** 执行器集成测试增加：204后断流且远端仍运行；prompt已接收但响应丢失；同会话旧completed消息；消息parts损坏；tool仍running；重复事件；快照乱序。每案断言POST计数与结果状态，不只测试纯谓词。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/agent/opencode -run TestCompleted -count=1
```

预期 PASS。还必须逐项确认：

- 运行 `go test ./internal/agent/opencode -count=1`；unknown路径POST恰好一次。
- 子事件不含主 done，敏感命令参数/模型凭据按既有脱敏规则处理。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/agent/opencode/executor.go internal/agent/opencode/normalizer.go internal/agent/opencode/normalizer_test.go internal/agent/opencode/executor_test.go
git diff --cached --check
git commit -m "feat(craft): r04 可核对的提交与子事件归一化"
```

### Task R05: 主 Agent 委派工具与结果回传

**依赖与用户结果：** 依赖 R04 和 G1 完整通过；主 Agent 可在第一次构建后继续决定修复或交付。

**Files:**
- Create: `internal/application/service/craft_delegate.go`
- Test: `internal/application/service/craft_delegate_test.go`
- Create: `internal/agent/tools/craft_delegate.go`
- Test: `internal/agent/tools/craft_delegate_test.go`
- Modify: `internal/application/service/agent_service.go`
- Modify: `internal/container/container.go`

**Interfaces:**
- Consumes：`craft.Executor.Execute(context.Context,craft.Task)(craft.Result,error)`，持久化 ToolExecutor 的调用身份和 Fence。
- Produces：模型参数 `DelegateArgs { Goal string; InputRefs []string }`；`ParseDelegateArgs([]byte)(DelegateArgs,error)`（tools 包）；`CraftDelegateService.Delegate(ctx context.Context,task craft.Task)(craft.Result,error)`；`NewCraftDelegateService(store craft.Store,executor craft.Executor) *CraftDelegateService`。工具通过现有 Tool 接口封装，Name固定 `craft_delegate`。

- [ ] **Step 1：先写失败测试。**

```go
func TestDelegateRejectsAuthorityArguments(t *testing.T) {
    if _,err:=ParseDelegateArgs([]byte(`{"goal":"report","tenant_id":2,"sandbox_url":"http://evil"}`)); err==nil { t.Fatal("authority injection") }
    if _,err:=ParseDelegateArgs([]byte(`{"goal":"report","input_refs":[]}`)); err!=nil { t.Fatal(err) }
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/agent/tools -run TestDelegateRejects -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type DelegateArgs struct { Goal string `json:"goal"`; InputRefs []string `json:"input_refs"` }
func ParseDelegateArgs(raw []byte) (DelegateArgs,error) {
    var a DelegateArgs
    d:=json.NewDecoder(bytes.NewReader(raw)); d.DisallowUnknownFields()
    if err:=d.Decode(&a);err!=nil{return a,err}
    var extra any
    if err:=d.Decode(&extra);err!=io.EOF{return a,errors.New("trailing input")}
    if strings.TrimSpace(a.Goal)=="" {return a,errors.New("goal required")}
    return a,nil
}
```

- [ ] **Step 4：** 在按Run创建的ToolRegistry注册工具；仅Craft+trpc会话开放，builtin不变。ToolCallID、tenant/user/session、workspace、允许模型/输入与deadline由持久化运行context装配；模型只能给目标和已授权输入引用。构造 Task 时 RequestHash 覆盖输入摘要/技能版本/目标。

- [ ] **Step 5：** Delegate先GetResult复用已存结果；planned经过PrepareTask；dispatching未知时Observe，不能直接调用Execute重发。使用恢复专项工具日志的外部引用保存workspace/oc session/prompt message ID；图apply_result只接受一次。

- [ ] **Step 6：** 返回摘要限8KiB并带resource引用/Checks，完整日志存事件或对象存储；产物需W01发布后才可成为最终文件引用。子工具成功让主图进入apply_result→model，主模型明确检查通过/失败/not_run；构建或预览失败可继续委派，达到截止时间则交代失败。

- [ ] **Step 7：** 集成测试的模型fixture按次返回“委派初稿→收到检查失败→委派修复→解释结果”；统计主模型调用至少3次、子委派2次、主终态只有最后一次。另测重复恢复不再次执行已保存ToolCall、builtin工具集不增加Craft。

- [ ] **Step 8：运行 GREEN 与验收。**

```bash
go test ./internal/agent/tools -run TestDelegateRejects -count=1
```

预期 PASS。还必须逐项确认：

- 没有把tRPC降为请求转发器；真实主图经历模型/工具/模型。
- 实际scope、提示词输入摘要、工具身份与当前Run一致；工具JSON注入被拒绝。

- [ ] **Step 9：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/application/service/craft_delegate.go internal/application/service/craft_delegate_test.go internal/agent/tools/craft_delegate.go internal/agent/tools/craft_delegate_test.go internal/application/service/agent_service.go internal/container/container.go
git diff --cached --check
git commit -m "feat(craft): r05 主 Agent 委派工具与结果回传"
```

### Task R06: 基本问题权限与停止安全

**依赖与用户结果：** 依赖 R04/R05、tRPC持久Decision/Cancel；B阶段也不能绕过权限等待。

**Files:**
- Create: `internal/craft/interaction.go`
- Test: `internal/craft/interaction_test.go`
- Create: `internal/application/service/craft_control.go`
- Test: `internal/application/service/craft_control_test.go`

**Interfaces:**
- Consumes：`craft.Executor.Abort/Observe` 与 tRPC `Resolve(ctx,key,Decision)`/`Cancel(ctx,key)`。
- Produces：`Interaction { ID, Kind, ArgsHash, Prompt string; Revision int64 }`；`DecisionAllowed(kind,action string) bool`；`StopStatus(abortRequested bool,o Observation) string`。service控制器把OpenCode等待挂到已有pending状态，详尽多选交互在C02交付。

- [ ] **Step 1：先写失败测试。**

```go
func TestControlDoesNotInventApprovalOrCancellation(t *testing.T) {
    if DecisionAllowed("question","approve") {t.Fatal("question approved")}
    if StopStatus(true,Observation{Idle:true})=="canceled" {t.Fatal("abort unverified")}
    if StopStatus(true,Observation{Aborted:true,Idle:true})!="canceled" {t.Fatal("abort lost")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/craft -run TestControl -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
func DecisionAllowed(kind,action string) bool {
    return (kind=="question" && (action=="answer" || action=="reject")) ||
        (kind=="permission" && (action=="approve" || action=="reject"))
}
func StopStatus(requested bool,o Observation) string {
    if requested && o.Aborted && o.Idle { return "canceled" }
    if requested { return "stopping" }
    return "running"
}
```

- [ ] **Step 4：** question与permission在服务端持久化PendingID、OC request ID、ToolCallID、参数摘要和revision；无UI支持的请求显示明确等待/拒绝，不后台自动同意。R06先支持单问题文本及once批准/拒绝，C02扩展复杂问题与恢复投递。

- [ ] **Step 5：** 停止入口先写主Run取消意图，阻止新dispatch，再调用Abort；202给客户端“正在停止”。Observe核对abort且idle后再确认，若已正常完成保留原结果和取消竞态说明；远端不明转等待。SSE取消context不得调用Abort。

- [ ] **Step 6：** 服务测试覆盖：审批后参数hash变化409；跨用户403；客户端断开Abort调用0；abort HTTP成功但running仍stopping；正常完成先到不覆写为canceled。授权动作必须写Decision再转发，远端投递不明保留outbox，C02负责可靠恢复。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
go test ./internal/craft -run TestControl -count=1
```

预期 PASS。还必须逐项确认：

- 在没有完整交互UI时，允许暂停与拒绝，禁止静默批准。
- 取消不承诺回滚已经写出的文件；终态与实际核对结果一致。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/craft/interaction.go internal/craft/interaction_test.go internal/application/service/craft_control.go internal/application/service/craft_control_test.go
git diff --cached --check
git commit -m "feat(craft): r06 基本问题权限与停止安全"
```

### Task R07: 两轮真实执行纵向验收

**依赖与用户结果：** 依赖 R01–R06；证明相同工作区和子session持续修改。

**Files:**
- Create: `internal/agent/opencode/live_test.go`
- Create: `internal/agent/opencode/testdata/sales.csv`
- Create: `docs/testing/craft/runtime-acceptance.md`

**Interfaces:**
- Consumes：R01 Client 与 R04 Executor，以及服务启动后的实际 G1 持久化入口。
- Produces：`LiveReport { FirstSession,SecondSession,FirstHash,SecondHash string; PromptPosts int; Checks []craft.Check }`；`ValidTwoTurn(r LiveReport) bool`（live_test.go测试内）；`TestLiveCraftTwoTurns` 仅显式 `CRAFT_LIVE=1` 运行。测试文件内实现 `runLiveTwoTurns(t *testing.T) LiveReport`，流程与证据按下列步骤，不在生产暴露故障后门。

- [ ] **Step 1：先写失败测试。**

```go
func TestLiveReportRejectsFreshSession(t *testing.T) {
    r:=LiveReport{FirstSession:"s1",SecondSession:"s2",FirstHash:"a",SecondHash:"b",PromptPosts:2}
    if ValidTwoTurn(r) {t.Fatal("new session disguised as continuation")}
    r.SecondSession="s1"
    if !ValidTwoTurn(r) {t.Fatal("valid continuation rejected")}
}
```

- [ ] **Step 2：运行 RED。**

```bash
go test ./internal/agent/opencode -run TestLiveReport -count=1
```

预期：新增接口不存在或目标行为断言失败。依赖/网络失败不算 RED；修复测试环境后再确认。

- [ ] **Step 3：实现核心规则。**

```go
type LiveReport struct { FirstSession,SecondSession,FirstHash,SecondHash string; PromptPosts int; Checks []craft.Check }
func ValidTwoTurn(r LiveReport) bool {
    return r.FirstSession!="" && r.FirstSession==r.SecondSession &&
        r.FirstHash!="" && r.SecondHash!="" && r.FirstHash!=r.SecondHash && r.PromptPosts==2
}
```

- [ ] **Step 4：** 创建sales.csv含date/region/revenue三列，2026-01-01东区100、2026-02-01西区200；启动固定镜像和受控模型测试端点，配置域名/凭据经服务端解析。第一轮“生成按月报告”，等待匹配主Run交付，记录session/工作区/promptID和文件摘要。

- [ ] **Step 5：** 第二轮“改为季度汇总”，使用相同创作session，确认OC session ID不变、文件摘要变化、引用新旧Run分别可追溯。live helper读取真实HTTP和持久化结果，不通过测试内直接写output冒充生成。

- [ ] **Step 6：** 另用实际配置模型完成一次非脚本化CSV生成，记录真实修复/检查与摘要；确定性模型仅证明协议。运行 `CRAFT_LIVE=1 go test ./internal/agent/opencode -run TestLiveCraftTwoTurns -count=1 -timeout=10m`；未配置live环境则明确未验收，禁止默认skip当作通过。

- [ ] **Step 7：运行 GREEN 与验收。**

```bash
go test ./internal/agent/opencode -run TestLiveReport -count=1
```

预期 PASS。还必须逐项确认：

- 保留受控模型协议证据与实际模型产品证据两份。
- 本阶段只有执行纵向链路，完整预览与用户浏览器交付仍依赖W06。

- [ ] **Step 8：独立提交。** 先检查暂存 diff 仅包含本任务；实现过程中新增的必要文件须明确加入同一提交清单。

```bash
git add internal/agent/opencode/live_test.go internal/agent/opencode/testdata/sales.csv docs/testing/craft/runtime-acceptance.md
git diff --cached --check
git commit -m "feat(craft): r07 两轮真实执行纵向验收"
```
