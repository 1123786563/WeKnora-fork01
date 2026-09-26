# 交互面板 parity 收官验收（panel-matrix final acceptance）

- 日期：2026-09-25
- 范围：panel-matrix 55 项 px 交互面板项（批 1 全局壳+对话 8 / 批 2 知识库域 19 / 批 3 设置域 18 / 批 4 系统·集成·免登录 10），spec `docs/specs/2026-09-24-interaction-panel-parity.md`
- 事实源：基线册 `baseline.md`（基线 run `auto-scan/2026-09-24T02-39-15`，全量 115 项）；收官终值 run `auto-scan/2026-09-25T10-13-13`（115/115 成功，平均 0.07%，>1% 仅 2 页且均为登记项）；SOP/台账 `docs/migrations/react/tdesign-migration-playbook.md` §6（#24-#28）
- 口径：pixdiff 容差 8、1280×720 稳态截图；豁免纪律=像素级归因+根因代码级取证，禁直接豁免

## 验收判定

**55 项全部落入收敛口径，交互面板 parity 批次判收。**

| 终值分类 | 项数 | 项 |
|---|---|---|
| 0.00% 全零 | 41 | 见下表（批 1 六项 / 批 2 十九项 / 批 3 十二项 / 批 4 四项） |
| 台账 #24 SP13 豁免 | 1 | px-shell-session-more 1.166%（五轮全量复勘稳态终值） |
| 台账 #25/#25 同类豁免（AA 栅格残差 ≤0.113%） | 7 | attach-tooltip 0.001、agent-filter 0.069、add-channel 0.078、embed-agent-filter 0.069、api-create-key 0.113、login/register-lang 0.001×2 |
| 已收敛 AA 残差（baseline 判已修，残差 AA 级，未单列豁免号） | 4 | models-card-more 0.003、storage-backend-card 0.002、websearch-provider-card 0.002、members-rbac-hint 0.043 |
| 漂移（静态底差传导，弹层本体零差；定性见下） | 2 | vectorstore-add-db 0.201、vectorstore-pg-card 0.224 |

非零 14 项无一为弹层本体异构：#24 是 spec 强制的 React-only 功能；#25 族与 AA 残差均为引擎栅格/字形 AA（≤0.113%）；漂移 2 项经 bbox 级取证确认差异带全落静态分区、交互弹层零差。

## 验收轮次数字（2026-09-25，全量 115 页）

数据为验收序列原值，已逐轮对照 `auto-scan/<run>/report.json` 的 `summary`（pages/ok/over_1pct/avg_diff_pct）核实一致：

| 轮 | run | 平均 diff% | >1% 页数 | >1% 明细 |
|---|---|---|---|---|
| 1 | （run `2026-09-25T05-52-41`，验收序列未记 path） | 0.12 | 5 | settings-general 5.439（存量）/ ix-kb-doc-detail 2.435 / px-shell-session-more 1.166 / px-envvars-sandbox-key-hint 1.837 / px-integration-api-create-key 2.374 |
| 2 | 2026-09-25T06-54-20 | 0.09 | 3 | settings-general 5.439 / px-shell-session-more 1.166 / px-integration-api-create-key 2.374 |
| 3 | 2026-09-25T09-06-09 | 0.07 | 2 | settings-general 5.439 / px-shell-session-more 1.166 |
| 4 | 2026-09-25T09-32-16 | 0.07 | 2 | 同上，hot cells 逐字同前 |
| 5 | 2026-09-25T09-53-01 | 0.07 | 2 | 同上 |
| 6 | 2026-09-25T10-13-13 | 0.07 | 2 | 同上（收官终值轮） |

轮 1→轮 2 清除的三项，处置与证据：

- **px-envvars-sandbox-key-hint 1.837→0**：漂移定性见下节（同名类串染，工作区修复）。
- **px-integration-api-create-key 2.374→0.113**：台账 #28 同名串染判例的回退实例，变体锚修复后回到 0.113 平台期终值（hot cells 4,7:2.3 / 7,6:3.1 与平台期同款，归 #25 家族）。
- **ix-kb-doc-detail 2.435→0**（ix-* 存量项，非 55 面板项）：双端各自跨轮自比非零（Vue 0.956% / React 2.775%，均在右下内容区 bbox≈[644,497,1261,689]，pixdiff 复算实测）＝fixture 文档内容态漂移（共享后端被并行测试改动），非代码回归；轮 2 起双端同零。

轮 3 起连续四轮全量 avg 0.07 / over 2 恒定，>1% 仅剩两登记项（settings-general 存量豁免传导 + px-shell-session-more #24），无新增欠账——判定稳态。

## 四批 55 项终值表

基线值引自 `baseline.md`（基线 run 02-39-15）；终值为收官轮 `auto-scan/2026-09-25T10-13-13/report.md`。

### 批 1：全局壳+对话（8 项）——收敛提交 f71293b8f/1d3c0c4c1/96162ac2a/11205900e/69acbada9（run 14-57-58 12/12 达标）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px-shell-user-menu | 0 | 0.00 | ✅ |
| px-shell-session-more | 1.166 | 1.166 | ⚠️ 豁免 #24（SP13 React-only「分享」菜单项，五轮复勘稳态终值） |
| px-chat-sandbox | 15.765 | 0.00 | ✅ 根因 A（SandboxSidePanel 端口） |
| px-chat-addtokb | 75.387（触发器修复后真值） | 0.00 | ✅ 根因 B（右侧 drawer 同构） |
| px-chat-reqinfo | 3.114 | 0.00 | ✅ |
| px-chat-agent-selector | 2.472 | 0.00 | ✅ 根因 C（锚定+行度量） |
| px-chat-model-selector | 1.64 | 0.00 | ✅ 根因 D（自定义 overlay） |
| px-chat-attach-tooltip | 0.342 | 0.001 | ⚠️ 豁免 #25（8px 图标 AA 相位） |

### 批 2：知识库域（19 项）——19/19 全零（run 08-41-39 收敛，收官轮复核同零）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px-kb-faq-breadcrumb | 0.239 | 0.00 | ✅ 27ee21189 |
| px-kb-demo-breadcrumb | 0.234 | 0.00 | ✅ 同上 |
| px-kb-wiki-breadcrumb | 0.237 | 0.00 | ✅ 同上 |
| px-kb-wiki-tab-graph-breadcrumb | 0.254 | 0.00 | ✅ + kbList type 透传 |
| px-kb-wiki-tab-wiki-breadcrumb | 0.255 | 0.00 | ✅ 同上 |
| px-kb-faq-card-more | 0 | 0.00 | ✅ |
| px-kb-faq-kb-info | 3.276 | 0.00 | ✅ 02a6f91b7 |
| px-kb-faq-tagfilter-prefix | 0 | 0.00 | ✅ |
| px-kb-faq-tagfilter-suffix | 0 | 0.00 | ✅ |
| px-kb-demo-tagfilter-prefix | 0 | 0.00 | ✅ |
| px-kb-demo-tagfilter-suffix | 0 | 0.00 | ✅ |
| px-kb-wiki-tagfilter-prefix | 0 | 0.00 | ✅ |
| px-kb-wiki-tagfilter-suffix | 0 | 0.00 | ✅ |
| px-kb-faq-doctype-select | 0（未命中假零，真值 0） | 0.00 | ✅ 触发器已修（代表页改 kb-demo） |
| px-kb-wiki-doctype-select | 0 | 0.00 | ✅ |
| px-kb-wiki-tab-wiki-newpage | 76.154 | 0.00 | ✅ Dialog 同构平移 + mouseAway 整定（判例 #26 先例） |
| px-kb-wiki-tab-wiki-newdir | 1.235 | 0.00 | ✅ WikiFolderActions 弹层平移 |
| px-kb-wiki-tab-wiki-treeview | 2.764 | 0.00 | ✅ |
| px-kb-wiki-tab-wiki-overview | 3.201 | 0.00 | ✅ |

### 批 3：设置域（18 项）——B3 串行流收敛 + 豁免 #27 回退复勘三项转工单修复（7d351553a/da187f072/ddad150ae，run 04-07-05 复勘）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px-userprofile-change-password | 2.949 | 0.00 | ✅ #27 回退后已修（弹层锚定失配改锚 .user-profile-password-popup-overlay） |
| px-mymemory-usage-hint | 0 | 0.00 | ✅ |
| px-models-model-card | 62.438 | 0.00 | ✅ SettingDrawer model-editor 家族 + ModelEditorDialog 移植（19-08-35 轮 0） |
| px-models-card-more | 0.764 | 0.003 | ✅ 已收敛（§13d dropdown AA 残差） |
| px-parser-engine-builtin | 65.353 | 0.00 | ✅ 已收敛（cae2d3269；AA 残差 0-0.002 轮间波动，收官轮 0） |
| px-storage-backend-card | 21.599 | 0.002 | ✅ 已收敛（35ccd539c；AA 残差） |
| px-storage-card-more | 1.14 | 0.00 | ✅ 已收敛（触发器已命中） |
| px-vectorstore-add-db | 10.629 | 0.201 | ⚠️ 漂移项（静态底差传导，见下节定性；弹层本体零差） |
| px-vectorstore-pg-card | 78.729 | 0.224 | ⚠️ 漂移项（触发器双端未命中+静态底差，见下节定性） |
| px-sandbox-what-is-hint | 0 | 0.00 | ✅ |
| px-envvars-sandbox-key-hint | 1.837 | 0.00 | ✅ 漂移项已归零（同名类串染修复，见下节定性；轮 2-6 恒 0） |
| px-skills-add | 56.966 | 0.00 | ✅ 已收敛（9f274c977；mouseAway 指针工件判例 #26） |
| px-mcp-add-service | 54.278 | 0.00 | ✅ SettingDrawer mcp-drawer 家族 + McpServiceDialog 移植（19-08-35 轮 0） |
| px-websearch-provider-card | 14.26 | 0.002 | ✅ 已收敛（AA 残差） |
| px-websearch-card-more | 0.775 | 0.00 | ✅ 已收敛（触发器已命中） |
| px-platform-api-keys-create | 62.964 | 0.00 | ✅ api-key-create-drawer 家族 + 分组全选（19-08-35 轮 0；#28 串染修复后无回退） |
| px-members-rbac-hint | 10.919 | 0.043 | ✅ #27 回退后已修（.permissions-compact 族重平移 + DOM 补齐；AA 残差级 0.043） |
| px-ollama-redetect | 2.496 | 0.00 | ✅ #27 回退后已修（MessagePlugin t-message 同构） |

### 批 4：系统/集成/免登录（10 项）——11/11 ≤0.113 收敛（run 03-07-18；提交 36a610456/1ce982001/891d0ab69/5fbde2205/7edec47b3）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px-system-auth-priority | 2.984 | 0.00 | ✅ t-popup 同构（#15 方位粒度） |
| px-system-create-user | 84.547 | 0.00 | ✅ 同上（A5 popover vs dialog 根因） |
| px-integrations-agent-filter | 1.562 | 0.069 | ⚠️ 残差豁免 #25 同类（边框亚像素+字形 AA） |
| px-integrations-add-channel | 59.292 | 0.078 | ⚠️ 残差豁免 #25 同类（#28 SettingDrawer 同构） |
| px-integration-embed-agent-filter | 1.558 | 0.069 | ⚠️ 残差豁免 #25 同类 |
| px-integration-api-create-key | 71.577 | 0.113 | ⚠️ 残差豁免 #25 同类（轮 1 曾回退 2.374，#28 变体锚修复后回平台期） |
| px-login-lang | 0.001 | 0.001 | ⚠️ 豁免 #25 同类（7px 语言图标字形 AA，几何对齐） |
| px-register-lang | 0.001 | 0.001 | ⚠️ 同上（同源图标） |
| px-login-register-confirm | 0 | 0.00 | ✅ 基线即零 |
| px-register-register-confirm | 0 | 0.00 | ✅ 基线即零 |

（批 4 附静态项 settings-integration-api 4.284→0，5fbde2205 mode-callout 补齐，收官轮 0 复核通过。）

## 漂移三项定性

三项的共同画像：**终值随环境/静态分区态浮动，交互弹层本体零差**。取证均为本次收官实测（pixdiff 复算 + report.json warnings + 只读 DOM 探针）。

### 1. px-envvars-sandbox-key-hint（1.837 → 0.00，轮 2 起恒 0）

- **现象**：基线至轮 1（05-52-41）恒 1.837%，bbox [440,120,801,246]；轮 2（06-54-20）起连续四轮全量 0.00%。
- **归因（跨轮自比实测）**：05-52-41 vs 06-54-20 截图自比——Vue 端 0.0%、React 端恰 1.837%（差全部在 React 侧变化）；settings-envvars 静态页双端跨轮均 0。＝React 侧渲染变化，非扫描噪声。
- **根因（代码级，settings.td.css 修复注记）**：`.hint-popover` 在 settings.td.css 存在两份裸全局定义——env 域（EnvVarSettings.vue：gap 12 + `__text` 上距 4px）与 sandbox 域（SandboxSettings.vue：gap 4 + 无上距）；hint 弹层 portal 到 body 后无 scoped 祖先可锚，后出现的 sandbox 版无条件覆盖 env 版 → envvars 沙箱密钥提示弹层用 sandbox 排版渲染，1.837%。
- **修复**：变体类分域——`hint-popover--env`（EnvVarSettingsPanel.tsx:115）/`hint-popover--sandbox`（SandboxSettingsPanel.tsx:1817），settings.td.css 裸定义改为基座+变体（settings.td.css:767-790），§3438 段删除裸定义。轮 2-6 恒 0 验证。
- **定性**：**同名类全局串染（域级 CSS 冲突），已修复归零**。与 #28 同名串染判例同族（portal 弹层的同名类必须变体分域锚定）。

### 2/3. px-vectorstore-add-db（0.121 → 0.201）与 px-vectorstore-pg-card（0 → 0.224）

- **现象**：两项自 2026-09-24T18-37-06 起恒定 0.201/0.224（含轮 1-6 全部六轮，hot cells 逐字相同），此前 12-48-10 轮分别为 0.121/0。settings-vectorstore 静态页同步 0→0.224。
- **归因（bbox 级实测）**：收官轮 pixdiff 复算——settings-vectorstore、px-vectorstore-pg-card、px-vectorstore-add-db 三者 diff bbox 完全同一：[358,282,647,294]（一条 290×13px 文本行带）。交互项残差＝静态分区底差原样传导，弹层本体零差。跨轮自比：双端 05-52-41 vs 10-13-13 均 0.0%（确定性结构差，非噪声）。
- **触发态变化（report.json warnings 实测）**：收官轮 px-vectorstore-pg-card 双端触发器未命中（`vue 未命中 {"clickCss":[".backend-card.is-env"],"clickText":["PostgreSQL"]}` + react 同款）——fixture 后端的 env PostgreSQL 向量库在 09-24 12:48-18:37 之间消失（共享后端环境态变化，非本仓代码事件），页面进入空列表态。
- **根因（代码级，DOM 探针实测）**：空列表态下双端空态语义分歧——React 在 (400,288) 命中 `P.wk-status`「尚未配置向量数据库。点击"添加数据库"开始设置。」（rect [357,279,760,18]，ResourceSettingsPanel.tsx:1091 的 role 豁免只覆盖 websearch 分区）；Vue 同点位命中 content-wrapper 背景（VectorStoreSettings.vue:20 `v-if="stores.length === 0 && !authStore.hasRole('admin')"` 对 admin 隐藏空态）→ 扫描 admin 账号下 React 多渲染一行状态文本，即 0.224% 静态带。px-vectorstore-add-db 抽屉正常打开，残差 0.201% 为同一静态带（bbox [358,282,646,294]）。
- **定性**：**环境态漂移（后端 env 向量库消失）揭出的潜在空态 role-gate 语义分歧**——代码级差异真实存在（ResourceSettingsPanel.tsx:1091 未对 vectorstore 分区复刻 Vue 的 admin 隐藏语义），但仅在空列表+admin 路径可见，且属静态分区欠账而非交互弹层缺陷。**处置：转静态页工作单池**（与 settings-general 5.439 存量同性质），对齐方案=vectorstore 空态纳入 role 豁免或按 Vue 源逐分区判定；后端 env 向量库恢复后该带自动消失（基线期两项即 0/0.121）。px-vectorstore-add-db 基线期的 0.002-0.121 focus 边框相位波动已被该静态带主导。

## 豁免台账增量（#24-#28）

台账正文见 `docs/migrations/react/tdesign-migration-playbook.md:205-209`（§6 DOM 差异台账），本节收官摘要：

| # | 名称 | 性质 | 本轮增量 |
|---|---|---|---|
| 24 | 会话行「更多」菜单「分享」项 | SP13 spec 强制的 React-only 功能豁免（px-shell-session-more 恒 1.166%＝一枚 32px 分享行+其下移位带） | 第二~五轮全量复勘入册：恒 1.166%、hot cells 逐字同前、双端各自跨轮自比 0px、菜单盒 rect 探针（React [81,274,162,219] 六项 vs Vue [86,279,152,176] 五项）；连续三轮恒定判定**稳态终值，后续轮次引用本行即可** |
| 25 | composer 附件 tooltip 图标 AA 相位（引擎栅格伪影，#16/#18/#22 亚像素家族） | 扫描豁免 | 批 4 五项「#25 同类」引用（agent-filter/add-channel/embed-agent-filter 0.069-0.078、api-create-key 0.113、login/register-lang 0.001）+ 批 1 attach-tooltip 0.001 |
| 26 | headless :hover 重算指针工件 | 扫描口径判例（mouseAway） | 批 3 px-skills-add 复现引用（56.966→0） |
| 27 | 静态底差转交（弹层同构） | **已回退证伪** | 三项实证（userprofile/ollama/members）逐项证伪后全部转工单修复，终值 0/0/0.043（7d351553a/da187f072/ddad150ae，run 04-07-05）；教训入册：终值与静态基线同值不构成弹层同构证明，豁免前必须弹层 rect 级 DOM 探针实证 |
| 28 | packages/views 抽屉族 SettingDrawer 模拟层 + 同名串染判例 | **本轮新增**（1d1f6f695 登记） | 模拟层三定律（open 态 transform 定格/直接子选择器/popper 取整微调）+ 批 4 实证（add-channel 59.292→0.078、api-create-key 71.577→0.113）；**判例④实例回退**：api-create-key 0.113→2.374（轮 1 复现）＝两 Vue 源共用 `.api-key-create-drawer` 但 scope-hint 规则不同源，简写/长写 margin 级联叠加残留 24px；修复=差异规则锚 `pak-create-drawer` 变体类（PlatformApiKeysPanel.tsx:286、settings.td.css:7131/7172），终值回 0.113 且 platform 侧无回退 |

## 存量与移交（非本批次欠账）

- **settings-general 5.439%**：SP14 T1 React-only「套餐与额度」卡豁免传导项（GeneralPreferencesPanel.tsx:256 注记；像素归因见 task-12a），六轮恒定，登记待归属域。
- **vectorstore 空态 role-gate 分歧**：本文档漂移定性新发现，转静态页工作单池（见上节）。
- **markdown `repairFlankingEmphasis` 跨域欠账**：SOP 已登记（chat 域归属），与面板批次无关。

## 收官时点声明（可审计性）

- 轮 2 起生效的两处修复（envvars hint 变体分域、api-create-key `pak-create-drawer` 变体锚）在本收官提交时点位于**工作区未提交**（`git status` 实测 M：EnvVarSettingsPanel.tsx / SandboxSettingsPanel.tsx / PlatformApiKeysPanel.tsx / settings.td.css），其效果由 06-54-20 起四轮全量（含收官轮 10-13-13）验证；本提交只含本文档（收官纪律：git add 仅清单文件）。
- 本文档全部数字为各 run `report.md`/`report.json` 原值；漂移定性所引 bbox/自比/warnings/DOM 探针均为本次收官实测（pixdiff.py 复算 + report.json 读取 + 只读 Playwright 探针，未点击任何确认类按钮）。

---

# 三期（px2-*）：无弹层交互面收官验收（2026-09-26）

- 范围：panel-matrix 三期 13 项 px2-* 无弹层交互面（kb 域 6 / settings 域 5 / system 域 1 / chat 域 1），基线册 `baseline-px2.md`（入表提交 ac5eccbb8；基线 run `auto-scan/2026-09-26T04-04-22`，13/13 成功，>1% 4 项入工单池）
- 分支序列：pp2/kb（合入 1a06fcc2a，收敛提交 7cb908b99）→ pp2/settings（合入 2f3093daf，收敛提交 86f841db7 + 台账 #29 Tabs 指示条 c57860040）→ pp2/system（合入 50a1612ad）→ pp2/chat（合入 1c17a15cd，收敛提交 458dd8204 + 台账 #29 zoom snapping）
- 口径：与前两期同源（pixdiff 容差 8、1280×720 稳态截图；headless 指针工件走 #26 mouseAway 判例；toast 瞬态 postSettle 3500）；豁免纪律不变＝像素级归因+根因代码级取证，禁直接豁免

## 验收判定

**终值层面 13/13 落入收官口径：6 项 0.00 全零收敛 + 7 项登记豁免/传导终值，13 项范围内无未登记欠账。**三期最终判收（全量稳态复勘轮）因验收轮失效待补，见「验收轮次」节。

| 域 | 项数 | 0.00 全零 | 登记豁免/传导终值 |
|---|---|---|---|
| kb | 6 | 4 | 2（#19：settings-nav / settings-chunkswitch 0.003） |
| settings | 5 | 1 | 4（models 0.005＝#16+#29、sandbox/systemglobal 0.001＝#29、fontradio 5.066＝SP14 传导） |
| system | 1 | 1 | 0 |
| chat | 1 | 0 | 1（sidebar-collapse 0.238＝#29 zoom snapping） |
| 合计 | 13 | 6 | 7 |

chat 域唯一项为豁免终值，按域口径**收敛 0 / 豁免 1**（16.06→0.238 的修复链已收敛至登记终值，不另计收敛项）。口径勘误：初记录 chat 域 converged=2 系误计——把 zoom=1 回归复扫（附带工作，不入终值表）计成了收敛项；正解四域合计 6+7=13。

## 13 项终值表

基线值引自 `baseline-px2.md`（基线 run 04-04-22）；终值为各域收敛 run 的 report.json 原值（逐项注明）。

### kb 域（6 项）——pp2/kb 收敛提交 7cb908b99；终值 run `auto-scan/2026-09-26T06-22-01`（6/6 成功，over_1pct=0 / avg=0，summary 原值）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px2-kb-wiki-tab | 0 | 0.00 | ✅ 基线即零，终值轮复核 0（06-06-44 轮曾现 3.928 瞬态，06-10-22 / 06-22-01 复扫归 0） |
| px2-kb-graph-tab | 0 | 0.00 | ✅ 终值轮复核 0 |
| px2-kb-wiki-reader-tab | 0 | 0.00 | ✅ 终值轮复核 0 |
| px2-kb-wiki-tree-expand | 0 | 0.00 | ✅ 终值轮复核 0 |
| px2-kb-settings-nav | 7.478 | 0.003 | ⚠️ 豁免 #19（弹窗四角圆弧 AA 残差 28px，与 ix-kb-settings 0.003 同签名；分块段 KBChunkingSettings.vue 同构重写） |
| px2-kb-settings-chunkswitch | 9.409 | 0.003 | ⚠️ 豁免 #19（同上；tdesign 控件平移+滚动锚定+分隔符折行复刻，取证链见 `baseline-px2.md:31-41`） |

### settings 域（5 项）——pp2/settings 收敛提交 86f841db7；终值 run `auto-scan/2026-09-26T05-45-02`（6 页含 chat 项，PARITY_REACT_URL=:5292）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px2-settings-models-tab | 0.005 | 0.005 | ⚠️ #16 play-circle 28px（静态存量豁免）+ #29 指示条 14px；基线即登记口径终值，收敛后复核恒定 |
| px2-settings-sandbox-tab | 0.001 | 0.001 | ⚠️ #29 指示条边缘 AA 12px（y244-246 x424-551，单灰阶级） |
| px2-settings-mymemory-tab | 0 | 0.00 | ✅ 第 2 tab offset 落整数相位，#29 不显形 |
| px2-settings-systemglobal-tab | 0.001 | 0.001 | ⚠️ #29 指示条边缘 AA 13px（y176-178 x456-571） |
| px2-settings-general-fontradio | 25.519 | 5.066 | ⚠️ SP14 套餐卡静态传导（存量豁免）；zoom 应用机制/toast 呈现/扫描瞬态三根因已修，React 藏卡+双端回滚顶实验 diff 0.000%（`baseline-px2.md:51`） |

（zoom/壳改动的静态回归复扫：run 05-43-25 六页 kb-list 0 / settings-general 5.439 不变 / mymemory 0 / models 0.003 / sandbox 0 / system-global 0——与存量一致，零回归。）

### system 域（1 项）——pp2/system；终值 run `auto-scan/2026-09-26T04-43-02`（1/1 成功 0%，PARITY_REACT_URL=:5293，worktree 复扫，目录在库）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px2-settings-runtimequeues-autorefresh | 0 | 0.00 | ✅ 基线即零（纯客户端 5s 轮询开关，无服务端写；受影响域测试 70/70 绿） |

### chat 域（1 项）——pp2/chat 收敛提交 458dd8204；终值 run 05-19-53（worktree 轮，主 checkout auto-scan/ 无该目录，数值引自已入册 `baseline-px2.md:63` 与台账 #29）

| 项 | 基线% | 终值% | 状态 |
|---|---|---|---|
| px2-chat-sidebar-collapse | 16.06 | 0.238 | ⚠️ 豁免 #29 zoom snapping（zoom 1.125 文本行基线光栅差 ≈2192px；前置修复链 4 项＝html zoom 机制/壳高度链/is-sidebar-collapsed 类/composer chip 盒模型。基线 16.06 含 zoom 错配分量，zoom 修复后真值 9.094＝run 05-45-02，chat 工单修复后 0.238） |

（附带工作不入终值表：zoom=1 生产默认态回归复扫 chat/agents/login/settings-general 0/0/0.001/5.439 与存量一致——台账 #29（playbook:210）尾注，run 05-34-25 同为 worktree 轮、主 checkout 无该目录。）

## 验收轮次（三期）——失效待补（如实记录）

三期验收序列设计为参考轮 + 3 轮全量容差验收（连续绿 3/3 判稳态）。**实际无有效验收轮：**

- 主会话核实：会话中断杀掉 dev 服务栈，三轮全量扫描秒败（exit 1，`auto-scan/` 未写 run 目录）；工作流解析脚本回退读到了最新的 PAGES 过滤轮报告（`auto-scan/2026-09-26T06-22-01`，kb 6 项复扫 over_1pct=0 / avg=0），产出三轮 violations=0 / avg=0 的**伪绿数据**。
- 处置：该序列数据无效——不入册为已验收、不记 PASS；对应 run id 不存在，不补写。「连续绿 3/3」当前**不成立**。
- 真实全量验收由主会话补跑后回填本节；补跑前，本文档三期判定仅以各域终值 run + 登记终值为据（见上表）。

## 豁免台账增量（三期）

台账正文见 `docs/migrations/react/tdesign-migration-playbook.md` §6。本期新增 2 条（均像素级+根因代码级取证入册，无直接豁免），引用既有 3 处；7 项登记终值合计＝#19×2 + #16/#29×3（models 为 #16+#29 复合）+ SP14 传导×1 + #29 zoom×1：

| 台账条目 | 性质 | 本期角色 | 取证要点 |
|---|---|---|---|
| #29 Tabs 活动指示条 transform/left 光栅路径族（playbook:215，c57860040 入册） | **三期新增（settings 域）** | 消费方：px2-settings-{models,sandbox,systemglobal}-tab 0.005/0.001/0.001 | DOM barRect 逐字段一致（Δ3e-5px）+ 两库代码级定位差（transform vs left）+ 决定性实验：React 同值 left 替换后双端 diff 0px + 无页面 seam（offset 随文本测量动态变化） |
| #29 html zoom（字号档位）文本行基线光栅 snapping（playbook:210，458dd8204 入册、1c17a15cd 并入） | **三期新增（chat 域）** | 消费方：px2-chat-sidebar-collapse 0.238 | 双端跨 run 自比 0px + computed 32 项/canvas measureText 逐项同 + 同元素行盒 fraction 分裂实证 + headed GPU 光栅下差异带换位而非消失 |
| #19 Dialog 弹窗圆角弧线 AA 阶梯（playbook:200） | 既有引用（kb 域） | px2-kb-settings-nav/chunkswitch 0.003＝28px 全落四角圆弧带，带外 0 差分 | 一期已取证（ix-kb-settings 同签名） |
| #16 play-circle 图标 AA（playbook:197） | 既有引用（settings 域） | px2-settings-models-tab 0.005 中的 28px 分量 | 静态存量豁免（settings-models 静态页同源 0.003） |
| SP14 套餐卡存量豁免传导（一期「存量与移交」登记，本文件上章） | 既有引用（settings 域） | px2-settings-general-fontradio 终值 5.066＝settings-general 5.439 的 zoom 态传导 | React-only 功能（GeneralPreferencesPanel.tsx:256 注记）；藏卡实验 0.000% |

（编号注记：台账现存两条 #29 并存——playbook:210（zoom snapping，chat）与 :215（Tabs 指示条，settings），系两分支并行入册撞号；本文引用一律以行号消歧。）

## 收官时点声明（可审计性）

- 四域合流已并入 main：1a06fcc2a（pp2/kb）→ 2f3093daf（pp2/settings）→ 50a1612ad（pp2/system）→ 1c17a15cd（pp2/chat，含 `baseline-px2.md` 冲突解）。
- 本提交只含本文档（收官纪律：git add 仅清单文件），不 push。
- worktree 轮目录说明：system 终值轮 04-43-02 目录在库；chat 终值轮 05-19-53 与 zoom 回归轮 05-34-25 产生于 pp2 worktree（隔离端口），未随合并进入主 checkout `auto-scan/`，数值以已入册记录（`baseline-px2.md` / 台账 #29）为据并逐处标注。
- 本章节全部数字为各 run report.json 原值或已入册文档（`baseline-px2.md`、台账 #19/#16/#29）原值；验收轮失效事实与处置按主会话核实结论如实记录，未把无效轮次记为 PASS、未虚构 run id。
