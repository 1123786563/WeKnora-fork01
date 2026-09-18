# WeKnora Craft · 详细设计与高保真开发包

日期：2026-09-18  
审阅基线：`1123786563/WeKnora-fork01@0cba2f85f8998a28691d564690a1261dcc041643`

## 从哪里开始

**查看产品：**用浏览器打开根目录 `index.html`。CSS、脚本与图标均内嵌，没有CDN，无需npm安装。不支持运行脚本的文件预览器只会展示静态内容，请使用完整浏览器。当前执行环境的file://导航被策略阻止，因此本包浏览器验证使用精确HTML内容的离线渲染，不声称已测试你的本机双击行为。

本机策略也限制本地脚本时，可在本目录执行 `python -m http.server 8000`，浏览器打开 `http://localhost:8000/index.html`。这只服务本地文件；不要将该原型作为企业生产站点部署。命令未替你在本机执行。

**查看设计：**先读 `docs/01-详细设计.md` 和 `docs/02-UI设计与令牌.md`。  
**开始开发：**读 `docs/03-实施计划.md`，先执行T001；36张任务卡在 `docs/tasks/`。  
**接口协作：**读 `docs/04-API与状态契约.md` 和 `examples/command-bridge.types.ts`。  
**验收与交接：**读 `docs/05-验收与执行提示词.md`、`qa/验证报告.md`。  
**核对证据：**读 `docs/06-来源与审阅边界.md`。

## 原型操作

首页输入目标与选择类型后点击开始创作；第一轮没有虚构的旧版本。点击“完成本轮演示”才模拟发布。作品库可按类型筛选和搜索；网页、文档、表格、演示稿各有可见预览。

工作台顶部来源/版本/导出打开Drawer。选择“查看此版本”只改变预览；“从此版本继续”需要确认并改变基线。右上齿轮切换执行、审批、问题、断线、失败、取消、只读、预览过期和结果不明。

输入修改要求后发送会增加一轮；再次完成才新增交付版本。批准先显示已记录，再由“模拟远端确认送达”推进；两者不混淆。导出仅下载明确标注的HTML示例，不生成真实DOCX/XLSX/PPTX。浏览器刷新会重置内存mock，没有后台持久化。

## 目录

```text
index.html                       单文件高保真原型
build.py                         从分离源码重新内嵌构建
prototype/                       shell.html / app.css / app.js / icons.json
design/                          tokens.css / tokens.json / tailwind-bridge.css / notices
docs/                            6份说明 + 36任务卡 + tasks.json
examples/command-bridge.types.ts  目标内部端口，不是当前HTTP DTO声明
screenshots/                     桌面、抽屉、不同作品与窄屏截图
qa/                              测试脚本、行为/对比度结果、实际验证报告
design-qa.md                     与原线框的结构和视觉复核
SHA256SUMS.txt                   交付文件校验值
```

## 修改原型与重建

修改 `prototype/app.css`、`prototype/app.js` 或 `design/tokens.css` 后运行：

```bash
python build.py
```

该命令只需要Python标准库。它不安装assistant-ui或构建生产React。不要把独立HTML的整页脚本直接粘贴进React，按规范中的包边界拆成组件与适配器。

## 可选：重跑原型检查

需要Python Playwright和兼容Chromium。环境有系统chromium时自动使用；也可以设 `CHROMIUM_PATH`。使用Playwright自带浏览器时先安装其浏览器依赖。下面只测试原型，非业务仓库：

```bash
python -m pip install playwright
python -m playwright install chromium
python qa/initial_contract_test.py
python qa/verify_design.py
python qa/verify_prototype.py
```

无需为查看index.html安装这些依赖。测试脚本会覆盖qa结果与截图。生产目标命令在任务卡内，尚未执行。

## 结果与限制

独立HTML行为检查39/39通过；3项初始合同检查通过；11组实际采用的核心文本配色达到4.5:1目标；任务依赖为无环，36任务卡齐全。原始亮绿白字与原始灰字/浅灰底的不足组合也记录在对比度结果中，未作为本包对应的默认动作/次文本组合使用。不是全面无障碍认证。

没有修改或部署GitHub仓库，没有完成真实assistant-ui、Go、tRPC-Agent-Go、OpenCode、权限隔离、商业计量或Office生成集成。仅提供浅色Web设计；不是移动App/小程序的实现。所有内容为模拟资料。图标许可见design目录；未附字体。
