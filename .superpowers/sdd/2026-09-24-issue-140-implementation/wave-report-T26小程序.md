# Wave 10 — T26 小程序：微信小程序申请、材料与本人投递（Issue #166 implement）报告

- 状态：**DONE**（真机门槛单列 blocked：无设备，T06/T24 先例）
- BASE `f6b14cf95` → HEAD `135ace9de`（本地 2 commits，未 push）
- Worktree：`/Users/wuyongjun/.codex/worktrees/issue-140-t26-miniprogram/WeKnora-fork01`
- 完整报告+证据：worktree 内 `.superpowers/sdd/2026-09-24-issue-140-t26-miniprogram/task-report.md`（7 张可区分截图、driver/fixture/probe 脚本、脱敏服务端日志、manifest）

## RED → GREEN（TDD）

- RED：`node --experimental-strip-types --test tests/application-material.test.mjs` → **tests 22 / pass 0 / fail 22**（createApplication/openMaterialExport/sha256Hex 等全部缺失）。
- GREEN（三命令照简报复跑，真实输出）：
  - `pnpm --filter @weknora/miniprogram test` → `ℹ tests 112 / pass 112 / fail 0 / skipped 0`（90 基线 + 22 新增）
  - `pnpm --filter @weknora/miniprogram typecheck` → 13 errors（全部 account/pages.tsx CommercialSummary 基线豁免，**零新增**）
  - `pnpm --filter @weknora/miniprogram build:weapp` → `Compiled successfully`；dist/career/application-material.* 产出；t-button 引用 `/npm/tdesign/button/button`
  - `git diff --check` → 干净

## 提交列表

| commit | 内容 |
| --- | --- |
| `275b6f704` | feat(miniprogram): T26 申请/材料/导出/本人投递——同源合同、认证兑付下载与三 seam 恢复（service 扩展+平台适配器+页面+22 用例+taro-stub 保真修复+路由注册） |
| `135ace9de` | fix(miniprogram): 真实 DevTools 验证驱动的三处页面修复（恢复态置顶/对账错误不静默/同源版本发布下载可达） |

## 文件清单

所有权内（简报 Files + 调度员补充 career service 扩展）：
- `apps/miniprogram/src/career/application-material.tsx`（新）+ `application-material.config.ts`（新）
- `apps/miniprogram/src/adapters/career-platform.ts`（扩展：sha256Hex、openExportedDocument 认证兑付+digest 校验+D3 副本清理、EXPORT_GRANT_EXPIRED/DIGEST_MISMATCH typed 码）
- `apps/miniprogram/src/services/career.ts`（扩展：evaluate/application/material/export/submission 全链 + writeRecoverable 未知结果恢复模式，与 Web 同一批 api-client 解码器=同源同版本）
- 测试落位说明：计划点名 `application-material.test.tsx`，但仓库 harness 为 `node --test tests/*.test.mjs`（strip-only 不支持 JSX）→ 按可执行惯例落位 `apps/miniprogram/tests/application-material.test.mjs`（T24 同款偏差）

必要配套（逐条声明）：
- `apps/miniprogram/src/app.config.ts`（career 分包注册）、`apps/miniprogram/src/core/routes.ts`（+careerApply）、`apps/miniprogram/src/career/discovery.tsx`（导入成功后入口行，页面可达无死代码）、`apps/miniprogram/tests/core.test.mjs`（路由清单 21→22）、`apps/miniprogram/tests/helpers/taro-stub.mjs`（readFileSync 精确字节范围：Node Buffer 池 byteOffset 丢失会污染摘要校验，真实 weapp 契约是精确文件字节）

## 行为验收要点（证据层级）

- **同源同版本**：与 Web 同一 API 冻结合同+同一批解码器；单测 A1-A4 断言请求体字段与服务端 DisallowUnknownFields 结构一一对应；材料确认追加不可变版本（versions=[1,2] 不覆盖）。
- **PDF/DOCX 认证兑付下载**：signed-url→带 Bearer 的 GET 兑付→**全字节 sha256 与授权 digest 比对**（校验层级=全字节摘要，如实声明）→私有副本打开→立即清理；DevTools+真实服务器上 pdf(2422B)/docx(2164B) digest 一致；匿名兑付 401；**过期授权（ttl=1s 真实过期）→404 export_grant_invalid→重签→成功**。
- **硬条件警示常驻+显式继续**：服务端 409 hard_ineligible_requires_continue typed；显式继续后 receipt.warning 携带、qualified=false；页面警示常驻（截图+图像复核实际可见）。
- **投递确认零自动提交零外发**：单测 G1 断言恰好一次 POST、零外部 origin、零原生副作用；重复确认 409 typed；G2 显式未知独占。
- **三 seam**：冲突回执（C1-C3 typed+currentRevision）、未知对账（D1-D3 原 requestId 恢复/安全重发）、空间切换（E1 SCOPE_CHANGED 且不为新作用域留 intent）。
- **真实 DevTools（57817）**：app 驱动 **18/18 PASS**（登录→同源档案→未知对账入口→申请回执+警示常驻→材料版本→导出双格式→投递 V2 绑定→跨租户 typed 拒绝）；服务端 fixture 探针 **23/23 PASS**；7 截图 7 个不同 sha256（文本断言+滚入视口+前缀两两不等+图像模型视觉复核）。

## 自查与遗留

- 修复轮中 live 驱动发现并修复三处页面缺陷（恢复态折叠线下不可见/对账错误静默/同源版本发布不可达）——已提交 `135ace9de` 并复跑三命令全绿。
- 真机验证 **blocked**（无设备）：仅官方 DevTools 模拟器（T06/T24 先例）。
- t-button GUI tap 不被 automator 触达（T24 定论，本轮复验一致）：t-button 动作业务由 112 单测（真实装配）+23 服务端探针组合覆盖；GUI 级触发需人工手指。
- 工程注意（新增实测事实）：DevTools 不热载 dist，重建后需 `cli quit`+`cli auto` 重启；automator 包装的 pageScrollTo 在本页失效，须 `callWxMethod('pageScrollTo',{scrollTop,duration:0})` 直调。
- 环境已清理：57817/9431 空闲，token/密码/密钥/SQLite 删除，日志无凭证（归档前再剔除）。

---

## 评审修复轮 1（F1 high + F2-F6 medium 已修；F7/F8 low 记录不修）

- HEAD `135ace9de` → `e44e3246a`（本地 1 commit，未 push）；worktree 证据 `round-2/`。
- RED（新增 6 用例全失败：S1/D4/D2b/D3b/M1/T1）→ GREEN 全过：
  - F1 投递版本显式三态：`resolveSubmissionVersion` unselected/unknown/bound；页面未选择常驻警示+typed 阻断（`submission_version_unselected`），绝不被推断为显式未知。
  - F2 材料恢复重放按 intent `op`（edit/confirm）重放同端点同内容——edit 意图绝不重放成 confirm（D2b：重发 body 逐字节等于原始 edit body、/materials/confirm 零调用）。
  - F3 expectedRevision 发送前捕获并随 intent 持久化，三个 retry* 逐字节重放原始值（D4/D3b：desk 修订 3→8/9/6 后重发仍 expectedRevision=3、requestId 不变——服务端全量指纹含 expectedRevision）。
  - F4 多节全量编辑：`editableFromBody`/`bodyFromEditable` 往返 deepEqual 保真（小节+claims 不丢、全空拒绝 `material_body_empty`）；页面按节编辑+增删。
  - F5 声明时间严格解析：`parseDeclaredOccurredAt`（数值构造+分量回读拒绝 2026-02-31 滚动；仅接受 YYYY-MM-DD[ ]HH:mm[:ss]）；无效输入 typed 阻断（`declared_time_invalid`），绝不静默回退确认时间。
  - F6 证据补档：fresh 一次性环境重跑，`round-2/fixture-run.log`（23/23 完整 stdout）、`round-2/driver-run.log`（18/18 完整 stdout）、`round-2/server-full.log`（完整服务端日志：signed-url×4、pdf/docx 认证兑付（含过期签名 404、B 403 兑付）、submissions POST×2、跨租户 403×3）、`round-2/shots/`（7 png，7 个不同 sha256）。
  - F7（low 记录）：retryPendingApplication/retryPendingSubmission 页面未接线——本轮 D4/D3b 已补测试且 F3 后语义正确；GUI 接线留待后续。F8（low 记录）：申请回执跨会话读取入口留待后续（材料已有）。

### 修复轮验证（真实输出）

```
pnpm --filter @weknora/miniprogram test        → ℹ tests 118 / pass 118 / fail 0 / skipped 0
pnpm --filter @weknora/miniprogram typecheck   → 13 errors（account/pages.tsx 基线，零新增）
pnpm --filter @weknora/miniprogram build:weapp → Compiled successfully in 47.04s
git diff --check                                → 干净
（live 复验）fixture 23/23 + driver 18/18，stdout/完整日志归档 round-2/
```
