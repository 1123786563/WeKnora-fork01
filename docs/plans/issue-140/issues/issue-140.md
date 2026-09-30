# #140 Spec：WeKnora 求职专业 Agent（三端完整闭环）

- URL: https://github.com/1123786563/WeKnora-fork01/issues/140
- 状态: open
- labels: ready-for-agent
- 更新时间: 2026-09-23T16:34:15Z
- 读取来源: GitHub REST API, gh api --paginate, 2026-09-24 Asia/Shanghai

## 正文

## Problem Statement

中国大陆技术类应届生要在多个招聘来源之间反复寻找岗位、核对届别等硬性资格、针对每个岗位修改材料、亲自投递并追踪后续进展。现有工具常把搜索、简历生成和申请记录割裂：来源与岗位要求可能变化，AI 可能把推断写成履历事实，投递时使用的材料版本也难以追溯。用户需要在 Web、原生移动端和微信小程序上连续完成这一过程，同时掌握每个事实、文件和投递动作。

## Solution

在 WeKnora 中提供求职专业 Agent。每位求职者在单成员个人求职空间内建立已确认、可追溯的求职档案；通过链接、JD、移动端分享或自然语言指令取得岗位机会；先看三值岗位资格判断，再看有证据的岗位匹配评估；为选定岗位建立独立求职申请及 Task，编辑结构化申请材料，导出内容一致的 PDF 与可编辑 DOCX；由本人在招聘平台投递，并记录实际使用的版本、申请进展事件和下一步提醒。三个客户端连接同一服务端事实，均能完成完整闭环。首批主动聚合重点覆盖北京、上海、深圳、杭州，具体来源须逐个核实可用方式及条件。

## User Stories

1. As a Chinese mainland tech graduate, I want to use my WeKnora identity across Web, native mobile and WeChat mini-program, so that I can continue one job search without duplicate accounts.
2. As a job seeker, I want a private, single-member personal career space, so that my profile and applications are isolated from team workspaces.
3. As a job seeker, I want to upload an existing résumé or complete a guided intake, so that I can start from information I already have.
4. As a job seeker, I want to confirm each extracted education, project, internship, skill and certification fact with its source, so that AI does not treat guesses as my history.
5. As a job seeker, I want to edit my graduation date, degree, preferred cities and target roles, so that eligibility checks reflect my current circumstances.
6. As a job seeker, I want to search even when my profile is incomplete, so that missing fields do not block discovery.
7. As a job seeker, I want to paste a recruitment link, so that I can evaluate a job I found elsewhere.
8. As a job seeker, I want to paste a full JD, so that I can evaluate jobs whose pages are gated or incomplete.
9. As a mobile user, I want to share a job link or JD into the app or mini-program, so that I can capture an opportunity without manual re-entry.
10. As a job seeker, I want to describe my job target in natural language, so that the Agent can run a one-time search for me.
11. As a job seeker, I want to explicitly create, edit, pause and resume a continuing search rule, so that background search happens only under conditions I understand.
12. As a job seeker, I want to see search frequency and expected usage before enabling continuing search, so that I can control cost.
13. As a job seeker, I want to see each opportunity's original link, source and last checked time, so that I can verify that it is current.
14. As a job seeker, I want an incomplete or blocked source to ask me for the JD, so that missing requirements are not invented from a short summary.
15. As a job seeker, I want likely duplicate observations merged only when evidence is strong, so that different jobs or hiring batches remain distinct.
16. As a job seeker, I want expiry, takedown and changed-JD signals, so that I know when an opportunity needs another check.
17. As a job seeker, I want to see whether graduation cohort, degree, location and duration requirements are met, unmet or unknown, so that I can decide whether to spend time applying.
18. As a job seeker, I want the specific hard-condition conflict to remain visible when I continue anyway, so that a high skill score cannot hide ineligibility.
19. As a job seeker, I want to see matched skills, projects, transferable experience and missing evidence separately, so that I can judge a recommendation rather than trust a single score.
20. As a job seeker, I want model inference, source-page facts and my confirmed facts identified separately, so that I can verify each claim.
21. As a job seeker, I want to override a low-match recommendation without altering the original assessment, so that my decision and the Agent's assessment remain distinguishable.
22. As a job seeker, I want one application and one WeKnora Task for each chosen job and hiring batch, so that preparation, permissions, budget and progress stay together.
23. As a job seeker, I want my application to retain the JD snapshot used for its evaluation, so that later page edits cannot rewrite why I applied.
24. As a job seeker, I want to generate a role-specific résumé and application-answer drafts from confirmed facts, so that my materials are relevant and truthful.
25. As a job seeker, I want unsupported experience, numbers and credentials flagged before export, so that I do not submit fabricated claims.
26. As a job seeker, I want to edit one structured material body on any client, so that Web, mobile and mini-program show the same content.
27. As a job seeker, I want each confirmed edit to create a new material version, so that I can compare it with older versions and recover what I used.
28. As a job seeker, I want the same approved material version as both PDF and editable DOCX, so that I can submit or refine it as a recruitment platform requires.
29. As a job seeker, I want PDF text and layout and DOCX content and editability checked, so that a broken export is not marked ready to apply.
30. As a job seeker, I want optional cover-letter and interview-preparation drafts, so that I can prepare for later stages from the same evidence.
31. As a job seeker, I want to submit the actual application myself on the recruitment platform, so that I approve every external communication.
32. As a job seeker, I want to record the channel, time and material version I actually submitted, so that later interview preparation references the correct content.
33. As a job seeker, I want an unknown submitted version displayed as unknown, so that the system does not guess which résumé the employer saw.
34. As a job seeker, I want to record tests, interviews, offers, rejections, withdrawals and corrections as dated events, so that the full application history remains available.
35. As a job seeker, I want a current stage projected from confirmed events, so that I can filter applications without losing their history.
36. As a job seeker, I want actionable reminders in the product and optional native push or mini-program subscription messages, so that I can follow up on time.
37. As a privacy-conscious user, I want notification text to omit company, role and interview details, so that sensitive career information does not appear on my lock screen.
38. As a job seeker, I want usage estimates and quota checks before paid search or generation, so that I can choose whether to spend credits.
39. As a job seeker, I want to keep reading my existing profile and applications when I run out of quota, so that payment limits cannot trap my records.
40. As a job seeker, I want export and deletion controls with clear retention limits, so that I can leave the product while understanding what external platforms still hold.
41. As a job seeker, I want a failed or uncertain search, material generation or application update to be recoverable without duplicate records, so that retries do not corrupt my history.
42. As a user switching devices or spaces, I want stale requests and cached data rejected or cleared, so that I never see another space's career records.
43. As a reviewer of an application, I want to inspect the source and timestamp behind a recommendation, material claim or progress event, so that I can audit a decision.

## Implementation Decisions

- Use the existing WeKnora identity, single-member Tenant, Task, Run, authorization, budget, cloud execution, notification and Artifact authorities. The job-search product is a specialist Agent, not a second execution or account system.
- Give career domain facts one owner, Career Office. It owns confirmed profile facts, source observations and immutable job snapshots, three-state eligibility, evidence-backed match assessments, search rules, applications, structured materials and versions, and append-only progress events. Other modules retain their own Task, identity, budget, notification and binary-artifact authority.
- Keep the Career Office public interface small: open a career reference, list filtered views, act on a closed set of business intents with request ID and expected revision, and read changes. Read scope comes from authenticated user and Tenant context, never client-supplied ownership fields.
- The job-search mobile app uses the existing Expo/React Native project for iOS and Android. Native components map the TDesign Mobile React visual language; the mobile-web component library is not imported into React Native. HarmonyOS native delivery is a separate compatibility gate requiring a real native build, authenticated Task read and platform capability probes. The WeChat mini-program keeps Taro 4 and integrates TDesign Miniprogram. Platform adapters own authentication, files, sharing, notifications and controlled storage.
- Give cross-client interaction state one owner, Career Desk. It exposes open/list/act/observe to Web, Expo iOS/Android and WeChat Taro 4 presentation adapters, with HarmonyOS only after its native compatibility gate. iOS/Android use Expo/React Native, HarmonyOS requires a separately validated native adapter, and WeChat uses Taro 4 with TDesign Miniprogram. It coordinates revision conflicts, pending intent recovery, scope changes and cache invalidation. Platform adapters provide file selection, share intake, controlled storage, navigation and notification subscription.
- A one-time or scheduled search runs as a Task but is distinct from a job-specific application Task. Creating an application must recover an unknown Task-link result with the same idempotency key instead of creating a second Task.
- A job opportunity can have several source observations. A fixed job snapshot is used for every assessment and application; source changes invite re-evaluation and never rewrite historical evidence. Deduplication needs sufficient evidence including job identifier, employer, place, role and hiring batch.
- Eligibility is compliant, noncompliant or needs confirmation for explicit hard requirements. Skill fit is a separate evidence-backed explanation, not a hiring probability. An applicant may explicitly continue despite a hard conflict; the conflict stays visible and this application is excluded from the qualified-application metric.
- The job-source interface accepts official employer campus pages, national or university platforms, legally accessible recruitment platforms and user-supplied content. Each adapter reports original link, retrieval time, completeness and failure state. It does not bypass login or access restrictions.
- Uploaded résumés and proof documents produce proposed facts. Only user-confirmed facts can become authoritative profile input for evaluation or generated claims. Minimize and mask unrelated identifiers before model use. Treat JDs as untrusted data, not Agent instructions.
- Material generation freezes input versions, drafts, performs independent claim review, receives user confirmation, renders PDF and DOCX from one structured body, verifies both exports, and publishes an immutable material version only after validation. A failed or unknown export stays pending and cannot appear ready for submission.
- The application records user-confirmed submission channel, time and exact material version. Progress is append-only; corrections append a new event. Current stage is a projection, with a stable event vocabulary fixed in the implementation contract.
- Use request IDs for idempotency and expected revisions for updates. A request ID with changed content is rejected. Unknown outcomes require receipt reconciliation, not blind retry. Cross-module side effects cannot be treated as one database transaction.
- Career holds opaque Artifact references. Existing Workbench authority must expose a version-bound download grant that checks Tenant, owner, application and material version and is revocable after deletion.
- Career search rules trigger budgeted runs through the existing admission path. Career emits non-sensitive attention facts; Workbench owns inbox and delivery. Push messages only prompt resynchronization.
- Data export and deletion go through the existing Tenant retention authority and must also revoke Task, Artifact and client cache access; deleting only career rows is insufficient.
- Web, the Expo mobile app (iOS and Android, plus HarmonyOS after its native gate), and the Taro 4 + TDesign Miniprogram WeChat mini-program deliver the same business capabilities with device-appropriate layout. The user selected prototype C, the conversational command view, on 2026-09-24. Natural-language job search is the primary entry point; each result keeps eligibility, evidence, unresolved facts, original source and next actions together. Profile, applications and materials retain explicit navigation on all three clients, and narrow screens must not hide hard-condition conflicts. The approved visual constraint is the WeKnora TDesign light theme, including the `#07c05f` brand ramp and its design tokens. Prototype data and mock actions are not production behavior.
- Any reused code, prompts or templates from ai-job-search or career-ops require provenance and license review. The public product uses WeKnora naming.

## Testing Decisions

- Test observable outcomes at the highest stable seams: Career Office for domain and authority behavior and Career Desk for shared three-client interaction behavior. Prefer a small number of end-to-end contract scenarios over tests that repeat helper implementation.
- Career Office tests cover cohort mismatch and unknown eligibility; persistent warning on explicit override; multi-source deduplication and hiring-batch distinction; immutable job snapshots; confirmation-gated profile facts; tenant and owner isolation; event correction; request idempotency and revision conflicts; quota exhaustion; unknown-result recovery; and deletion revoking downloads.
- Career Desk tests cover cross-client material version visibility, stale revision conflicts, pending receipt reconciliation, scope switch cancelling late responses, controlled offline drafts requiring user submission, and actionable error presentation.
- Adapter contract tests use fixed source responses to exercise complete, incomplete, blocked and changed jobs; real PDF/DOCX rendering to inspect extracted text, visual page output and editability; and the existing Identity, Workbench admission, Task, Artifact and notification interfaces for access and recovery.
- User-flow acceptance runs on Web, Expo iOS, Expo Android, a verified HarmonyOS native app and a real Taro 4 + TDesign Miniprogram WeChat mini-program environment, from intake to job evaluation, material export, user-confirmed submission and follow-up. Share import, file download and notification permissions require device-level checks; a shared-library test or successful build alone is insufficient.
- Existing repo precedents are Workbench Task and artifact authorization behavior tests, mobile-core scope and runtime tests, and Taro task-flow tests. Reuse their public seams rather than adding per-field helper tests.
- A good test changes a fact or input and observes the externally meaningful result: for example, changing a graduation year changes eligibility while leaving the old application snapshot readable. It should not assert internal call order, private helpers or cosmetic markup.

## Out of Scope

- Automatic submission to recruitment sites, automatic email sending, cross-site form filling, CAPTCHA or login bypass.
- Reading a user's mailbox to infer application progress.
- Editing a DOCX binary directly inside the app; the editable object is the structured material body, and DOCX is an export.
- Guaranteed nationwide job coverage, admission-probability predictions or guaranteed offers.
- Replacing WeKnora identity, Task, Run, budget or Artifact systems with the reference projects' CLI storage or services.
- Treating the three exploratory prototype layouts as approved production UX, or publishing the throwaway prototype as a product page.
- Fixing exact free-tier quantities, prices or source catalog before cost and access validation.

## Further Notes

- Source of product decisions: the approved 2026-09-23 WeKnora career design, nine rounds of discovery, the 2026-09-24 selection of prototype C, the project domain glossary, and career ADRs 0015–0018. The proposed Career Office/Career Desk module boundaries come from the codebase-design recommendation and need implementation-contract review before coding.
- First-wave active aggregation targets Beijing, Shanghai, Shenzhen and Hangzhou; manual import is not city-limited. Publish an actual source list and coverage before launch.
- The launch gate is a complete Web, Expo iOS/Android, verified HarmonyOS native app and Taro 4 + TDesign Miniprogram WeChat mini-program loop. Public operations must separately verify job-source terms, deployment/data processing, WeChat capabilities, personal-information notice and payment flow.
- Primary outcome measures are completed qualified applications and material trustworthiness, supported by preparation time, incorrect eligibility rate and interview feedback. Raw application count is not the primary success measure.





## 评论

无。
