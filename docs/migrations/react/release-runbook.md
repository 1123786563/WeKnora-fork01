# React 多端迁移发布与灰度 Runbook

本 Runbook 约束 T24 的构建和灰度证据。T25 之前，Vue `frontend/` 仍是当前生产回退源；React 产物必须以独立候选 artifact 验证，不得把编译成功当作功能等价或真机验收。

## 1. 构建输入与可辨识性

所有候选产物记录以下三元组：

| 字段 | 来源 | 用途 |
|---|---|---|
| 版本 | 发布 tag 或 `dev` | 用户可见发布版本 |
| commit | `VITE_FRONTEND_COMMIT` / `git rev-parse --short HEAD` | 关联源码与回退 artifact |
| renderer | `vue` 或 `react` | 防止误把候选产物当成当前生产产物 |

React 候选使用：

```bash
pnpm install --frozen-lockfile
node scripts/check-react-boundaries.mjs
REACT_BUILD_VERSION=vX.Y.Z VITE_FRONTEND_COMMIT="$(git rev-parse --short HEAD)" \
  pnpm run build:react-bundle
```

输出为 `dist/react-web/web/`，必须同时包含 `index.html`、`embed.html`、`assets/`、`embed/assets/` 和 `BUILD_INFO.json`。Lite 发布 workflow 将它打成 `WeKnora-react-web_<version>.tar.gz`；T25 切换前，正式 Lite 二进制仍使用 `frontend-dist`。

## 2. 静态路由与代理检查

在候选 artifact 或 Nginx 容器前，用相同后端 fixture 检查：

| 请求 | 期望 | 证据 |
|---|---|---|
| `GET /` | Web `index.html`，`Cache-Control: no-cache` | 响应头和内容指纹 |
| `GET /platform/chat/<id>` | Web SPA fallback，不返回 404 | deep-link 刷新 |
| `GET /embed/<channelId>` | 独立 `embed.html`，不回退到 Web `index.html` | 内容指纹/Embed JS 路径 |
| `GET /assets/<hash>.js` | 静态资源，长期缓存 | 响应头 |
| `GET /embed/assets/<hash>.js` | Embed 静态资源，长期缓存 | 响应头 |
| `GET /api/...` | 原样转发到后端，不被 SPA 截获 | 后端 request id |
| `GET /files`、`GET /r/<token>` | 文件/短链路由，不被 SPA 截获 | 状态码和后端日志 |
| SSE 请求 | `proxy_buffering off`，长连接不被缓冲 | 首 token 与持续事件 |
| Terminal WS | `Upgrade`/`Connection` 转发，长超时 | 101 handshake |

Nginx 镜像还要用非根子路径部署做一次验证，例如 Ingress `/weknora/`；若部署方式不支持 Vite `base`，必须保留独立 origin，而不是临时改写 token 或 API 基址。

## 3. 回归矩阵

每个灰度批次在同一个后端版本和 fixture 上记录以下交叉维度。未执行的格子写明原因，不得记为通过：

| 维度 | 最小集合 |
|---|---|
| 角色 | viewer、editor、owner/system admin |
| 空间 | 无租户、单租户切换、跨租户拒绝 |
| 部署 | Nginx Docker、Lite Web CLI、Wails 桌面、移动 bundle |
| 语言 | zh-CN、en-US、ja-JP、ko-KR、ru-RU、Embed locale |
| 浏览器/OS | Chromium/macOS、Chromium/Linux、Safari/iOS、Android；原生项须有真机或模拟器证据 |
| 旅程 | 登录/刷新、知识库 CRUD、上传/处理、聊天 SSE/停止/恢复、审批/OAuth、引用/产物、Embed、WS 终端 |

至少保存：旧 Vue 与新 React 的 HTTP 状态/请求体对照、首屏时间、首 token、长消息渲染、内存和包体积。交互 p95 相对 T01 基线恶化超过 10% 时暂停扩大灰度并建立性能问题记录。

## 4. 灰度与停止条件

1. **内部灰度**：维护者账号、非生产 fixture、独立 React artifact/origin。
2. **小范围灰度**：明确租户 allowlist，保留同版本 Vue artifact 与上一受支持客户端。
3. **全量**：只有矩阵中适用项接受、安装包/原生运行证据齐全，且回退演练成功后执行。

任意批次出现白屏、401 异常上升、SSE 流失败/重复、上传失败或 WS handshake 失败显著超基线，立即停止扩大范围；切回上一 artifact，保留失败请求的 request id、renderer/commit、后端版本和部署配置。切回 UI 不触发数据库破坏性迁移，也不删除旧客户端可用的凭证命名空间。

## 5. 证据分层与签收

- 静态边界、TypeScript、单元测试和 bundle export：只能证明构建/规则层。
- mock 后端与本地 Nginx：证明协议拼装、fallback 和代理配置，不证明生产权限或真实模型链路。
- 真实后端 smoke：至少覆盖 bearer refresh、租户/角色负例、SSE、上传、Embed token/session、WS ticket。
- Wails 安装包和 iOS/Android 真机/模拟器：必须分别记录平台、版本、artifact checksum 和运行结果。

只有四层证据与对应矩阵项都齐全，T24 才能从 `review` 推进到 `accepted`；否则保留 `review`，列出下一步与未覆盖单元。
