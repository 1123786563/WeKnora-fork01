# T09/#149 链接导入与不完整来源回退研究

研究日期：2026-09-24（Asia/Shanghai）
范围：只读检查 Issue #149、批准求职 Spec、ADR、T08 Opportunity 契约和现有 URL fetch seam。未修改生产代码、需求或远程 Issue。

## 已核实事实

- #149 的验收是：仅接入经过核验允许读取的来源；保留原链接、取得时间、完整性和失败原因；摘要不足以推断届别/学历等硬条件；用户补充 JD 后生成新的固定快照且原始来源可追溯。验证要求覆盖完整来源、登录阻断、摘要、不存在、超时，以及链接失败后粘贴 JD 成功。它显式依赖已集成的 #146/T08 Opportunity。
- 批准 Spec §4 要求来源上线前核实开放方式、使用条件和可取得内容；首批来源需公布。登录、阻止、摘要或不完整内容必须显示“需用户补充 JD”，保留原链接，不绕过登录，也不从摘要推断硬条件。§3/关键不变量要求外部原文是不可信输入，失败和未知不得展示为成功；§8 验收要求无法读取 URL 时请求粘贴 JD，且历史快照仍可打开。
- T08 当前公共证据写入入口是 `POST /api/v1/career/opportunities/import`，输入 `requestId/rawText/sourceLabel/sourceReference`，来源类型固定为 `manual_paste`；读取使用 `opportunityId + snapshotId`。T08 的 `OpportunityEvidence` 已保存精确原文、SHA-256、source、`acquiredAt`、status、逐字段 known/unknown；同一作用域 requestId 重放返回同一 ID，意图变化返回冲突。T09 应把 URL 取得结果转换为这一同样的 Opportunity observation/snapshot 语义，不能覆盖旧 snapshot。
- 现有 `internal/infrastructure/web_fetch` 提供 SSRF 安全的 HTTP/HTTPS 获取和文本抽取：URL scheme/host 校验、公共 DNS/IP、重定向约束、超时、body 限制、HTTP 状态和空内容错误分类；`NewFetcher` 可使用 Chromium fallback，`NewPipelineFetcher` 为 HTTP-only。错误码已有 invalid URL、DNS、timeout、403、429、5xx、SSRF/redirect、empty/unsupported 等。
- 现有 fetcher 会设置浏览器请求头并尝试浏览器渲染，但没有 Career 来源许可清单、robots/条款核验、登录墙/验证码的可靠业务分类、摘要完整性判定或 Career 快照持久化。聊天 pipeline 的 `PluginWebFetch` 仅把 web search 结果临时替换为文本，不是岗位来源权威，也不适合直接复用为 T09 的写入流程。
- 当前 Career 路由和 T08 handler 已有 `POST /career/opportunities/import`、`GET /career/opportunities/receipt`、`GET /career/opportunities/:opportunityId`。新增 URL 读取必须是 Career-owned adapter/服务接口，并通过同一作用域、不可变 snapshot 和 HTTP 合同；不要把 URL fetch 直接放入 Web 或通用聊天工具路径。

## 信任、法律和完整性边界

- 只允许显式配置且已完成开放方式/使用条件核验的来源 host/路径策略；未知来源应进入 `source_unknown`/`needs_review`，不得被标成已核验。不能绕过登录、验证码、付费墙、反爬或需要用户会话的限制，也不能把通用 `web_fetch` 的可访问性当作法律/许可批准。
- URL 本身是用户输入；必须先做 scheme/host/SSRF 校验，再按来源策略读取。重定向目标需要重新检查来源许可和 SSRF；不能允许 URL 或返回页面内容触发工具、权限、grant、二次 URL fetch 或 Agent 指令。
- 来源响应应保存原链接（用户提交和最终允许的 URL，如契约允许）、取得时间、来源适配器/状态、失败分类和完整性状态。HTTP 200 或可抽取文本不等于完整 JD；登录页、验证码页、仅摘要、空页、截断、超时和拒绝都应产生可解释的失败/不完整结果，并请求用户补充。
- 届别、学历等硬条件只有在完整且有证据的 JD 文本中才可作为已知字段；摘要或部分页面只能 unknown/needs_review。用户补充文本必须新建 observation/snapshot，保留原 URL observation 和失败原因，不能更新旧快照。

## 推断与建议（非已批准新需求）

- 最小接口建议为 `ImportURL`/`Career Source Adapter`，返回带状态的 Opportunity receipt，而不是直接返回字符串：`source_status`（例如 `fetched_complete`, `blocked_login`, `blocked_policy`, `summary_only`, `not_found`, `timeout`, `unknown`）、`completeness`（complete/incomplete/unknown）、`failure_code`（可枚举、无敏感原始错误）和固定 `opportunityId/observationId/snapshotId`。确切枚举应在契约测试前冻结。
- T09 不应尝试做通用爬虫或全国来源覆盖。先接入一个可公开读取、允许读取的固定测试来源适配器，并用 HTTP test server 契约覆盖完整、登录阻断、摘要、不存在和超时；生产来源清单/许可核验可以配置化，不能由客户端声明“已核验”。
- URL 成功取得的原文应调用同一个 Opportunity persistence seam，或抽出 T08 的通用 `StoreOpportunityObservation`；不要把 URL 内容伪装成 `manual_paste`。用户补充 JD 使用 T08 `manual_paste` 入口，产生新 snapshot 并保留 URL observation。
- 若 T08 目前 `OpportunityEvidence` 没有失败来源 observation 的字段，需先扩展公共契约并写契约测试，再实现 URL adapter；失败记录仍应可由 URL 导入结果或来源状态读模型访问。避免将失败记录塞进 profile `career_receipts`，也不要改变 profile revision。

## 最小纵向 Task 拆分建议

1. **T09-A：来源策略与 URL adapter（后端）**：冻结来源状态/完整性/失败码契约；接入一个允许公开读取的固定适配器；复用 SSRF-safe fetch seam；拒绝未知、登录、验证码/反爬和不完整响应；将完整结果或失败 observation 原子写入 Career Opportunity，历史 snapshot 只追加不覆盖。文件边界预计 `internal/modules/career/`、必要的 `internal/infrastructure/web_fetch/` 接口、Career migrations/contract tests。
2. **T09-B：Web 对话回退（前端）**：在现有聊天导入动作上只消费 typed receipt；显示来源状态、原链接、取得时间、完整性/失败原因；失败或不完整时提供粘贴 JD 入口；粘贴成功后导航到新的固定 snapshot，并能回看原始 URL 记录。文件边界预计 `apps/web/src/career/` 与最小 chat host hook，避免解析任意 assistant prose。
3. **T09-C：验收验证**：后端固定来源响应契约 + scope/privacy + no-tool/no-bypass；Web 浏览器场景覆盖成功、登录阻断、摘要、404、超时、失败后粘贴 JD、历史 URL snapshot 保留。只有在 T09-A 契约稳定后放行 T09-B。

## 主要风险

- 现有 fetcher 的 Chromium fallback 可能在产品语义上越过“允许读取”边界；Career adapter 必须显式禁用或受来源策略控制，而不能无条件复用 `NewFetcher`。
- 登录/验证码/摘要检测容易误报；状态应宁可 `needs_review`，并保留有限、稳定的失败码，不把模型猜测或 HTTP 200 当完整事实。
- T08 目前主要是成功 manual paste 的证据模型；若失败来源也必须可追溯，公共字段/迁移/前端读模型需要先共同冻结，否则 T09 Web 会依赖未定义的状态。
- 无法对任意真实招聘网站做稳定自动化验收；固定本地/受控响应契约只能证明分类和回退行为，真实来源清单及法律/条款核验仍是发布前运营门槛，不能在代码测试中宣称完成。

## 来源指针

- `docs/plans/issue-140/issues/issue-149.md`
- `docs/specs/2026-09-23-weknora-job-search-design.md` §§3–4, 8, 10
- `docs/adr/0015-job-search-as-weknora-specialist-agent.md`, `docs/adr/0017-immutable-job-and-application-evidence.md`, `docs/adr/0009-cloud-data-trust-boundary.md`
- `docs/plans/issue-140/task-8-architecture.md`
- `internal/modules/career/opportunity.go`, `internal/modules/career/handler.go`, `internal/router/routes_career.go`
- `internal/infrastructure/web_fetch/fetcher.go` and `fetcher_test.go`
- `internal/modules/conversation/chat_pipeline/web_fetch.go`
