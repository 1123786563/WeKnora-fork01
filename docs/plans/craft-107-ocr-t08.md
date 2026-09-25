# T08 OCR 报告（第 1 次运行）

范围说明：ask 给定线性范围 13c656301^..1d7a21d7b 经 git 核实含 38 个提交（其中 36 个为 T19/T05/T03/T02 等已各自完成 OCR 且入账的提交，1489 变更文件/458 待审/31.6 万行插入，不可行且违背任务级增量意图）。按 ask"以实际核实为准"，实际审查 T08 的 2 个提交：段一 --commit 13c656301（现场收编，9 文件审查），段二 --from 25caebd13 --to 1d7a21d7b（接线核验，1 文件审查，0 findings）。以下为两段 ocr 原文合并。

## 段一：--commit 13c656301

Review complete: 10 finding(s) across 9 selected item(s).

─── packages/views/package.json:21-22 ───
[maintainability · low] craft 分组的 exports 原本严格按字母序排列（document → files → home → interaction → library
→ presentation → …），新增的 "access" 按序应位于 "./craft/document" 之前，当前插在 interaction 与 library
之间破坏了排序约定，影响映射表可读性，且会被 sort-package-json 等自动排序工具在后续提交中反复产生无谓 diff。

      "./craft/access": "./src/craft/access.tsx",
-     "./craft/library": "./src/craft/library.tsx",
+     "./craft/document": "./src/craft/document.tsx",


─── packages/views/src/craft/access.tsx:55-57 ───
[maintainability · medium] 新增了十余处纯静态的内联 style 对象（listStyle/flex 布局/gap/boxSizing 等），违反 React
内联样式规范（仅动态样式允许内联）。craft 包既有组件统一走 className 体系（shell.tsx 的 wk-craft-notice、wk-craft-drawer
等），本组件独自内联会导致同一设计体系出现两套样式实现、无法被主题/CSS 覆盖。建议迁移为 CSS 类（如
wk-craft-access、wk-craft-access__item）对齐既有模式；注意 access.test.tsx:68 断言了内联 overflow-wrap，需同步调整测试断言目标。

-   return <section aria-label="Task access" style={{ minWidth: 0 }}>
+   return <section aria-label="Task access" className="wk-craft-access">
      <h3>Task access</h3>
-     <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
+     <ul className="wk-craft-access__list">


─── packages/views/src/craft/access.tsx:34-36 ───
[maintainability · low] 两个问题：1) grant 与 revoke 的 pending 设置 → try/catch/finally → feedback
更新流程高度重复，后续调整反馈逻辑需双处同步，建议提取统一的 runAction 辅助函数；2) 两处 catch 均完全丢弃原始异常（可选 catch
绑定且无任何记录），线上无法区分网络故障、4xx 权限拒绝与服务端 5xx——尤其 grant 的错误文案固定为 "Check your connection"，若服务端因目标用户不存在/无权限返回
4xx 会误导排障方向。建议在提取的辅助函数中绑定 error 并预留统一日志/上报通道。

-     } catch {
+     } catch (error) {
        setFeedback({ kind: 'error', message: 'Member could not be added. Check your connection and try again.' });
+       // 保留 error 引用以便接入统一日志/上报，区分网络错误与服务端 4xx/5xx
      } finally {


─── packages/views/src/craft/access.tsx:76-76 ───
[bug · low] role="alert" 与 aria-live="polite" 语义冲突：role="alert" 隐含 aria-live="assertive"，显式设置的
polite 会覆盖该隐含值，导致错误提示被降级为排队播报、无法及时打断读屏，与选用 alert 角色的意图相悖（ARIA in HTML 亦不建议此组合）。建议按状态切换 aria-live：错误用
assertive、pending/success 维持 polite；项目内既有反馈组件（shell.tsx 的 CraftNotice、message-face.tsx）均为纯 polite
模式，可对齐。保留 aria-live 属性不会影响 access.test.tsx 基于 [aria-live] 选择器的断言。

-     {feedback && <p role={feedback.kind === 'error' ? 'alert' : 'status'} aria-live="polite" aria-atomic="true" data-state={feedback.kind}>{feedback.message}</p>}
+     {feedback && <p role={feedback.kind === 'error' ? 'alert' : 'status'} aria-live={feedback.kind === 'error' ? 'assertive' : 'polite'} aria-atomic="true" data-state={feedback.kind}>{feedback.message}</p>}


─── packages/views/src/craft/access.tsx:33-33 ───
[documentation · low] 成功文案与实际接线行为不符：消费方（apps/web/src/features/craft/routes.tsx 的
grantAccess/revokeAccess）在 onGrant/onRevoke 回调内部就会 await refreshAccess 完成成员列表的权威重取，即 Promise resolve
时列表已经更新完毕，无需用户刷新。"will update when refreshed" 会让用户误以为需要手动刷新（或以为功能未生效）。revoke 分支第 47
行的同类文案存在相同问题。建议去掉"刷新"措辞，直接以刷新后的列表呈现结果。

-       setFeedback({ kind: 'success', message: 'Member added. The access list will update when refreshed.' });
+       setFeedback({ kind: 'success', message: 'Member added.' });


─── internal/router/routes_chat.go:95-100 ───
[documentation · low] 新增的 registerSessionRoutes 被插入在原 "// RegisterSessionRoutes 注册路由。" 文档注释块与
RegisterSessionRoutes 函数体之间,导致:(1) 导出函数 RegisterSessionRoutes 失去了原有文档注释;(2) 该注释块现挂在
registerSessionRoutes 上,首行名称与函数不匹配,且与新追加的两行说明混在一起,godoc 与阅读均有误导性。建议把包装函数整体移到原注释块之前,或拆分为两段独立注释。

  // registerSessionRoutes is the container/router seam. Keeping RouterParams in
  // this call ensures each NewRouter mounts only the Craft feature registry
  // provided by the same dig assembly.
  func registerSessionRoutes(r *gin.RouterGroup, params RouterParams, g *rbacGuards) {
  	RegisterSessionRoutes(r, params.SessionHandler, params.MessageSuggestionHandler, g, params.CraftFeatureRoutes)
  }
+ 
+ // RegisterSessionRoutes 注册路由。
+ //
+ // (原有注释块整体下移回 RegisterSessionRoutes 之上)


─── internal/container/craft_access_wiring.go:13-15 ───
[test · high] craftTaskAccessChecker 的返回类型从 craft.TaskAccessChecker 改为 craft.TaskRunAccess 后，dig
容器（go.uber.org/dig v1.19.0）注册的提供类型也随函数签名变为 craft.TaskRunAccess。dig 严格按 reflect.Type
精确匹配依赖，不做隐式接口转换，因此容器中不再存在 craft.TaskAccessChecker 类型。这会破坏 craft_access_wiring_test.go:132 中
assembleCraftAccessTestFeature 的 container.Invoke(func(svc *service.CraftAccessService, checker
craft.TaskAccessChecker, ...) —— dig 会因 "missing type: craft.TaskAccessChecker" 解析失败，require.NoError
随之失败，TestCraftAccessFeatureRegistriesAreAssemblyOwned 无法通过。生产侧（container.go:498 Provide +
NewSessionService 请求 craft.TaskRunAccess）不受影响。建议：更新该测试的 Invoke 参数为
craft.TaskRunAccess；若仍需向未来的消费者暴露窄接口，在 container.go 中额外注册一层适配（如 must(container.Provide(func(r
craft.TaskRunAccess) craft.TaskAccessChecker { return r }))）。

  func craftTaskAccessChecker(svc *service.CraftAccessService) craft.TaskRunAccess {
  	return svc
+ }
+ 
+ // craftTaskAccessNarrowPort re-exposes the same instance through the
+ // legacy narrow interface type for dig consumers that request
+ // craft.TaskAccessChecker (dig matches types exactly, no implicit
+ // interface satisfaction).
+ func craftTaskAccessNarrowPort(access craft.TaskRunAccess) craft.TaskAccessChecker {
+ 	return access
  }


─── internal/application/service/session.go:295-298 ───
[bug · medium] IsCraftTask 的任何错误（包括数据库瞬时故障）都被统一压平为
apperrors.ErrSessionNotFound，handler（internal/handler/session/handler.go:318-324）会将其映射为 404 并仅记 Warn
日志。对比原读取路径 loadSessionForRead（session.go:88-90）：非 NotFound 的数据库错误会原样传播并由 handler 映射为 500。由于生产装配中
craftTaskAccess 恒非 nil，数据库抖动期间所有会话详情读取（包括非 Craft 会话——分类查询先于一切执行且失败）都会以 404 呈现，从客户端到日志层面都掩盖了基础设施故障。这与
IsCraftTask 注释中 "Database errors propagate so callers can fail closed"
的设计意图相悖——传播的目的正是让调用方区分对待，而此处全部吞掉。下方 recheck 处的 recheckErr 同理。建议仅将分类语义错误（craft.ErrNotFound /
craft.ErrForbidden，即会话不存在/租户不匹配/参数无效）映射为 404，其余数据库错误原样返回，保持与 loadSessionForRead 一致的错误语义。

  		registeredCraft, classifyErr := s.craftTaskAccess.IsCraftTask(ctx, tenantID, id)
  		if classifyErr != nil {
+ 			if stderrors.Is(classifyErr, craft.ErrNotFound) || stderrors.Is(classifyErr, craft.ErrForbidden) {
- 			return nil, apperrors.ErrSessionNotFound
+ 				return nil, apperrors.ErrSessionNotFound
+ 			}
+ 			return nil, classifyErr
  		}


─── internal/application/service/craft_access.go:144-145 ───
[bug · medium] 拒绝审计行的 ActorUserID 会在 PG 上溢出导致写入静默失败：audit_logs.actor_user_id 在迁移 000044 中为
VARCHAR(36) NOT NULL，而 scope.UserID 来自 SessionOwnerIDFromContext，对 API-key/external-user 主体是合成
ID（"api_external_user:<tenant>:<extid>"，extid 最长 128 字符，见 internal/middleware/auth.go 的
maxExternalUserIDLen/resolveAPIPrincipal，总长可超 140 字符）。这类主体经 sessions 组的 API-key 守卫即可触达
CheckTaskAccess（knownTask 为 true 即落审计），PG 对超长 varchar 的 INSERT 返回 22001
错误，审计行被丢弃（仅剩错误日志），且函数刻意忽略插入错误，拒绝证据不可恢复。建议将 actor 标识截断/摘要到 36 字符内（完整 ID 放入 Details），或通过迁移加宽该列。

+ 	actor := scope.UserID
+ 	if len(actor) > 36 {
+ 		// audit_logs.actor_user_id is VARCHAR(36); keep the full identity
+ 		// inside Details so the durable denial row survives synthetic principals.
+ 		actor = actor[:36]
+ 	}
  	err := s.db.WithContext(ctx).Create(&craftAccessAudit{
- 		TenantID: scope.TenantID, ActorUserID: scope.UserID, Action: "craft.access_denied",
+ 		TenantID: scope.TenantID, ActorUserID: actor, Action: "craft.access_denied",


─── internal/application/service/craft_access.go:147-148 ───
[performance · medium]
每次被拒请求都无条件写入一条持久审计行，没有任何去重/限流。代码库对同类拒绝审计已有明确先例：auditLogService.LogDenied（internal/application/servic
e/audit_log.go）实现了 1 分钟滑动窗口去积（denyDedupWindow + CountSinceForDedup），注释明确说明是为了"防止探测客户端灌满 audit_logs
表"。本路径与该场景完全同构：任何非授权主体反复请求同一个已知 Craft 任务（GET /sessions/:id、preview、run 准入等缝隙都会走到 CheckTaskAccess
的拒绝分支），即可按请求速率无界增长 audit_logs 及其索引。建议复用同等粒度的滑动窗口去重（按 tenant_id+actor+scope_id+action），或至少在 Details
中记录请求路径并对高频重复拒绝限流。



LLM retry report summary: 1 of 89 requests affected -- 1 request recovered after retry

Core review (1 request):
- internal/router/router.go,internal/router/routes_chat.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).

## 段二：--from 25caebd13 --to 1d7a21d7b

Review complete: 0 finding(s) across 1 selected item(s).
