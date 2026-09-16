# 认证浏览器与可用后端审计 — 2026-09-16

## 结论

本轮未取得可接受的认证后 Vue/React 逐页证据，认证页面、租户权限、真实 mutation 和同条件 Portal 交互均保持 `blocked-env`。原因是 CUA 浏览器连接在初始化阶段返回 `Unable to load browser request-header policy`，且当前审计没有提供可安全使用的测试凭据；不能把旧会话截图或静态代码当作本轮认证证据。

本轮只新增本目录证据，没有修改业务代码、Vue、移动端或桌面业务实现。共享 worktree 中其他代理的未提交修改保持原样。

## 运行条件

| 项目 | 值 |
|---|---|
| worktree | `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient` |
| Vue | `http://127.0.0.1:5180`，Vite 可达 |
| React | `http://127.0.0.1:5181`，本轮只启动只读 Vite dev server |
| 后端 | `http://127.0.0.1:8080`，Docker `WeKnora-app` healthy |
| 浏览器 fallback | Playwright Core，`1355×776`，`zh-CN`，fresh anonymous contexts |
| CUA | 初始化失败：无法加载 browser request-header policy |

## 浏览器证据

执行：

```sh
node artifacts/vue-react-public-browser-evidence.mjs \
  artifacts/browser-evidence-20260916-r2
```

机器结果：[results.json](./browser-evidence-20260916-r2/results.json)。截图已检查并接受为本轮“公开页面观察”证据：

- [Vue login](./browser-evidence-20260916-r2/vue-login-public.png)
- [React login](./browser-evidence-20260916-r2/react-login-public.png)

观察结果：

| 检查 | Vue | React |
|---|---|---|
| `/login` | HTTP 200，登录表单可见 | HTTP 200，登录表单可见 |
| 未认证 `/platform/apps` | 最终 URL `/login`，登录标题可见 | 最终 URL `/login?next=%2Fplatform%2Fapps`，登录标题可见 |
| 浏览器内认证状态 | 未执行登录，未读取或写入凭据 | 未执行登录，未读取或写入凭据 |
| 租户/权限/mutation | `blocked-env` | `blocked-env` |

截图同时显示公开登录页存在可见的布局/控件几何差异；该差异只能作为继续修正的定位证据，不能凭截图直接宣称整体 parity 已通过。

## 后端边界

执行的只读请求及结果：

```sh
curl -sS http://127.0.0.1:8080/health
# {"status":"ok"}

curl -sS http://127.0.0.1:8080/api/v1/auth/config
# {"complex_password_enabled":false,"registration_mode":"self_serve","success":true}

curl -sS http://127.0.0.1:8080/api/v1/auth/oidc/config
# {"success":true,"enabled":false,"provider_display_name":"OIDC"}

curl -i -sS http://127.0.0.1:8080/api/v1/auth/me
# HTTP/1.1 401 Unauthorized
# {"error":"Unauthorized: missing authentication"}

curl -i -sS http://127.0.0.1:8080/api/v1/knowledge-bases
# HTTP/1.1 401 Unauthorized
# {"error":"Unauthorized: missing authentication"}
```

Docker 状态检查显示：`WeKnora-app=running/healthy`、`WeKnora-postgres=running/healthy`、`WeKnora-docreader=running/healthy`、`WeKnora-redis=running`。这证明基础服务可用和匿名边界可观察，不证明认证成功、租户隔离、角色差异、业务写入或 provider 回调。

## 测试证据

| 命令 | 结果 | 证据层 |
|---|---|---|
| `pnpm --filter @weknora/web test` | 1172 passed, 0 failed | React 单元/DOM 集成 |
| `pnpm test:shared -- packages/api-client/src/auth/*.test.ts packages/domain/src/auth/*.test.ts` | 495 passed, 0 failed | shared API/domain 单元 |
| `pnpm test:desktop` | 9 passed, 0 failed | Desktop renderer 单元 |
| `pnpm typecheck:web` | 通过 | 静态类型检查 |
| `pnpm typecheck:desktop` | 通过 | 静态类型检查 |
| `git diff --check` | 通过 | 文本差异检查 |
| `pnpm test:embed` | 失败：并发代理未完成的 `apps/embed` 改动导致导出缺失 | 当前共享 worktree 的未稳定状态 |
| `pnpm typecheck:embed` | 失败：同一并发改动含重复属性和未定义符号 | 当前共享 worktree 的未稳定状态 |

Embed 两个失败命令的错误涉及 `apps/embed/src/embed-ui.test.ts`、`apps/embed/src/embed-ui.ts`、`apps/embed/src/EmbedApp.tsx`；本轮没有修复、回滚或覆盖这些文件。

## 认证验收阻塞项

以下项目必须在后续具备稳定浏览器连接、隔离测试凭据和可复现租户 fixture 后重新执行：

1. Vue 与 React 同一用户、租户、角色、语言、主题、视口下的逐页截图和 computed style。
2. owner/admin/contributor/viewer 的入口可见性、403/404、只读和 mutation 结果。
3. 登录成功、注册成功/失败、OIDC 回调、邀请、workspace onboarding 和重定向目标。
4. Apps 连接/授权/动作、知识库文档、设置 Portal、上传/预览和聊天流的真实后端状态。
5. Wails renderer 的原生启动、外部 URL handoff、凭据桥接和逐功能交互。

这些缺口均为 `blocked-env` 或并发工作树未稳定状态，不应被 1172/1172、构建、匿名截图或后端 health 结果升级为 parity accepted。
