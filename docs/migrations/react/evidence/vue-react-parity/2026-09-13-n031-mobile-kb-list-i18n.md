# N031 移动知识库列表本地化证据

日期：2026-09-13  
范围：`apps/mobile/src/features/knowledge/KnowledgeBaseListScreen.tsx`、`list.ts`、`list.test.ts`、`apps/mobile/src/runtime.tsx` 与共享 `packages/i18n/src/index.ts`

## 实施内容

- 从 SecureStore 恢复并校验 `locale`，通过 runtime 暴露 `setLocale` 完成持久化入口。
- transport 使用当前 locale 发送 `accept-language`。
- 列表标题、scope、导航、加载/错误/空状态、创建表单、数量和无障碍标签均使用共享 i18n。
- 新增 favorites/recents、loadFailed 和 favorite/unfavorite 五语言 key，并保持 locale key 集一致。

## 验证

| 层级 | 结果 |
|---|---|
| Mobile 全量测试 | 96/96 passed |
| Mobile typecheck | passed |
| Shared i18n knowledgeList 测试 | 4/4 passed |
| `git diff --check` | passed |
| 独立只读复审 | PASS，无明确 FAIL |

## 尚未证明

该证据只覆盖列表本地化代码和自动化门禁，不代表 N031 整体 accepted。移动知识库详情、数据源、上传进度、图谱视觉/能力矩阵，以及浏览器/Vue 截图、真实后端、iOS 和 Android 原生运行证据仍缺失。
