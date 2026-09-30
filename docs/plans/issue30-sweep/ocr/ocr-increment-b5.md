Review partially complete: 13 finding(s); 66 of 102 selected item(s) failed.

─── internal/application/service/agent_upgrade.go:230-230 ───
[performance · medium] reconcileProposals 在每次 List/Get 请求时对每个 active adoption 重放 listing + from/to
两份 Release 读取，并完整解码两份 Bundle JSON（携带完整便携 payload）计算 diff，即使该 (adoption, to_release) 的 proposal
已物化（open 或 dismissed 终态）也不短路——FindOrCreateProposal 的存在性检查发生在昂贵的 diff
之后。GetMarketplaceListing/GetRelease 还可能触发 #60 introduction-ledger 合成回退，进一步放大每次请求的成本。adoption
数量较大的租户每次列表/详情请求都会产生 O(N) 次大 JSON 解码与 ~3N 条 DB 往返（读路径退化）。建议在解码前先按 (adoption_id, to_release_id)
做存在性短路（新增 scoped 读或在循环前一次性 ListProposals 建索引），已物化的组合直接 continue。

+ 		// 已物化（含 open/dismissed 终态）的 (adoption, to_release) 无需重算 diff：
+ 		// 先做 scoped 存在性检查，避免每次请求解码两份 Bundle。
+ 		if existing, err := s.repo.GetProposalByScope(ctx, tenantID, adoption.ID, toReleaseID); err != nil {
+ 			return err
+ 		} else if existing != nil {
+ 			continue
+ 		}
  		diff, err := diffUpgradeBundles(fromRelease, toRelease)


─── internal/application/service/agent_upgrade.go:212-215 ───
[bug · medium] open 状态的升级建议不会随 Adoption 前进而关闭：#59 re-adopt 路径（reconcileAdoptionTx，last-write-wins）会把
AcceptedReleaseID 前进到新 Release，但 reconcileProposals 只有「toReleaseID == AcceptedReleaseID 跳过创建」逻辑，缺少对
from_release_id 已与当前接受指针脱节的 open 建议的终态迁移；AcceptUpgradeProposal 也只校验 proposal open / adoption active
/ release 归属 listing，不校验 row.FromReleaseID == adoption.AcceptedReleaseID。结果：过期 open 建议永远残留在列表且可
Accept，创建指向已被越过（甚至更旧）Release 的冗余草稿 Variant。建议 reconcile 时将 from_release_id 与当前接受指针脱节的 open 建议迁至终态（如
stale/dismissed），并在 Accept 前加 FromReleaseID 一致性校验。



─── internal/application/repository/agent_marketplace_lineage.go:86-86 ───
[bug · medium] UpsertLicense 的 OnConflict DoUpdates 包含 created_by，与实体契约不符（AgentLicenseEntity 注释明确
re-register 只「updates the flags」），且与同为创建期字段的 created_at（未列入
DoUpdates、被保留）处理不一致——许可翻转的正常传播方式会永久丢失「首次登记人」的审计信息。另外冲突更新路径返回的是输入副本（CreatedAt=本次 now、CreatedBy=本次
actor），handler 直接以 licenseDTO(row) 渲染响应，API 会在重复登记后报出与库中行不符的 created_at/created_by。建议从 DoUpdates 移除
created_by，并在 upsert 后重读库中行返回。

- 			DoUpdates: clause.AssignmentColumns([]string{"name", "allows_redistribution", "created_by", "updated_at"}),
+ 			DoUpdates: clause.AssignmentColumns([]string{"name", "allows_redistribution", "updated_at"}),
+ 		}).
+ 		Create(license).Error
+ 	if err != nil {
+ 		return nil, err
+ 	}
+ 	// 冲突更新路径下输入副本不等于库中行（created_at/created_by 应保留原值），重读后返回。
+ 	return r.GetLicense(ctx, license.ID)


─── internal/application/service/agent_upgrade.go:33-36 ───
[maintainability · low] now func() time.Time 字段在整个代码库无任何调用点（生产代码、agent_upgrade_test.go、container
装配均未引用；时间戳全部由 repository 层生成），属死代码，并误导后续维护者以为服务层控制时间源。建议移除该字段与构造函数中的 now: time.Now 注入（并同步移除仅为其存在的
time 导入）。

  type AgentUpgradeService struct {
  	repo repository.AgentUpgradeRepository
- 	now  func() time.Time
  }


─── internal/modules/appconnector/confluence_create.go:87-89 ───
[bug · critical] Cloud v2 创建请求体键名与官方契约不符（对应仓库 OCR 判定 B5-F43）。Atlassian Confluence Cloud REST v2
`POST /api/v2/pages` 的请求体是 camelCase（`spaceId`/`parentId`）——同文件 `confluencePageWire` 对 v2 响应也按
camelCase（`json:"spaceId"`）解码，可互为佐证。当前发送 `space_id`/`parent_id`，真实 Cloud 实例会因缺少必填 `spaceId` 直接返回
400，被 createPage 映射为 definitive provider_error → ActionFailed：T20 的 Cloud 版本（atlassian.net 默认
edition）首次调用即不可用。测试 fake（confluence_create_test.go:120、publish/confluence_bridge_test.go:102）按同构
snake_case 解码，所以现有测试对这一缺陷无感。建议改为官方 camelCase 键名，并让契约 fake 仅认官方键（snake_case 返回 400），防止"fake
原样回显掩盖差异"再次发生。

  type confluenceCloudCreateRequest struct {
- 	SpaceID  string                `json:"space_id"`
- 	ParentID string                `json:"parent_id,omitempty"`
+ 	SpaceID  string                `json:"spaceId"`
+ 	ParentID string                `json:"parentId,omitempty"`


─── internal/modules/appconnector/confluence_create.go:162-165 ───
[bug · high] confluenceDo 的响应读取上限会静默截断（对应仓库 OCR 判定 B5-F44）。`io.ReadAll(io.LimitReader(r, N))` 在响应超过
N 字节时"成功"返回前 N 字节、不报任何错误。上游 MaxPublishArtifactBytes 允许 1MiB 原文，而 ConfluenceStorageBody 的
html.EscapeString 可把 `'`/`"`/`&` 各放大 4-6 倍，因此合法派发的 storage 正文可达数 MB；create/update 的 provider 回显以及
Query 对账读（GET 显式请求 body.storage）都会携带完整正文：一旦超过 1MiB，截断后的 JSON 解析必然失败 → 已实际成功的发布被判
ActionUnknown（回执不可证），且后续每次 Query 读回同样被截断、永远无法收敛到 succeeded。建议读 N+1 字节并在超限时显式报错（包
ErrConfluenceOutcomeUnknown，语义上诚实地落 unknown，而不是拿半截字节去解析）。

- 	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
+ 	const maxReplyBytes = 1 << 20
+ 	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxReplyBytes+1))
  	if err != nil {
  		return resp.StatusCode, nil, fmt.Errorf("%w: %v", ErrConfluenceOutcomeUnknown, err)
+ 	}
+ 	if len(raw) > maxReplyBytes {
+ 		return resp.StatusCode, nil, fmt.Errorf("%w: response exceeds %d-byte cap", ErrConfluenceOutcomeUnknown, maxReplyBytes)
  	}


─── internal/modules/appconnector/confluence_update.go:287-289 ───
[bug · high] Query 对账用字节精确相等比较 storage，会导致含引号的更新永久无法从 unknown 收敛（对应仓库 OCR 判定 B5-F71）。snap.Storage 由
ConfluenceStorageBody 用 Go `html.EscapeString` 生成（`'`→`&#39;`、`"`→`&#34;`），而 Confluence 服务端会把
storage XHTML 重新序列化（实体通常还原为裸字符、空白归一）。正文含引号/可归一空白时，一次实际已落地的 PUT 一旦因丢回复/5xx park unknown，本函数的
`cur.BodyStorage != snap.Storage` 永远成立 → 每次对账都返回 unverifiable，action 永久停在 unknown；又因为更新计划的授权依赖
LatestPublishedByDestination 只认 published 回执，该页面从此无法再经本管线更新（必须人工介入）。建议改为语义级比较：对两侧做
html.UnescapeString + 空白归一后比较，标题 trim 比较（判定失败时仍保持 unverifiable 的诚实语义）。

- 	if cur.Title != snap.Title || cur.BodyStorage != snap.Storage {
+ 	if strings.TrimSpace(cur.Title) != strings.TrimSpace(snap.Title) || !confluenceStorageEqual(cur.BodyStorage, snap.Storage) {
  		return ActionResult{State: ActionUnknown}, fmt.Errorf("confluence_query_unverifiable: remote content drifted from the approved snapshot")
+ 	}
+ 
+ // confluenceStorageEqual compares the read-back storage with the approved
+ // snapshot semantically: Confluence re-serializes storage XHTML (entity and
+ // whitespace normalization), so byte equality is an unsound basis.
+ func confluenceStorageEqual(remote, approved string) bool {
+ 	norm := func(s string) string { return strings.Join(strings.Fields(html.UnescapeString(s)), " ") }
+ 	return norm(remote) == norm(approved)
- 	}
+ }


─── internal/modules/codedelivery/gitlab_client.go:433-437 ───
[bug · medium] PullRequestForHead 缺少 target_branch 维度，GitLab 语义不可从 GitHub 平移：GitHub 中同一 head
分支全仓库只能有一个 open PR（head 即唯一键），而 GitLab 明确允许同一 source_branch 对不同 target_branch 存在多个开放 MR。当前按
source_branch+state 列表取第一条即复用，后果：(1) DraftPullRequest 可能复用一个指向其他 base 的无关 MR，把错误的 iid/web_url
写入回执，而本交付目标的 MR 永远不会被创建；(2) QueryProvider 凭一条无关 MR 就把 unknown 交付误判为 delivered（错误终态，无法自愈）。另外固定
per_page=20 不翻页，>20 条时目标 MR 也会漏检（409 兜底同样查不到）。建议在查询中加 q.Set("target_branch", base)（需将 base 从
material/PullRequestInput 传入），并在结果中比对 target。

- func (c *gitLabRestClient) PullRequestForHead(ctx context.Context, head string) (*PullRequestReceipt, error) {
+ func (c *gitLabRestClient) PullRequestForHead(ctx context.Context, head, base string) (*PullRequestReceipt, error) {
  	q := url.Values{}
  	q.Set("source_branch", c.branchOf(head))
+ 	q.Set("target_branch", base)
  	q.Set("state", "opened")
  	q.Set("per_page", "20")


─── internal/modules/codedelivery/gitlab_client.go:128-128 ───
[bug · medium] callRaw 以 8MB LimitReader 静默截断超限响应体：io.LimitReader 无法区分「恰好 N 字节」与「超过 N
字节」，超限时不产生任何错误。该路径服务于 Blob()（raw blob 拉取），而上游 maxBaselineBytes=16MB（service.go:21）允许单个基线文件最大约
16MB——两处限制互相矛盾且可达：(1) MaterializeBaseline 对 >8MB 的基线 blob 会把截断后的内容写进会话工作区（静默基线损坏）；(2) EnsureBranch
兜底取回的基线 blob（stagedBlobs 未命中、起点分支缺失该文件时）截断后被 base64 提交进 GitLab，任务分支上落地损坏内容，且无 sha 校验兜底。建议读 limit+1
判定超限显式报错，更稳妥的是在 Blob() 里校验 GitBlobSHA(raw)==sha（内容寻址天然是完整性校验，可同时覆盖传输截断）。

- 	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
+ func (c *gitLabRestClient) Blob(ctx context.Context, sha string) ([]byte, error) {
+ 	var raw []byte
+ 	err := c.callRaw(ctx, http.MethodGet,
+ 		"/api/v4/projects/"+c.projectSegment()+"/repository/blobs/"+url.PathEscape(sha)+"/raw", &raw)
+ 	if err != nil {
+ 		return nil, err
+ 	}
+ 	// 内容寻址校验：截断/损坏立即失败，绝不进入交付链。
+ 	if GitBlobSHA(raw) != sha {
+ 		return nil, fmt.Errorf("%w: blob %s integrity mismatch", ErrCodeTransport, sha)
+ 	}
+ 	return raw, nil
+ }


─── internal/modules/codedelivery/gitlab_client.go:108-112 ───
[bug · low] 响应体解码失败被归类为 ErrCodeRequestInvalid，违反该错误的既有契约：github.go:20
与本文件注释均定义其为「请求从未离开进程」（malformed request never left the process）。json.Unmarshal 失败发生在 http.Do 成功、读到
2xx 响应之后——请求明明已出网。误分类的可达后果：prepare 面（如 Repository/BranchProtected 的 200-畸形响应，包括被 4MB LimitReader
截断的合法大响应）会经 workbench_delivery.go:239 映射为 500
code_delivery_request_invalid（客户端/本地问题），而非提供方/传输问题，误导排障方向；同时把「远端已应答但不可解析」伪装成「可证未出网」。对照 GitHub
客户端对解码失败返回原始未归类错误（github_client.go:72）。建议解码失败改归 ErrCodeTransport（远端状态不可观测）。

  	if out != nil {
  		if err := json.Unmarshal(raw, out); err != nil {
- 			return &header, fmt.Errorf("%w: decode %s %s: %v", ErrCodeRequestInvalid, method, path, err)
+ 			// 2xx 已收到：远端状态不可观测，归 transport 而非 request-invalid。
+ 			return &header, fmt.Errorf("%w: decode %s %s: %v", ErrCodeTransport, method, path, err)
  		}
  	}


─── internal/modules/codedelivery/gitlab_client.go:353-355 ───
[bug · medium] 新任务分支从默认分支 start_branch 收敛，导致 commit actions 数 = 完整 default↔baseline 增量（而非仅本交付的
staged 变更）。基线是用户任意指定的历史 SHA（MaterializeBaseline 的核心场景就是锚定旧基线），默认分支自基线后的演进全部计入 delete/update 动作：超过
2*MaxDeliveryFiles=1000 即在派发时（批准已被消耗之后）以 ErrBaselineTooLarge 失败——错误语义误导（基线 ≤500 文件早已在 prepare
面通过校验，仓库其他部分并不受该上限约束）；默认分支 blob 超过 Tree 的 50 页上限（5000 entries）时同样派发期失败。对照 GitHub 路径：首颗 commit 直接以
BaselineSHA 为 parent、分支锚定基线 commit，无此增量。另外每次派发两次全量递归树拉取（intended+current）也放大了延迟与配额消耗。建议新分支先用 POST
/repository/branches（branch, ref=baselineSHA）把分支锚到基线上，再仅提交 staged entries；至少应在 prepare
面提前检出该增量或把此错误改归更准确的语义。

+ 	if !exists {
+ 		// 先把分支锚定到基线 commit，commit 动作只承载 staged 变更，
+ 		// 避免 default↔baseline 全量增量计入 cap。
+ 		if err := c.createBranchAt(ctx, branch, c.stagedBase); err != nil {
+ 			return err
+ 		}
+ 	}
  	if len(actions) > 2*MaxDeliveryFiles {
  		return fmt.Errorf("%w: %d commit actions exceed cap %d", ErrBaselineTooLarge, len(actions), 2*MaxDeliveryFiles)
  	}


─── internal/modules/appconnector/publish/confluence.go:30-38 ───
[maintainability · low] ConfluencePublishService 绕过了同一批次刚建立的 provider 中立抽象：publish/plan.go 的
NewProviderPublishService + provider.go 的 ProviderProfile 文档明确写着是 "#49 feishu, #50 confluence"
的指定扩展点（"ProviderProfile is the ONLY place a provider's plan-level identity lives (AC1)"），而本文件将近 150
行的 FormPlan/Execute/Reconcile/Receipt/project 与 plan.go 中 NotionPublishService 的实现逐行重复（仅 pre-read
严格度和 Conflict 前缀不同），ConfluenceVersionConflictResult/appIDConfluence 也落在 confluence_bridge.go 而非
profile。安全相关的 settle 逻辑（receipt 结算、冲突标记）从此要在两处并行维护，且复制件已经开始漂移——plan.go 的 project() 带有 "Defensive: a
succeeded payload that no longer parses ... skips settle" 注释和 `ConflictResultPrefix != ""`
守卫，本文件的复制件把两者都丢了。建议改为注册一个 ConfluenceProfile（CreateArgs/UpdateArgs 直接产出 {parent,title,storage} /
{page_id,expected_version,title,storage} 快照，BlocksOf 用 ConfluenceStorageBody，ParseReceipt 用
ParseConfluencePageReceipt），并把 NewProviderPublishService 的 scope 源参数泛化后复用；短期至少应回补 project()
的防御性说明与守卫，避免两份结算逻辑进一步分叉。



─── internal/modules/codedelivery/gitlab_client.go:351-351 ───
[bug · medium] 空收敛 + 新分支路径返回裸 ErrInvalidMaterial，违反本模块自身的 settle 契约，会把交付永久卡死在 unknown。该错误发生在 commits
API 调用之前——是「可证明未出网」的本地拒绝（与 ErrDispatchNotStarted 同类），但 deliver()
原样上抛后，settleOutcome（action.go:576-582）只认 ErrDispatchNotStarted 落 failed，其余一律落
ActionUnknown。此路径既没创建分支也没创建 MR，QueryProvider（dispatcher.go:283-293）永远查不到远端事实（分支缺失、MR 缺失 → 持续
ErrDispatchUnknown），交付行在批准已被消耗后永久滞留 unknown，无任何自愈路径。触发可达：基线为较旧 SHA、工作区树恰好等于当前默认分支树（prepare 面 changes
非空校验可通过——workspace≠baseline 但 intended==default），首次派发即命中。建议在 dispatcher.deliver
把适配器本地材料错误按前置门拒绝包装（与同函数内 workspace 读失败的处理一致）：

- 		return fmt.Errorf("%w: task branch %s would be empty; nothing to push", ErrInvalidMaterial, branch)
+ // dispatcher.go deliver() 内（EnsureBranch 调用处）：
+ if err := client.EnsureBranch(ctx, material.Branch, commitSHA); err != nil {
+ 	if errors.Is(err, ErrInvalidMaterial) || errors.Is(err, ErrBaselineTooLarge) {
+ 		// 适配器本地拒绝：可证明未出网，按前置门落 failed（区别于远端 4xx/传输错误）
+ 		return appconnectorsvc.DispatchOutcome{}, fmt.Errorf("%w: %v", appconnectorsvc.ErrDispatchNotStarted, err)
+ 	}
+ 	return appconnectorsvc.DispatchOutcome{}, err
+ }


LLM retry report summary: 34 of 288 requests affected -- 22 requests failed, 12 requests recovered after retry

Review planning (13 requests):
- apps/mobile/src/app/tasks/detail.tsx,apps/mobile/src/app/tasks/materials.tsx,apps/mobile/src/composition.ts,apps/mobile/src/screens/MaterialsScreen.tsx,apps/mobile/src/screens/TaskDetailScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks/research.tsx,apps/mobile/src/research-integration-smoke.ts,apps/mobile/src/research-view.ts,apps/mobile/src/screens/ResearchScreen.tsx,packages/api-client/src/mobile/research.ts,packages/contracts/src/mobile/research.ts,packages/mobile-core/src/research/in-memory-research-remote.ts,packages/mobile-core/src/research/task-research.ts,packages/mobile-core/src/research/types.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/ios-core-workflow-integration-smoke.ts,apps/mobile/src/ios-release-evidence-cli.ts,apps/mobile/src/ios-release-evidence.ts,docs/plans/issue30-sweep/ios-evidence/t39-outcomes.json,docs/plans/issue30-sweep/ios-evidence/t39/push-payload.json,docs/plans/issue30-sweep/ios-evidence/t39/t39-record.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/application/repository/task_research.go,internal/container/workbench.go,internal/handler/session/workbench_delivery.go,internal/handler/session/workbench_research.go,internal/router/routes_workbench.go,internal/types/task_research.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/container/confluence_publish.go,internal/container/container.go,internal/container/feishu_publish.go,internal/container/notion_publish.go,internal/router/routes_app_action_plan.go,internal/router/routes_app_confluence_publish.go,internal/router/routes_app_connectors.go,internal/router/routes_app_feishu_publish.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 8 more

Core review (21 requests):
- apps/mobile/.gitignore,apps/mobile/app.json,apps/mobile/eas.json,apps/mobile/package.json,apps/mobile/scripts/emit-acceptance-record.ts,apps/mobile/scripts/ios-acceptance-run.sh,apps/mobile/scripts/ios-release-build.sh: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/adapters/dictation-capture.ts,apps/mobile/src/app/tasks/voice.tsx,apps/mobile/src/screens/VoiceRoomScreen.tsx,apps/mobile/src/voice-room-integration-smoke.ts,apps/mobile/src/voice-room-view.ts,packages/api-client/src/mobile/voice-sessions.ts,packages/mobile-core/src/voice-room/in-memory-voice-session.ts,packages/mobile-core/src/voice-room/voice-room.ts,packages/mobile-core/src/voice/dictation.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/adapters/notification-permission.ts,apps/mobile/src/android-acceptance.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks/detail.tsx,apps/mobile/src/app/tasks/materials.tsx,apps/mobile/src/composition.ts,apps/mobile/src/screens/MaterialsScreen.tsx,apps/mobile/src/screens/TaskDetailScreen.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/mobile/src/app/tasks/research.tsx,apps/mobile/src/research-integration-smoke.ts,apps/mobile/src/research-view.ts,apps/mobile/src/screens/ResearchScreen.tsx,packages/api-client/src/mobile/research.ts,packages/contracts/src/mobile/research.ts,packages/mobile-core/src/research/in-memory-research-remote.ts,packages/mobile-core/src/research/task-research.ts,packages/mobile-core/src/research/types.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 16 more

Per-attempt detail: --format json (retry_report).
