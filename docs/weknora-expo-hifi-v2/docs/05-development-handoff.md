# 开发交接与原型使用

## 1. 打开方式

解压后首先打开 `index.html`。它是自包含文件，没有在线字体、CDN库或外部脚本。顶部切换场景与主题；左侧切18页；右侧查看页面说明/令牌。移动宽度使用顶部页面选择器。`pages/`每个文件可单独打开，默认进入对应页面。`design-system.html`默认显示组件与令牌总览。

HTML在普通浏览器可作为本地演示；本轮自动化环境不允许file://及localhost导航，采用Chromium `set_content`渲染同一完整HTML。因此本轮证明的是渲染和交互，不是该环境中的本地文件导航、HTTP托管、刷新持久化或PWA安装。工具限制与结果见verification报告。

## 2. 原型支持什么

18页导航、双主题、字体125%展示、场景切换、新建任务表单、Agent/目标选择、知识关联、模拟附件校验、输入与搜索、一次审批确认、冲突处理、取消待确认、未知请求对账、按空间隔离的草稿、可编辑转写→未发送草稿、Markdown示例下载。

不支持：真实认证/SDK/Go后端/数据库/推送/摄像头/麦克风/原生分享/Provider/支付；不含全部W32高级Happy交互；M09完整演示tool审批，其他交互类型在详细设计与MX-020定义。模拟主体、文件与Credits不对应真实用户数据。

## 3. 开发读取顺序

先读README → `docs/01-detailed-design.md` → `docs/02-screen-specifications.md` → `docs/03-design-system.md` → `docs/04-api-contracts.md` → `plans/00-implementation-master.md`。具体任务再读对应阶段分册和原W/T/H完整任务；不能仅拿截图写代码。

## 4. 文件角色

| 位置 | 角色 |
|---|---|
| index.html / pages | 高保真交互基准；不是RN业务源码 |
| prototype-src | 原型源文件，便于审阅与迭代；不要复制全局演示model到产品领域 |
| tokens/tokens.json | 颜色与尺寸源 |
| tokens/tokens.css | Web输出 |
| tokens/native-tokens.ts | RN数值输出；集成到现有主题系统 |
| contracts/mobile-proposal.ts | 契约草案与形状示例；正式使用前与现有Go/TS冻结 |
| plans/task-index.json | 36个MX任务、依赖、所有权、验收与原计划映射 |
| scripts | 本交付生成与静态检查工具，不执行仓库初始化 |
| references | 原始输入与本轮官方核验来源 |
| verification | 只对本交付工件的检查记录 |

## 5. 生成与检查

```bash
# Python3标准库即可生成；不会安装生产依赖。
python scripts/generate_tokens.py
python scripts/build.py
python scripts/validate.py
# 原型纯状态机测试（需要Node）
node --test verification/model.test.cjs
# 提案和原生令牌的静态类型检查（需要TypeScript）
tsc --noEmit --strict --target ES2022 --module ESNext tokens/native-tokens.ts contracts/mobile-proposal.ts
```

浏览器检查需要已安装Python Playwright与Chromium，脚本通过环境变量CHROMIUM指定可执行文件；不要把它当Expo测试。源文件改变后重新生成逐页HTML并重跑验证，更新截图与报告，不手改build输出。

## 6. 接入Expo的次序

MX-001先对账当前HEAD与旧设计差异。MX-007/008落主题与基础组件；MX-003/004/005/006先冻结能力、SSE、审批和请求语义。MX-009以后接真实产品host，不把Happy全局token迁入新scope；核心链完成后接通知和资源，再按profile加入远程与语音。

Screen只消费VM；API只在SDK/Controller；组件不读取localStorage。原生Provider和SQLite密钥通过根装配注入。真实上线必须有明确profile的iOS与Android证据、后端与数据库证据、外部服务授权证据。

## 7. 本次未改动的范围

未修改GitHub仓库、创建Issue/PR、提交/合并代码、部署服务、改变旧台账状态、向第三方写数据或支付。新增MX任务全部pending，表示工程实施尚待完成；高保真原型完成不改变它们的状态。
