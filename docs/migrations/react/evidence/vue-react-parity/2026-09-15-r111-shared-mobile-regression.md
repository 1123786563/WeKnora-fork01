# r111 Shared 与 Mobile 回归门禁

- `pnpm test:shared`：467/467 通过。
- `pnpm --dir apps/mobile test`：190/190 通过。
- `pnpm --dir apps/mobile run typecheck`：通过。
- 覆盖本轮移动端 header、计数插值、Wiki/图谱禁用能力本地化改动，以及共享 i18n 五语言消息注册。
