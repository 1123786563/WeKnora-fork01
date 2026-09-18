# CFT 文件锁（LOCKS）

唯一写入者登记。两个任务需要同一文件时：串行交接，或由集成负责人统一修改。

| File Lock Key | 覆盖文件 | 当前持有者 | 状态 |
|---|---|---|---|
| repo-baseline | BASELINE.md、task-status.json、STATUS.md、RESUME.md | CFT 总控 | 持有（台账单写原则） |
| root-scripts | scripts/test_craft_shared.sh、根 package.json 测试脚本段 | 总控（T001 已释放） | 空闲 |
| craft-contracts-dto | packages/contracts/src/craft/、internal/handler/session/craft.go DTO 段 | 待 T002 领取 | 空闲 |
| craft-api-client | packages/api-client/src/craft/ | 待 T002/T008 领取 | 空闲 |
| craft-tokens-theme | packages/views/src/craft/craft.css、packages/design-tokens、packages/ui/src/theme.css | 待 T003 领取 | 空闲 |
| dep-lockfiles | pnpm-lock.yaml、各 package.json | 待 T006 领取（assistant-ui 安装） | 空闲 |
| craft-views-workbench | packages/views/src/craft/（除 craft.css） | 待 T006/T007/T010 领取 | 空闲 |
| craft-routes | apps/web/src/features/craft/routes.tsx | 待 T008/T009 领取 | 空闲 |
| opencode-executor | internal/agent/opencode/、internal/container/craft_runtime.go | 待 T013 领取 | 空闲 |
| manifest-version-preview | internal/craft/version.go、preview.go、internal/application/service/craft_artifacts.go、craft_preview.go | 待 T019/T020 领取 | 空闲 |
| craft-session-svc | internal/application/service/craft_session.go | 待 T002/T016 领取 | 空闲 |
| release-gate-evidence | docs/craft/execution/、docs/craft/evidence/ 索引 | 总控 | 持有 |
| e2e-craft-stack | apps/web/e2e/craft-stack.sh、apps/web/e2e/craft-*.spec.ts、playwright.craft.config.ts | 总控（T001 修复已提交释放） | 空闲 |
