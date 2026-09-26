# T33 微信小程序 DevTools 抽查（2026-09-26，复用 T24/T26 模式）

- 构建：`WEKNORA_API_ORIGIN=http://127.0.0.1:57828 pnpm --filter @weknora/miniprogram build:weapp` → EXIT=0（dist/common.js 内联 127.0.0.1:57828，grep -rl 命中）
- DevTools：CLI touristappid + `cli auto --auto-port 9433`；automator 装于仓库外 /tmp/wk-t33-automator（T24 先例）；驱动脚本 wx-driver.cjs（本目录）
- 服务器：Lite 127.0.0.1:57828（与 Web 端同一实例同一 DB——同源跨端抽查）
- 冷编译 settle-retry ≥30s（首轮 reLaunch 报 getPageMetaByWebviewId null，等待后恢复；T24 已知现象）

## 驱动真实输出（node t33-driver.cjs）

[t33] shot 01-login-filled
[t33] consent tap true
[t33] enter workspace true
[t33] PASS login reached home with 求职工作台 entry
[t33] shot 02-home
[t33] PASS career-page web-created profile fact 毕业时间=2026-06 · 修订 2 visible in weapp
[t33] shot 03-career-discovery
[t33] PASS search-entry one-shot search entry present
[t33] PASS apply-material-page 申请与材料|t33owner's Workspace|一份申请，一份材料，|一次本人投递。|与 Web 端同源同版本：同一份档案、同一份申请与结构化正文；材料确认后生成不可变新版本，旧版本不被覆盖。|当前档案修订 2（申请与材料写入将按此修订校验）|待确认事实 0 条 · 已确认 1 条|岗位与资格评估|岗位编号|快照编号|结果未知的写入都可以用原请求编号对账或安全重发（同一请求编号服务端不会重复执行）；空间切换后旧响应一律失效。
[t33] RESULT [{"step":"login","pass":true,...},{"step":"career-page","pass":true,...},{"step":"search-entry","pass":true,...},{"step":"apply-material-page","pass":true,...}]（4/4 PASS）

## 抽查结论

1. 登录（真实 POST /api/v1/auth/login @57828，账号 t33a@t33.io）→ 进入 home ✓
2. 求职工作台（/career/discovery）：Web 端创建的已确认档案事实「毕业时间：2026-06 · 已确认 · 修订 2 · 2026-09-26 20:42」在微信端可见——同源跨端同步实证 ✓
3. 一次性找岗入口与「结果未知可原请求对账/空间切换旧响应失效」说明在页 ✓（搜索执行与诚实失败态已在同服务器 Web 端真实取得：searchId 936c6f6e…「暂无已核验来源，本次未抓取任何数据」；t-button GUI 触达限制按 T24 定论由 172 单测覆盖）
4. 申请与材料页（/career/application-material）：「当前档案修订 2」「已确认 1 条」「岗位与资格评估」「不可变版本」说明呈现 ✓（Web 端同服务器已完成申请→材料→导出→投递全链，见 web-network-requests.log）
5. 截图：wx-01-login-filled.png / wx-02-home.png / wx-03-career-discovery.png / wx-04-application-material.png（本目录）

## 诚实边界

- 真机（手机）验证 blocked：无设备（T06/T24/T26 先例）；本轮为官方 DevTools 模拟器 + tourist appid。
- t-button GUI tap 不被 automator 触达（T24 定论）：「搜索发起/确认/上传/导入」等 t-button 动作的 GUI 触发未复验，业务由 172 项单测（真实装配）+ Web 同服务器真实链组合覆盖。
- 站内信/订阅消息推送需真实 appid 与微信平台配置，DevTools tourist 模式不可核验（T28/T30 已记录）。
