# N005 Web 上传进度与高亮切片证据

日期：2026-09-13  
目标 worktree：`/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`

## 本轮范围

对照 Vue `frontend/src/components/upload-mask.vue` 与
`frontend/src/views/knowledge/KnowledgeBaseList.vue`，补齐 React Web 知识库列表的：

- 上传任务 start/progress/complete 事件接线与按知识库聚合；
- 成功/失败任务的 10 秒清理、失败保留和单次延迟刷新；
- 手动、URL、文件批次的完成事件语义（全失败不刷新）；
- `highlightKbId` 成功加载后的立即 URL 清理、滚动高亮与所有 scope 的分页跳转。

## 已执行证据

| 层级 | 命令/结果 |
|---|---|
| 专项单测 | `node --test --import tsx src/knowledge-bases/upload-progress.test.ts`：4/4 passed |
| Web 全量测试 | `npm run test`：301/301 passed |
| Web 构建 | `npm run build`：passed（仅既有大 chunk warning） |
| 工作区静态检查 | `git diff --check`：passed |
| 独立复审 | 第二次只读复审：此前 7 项问题全部 PASS，未修改文件 |

## 证据边界

本记录只证明代码、单测、Web 全量回归和构建门禁。当前没有新增以下证据，因此 N005 继续保持 `implementing`，不得升级为 `accepted`：

- 已认证账号下的 React 浏览器逐项操作与截图；
- 同数据、语言、主题、视口下的 Vue/React 截图差异报告；
- 真实后端上传、迟到响应、错误恢复和租户隔离 E2E；
- Wails 原生运行；
- iOS 与 Android 原生运行。

## 变更文件

- `apps/web/src/App.tsx`
- `apps/web/src/documents/KnowledgeDocumentsPage.tsx`
- `apps/web/src/knowledge-bases/upload-progress.ts`
- `apps/web/src/knowledge-bases/upload-progress.test.ts`
- `apps/web/src/styles.css`
