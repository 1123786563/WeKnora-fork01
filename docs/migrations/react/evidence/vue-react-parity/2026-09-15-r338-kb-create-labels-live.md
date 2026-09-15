# 2026-09-15 R338 — knowledge-base create labels live recheck

- Authenticated React browser, 1440×900, zh-CN, real backend, opened the create knowledge-base dialog.
- Visible labels now resolve to Vue-derived localized values: `知识库名称`, `知识库类型`, `知识库描述`, `Embedding 嵌入模型`, and `LLM 大语言模型`.
- Placeholders resolve to `请输入知识库名称`, `请输入知识库描述（可选）`, `请选择 Embedding 模型`, and `请选择 LLM 模型（可选）`; no raw English field labels or model-id placeholders remain.
- This verifies the repaired form copy only; submit success/failure and the full guided flow remain separate acceptance items.

