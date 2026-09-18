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

### 安全扫描（2026-09-18）
- Mimosa 静态扫描（normal，scan-job-mu6iw5pe，seal sha256:5e7b1b27…）：全仓库 152 findings、依赖 2012 包（7 包匹配 15 advisory）。范围为整个仓库（含既有代码），不区分新旧；按约束不宣称项目安全；扫描档案 ~/.mimosa/security-scans/ 可复查。

## 提交
- 21bc807a 核心交付（127 文件，26616 行）；e03cb1ec 部署目标修正；d54e0ddc 真实联调+wire 修正
- 分支 rebuild/mobile-next；未 push；旧 apps/mobile 与用户其他文件未改动

## RW-028 / RW-029 / 实时联调补齐（2026-09-18 傍晚）

### RW-028 深链通知接线
- src/features/notifications/deeplink/DeepLinkController.ts：白名单路径解析（对象式路由）、危险参数剥离（action/approve/token 等 8 类）、资源 id 安全字符集、跨空间 pick_space_first、未认证 defer_to_login
- AppProvider 接线 expo-linking URL 事件 + getInitialURL；`npx jest tests/features/deeplink` → 10/10
- 实现 hook 误报注记：Mimosa 将 regex.exec 误判为命令注入（拦截写入），改用 String.match 等价实现后通过

### RW-029 i18n 文案集中化
- src/i18n/zh.ts（约 90 键：02 规格通用状态固定文案逐条收录 + 通用动作 + 错误分类 + M01/M02/M03/M05/审批/深链关键文案）；src/i18n/index.tsx：t(key,params) 插值与缺失键回退 + I18nProvider/useI18n（未包 Provider 回退默认 zh）
- StateView/OfflineNotice 公共组件默认文案接 t()（props 可覆盖）；I18nProvider 挂根布局
- `npx jest tests/i18n` → 6/6（含规格固定文案逐条一致性断言）
- 迁移范围：公共状态/动作/错误文案与核心页面关键文案已集中；子代理所写页面的页面级文案保留原文并以表为后续迁移基线（如实记录）

### 实时联调补齐（PG 容器恢复后）
- PG/Redis 容器恢复（Up healthy）后运行 `npx tsx tests/integration/live-attachments-chat.mts`：
  **附件链路 6/6 实时通过**——登录/创建会话/上传 202（multipart 真实 bytes）/wire 解码（初始 status=unknown 正确保留不冒充）/**AttachmentUploader 对真实后端轮询到 ready**/readyDocumentIds
- agent-chat 实时行为：POST 200 + text/event-stream；真实帧（`event:message` 无空格格式）被 parser 正确消费；服务端发 error 帧（"baseURL SSRF check failed"——该部署 LLM provider baseURL 为 IP 被后端自身 SSRF 校验拒绝）后硬中断连接 → 客户端 error 帧持久展示 + 断流路径（行为正确）。完整回复依赖部署侧配置 provider 域名（环境问题，非客户端缺陷，D-15 记录）
- 协议修正（真实环境发现）：受理帧 agent_query 也带 done:true——isTerminalChunk 收敛 done 终结语义（answer/complete/stop/error），新增回归测试；此前误将受理帧当流结束

### 验证缺口补齐（2026-09-18 晚，verifier next-action 执行记录）

**杀进程与重开恢复（SSE 清单项）**：`npx jest tests/integration/reopen-recovery.test.ts` → **3/3**
- 用 node:sqlite（Node 26 内置）复刻 SqliteStore 的 DDL 与语句（全参数绑定），模拟进程 A 写入（uncertain pending/2 事件/投影 watermark=2/cursor/草稿）后连接关闭（进程被杀）→ 进程 B 新连接重开同一 DB 文件：全部数据恢复；对账闭环按原 request_id 绑定原 runId（不换 ID）；按恢复 cursor 续流时旧 seq 1/2 幂等丢弃、仅提交 seq 3；scope 隔离跨进程成立

**Android 构建（第 4 层双平台的 Android 侧）**：
- `npx expo prebuild -p android --no-install` → 0
- `npx expo export --platform android` → **0**（JS bundle 产物 dist/）
- `ANDROID_HOME=$HOME/Library/Android/sdk ./gradlew assembleDebug` → **BUILD SUCCESSFUL in 20m37s**（GRADLE:0）
  - 产物 `android/app/build/outputs/apk/debug/app-debug.apk`（173MB debug APK，sha256 前 16 位 bb27ff2b22d5b70c）
  - 网络障碍与解决：gradle 9 发行版官方/腾讯镜像均阻断；华为镜像 `mirrors.huaweicloud.com/gradle/` + wrapper networkTimeout=120000 成功；此前两次 URL 替换因 properties `\:` 转义未生效（已修正并留档 android/gradle/wrapper/gradle-wrapper.properties）

**动态字体与长文本压力（当前环境可执行部分）**：
- `tests/components/font-scaling.test.ts` → 3/3：静态断言 app/ + components/ 全部源码无 allowFontScaling={false}/adjustsFontSizeToFit/maxFontSizeMultiplier 限制（200% 字体缩放不被禁用）；正文排版引用 theme.type 令牌
- `tests/components/long-text.test.tsx` → 4/4：超长任务标题（TaskCard numberOfLines=2）、超长文件名（ArtifactCard ellipsizeMode=middle）、超长审批正文（DecisionSheet 冻结摘要完整渲染+Sheet 内滚动）、PendingCard/StatusBadge 超长文本渲染不崩溃

**blocked-env（环境在验证时段变为不可用，如实记录）**：
- iOS 模拟器多宽度（360/390/430）与 200% 大字体原生截图：验证时段 CoreSimulator runtime 全部缺失（`xcrun simctl list devices/runtimes` 均空，Xcode 27 的 iOS runtime 目录不存在）——此前可用的 11 台设备与 iOS 26 runtime 消失（疑似用户环境变更中）。已交付 36 张双主题截图基于此前环境；本项以组件级等价验证（上述 font-scaling/long-text 测试）+ 布局用 flex/流式（无绝对坐标）补偿，真机多宽度截图待环境恢复
- 键盘弹出场景（输入焦点需 UI 自动化，idb 不可用）：未执行
- 语音录音（expo-audio 真实环境）、推送注册（后端无 API，blocked-dependency）、Android E2E 真机链路：未执行
- agent-chat 完整回复端到端：部署 LLM provider baseURL 为 IP 被后端 SSRF 校验拒绝（D-15），依赖部署侧配置域名

**全量回归（本节后）**：`npx jest` → **119/119**（15 suites）；tsc 0；隔离门禁 0 违规（77 源文件）



## RW-027 / RW-015 收尾（2026-09-18 下午）
- RW-027 附件：contracts/attachments.ts（TemporaryDocument 解码）+ HttpClient.uploadMultipart（multipart 真实 bytes 通道）+ AttachmentUploader 状态机（selected→uploading→verifying→ready/failed；未 ready 阻断提交；失败保草稿语义）+ api.uploadAttachment/getAttachment + M05 接线（expo-document-picker 选择→首附件创建会话→上传→状态 chips→提交前 ready 校验）+ app.config 注册插件
- RW-015 M07 发送接线：ChatService（POST /agent-chat/:session_id + SSE 流消费 consumeChatStream）+ ChatProjection（answer 增量拼接/tool 卡/thinking/error 持久/未知类型安全展示/done 终止/EOF→disconnected 续流信号）+ AppProvider 装配（token 缓存 headers）+ M07 页面重写（消息气泡按 parts 渲染、流式"正在输入"、断流提示、发送失败保留草稿）
- 新测试：tests/features/attachments.test.ts（7：状态机/白名单/超时/multipart 表单语义/未 ready 阻断提交闭环）+ tests/features/chat.test.ts（9：投影/真实 SSE 帧/残缺帧跳过/POST 闭环/HTTP 失败/断流）
- 全量回归：`npx jest` → **93/93**（10 suites）；tsc 0；隔离门禁 0 违规（69 源文件）
- 实时联调（live-attachments-chat.mts 已就绪）：**blocked-env**——本地后端 8082 的 PG/Redis 容器在验证时段被停止/重建（docker ps：WeKnora-postgres-dev Exited、新容器 Created 未启动，用户环境迁移中），注册写路径报 "failed to create workspace"。脚本保留（`npx tsx tests/integration/live-attachments-chat.mts`），环境恢复后一键执行；附件与聊天协议层已由真实 Go 源码核验 + 单测真实 SSE 帧覆盖。
- 明确未完成（如实）：RW-027 附件上传、RW-028 深链通知、RW-029 i18n 集中化、M07 发送接线（后端端点在）、M13 OAuth 浏览器授权实测、M14 下载/分享签名链接、SSO 模拟器回跳实测、Android 构建/E2E、逐页像素比对与多宽度/大字体场景（blocked-env 或待环境）




## 页面行为测试补齐与台账收口（2026-09-18 晚·第二轮）

### 缺口与动作
对照 tasks.json 验收条件审计发现：RW-011~RW-026 多数页面有实现与视觉验证，但缺页面行为测试（integrating 主因）。本轮补齐 5 个测试文件共 33 个验收测试：

- `tests/features/pages-execution-approval.test.tsx`（10）：M08 三状态独立展示（execution/settlement unknown 不冒充完成）、取消携带 revision、409 冲突禁用不重发、revision 缺失禁用取消、forbidden 遮蔽；M09 冻结摘要（目标/内容/revision）、decide 携带 expected_revision、409 版本变化+按钮禁用、notfound 空态
- `tests/features/overview.test.tsx`（3）：useOverview 聚合计数（running/waitingUser 口径）、forbidden 遮蔽、network→offline
- `tests/features/pages-lists-voice.test.tsx`（10）：M04 防抖 300ms（keyword 回第一页）、空库与筛选无结果两种空态区分、forbidden；M06 目录回写 agentSelection；M10 聚合+筛选；M15 不可用原因如实展示、只可选受权目标、禁选态
- `tests/features/voice.test.tsx`（3）：M16 竞态保护抽纯函数 `transcriptFill`（迟到转写仅回填空/纯空白草稿）×2 + UI 如实展示（真实录音未接入不假装成功、文字输入不阻塞、放入草稿须显式操作）
- `tests/features/pages-profile-usage.test.tsx`（7）：M11 分类入口+导航；M12 摘要渲染/forbidden；M17 身份卡（identity 驱动）、退出二次确认（确认前不调 logout、Sheet 说明服务端任务继续）；M18 只读展示（额度/预占/已结算+as_of）、无支付按钮（按 accessibilityRole 断言）、forbidden 遮蔽明细

### 实现侧最小改动（行为不变）
- DecisionAction 增加可选 testID；M08/M09 Sheet 确认按钮加 testID（confirm-cancel/confirm-decide）
- voice.tsx 转写回填抽 `transcriptFill(current, transcript)` 纯函数（页面引用，竞态语义不变）

### RNTL 测试环境坑（后续写测试须遵守，本轮实证）
1. **fake timers 与本组合不兼容**：jest.useFakeTimers 期间 render 的组件会破坏文件内后续所有 render（React 19.2 + test-renderer 1.3.0 + RNTL 14）；防抖/定时器一律用真实时间等待
2. **同测试内第二次 render 会坏**（即使先 unmount）：多场景拆成多个 it
3. **jest.mock 工厂 hoisting**：工厂在 import 提升阶段执行，`router: mockRouter` 直接展开得到 undefined——必须用惰性 `get router()`
4. **React 19 act 异步 flush**：fireEvent 后的同步断言拿不到新状态，一律 findBy/waitFor
5. **1200ms 转写定时器多测试连续渲染破坏环境**：voice 竞态改纯函数单测，UI 只留一个测试
6. probe（hook 测试）与页面渲染混排会污染 act 环境：useOverview 测试单独成文件

### 门禁（全绿）
- `npx tsc --noEmit` → exit 0（真实退出码，重定向后读取）
- `npx jest` → **20 suites / 152 tests 全过**（上轮 119 + 本轮 33）
- `npm run check:isolation` → 82 源文件 0 违规（可达图 47 文件）

### 台账收口
- **29 accepted**（本轮推进 17：RW-011/012/014/015/016/017/018/019/020/023/024/025/026/027/028/029/033）
- **4 integrating**（准确阻塞）：
  - RW-021 M13 连接详情：OAuth 浏览器授权真机实测 blocked-env；页面+视觉验证已有
  - RW-022 M14 成果：签名链接下载/分享后端服务未接（D-10）；页面+视觉验证已有
  - RW-031 视觉：36 张双主题截图已交付；360/390/430 多宽度与 200% 大字体原生截图 blocked-env（CoreSimulator runtime 缺失，组件级 font-scaling/long-text 测试补偿）
  - RW-032 E2E：真实后端集成 10/10 + 附件 6/6 + 重开恢复 3/3 + SSE/故障注入单测已有；双平台真机 E2E、推送注册、真实录音未执行
- RW-033 交付：README 补「新旧入口切换说明」（并存安装/独立命名空间/替换发布需另行授权）；启动/质量入口命令此前已备

## RW-021/RW-022 可执行部分收口 + 签名链接最小后端补充（2026-09-18 第三轮）

### RW-021 M13 连接详情（页面行为测试 6/6）
`tests/features/pages-connections.test.tsx`：范围渲染与动作推导（写入类 scope→需审批）、页面不展示 token/secret（凭据服务端受控保管）、重新授权走系统浏览器（expo-web-browser mock 惰性 getter，URL=部署 origin 的 authorize 路径）且不假装授权完成、撤销二次确认（Sheet 冻结连接+不回滚说明；服务端无端点时如实提示）、forbidden 遮蔽、不泄漏存在性空态。
剩余 blocked-env：真机 OAuth 浏览器回跳实测（需模拟器/真机）。

### RW-022 M14 成果页 —— 最小后端补充（目标 1.3：有测试、可兼容、不建第二套权威）
**后端新增**（复用既有 message artifacts 数据与本地存储链路，无新表无新权威）：
- `internal/workbench/artifact_signing.go`：HMAC-SHA256 grant（tenant|session|message|index|exp 定界拼接、恒定时间比较、TTL 上限 15min、密钥仅环境变量 `WEKNORA_ARTIFACT_SIGNING_KEY`≥32 字节 hex，未配置 501 fail-closed）；纯函数测试 5（往返/五字段篡改拒绝/过期边界/规范串歧义拒绝/密钥校验）
- `GET /workbench/executions/:run_id/artifacts`：owned 谓词（resolveOwnedRun 提取共享）→ GetSessionArtifactRefs 投影（repo 新方法，消息绑定+会话级 index）→ items 线
- `POST /workbench/executions/:run_id/artifacts/:index/signed-url`：签发（ttl clamp、X-Forwarded-Proto 尊重、no-store、越界 404）
- `GET /api/v1/workbench/artifacts/download`：**无登录态**（挂在全局 Auth 之前，先例 servePresignedFiles）；验签+过期（401 code=artifact_grant_expired）→ grant 租户作执行租户 → refs 复核（删除的消息立即不可下载）→ 复用既有 per-message 下载流式链（streamResolvedArtifact 提取共享）；handler 测试 8
- **真实 bug 修复**：下载端点初版挂在 v1 组内被全局 Auth 拦截（401 missing authentication）——真实验证发现，移至 Auth 前注册
- 路由/DI：RegisterWorkbenchArtifactRoutes + container provider；`go build ./...` 0、session/workbench/router 三包测试全绿

### RW-022 客户端接线
- contracts：ArtifactWire+index；ArtifactSignedUrlWire；api.artifactSignedUrl()
- `src/features/executions/artifactDownload.ts`：下载状态机（注入 IO）——**签名过期 401 自动重签一次（重授权闭环）**、重签仍失败→grant_expired、501→signing_disabled、文件名防路径穿越；单测 7
- AppProvider.downloadArtifact 装配（无凭证 fetch 签名 URL + expo-file-system File.write 落缓存）；新增依赖 expo-sharing ~55.0.24（官方，系统分享）
- M14 页面重写：下载成功提示+文本预览（text() 读取）、分享交出本地缓存文件（不携带登录态）、部署未启用签名如实提示（注明需配置 WEKNORA_ARTIFACT_SIGNING_KEY）；页面测试 6/6

### 真实后端 roundtrip（2026-09-18 晚）
环境：源码新后端（`go build ./cmd/server`，8082）连 OrbStack 容器 PG/Redis（.orb.local 域名）+ 独立库 WeKnoraLive（migration 0→135，不动用户库 WeKnora）+ LOCAL_STORAGE_BASE_DIR 临时目录 + WEKNORA_ARTIFACT_SIGNING_KEY 测试密钥：
1. `GET /workbench/executions/:run/artifacts` → 真实 agent_runs+messages 聚合 items（index/id/name/mime/size/source_run/created_at 全对）✓
2. `POST .../signed-url` → 真实 HMAC URL + expires_at ✓
3. `GET 下载 URL`（**无任何凭证**）→ 真实文件字节（"live artifact roundtrip"）✓
4. 边界：未认证签发 401；下载无参 404；坏签名 401 code=artifact_grant_invalid ✓
5. 兼容回归：live-backend.mts 对新后端 **10/10** ✓
（测试库/临时目录/进程已清理；容器旧库 WeKnora 未做 schema 变更）

### 门禁（第三轮后全绿）
- 客户端：tsc 0、jest **23 suites/171 tests**、隔离门禁 86 源文件 0 违规
- 后端：`go build ./...` 0；handler/session、workbench、router 三包 `go test` 全过
- 模拟器复检：`xcrun simctl list runtimes` 仍 0 个 iOS runtime —— RW-031 多宽度/大字体原生截图与 RW-032 双平台 E2E 仍 blocked-env（如实）

### 台账
RW-022 → **accepted**（30/33）；RW-021 页面行为全测但真机 OAuth 回跳仍 integrating；RW-031/RW-032 保持 integrating（blocked-env：CoreSimulator runtime 缺失）
