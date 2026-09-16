# A4 — FAQ Excel (.xlsx/.xls) 导入对齐（2026-09-14）

分支：codex/react-multiclient（worktree .worktrees/react-multiclient）。实施 agent：FAQ A4 Excel 导入切片。未 commit，交由协调者复核集成。

## 裁决与依赖来源

- 协调者裁决 A4：采用 Vue 同源 vendored xlsx 依赖。
- Vue 权威声明：frontend/package.json:45 `"xlsx": "file:./packages/xlsx-0.20.2.tgz"`。
- 拷贝：frontend/packages/xlsx-0.20.2.tgz → apps/web/vendor/xlsx-0.20.2.tgz，`cmp` 逐字节一致。
- sha256（两份相同）：`14e0f4cf262c222f61a426864d192b71733a2af4a2b5c2c42d1e45317f246f7c`
- apps/web/package.json dependencies 增加 `"xlsx": "file:./vendor/xlsx-0.20.2.tgz"`（仅 deps 部分）；`pnpm install --no-frozen-lockfile` 更新根 pnpm-lock.yaml（仅 xlsx 相关 3 处条目，无其他漂移）。安装版本解析为 0.20.2，与 Vue 相同工件。

## Vue 基准（frontend/src/views/knowledge/components/FAQEntryManager.vue）

- processFile 分支：:1905-1915（.json/.csv/.xlsx|.xls/否则 unsupportedFormat 警告）。
- parseExcelFile：:1999-2033 —— `XLSX.read(await file.arrayBuffer(), { type: 'array' })`；取 `SheetNames[0]` 第一个 sheet；`sheet_to_json(ws, { defval: '', raw: false })`；表头规范化（trim → 去除 ASCII 括号说明 `\([^)]*\)` → trim → 含中文保留、否则小写）；单元格值 `String(v || '').trim()`。
- 列映射（:2024-2030，与 parseCSVFile :1977-1983 同语义）：
  - 问题 || standard_question || question → standard_question
  - 机器人回答 || answers → splitByDelimiter（## 分隔，:2035-2051；无 ## 则整值为单项）
  - 相似问题 || similar_questions → 同上
  - 反例问题 || negative_questions → 同上
  - 是否停用 → parseBooleanField（:2054-2064，TRUE/1/是/YES→true，FALSE/0/否/NO→false，空→undefined，其他→默认 false），is_enabled = 取反（undefined 时不发）
  - tag_id → 非空时 Number()
  - 标签 || 分类 || tag_name → tag_name（React 侧丢弃，见"偏差"）
- normalizePayload：:2066-2074，宽松（不过滤空问题/空答案、不抛错），数组 filter(Boolean)。

## React 变更

1. apps/web/src/faq/import-export.ts：新增 `parseExcelFile(file)` + 私有 `splitByDelimiter`/`parseBooleanField`/`normalizeExcelPayload`，逐字对齐上述 Vue 行为（含注释行号）。注意：刻意不走 React 既有 `normalizeFAQPayload`（其会因空问题/空答案抛错整文件失败）；Excel 路径保持 Vue 的宽松语义，坏行由后端导入任务按行回报（Vue 同款流程）。undefined 的 tag_id/is_enabled 以条件展开省略，与 JSON 路径 wire format 一致。
2. apps/web/src/faq/FAQPage.tsx（仅 Excel 分支）：
   - handleImportFile（:787）：excel 分支由 unsupportedFormat 警告改为 `parseExcelFile(file)` 异步填充预览，catch 走既有 parseFailed 消息；JSON/CSV 分支未动。
   - confirmImport（:797）：excel 分支由 throw unsupportedFormat 改为 `parseExcelFile(importFile)`；JSON/CSV 仍 `parseFAQImportText(text, format)` 未动。
   - i18n key `knowledgeEditor.faqImport.unsupportedFormat` 在 FAQPage 中不再被引用（key 保留在 @weknora/i18n，未触碰）。
3. 新增 apps/web/src/faq/import-export.excel.test.ts（3 用例）。

## TDD 记录（前红后绿）

- 红：`npx tsx --test src/faq/import-export.excel.test.ts` → `SyntaxError: ...does not provide an export named 'parseExcelFile'`（1 fail）。
- 过程修正（均为测试侧）：① 测试夹具初版使用全角括号表头「问题（必填）」，Vue 正则仅剥 ASCII 括号——按 Vue 导出模板 FAQEntryManager.vue:2499 的真实表头（'标签(必填)'、'问题(必填)'、'相似问题(选填-多个用##分隔)'、'反例问题(选填-多个用##分隔)'、'机器人回答(必填-多个用##分隔)'、'是否全部回复(选填-默认FALSE)'、'是否停用(选填-默认FALSE)'、'是否禁止被推荐(选填-默认False 可被推荐)'）修正夹具；实现零改动即对齐，反向证明逐字一致。② helper 类型 `extraSheets: string[][][]`、`new Uint8Array(bytes)`（TS6 DOM BlobPart）。
- 绿：3/3 pass。

## 最终验证（全部绿）

- `cd apps/web && npx tsx --test src/faq/*.test.tsx src/faq/*.test.ts` → **tests 25 / pass 25 / fail 0**（基线 22 + 新增 3）。
- `pnpm run typecheck:web` → 通过（tsc -p tsconfig.json --noEmit）。
- `pnpm run build:web` → ✓ built in 4.09s（chunk >500kB 警告为既有现象）。

## 测试覆盖点（import-export.excel.test.ts）

1. Vue 模板中文表头（含 ASCII 括号说明）+ ## 分隔 + 是否停用=FALSE→is_enabled:true + tag_id=3；忽略第二个 sheet。
2. 英文表头小写化回退（QUESTION/ANSWERS/…）+ 空 是否停用 → 不发 is_enabled；反例列为空数组。
3. parseBooleanField 全分支：''→无 is_enabled、TRUE→false、是→false、未知值→默认 false→is_enabled:true（与 Vue 逐字一致）。

## 已知偏差（需协调者知悉）

- **tag_name（标签/分类 列）被解析但不发送**：React 客户端 `FAQEntryPayload`（packages/api-client/src/knowledge/faq.ts:16）无 tag_name 字段，而 Vue FAQEntryPayload :906-923 有且后端 internal/types/faq.go:336 导入结构亦含 `tag_name,omitempty`。若要对齐 Vue"导入即建标签"，需协调者侧在 @weknora/api-client 增加可选 `tag_name?: string` 并在 parseExcelFile 的 normalizeExcelPayload 中透传——该文件不在本切片可写范围，仅提交需求不改动。
- Vue 模板中的 是否全部回复/是否禁止被推荐 两列在 Vue parseExcelFile :2023-2031 中同样未读取，React 保持一致（不读取）。

## 修改文件清单（本切片独占）

- apps/web/package.json（仅 dependencies）
- apps/web/vendor/xlsx-0.20.2.tgz（新增，Vue 工件逐字节拷贝）
- pnpm-lock.yaml（仅 xlsx 条目）
- apps/web/src/faq/import-export.ts（新增 parseExcelFile 及私有 helper，既有导出未动）
- apps/web/src/faq/FAQPage.tsx（仅 Excel 分支与 import 行）
- apps/web/src/faq/import-export.excel.test.ts（新增）
- docs/migrations/react/evidence/vue-react-parity/2026-09-14-a4-excel-import.md（本文件）
