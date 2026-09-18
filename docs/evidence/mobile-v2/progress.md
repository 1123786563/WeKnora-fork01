# MX 执行台账 · mobile-v2（唯一进度事实源）

仓库：1123786563/WeKnora-fork01 · 实施分支：codex/expo-mobile-v2（worktree .worktrees/expo-mobile-v2）
基线：main `6a70c35a`（2026-09-18）· 设计包：docs/design/mobile-v2（v2.0，task-index sha256 1c910fd8…）
模式：单实现者串行（D-001）+ 每任务独立 review 子代理。状态机：pending→ready→implementing→review→integrating→accepted（fixing/blocked-* 例外）。

| 任务 | 状态 | BASE→HEAD | 测试（cmd·exit·发现/通过） | Review | 证据 |
|---|---|---|---|---|---|
| MX-001 | accepted | 6a70c35a→04176329 | tsx --test mx-001.test.ts · RED exit1 → GREEN exit0 · 1/1（含基线 kind 不变量） | R1 FAIL(2×P1)→修复→复核 PASS | MX-001.md |
| MX-002 | accepted | 64b8da68→52c87b1c | test:mobile-v2 · RED exit1(13+真实偏差) → GREEN exit0 · 2/2；test:shared 579/579；prebuild 双平台 exit0 | 规格 PASS 6/6 · 质量 APPROVED（4×P2 非阻塞，见注） | MX-002.md / native-baseline.md |
| MX-003 | accepted | 52c87b1c→2bd4c682 | mv2 3/3 · shared 579/579 · go workbench+handler ok · RED=3 真实断言失败 | 规格 PASS 5/5 · 质量 APPROVED（3×P2 已承接，D-016） | MX-003.md |
| MX-004 | accepted | 2bd4c682→98002709 | mv2 5/5 · go handler 全绿 · RED=parser 对真实 v2 字节崩溃 | 规格 PASS 5/5 · 质量 APPROVED（4×P2：3 项由 MX-006 承接） | MX-004.md |
| MX-005 | accepted | 98002709→c9505e26 | mv2 7/7 · shared 589/589 | R1 FAIL→修复→复核 FAIL(注册表回归)→R2 修复→终核 PASS | MX-005.md |
| MX-006 | review | d8550bde→d9cfc83c(+fix) | mv2 8/8 · shared 589/589 · domain 5/5 · adapter 2/2 | R1 单点 FAIL(.gitignore 登记)→P2/P3 已修复→待 scoped 确认 | MX-006.md |
| MX-007 | review | c9505e26→5e6e3c29 | mv2 8/8 · shared 589/589 · 对比率 ACTIVE 全过(disabled 豁免) · typecheck 14=基线 | 审查进行中 | MX-007.md |
| MX-008 | accepted | 5e6e3c29→12a9c4da(含fix) | mv2 10/10 · 挂载 4/4 · typecheck 14=基线 | R1(2×P1)→修复→复核 PASS | MX-008.md |
| MX-009 | accepted | 08a5c9cb→12a9c4da | mv2 10/10 · 挂载 4/4 · happyAuthRequests=0 | 规格 PASS · 质量 APPROVED（P2 锁账澄清→D-023） | MX-009.md |
| MX-010 | accepted | 12a9c4da→95973585(+fix2) | mv2 13/13 · tsc 14=基线 | R1 FAIL→修复→复核 PASS（P2-1 probe 二修随 MX-012 R1 批次） | MX-010.md |
| MX-011 | accepted | 35ad2c0d→95973585(含fix) | mv2 12/12 · shared 590/590 · frozen B/false/0 | 规格 PASS 7/7 · 质量 APPROVED（P2 已处置，D-024） | MX-011.md |
| MX-012 | fixing→review | 95973585→202225bb(+fix) | mv2 13/13 · 挂载 6/6（含 single-flight 竞态回归） · tsc 14=基线 | R1 FAIL(P1 密钥竞态)→已修复→待复核 | MX-012.md |

## 首次更新（2026-09-18，MX-001）

- REPO/DESIGN 定位、基线与环境见 preflight.md；G01-G12 对账见 current-state-matrix.md。
- 首批就绪：MX-001（本任务）→ 完成后 MX-002、MX-003、MX-007 就绪（D-001 串行）。

## 注记（2026-09-18，MX-002 审查 P2 处置）

- P2-3「test:shared 偶发失败」已定位：审查代理运行时段恰逢 MX-003 并发修改 snapshot 契约与消费方 fixture（同一批失败断言随后在 2bd4c682 修复）。非 flaky，登记澄清。
- P2-1 probe 自研 satisfies 的 ^0.x 语义：当前矩阵无 ^0.x range 不触发；若未来出现 0.x 期望再修（记录于本注）。
- P2-2 libsodium 包名精确为 @more-tech/react-native-libsodium@1.5.6；P2-4 RED 阶段平台顺序为过程笔误。均不阻塞。

## 注记（2026-09-18，注册表回归教训）

mx-005 fix 轮把 interactions.ts 的 kind 误改 modify，违反 mx-001 基线守护（该文件相对基线 6a70c35a 恒为 create）；复核代理实跑抓获。R2 已修复并复跑 7/7。纪律固化：**任何 file-ownership.json 变更后必须重跑 test:mobile-v2**（守护测试就是为此存在）。

| MX-013 | accepted | 6d94a204→0ff12076 | frozen Go ok（含派生断言）· mv2 16/16 | R1 FAIL(P1 虚构列)→修复→复核 PASS | MX-013.md |
| MX-014 | review | cd10655d→(待提交) | mv2 15/15 · shared 590/590 · tsc 14=基线 | 待独立review | MX-014.md |

| MX-014 | accepted | cd10655d→964bf811(+fix2) | mv2 18/18（含真实翻页/重放去重断言） | R1→修复→PASS/APPROVED（证据缺口 R2 补齐：frozen 补真实翻页+重放页） | MX-014.md |
| MX-017 | accepted | 336b0ffb→5e53cb6c(含fix) | mv2 19/19 · tsc 11=基线 | R1 P2→修复→复核 PASS（类型层防回归设计获确认） | MX-017.md |
| MX-016 | accepted | 964bf811→424cfe97 | mv2 17/17 · shared 590/590 · tsc 11=基线 | 规格 PASS · 质量 APPROVED（双突变杀优认证） | MX-016.md |
| MX-022 | accepted | 424cfe97→620ddcb8 | mv2 19/19 · frozen 撤权零可见/禁问 | 规格 PASS · 质量 APPROVED（突变杀优认证） | MX-022.md |
| MX-030 | accepted | 620ddcb8→0119d82a(含fix) | mv2 19/19（含离线撤销韧性）· tsc 11=基线 | R1(2×P2)→修复→复核 PASS（代码/测试）；R2 记录项修正（D-027） | MX-030.md |
| MX-015 | accepted | 0119d82a→082489e6 | mv2 21/21 · frozen 未就绪 0/保留 · tsc 11 | 规格 PASS · 质量 APPROVED（双突变杀优认证；P3 记录项本批修正） | MX-015.md |
| MX-018 | accepted | 082489e6→16adf375 | mv2 21/21 · frozen stop_pending/停止待确认/false · tsc 11 | 规格 PASS · 质量 APPROVED（kill1/2b/2c 被抓；P3 观察登记） | MX-018.md |
| MX-019 | accepted | 16adf375→188da5fd | mv2 22/22（HEAD 集）· frozen 陈旧确认 0/refresh · tsc 11 | 规格 PASS · 质量 APPROVED（双突变被抓；P3 观察登记） | MX-019.md |
| MX-031 | accepted | 188da5fd→569eb068 | mv2 23/23 · frozen 120/38/1260/0 · tsc 11 | 规格 PASS · 质量 APPROVED（突变红） | MX-031.md |
| MX-024 | accepted | 569eb068→52a3627d | mv2 24/24 · frozen 0/[] · tsc 11 | 规格 PASS · 质量 APPROVED（2/2 突变红） | MX-024.md |
| MX-021 | accepted | 52a3627d→c600224e | mv2 25/25 · go 全绿（路由+DI 四环核验）· tsc 11 | 规格 PASS · 质量 APPROVED（突变红） | MX-021.md |
| MX-020 | accepted | c600224e→e3639894 | mv2 26/26 · frozen 0/编辑文本/authorizing · tsc 11 | 规格 PASS · 质量 APPROVED（双向突变验证） | MX-020.md |
| MX-023 | accepted | e3639894→0dda83ba | mv2 27/27 · frozen 0/[] · tsc 11 | 规格 PASS · 质量 APPROVED（突变红） | MX-023.md |
| MX-025 | accepted | 0dda83ba→ddfe1831(含fix) | mv2 frozen false/true（杀伤复验：双守卫突变被抓） | R1→修复→复核 PASS（变异验证） | MX-025.md |
| MX-028 | accepted | e9ce0fc2→339bea50 | mv2 29/29 · frozen 编辑文本/0 · tsc 11 | 规格 PASS · 质量 APPROVED（突变被杀） | MX-028.md |
| MX-026 | accepted | 339bea50→b9e00fda | mv2 30/30 · frozen platform/0 · tsc 11 | 规格 PASS · 质量 APPROVED（突变被杀） | MX-026.md |
| MX-029 | accepted | b9e00fda→ddfe1831(含fix) | frozen true/0（ports 级断言+变异验证）· go ok | R1→修复→复核 PASS | MX-029.md |
| MX-032 | accepted | 9496ec29→1e015177 | mv2 32/32 · go caps ok · frozen unavailable/missing_cancel_evidence | 终审 PASS/APPROVED（收尾批） | MX-032.md / profile-gates.md |
| MX-033 | accepted | 1e015177→32786008 | mv2 40/40（recovery 3+security 4）· frozen 1/1 · go admission ok | 终审 PASS/APPROVED（收尾批） | MX-033.md / fault-matrix.md |
| MX-034 | blocked-env | ddfe1831→09b56a52 | 显式命令 exit=1 blocked-env（正确的红）· mv2 40/40（E2E 独立） | 待独立review（收尾批） | MX-034.md / native-e2e.md |
| MX-035 | partial | 09b56a52→a799d30c | 静态层 frozen 双空（41/41，含真缺口修复）· 设备层 pending · tsc 11 | 待独立review（收尾批） | MX-035.md / accessibility-performance.md |
| MX-036 | accepted | a799d30c→fa407fed(含R1/R2) | frozen 真实回归（~16s，split 解析版杀伤复验）· mv2 42/42 | R1 空门 P1→修复→终核 PASS（minor 虚报更正） | MX-036.md / release-report.md |

## 接续说明（2026-09-18 本轮收口）

- 分支 codex/expo-mobile-v2（worktree .worktrees/expo-mobile-v2），HEAD `202225bb`，工作区干净，未 push。
- MX-001–012 全部提交；accepted：001–009、011；**review 在途**：MX-010（P1 修复待复核）、MX-012（待审查）——复核代理已派发，结论到达后按 R1 模式处置并更新本表。
- 下一就绪（依赖全满足）：**MX-013**（工作台聚合读模型+首页+入口接线：Go service/handler/路由/DI + SDK + HomeScreen + 按 D-023 锁转移完成 _layout 入口切换）；随后 MX-014/016/017/022/030。
- 常用命令：`pnpm run test:mobile-v2`（13/13 基线）、`pnpm run test:shared`（590/590）、`cd apps/mobile && pnpm exec tsc --noEmit | grep -c "error TS"`（14=基线）、挂载套件 `pnpm --filter @weknora/mobile exec vitest run -c vitest.mobile-v2.config.ts`（4/4）、Go `go test ./internal/workbench/ ./internal/handler/session/ ./internal/application/service/workbench/`。
- 纪律提醒：file-ownership.json 任何变更后必须重跑 test:mobile-v2（mx-001 基线守护）；typecheck 证据用全量 grep -c 禁止 tail；提交被 Mimosa 全项目误报间歇拦截时按 D-022 处置（重试，不改文案换绿灯）。
- 证据索引：本目录 progress.md（唯一进度事实）、decisions.md D-001–D-024、current-state-matrix.md、file-ownership.json（~200 锁）。

## 接续说明（2026-09-18 第二轮收口）

- HEAD `463a61db`（MX-030），工作区干净，未 push。**已提交 18/36**：MX-001–014 accepted（014 证据补齐后视为 accepted）；MX-016（424cfe97）/MX-017（d782cc4f+fix 5e53cb6c）/MX-022（620ddcb8）审查代理已派（结论到达后按 R1 模式处置）；MX-030（463a61db）待派审查。
- **下一就绪**（016/017/022 已 accepted）：**MX-015**（新建任务流，deps 全满足——含 M05/M06 路由挂载与提交协调器接线）→ MX-018 → MX-019 → MX-024 → MX-031；MX-021 需 MX-019。
- 回归基线：mv2 **19/19**、shared **590/590**、挂载 9/9、apps/mobile tsc **11=新基线**（D-026；app/5、CommandPalette/3、SessionsList/2、useNavigateToSession.test/1 为 legacy 遗留）、go 全量 100 包 ok。
- 纪律重申：注册表变更后必跑 test:mobile-v2（mx-001 守护已两次抓住回归）；证据命令全量计数禁 tail；提交被 Mimosa 全项目 i18n 误报拦截时按 D-022 重试（本轮 1-3 次重试均通过）。
- 修复轮历史：审查体系已抓 6 个 P1（pending wire 空 action、注册表 kind 回归、Sheet 键盘/返回焦点、SSO state 契约、AEAD 密钥竞态、overview 虚构列）——全部修复并有回归测试。

## 接续说明（2026-09-18 第三轮收口——36/36 已全部处置）

- HEAD `74d3337b`，工作区干净，未 push。**36 项全部处置完毕**：29 项 accepted（001-026/028/030/031，含 025/029 修复轮）+ MX-027 not-activated（remote 条件边未激活——manifest 强制关闭）+ MX-034 blocked-env（编排+探测交付，正确的红）+ MX-035 partial（静态层 frozen 通过/设备层 pending）+ MX-032/033/036 已提交待收尾批复核（代理在途，结论到达后按 R1 模式处置并更新 acceptance-index）。
- 最终回归基线：**mv2 42/42、shared 590/590、挂载 9/9、go 99 包 ok（1 项上游轮既有 flaky 登记）**、tsc **11=新基线**。
- 发布裁决：core=releasable-with-conditions（设备证据+收尾批复核两条件）；globalAllPassed=false（如实）。
- 待办移交：①收尾批复核结论登记（acceptance-index review-pending→accepted）②解除 blocked-env 三要素后执行 MX-034 配方+MX-035 矩阵采集回填③MX-027 若激活 remote profile 需 W17-24/W26 依赖链评估。
- 纪律提醒不变（D-027 替换必须 assert；注册表变更后必跑 mv2 守护；Mimosa 拦截按 D-022 重试）。

## 接续说明（2026-09-18 第四轮——设备解封会话）

- 未提交改动：app.config.ts / index.ts / origin-storage.ts / 三处 LoginScreen key / tsconfig / decisions(D-028) / native-e2e 第四轮记录 / artifacts/e2e 过程截图；已验证 mv2 42/42、tsc 13=新基线（D-028）。
- 设备成果：Android+iOS dev client 均构建成功并运行；本地栈+测试账户就绪；三项真实缺陷修复（D-028）；产品树（sources/app）已在设备可达。
- **下一安全动作**：定位 server 屏 Continue 无导航（疑 auth loading 挂起或 gate/replace 竞态）→ 打通后按 native-e2e.md 配方采集 18 页矩阵并回填 visual-baseline.json → MX-034/035 升级。
- 运行态：Metro 8083 已重启存活（nohup+disown，/tmp/metro6.log）、emulator-5554 在跑、iPhone 17 Pro booted、iOS app 产物在 ios/build/DerivedData。8085 端口属其他任务（apps/mobile-next）勿动。

## 接续说明（2026-09-18 第五轮——设备解封第二轮修复后收口）

- HEAD `eee89daa`，工作区干净。本轮设备会话累计修复 **5 项真实缺陷**（D-028 三项 + D-029 两项，全部由 Android 真机实测发现）+ gate loading 中间态；插桩已清理。
- **设备状态**：Android dev client（test36 模拟器 emulator-5554）冷启动全链贯通至 **M01 产品登录屏**（React 状态/按钮可点击均已插桩验证）；iOS WeKnora.app 已产出待安装（iPhone 17 Pro booted）；Metro 在 8083（EXPO_ROUTER_APP_ROOT=./sources/app）；本地后端+e2e_tester 账户就绪。
- **唯一剩余单点**：Sign in 按钮的 raw `input tap` 进入系统输入队列但不触发 RN submit（状态正确，非代码缺陷；模拟器 input/RN Fabric 特性）。**下一轮第一动作**：`/Applications/Maestro.app/Contents/MacOS/Maestro test tests/mobile-v2/maestro/flows/core.yaml --platform android`（tapOn 可达性驱动）或先跑 `Maestro --version` 完成首启驱动下载。
- 打通后：登录→索引重定向→/product 产品壳 M03→四 Tab 截图→明暗/宽度/大字体矩阵→回填 visual-baseline.json→升级 MX-034/035。
- 回归基线：mv2 42/42、shared 590/590、tsc **13**（D-028：11+2 legacy '/dev' 死链）。
