# R205 Paired chat AX boundary (2026-09-15)

Fresh Chrome tabs were opened in the same browser session at:

- React: `http://localhost:5181/platform/creatChat`
- Vue: `http://localhost:5173/platform/creatChat`

Both tabs expose the welcome heading, composer entry area and quick-answer agent surface. React exposes `上传附件`, `知识库`, and a disabled `mock-stream-model 200K` chip; Vue exposes the disabled send action and `未配置` model text. React also received two session records while Vue returned no visible session rows.

This reproduces the earlier model-chip difference while preserving the same empty-chat layout. The source contract comparison in R198 shows identical model filtering and fallback rules, so the discrepancy is recorded as unequal API/local-storage state. No speculative UI change is made.

Acceptance boundary: accessibility and layout anchors are captured; same-condition model/session response, successful stream, and protected backend parity remain open.
