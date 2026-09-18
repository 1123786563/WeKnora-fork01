# 测试报告（2026-09-18）

分支：`fix/miniprogram-qa-regression`（起点 `6a70c35a` = main）。本报告只声明有证据的结论层级。

## 1. 实际执行与退出码

| 命令 | 首次（基线 6a70c35a） | 修复后（分支 HEAD） |
| --- | --- | --- |
| `pnpm install --frozen-lockfile`（隔离干净检出） | 0（29.5s，2340 resolved） | 未重复（无锁文件变更） |
| `pnpm --filter @weknora/miniprogram run tokens:check` | 0 | 0（125 tokens / 31 引用） |
| `pnpm --filter @weknora/miniprogram run test` | 0（25/25） | **0（41/41**，累计新增 16 项：auth 2+3、装配 11） |
| `pnpm --filter @weknora/miniprogram run typecheck` | 0 | 0 |
| `WEKNORA_API_ORIGIN=占位 pnpm --filter @weknora/miniprogram run build:weapp` | 0（10.28s） | 0（9.04s） |
| `pnpm exec tsx --test packages/api-client/src/chat/mini-regression.test.ts` | 0（7/7） | 0（7/7） |
| `pnpm test:shared` / `pnpm typecheck:shared` | 0 / 0 | 0 / 0（分支后复跑） |
| `pnpm typecheck:web` / `pnpm build:web` | 0 / 0 | 0 / 0 |
| `pnpm test:embed` / `typecheck:embed` / `build:embed` | 0/0/0 | 0/0/0 |
| `pnpm test:desktop` / `typecheck:desktop` | 0 / 0 | 0 / 0 |
| `pnpm build:desktop-renderer` | **1（inherited）** | 1（干净 main 同样失败：views craft 子路径解析 ENOTDIR） |
| `pnpm test:mobile` | **1（inherited）** | 1（干净 main 同样失败：useNavigateToSession 缺 createProductSessionNavigation 等 82 套件） |
| `pnpm typecheck:mobile` | **2（inherited）** | 2（干净 main 同样失败：ConversationScreen TS2339） |

> desktop build / mobile test / mobile typecheck 三项在干净 main（6a70c35a）上以相同退出码复现
> （1/1/2），与本分支改动无关，属其他泳道的既有问题；本分支未触碰其源码。

## 2. 真实后端联调（OrbStack 本地 Go 后端，测试账号，只读操作）

- `POST /api/v1/auth/login` → success，token/refresh_token 位于根级（与 parseSession 根级回退匹配），tenant 10001。
- 以真实 Bearer 探测：`knowledge-bases` **200**；`workbench/executions?limit=20` **404**（Go 无列表路由）；
  `execution-targets` **404**、`commercial/summary` **404**（本部署未注册，客户端诚实降级）。

## 3. 微信开发者工具模拟器（SIM，真实 origin 构建）

工具 Stable 2.02.2608070，项目窗口完整在屏。第二轮（下午）补齐 16 分包页逐页验证 +
真实登录链路（Up 栈后端、parity-up 测试账号、表单输入→提交→选空间→进入工作台全程真实请求），
20/20 页渲染通过，证据见 [test-matrix.md](./test-matrix.md) 下午轮表格与
[evidence/sim-tour/](./evidence/sim-tour/) 20 张截图。第三轮（晚间）交互层测试：Tab 切换、
二级导航返回、搜索过滤、表单禁用态、无效 ID 恢复降级、审批空态（无批准入口）、退出确认取消、
长中文输入、冷启动会话恢复（曾疑"重启丢会话"，查实为 dist 被并行会话替换成别的 origin
导致的预期 key 不匹配，非缺陷）、尺寸转换链路核验（token→rpx 一次性 ×2，无重复乘二，
实测 .wk-card 左缘 20px 与 40rpx 换算一致）——断言 24/25 通过，唯一 FAIL 为脚本断言
错位且实际行为更严。证据见 test-matrix 交互层表格与
[evidence/sim-interactions/](./evidence/sim-interactions/)。第一轮（上午）为 dev 栈 4 Tab 验证：

工具 Stable 2.02.2608070，项目窗口完整在屏。已登录真实会话（既往手动登录的存储 + 本次真实 origin 重建后 bootstrap 复验通过）：

| 页面 | 结果 | 截图 |
| --- | --- | --- |
| pages/home/index | 真实空间名 parity-test's Workspace、Agent 列表与任务中心真实数据；Errors: 0 | [evidence/sim-home-logged-in.png](./evidence/sim-home-logged-in.png) |
| pages/tasks/index | 列表端点 404 → "此资源不存在…" + "当前基线尚缺任务集合读模型" 诚实降级；恢复输入框空时"读取任务状态"禁用 | [evidence/sim-tasks-page.png](./evidence/sim-tasks-page.png) |
| pages/knowledge/index | 真实知识库数据（"AI 产品研究"，api 来源） | [evidence/sim-knowledge-page.png](./evidence/sim-knowledge-page.png) |
| pages/me/index | summary 404 → 错误条 + 重新加载，不伪造余额 | [evidence/sim-me-page.png](./evidence/sim-me-page.png) |

调试器 Errors 计数与上述 404 一一对应（home 为 0，访问降级页后增加），无渲染错误。
未覆盖：16 个分包页的逐页 SIM 导航、Android/iOS 真机（复验步骤见第 7 节）。

## 4. 修复缺陷清单

见 [bug-ledger.md](./bug-ledger.md)：D1（测试吞导入错误）、D2（CI 缺门禁）、D3（文档断链）、
D4（refresh 401 僵尸会话，P1）、D5（unknown 后提交永久卡死，P1）、D6（core-js 目录导入阻断直测）、
**D7（auth 存储 key host 大小写归一化 + 变体凭证迁移，P2）**。
D4/D5/D7 均有修复前红证据（stash/新增测试先红后绿复跑确认）。

## 5. 依赖、锁文件与共享包影响

- 锁文件零变更；frozen 安装在隔离检出通过。
- 共享包（api-client/contracts/domain）零改动；小程序改动全部位于 apps/miniprogram 与 CI/docs。
- 因此 Web/Embed/Desktop 的通过项与 main 持平；三项失败已证明为 main 既有。

## 6. 未解决与风险

1. **inherited（非本分支）**：desktop-renderer 构建、mobile 测试与类型检查在 main 即失败（详见第 1 节），需对应泳道处理；合并本分支不会使其恶化。
2. **部署缺口**：本地后端未注册 execution-targets / commercial 路由（dev 与 Up 栈均 404）；正式联调环境需确认。
3. **not-implemented**：任务集合读模型（Go 无路由）、微信支付、快捷登录、审批正向批准、产物下载定位。
4. 真机验证未做（时间盒）；聊天/执行流式回答未在真实后端跑通（产生模型消耗的副作用操作未获授权执行）。
5. **环境事件（blocked-env，已恢复）**：验证中段 dev 栈容器 `WeKnora-app` 被外部进程移除导致
   `.orb.local` DNS 失效、account 分包页一度挂起；根目录 node_modules 亦被外部进程清空一次
   （`pnpm install --frozen-lockfile` 27.7s 恢复，反向验证了锁文件可重复安装）。
   上述均非本分支代码问题，完整记录见 test-matrix.md。

## 7. 干净环境复验命令

```bash
git clone <repo> && cd WeKnora-fork01 && git checkout fix/miniprogram-qa-regression
pnpm install --frozen-lockfile
pnpm --filter @weknora/miniprogram run tokens:check
pnpm --filter @weknora/miniprogram run test
pnpm --filter @weknora/miniprogram run typecheck
WEKNORA_API_ORIGIN=https://placeholder.invalid pnpm --filter @weknora/miniprogram run build:weapp
pnpm exec tsx --test packages/api-client/src/chat/mini-regression.test.ts
# 模拟器复验：WEKNORA_API_ORIGIN=<真实 HTTPS origin> pnpm --filter @weknora/miniprogram run build:weapp
# 微信开发者工具打开 apps/miniprogram → 依次点击四 Tab 与各分包入口，对照 evidence/ 截图。
```

## 8. 合并建议

- 小程序门禁（安装/令牌/测试/类型/构建/共享回归）在本分支全绿，且新增 CI 门禁持续盯防；
  20 页（4 Tab + 16 分包页）已在微信开发者工具模拟器对真实后端逐页渲染验证，
  登录链路（表单→提交→选空间→进入工作台）真实后端走通——**可合入**。
- 因 desktop/mobile 的 inherited 失败仍在 main 存在（非本分支引入、亦非本分支职责），
  PR 建议**合入前保持 Draft** 直至：① 对应泳道修复或独立豁免；② 真机抽验完成。
- 严禁在补齐真实支付渠道、审批动作详情前提交微信审核或对外发布。
