# T15 OCR 报告（第 2 次运行）

范围说明：ask 给定线性范围 05fb4d496^..d87732116 为 T15 的 4 个连续提交。前 3 个第 1 轮已审（4 条 findings：探针事实读-改-写无并发防护 [high]、晋升栅栏仅比 revision 未绑定身份 [high]、选择器 error 静默降级最新版本 [medium]、web-promotion 未注册 exports [low]），已由 d87732116 修复（行锁两处并发写手 5 连跑零丢失、栅栏三腿身份绑定、选择器保守降级席位留空+Warnf、exports/tsconfig/desktop vite 别名补齐）。本轮审查第 1 轮之后的增量段 --from 0b51eb71c --to d87732116（含全部修复 diff，11 文件中 6 个非测试源码被审）。

## 本轮：--from 0b51eb71c --to d87732116

Review complete: 1 finding(s) across 6 selected item(s).

─── apps/web/tsconfig.json:53-55 ───
[maintainability · medium] 本次为 `@weknora/domain/craft/web-promotion` 同步了 TS 路径映射（apps/web/tsconfig.json）与桌面端 Vite 别名（apps/desktop/vite.config.ts），但 `apps/web/vite.config.ts` 的 `resolve.alias` 未同步添加对应条目。由于该配置中存在裸前缀别名 `@weknora/domain`（指向 query-key.ts），而 Vite 对象别名按前缀匹配（apps/desktop/vite.config.ts:16-18 的注释正是描述此陷阱），一旦 web 侧代码 import `@weknora/domain/craft/web-promotion`，rollup 会将其重写为 `<query-key.ts>/craft/web-promotion` 并以 ENOTDIR 构建失败。建议在 apps/web/vite.config.ts 中与其他 craft 别名（capabilities/state/reconnect，第 55/77/78 行）一并补上该条目，保持四处配置（domain exports、web tsconfig、web vite、desktop vite）同步。

第 1 轮 4 条 findings 的修复经重审确认无回归，新暴露 1 条四处配置漏同步 web vite 别名的边界问题。
