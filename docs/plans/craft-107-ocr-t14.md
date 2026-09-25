# T14 OCR 报告（第 1 次运行）

范围说明：ask 给定线性范围 eeb1ee2db^..7b223e731 经 git 核实横跨 36+ 个非 T14 已审提交（T01-T10 lane 交错，1579 变更文件 / +34.3 万行，不可行且违背任务级增量意图，同 T08 核实先例）。T14 实际提交 2 个（不相邻）：eeb1ee2db（现场收编）与 7b223e731（接线补全，HEAD）。实际分两段审查：段 A --commit eeb1ee2db（2 文件被审），段 B --from c799642d0 --to 7b223e731（2 文件被审）。以下为两段 ocr 原文合并。

## 段 A：--commit eeb1ee2db

Review complete: 5 finding(s) across 2 selected item(s).

─── internal/application/service/craft_preview.go:111-115 ───
[performance · medium] 每次调用 NetworkMode 都新建一个 moby client 并在使用后立即 Close。该检查器位于 lookup()
的每资源请求路径上：一个预览页加载 N 个静态资源就会执行 N 次 CheckPreviewNoEgress，即 N 次 client.New + N 条新建 unix socket 连接，且每个新
client 的版本协商是惰性完成的（sandbox 模块 docker_engine.go 注释明确指出协商要花一次额外往返），成本再翻倍。沙箱模块自身的惯例是
dockerEngineClientPool 按 endpoint 复用共享 client。建议按 daemon host 缓存 client（如包级
sync.Once/map+mutex），复用时不要在每次请求后 Close。

─── internal/application/service/craft_preview.go:116-116 ───
[bug · medium] ContainerInspect 直接使用调用方的请求 ctx 且无任何期限。沙箱模块对短 Engine API 调用的既有惯例是
withDockerRPCTimeout 包一层 WithTimeout（docker_rpc_timeout.go 明确覆盖 ContainerInspect）；Gin 默认没有 server
级超时，若本地 daemon 挂起，每个预览资源请求（页面上是每资源一次）都会阻塞 worker 直到浏览器断开连接（request ctx 取消）。建议为 inspect
派生一个短超时上下文，与模块内 RPC 预算一致。

─── internal/application/service/craft_preview.go:229-230 ───
[bug · low] AcceptsPreviewHost 用 strings.EqualFold(c.Request.Host, u.Host)
做精确比较，未归一化默认端口：PreviewOrigin 配置为不带端口的 https origin（如 https://preview.example.com）而请求到达时 Host
带显式默认端口（preview.example.com:443，RFC 7230 允许，或某些反代原样转发带端口的 Host）时，比较失败导致预览在共享路由上静默 404。虽然
fail-closed，但属于部署相关的可用性陷阱且难以排查。建议比较前先剥离等于 scheme 默认端口（https=443）的端口号，仅非默认端口参与比较。

─── internal/application/service/craft_preview.go:130-133 ───
[maintainability · low] 此处 err 可能为 nil（binding 为 nil、StaleAt 非nil、Provider 非 Docker 或 ConfigID 为空时
Get 本身成功），错误消息会以 "...: <nil>" 结尾。该模式在下方 138 行（loadErr 为 nil 但 !selected.Found）和 150 行（actual 非 none
但 err 为 nil）重复出现。这些错误经 Issue 路径以 BadRequestError(err.Error()) 形式返回给已认证用户，"<nil>" 文本既误导排障也不专业。建议仅在
err != nil 时拼接底层错误文本（三处同样处理）。

─── internal/application/service/craft_preview.go:394-402 ───
[performance · low] lookup 将 O(1) 的本地 allowlist 校验（grant.files[rel]）放在了两轮 I/O
密集检查之后：RequireTaskAccess 走成员资格存储，requireNoEgress 走绑定存储 Get + 配置 Load + 容器 ContainerInspect。持有有效
capability 的浏览器对不在 manifest 内的路径（生成页面几乎必然触发的 /p/<cap>/favicon.ico、失效的相对资源引用、探测请求）每次都会完整执行这 3+
次后端往返后才得到 404，与已确认的 per-resource 检查成本叠加。allowlist 在签发时从不可变版本固化、无副作用，先做该检查再执行授权/egress
校验不会改变对外行为（handler 对 ErrNotFound/ErrForbidden/ErrUnsupported 统一映射 404），也不削弱安全。建议将 grant.files 检查移到
RequireTaskAccess 之前（Issue 中 requireNoEgress 先于 versions.Get 的排序同理，但那里两者均为 DB 查询、影响较小）。

## 段 B：--from c799642d0 --to 7b223e731

Review complete: 1 finding(s) across 2 selected item(s).

─── internal/container/container.go:2586-2590 ───
[bug · medium] 无 Redis（Lite 模式）时，selectSessionBindingStore(nil, false) 会新建一个独立的
MemorySessionSandboxBindingStore 实例；而会话绑定的实际写入方是 newTenantSandboxResolver 内部自建的另一个内存
store（sandbox.go:120 → tenant_resolver.go:197 SessionBoundManager.Store）。两个实例互不相通，因此单进程部署中即使会话已绑定真实的
Docker network_mode=none 沙箱，CheckPreviewNoEgress 的 Bindings.Get 也永远 miss，preview 签发恒返回
ErrUnsupported("no current Docker sandbox binding")，且仅有通用的 "No Redis configured" 日志，没有任何 preview
专属提示。结果是 fail-closed（安全上没问题），但功能在 Lite 模式下静默失效、难以排查。建议通过 DI 共享 resolver 的 binding store（或将单一 store
实例 Provide 出来注入两处）；若维持现状，至少应记录一条明确的日志/注释，说明 preview 签发需要 Redis 部署。
