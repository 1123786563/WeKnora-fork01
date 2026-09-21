# React 端 TDesign 同构迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** apps/web + packages/views 的 UI 层整体切换为 tdesign-react + tdesign-icons-react，与 Vue 端同构，达到 parity 扫描逐项 0.00% diff，并彻底移除 Tailwind/shadcn 栈。

**Architecture:** 按页同构重写——以 Vue 端 SFC 为事实源，React 端复刻 DOM 结构（t-* 类名天然一致）+ 平移 SFC `<style>` 为普通 CSS；Phase 0 spike 闸门验证 React 19 兼容，Phase 2 pilot 沉淀 playbook 后按批全量，Phase 4 清理旧栈并全量验收。

**Tech Stack:** tdesign-react@1.18.3、tdesign-icons-react@0.6.11、React 19.3、Vite 7、pnpm workspace、Playwright（parity 扫描）。

**Spec:** `docs/specs/2026-09-21-tdesign-react-migration-design.md`（本计划从 spec 出发，执行者需同时读 spec）

## Global Constraints

- tdesign-react 锁定 `1.18.3`、tdesign-icons-react 锁定 `0.6.11`（spec §2 查证值，peer `react>=16.13.1` 无安装冲突）。
- **禁止改动 Vue 端（frontend/）**，唯一例外：Phase 0 的临时 spike dev 页，验证后同任务内删除（spec §4）。
- `wk-*` 类名必须保留在 DOM 上（测试查询 hook），只换样式实现（spec §6）。
- 验收口径：pixdiff 容差 8、1280×720 稳态截图下每项差异像素占比 0.00%（spec §8）。
- parity 环境：Vue dev :5174、React dev :5175、后端 :8084，凭据 `~/.weknora-parity-creds.env`；扫描命令 `node scripts/parity/auto-scan.mjs`，单页过滤 `PAGES=<id>`。
- Phase 4 之前 `@weknora/ui` 与 tdesign-react 并存，**已迁页面禁止新增旧栈引用**。
- 提交粒度：一页一提交 `feat(parity): migrate <page> to tdesign-react`；基建任务独立提交。
- CSS 引入顺序约束：`tdesign.css`（unlayered）最前 → 主题 token（unlayered）→ 页面 CSS（unlayered，靠后者覆盖）——镜像 Vue 端 document order；过渡期残留的 Tailwind utilities 在 `@layer utilities`（层级弱于 unlayered，与现状一致，页内冲突随该页迁移自然消除）。
- 测试命令：`pnpm --filter @weknora/web test`（node test runner + jsdom）。

## 启动前置（主会话执行，不派子代理）

1. 停止 parity 自动化：删除/暂停 Cron automation-41aaa5e6（扫描+修复闭环），避免迁移期间互相收割（spec §10）。
2. 收尾当前工作树：把未提交的 parity 修改（约 30 个文件）确认后单独提交，工作树必须干净再开迁移分支。
3. 创建迁移分支：`git checkout -b feat/tdesign-react-migration`。

---

## Phase 0 — Spike：React 19 验证闸门

### Task 1: 安装依赖并引入 TDesign CSS

**Files:**
- Modify: `apps/web/package.json`（经 pnpm 命令）
- Modify: `apps/web/src/styles.css:1-8`

**Interfaces:**
- Produces: apps/web 可 import tdesign-react 组件；styles.css 顶部引入 `tdesign-react/dist/tdesign.css`（与 Vue 端 `frontend/src/main.ts:8` 的全量引入方式一致）。

- [ ] **Step 1: 安装依赖**

```bash
pnpm --filter @weknora/web add tdesign-react@1.18.3 tdesign-icons-react@0.6.11
```

预期：安装成功，无 peer 冲突报错（peer `react>=16.13.1` 涵盖 19.3）。

- [ ] **Step 2: styles.css 引入 TDesign 全量样式（置于最前）**

在 `apps/web/src/styles.css` 文件最顶部（现有 `@layer` 声明之前）加入：

```css
/* TDesign 同构迁移：与 Vue 端 frontend/src/main.ts:8 全量引入一致，unlayered 且最先加载 */
@import "tdesign-react/dist/tdesign.css";
```

- [ ] **Step 3: 验证构建与现有测试不破**

```bash
pnpm --filter @weknora/web build && pnpm --filter @weknora/web test
```

预期：build 成功，全部测试通过。

- [ ] **Step 4: Commit**

```bash
git add apps/web/package.json pnpm-lock.yaml apps/web/src/styles.css
git commit -m "feat(parity): 引入 tdesign-react 1.18.3 + icons 0.6.11 基础依赖"
```

### Task 2: 双端 Spike 验证页（throwaway，dev-only）

**Files:**
- Create: `apps/web/src/dev/TDesignSpikePage.tsx`（React 端，挂 `/platform/dev/tdesign-spike`）
- Create: `frontend/src/views/dev/TDesignSpike.vue`（Vue 端，同路径路由）
- Modify: 两端路由表（React `apps/web/src/routes.tsx`；Vue `frontend/src/router/` 现有 dev 路由旁）

**Interfaces:**
- Produces: 两端同路径、同 DOM 结构的验证页，供 Task 3 截图对比。**验证完成后本任务产物整体删除**。

- [ ] **Step 1: React 端 spike 页——组件矩阵 + 命令式 API 触发按钮 + 20 个图标**

```tsx
// apps/web/src/dev/TDesignSpikePage.tsx
import { useState } from 'react';
import {
  Button, Input, Select, Table, Dialog, Tabs, Switch, Tooltip,
  MessagePlugin, NotificationPlugin,
} from 'tdesign-react';
import {
  AddIcon, SearchIcon, DeleteIcon, EditIcon, CloseIcon, CheckIcon,
  UserIcon, SettingIcon, DownloadIcon, UploadIcon, RefreshIcon,
  ChevronLeftIcon, ChevronRightIcon, ChevronDownIcon, InfoCircleIcon,
  ErrorCircleIcon, CheckCircleIcon, TimeIcon, CalendarIcon, FolderIcon,
} from 'tdesign-icons-react';

const ICONS = [
  AddIcon, SearchIcon, DeleteIcon, EditIcon, CloseIcon, CheckIcon,
  UserIcon, SettingIcon, DownloadIcon, UploadIcon, RefreshIcon,
  ChevronLeftIcon, ChevronRightIcon, ChevronDownIcon, InfoCircleIcon,
  ErrorCircleIcon, CheckCircleIcon, TimeIcon, CalendarIcon, FolderIcon,
];

const ROWS = [
  { id: 1, name: 'alpha', status: 'active' },
  { id: 2, name: 'beta', status: 'inactive' },
];

export default function TDesignSpikePage() {
  const [dialogVisible, setDialogVisible] = useState(false);
  return (
    <div style={{ padding: 24 }}>
      <div>
        <Button theme="primary">主按钮</Button>
        <Button theme="default" variant="outline">描边按钮</Button>
        <Button theme="danger">危险</Button>
      </div>
      <div style={{ marginTop: 16 }}>
        <Input placeholder="请输入" defaultValue="spike-input" />
        <Select defaultValue="a" options={[{ label: '选项一', value: 'a' }, { label: '选项二', value: 'b' }]} />
      </div>
      <div style={{ marginTop: 16 }}>
        <Tabs value="t1" list={[{ label: '标签一', value: 't1' }, { label: '标签二', value: 't2' }]} />
        <Switch defaultValue />
        <Tooltip content="提示文本"><Button>Tooltip 宿主</Button></Tooltip>
      </div>
      <div style={{ marginTop: 16 }}>
        <Table
          rowKey="id"
          data={ROWS}
          columns={[
            { colKey: 'id', title: 'ID' },
            { colKey: 'name', title: '名称' },
            { colKey: 'status', title: '状态' },
          ]}
        />
      </div>
      <div style={{ marginTop: 16 }}>
        <Button onClick={() => setDialogVisible(true)}>打开 Dialog</Button>
        <Dialog visible={dialogVisible} header="Spike 弹窗" onConfirm={() => setDialogVisible(false)} onClose={() => setDialogVisible(false)}>
          <p>dialog-content-spike</p>
        </Dialog>
        <Button onClick={() => MessagePlugin.success('message-spike')}>触发 Message</Button>
        <Button onClick={() => NotificationPlugin.success({ title: 'notification-spike' })}>触发 Notification</Button>
      </div>
      <div style={{ marginTop: 16, fontSize: 20 }} data-spike-icons>
        {ICONS.map((I, i) => <I key={i} size="20px" />)}
      </div>
    </div>
  );
}
```

注册路由 `/platform/dev/tdesign-spike`（对照现有 `/platform/dev/markdown` 的注册方式，`apps/web/src/routes.tsx` + `DevMarkdownPage.tsx` 的挂载模式）。dev-only 页无需进生产守卫，与 DevMarkdownPage 同策略。

- [ ] **Step 2: Vue 端 spike 页——同名组件、同 props、同 DOM 顺序 1:1 复刻**

`frontend/src/views/dev/TDesignSpike.vue` 使用 tdesign-vue-next 对应组件（t-button/t-input/t-select/t-table/t-dialog/t-tabs/t-switch/t-tooltip）与 `tdesign-icons-vue-next` **同名 20 个图标**（AddIcon…FolderIcon），布局与 React 版逐节点一致（相同 padding/间距 inline style、相同文本、相同 data 属性）。在 `frontend/src/router/` dev 路由处注册同路径。

- [ ] **Step 3: 两端 dev server 手动冒烟**

```bash
pnpm --filter @weknora/web dev &   # :5175
cd frontend && pnpm dev &          # :5174
```

浏览器打开 `http://localhost:5175/platform/dev/tdesign-spike` 与 `:5174/...`，确认：组件渲染正常、无控制台报错、**点击"触发 Message/Notification"两端都正常弹出**（React 19 命令式 API 关键验证点）。

- [ ] **Step 4: Commit（throwaway 标记）**

```bash
git add apps/web/src/dev frontend/src/views/dev apps/web/src/routes.tsx frontend/src/router
git commit -m "chore(spike): 双端 TDesign spike 验证页（throwaway，Phase 0 末删除）"
```

### Task 3: Spike 截图对比与闸门判定

**Files:**
- Create: `scripts/parity/tmp-spike-tdesign.mjs`（一次性脚本，跑完删除）
- Create: `docs/migrations/react/evidence/vue-react-parity/spike-tdesign-react19.md`（判定结论存档）

**Interfaces:**
- Consumes: Task 2 的两端 spike 页（需 dev server + 后端 :8084 在线，登录态注入复用 `auto-scan.mjs` 的 localStorage 方案——直接抄其 login() 与 context 注入代码）。
- Produces: 闸门判定结论（通过 / patch / 降级评估 / 止损），写入证据文件。

- [ ] **Step 1: 编写一次性截图对比脚本**

复用 `auto-scan.mjs` 的 chromium 加载、login()、双 context localStorage 注入逻辑，新增两个页面对比项：

```js
const PAGES = [
  { id: 'spike-tdesign', path: '/platform/dev/tdesign-spike', settle: 1500 },
  // 交互态：先点"打开 Dialog"再截图
  { id: 'spike-tdesign-dialog', path: '/platform/dev/tdesign-spike',
    actions: [{ clickText: ['打开 Dialog'] }], settle: 800 },
];
```

截图规格与 auto-scan 一致（1280×720），调用 `python3 scripts/parity/pixdiff.py` 对比。

- [ ] **Step 2: 运行并记录三项判定**

```bash
node scripts/parity/tmp-spike-tdesign.mjs
```

判定标准（写入证据文件）：
1. **运行时兼容**：Message/Notification 点击后两端均弹出、无 React 报错（如 `ReactDOM.render is no longer supported`）；
2. **组件像素**：spike-tdesign 静态页差异 < 0.5%（弹层/日期类组件允许暂有差异，pilot 逐个核对）；
3. **图标像素**：`data-spike-icons` 区域差异 ≈ 0%（icons-react 0.6.11 vs icons-vue-next 0.4.4 同名 SVG）。

- [ ] **Step 3: 按决策树行动（spec §7 Phase 0）**

- 全过 → 在证据文件记录 PASS，继续 Task 4；
- 命令式 API 触雷 → `pnpm --filter @weknora/web add -D patch-package`，针对 `ReactDOM.render` 调用处写 patch 换 `createRoot`，复测；patch 后仍不可行 → **停止，向主会话汇报**评估 React 18 降级（需核对 TanStack Router/recharts 兼容）；仍不可行 → 止损，迁移取消。

- [ ] **Step 4: Commit 证据 + 清理 throwaway**

```bash
rm scripts/parity/tmp-spike-tdesign.mjs
rm -r apps/web/src/dev/TDesignSpikePage.tsx frontend/src/views/dev/TDesignSpike.vue
# 并移除两端路由注册
git add -A && git commit -m "chore(spike): Phase 0 完成——React19 兼容判定 PASS，清理 spike 页"
```

（若判定非 PASS，则不清理，保留现场供决策。）

---

## Phase 1 — 基建

### Task 4: 主题 token 平移

**Files:**
- Create: `packages/design-tokens/src/tdesign-theme.css`
- Modify: `packages/design-tokens/src/styles.css`（末尾追加 import 或在 index 引出，跟随包内现有引出方式）
- Modify: `apps/web/src/styles.css`（在 tdesign.css 之后引入）
- Test: `packages/design-tokens/src/tokens.test.ts`（扩展）

**Interfaces:**
- Consumes: `frontend/src/assets/theme/theme.css`（201 行，light+dark 完整 `--td-*` 覆盖，` :root:root ` 双选择器提权 + `[theme-mode="dark"]` 分支——**原样平移，不重排不重命名**）。
- Produces: React 端 TDesign 组件的主题计算值与 Vue 端一致（品牌绿 `#07c05f` 系、圆角、阴影、字体族全部经 token 生效）。

- [ ] **Step 1: 写失败测试——token 键存在性断言**

在 `packages/design-tokens/src/tokens.test.ts` 追加：读取 `tdesign-theme.css` 源文本，断言包含 `--td-brand-color-4: #07c05f`、`--td-radius-default: 3px`、`[theme-mode="dark"]`、`--app-font-family`（4 个关键锚点，防止平移截断）。

- [ ] **Step 2: 运行确认失败**

```bash
pnpm --filter @weknora/design-tokens test
```

预期：新增断言 FAIL（文件不存在）。

- [ ] **Step 3: 平移 theme.css**

```bash
cp frontend/src/assets/theme/theme.css packages/design-tokens/src/tdesign-theme.css
```

并在 `packages/design-tokens/src/styles.css` 末尾加 `@import "./tdesign-theme.css";`（保持包内既有引出链）。确认 `apps/web/src/styles.css` 中 design-tokens 的 import 位于 `tdesign-react/dist/tdesign.css` **之后**（token 覆盖库默认值，与 Vue 端顺序一致：tdesign.css → theme.css）。

- [ ] **Step 4: 测试通过 + 构建**

```bash
pnpm --filter @weknora/design-tokens test && pnpm --filter @weknora/web build
```

- [ ] **Step 5: Commit**

```bash
git add packages/design-tokens apps/web/src/styles.css
git commit -m "feat(parity): 平移 Vue 端 TDesign 主题 token 到 design-tokens"
```

### Task 5: 图标离线守卫移植 + 本地 SVG sprite 注册

**Files:**
- Read: `frontend/src/utils/tdesign-icon-offline.ts`（守卫实现事实源——预插占位节点让 tdesign-icons 的 `checkScriptAndLoad`/`checkLinkAndLoad` 去重命中，不注入真实 CDN 节点）
- Create: `apps/web/src/tdesign-icon-offline.ts`
- Modify: `apps/web/src/main.tsx`（渲染前执行）
- Create: `apps/web/public/tdesign-icons/`（复制 `frontend/public/tdesign-icons/` 整目录）
- Modify: `apps/web/index.html`（加本地 sprite script）

**Interfaces:**
- Consumes: 已查证的 tdesign-icons-react 0.6.11 内部常量——CDN URL 硬编码为 **0.4.5 版本**（`esm/svg-sprite/svg-sprite.js:13` 与 `esm/iconfont/iconfont.js:12`），去重选择器类名与 Vue 端相同（`t-svg-js-stylesheet--unique-class` / `t-iconfont-stylesheet--unique-class`）。注意：与 Vue 端拦截的 0.4.0–0.4.4 集合**不同**，React 守卫必须拦 0.4.5。
- Produces: `installTDesignIconOfflineGuard()`；name-based `<Icon name=...>` 依赖的本地 sprite（与 Vue 端 `frontend/index.html:65` 同源同版本 `/tdesign-icons/0.4.1/fonts/index.js`，保证两端 name 图标像素同源）。

- [ ] **Step 1: 写守卫文件（可直接落盘）**

```ts
// apps/web/src/tdesign-icon-offline.ts
// 阻断 tdesign-icons-react 0.6.11 对 tdesign.gtimg.com 的 iconfont/svg-sprite 注入。
// 机制同 frontend/src/utils/tdesign-icon-offline.ts：预插带库内部去重选择器
// 的占位节点（非标准 type/rel，浏览器不发请求），使 checkScriptAndLoad/
// checkLinkAndLoad 命中去重直接返回。CDN URL 为 0.6.11 内部硬编码的 0.4.5。
const SVG_SCRIPT_CLASS = 't-svg-js-stylesheet--unique-class';
const ICONFONT_LINK_CLASS = 't-iconfont-stylesheet--unique-class';
const BLOCKED_SCRIPT_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.js';
const BLOCKED_LINK_URL = 'https://tdesign.gtimg.com/icon/0.4.5/fonts/index.css';

let installed = false;

export function installTDesignIconOfflineGuard(): void {
  if (installed || typeof document === 'undefined') return;
  installed = true;

  const body = document.body;
  if (!body) {
    document.addEventListener('DOMContentLoaded', () => installTDesignIconOfflineGuard(), { once: true });
    installed = false;
    return;
  }

  const stubScript = document.querySelector(`script.${SVG_SCRIPT_CLASS}[src="${BLOCKED_SCRIPT_URL}"]`);
  if (!stubScript) {
    const s = document.createElement('script');
    s.setAttribute('class', SVG_SCRIPT_CLASS);
    s.setAttribute('src', BLOCKED_SCRIPT_URL);
    s.setAttribute('type', 'text/no-load'); // 非标准 MIME，跳过 fetch/执行
    s.setAttribute('data-weknora-blocked-cdn', 'tdesign-icons');
    body.appendChild(s);
  }

  const stubLink = document.querySelector(`link.${ICONFONT_LINK_CLASS}[href="${BLOCKED_LINK_URL}"]`);
  if (!stubLink) {
    const l = document.createElement('link');
    l.setAttribute('class', ICONFONT_LINK_CLASS);
    l.setAttribute('href', BLOCKED_LINK_URL);
    l.setAttribute('rel', 'preload-blocked'); // 不声明 stylesheet，不发请求
    l.setAttribute('data-weknora-blocked-cdn', 'tdesign-icons');
    document.head.appendChild(l);
  }
}
```

- [ ] **Step 2: 本地 sprite 注册**

```bash
cp -r frontend/public/tdesign-icons apps/web/public/tdesign-icons
```

在 `apps/web/index.html` 的 `<body>` 起始处加（对照 `frontend/index.html:65`）：

```html
<!-- 离线部署兼容：本地加载 tdesign-icons SVG sprite（同 Vue 端版本，保证 name 图标同源渲染） -->
<script src="/tdesign-icons/0.4.1/fonts/index.js"></script>
```

- [ ] **Step 3: main.tsx 渲染前调用**（对照 `frontend/src/main.ts:19-23` 的位置语义：`installTDesignIconOfflineGuard()` 必须在 `createRoot(...).render(...)` 之前）。
- [ ] **Step 4: 验证**：断网状态下打开 dev 页，DevTools Network 面板无 gtimg 请求且无图标缺失报错。
- [ ] **Step 5: Commit**

```bash
git add apps/web/src/tdesign-icon-offline.ts apps/web/src/main.tsx apps/web/public/tdesign-icons apps/web/index.html
git commit -m "feat(parity): 移植 TDesign 图标离线守卫 + 本地 sprite 到 React 端"
```

### Task 6: TDesign locale 接线

**Files:**
- Modify: `apps/web/src/App.tsx`（或全局 Provider 所在层，以现有 i18n 挂载点为准）
- Read: `frontend/src/App.vue:18-22`（locale 映射事实源：en_US/zh_CN/ko_KR/ja_JP/ru_RU 五语言）

**Interfaces:**
- Produces: `ConfigProvider`（tdesign-react）包裹应用根，`globalConfig` 随当前 i18n 语言切换（tdesign-react locale 路径 `tdesign-react/es/locale/zh_CN` 等，按包内实际导出为准），使分页"共 x 条"、日期选择器等内置文案与 Vue 端逐语言一致。

- [ ] **Step 1: 建立 语言→locale 模块 映射表**（对照 App.vue 的 import 结构，React 侧懒加载或静态 import 五个 locale）。
- [ ] **Step 2: ConfigProvider 包裹应用根**，跟随现有语言切换状态。
- [ ] **Step 3: 冒烟**：切到 en_US，含 t-pagination 组件的页文案为英文（此任务后尚无已迁页，可用 dev 页或临时挂一个 Pagination 验证后即删）。
- [ ] **Step 4: Commit**

```bash
git add apps/web/src/App.tsx
git commit -m "feat(parity): TDesign ConfigProvider locale 接线（五语言对齐 Vue 端）"
```

### Task 7: 平移规则 Playbook 固化

**Files:**
- Create: `docs/migrations/react/tdesign-migration-playbook.md`

**Interfaces:**
- Produces: Phase 3 全部页面任务共同遵循的 SOP 文档（Phase 2 pilot 验证并回填修订）。内容必须包含：

1. **组件映射表**：`@weknora/ui` 16 组件 + 常用 HTML 控件 → tdesign-react 对应组件（Button→Button、Input→Input、Textarea→Textarea、Select→Select、checkbox→Checkbox、radio→Radio、switch→Switch、Dialog→Dialog、dropdown-menu→Dropdown、tabs→Tabs、tooltip→Tooltip、table→Table、badge→Badge、alert→Alert、sheet→Drawer、number-input→InputNumber、range→Slider）。
2. **样式平移规则**：Vue SFC `<style scoped/less>` → React 侧每页一个 `.css` 文件；scoped 选择器改显式前缀（该 SFC 根元素类名）限定；less 嵌套展平；类名不改（Vue 端类名即事实源，`wk-*` hook 必须保留）。
3. **DOM 复刻规则**：React JSX 逐节点对照 Vue template（标签、类名顺序、条件渲染分支）；t-* 组件的 props 从 Vue 端同名 props 直译。
4. **删除规则**：该页 tsx 中全部 Tailwind utility 类删除，布局值进平移 CSS；该页专属 parity 补丁 CSS（如 `agents.css` 中 agents 相关段）删除或并入平移 CSS。
5. **验证 SOP**（每页）：单测绿 → `PAGES=<id> node scripts/parity/auto-scan.mjs` → 该页所有相关扫描项（含 `ix-*` 交互项）0.00% → 一页一提交。
6. **DOM 差异台账**（空表起步，pilot 起回填）：`| 组件 | react DOM/默认值 | vue-next DOM/默认值 | 补齐方式 |`。

- [ ] **Step 1: 按上述 6 节写出 playbook 初版**（内容为规则本身，不是占位符）。
- [ ] **Step 2: Commit**

```bash
git add docs/migrations/react/tdesign-migration-playbook.md
git commit -m "docs(parity): TDesign 迁移 playbook 初版（Phase 2 pilot 回填）"
```

---

## Phase 2 — Pilot + 扫描工具稳态化

### Task 8: 扫描器稳态化

**Files:**
- Modify: `scripts/parity/auto-scan.mjs`（截图函数内，约 settle 逻辑处）

**Interfaces:**
- Produces: `waitForSteady(page, settleMs)` 门——所有截图前统一执行，消除"Vue 瞬态空白"伪差（历史坑：Vue 端路由切换瞬间白屏被截入，伪造恶化）。

- [ ] **Step 1: 实现 steady 门并替换现有裸 settle**

```js
async function waitForSteady(page, settleMs) {
  await page.waitForLoadState('networkidle').catch(() => {});
  await page.evaluate(() => document.fonts.ready).catch(() => {});
  await page.evaluate(() => {
    document.getAnimations().forEach(a => a.pause?.());
  }).catch(() => {});
  await page.waitForTimeout(settleMs);
}
```

在双端截图前统一调用（保留每页 `settle` 字段的差异化时长；`freezeCarousel`（login/register）逻辑不动——它冻结的是轮播相位，与本门叠加生效）。注意：`getAnimations().pause()` 只冻结过渡动画，不改变已渲染稳态。

- [ ] **Step 2: 全量基线扫描一次，确认无环境性退化**

```bash
node scripts/parity/auto-scan.mjs
```

预期：各页 diff 较历史基线持平或下降（稳态门只应消除伪差）；若有页面异常抬升，检查是否动画冻结误伤（如进度条被 pause 在中间态——此类页面在 ALL_PAGES 该项加 `noFreeze: true` 白名单跳过动画冻结）。

- [ ] **Step 3: Commit**

```bash
git add scripts/parity/auto-scan.mjs
git commit -m "feat(parity): 扫描器稳态门（networkidle+fonts+动画冻结）消除瞬态伪差"
```

### Task 9: Pilot — agents 页同构迁移

**Files:**
- Vue 事实源：`frontend/src/views/agent/`（列表页 + 编辑器弹窗等全部子组件）
- Modify: `apps/web/src/agents/`（AgentsPage.tsx、AgentEditorModal.tsx、AgentParserRules.tsx、PersonaSection.tsx、SubagentsSection.tsx、MbtiTestModal.tsx 及 list.ts/state.ts 等纯逻辑文件不动）
- Create: `apps/web/src/agents/agents.td.css`（从 Vue SFC `<style>` 平移）
- Delete/Modify: `apps/web/src/agents/agents.css`（parity 补丁段删除，仍被未迁页引用的段保留并注明）
- Test: `apps/web/src/agents/AgentsPage.test.tsx`、`agent-editor.test.tsx` 等现有测试随迁移更新

**Interfaces:**
- Consumes: Task 4 主题 token、Task 5 图标守卫、Task 6 locale、Task 7 playbook 规则。
- Produces: 首个完全同构页 + playbook 实证回填素材 + DOM 差异台账首批条目。

- [ ] **Step 1: 通读 Vue 事实源**：`frontend/src/views/agent/` 下全部 SFC 的 template 结构、类名、`<style>` 块、props；列出组件树清单。
- [ ] **Step 2: 重写 AgentsPage.tsx 及子组件**：tdesign-react 组件直译（按 playbook 组件映射表），DOM/类名 1:1 复刻 Vue template，`wk-*` hook 保留；图标换 tdesign-icons-react 同名。
- [ ] **Step 3: 样式平移**：Vue 各 SFC `<style>` → `agents.td.css`（scoped 展平规则按 playbook §2）；该页 tsx 全部 Tailwind utility 移除，布局值入 CSS。
- [ ] **Step 4: 单测更新并通过**

```bash
pnpm --filter @weknora/web test
```

- [ ] **Step 5: 单页扫描收敛循环**

```bash
PAGES=agents,ix-agents-create node scripts/parity/auto-scan.mjs
```

循环：diff > 0 的项 → DevTools 对照两端 computed style / DOM 差异 → 修（优先改平移 CSS；若根因是 tdesign-react↔vue-next 组件 DOM 差异，记入台账并 CSS 补齐）→ 重扫，直至两项 0.00%。

- [ ] **Step 6: 一页一提交**

```bash
git add apps/web/src/agents
git commit -m "feat(parity): migrate agents to tdesign-react（首个同构页）"
```

### Task 10: Playbook 回填

**Files:**
- Modify: `docs/migrations/react/tdesign-migration-playbook.md`

**Interfaces:**
- Consumes: Task 9 全程遇到的差异与解法。
- Produces: 实证版 playbook（Phase 3 各批任务的执行手册）——DOM 差异台账首批条目、agents 页实际踩坑（props 直译例外、弹层挂载点、图标尺寸差等）、每页耗时参考。

- [ ] **Step 1: 回填台账与规则修订，Commit**

```bash
git add docs/migrations/react/tdesign-migration-playbook.md
git commit -m "docs(parity): playbook 回填 pilot 实证（DOM 差异台账 v1）"
```

---

## Phase 3 — 按页全量（每批 = 批内每页扫描项全部 0.00%）

> **每页统一 SOP（各批任务按此执行，页清单见表格；规则细节以 playbook 实证版为准）：**
> ① 读 Vue 事实源 SFC → ② tdesign-react 重写（DOM/类名 1:1）→ ③ `<style>` 平移到该页 `.td.css` → ④ 删该页 Tailwind utility 与专属 parity CSS 段 → ⑤ `pnpm --filter @weknora/web test` 绿 → ⑥ `PAGES=<该页全部扫描id> node scripts/parity/auto-scan.mjs` 收敛至全 0.00% → ⑦ `feat(parity): migrate <page> to tdesign-react` 一页一提交。
> 批内页面相互独立，可按 AGENTS.md 用独立 worktree 并行（`superpowers:using-git-worktrees`），主会话集成。共享组件（如 PlatformShell 布局、分页器）在批 1 首个页面处理时一并迁移，批内后续页面复用。

### Task 11: 批次 1 — 平台核心

| 扫描 id | Vue 事实源 | React 目标 |
|---|---|---|
| kb-list、ix-kb-list-create | `frontend/src/views/knowledge/`（列表） | `apps/web/src/knowledge-bases/` 列表部分 |
| kb-demo / kb-faq / kb-wiki 及子 tab、ix-kb-settings、ix-kb-batch、ix-kb-listview、ix-kb-doc-detail | `frontend/src/views/knowledge/`（详情/文档/FAQ/Wiki） | `apps/web/src/knowledge-bases/` + `apps/web/src/documents/` + `apps/web/src/faq/` + `apps/web/src/wiki/` |
| orgs、ix-orgs-created、ix-orgs-create | `frontend/src/views/organization/` | `apps/web/src/organizations/` |
| apps、apps-connections | `frontend/src/views/apps/` | `apps/web/src/apps/` + `apps/web/src/appconnector/` |
| creatchat、kb-demo-creatchat | `frontend/src/views/creatChat/` | `apps/web/src/chat/`（创建对话部分） |

含共享层迁移：`apps/web/src/platform/PlatformShell.tsx`（平台布局壳，Vue 端对应 `frontend/src/views/platform/` 布局组件）。

- [ ] 按上方 SOP 逐页执行至表中全部扫描 id 0.00%，每页一提交。

### Task 12: 批次 2 — settings 23 section + integrations 6 tab

| 扫描 id | Vue 事实源 | React 目标 |
|---|---|---|
| settings-general … settings-system-audit-log（23 项，键表见 `auto-scan.mjs` SETTINGS_SECTION_KEYS） | `frontend/src/views/settings/`（Settings.vue + 各 section 子组件） | `packages/views/src/settings/` + `apps/web/src/settings/`（ConfigSettingsPanel、ModelSettingsPanel、ParserEngineSettingsPanel、ResourceSettingsPanel、SandboxSettingsPanel、SystemAuditLogPanel 等） |
| settings-integration-im/embed/api/cli/chrome/claw（6 项） | `frontend/src/config/integrations.ts` + 对应视图 | `packages/views/src/integrations/` |

- [ ] 按 SOP 逐 section 执行至 29 项扫描全 0.00%；settings 壳层（左侧 nav 布局）随首个 section 迁移。

### Task 13: 批次 3 — chat 家族 + documents + faq

| 扫描 id | Vue 事实源 | React 目标 |
|---|---|---|
| chat、ix-chat-header-menu、ix-chat-mention | `frontend/src/views/chat/`（最复杂：会流、工具调用渲染、侧栏） | `packages/views/src/chat/`（page.tsx、session-sidebar.tsx 等） |
| ix-faq-tagfilter、ix-faq-retrieval、ix-faq-import | `frontend/src/views/knowledge/`（FAQ 管理段） | `apps/web/src/faq/FAQPage.tsx` |
| documents 相关（kb-demo 内文档视图） | `frontend/src/views/knowledge/` | `apps/web/src/documents/` |

- [ ] 按 SOP 执行至全部扫描 id 0.00%。chat 放最后：先迁子组件再迁会话流主体。

### Task 14: 批次 4 — 其余全部

| 扫描 id | Vue 事实源 | React 目标 |
|---|---|---|
| login、register（freezeCarousel 项） | `frontend/src/views/auth/` | `apps/web/src/auth/LoginPage.tsx` |
| redirect-system、redirect-integrations | —（重定向目标页已迁，本项验证落点一致） | `apps/web/src/router.tsx` 重定向逻辑 |
| dev-markdown | `frontend/src/views/dev/` | `apps/web/src/DevMarkdownPage.tsx` |
| 未进扫描清单的页面（一致性要求同 SOP，验收靠目检+单测） | `frontend/src/views/` 其余（analytics/market/experts/administration/commercial/data-sources/embed/guides/craft 等） | `apps/web/src/` 对应目录 + `packages/views/src/{embed,guides,craft}` |
| 404 | Vue 404 页 | `apps/web/src/NotFoundPage.tsx`（注意：历史残差中 Vue 404 项属环境性，若仍不可达 0 记录进豁免台账，不阻塞批次） |

- [ ] 按 SOP 执行；本批完成后 `pnpm --filter @weknora/web build && pnpm --filter @weknora/web test` 全绿。

---

## Phase 4 — 清理与验收

### Task 15: 删除旧栈

**Files:**
- Delete: `packages/ui/`（整包）、`apps/web/src/agents/agents.css` 等全部 parity 补丁 CSS 中已无引用者
- Modify: `apps/web/package.json`（移除 `@weknora/ui`、`tailwindcss`、`@tailwindcss/vite`）、`packages/views/package.json`（移除 `@weknora/ui` 引用）、`pnpm-workspace.yaml`（若 packages/ui 移除）
- Modify: `apps/web/src/styles.css`（删除 `@layer` 声明、tailwind theme/utilities import、`@source` 指令、`packages/ui/theme.css` import；保留 tdesign.css、主题 token、body 规则、跨页公共段）
- Modify: `apps/web/vite.config.ts`（移除 tailwindcss vite 插件）

- [ ] **Step 1: 全局搜残留引用（含测试）**

```bash
grep -rn "@weknora/ui\|tailwind" apps/web/src packages/views/src --include='*.tsx' --include='*.ts' --include='*.css'
```

预期为空（测试文件中的引用一并清零——Phase 3 每页已同步更新测试；若有漏页残留，回到 Phase 3 SOP 补迁后再继续）。

- [ ] **Step 2: 执行删除与依赖清理，构建+测试全绿**

```bash
pnpm install && pnpm --filter @weknora/web build && pnpm --filter @weknora/web test
```

- [ ] **Step 3: Commit**

```bash
git add -A && git commit -m "chore(parity): 删除 tailwind/shadcn 旧栈（@weknora/ui、tailwindcss、补丁 CSS）"
```

### Task 16: 全量验收（3 轮）

- [ ] **Step 1: 全量扫描连续 3 轮，逐项 0.00%**

```bash
for i in 1 2 3; do node scripts/parity/auto-scan.mjs || break; done
```

验收：3 轮 report 中 60 项全部 0.00%（唯一允许例外：批 4 记录进豁免台账的环境性项）。证据归档 `docs/migrations/react/evidence/vue-react-parity/final-acceptance/`。
- [ ] **Step 2: 构建体积对比记录**（迁移前基线取 git 历史构建或 spec 记录，对比 dist 产物大小），写入验收证据。
- [ ] **Step 3: Commit 验收证据**

```bash
git add docs/migrations/react/evidence
git commit -m "test(parity): TDesign 同构迁移全量验收——60 项 0.00% × 3 轮"
```

### Task 17: 收尾

- [ ] **Step 1: 恢复 parity automation**：重建 Cron（每 30 分钟扫描，**只告警不自动修复**，守护模式；确认 0.00% 基线后再由用户决定是否恢复自动修复）。
- [ ] **Step 2: spec 状态更新**：`docs/specs/2026-09-21-tdesign-react-migration-design.md` 头部状态改为"已实施（完成日期 + 验收证据链接）"。
- [ ] **Step 3: 主分支合并**：迁移分支经 review 合入 main，worktree 清理。
- [ ] **Step 4: Commit**

```bash
git add docs/specs && git commit -m "docs(spec): TDesign 迁移 spec 标记已实施"
```
