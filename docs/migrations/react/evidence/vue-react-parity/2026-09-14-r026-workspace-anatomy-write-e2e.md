# 2026-09-14 R026 工作区记忆面板对齐 + 个人记忆真实写路径 e2e（S00）

## R026 MemoryWorkspacePanel 剩余视觉差异（Vue MemoryWorkspaceSettings.vue 基线）

修复前 React 以 Card 包裹 + h3 标题 + 泛绿 intro（#f3f9f5，strong+span 行内）+ 下拉写模式 + 右栏自定义提示词。对照 Vue 裸排版逐项修复：

- **裸排版**：去除外层 Card，改 .wk-memory-workspace-settings（Vue .memory-workspace-settings，width 100%）。
- **分区头**：h2 20px/600 + description 14px/rgba(0,0,0,.6)，margin-bottom 24px（Vue .section-header）。
- **intro 盒**：Vue .intro — 中性底 #f3f3f3（--td-bg-color-secondarycontainer）、品牌色 info 图标 + 标题/描述纵向堆叠；替换 React 原泛绿盒。
- **设置行**：.wk-mws-row 20px 0 边框行、label 15px/500、desc 13px/.6、hint 颜色降为 .4（Vue .setting-info .hint）、末行无下边框。
- **写模式控件**：下拉 select → .wk-segmented 单选对（explicit_only/auto），复用 settings-wrapper.css:175-179 既有样式与 GeneralPreferencesPanel 的 radiogroup 惯例，对齐 Vue t-radio-group。
- **自定义提示词行**：右栏窄列 → Vue instructions-row 的纵向堆叠全宽布局。

**验证**：typecheck:web 0 错误；SettingsPage.test 15/15；test:web 856/856、test:shared 444/444、build ✓；live 截图 screenshots/memory-write-20260914/w1-workspace-enabled-react.png（对照 Vue w1-workspace-enabled-vue.png）。

## 个人记忆真实写路径 e2e（R025 上轮登记的开放项）

脚本 .parity-tools/memory-write-e2e.cjs，真实后端 :8080、parity 账号（owner@tenant 10000）。共享态写入仅为该账号自身工作区配置与一条个人记忆，且全程走真实 UI，结束已复原并经 API 复核：

| 步骤 | 操作 | 结果 |
|---|---|---|
| W1 | Vue UI 开启空间长期记忆开关（状态感知：仅在与期望不符时点击，避开前次运行遗留态） | ok:true, after:true |
| W2 | Vue 我的记忆 UI 添加弹层创建条目「Parity 写入验证：偏好使用 Docker 部署」 | ok:true，列表可见 |
| W3 | React 我的记忆看到同一条目（同一用户存储）→ 行内编辑改写 → 保存 | seesVueItem:true, edited:true |
| W4 | React popconfirm 删除 | deleted:true |
| R1 | React UI 关闭空间长期记忆复原 | ok:true, after:false |

**API 复核（复原态）**：GET tenants/kv/memory-config → enabled:false；GET memory/items → Parity 测试条目数 0；GET memory/settings → effective:false。截图 memory-write-20260914/w1/w2/w3 系列。

至此 R025/R026 的真实数据写路径（增/改/删/跨端可见性/配置持久化）已在真实后端双向验证。

## matrix 结构修复

canonical 表 L41 空行（R009/R010 之间）导致表格断开——S00 审计发现 #1 的残留（行内容此前已修复）；已删除空行恢复表格连续性。

## 门禁

test:web 856/856 · test:shared 444/444 · typecheck:web/shared 0 错误 · build:web ✓

## 仍开放

- 空间记忆 auto 写模式的模型选择器（ModelOptionSelect）与 Vue ModelSelector 的弹层级 computed-style 对照。
- 我的记忆/长期记忆的暗色主题与 Wails/native 平台证据。
