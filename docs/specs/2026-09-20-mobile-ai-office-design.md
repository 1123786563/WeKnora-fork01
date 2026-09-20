# WeKnora 移动 AI Office 正式规格

状态：Approved Specification
日期：2026-09-20

## Problem Statement

团队与企业成员目前需要在知识问答、研究、办公文档、外部业务操作和软件开发之间切换不同入口。现有 WeKnora 已拥有 Tenant、知识、Agent、Connector、持久 Run、审批、预算与审计能力，但缺少一个能在手机上统一发起目标、监督云端执行、处理审批并取得成果的产品入口。

现有 Web 更适合治理和配置，不能替代移动工作入口。Happy 与 Paseo 证明了移动监督 Agent、时间线、断线恢复、Diff、文件和语音的价值，但二者围绕个人设备或本地 Daemon 建模，不能直接承载 WeKnora 的企业 Tenant、权限、预算、知识与云端执行语义。

用户还面临一致性和信任问题：网络断开后不知道请求是否成功；Run 状态与 Task 是否完成容易混淆；连接器或代码写入可能被重复执行；共享资源可能意外扩大 Task 可见性；Agent 更新、Marketplace 引入和依赖升级可能让运行行为静默漂移；移动缓存、推送和多 Deployment 可能泄露跨 Tenant 内容。

产品需要在不建立第二套身份、Task、审批、钱包或执行权威的前提下，把这些复杂性隐藏在少量深 Module 后，并让所有可观察行为都能通过稳定 Interface 验证。

## Solution

建设面向团队与企业成员的 iOS 与 Android 原生移动 AI Office。成员从统一入口提交目标，由一个固定版本的 Lead Agent 对结果负责，并在 Task Grant 范围内使用知识、工具、专业 Agent、Connector 和开发环境。

WeKnora 后端继续作为 Deployment、Tenant、Task、Run、权限、预算、凭据、审批、产物与审计的唯一业务权威。首版所有 Agent 和工具执行均发生在 WeKnora 管理的隔离云环境中。移动端负责提交意图、展示权威状态、保存受控离线副本、接收行动通知和提供原生交互，不承担执行调度或授权决策。

首版以 Task 为统一工作单元，并闭环五类工作：有证据的知识问答、多来源研究、版本化办公产物、受审批的外部操作、Developer 代码交付。Task 是现有 Session 的产品名称，一个初始目标创建一个 Task，同一目标的追问继续原 Task，Run 表示其中一次执行。

移动端采用首页、任务、新建、资源、我的五个一级入口。Task 页面结果和行动优先，时间线是可恢复事实流；文件、Diff、终端、证据和产物按能力展开。完整治理继续留在 Web，包括 Agent、知识、连接、成员、策略、预算和 Marketplace 管理。

移动业务行为由 Mobile Runtime、Task Office、Resource Shelf、Task Material、Voice Room、Scoped Vault 六个深 Module 提供。Screen 是 presentation Adapter；REST/SSE、SQLite、SecureStore、推送、WebRTC 和系统分享位于 seam 上。

## User Stories

1. As a team member, I want to connect the app to an official or self-hosted WeKnora Deployment, so that I can work within my organization’s approved environment.
2. As a team member, I want to authenticate through the Deployment’s login or OIDC flow, so that the app does not create a second identity system.
3. As a member of multiple Tenants, I want one Active Tenant at a time, so that tasks, search, requests and caches cannot mix across spaces.
4. As a member of multiple Tenants, I want to see only minimal pending counts for inactive Tenants, so that I know where attention is needed without leaking content.
5. As a returning user, I want the app to restore my Deployment, identity and Active Tenant in a verified order, so that stale cached content is never shown as current.
6. As a user on an incompatible client or server version, I want a clear upgrade explanation and safe read-only surface, so that unsupported commands are not attempted.
7. As a member, I want a Home view of items needing me, active work and recent results, so that I can orient quickly.
8. As a member, I want a unified task list with search, filters and archive state, so that all work types use the same mental model.
9. As a member, I want one universal New entry, so that I can describe a goal without classifying it first.
10. As an advanced member, I want optional model, reasoning and budget controls, so that I can override safe defaults when authorized.
11. As a member, I want the system to recommend a Lead Agent and relevant resources, so that common tasks require little setup.
12. As a member, I want every initial goal to create a Task, so that even quick questions have a durable identity and history.
13. As a member, I want follow-up instructions for the same goal to remain in the Task, so that context is preserved without creating duplicate work.
14. As a member, I want to fork or explicitly create a new Task when the goal changes, so that unrelated work does not share identity.
15. As a Task Owner, I want a Task to contain multiple Runs, so that retries and follow-up execution retain the same goal and history.
16. As a Task Owner, I want Task lifecycle, Run status and Attention status shown separately, so that execution state is not mistaken for goal completion.
17. As a member, I want the Task page to show current result and required action before raw activity, so that I can act without reading logs.
18. As a member, I want a durable Task Timeline, so that inputs, conclusions, tools, approvals, Runs, artifacts and external receipts are recoverable.
19. As a member, I want raw tool and terminal output collapsed by default, so that the mobile page remains understandable.
20. As a member, I want only factual stages and progress counts, so that decorative progress is not presented as execution truth.
21. As a Task Owner, I want to steer the current Run at a safe point, queue an instruction for the next Run, or stop and restart, so that each intervention has explicit semantics.
22. As a Task Owner, I want the app to show which Run accepted an intervention, so that I know whether it affected current or future work.
23. As a Task Owner, I want a stop request distinguished from confirmed process termination, so that unknown effects are not hidden.
24. As a member, I want quick knowledge questions to complete and auto-file into recent history, so that the task list is useful without losing traceability.
25. As a knowledge worker, I want the Lead Agent to search only knowledge I may access and the Tenant permits it to discover, so that convenience does not bypass governance.
26. As a knowledge worker, I want key conclusions linked to exact source versions and retrieval times, so that I can inspect the evidence.
27. As a knowledge worker, I want fact, rule inference and model inference distinguished, so that generated reasoning is not misrepresented as source truth.
28. As a researcher, I want the Lead Agent to delegate independent read-only research using least privilege, so that work can proceed in parallel safely.
29. As a researcher, I want one writing Run to assemble parallel findings into a versioned report, so that concurrent research does not create conflicting drafts.
30. As a member, I want to annotate an Artifact or request changes to a fixed version, so that approved versions are never silently overwritten.
31. As a member, I want to preview supported documents, spreadsheets, slides, PDFs, images and code, so that I can review results on mobile.
32. As a member, I want unsupported or large artifacts to offer authorized download and system share, so that the app does not pretend it can render everything.
33. As a member, I want files, Diff, test reports and terminal output to be read-only on mobile, so that manual edits cannot bypass Task history and single-writer rules.
34. As a member, I want a generated Artifact published to Feishu, Notion or Confluence only after reviewing the exact version and destination, so that external writes are intentional.
35. As a collaborator in an external office suite, I want the external document to become the collaboration authority after publication, so that WeKnora does not overwrite later human edits blindly.
36. As a Task Owner, I want subsequent publication to read the external current version first, so that version conflicts are visible.
37. As a Task Owner, I want related external actions grouped into a readable Action Plan, so that I can approve the intended operation as a whole or exclude individual items.
38. As a Task Owner, I want changed targets, content, connections or operation sets to invalidate prior approval, so that approval cannot drift.
39. As a Task Owner, I want every external action to retain an independent result, so that partial success can be reconciled without repeating successful actions.
40. As a Task Owner, I want unknown external outcomes checked against remote facts, so that timeouts do not cause blind retries.
41. As a developer, I want to select an authorized GitHub or GitLab repository and fixed baseline, so that code execution starts from a reproducible state.
42. As a developer, I want the Agent to modify and test code in a Task-owned cloud Workspace, so that work does not rely on my personal computer.
43. As a developer, I want to review Diff, tests and a candidate commit before delivery, so that remote writes bind to determined content.
44. As a developer, I want delivery limited to a task branch and draft PR or MR, so that protected branches and merges remain under code-platform governance.
45. As a developer, I want personal and Tenant code-platform connections kept distinct, so that personal credentials never become shared credentials.
46. As an auditor, I want delivery records to identify initiator, approver and actual remote identity, so that attribution remains clear.
47. As a developer, I want push-success and PR-creation failure represented as partial completion, so that recovery does not repeat the push.
48. As a security administrator, I want general Shell execution unable to access remote write credentials, so that delivery approval cannot be bypassed.
49. As a Task Owner, I want my Task private by default, so that shared resources do not automatically expose my work.
50. As a Task Owner, I want to grant Viewer or Collaborator access explicitly, so that reading and participation are separate permissions.
51. As a Viewer, I want to inspect permitted results without running or changing the Task, so that read-only sharing is real.
52. As a Collaborator, I want to comment, add instructions and request Runs without becoming Owner, so that teamwork does not transfer control.
53. As a Task Owner, I want only myself or an authorized actor to approve use of my personal connections and code delivery, so that sharing cannot delegate my identity.
54. As a compliance administrator, I want metadata available by default and content access gated by a reasoned, time-limited, audited process, so that privacy and compliance coexist.
55. As a Tenant administrator, I want Task retention and legal hold policies, so that users cannot bypass organizational obligations.
56. As a Task Owner, I want internal deletion not to delete external documents or code implicitly, so that destructive side effects require their own approval.
57. As a Task Owner, I want every Task to have a cumulative budget including delegated work, so that subagents and retries cannot multiply costs invisibly.
58. As a Task Owner, I want estimated, used, reserved and remaining cost shown distinctly, so that I understand budget state.
59. As a billing administrator, I want only authorized actors to raise Task Budget, so that Collaborators cannot expand spend.
60. As a member, I want budget exhaustion to pause durably without deleting the Workspace, so that authorized continuation is possible.
61. As a member, I want the app to preserve drafts while offline, so that interrupted mobile work is not lost.
62. As a member, I want offline drafts submitted only after my confirmation, so that reconnect does not replay stale intent.
63. As a member, I want cached task content encrypted and scoped to Deployment, user and Tenant, so that another account or space cannot read it.
64. As an administrator, I want device revocation to remove or cryptographically invalidate cached content, so that lost devices can be contained.
65. As a member, I want push notifications only for required action, failure or unknown outcome, completion and important budget events, so that alerts remain useful.
66. As a privacy-conscious member, I want push bodies to omit sensitive business content, so that lock-screen notifications do not leak data.
67. As a self-hosted administrator, I want to use a blind notification gateway, enterprise-signed app or no push, so that deployment policy controls metadata exposure.
68. As a member, I want voice dictation to produce an editable draft, so that transcription errors can be corrected before submission.
69. As a member, I want real-time voice conversation attached to a Task, so that hands-free interaction retains Task semantics.
70. As a security-conscious member, I want high-risk approval to require a readable confirmation screen, so that speech alone cannot authorize side effects.
71. As a privacy administrator, I want raw voice deleted after processing unless explicit retention is enabled and disclosed, so that biometric data is minimized.
72. As a member, I want the Resources view to show only Available Agents, knowledge and connections that my current Tenant permits me to use, so that discovery matches actual capability.
73. As a member, I want Agent unavailability explained as unavailable or forbidden with a reason, so that missing capability is not guessed as success.
74. As a Tenant administrator, I want Marketplace Agent Adoption, local capability mapping, testing and version publication performed on Web, so that mobile use never bypasses governance.
75. As a Tenant administrator, I want multiple local Agent Variants from one Adoption, so that departments can bind different knowledge and policies.
76. As a Tenant administrator, I want Agent upgrades to be explicit proposals, so that a Marketplace Release cannot change existing Agent Versions or Tasks silently.
77. As a security administrator, I want revoked Agent Releases and dependencies to block new Tasks and Runs while preserving history, so that risk containment does not destroy evidence.
78. As a Publisher, I want Marketplace Metrics to be aggregated and de-identified, so that I can improve a Release without seeing adopting Tenant content.
79. As a user of assistive technology, I want system themes, dynamic type, screen-reader labels, reduced motion and reachable controls, so that the core workflow remains usable.
80. As a release operator, I want iOS and Android behavior validated on real devices, so that browser prototypes and bundle exports are not mistaken for native acceptance.

## Implementation Decisions

- WeKnora remains the only business authority. The mobile client does not create parallel identities, Tasks, approvals, balances, credentials or execution state.
- The first release uses platform-managed cloud execution only. Personal devices, local Daemons and enterprise private workers are not execution targets.
- Task is the product name for the existing Session identity. A Task contains multiple Runs; initial goals create Tasks and same-goal follow-ups stay within them.
- Task lifecycle, Run status and Attention status are separate. Agent-specific progress phases never replace these canonical dimensions.
- One Task permits at most one write Run. Independent read-only delegation may run concurrently and is combined by the single writer.
- Lead Agent Version, Artifact versions, Action Plans and candidate code commits are immutable approval anchors. Changes invalidate prior approvals.
- The mobile information architecture is Home, Tasks, New, Resources and Me. Task pages are result-and-action first, with Timeline and contextual material behind them.
- Mobile Runtime owns Deployment, identity, Active Tenant, compatibility, device registration and Scope Lease.
- Task Office owns Home/Task projections, durable submission identity, reconciliation, Snapshot/SSE recovery, intervention, decisions, budget and Task lifecycle.
- Resource Shelf owns Available Agent, knowledge, connection and attachment selection. Marketplace governance remains on Web.
- Task Material owns evidence, Artifact versions, Files, Diff, tests, read-only Terminal, download, share and annotation.
- Voice Room owns real-time audio and transcription. Confirmed text enters Task Office; voice never grants approval.
- Scoped Vault owns encrypted scope storage, drafts, submission journal, event projection, retention and revocation.
- The App Shell is a composition root and presentation Adapter. Screens do not call wire clients directly or maintain request IDs, cursors, revisions or scope generations.
- Shared contracts contain wire shapes and parsers only. Shared domain code contains pure value objects, policies and projections. A dedicated mobile core owns deep Module Interfaces, implementations and Ports.
- Mobile core does not depend on React Native, DOM or concrete transport. Remote and native details are injected as Adapters.
- Existing DOM-oriented UI and view packages are not imported into the React Native app. Design tokens and internationalized copy may be reused through native mappings.
- REST submits commands and loads authoritative Snapshots. Cursored SSE carries durable Task/Run events. WebSocket or WebRTC is reserved for real-time voice. Push is a synchronization hint.
- Each command that can have an unknown outcome uses a durable idempotency identity. Network failure triggers lookup or reconciliation, not silent replay.
- Cache identity includes Deployment, user, Tenant, Task and Run where applicable. Scope changes invalidate subscriptions and reject late responses.
- Offline mode permits approved reads, drafts and annotations. It prohibits Run commands, approval, budget expansion and external Actions.
- Registered devices are separate per Deployment. Device identity assists push and key wrapping but never replaces account or Tenant authorization.
- The server may process authorized plaintext because cloud execution, retrieval and Connector calls require it. Transport, persistence, backups and mobile cache are encrypted, and credentials use secret management.
- Task Grant is subordinate to member permission and Tenant policy. Delegated Agents receive only a minimum subset. Agent declarations never create permission.
- Viewer, Collaborator and Owner are distinct Task roles. Compliance content access is an audited process, not ordinary administrator membership.
- Internal Artifact versions coexist with external office documents. After publication, the external document is the collaboration authority; later updates read its current version first.
- The first office write Adapters are Feishu, Notion and Confluence. Existing read/sync capability does not imply write permission.
- Developer supports personal and Tenant GitHub/GitLab connections, task branches and draft PR/MR only. It does not merge automatically or expose remote credentials to Shell.
- Agent Catalog combines Built-in, Public Marketplace and Tenant Catalog Listings. Mobile displays only locally approved Available Agents.
- Marketplace distributes immutable, sanitized Agent Releases with Manifest and Dependency Lock. Tenant Adoption creates local Variants that require mapping, testing and local Agent Version publication.
- Marketplace upgrade, retirement, unlisting, deprecation, security revocation and dependency security block are distinct states. None silently rewrite existing Task history.
- The first Marketplace release is free distribution only. Publisher payment, revenue share and settlement are not part of this specification.
- Official cloud and self-hosted Deployments use capability negotiation. Missing security-critical capabilities produce an explanation or limited read-only mode, not optimistic calls.
- Existing Session history is projected as legacy Tasks without fabricating historical Grants, budgets, approvals or Agent Versions.
- The Taro mini-program remains a contract and migration reference until the native app passes agreed capability, security and real-device gates.
- The high-fidelity prototype confirms navigation, information hierarchy, double theme, task submission interaction, approvals, unknown-request reconciliation, scope-isolated drafts, editable voice transcript and artifact download affordances.
- Prototype HTML, screenshots and simulated state machines do not prove native runtime, backend integration, persistence, push, audio, provider, payment or deployment behavior.

## Testing Decisions

- Tests target observable behavior at the highest stable Interface. Screen tests verify rendering and navigation; they do not duplicate Module internals.
- Mobile Runtime Interface tests cover login restoration, capability negotiation, Active Tenant switching, Scope Lease revocation, device revocation and late-response rejection.
- Task Office Interface tests cover durable request identity, lost acknowledgements, unknown reconciliation, Snapshot hydration, SSE gaps, cursor expiry, single-writer admission, intervention routing, decision CAS and three-dimensional state projection.
- Resource Shelf Interface tests cover Available Agent filtering, knowledge authorization, connection capability, attachment preparation, revoked resources and explicit unavailable/forbidden reasons.
- Task Material Interface tests cover immutable versions, citation provenance, grant expiry, supported/unsupported previews, Diff, terminal read-only behavior, download and system share.
- Voice Room Interface tests cover permission denial, disconnect, editable transcription, confirmation into Task, raw-audio deletion and refusal to authorize high-risk Actions by voice.
- Scoped Vault Interface tests cover encryption adapter failures, Deployment/user/Tenant isolation, key rotation, revocation, retention, offline drafts and rejection of offline side effects.
- Remote-owned dependencies use in-memory scenario Adapters for Module tests and real HTTP/SSE contract tests for production Adapters.
- True external dependencies such as APNs, FCM, WebRTC, system audio and system share use mock or scripted Adapters at the Port and real-device acceptance separately.
- Wire contract tests start from real serialized bytes or representative fixtures and exercise parser-to-domain projection. Tests do not assert private helper structure.
- Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.
- End-to-end tests cover the five vertical workflows rather than isolated frontend/backend layers.
- Security tests cover cross-Tenant and cross-Deployment leakage, forged cursors, revoked connections, stale approvals, shell credential isolation, dependency revocation and compliance access auditing.
- Migration tests verify old Sessions remain readable and only gain new capabilities through explicit upgrade or new Run admission.
- Accessibility acceptance covers 320/390-width layouts, system dark/light themes, dynamic text including large sizes, VoiceOver/TalkBack, reduced motion, focus order and minimum touch targets.
- Native release evidence must come from installed iOS and Android builds with real login, background/foreground recovery, weak network, notifications, downloads, voice permissions and secure storage.
- Prototype checks remain useful as design regression tests only. Their simulated data and browser state are never counted as backend, database, native or external-provider acceptance.
- Existing tests around mobile submission, execution cache, compatibility, Workbench handlers and Taro transport are prior art; as deep Module Interface tests replace them, redundant shallow white-box tests should be removed.

## Out of Scope

- Agent execution on personal computers, local Daemons or enterprise private workers.
- A new desktop client, Expo Web client or replacement of the existing Web administration experience.
- Full Office document, spreadsheet or presentation editors inside the mobile app.
- Direct mobile file/source editing or interactive PTY.
- Automatic merge, protected-branch writes or unattended code delivery.
- Bidirectional real-time synchronization between WeKnora Artifacts and external office documents.
- Offline Run execution, offline approval, offline budget changes or automatic command replay.
- Public sharing of complete Tasks or cross-Tenant Task collaboration.
- Claims that the WeKnora server cannot read authorized Task content.
- Hard requirement for confidential-computing hardware in the first release.
- Paid Marketplace Agents, trials, subscriptions, refunds, Publisher revenue share, settlement or advertising.
- Consumer star ratings, free-form Marketplace reviews and arbitrary private Release links.
- Automatic Agent/Skill dependency upgrades or three-way merging of locally modified portable behavior.
- Marketplace authoring, review, Adoption or Local Capability Mapping in the mobile app.
- Guaranteed support for every existing Session as a fully upgraded Task.
- A commitment to a sandbox vendor, fixed budget values or production performance SLOs before validation.

## Further Notes

- The current checkout does not contain an active native apps/mobile tree. A historical Flutter/Conduit tree existed and was removed; it is evidence of coupling risk, not the implementation base.
- Current reusable seams include shared mobile wire contracts, mobile request adapters, pure mobile domain policies, the Taro native Adapter and Go Workbench read/write paths.
- Current implementation still has known gaps: Workbench overview is Owner-oriented, the canonical Task/Run/Attention projection is incomplete, mobile Available Agent projection does not yet reflect the new Marketplace model, and native encrypted storage/voice Adapters do not exist in the current checkout.
- The prototype covers 18 navigable pages, dark/light presentation, abnormal-state demonstrations and design-level interaction tests. It explicitly does not verify real authentication, Go/database wiring, push, microphone, system sharing, external providers, billing or app-store deployment.
- Implementation must begin with the capability validation described by this specification and supporting designs. A failed validation that conflicts with an ADR or domain invariant returns to design; implementation must not silently redefine the product.
- Supporting design documents remain normative for detail: the Agent Marketplace domain model and the mobile deep Module/seam design. CONTEXT.md remains the canonical glossary, and ADRs remain authoritative for irreversible architecture decisions.
