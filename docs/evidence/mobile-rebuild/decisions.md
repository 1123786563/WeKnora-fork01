# decisions.md · 实施裁决记录

## D-01 视觉核对值冲突：以设计包 tokens.json 为权威
任务提示给出的核对值（brand #09694D/#80DDB1、bg #F5F7F3/#101C16、hero #153D30、正文 16/24）与设计包 `tokens/tokens.json` 实际值（brand #08766A/#94E0BA、bg #F6F7F3/#111F1B、hero #DDEDE4/#274936、hero-ink #153D30、body 16/26）不一致。
指令明确"实际样式全部从新设计 tokens/tokens.json 生成和引用，不在各页面写死"，且视觉权威="本次新设计、令牌与高保真页面"。**裁决：全部视觉值从 tokens.json 生成；提示中的核对值视为过时草稿。** 深浅主题同步实现。

## D-02 工具链：Expo SDK 55 独立 npm 工程
- `apps/mobile-next` 不加入根 pnpm-workspace（workspace 未含它即物理隔离，杜绝对 packages/*（旧业务包）的 workspace-link；对其他端零改动）。
- 版本：expo ~55.0.x、react 19.3.x、react-native 0.83.x、expo-router ~55.0.x、typescript、jest、@testing-library/react-native。依据：设计包 verified-sources 的 SDK55 矩阵 + 本地已有 55 系列缓存（安装可靠）。不继承旧 package.json/别名/原生目录/依赖全集。
- 原生模块最小集：expo-secure-store、expo-sqlite、expo-file-system、expo-av（听写录音）、expo-web-browser（SSO/授权）、expo-linking（深链）。均为官方维护。

## D-03 后端接口事实为唯一 API 依据
已核验路由（api-facts.md）。客户端 wire 类型从 Go DTO 重写；OpenAPI（docs/swagger.yaml）存在但不自动生成（先手写 + 单测对齐 Go 源），生成工具化列为后续优化。

## D-04 `/workbench/overview` 后端缺失 → 客户端聚合降级 + 最小后端补充候选
设计要求薄聚合避免 N+1。后端无该端点。**v1：客户端聚合**（GET /sessions + pending interactions + recent artifacts 并发读取，单 scope 缓存 + as_of）；同时在台账记录"最小后端补充"候选（一个只读聚合路由），待后续后端任务实施。不假装接口存在。

## D-05 四类交互：三类有 API，question/connection 走既有专端点
真实 kind：`tool_approval(approve/reject)`、`budget(extend)`、`recovery(retry/provide_result/terminate)`（internal/workbench/interaction.go）。question 仅 craft 会话交互端点；connection 授权= MCP OAuth resolution + /apps/connections。**裁决：M09 主实现 tool/budget/recovery；question/connection 在 UI 建模为独立分支并接 craft/OAuth 端点；不把四类压成一个 approve 布尔。**

## D-06 通用通知 inbox 后端缺失
仅 /me/invitations（邀请）。**v1 收件箱 = pending interactions（需你处理）+ 邀请 + 最近完成的本地归并读模型，数据全部来自真实 API；推送注册列为 blocked-dependency（后端无 push 投递 API）。**

## D-07 实时语音后端能力缺失
无实时语音端点。M16 实现确认式听写全链路；"实时语音"入口如实展示能力不可用（CapabilityReason），不伪造。

## D-08 模拟器/后端环境
本地 dev 后端 8082（记忆：源码启动）。集成测试可指向本地 Go；无环境的项目用录制 fixture 并显式标注。截图验证用 iOS 模拟器（本机 darwin）。

## D-09 设计包中"沿用 apps/mobile/复用 Happy"段落
按用户 2026-09-18 指令整体覆盖：apps/mobile-next 从零实现；本仓库旧 apps/mobile、packages/* 保持原样不删除、不依赖。隔离由 scripts/check-isolation.mjs 门禁保证（import 图+别名+依赖+可达图+metro 五项检查）。

## D-10 原型模拟文案 → 生产语义替换
M01 的"进入演示空间"→"登录"；"演示无需真实密码"删除；notice 从"本地交互原型，不发送账户信息"改为真实隐私说明（"登录信息只发送到你指定的 WeKnora 服务器"）。M05 附件按钮在未接后端时不假装成功，显示持久提示。M14 下载/分享在签名链接服务未接入前如实提示能力待接入。M16 听写在原生录音验证前提供草稿编辑模式并如实说明。visualFixture 是显式视觉对照开关（AppProvider），非生产默认，fixture 数字仅作隔离测试数据。

## D-11 SSE 帧格式以真实 Go writer 为准（G01 实测）
workbench 流：`id: seq\nevent: type\ndata: payload\n\n`（workbench_read.go:318）；agent_run 流：`event: <seq数字>\ndata: {seq,attempt_id?,type,payload}` + run/keepalive/error 帧（agent_run.go:175-228）；409 cursor_expired 在 HTTP 层非 SSE 帧。Parser 字节级实现（UTF-8 跨块/CRLF 跨块/多行 data/缺口/重复），fixture 由 tests/sse/gen/main.go 用真实写出逻辑生成（go run），TS 测试消费真实字节（12/12 绿）。

## D-12 request_id 生成不依赖 expo-crypto
request_id 非安全敏感（幂等键），时间戳+随机即可；避免纯逻辑模块依赖原生模块（jest transform 边界）。未来需要更强唯一性时在 native 装配层注入。

## D-13 依赖对齐 SDK 55 期望版本 + react-dom/test-renderer
`expo install --fix` 半途失败（react-dom 19.3 peer 冲突）后手动完成：react 19.2.0、react-dom 19.2.0、RN 0.83.10、safe-area-context ~5.6.2、screens ~4.23.0、svg 15.15.3、@types/react ~19.2.10，legacy-peer-deps 安装。RNTL 14 需要新 `test-renderer` 包（RNTL 作者维护的 react-test-renderer 现代替代，1.3.0），不能用 react-test-renderer 19.2（无 createRoot）。

## D-14 幽灵路由修复：src/app → src/host
typedRoutes 暴露 expo-router 将 `src/app/` 作为路由根扫描，组合根 AppProvider.tsx 曾被当成路由 `/AppProvider`。修复：移至 `src/host/AppProvider.tsx`。教训：expo-router 工程内不得使用 `src/app` 作为业务目录名。


