Review complete: 7 finding(s) across 6 selected item(s).

─── internal/application/repository/craft_stop_intent.go:63-65 ───
[bug · high] PutStopIntent 的「CONFIRMED 永不降级」守卫存在 check-then-act 竞态：事务内的 Take 是无锁快照读，而 OnConflict 的
DoUpdates 无条件覆盖 status。调用方 CraftControlService.StopRun 是按 HTTP 请求触发的（craft_control.go:578 requested
写入与 :659/:737/:748 confirmed 回填/标记），服务层没有按 run 的串行化，同一 run 的重放可与确认写入并发到达。Postgres READ COMMITTED
下时序为：重放事务 Take 读到 requested/不存在 → 确认事务提交 status=confirmed → 重放事务的 INSERT ... ON CONFLICT DO UPDATE
仍以 status=requested 覆盖，已确认事实被降级。这破坏了控制服务幂等重放所依赖的固定存储契约：之后 GetStopIntent 不再答 confirmed，轮询将在终态
canceled 的 run 行旁永久投影 Outcome=requested（正是 craft_control.go:730-740
注释描述并专门修复的不一致）。仓库内同模式读-改-写一律使用行锁（craft_draft_head.go:344、craft_run_capture.go:285、tenant_member.go:2
5 的 forUpdateClause 辅助），本处应同样在事务内对 Take 加 FOR UPDATE，使降级守卫基于当前已提交行做出判定；亦可另行将 DoUpdates 的 status
改为条件赋值（status=CASE WHEN 已是 confirmed THEN 保持 ELSE EXCLUDED.status END）作为无锁加固。

  		var current craftStopIntentRow
- 		err := tx.Where("tenant_id = ? AND session_id = ? AND run_id = ?",
+ 		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND session_id = ? AND run_id = ?",
  			scope.TenantID, scope.SessionID, intent.RunID).Take(&current).Error


─── apps/web/src/features/craft/routes.tsx:898-899 ───
[bug · medium] 编辑运行被接纳后仅刷新了 workbenchInfo 投影，未重载控制器：另外两条改变 run 状态的路径（enrichedSend 在提交后
L811、requestBudgetExtension 在扩额后 L550）都会调用 controller.load(sessionId)——它重读权威快照并对非终态 active run 启动
SSE 订阅（controller.ts load()）。跳过它意味着协作者刚被接纳的编辑运行虽在服务端执行，但工作台的对话/进度区不会订阅其事件流，停留旧状态直到用户手动导航/刷新。建议与
refreshWorkbench 一并 best-effort 重载控制器（注意需把 controller 加入本 useMemo 依赖数组，参照 enrichedSend 的写法）。

-             void refreshWorkbench(sessionId).catch(() => {});
+             void (async () => {
+               try {
+                 await refreshWorkbench(sessionId);
+                 await controller.load(sessionId);
+               } catch { /* best-effort — the panel already answered from the submit envelope */ }
+             })();
              return raw;


─── packages/api-client/src/craft/index.ts:248-261 ───
[maintainability · medium] submitEdit 与既有 submit 的路径拼接和 5 字段请求体构造完全重复，仅返回值投影不同（parseCraftRunView vs
原始 unwrap）。一旦 POST /craft/runs 契约变更（新增必填字段、调整路径），需要在两处同步修改，存在漂移风险——submit
漏改会造成同一端点两种请求体。建议抽取共享的请求构造辅助（如模块级 postRunAdmission(sessionId, input, signal)），submit 与 submitEdit
均委托调用，各自只保留响应投影差异。

      async submitEdit(sessionId: string, input: CraftSubmitRunInput, signal?: AbortSignal): Promise<unknown> {
-       return unwrap(await request({
-         method: 'POST',
-         path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs',
-         body: {
-           request_id: input.request_id,
-           prompt: input.prompt,
-           input_refs: input.input_refs ?? [],
-           knowledge_scope: input.knowledge_scope ?? '',
-           base_version_id: input.base_version_id ?? '',
-         },
-         signal,
-       }), 'run admission');
+       return unwrap(await postRunAdmission(sessionId, input, signal));
      },
+     // 模块级共享辅助，submit 的 request 调用同样委托：
+     // async function postRunAdmission(sessionId: string, input: CraftSubmitRunInput, signal?: AbortSignal) {
+     //   return request({
+     //     method: 'POST',
+     //     path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs',
+     //     body: { request_id: input.request_id, prompt: input.prompt, input_refs: input.input_refs ?? [], knowledge_scope: input.knowledge_scope ?? '', base_version_id: input.base_version_id ?? '' },
+     //     signal,
+     //   });
+     // }


─── apps/web/src/features/craft/routes.tsx:904-904 ───
[maintainability · low] useMemo 依赖数组中 locale 出现两次（第 2 位与倒数第 2 位）。冗余依赖不引发功能问题，但属复制粘贴痕迹，影响可读性与 lint
一致性（react-hooks/exhaustive-deps 通常会告警），应删除末尾重复项。

-   ]), [sessionId, locale, inputDecisionState, associatedInputs, decideInput, craftApi, scopeController, accessState, currentMeId, grantAccess, revokeAccess, activeRun, locale, refreshWorkbench]);
+   ]), [sessionId, locale, inputDecisionState, associatedInputs, decideInput, craftApi, scopeController, accessState, currentMeId, grantAccess, revokeAccess, activeRun, refreshWorkbench]);


─── apps/web/src/features/craft/routes.tsx:515-522 ───
[maintainability · low] refreshWorkbench 的 get+merge 块与 requestBudgetExtension（本文件
L537-545）内联的同一段逻辑逐字重复；且新注释声称"预算扩展（a budget extension）复用它"，实际 requestBudgetExtension
仍持有自己的副本，注释与现状不符。建议让 requestBudgetExtension 委托 refreshWorkbench（其后照旧 await
controller.load），消除双处维护同一合并的漂移风险；若暂不改调用方，请修正注释避免误导后续维护者。

      setWorkbenchInfo((prev) => prev === null ? prev : {
        ...prev,
        resumed: view.active_run !== null,
        activeRun: view.active_run === null
          ? null
          : { id: view.active_run.run_id, status: view.active_run.status, waitReason: view.active_run.wait_reason },
      });
    }, [craftApi, scopeController]);
+   // requestBudgetExtension 中改为：await refreshWorkbench(sessionId); await controller.load(sessionId);


─── packages/api-client/src/craft/index.ts:248-261 ───
[bug · critical] submitEdit 返回 unwrap(request(...))，而 unwrap 只返回 envelope.data：这会剥掉 data 外层以及信封级的
writer_acquisition 与 initiated_by。但消费方 projectEditOutcome（workbench-edit.tsx）要求入参是完整 PostCraftRun
信封——它先检查 raw.data 是对象、再从信封层读 writer_acquisition/initiated_by，其测试夹具 acquiredWire 也注明 "The
PostCraftRun wire envelope, exactly as the server answers it"（{ success, data: { run_id, ... },
writer_acquisition, initiated_by }）；本方法自己的 JSDoc 同样写明 "The resolved value is the RAW run
envelope"。结果：routes.tsx 的 onRequestEdit 把 unwrap 结果原样传回面板后，raw.data 为 undefined → projectEditOutcome
恒返回 null → 即使服务端成功接纳编辑运行，用户也只会看到"修改请求失败: 服务器响应无效"，run_id/发起成员/租约结果全部丢失。注意相邻的 export-consent seam 也用
unwrap，但其投影方消费的是扁平 data 载荷，两处 "raw" 约定不同。建议改为校验 success 后返回整封信封。

      async submitEdit(sessionId: string, input: CraftSubmitRunInput, signal?: AbortSignal): Promise<unknown> {
-       return unwrap(await request({
+       const envelope = await request({
          method: 'POST',
          path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs',
          body: {
            request_id: input.request_id,
            prompt: input.prompt,
            input_refs: input.input_refs ?? [],
            knowledge_scope: input.knowledge_scope ?? '',
            base_version_id: input.base_version_id ?? '',
          },
          signal,
-       }), 'run admission');
+       });
+       // Fail closed on error envelopes, but hand back the RAW envelope
+       // (data.run_id / writer_acquisition / initiated_by) the panel projects.
+       unwrap(envelope, 'run admission');
+       return envelope;
      },


─── apps/web/src/features/craft/routes.tsx:882-884 ───
[bug · low] edit-request 面板把 accessState 的 loading（以及 error）态折叠为
canWrite=false，CraftEditRequestPanel 随即渲染"当前身份为只读成员，不能发起修改"。在角色尚未加载完成（或 access 拉取失败）时，这对
owner/collaborator 是不实陈述——fail-closed 拒绝操作是对的，但文案把"角色未知"断言成了"只读成员"。相邻的 task-access 条目对该状态有专门的
loading/error 区分（本文件 L852-857）。建议在 access 未就绪时渲染 null 或中性提示，而非让面板输出只读结论。

-         const currentMember = currentMeId === null || accessState.sessionId !== sessionId || accessState.status !== 'ready'
+         if (accessState.sessionId !== sessionId || accessState.status !== 'ready') {
+           // Role not established (loading or errored): render nothing rather
+           // than a read-only claim that misstates the member's role.
+           return null;
+         }
+         const currentMember = currentMeId === null
            ? undefined
            : accessState.members.find((member) => member.user_id === currentMeId);

