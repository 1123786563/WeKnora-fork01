# T40 / Issue #70 整计划最终审查包（t70 final-pkg）

> 本文件为最终审查任务的指定消费物，由最终修复批次补生成（2026-09-27）。审查所需的全部事实源、任务→提交映射、验证证据与残余声明汇总于此；每一条均可由 git 历史 / 文件现状 / 命令实跑复核。

## 一、定位

- 分支：`codex/issue30-t70`（worktree `.worktrees/issue30-sweep-t70`）；分支起点 `11a067674`（b5 plans docs 提交），任务提交区间 `7c02865bd..322c57d01` 共 8 个提交。
- 需求 Issue：`docs/plans/issue30-sweep/issues/issue-70.md`（What to build：「真实 Android Release 包完成核心流程，并验证系统返回、后台限制、通知、文件 URI、Keystore、麦克风和弱网络恢复」；三条验收标准原文见 plan Global Constraints）。
- 实施计划：`docs/plans/issue30-sweep/plans/plan-t70.md`（1307 行，7 任务）。
- 上游 Spec/ADR：`docs/specs/2026-09-20-mobile-ai-office-design.md`、`docs/specs/2026-09-20-mobile-module-seams.md`（§10/§12/§13）、`docs/adr/0005/0007/0010/0012`、`CONTEXT.md:339`（原始音频默认实时处理后删除）。

## 二、任务 → 提交映射（git show --stat 逐项核实，2026-09-27）

| Task | 提交 | 交付（文件 / 规模） |
|---|---|---|
| 1 通知权限 Adapter | `7c02865bd` | `apps/mobile/src/adapters/notification-permission.ts`（+53）+ 同名测试（+54）+ mimosa 裁决记录（+6）＝ 3 files / +113 |
| 2 组合根接线 | `777e63d60` | `composition.ts`（+14/−2，`registerActiveDeviceIfPossible` 第 4 可选参 + `'permission-denied'`）+ `app-smoke.test.tsx`（+42）＝ 2 files / +54/−2 |
| 3 mobile-core 清理端口 | `ed16812be` | `packages/mobile-core/src/voice/dictation.ts`（+8，`DictationPorts.audioCleanup?` + dropIntent 删除）+ `dictation.test.ts`（+87）＝ 2 files / +95 |
| 4 录音临时文件清理 Adapter | `ef23a8b22` | `dictation-capture.ts`（+39/−11，cancel 即删 + `createNativeAudioFileCleanupIfAvailable`）+ 其测试（+34）+ `composition.ts`（+18/−2）＝ 3 files / +80/−11 |
| 5 Android Release 工程配置 | `35a51d812` | `app.json` android 段 + `eas.json` + `package.json` 原生依赖（expo-audio/expo-network/expo-file-system）+ `.gitignore` android/ + `android-release-config.test.ts`（+55）+ prebuild 证据两文件 + `pnpm-lock.yaml` ＝ 8 files / +174/−1 |
| 6 AppState 测试补位 | `88ed6011c` | `apps/mobile/src/adapters/app-state.test.ts`（+78，两态映射 + no-op 降级）＝ 1 file / +78 |
| 7 平台一致性扫描 + 验收矩阵 | `6b09d1a3b` | `android-acceptance.ts`（+130，七工作流矩阵）+ `android-parity.test.ts`（+67）＝ 2 files / +197 |
| 7 修复轮 1 | `322c57d01` | `android-parity.test.ts`（+39/−1，平台 API 守卫补 ESM 行为符号扫描 + 守卫自证测试）＝ 1 file / +39/−1 |

任务级实施报告：Task 7 报告在 `docs/plans/issue30-sweep/plans/plan-t70.md-report.md`（含 RED/GREEN、修复轮全证据、worktree 重建环境事件、Concerns；本批次补入库）。Task 1-6 无独立会话级报告——其可复核证据即上表提交本身 + 计划级 gate 台账（`.superpowers/sdd/plan-t70/progress.md`），本审查包以提交映射代偿汇总，不伪造会话记录。

## 三、验证证据（本审查包生成同日实跑，worktree 根）

计划级验证命令（`plan-t70.md:1279` 整链逐字执行）：

```
pnpm exec tsx --test apps/mobile/src/adapters/notification-permission.test.ts \
  apps/mobile/src/adapters/dictation-capture.test.ts apps/mobile/src/adapters/app-state.test.ts \
  apps/mobile/src/android-release-config.test.ts apps/mobile/src/android-parity.test.ts \
  packages/mobile-core/src/voice/dictation.test.ts \
&& pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx \
&& pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck

# tests 41 / # pass 41 / # fail 0     ← 六文件定向（初版 40 + 修复轮 ESM 守卫自证 1）
# tests 67 / # pass 67 / # fail 0     ← app-smoke 全量
# tests 244 / # pass 233 / # fail 0 / # skipped 11   ← apps/mobile 全量（skip 为 opt-in 集成冒烟）
> tsc --noEmit                       ← typecheck 通过
PLAN_LEVEL_EXIT=0
```

（修复轮 1 全量同型：`pnpm --filter @weknora/mobile test` → 244/233/0/11，TEST_EXIT=0；见 Task 7 报告。）

## 四、七工作流验收矩阵（`android-acceptance.ts`，行号为当前 HEAD）

`system-back`(:34) / `background-limits`(:45) / `notifications`(:56) / `file-uri`(:71) / `keystore`(:86) / `microphone`(:102) / `weak-network`(:116)——与 Issue 正文枚举一一对应；每行含 `adapterSeam` + `localEvidence[]`（`android-parity.test.ts` existsSync 强制真实存在）+ `residual: { kind: 'blocked-env'; reason; unblock }`。守卫测试同时强制：react-native 平台 API（CJS require + ESM 行为符号导入双形态）只在 `src/adapters/`；screens/app 禁直连 `@weknora/api-client|contracts`。

## 五、blocked-env 残余（计划如实声明，本地不给替身记通过）

- **B1** Android Release 构建 + 真机安装运行：本环境无 Gradle/Android SDK/真机/release Keystore。本地等价证据：Task 5 prebuild 从零生成成功 + 生成物清单 + `eas.json` Release profile + 签名/构建 runbook（`docs/plans/issue30-sweep/android-evidence/android-release.md`）。
- **B2** 真实 FCM 推送送达：服务端 #67 本地形态 disabled provider、无 FCM 凭据。等价证据：盲推送/前台同步/设备注册集成冒烟 + Task 1/2 权限路径。
- **B3** 真机系统返回手势、Doze、硬件 Keystore、麦克风真实拒权：需 Android 真机。等价证据：`app-state.test.ts` / `dictation-capture.test.ts` / `offline-vault-integration-smoke.test.ts`。

## 六、审查时已知事项（不隐藏）

1. 分支 `codex/issue30-t70` 领先集成分支 `codex/issue30-mobile-office`（已合并 @ `2d811d6e4`）一个提交 `322c57d01`，需再合并一次。
2. `internal/types/vectorstore_test.go` 存在既有硬编码凭据（历史提交 `5cf093706` 引入，非本任务文件/改动），曾触发 Mimosa 强制拦截——留主控专门任务处置，本计划未触碰。
3. `android.blockedPermissions` 探测无效（计划差异记录第 5 条）：模板默认权限不因 blockedPermissions 剔除，本计划只做显式声明，剔除留待真实证据。
4. Go/SQL 零改动；未触碰迁移、voiceRoomFor、miniprogram。

## 七、修复批次（本审查包所属）

最终修复批次报告：`.superpowers/sdd/t70/final-fix-report.md`（3 项 minor 发现全处置：本审查包补生成、Task 7 报告补入库、notification-permission.ts:30 注释措辞对齐行为）。
