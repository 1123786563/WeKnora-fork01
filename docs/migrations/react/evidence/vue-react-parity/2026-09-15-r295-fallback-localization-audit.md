# R295 用户可见 fallback 本地化审计（2026-09-15）

- 文档批量重解析在没有可处理项时改用 `common.operationFailed`，不再输出英文句子。
- 数据源连接器类型加载失败的非 Error 兜底改用现有 `dataSource.resourceLoadFailed`，服务端/异常消息仍保持优先。
- 数据源 focused tests：4/4；文档 page-chrome：14/14。
- `pnpm run typecheck:web` 与 `git diff --check` 通过。

该证据覆盖代码级 fallback 路径；真实 provider 错误响应、浏览器五语言截图和 native 验收仍待补齐。
