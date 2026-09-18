# WeKnora 微信小程序 · 详细设计与高保真开发交付

版本1.0 · 2026-09-17。延续上一版20页线框与MP00–MP04计划。新增Taro React端，复用现有Go及共享包；本包不是可提交微信审核的生产小程序。

## 先打开这三个入口

| 入口 | 用途 |
|---|---|
| `WeKnora-HiFi.html` | 单文件高保真原型：桌面左侧切20页、中间手机预览、右侧字段/接口/任务说明；窄屏只显示小程序内容 |
| `design/components.html` | 设计令牌、配色、文字、按钮、表单、状态与卡片组件展板 |
| `WeKnora-Task-Plan.md` | 59个任务的完整合并计划，适合分发给开发或代码Agent |

下载后可使用现代浏览器打开HTML。原型所有JS/CSS均在文件内，无CDN、外部字体或真实业务请求。当前验证环境采用set_content加载相同HTML字节，未验证受管环境禁止的file导航；详见07。内容可滚动，交互按钮改变本地演示状态；刷新恢复默认数据。所有账号、额度、价格、任务、资料均为虚构示例。

## 文档索引

| 文件 | 内容 |
|---|---|
| `WeKnora-Detailed-Design.md` | 前端、后端、视觉、20页规格与测试矩阵的合并阅读版 |
| `docs/01-前端详细设计.md` | 分层、目录、平台适配、状态、流、作用域、恢复、文件与工程约束 |
| `docs/02-后端与共享契约详细设计.md` | Go增量、身份绑定、列表/待办/启动选项、数据约束、支付与安全 |
| `docs/03-视觉与组件规范.md` | Quiet Work视觉风格、106令牌、单位映射、组件与状态规格 |
| `docs/04-页面详细设计.md` | 每页路由、目标、ViewModel、组件、字段、接口、校验、交互和验收 |
| `docs/05-任务拆分与并行计划.md` | 59任务总表、依赖、文件锁、阶段门禁；完整卡片见tasks/ |
| `docs/06-测试矩阵与生产发布验收.md` | 28个后续生产验收场景与证据要求 |
| `docs/07-本次验证与交付边界.md` | 本次真正执行的验证，以及未执行的生产验证 |
| `docs/08-开发执行说明.md` | 领取任务、代码Agent上下文、接口变更和合并流程 |

## 开发资产与可维护源

`design/tokens.json`是106令牌的唯一源，采用项目自有schema。构建输出CSS、Taro SCSS和TS。375逻辑画板 → designWidth=750：16逻辑px先生成32设计px；Taro再转换。运行时胶囊/安全区尺寸不得再乘二，细线采用1Px例外。

`prototype/src/high-fidelity.js`与`high-fidelity.css`承载新增视觉页面；`wireframe-base.*`保留原先交互基础；`build-prototype.py`整合为离线文件。原型代码集中用于设计评审，生产实现应按详细设计拆为Taro组件和feature控制器，不能把大HTML页放进WebView冒充实现。

`prototype/pages/`有20个独立初始页；`prototype/page-manifest.json`为页面清单，`developer-mapping.json`把页面连到Taro路径、ViewModel、组件、接口与任务。`prototype/overview.png`与`screenshots/`是实际浏览器渲染截图。

`tasks/task-index.json`为机器可读台账，`tasks/*.md`为59张任务卡，`stages/MP00.md`至`MP04.md`为阶段入口。所有生产任务当前都是planned；页面原型完成不等于对应生产任务完成。

`contracts/mini-read-models.ts`是明确标记为新增的读取投影建议，不替代已有execution/decision/commercial契约，不包含未经渠道核验的支付参数。

## 重新生成与检查

以下命令从解压后的交付包根目录执行；它们构建本地设计资产，不需要WeKnora源码。

```bash
python design/build-tokens.py
python design/build-gallery.py
python prototype/build-prototype.py
node --check prototype/src/app.js
python -m unittest discover -s tests -p 'test_*.py' -v
python tests/check_design_and_traceability.py -v
```

浏览器验证需另安装`tests/requirements-test.txt`中的工具；默认尝试系统chromium，也可用CHROMIUM_EXECUTABLE指定或安装Playwright的Chromium。工具依赖仅供交付验证，不是微信小程序业务依赖。

```bash
python tests/verify_prototype.py
python prototype/build-overview.py
```

## 范围与源码基线

继续使用上版选读基线`9cc91c31ef7853fb50073f92edd92807817c9259`。随包baseline保留原4份依据；本轮没有再次拉取并验证当前GitHub HEAD，也没有克隆整仓、修改、提交、推送、创建Issue或PR。实施先执行MP00差异核查。

没有进行Taro构建、微信开发者工具或真机测试、Go启动、数据库迁移、真实登录/上传/支付、商户资格审核。生产代码与资格核验均是任务计划内容，不是本轮完成声明。
