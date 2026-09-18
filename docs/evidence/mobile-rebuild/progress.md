# progress.md · mobile-rebuild 进度证据

格式：每条工作记录命令、退出码、结果。没有运行写"未执行"。

## 2026-09-18

### 预检
- REPO_ROOT=`/Users/wuyongjun/trea/WeKnora-fork01`（本地已有，直接使用）；分支从 main 新建 `rebuild/mobile-next`（git checkout -b，退出码 0）
- DESIGN_ROOT=`/Users/wuyongjun/Downloads/weknora-expo-hifi-v2`；全包哈希 `54e02578a6ddbaae4366729285198e3d812df36a`
- 已读：tokens.json、native-tokens.ts、docs/01、02、03（部分）、contracts/mobile-proposal.ts、plans/task-index.json（36 项 MX）
- 视觉核对：M03 截图已查看（视觉分析）；HTML 为 JS 渲染壳，DOM 结构以内联脚本为还原参考
- 后端 API 只读核验完成 → api-facts.md（Explore 代理，54 次工具调用）

### 台账
- docs/mobile-rebuild/{implementation,tasks,traceability}.md、api-facts.md 建立；docs/evidence/mobile-rebuild/decisions.md 建立（D-01~D-09）
- tasks.json：RW-001~RW-033；RW-001 implementing

### RW-001 工程初始化
- apps/mobile-next 独立 npm 工程（不加入 pnpm workspace）；expo ~55.0.8 / react 19.3.0 / RN 0.83.1 / expo-router ~55.0.7 / jest-expo 55.0.22
- 首次 npm install 失败：expo-av@~55.0.0 不存在（已被 expo-audio 取代）→ 修正后 EXIT:0（/tmp/npm-install2.log）
- react-native-svg 15.12.1 补装 EXIT:0

### RW-002 主题
- scripts/gen-theme.mjs 从 docs/mobile-rebuild/design-ref/tokens.json 生成 src/theme/tokens.generated.ts（归档与 DESIGN_ROOT 逐字节 diff 一致：TOKENS_IDENTICAL）
- `npx jest tests/theme` → 6/6 绿（JEST:0）
- `npx tsc --noEmit` → 0

### RW-003 组件
- 13 组件：Icon(50 path 设计图标集)/ActionButton/IconButton/FormField/StatusBadge/TaskCard/PendingCard/DecisionSheet/OfflineNotice/RunStatusStack/ArtifactCard/StateView/SearchField+SectionHeader/FilterChips
- `npx jest tests/components` → 14/14 绿（RNTL v14 需 await render）

### RW-005/006 contracts+api
- wire.ts（envelope 三形态/decoder 工具/未知枚举保留）、auth.ts、workbench.ts（run/interaction/SSE 流/usage/targets/knowledge/connection/artifact）
- http.ts：origin 规范化+信任边界（dev 开关显式）、Bearer+X-Tenant-ID、401 单飞刷新（并发共享）、403 不刷新、错误分类（network/unauthorized/forbidden/not_found/conflict/cursor_expired/validation/server/aborted/decode）
- tests: tests/domain/scope.test.ts + tests/api/http.test.ts → 19/19 绿

### RW-010 SSE（G01：Go bytes → TS parser）
- 内部事实核验：workbench_read.go:318 帧 `id: seq\nevent: type\ndata: payload`；agent_run.go:175-228 `event: <seq>\ndata: RunEvent{seq,attempt_id,type,payload}` + run/keepalive(15s)/error 帧；RunEvent{seq,attempt_id?,type,payload}（internal/agent/runtime/contracts.go:67）
- tests/sse/gen/main.go 用 Go 1.26.3 复刻 writer 逻辑生成 fixtures/go-real-bytes.txt（GEN:0）
- parser 字节级（TextDecoder 流式跨块中文/CRLF 块尾等待/多行 data/重复幂等/缺口 onGap/控制帧不进 cursor/缺 type 不推进 cursor）
- `npx jest tests/sse` → 12/12 绿（修复：对象比较恒 false 的循环条件 bug、CRLF 跨块拆帧 bug、type 后验 bug）

### RW-007 platform
- store.ts：MobileStore 端口（四表+草稿）+ InMemoryStore + WriteQueue 串行
- native.ts：SqliteStore（expo-sqlite，DDL 四表+drafts，命名空间 weknora_mobile_next_v1）+ secureCreds（expo-secure-store）

### RW-008/009 认证与空间
- AuthController：bootstrap（凭证→me→memberships→恢复/回落/清凭证）、loginWithPassword、switchSpace（禁写→gen+1→abort→清旧数据→校验→reEnable）、logout
- `npx jest tests/features/auth.test.ts` → 12/12 绿

### RW-013 提交状态机
- SubmissionService：prepared 落盘先行（顺序断言 save:prepared < http）、落盘失败不发送、network→uncertain→reconcile lookup（不新建 ID）、request_id+不同输入冲突、scope 只读拒绝
- `npx jest tests/features/submit.test.ts` → 9/9 绿

### 全量（2026-09-18）
- `npx jest` → 72/72 绿（7 suites）；`npx tsc --noEmit` → 0
- `node scripts/check-isolation.mjs` → 45 源文件、可达图 33，0 违规（ISOLATION:0）

### 页面
- M01 login.tsx、M02 spaces.tsx、M03 (tabs)/index.tsx、M05 new-task.tsx、gate 接 AuthStage —— 主线完成
- M04–M10（6 页 + labels/agentSelection 模块）与 M11–M18（8 页 + targets/selection 模块）由两个实现子代理完成（各自 tsc 0）；M05 已接 agentSelection/targetSelection 回读（useFocusEffect）

### 全量集成（版本对齐后）
- 依赖对齐 SDK 55 期望版本（react 19.2.0/react-dom/RN 0.83.10/svg 15.15.3/screens ~4.23/safe-area ~5.6.2/@types/react ~19.2.10 + test-renderer 1.3.0，decisions D-13）
- 幽灵路由修复：src/app → src/host（expo-router 把 src/app 扫为路由根，typedRoutes 抓出 /AppProvider 幽灵路由，decisions D-14）
- `npx jest` → 77/77 绿（8 suites）；`npx tsc --noEmit` → 0；`npx expo export --platform ios` → 0（18 页 bundle）
- `node scripts/check-isolation.mjs` → 64 源文件 0 违规

### 原生构建与运行（RW-031 证据）
- `npx expo run:ios`：prebuild ✓、CocoaPods ✓、Simulator.app 定位失败（Xcode 27 布局，expo CLI 已知问题）→ 改直接 xcodebuild
- `xcodebuild -workspace ios/WeKnora.xcworkspace -scheme WeKnora -configuration Debug -sdk iphonesimulator build` → **BUILD SUCCEEDED**（XCB:0；两次修复：Pods deployment target 抬 16.0——RNSVG 12.4/SDWebImage 9.0 低于 Xcode 27 下限、expo-router LinkPreview 需 iOS 16 API）
- `xcrun simctl install booted WeKnora.app` + `launch com.weknora.mobilenext` → **原生 App 真机级运行成功**（pid 65543）
- 36 张原生截图（18 页 × 双主题）→ screenshots/native/；抽样复核 M01/M03/M08 通过（visual-verification.md）

### 真实后端联调（RW-008/RW-032，2026-09-18）
- 本地 Go 后端运行于 localhost:8082（/api/v1/auth/config 探活 200）
- `npx tsx tests/integration/live-backend.mts`（随机测试账号，self_serve 注册）→ **10/10 通过**：
  注册 201 / loginWithPassword token+凭证 / memberships 解码 / stage=ready / me()（data.user 修正后）/ tenants()（data.items 修正后）/ sessions()（total=0）/ agents()（4 个 builtin）/ 坏 token→unauthorized / 无凭证 bootstrap→login
- 真实 wire 修正两处解码器：/auth/me 的 user 在 data.user；/tenants 列表在 data.items（auth.ts/weknora.ts，注释标注来源）
- 回归：jest 77/77、tsc 0、隔离门禁 0 违规

### 视觉验证
- 见 visual-verification.md：M01/M03/M08 深度核对通过；36 张存档；逐页像素比对/多宽度/大字体/Android 为 blocked-env 或未完成，如实记录

### Expo Go（备用验证通道）
- Expo Go 55.0.34 simulator build 经镜像下载安装成功（GitHub 直连被网络阻断，gh-proxy 镜像可用）；深链不自动连 Metro（记录，未再用）


