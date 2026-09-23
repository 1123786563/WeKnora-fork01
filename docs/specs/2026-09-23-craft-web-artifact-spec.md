# Craft 网页作品完整规格

- 日期：2026-09-23
- 状态：产品范围已批准；测试优先沿用现有 Craft API 用户旅程边界。
- 跟踪：[GitHub Issue #107](https://github.com/1123786563/WeKnora-fork01/issues/107)（`ready-for-agent`）
- 范围依据：[Craft 网页作品首版范围](2026-09-23-craft-web-artifact-scope-design.md)。本文细化同一范围，不扩大首版能力。

## Problem Statement

WeKnora 的企业成员可以检索知识并与 Agent 对话，但缺少一条把明确授权的企业资料转成可运行交互网页、随后持续修改和交付的完整路径。成员需要自行在对话、文件、开发环境和预览之间搬运结果，也难以判断生成网页使用了哪些资料、是否实际通过构建、关闭页面后能否继续，以及下载或分享时哪些受限数据会随作品流出。

## Solution

在现有 WeKnora Task 内提供 Craft 网页作品工作台。成员指定知识范围并上传材料，Lead Agent 理解目标和检索有权使用的知识，委派隔离沙箱内的 OpenCode 生成一个网页作品。工作台展示对话、来源、运行进度、文件、预览和不可变版本。新版本只有经过构建、预览可访问及页面加载检查后才成为默认预览；成员可继续修改、重新打开 Task、查看和下载旧版本与源码。权限、预算、停止、恢复和下载数据确认由服务端维持权威状态。

## User Stories

1. As an 企业成员, I want to create a Craft Task from a natural-language goal, so that I can begin a webpage without setting up a repository or terminal.
2. As an 企业成员, I want to select specific knowledge bases for a Craft Task, so that the Agent uses only the knowledge scope I chose.
3. As an 企业成员, I want to upload files alongside the goal, so that my own data can inform the webpage.
4. As an 企业成员, I want to upload a file regardless of its extension within the stated quotas, so that unfamiliar formats are not rejected solely by name.
5. As an 企业成员, I want to know when an uploaded format cannot be understood, so that I do not mistake its presence for successful analysis.
6. As an 企业成员, I want to choose whether to continue without an unrecognized file, so that one unreadable attachment does not silently distort the result.
7. As an 企业成员, I want uploaded code to remain input material even when I ask to run it, so that uploading does not become an uncontrolled execution path.
8. As an 企业成员, I want a bounded archive to be usable as input, so that its contents can inform the work without bypassing upload limits.
9. As an 企业成员, I want the Agent to state which materials it actually used, so that I can check the result against its sources.
10. As an 企业成员, I want facts in the webpage linked to evidence, so that I can distinguish source facts from model inference.
11. As an 企业成员, I want to watch the Lead Agent and delegated build progress, so that I know whether the Task is running, waiting, stopping or complete.
12. As an 企业成员, I want to preview a working interactive webpage, so that I can assess the result without installing its source.
13. As an 企业成员, I want the previous successful version to remain visible while a new Run builds, so that an unfinished change does not replace a usable result.
14. As an 企业成员, I want a failed build or inaccessible preview shown as incomplete, so that I do not trust a broken webpage.
15. As an 企业成员, I want to request a change in the same Task, so that the Agent can improve the same webpage using its persistent Workspace.
16. As an 企业成员, I want only one writing Run per Workspace, so that simultaneous instructions do not corrupt the work.
17. As an 企业成员, I want to stop an active Run and see whether it has actually stopped, so that I can make a safe next decision.
18. As an 企业成员, I want unfinished files retained as a draft after failure or stop, so that a later Run can repair them without replacing the last successful version.
19. As an 企业成员, I want to close and reopen the browser during a Run, so that work continues without a duplicate submission.
20. As an 企业成员, I want to reopen an older Craft Task, so that I can see its true Run state and continue editing its webpage.
21. As an 企业成员, I want to view and download any saved version, so that I can inspect a result from an earlier Run.
22. As an 企业成员, I want a saved version to remain immutable, so that subsequent changes do not alter a result I previously reviewed.
23. As an 企业成员, I want to download the webpage source and a source list, so that I can inspect or use the delivered result outside WeKnora.
24. As a Task Owner, I want to see derived data files and their origins before they enter a source download, so that I can explicitly decide whether to export restricted information.
25. As an 企业成员, I want restricted original knowledge links to remain authenticated after download, so that a source list does not grant access to the underlying documents.
26. As an 企业成员, I want an old version's evidence to refer to what was used at the time, so that a later knowledge update does not rewrite its history.
27. As an 企业成员, I want a new Run to retrieve current knowledge within its present grant, so that I can deliberately refresh a webpage using updated material.
28. As a Task Owner, I want a Craft Task private by default, so that its generated webpage and inputs are not exposed to other members automatically.
29. As a Task Owner, I want to add Task Collaborators and Task Viewers, so that I can invite editing or read-only participation explicitly.
30. As a Task Collaborator, I want to request a new Run, so that I can improve the shared webpage within my permissions.
31. As a Task Viewer, I want to inspect permitted results without being able to modify them, so that my access remains read-only.
32. As a Task Owner, I want to confirm sharing a webpage derived from restricted sources, so that sharing the result is a conscious choice.
33. As a Task Viewer, I want source links to respect my own knowledge permissions, so that shared results do not implicitly share private originals.
34. As a Task Owner, I want model and sandbox spending to respect the Task Budget, so that the Task cannot spend beyond its authorized limit.
35. As an 获授权账单管理员, I want a budget-exhausted Run to pause with a clear reason, so that I can decide whether to authorize more spending.
36. As an 企业成员, I want the webpage and its build to work from provisioned dependencies without outside network access, so that results are reproducible within the declared sandbox boundary.

## Implementation Decisions

- Reuse Session as Task identity and represent each new execution as a Run. One Craft Task has one primary Craft 网页作品, backed by one persistent Workspace. Internal OpenCode delegation remains within the Run and never becomes a second user-facing Task.
- Keep the Lead Agent responsible for the user goal, knowledge retrieval, authorization and final answer. OpenCode receives bounded materials and a delegated build objective in an isolated sandbox. Agent completion alone does not mark the Task or Artifact complete.
- Reuse the existing assistant-ui projection for conversation, messages, tool progress and member interactions. WeKnora remains authoritative for Task, Run, pending decisions, version, permission and budget state. Use the approved TDesign migration target for new or migrated workbench controls.
- Extend the Craft input acceptance boundary to accept any extension as an opaque read-only attachment within the existing 20 MiB per-file, 20-file and 100 MiB per-round quotas. Preserve content addressing, path normalization and input immutability. Content understanding is a distinct outcome; unrecognized content is disclosed before the Run proceeds or is cancelled by the member.
- Treat archives as bounded inputs. Extraction must enforce canonical paths, cumulative extracted size, file count, depth and resource limits. Uploaded code is never directly executed, including when the member requests it; the Agent may read it and create its own generated code in the writable Workspace.
- Pass only chosen and authorized knowledge or uploads to the Run. Record actual sources and immutable evidence references. The generated webpage and downloaded source include visible citations and a source list; inference is distinguishable from source facts. A source link resolves through the viewer's own authorization.
- Provision a fixed web template and dependency set for offline generation. Enforce no external egress for build and preview, including configurations whose sandbox default would otherwise allow egress. The preview remains isolated from the main product origin and cannot write to business systems.
- Promote a new immutable Task Artifact version only after successful build, accessible preview and actual page-load check. Keep validation evidence with the version. A failed, stopped or unknown Run can leave a Workspace draft but cannot replace the last successful default preview.
- Serialize writing Runs for a Workspace. Persist stop intent separately from confirmed stop, and recover by inspecting authoritative Run and sandbox state before deciding whether to retry. Browser reconnection reads the existing Task and Run instead of submitting another command.
- Existing versions remain viewable and downloadable. The first release only continues editing from the current Workspace; restoration and editing from an arbitrary historical version is deferred.
- Apply Task Owner, Collaborator and Viewer permissions to all Craft operations. Sharing the generated result requires an explicit owner decision when restricted knowledge contributed; it does not grant access to original sources.
- Source download presents a manifest of derived data files and their origins. Inclusion of restricted derived data requires an explicit Task Owner confirmation. Original restricted knowledge files are not bundled by default.
- Reuse the tenant policy and Task Budget for all model and sandbox activity. If budget is exhausted, pause the Run and surface the required owner or billing-admin action.

## Testing Decisions

- Test observable behavior, persisted outcomes and authorization at the highest existing Craft API user-journey seam. Tests should submit a Task/Run and inspect its observable state, Artifact versions, preview and download results; they should not assert component internals or mock away the permission boundary.
- Use a small browser end-to-end suite to verify the workbench projection: source selection, unknown-file warning, build status, old-version preview during a new Run, reconnection, sharing, stopping and download confirmation.
- Exercise the input boundary with known, unknown, script and archive files, quota edges and hostile archive paths. Assert accepted-but-unrecognized is distinct from parsed-and-used, and that uploaded code is not run.
- Exercise knowledge scoping and citation behavior: only selected sources may be used, old versions retain their original evidence, and source links deny access to a Viewer without original permission.
- Exercise version promotion with real build, preview reachability and page-load evidence; a generated file alone cannot become the default Artifact. Verify old versions remain immutable after a follow-up Run.
- Exercise Run interruption and recovery: stop requested versus confirmed stop, browser disconnect, unknown sandbox result, concurrent write attempts and budget exhaustion. Assert no duplicate work or false success.
- Verify build and preview network denial from the actual sandbox deployment configuration, including the Docker default that permits egress unless overridden. Verify the preview remains on its isolated origin.
- Verify source download manifests and owner decisions with restricted derived data; declined confirmation must not return that data.
- Existing prior art includes Craft input, knowledge, preview, version and recovery tests in the backend, and assistant-ui, stop, presentation and version tests in the React workbench. Extend these seams where possible; add a higher seam only where user-visible behavior cannot be proven by existing tests.

## Out of Scope

- Public or enterprise application hosting, domain binding and publication of the sandbox preview.
- Documents, spreadsheets, slide decks and other Craft Artifact types in the first release.
- Direct execution of uploaded scripts or binaries, outbound internet access, internal network access or arbitrary writes to external business systems.
- Simultaneous writing to one Workspace, editing from a chosen historical version and a full online IDE.
- Treating assistant-ui or the OpenCode session as an independent source of truth for WeKnora Task state.

## Further Notes

- The source repository already has a real assistant-ui runtime in the Craft view. Existing upload validation still uses a restricted extension list, while Docker's default bridge network permits egress. These are implementation gaps against this Spec, not evidence that the approved behavior already works.
- The existing Craft proposal remains background architecture. Where its suggested phases or feature breadth conflict with the approved web-artifact scope, this Spec and its approved scope source govern the first release.
- Acceptance should use the concrete scenario of a member generating a region-filterable sales webpage from a selected knowledge base and uploaded CSV, modifying it twice, verifying citations and version history, and reconnecting without duplicate execution.
