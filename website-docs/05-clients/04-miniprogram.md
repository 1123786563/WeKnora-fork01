# 微信小程序客户端（Taro）

微信小程序客户端源码位于 `apps/miniprogram`，基于 Taro 4.2.1 + React 18，覆盖 home/chat/auth/knowledge/execution/account 六个域。它与 Expo 原生 App（`apps/mobile`）共享同一套业务深 Module：

- `@weknora/mobile-core`：Mobile Runtime（会话编排）、Task Office（任务列表/详情/发起/审批收件箱）、Resource Shelf（Agent/知识库/连接）、Task Material（产物/预览/终端）；
- `@weknora/api-client`：`mobile/*` remote Adapter（复用既有 ClientRequest 通道，不新建 HTTP client）；
- `@weknora/contracts` / `@weknora/domain`：wire 契约与纯领域策略。

小程序只提供平台 Adapter：`src/platform/transport.ts`（wx.request/uploadFile 的 HTTP/SSE 传输）、`src/platform/credential-store.ts`（本地存储凭据仓——只有 MobileRuntime 一个写者）、`src/platform/authorized-channels.ts`（授权 REST/SSE 通道与免凭据 blob 抓取）、`src/platform/intent-log.ts`（耐久任务意图）。组合根在 `src/services/runtime.ts`（MobileRuntime 装配）与 `src/services/mobile-office.ts`（深模块记忆化工厂）。

## 迁移记录（replace-dont-layer）

仓库根原有的原生微信小程序 `miniprogram/`（无框架、API Key 直连、仅知识库问答）已于本迁移中**整体删除**：Taro 编排器经同一 Task/Resource/Material Interface 覆盖其能力后，旧树按「replace-don't-layer」退出（见 `docs/specs/2026-09-20-mobile-module-seams.md` §14）。删除的门槛由 `apps/miniprogram/tests/orchestrator.test.mjs` 长期守卫：旧树不得复活、workspace 恰一个 miniprogram 条目、存续编排器必须依赖 `@weknora/mobile-core`。旧的 `tests/miniprogram/*.test.js` 白盒测试随旧树一并退出，其行为由 mobile-core Interface 级测试与 `apps/miniprogram/tests/office-assembly.test.mjs` 场景测试承接。

## 认证与连接

- 后端地址来自构建期注入的 `__API_ORIGIN__`（见 `config/index.ts` 与 `.env.example`）；host 大小写归一化后作为唯一会话身份键。
- 登录走 MobileRuntime（邮箱+密码，Bearer + refresh 单飞轮换）；凭据只存于本机 storage 的 `wk:auth:<origin>` 键，**只有 Runtime 一个写者**，UI 与快照视图永不包含 token。
- 切换工作空间 = `MobileRuntime.activateTenant`（服务端重新签发并复核身份）；登出撤销本地 scope 与私有缓存，远端吊销尽力而为。

## 本地开发与测试

```bash
pnpm install
pnpm --filter @weknora/miniprogram run dev:weapp    # 微信开发者工具导入 dist/
pnpm --filter @weknora/miniprogram run test         # node --experimental-transform-types --test tests/*.test.mjs
pnpm --filter @weknora/miniprogram run typecheck
pnpm --filter @weknora/miniprogram run tokens:check
```

测试在 Node 内以契约级 Taro 替身（`tests/helpers/taro-stub.mjs`）装配真实源码：`tests/assembly.test.mjs` 验证 Runtime 编排（登录恢复/401 单飞刷新/迟到丢弃/SSE 装配），`tests/office-assembly.test.mjs` 在最高稳定 Interface 上跑关键 scenario（home 聚合/列表分页/耐久发起/详情快照+SSE/审批四态/资源三态/材料预览）。真实后端集成证据为 opt-in：设置 `WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD`（可选 `WEKNORA_MOBILE_TEST_START_TASK=1`）后运行 `tests/integration/miniprogram-office-integration.test.mjs`。

## 边界与诚实声明

- steer/cancel 命令暂经授权 API 直发（统一 Task 意图通道属后续 Issue）；审批页在 wire 提供动作详情前仅支持安全拒绝。
- 小程序不执行 HTML/脚本/终端输入；材料下载以短时效签名链接提供。
- 支付通道未接入；订单与权益页如实展示服务端状态。
