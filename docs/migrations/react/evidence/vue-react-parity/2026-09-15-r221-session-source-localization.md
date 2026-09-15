# R221 会话来源提示本地化（2026-09-15）

会话侧栏来源徽标此前使用写死的英文 `Session source` 作为 title。现改为共享聊天 copy 的 `sourceLabel`，在五种语言下随当前界面语言变化；徽标文本、筛选和会话逻辑不变。

验证：`pnpm test:shared` 469/469 通过。
