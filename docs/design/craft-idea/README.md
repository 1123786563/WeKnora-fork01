# WeKnora Craft 设计交付包

代码复核基线：`1123786563/WeKnora-fork01@0cba2f85f8998a28691d564690a1261dcc041643`。

## 阅读顺序

先打开 `craft-wireframes.html`，点击“季度经营分析看板”查看双栏工作台。顶部状态选择器可切换运行、审批、问题、断线、失败、取消、预览过期、只读。版本记录和来源按钮打开抽屉。首页可新建一个没有历史版本的模拟作品。

再阅读 `craft-architecture-review.md`，其中包含源码依据、八份文档的重梳建议、技术边界、API、交互规格、12项增量任务及验收门槛。

## 文件

| 文件 | 用途 |
|---|---|
| craft-wireframes.html | 单文件交互线框，无外部资产或字体依赖 |
| craft-architecture-review.md | 中文架构复核与设计文档 |
| design-tokens.json | 设计令牌提案，正式实现需映射现有组件库 |
| 01-home.png | 桌面创作首页 |
| 02-workbench.png | 桌面对话与作品双栏 |
| 03-approval.png | 等待授权状态 |
| 04-mobile.png | 390px 响应式作品视图 |
| verification*.json | 本地原型检查结果，非业务仓库验收 |
| test_wireframes.py / test_extra.py | 原型浏览器检查脚本 |

## 原型限制

页面、文件大小、hash说明、业务数字、检查结果均为模拟。上传只选择并显示文件名，不读取或上传文件内容。导出只下载明确标注的演示说明，不生成真实网页交付包、DOCX、XLSX或PPTX。刷新页面会恢复内置示例状态。

此 HTML 不使用真实 assistant-ui 依赖，是其目标界面和交互的设计原型，不是 WeKnora 的已接入版本。模板和能力说明页是提案，不代表新增管理API已上线。

本轮通过 GitHub 连接读取关键代码和指定文档的核心契约，没有完成完整 git clone、业务编译、真实后端、模型或沙箱验收。onyxCraft 参考仓库本轮访问404，未完成其源码对标。业务仓库没有被修改或提交。

## 原型验证

验证环境使用 Python Playwright + Chromium。由于执行环境策略限制 file:// 导航，测试直接将同一 HTML 字节载入 Chromium (`set_content`)，不访问外部网络。13项主交互检查和7项补充状态检查分别记录在JSON中。截图来源于该实际浏览器渲染。

重跑脚本需要 Python 的 playwright 包及 Chromium。脚本优先使用 CHROMIUM_PATH 环境变量，其次寻找系统 chromium/google-chrome，否则使用 Playwright 管理的浏览器。测试结果只说明此原型的已列举交互，并非完整可访问性、全浏览器或生产安全认证。

```bash
python test_wireframes.py
python test_extra.py
```

下载 HTML 后可用浏览器打开；受到企业浏览器本地文件策略限制时，可由本地静态文件服务提供。无需安装前端业务依赖。
