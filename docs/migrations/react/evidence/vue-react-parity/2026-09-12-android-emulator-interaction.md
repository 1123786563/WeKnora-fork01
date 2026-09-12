# Android 模拟器交互验收（2026-09-12）

环境：Android 模拟器 test36-small（emulator-5554，720x1440），原生 dev client com.weknora.mobile（Gradle assembleDebug 重建，含 react-native-netinfo 原生模块），后端 make dev-app 于 :8080，应用 API 地址 10.0.2.2:8080（模拟器宿主回环标准映射），Metro :8087。

已验证（全部有截图证据 screenshots/android-sim-*.png）：
1. 重建 APK（Gradle assembleDebug，修复缺 netinfo 原生模块导致的启动崩溃）+ adb 安装成功 ✅
2. Expo dev client 启动器可用（Development Servers 列表 + New development server 输入 10.0.2.2:8087 + Connect）✅
3. JS bundle 从 Metro 加载，登录界面渲染（WeKnora 标题/Email/Password/Sign in/SSO/注册/邀请/Change server）✅
4. 真实凭据登录成功（parity-test@local.dev / Parity12345678，后端返回 200）✅
5. 登录后跳转 knowledge/index，KB 列表渲染真实后端数据（parity-faq-kb / Parity KB Demo / 产品知识库）✅ 见 android-sim-08-loggedin.png
6. 应用正确渲染后端 401 错误映射（Invalid email or password，见 android-sim-06/07）✅

环境限制/过程记录（如实）：
- adb input text 对特殊字符（!）注入不稳定；验收期间临时将测试账号密码切换为纯字母数字 Parity12345678，验收完成后已恢复为 Parity123456!（curl 验证恢复成功），web E2E 脚本不受影响。
- Gradle 构建需显式 ANDROID_HOME=$HOME/Library/Android/sdk。
