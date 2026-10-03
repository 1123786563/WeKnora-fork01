# T41 #71 可访问性矩阵补齐 — Android（AVD test36，模拟器口径，2026-10-03）

环境：AVD test36（android-36 google_apis arm64）emulator-5554，WeKnora 0.1.0（release apk），后端 :8084（HEAD cc5de43af）via https://192-168-3-33.nip.io:8443，Casdoor t01live（凭据仅 env 引用，未落盘）。

## 逐项 disposition（最高稳定 Interface；截图 + settings-proof + 客观判定）

| 项 | 文件 | 判定 |
|---|---|---|
| Leg1 320 宽 | leg1-d-login-320.png / leg1-b-home-320.png / leg1-a-workspace-320.png / leg1-c-taskdetail-320.png（+默认分辨率基线 leg1-a-home-default / leg1-a2-taskdetail-default） | evidenced：login/workspace/home/taskdetail 四屏在 `wm size 320x640` 下渲染，uiautomator dump 横向越界节点=0（xml 在案；任务长标题为正常 ellipsize 非布局破损）。观察：640 高度下列表区被 tab 栏压缩，任务行需滚动可达 |
| Leg2 动态字体 | leg2-a-taskdetail-font130.png / leg2-b-home-font130.png（settings put system font_scale 1.3，回执在 settings-proof-android.txt，已恢复 1.0） | evidenced：字号放大后屏面结构与操作项完整 |
| Leg3 TalkBack | leg3-i-talkback-on-home.png / leg3-j-explore-single-tap.png / leg3-k-double-tap-activated.png + leg3-talkback-speech-log.txt | evidenced：`enabled_accessibility_services` 回显 + dumpsys Bound services（TalkBack, FEEDBACK_SPOKEN/HAPTIC/AUDIBLE, touchExplorationEnabled=true）+ SpeechController 朗读活动日志 + 触摸探索语义实测（单击=聚焦不激活）。labels：home 屏 content-desc 8 项（调整当前运行/排队下一 RUN/停止运行/展开证据×2/任务材料/任务预算/语音房/重新同步快照，见 settings-proof）。元素级朗读序列日志不可得（TTS 成功路径无 logcat 输出）——模拟器口径限制，如实登记 |
| Leg4 主题 | leg4-a-taskdetail-night.png / leg4-b-home-night.png（cmd uimode night yes，已恢复 no） | evidenced：亮度判定 home night avg-luma=73 vs day=165；taskdetail night=73 vs day=203 → app 跟随系统夜间主题 |
| Leg5 减动效 | leg5-a-home-anim0.png / leg5-b-taskdetail-anim0.png（settings put global animator_duration_scale 0，已恢复 1） | evidenced（settings 口径）：设置回执 + 全屏截图；app 侧无动效消费点可观察差异（Task-1 审计：mobile 无 isReduceMotionEnabled 消费） |
| Leg6 回滚（pragmatic，待 Task 3 裁定） | leg6-a/b/c-*.png（settings-proof leg6 段） | evidenced（pragmatic 口径）：`adb uninstall` → `adb install` app-release.apk 0.1.0 → 首启 login 屏 → SSO 真实登录（Casdoor t01live，密码经 Chrome 表单输入，OIDC 回跳）→ workspace 列表与主面板完整恢复 → 任务面可入。迁移侧回滚覆盖引用：migrations/versioned/000191_task_grants.down.sql、migrations/versioned/000272_workbench_device_registrations.down.sql、migrations/sqlite/000191_workbench_device_registrations.down.sql。语义（app 降级 vs 迁移 down vs 旧包重装）待 Task 3 对照 AC 裁定 |

## 恢复态（settings-proof 末段）
wm-size 1080x2400 无 override / font_scale 1.0 / Night no / animator 1 / a11y services null。
