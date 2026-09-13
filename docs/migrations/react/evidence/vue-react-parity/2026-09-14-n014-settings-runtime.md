# N014 设置抽屉运行时证据

日期：2026-09-14

## React 浏览器

- 地址：`http://localhost:5181/platform/settings?section=general`
- 条件：已认证本地 Chrome、zh-CN、默认浅色主题。
- 分区切换：点击“用户信息”后，URL 保持设置入口并显示“用户信息”标题、账户信息和修改密码表单；旧“常规设置”内容消失。
- Escape：在设置 dialog 上按 Escape 后 URL 为 `/platform/knowledge-bases`，设置 dialog 数量为 0，确认抽屉已关闭。
- 关闭按钮：重新打开设置后点击“关闭设置”，Chrome URL 回到 `/platform/knowledge-bases`，设置容器从可访问树消失，页面焦点回到 WebArea；该路径与 Escape 共用关闭逻辑。

## Vue 对照限制

- Vue `http://localhost:5180/platform/knowledge-bases/kb-1?tab=graph` 当前重定向到登录页，未取得同一认证/租户条件，因此本条不宣称双端同条件视觉或焦点恢复通过。
- 本条证明 React 的分区切换、关闭按钮和 Escape 关闭运行时行为；尚未证明焦点恢复到打开源元素（Vue 本身也只做 blur）、computed-style、Vue 同条件截图及 Wails/native 证据。
