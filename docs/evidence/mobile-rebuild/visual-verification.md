# 视觉验证记录（RW-031）· 原生截图

日期：2026-09-18。环境：iPhone 17 Pro 模拟器（iOS 26.x，Xcode 27A266a）、dev 构建（xcodebuild iphonesimulator Debug，BUILD SUCCEEDED）、Metro 8085、EXPO_PUBLIC_VISUAL_FIXTURE=1（显式视觉对照模式，非生产默认）。

## 产物

`docs/evidence/mobile-rebuild/screenshots/native/`：18 页 × light/dark = 36 张 PNG（simctl io screenshot 原生渲染）。

## 抽样核对结论（视觉分析复核）

| 页面 | 结论 |
|---|---|
| M01 登录 | ✓ 品牌图标/双行标题/邮箱密码表单/绿色主按钮/企业 SSO/隐私说明/私有服务器入口全部渲染；浅色主题正确 |
| M03 工作台 | ✓ 顶部头像+空间切换+铃铛；浅绿 Hero（"把想法交给 Agent，把时间留给自己"+新建任务）；底部四 Tab（工作台/会话/资源/我的，图标 home/chat/grid/user 与设计一致）；无布局破损。空间名"未选择空间"符合预期（深链进入无 identity，真实模式数据来自 bootstrap） |
| M08 执行详情 | ✓ 三状态清单独立三行（任务状态/执行观察"等待同步"/结算状态"待对账 · 非最终消耗"）+最近同步；未知状态显示等待同步，未冒充完成；浅色主题正常 |

## 已知视觉事实（非缺陷）

- 深链直接进入页面时 identity 为空（未走登录流程），卡片标题等真实数据字段显示空态/兜底文案；这是数据状态不是布局缺陷。
- 指标/待处理等区块位于 Hero 下方滚动区，首屏以 Hero 为主（与设计首屏层级一致）。

## 未完成项（如实记录）

- 逐页像素级人工比对表（36 张 vs 设计包 screenshots/）未完成——仅完成 3 页深度抽样复核 + 全量截图存档。
- 360/430 逻辑宽度、200% 动态字体、键盘/长文本压力场景未执行（需模拟器自动化 UI 输入，idb 不可用：blocked-env）。
- Android 双平台截图未执行（blocked-env：Android 模拟器未启动；构建链路已由 iOS xcodebuild + bundle export 验证）。
