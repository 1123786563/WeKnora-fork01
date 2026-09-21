# R480 证据：settings 9 行错误态浏览器锚定（对称 500 拦截）

- 日期：2026-09-19；执行：编排者直跑 Playwright MCP（浏览器代理连续两轮中断后的替代路径）
- 环境：React http://localhost:5181 / Vue http://localhost:5180，parity-test@local.dev（tenant 10002），已登录态，zh-CN 浅色，视口 1440×900
- 截图：`screenshots/r480-20260919/`（21 张：9 分区双端 + retrieval 抽屉双端 + R028 真实连接失败对）
- 代理报告：`.omc/state/r480/report-A1.md` / `report-A2.md`（编排者按打捞协议续写完成）
- 本轮零代码修改（纯锚定轮）；worktree HEAD 213630c9。

## 方法

1. 端点发现：`page.on('request')` 监听 + 分区点击 / 整页 reload，逐分区确认 API 端点（双端）。
2. 对称拦截：`page.route(glob, fulfill 500 '{"error":"R480 simulated failure"}')`，带 hits 计数器证明拦截命中。
3. DOM 探针：错误文本命中（`/(simulated failure|500)/`）、重试计数（`/重试|重新检测|重新加载/`）、alert 节点数（`[role="alert"], .t-alert, [class*="banner"], [data-slot="alert"]`）、alertText 摘录。
4. 关键前提（本轮新发现的 Vue 行为）：**Vue 在 settings boot 预取全部资源列表 + retrieval-config；重进分区不重新拉取**。因此资源类分区必须「先 route → 整页 reload」才触发；点击法只适用于 parser/system/userprofile/ollama 等按需拉取的分区。

## 端点对照（双端一致，React 无私有端点）

| 分区 | 端点（glob） |
|------|--------------|
| ollama | `**/ollama**` |
| parser | `**/parser-engines**` |
| retrieval | `**/retrieval-config**`（/api/v1/tenants/kv/retrieval-config） |
| storage | `**/storage-backends**`（list + /types，hits=2） |
| system | `**/system/info**` |
| userprofile | `**/auth/me**` |
| vectorstore | `**/vector-stores**`（list + /types） |
| websearch | `**/web-search-providers**`（list + /types） |
| cloud | `**/weknoracloud**`（/api/v1/models/weknoracloud/status） |

## 逐行判定（9/9 锚定完成）

| 行 | 分区 | Vue 基准（500 下） | React 现状（500 下） | 判定 |
|----|------|--------------------|----------------------|------|
| R028 | Ollama | 横幅：友好文案「连接失败，请检查 Ollama 是否运行或服务地址是否正确」+ 重试；body 被吞 | 横幅：裸 body，无重试按钮 | ❌ 缺口 |
| R029 | 解析引擎 | 横幅：原始错误体 + 重试 | 同左（alertText=body, retry=1） | ✅ 对齐 |
| R030 | 检索设置 | boot 预取失败静默吞掉；命令面板抽屉（Vue 唯一入口）渲染默认表单，无错误 UI、零重拉（hits=0） | 抽屉：裸 body 横幅、表单被吞、无重试；另有 ?section=retrieval 深链页（超集）同形态 | ❌ 缺口 |
| R034 | 存储引擎 | 完全静默（hits=2）：空列表 + 添加按钮，无任何错误 UI | 横幅 ×2 + 重试 ×2 | ❌ 缺口 |
| R036 | 版本信息 | 横幅：原始错误体 + 重试 | 同左 | ✅ 对齐 |
| R039 | 用户信息 | 横幅：原始错误体 + 重试；auth/me 失败不登出跳转 | 同左；同样不跳转 | ✅ 对齐 |
| R040 | 向量数据库引擎 | 完全静默 | 横幅 + 重试 | ❌ 缺口 |
| R041 | WeKnora Cloud | 完全静默 | 横幅：裸 body + 重试 | ❌ 缺口 |
| R042 | 网络搜索 | 完全静默 | 横幅：裸 body（按钮级重试 0；一次探测见游离「重试」文本，不稳定） | ❌ 缺口 |

合计：3 对齐（R029/R036/R039），6 缺口（R028/R030/R034/R040/R041/R042）。

## 探针原始摘录（关键行）

- Vue ollama：`{errHit:false, retry:1, alertNode:1, alertText:"连接失败，请检查 Ollama 是否运行或服务地址是否正确"}`
- Vue parser：`{errHit:true, snippet:"...优先于服务端环境变量，留空则使用环境变量默认值。 R480 simulated failure 重试", retry:1}`
- Vue storage（reload 法）：`{hits:2, errHit:false, retry:0, alertNode:0}`，页面文本止于「添加存储实例」
- Vue system / userprofile：`{errHit:true, alertText:"R480 simulated failure 重试", retry:1}`
- Vue retrieval 抽屉：`{hits:0, retry:0}`（boot 已拉取并缓存，抽屉零请求）
- React storage：`{err500:true, retry:2, alertNode:2}`；React vectorstore：`{err500:true, retry:1, alertNode:1}`
- React ollama（按钮级复核）：`{alertText:"R480 simulated failure", retryBtnCount:0}`
- React retrieval 抽屉：`{hits:1, drawerText:"搜索设置 × R480 simulated failure", retry:0}`（表单被吞）

## 新基线事实（修复轮必须遵守）

1. Vue 资源类分区（storage/vectorstore/websearch/cloud）+ retrieval-config 的失败处理 = **完全静默降级**（空列表/默认表单），不显示横幅、toast 或重试。React 当前为超集错误面——按「Vue 没有的不添加」原则需移除/降级。
2. Vue ollama 失败 = 友好文案横幅 + 重试（吞 body）；React 需补齐语义化文案与重试按钮。
3. Vue 的检索设置唯一入口是命令面板抽屉（`GlobalCommandPalette.vue` L153-L157，t-drawer 420px）；`?section=retrieval` 深链在 Vue 渲染空白。React 双入口（抽屉 + 深链页）为超集，深链页保留与否列入 N015 同类裁决。
4. Vue settings boot 预取全部资源列表（storage-backends / vector-stores / web-search-providers / retrieval-config 等）；「重进分区不重拉」是 Vue 的数据行为，React 的按需重拉行为差异本轮未列为缺口（用户可见错误态才是判定面）。

## 后续队列

- 修复轮（R481 候选）：R028/R030/R034/R040/R041/R042 六行按上表基线修 React（TDD：先红后绿，scoped 套件）。
- R039 密码策略失败变体取证（随修复轮）。
- N015 裁决取证（role-denied 分区双端行为）。
- 终局计划剩余：N021 tool fixture、N015 裁决、C 行清至 ≤1。
