# r101 iOS Metro 连接验收

## 尝试

1. 启动 `expo start --dev-client --lan --scheme weknora`，Metro 在宿主机 `192.168.3.30:8081` 返回 HTTP 200。
2. 使用 Expo CLI `i` 和 `weknora://expo-development-client/?url=http://192.168.3.30:8081` 打开 iPhone 17 Pro 模拟器。
3. 重新安装并启动 `com.weknora.mobile` 后观察屏幕与系统日志。

## 结果

- Metro 成功打包 Expo Router：`Bundled ... (1376 modules)`。
- 当前 worktree 的 `WeKnora` 进程仍停留在 Development Build 选择页，未加载业务 bundle。
- `simctl log show` 显示模拟器到 `8081/8082/8083` 的连接被拒绝，业务页面未进入。此前同一模拟器中出现的 `Unistyles was loaded, but it's not configured` 日志属于另一个 `WeKnoraMobiledev`（`happy-mobile-verify` worktree）进程，不能归因到本 worktree。
- 当前证据只能证明原生编译/安装/进程启动和 Metro 宿主机打包，不能证明 iOS 业务首屏或认证交互。

## 后续动作

需要在模拟器可达的网络地址上提供 Metro（或生成包含 JS bundle 的 release/preview 包）后再继续业务交互验收；另一个 worktree 的 Unistyles 日志不作为本分支缺陷证据。
