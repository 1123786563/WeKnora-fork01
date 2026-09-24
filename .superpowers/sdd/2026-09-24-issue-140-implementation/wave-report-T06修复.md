# T06 修复任务报告：真实环境缺陷 D1/D2/D3（Issue #148）

- **BASE**: `7cbad8941`（integration 记录基线）
- **HEAD**: `52ae599b6`
- **提交**（每缺陷一个 + 复验修一个）：
  1. `51606b0e0` fix(miniprogram): D1 官方构建产物可被 DevTools 加载——TDesign 组件以 dist 内绝对路径引用
  2. `bfe9a7c85` fix(miniprogram): D2 产物页 t-button 在真实运行时恢复点击与属性——改用 JSX 触发 Taro 三方组件属性/事件收集
  3. `d1c67c622` fix(miniprogram): D3 受保护产物临时清理真实成立——可写目录私有副本 + 异步 unlink
  4. `52ae599b6` fix(miniprogram): D3 复验修真实运行时两处契约——USER_DATA_PATH 取自 wx 全局、副本扩展名收尾
- **Worktree**: `/Users/wuyongjun/.codex/worktrees/issue-140-t06-gui-fix/WeKnora-fork01`（未 push）

## 0. 结论速览

| 缺陷 | 根因 | 修复 | 单测 | 真实 DevTools 复验 |
| --- | --- | --- | --- | --- |
| D1 官方构建白屏 | usingComponents 用 npm 包名，被 Taro MiniPlugin 解析为 node_modules 绝对路径并重写成 `/npm/.pnpm/…/node_modules/…`；DevTools 拒绝含 node_modules 段的组件路径，且 dist/npm 从不产出 | `config/index.ts` 以 `mini.copy` 把 `tdesign-miniprogram/miniprogram_dist`（realpath 解 pnpm 链接）拷入 `dist/npm/tdesign`；页面 usingComponents 改引用 `/npm/tdesign/button/button` | `tests/build-output.test.mjs` D1 断言 | **PASS**：官方构建产物零手工补丁启动 |
| D2 按钮无事件 | Taro 4.2.1 构建期三方组件属性/事件收集器（webpack5-runner `TaroNormalModulesPlugin`）只识别 JSX 产物 `_jsx("t-button",…)` 与 `React.createElement` 成员调用；页面里裸标识符 `createElement("t-button",…)`（babel 直出原样）不被收集 → base.wxml 的 t-button 模板无 `bindtap="eh"` 也无任何属性绑定 | 页面改 JSX 书写 `<t-button …>`（`t-button.d.ts` 提供类型并记录根因）；编译产物模板恢复 `bindtap="eh"` + block/disabled/loading/theme/size/ariaLabel/customStyle 绑定 | `tests/build-output.test.mjs` D2 断言 | **PASS**：真实点击触发 signed-url POST + 下载到达服务器；属性/主题/CSS 变量真实渲染 |
| D3 清理静默失败 | downloadFile 临时文件（http://tmp/…）unlink/unlinkSync 一律 permission denied；旧代码 finally unlinkSync 被吞 | 下载校验后 `copyFile` 到 USER_DATA_PATH 私有副本 → `openDocument(副本)` → 异步 `unlink(副本)`；清理失败不再静默（无业务错误时抛可读错误，有则保留原始错误）；复验修两处真实契约（见 §4） | `tests/artifact-cleanup.test.mjs` 4 场景 + assembly 契约更新 | **PASS**：copyFile→openDocument.success→unlink 全链路实测，USER_DATA_PATH 无 wk-open-* 残留 |

## 1. 根因分析（systematic-debugging，证据可复现）

### D1
- 复现：`WEKNORA_API_ORIGIN=http://127.0.0.1:<port> pnpm build:weapp` 后
  `dist/subpackages/execution/artifact/index.json` = `{"usingComponents":{"t-button":"/npm/.pnpm/tdesign-miniprogram@1.17.0/node_modules/tdesign-miniprogram/button/button",…}}`，且 `dist/npm` 不存在。
- 机制（node_modules 源码定位）：`@tarojs/webpack5-runner/dist/plugins/MiniPlugin.js` `compileFile` 对非 `./`、`/` 开头的引用调 `getNpmPackageAbsolutePath` 解析为 node_modules 绝对路径；`adjustConfigContent` 把首个 `node_modules` 段替换为 `npm` 后截取 → pnpm 布局下产出含 `node_modules` 段的路径（且组件从未拷入 dist）。实测报告（task-6-live-validation）已证 DevTools 拒绝该路径（即使把文件放到位）。
- 修复选用与 Wave-1 解锁实验同构但**源码化**的方案：Taro `mini.copy`（运行时支持：`MiniWebpackPlugin.getCopyWebpackPlugin` 消费 `config.copy`；`IMiniAppConfig` 类型缺该字段属类型缺口，用展开注入并在注释说明）。

### D2
- 证据：基线构建产物 `dist/base.wxml` 的 `tmpl_0_t-button` 模板只有 `id`/`data-sid`，无 `bindtap`、无属性绑定。
- 机制（源码定位）：`TaroNormalModulesPlugin` 用 acorn-walk 收集三方组件 props/事件——只匹配 MemberExpression `.createElement` 与 `_jsx/_jsxs` 标识符两种调用形态。babel 对本页 TSX 的产物是 `_jsx(View,…)`（收集）与**裸标识符** `createElement('t-button',…)`（不匹配）→ `componentConfig.thirdPartyComponents.get('t-button')` 的 attrs Set 恒空 → 模板生成（`@tarojs/shared` `buildThirdPartyAttr`：`onXxx → bindx="eh"`、其他 attr → `attr="{{i.attr}}"`）全数缺失。
- Wave-1 模板级补 `bindtap="eh"` 仍失败的原因：React 侧 `onTap` 经 `setEvent` 注册的监听器依赖完整数据通道（hydrate/attrs 对齐），仅补事件绑定时运行期 props 序列化与模板不匹配，事件对象到达但派发链断裂。官方机制（JSX 产物被收集）下整条链成立：模板 `bindtap="eh"` → 页面 `eh` → `TaroElement.dispatchEvent('tap')` → `setEvent('onTap'→'tap')` 监听 → React handler。

### D3
- 证据（Wave-1 实测）：`unlinkSync(http://tmp/…pdf)` → `permission denied`，403 字节文件残留；单测替身当时无该契约。
- 修复设计：运行时临时目录不归应用管理（删除被拒），应用可写可删的只有 `USER_DATA_PATH`。改为“私有副本”生命周期：copy → open → unlink，UI 承诺对“我们创建的文件”真实成立；运行时管理的下载临时文件不再声称清理（测试契约同步更新）。

## 2. TDD 证据（RED → GREEN）

**RED（修复前，全套 44/59，15 个失败全部为 D1/D2/D3 表达）**：
- `tests/build-output.test.mjs`（新增，构建产物断言）：
  - D1 `引用不得含 node_modules 段（当前：/npm/.pnpm/tdesign-miniprogram@1.17.0/node_modules/tdesign-miniprogram/button/button）`
  - D2 `t-button 模板必须把 tap 绑定到 Taro 事件通道 eh`
- `tests/artifact-cleanup.test.mjs`（新增，替身复现平台契约：tmp 路径 deny unlink）：
  - `必须先把下载临时文件复制进可写目录 (0 !== 1)`
  - `预览失败也要删除私有副本 (0 !== 1)`
  - `Missing expected rejection`（清理失败被静默吞——D3 不诚实行为的直接表达）
- 9 个既有 assembly 用例按旧契约断言 removedFiles=[tmp 路径]，在新契约替身下一并转 RED（属同一缺陷的表达，随修复更新为新契约）。

**GREEN（修复后）**：`pnpm --filter @weknora/miniprogram test` → `# tests 59 / # pass 59 / # fail 0 / # skipped 0`。

**复验中的第二、三轮 TDD**（真实 DevTools 复验发现替身失真，见 §4）：
- 替身还原真实契约（Taro 无 env、wx 全局提供 USER_DATA_PATH）→ 3 个用例 RED（TypeError 路径）→ 修 `files.ts` 读 `wx.env` → GREEN。
- 新增断言 `副本文件名必须以原始扩展名结尾` → RED（随机后缀在扩展名之后）→ 修 `toUserCopyPath`（后缀前置）→ GREEN（59/59）。

## 3. 验证命令与输出摘要（全部实际执行）

```
pnpm --filter @weknora/miniprogram test        # 59/59 pass, 0 fail（53 基线 + 6 新增）
pnpm --filter @weknora/miniprogram build:weapp # Compiled successfully（size/async-chunk 警告为既有基线）
pnpm --filter @weknora/miniprogram typecheck   # 13 errors，全部 src/features/account/pages.tsx（CommercialSummary 基线豁免，未新增）
git diff --check                               # clean
```

构建产物断言（D1/D2）在 build 后运行：
```
dist/subpackages/execution/artifact/index.json → {"usingComponents":{"t-button":"/npm/tdesign/button/button","comp":"../../../comp"}}
dist/npm/tdesign/button/button.{wxml,js,json,wxss} 存在
dist/base.wxml → <t-button block="{{i.block}}" disabled="{{i.disabled}}" loading="{{i.loading}}" theme="{{i.theme}}" size="{{i.size}}" ariaLabel="{{i.ariaLabel}}" customStyle="{{i.customStyle}}" bindtap="eh" …>
```

## 4. 真实 DevTools 复验（本轮核心验收）

环境：WeChat DevTools CLI `2.02.2608070`，模拟器基础库 3.16.3，tourist appid，官方 `build:weapp` 产物**零手工补丁**，`cli auto --auto-port 9422` + miniprogram-automator（仓库外安装）。隔离 Lite 服务器（SQLite/本地存储/内存流/一次性密钥）+ 两个一次性账号。

**端口偏差声明**：调度分配 57804，但实测该端口已被并行 T09-web 会话的服务器（cwd `issue-140-t09-web`，DB `/private/tmp/weknora-t09-web.*`）占用（2026-09-25 04:22 起 LISTEN）。未动他方进程，改用空闲回环端口 **57844** 并以该 origin 重建小程序。隔离性不受影响（独立 DB/存储/密钥/账号），仅端口号与分配值不同。

| GUI/运行时步骤 | 结果 |
| --- | --- |
| 官方构建产物加载启动（无任何手工补丁） | **PASS** — entry `pages/home/index` 正常渲染（此前 D1 白屏）；`shot-01-entry-official-build-boots.png` |
| GUI 登录（邮箱+密码+同意→登录并继续） | PASS — `POST /api/v1/auth/login` 200，进入工作空间页 |
| 任务 tab | PASS — fixture run `T06 修复实测` 由应用 `listTasks` 渲染 |
| 任务行→执行详情→查看产物 | PASS — 产物卡片 `t06d-fix.pdf · application/pdf · 593 B`；t-button 渲染且**全部属性真实生效**（渲染 WXML：`block="true" theme="primary" size="large" custom-style="--td-brand-color:var(--wk-…)" aria-label="打开或保存 t06d-fix.pdf"`） |
| **真实点击 t-button**（automation `element.tap()` 于 TDesign 内部原生 `.t-button`，人类点击路径：内层 button→TDesign `catch:tap`→`triggerEvent('tap')`→宿主 `bindtap="eh"`） | **PASS** — 服务器收到 `POST …/artifacts/0/signed-url`（04:43/04:50 两轮，与 tap 时间戳对齐）+ `GET …/artifacts/download?…` 200/593 字节；D2 判据（GUI 点击触发 signed-url POST）达成 |
| 打开与清理（D3） | **PASS** — wx 调用埋点：`copyFile http://tmp/… → http://usr/wk-open-t06d-fix-<rand>.pdf` → `openDocument showMenu:true` → **`openDocument.success`** → `unlink`；`USER_DATA_PATH` 事后仅剩系统文件 `miniprogramLog`，**无 wk-open-* 残留**（`evidence/gui-success-chain.txt`） |
| SHA-256 三方比对 | **PASS** — stored=`684fe492…a098` = 真实运行时 wx.downloadFile 字节 = `POST …/artifact-versions/t06dfixver01/signed-url` 的 `data.digest` |
| 危险通知（失败路径 GUI 元素） | 真实失败点击曾触发 Notice（含“重新获取”指引文案）渲染；修复后成功点击无任何错误通知 |
| 跨租户/无凭据/过期/篡改（HTTP 层复核） | PASS — tenant-B 对 A 的 run 签名 404；无凭据 mint 401；`ttl_seconds=1` 2 秒后 redeem 401 `artifact_grant_expired`；篡改 `message_id` 401 `artifact_grant_invalid`；有效签名 URL 免凭据 redeem 200 |

复验本身发现并修复的两个真实契约问题（第 4 个提交）：
1. `Taro.env.USER_DATA_PATH` 在真实运行时是 **undefined**（`@tarojs/api` 的 Taro 对象无 env，weapp 插件 `initNativeApi` 只注入 API 不复制 `wx.env`；`.d.ts` 类型是假的）——单测替身曾错误地在 Taro 对象上提供 env 掩盖了它。改为读 weapp 全局 `wx.env`，替身还原真实契约。
2. `wx.openDocument` 按文件扩展名识别类型：私有副本文件名的随机后缀必须放在扩展名之前（同内容的合法 PDF，无 `.pdf` 结尾即报 `filetype not supported`；证据：tmp 路径直接 openDocument 同字节成功、usr 路径带后缀失败→修后成功）。

**证据**：`evidence/gui-success-chain.txt`（调用链+三方摘要+探针，签名/凭据已擦除）、`evidence/server-log-key-lines.txt`（服务器关键行，signature 已擦除）、`evidence/gui/`（截图 4 张、gui-results.json、tap-time.txt）。凭据/密钥/临时目录（/tmp/wk-t06dfix）已全部销毁。

## 5. 单测覆盖边界声明

- `build-output.test.mjs` 只验证**构建产物内容**（页面 JSON 路径、组件文件存在、模板事件/属性绑定）；**不覆盖 DevTools 运行时行为**——真实判据是本轮 DevTools 复验（已 PASS）。dist 不存在时 skip（先 build 再 test）。
- `artifact-cleanup.test.mjs` 覆盖服务/适配器层的文件生命周期（副本创建、失败清理、清理失败不静默、401 无副本）；平台契约由 `taro-stub.mjs` 承载（tmp deny unlink、USER_DATA_PATH、copyFile/异步 unlink）。**未覆盖**：真机（无设备）、openDocument 预览视觉内容（fixture 为手工构造的最小合法 PDF，内容渲染未目视确认）。
- 过期 grant 的 GUI 提示映射：应用每次点击都新铸 grant（默认 TTL 上限），真实 GUI 点击无法自然命中过期态；该映射由单测（`assembly: signed download 401…` 断言 `重新获取` 文案）+ HTTP 层 `artifact_grant_expired` 语义复核覆盖，GUI 层以“真实失败点击触发含‘重新获取’指引的 Notice 渲染”佐证。此为边界，非全量 GUI 过期演示。

## 6. 文件清单（相对 apps/miniprogram/）

- `config/index.ts`（D1：tdesignDist 拷贝 + copy 注入）
- `src/subpackages/execution/artifact/index.config.ts`（D1：`/npm/tdesign/button/button`）
- `src/subpackages/execution/artifact/index.tsx`（D2：JSX 化 t-button）
- `src/subpackages/execution/artifact/t-button.d.ts`（D2：类型 + 根因备注）
- `src/platform/files.ts`（D3：私有副本生命周期 + wx.env + 扩展名收尾）
- `tests/build-output.test.mjs`（D1/D2 产物断言）
- `tests/artifact-cleanup.test.mjs`（D3 场景）
- `tests/helpers/taro-stub.mjs`（平台契约还原）
- `tests/assembly.test.mjs`（新契约适配）
- `.superpowers/sdd/2026-09-24-issue-140-t06-gui-fix/evidence/*`（复验证据，脱敏）

## 7. 自查与遗留

- 保持 TDesign Miniprogram 路线：未替换原生 button；t-button 在真实 DevTools 点击/属性/主题全链路工作。
- 遗留/未验证：真机（无设备）；openDocument 预览视觉内容；`project.private.config.json` 按 gitignored 约定留在本地未入库；CommercialSummary 13 个 typecheck 基线错误未动。
- 端口偏差（57804 被并行会话占用 → 57844）已声明；未触碰他方进程与数据。

---

# 第 2 轮：评审 findings 修复（F1/F2/F3；F4/F5/F6 low 记录不修）

## 提交追加
5. `5bb130fec` fix(miniprogram): F1 主包体积——TDesign 拷贝收窄到 button 运行时闭包并排除 .d.ts
6. `2b96b0473` fix(miniprogram): F1 追修——TDesign 闭包必须携带 miniprogram_npm（裸 require tslib 的解析根）

## F1（high）主包超限——修复与实测
- RED：build-output 新增 3 断言（无 .d.ts 入包 / 主包 <2MB / 闭包传递依赖完整且目录集合恰为闭包），修复前 3 条全失败（392 个 .d.ts、主包 2285KB、含 action-sheet 等无关目录）。
- 修复：config 构建期按组件 json 的 usingComponents 递归收集闭包（button+icon+loading）+ common，逐目录拷贝并 ignore `**/*.d.ts`。
- **第 2 轮实测又发现**：仅闭包+common 的构建在真实 DevTools 冷启动白屏（"Page has not been registered yet"），自动化 exception 捕获根因：`module 'npm/tdesign/button/tslib.js' is not defined, require args is 'tslib'`——TDesign 组件 JS 裸 require("tslib")，微信按向上查找 `miniprogram_npm/<name>` 解析；`miniprogram_dist/miniprogram_npm`（tslib/dayjs/marked/tinycolor2，87KB）必须随闭包。A/B 验证：全量=可启动 / 闭包(无 miniprogram_npm)=白屏 / 闭包+miniprogram_npm=可启动（两次构建的 webpack 产物 diff 字节级一致，排除其他变量）。已补入 config 与产物断言（含 miniprogram_npm/tslib 存在性 + 目录集合恰为闭包+miniprogram_npm）。
- 终值：主包 **1101KB < 2048KB**；dist/npm/tdesign 仅 button/common/icon/loading/miniprogram_npm；0 个 .d.ts。真实 DevTools 冷启动 + 全链路复验通过（见 evidence/round2/）。

## F2（medium）命令复跑（最终 HEAD，全部真实执行）
```
pnpm --filter @weknora/miniprogram build:weapp   → Compiled successfully in 1.21m（主包 1101KB）
pnpm --filter @weknora/miniprogram test          → # tests 62 / # pass 62 / # fail 0 / # skipped 0
pnpm --filter @weknora/miniprogram typecheck     → 13 errors（全部 account/pages.tsx 基线豁免，无新增）
git diff --check                                  → clean
```

## F3（medium）过期 grant GUI 提示证据（新增）
- 方法：automation `mockWxMethod('request', result)` **仅**替换 mint 响应，注入一枚真实服务器签发、已过期的签名 URL（HTTP mint?ttl_seconds=1 + 2s 后预核验 401）；随后的 downloadFile、错误体读取、错误码映射、Notice 渲染全部为应用真实链路。
- 真实点击（2026-09-24T22:09:26.253Z）→ 服务器同秒收到过期 URL redeem：`06:09:26.296 GET …download… → 401`（evidence/round2/server-log-key-lines.txt）。
- GUI Notice（shot-f3v4-02 + f3-expiry-results.json WXML dump）：**“下载授权已过期，请再次点击“打开或保存”重新获取。”**（expiry 专属文案，非 fallback）。
- 恢复后再真实点击 = “重新获取”真实成立：`06:09:37.947 POST signed-url 200 → 06:09:38.030 GET download 200`，danger notice=null，USER_DATA_PATH 无 wk-open-* 残留（shot-f3v4-03）。

## low findings 记录（不修）
- F4：进程在 copyFile 后被杀可能残留 wk-open-* 副本（无启动期清扫）——记录待后续任务。
- F5：downloadFile 临时文件不再清理属契约变更（运行时管理、非持久），UI 文案对“打开的副本”成立；报告与测试已声明。
- F6：build-output 断言在 dist 缺失时 skip；建议 CI 固定 build→test 顺序——记录，本轮不强制。

## 第 2 轮复验证据
`evidence/round2/`：round2-summary.txt（全过程摘要）、gui/（截图 5 张 + f3-expiry-results.json）、server-log-key-lines.txt（签名已擦除）。临时环境（服务器 57844、DevTools 会话、/tmp/wk-t06r2）已全部停止与销毁。
