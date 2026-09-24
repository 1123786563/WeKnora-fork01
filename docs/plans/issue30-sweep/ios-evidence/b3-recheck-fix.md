# B3 iOS 复验问题修复报告

- 日期：2026-09-24
- worktree：`.worktrees/issue30-sweep`（分支 `codex/issue30-mobile-office`，基线 HEAD `01c9ccf5c`）
- 输入：B3 复验报告（`ios-evidence/b3-recheck.md`）登记的 2 项 minor 问题
- 本轮证据目录：`ios-evidence/b3-recheck/fix/`
- 结论速览：问题 1 已定位双层根因并落地宿主层修复（含一项新发现的上游阻塞）；问题 2 已补跑深度扫描至完成并封印（B3 范围 0 findings）；定向测试与模拟器重复构建/启动验证全部通过。

## 问题 1：MCP ios-simulator ui backend 误报 idb unavailable

### 根因（两层，本轮均有实测证据）

**层 A——MCP 调度进程 PATH 与用户 shell 不一致（原报告登记的根因，已证实）**

- 承载 ios-simulator 插件 MCP 的调度进程（`ps ewww` PID 6256，`ZCode Helper --utility-sub-type=node.mojom.NodeService`，env 含 `ZCODE_PROCESS_LABEL=scheduler`）的环境为 `PATH=/usr/bin:/bin:/usr/sbin:/sbin`——GUI 启动的默认 PATH，不含 `/opt/homebrew/bin` 与 `~/.local/bin`。插件探测直接继承该 PATH（`/Applications/ZCode.app/Contents/Resources/glm/packages/ios-simulator-plugin/dist/lib/run.js:3-6` 用 `process.env` spawn `idb`）。
- 复现实验（本轮执行）：
  - `env -i PATH=/usr/bin:/bin:/usr/sbin:/sbin HOME=$HOME idb --version` → `env: idb: No such file or directory`，exit 127（ENOENT）——与 MCP 误报 `idb is not installed or not on PATH` 一致。
  - `env -i PATH=/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin HOME=$HOME idb --version` → 找到 idb（打印 usage）。
- 对照组：之后由 shell 环境拉起的 `zcode-cli` 代理进程（PID 6866/6867/7798 等）PATH 均为完整用户 PATH（含 `/opt/homebrew/bin`）——即问题只影响 GUI 启动的调度进程链。

**层 B——新发现：插件探测命令与 fb-idb 1.6.1 不兼容（上游阻塞）**

- 插件 `status()` 执行 `idb --version` 且要求 exit 0 才判可用（`dist/providers/ui.js:3-11` + `dist/lib/run.js:75-77` `ok = code === 0 && !timed`）。
- 本机两处 idb 客户端均为 fb-idb 1.6.1（uv 工具版 `~/.local/share/uv/tools/fb-idb`（`/opt/homebrew/bin/idb` 符号链接指向它）与 miniconda 版 `/opt/homebrew/Caskroom/miniconda/base/bin/idb`），`idb --version` 均报 `unrecognized arguments: --version`、exit 2。
- 推论：即使层 A 修复生效，`ios_ui_status.available` 仍将为 false（`exec()` 同样被 `status()` 门禁，`ui.js:55-58`）。该探测缺陷在 ZCode 应用包内，本轮不改动应用包（非最小改动）。

### 修复动作

1. **宿主 launchd GUI 域 PATH 持久化（已执行并验证）**：`launchctl setenv PATH "$PATH"` → `launchctl getenv PATH` 返回完整用户 PATH（含 `/opt/homebrew/bin`）。此后经 launchd 拉起的 GUI 进程（含下次重启后的 ZCode）继承该 PATH。
   - 生效边界（如实说明）：对**当前已在运行**的调度进程无效——需重启 ZCode 才能让 ios-simulator MCP 进程拿到新 PATH；重启 ZCode 会终止本工作流会话，故本轮未做，属后续动作。
   - 跨重启持久化需 `sudo launchctl config user path …`（需 sudo，超出本轮权限，未执行，留待用户决定）。
2. **层 B 属上游问题**：不改 ZCode 应用包；已在报告中固化根因与证据，避免后续轮次重复排查。idb CLI 直调（`idb ui describe-all --udid …`）继续作为 AX 校验路径，本轮复验中再次全程可用（见第 4 节）。

## 问题 2：Mimosa 预提交扫描 scanner_enobufs 无结论

### 修复动作：补跑深度扫描至完成并封印

- 命令：Mimosa MCP `security_scan_start`（depth=deep，project=本 worktree，focusFiles=B3 变更的 24 个代码文件：`apps/mobile/src/**`、`packages/mobile-core/src/{inbox,task-office,material}/**`、`packages/api-client/src/mobile/**`），jobId `scan-job-mufi9yzh-0a3d664f699fed2b`，attempt 1 → `security_scan_status` 确认 **completed**。
- 结果（`~/.mimosa/security-scans/project-0abb52f1c9cae31b539b16fe/scan-2026-09-24T12-27-16.425Z-f50edfe3ac10/`）：
  - scanId：`scan-2026-09-24T12-27-16.425Z-f50edfe3ac10`
  - 封印：`sha256:8507dabcad5ce947b6bace8b3065e0a9f0c142586968d5913b45cc8aa2f328e5`
  - findings：241（0 business-logic candidate）；**B3 范围（apps/mobile / packages/mobile-core / packages/api-client/src/mobile）findings = 0**，241 条全部为 B3 范围外的仓库既有静态发现（frontend/、apps/web、cmd/desktop、docs/poc 等，抽样见 report.md）。
  - 各阶段全部 completed：threatModel / findingDiscovery（241）/ validation / pathAnalysis（29114 函数、37802 调用边）/ reporting；依赖扫描 completed（1135 包，离线 advisory 命中 12 包 / 67 条）。
  - 诚实边界：coverage 自评 `completeness: partial`、runStatus `inconclusive`——唯一缺口为声明的"调用图部分不完整（动态派发/超分析规模）"，是静态分析固有边界，**不是**中途失败。与提交时 `scanner_enobufs`（扫描器错误、无任何结论）性质不同。
- 表述约束（延续原登记口径）：本扫描结果是"完成并封印的静态深度扫描、B3 范围零发现"，**不**宣称"项目级完整安全审计通过"；241 条范围外发现的处置不在本轮 minor 修复范围。

## 定向测试（B3 mobile 全套 TS 测试）

- 命令（与 B3 报告口径一致）：`pnpm exec tsx --test "packages/domain/src/mobile/*.test.ts" "packages/mobile-core/src/**/*.test.ts" "packages/api-client/src/mobile/*.test.ts" "apps/mobile/src/**/*.test.ts*"`
- 结果：`# tests 518 / # pass 509 / # fail 0 / # skipped 9`（skip 均为 opt-in integration 用例）——与 B3 基线（518/509/0/9）完全一致，无回归。
- `pnpm run typecheck:mobile` → exit 0 PASS。

## 模拟器重复构建 + 启动验证（新证据 → `fix/`）

构建命令与复验轮相同（增量，同 `-derivedDataPath build`）：

```
xcodebuild -workspace WeKnora.xcworkspace -scheme WeKnora -configuration Release \
  -sdk iphonesimulator \
  -destination "platform=iOS Simulator,id=0A38DB71-CEE1-4A89-8B19-6DD24A3E85FC" \
  -derivedDataPath build ARCHS=arm64 ONLY_ACTIVE_ARCH=YES build
```

| 步骤 | 结果 | 证据 |
| --- | --- | --- |
| Release 增量构建 | **BUILD SUCCEEDED**；无 `error:`；55 条 `warning:`（全为 React-Fabric/gesture-handler/expo 等三方头文件，与复验轮同类） | `fix/xcodebuild-release.log` |
| 产物 | `main.jsbundle` 3,672,281 字节（20:27 生成；复验轮记录为 5.47MB@20:14，源码零变更下的度量差异，行为验证全通过） | — |
| 安装/启动 | `simctl install` OK；`simctl launch` → PID 77246 | — |
| 首屏 | AX：`Sign in to WeKnora` / `Sign in` / `Continue with single sign-on`，与复验轮一致 | `fix/01-first-screen.png` + `fix/ax-first-screen.txt`（idb CLI） |
| `/inbox` warm deep link | 「请先登录并激活空间，再查看行动通知。」 | `fix/02-inbox.png` + `fix/ax-inbox.txt` |
| `/tasks/materials` warm deep link | 「返回任务」「任务材料」「刷新材料」「尚无材料索引」「请先登录并激活空间，再查看任务材料。」 | `fix/03-tasks-materials.png` + `fix/ax-tasks-materials.txt` |
| 冷启动 deep link（terminate → openurl inbox → 10s） | 「请先登录并激活空间，再查看行动通知。」直达 inbox | `fix/04-coldlaunch-inbox.png` + `fix/ax-coldlaunch-inbox.txt` |
| 启动日志 | 595 行，`fatal/SIGTRAP/crash/RCTFatal` 关键字 0；4 行 " error " 全为 iOS 27 `com.apple.uiintelligencesupport` XPC 系统噪声（对所有 app 通用） | `fix/app-launch-log.txt` |

截图均为 1206×2622 有效 PNG（`sips` 校验）；`02-inbox.png` 与 `04-coldlaunch-inbox.png` 字节相同属预期（同一 gate 页面）。

## 未验证 / 遗留项

1. `ios_ui_*` MCP 工具在当前会话仍报 unavailable：层 A 修复需重启 ZCode 才进入 MCP 进程；层 B（`idb --version` exit 2 探测缺陷）在 ZCode 应用包内，需上游修复或提供支持 `--version` 的 idb 构建。本轮未重启 ZCode（会终止本工作流）。
2. `sudo launchctl config user path`（跨重启持久化）需 sudo，未执行。
3. 241 条 B3 范围外静态 findings 的逐条处置不在本轮 minor 修复范围。

## 结论

两项 minor 问题均闭合到本轮可达边界：问题 1 完成双层根因定位（含新发现的上游探测缺陷）并落地 `launchctl setenv PATH` 宿主修复（已验证生效于 launchd 域）；问题 2 以完成并封印的深度扫描替代此前的无结论状态，B3 范围零发现。定向测试（518 用例 0 fail）与模拟器重复构建/启动/路由/冷启动验证全部通过，与 B3 复验基线一致，无回归。
