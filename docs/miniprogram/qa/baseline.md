# 测试基线（2026-09-18）

## 环境与版本

| 项 | 值 |
| --- | --- |
| 基线提交 | `6a70c35a`（main，与 origin/main 同步） |
| Node | v26.7.0（CI 为 24；engines 要求 ≥22.16） |
| pnpm | 10.28.2（根 packageManager） |
| TypeScript（miniprogram） | 5.8.3（根共享门禁为 6.0.3） |
| Taro / React | 4.2.1 / 18.3.1 |
| 平台 | darwin 25.6.0 arm64 |

## 基线命令与首次结果（工作区，commit 6a70c35a）

| 命令 | 退出码 | 结果 |
| --- | --- | --- |
| `pnpm install --frozen-lockfile` | 0 | 1.6s（增量；node_modules 已存在） |
| `pnpm --filter @weknora/miniprogram run tokens:check` | 0 | 125 tokens 匹配；31 SCSS 引用有效 |
| `pnpm --filter @weknora/miniprogram run test` | 0 | 25/25 通过 |
| `pnpm --filter @weknora/miniprogram run typecheck` | 0 | 无错误 |
| `WEKNORA_API_ORIGIN=https://placeholder.invalid pnpm --filter @weknora/miniprogram run build:weapp` | 0 | 10.28s 编译成功；产物含 4 主包页 + 16 分包页（app.json 实测）；仅体积告警（common.js 324KiB） |
| `pnpm exec tsx --test packages/api-client/src/chat/mini-regression.test.ts` | 0 | 7/7 通过 |

## 干净隔离检出复验（同日）

`git clone <repo> /tmp/weknora-clean-verify`（提交 6a70c35a）后：

| 命令 | 退出码 | 结果 |
| --- | --- | --- |
| `pnpm install --frozen-lockfile` | 0 | 29.5s；resolved 2340 / reused 2245 |
| `apps/miniprogram/node_modules/@tarojs/shared` 链接 | — | 存在（既往 2026-09-17 出现过的链接缺失未复现，判定为一次性安装中断现象；本分支已在 CI 增加 frozen 安装门禁持续盯防） |
| tokens:check / test / typecheck / build:weapp | 0/0/0/0 | 全部通过 |

结论：依赖在干净环境中可重复安装（要求 1 满足），锁文件无需变更。

## 基线后的修复分支

`fix/miniprogram-qa-regression`（自 6a70c35a 创建）。分支内全部门禁的最终结果见
[report.md](./report.md)；缺陷与修复见 [bug-ledger.md](./bug-ledger.md)。
