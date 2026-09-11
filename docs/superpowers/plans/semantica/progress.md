# Semantica 实施台账

状态：V01–V03、C01–C02 已完成并验证，其余 19 个任务未开始。总计划：[实施入口](../2026-09-11-semantica-implementation.md)。

状态值 pending / in_progress / blocked / implemented / verified。每项验证记录必须包含commit SHA、精确命令、退出码、环境、产物路径、失败/限制；无真实证据不标记verified。执行前记录实际基线与已有脏文件。

| 任务 | 名称 | 依赖 | 状态 | 证据/提交/阻断原因 |
|---|---|---|---|---|
| V01 | 冻结版本与最小安装契约 | 无 | verified | semantica==0.6.8（+graph-neo4j extra）冻结于上游 commit f73f599a；Python 3.12.13；见运行记录 2026-09-11 V01 与 capability-evidence.json |
| V02 | 验证持久图桥接和两类推理 | V01 | verified | Neo4j 5.26 真实持久化+进程重启溯源通过；规则正/负/中文推导通过；模型推断无受批准凭据保持 unverified（未发起调用）；见运行记录 2026-09-11 V02 与 bridge-evidence.md |
| V03 | 中文质量与上线阈值评估基线 | V02 | verified | semantica 模式 30 题实测（正确率 0.767、泄漏 0、recall 0.925）；native 对照阻断于隔离测试部署+模型凭据，approved=false；见运行记录 2026-09-11 V03 与 evaluation-baseline.md |
| C01 | 版本化协议和跨语言领域类型 | V01 | verified | proto 全 14 DTO+7RPC 双语言同源生成（幂等）；Go 全量映射+全 5 枚举表；uint64 十进制字符串边界；未知枚举不映射成功；见运行记录 2026-09-11 C01 |
| C02 | 认证服务骨架和Go客户端 | C01 | verified | 真实 TLS+内部令牌 gRPC 服务（缺身份→UNAUTHENTICATED、未实现→UNIMPLEMENTED、健康真实）；Go 客户端 deadline/取消/错误映射+仅读操作重试；enabled=false 默认可启动；见运行记录 2026-09-11 C02 |
| C03 | 事实与证据校验模型 | C01,V02 | pending | 尚未执行 |
| I01 | 持久操作、幂等与worker租约 | C02 | pending | 尚未执行 |
| I02 | 业务revision、outbox与授权版本 | C01 | pending | 尚未执行 |
| I03 | 有来源的构图与generation原子发布 | C03,I01,I02,A03 | pending | 尚未执行 |
| I04 | 删除屏障、支持撤销和清理receipt | I03,A01 | pending | 尚未执行 |
| I05 | 文档任务、attempt与终态协调 | I04 | pending | 尚未执行 |
| A01 | 可信AccessScope与权限变更屏障 | C02,I02 | pending | 尚未执行 |
| A02 | 授权事实子图与缓存隔离 | A01,I03,I04 | pending | 尚未执行 |
| A03 | 模型代理、原始用量与预算 | C02,I01,A01 | pending | 尚未执行 |
| Q01 | GraphRAG检索与有界执行 | A02,V03 | pending | 尚未执行 |
| Q02 | 注册规则与可核验推导 | Q01 | pending | 尚未执行 |
| Q03 | 模型推断与证据不足判定 | Q01,A03,V03 | pending | 尚未执行 |
| Q04 | Go检索融合、Agent工具与最终授权 | Q02,Q03,I05 | pending | 尚未执行 |
| W01 | 用户API与共享客户端契约 | Q04,I05 | pending | 尚未执行 |
| W02 | React索引状态与推理证据流程 | W01 | pending | 尚未执行 |
| W03 | 后端影子构建、切换与回滚 | W01,I04,Q04 | pending | 尚未执行 |
| O01 | 独立部署、探针与可观测性 | C02,I03,A03 | pending | 尚未执行 |
| O02 | 故障注入、清理与恢复演练 | O01,I04,W03,Q04 | pending | 尚未执行 |
| O03 | 质量回归、CI门禁与最终交付 | O02,W02,V03 | pending | 尚未执行 |

## 运行记录模板

每次执行新增一条记录，填写实际内容，不改写旧证据：日期、任务、基线SHA、锁文件hash、修改文件、RED命令及退出码、GREEN命令及退出码、真实环境、证据位置、review结果、提交SHA、剩余限制。

### 2026-09-11 V01 冻结版本与最小安装契约（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：d3702543491865afbbe2fba14231121d44e82d06。
- 锁文件 hash（uv.lock sha256）：40dde78c3e19077419d36947bf6ee88bfe454239040304f59b152b549c395abe。
- 上游冻结：semantica[graph-neo4j]==0.6.8（PyPI wheel sha256 0af4d9dd…5917d7；源码 tag v0.6.8 → commit f73f599a22c320676a45f247247038ca1fcf40f0）。Python 3.12.13（uv 托管），pytest>=8.0 为唯一 dev 依赖。
- 修改文件：semantic/experiments/{pyproject.toml,uv.lock,.python-version,test_import_contract.py,verify_version.py}、docs/superpowers/plans/semantica/capability-evidence.json、本台账、00 计划勾选。
- RED：`uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q`（退出码 1，ModuleNotFoundError: No module named 'semantica'——测试环境 pytest/解释器正常，失败即目标能力缺失）。
- GREEN：同命令退出码 0（1 passed；首次冷导入约 210s，重跑约 5s）。
- 干净环境重放：复制 pyproject.toml+uv.lock+.python-version 到 /tmp/semantica-replay，`uv sync --frozen` 退出码 0，导入七类 API 成功（semantica 0.6.8 / Python 3.12.13）。与 docreader 项目环境（docreader/pyproject.toml 独立 uv 项目、无共享 venv）互不依赖。
- 证据：`uv run --project semantic/experiments python semantic/experiments/verify_version.py --output docs/superpowers/plans/semantica/capability-evidence.json` 退出码 0；产物含 7 项 capability 的真实 inspect.signature、源码 commit、wheel/lock 哈希与合同测试实跑退出码。
- 环境：macOS arm64（darwin/aarch64），uv 0.9.30；PyPI 大文件下载多次超时，torch 121MiB wheel 以 curl -C - 断点续传取回并经 sha256 校验后放入 UV_FIND_LINKS 本地目录完成安装（一次性引导手段，不进入仓库；干净重放使用 uv 缓存完成）。
- review：规格符合性 PASS、代码质量 PASS（子代理复核：7 项签名与实机 inspect 一致、lock 哈希与 shasum 一致、tag→commit 独立复现一致、无凭据泄漏）。两个 MINOR（pin 常量硬编码、未交叉校验已装版本）已在提交前修复：verify_version.py 改为从 pyproject/uv.lock 解析 pin 并校验 installed==pinned；fail-closed 已实测（裸解释器运行退出码 1 并写入 failures）。
- 提交 SHA：f2ea1ea（feat(semantic): v01 冻结版本与最小安装契约；含 .python-version，因仓库 .gitignore 点号通配规则对它使用 git add -f，仅为保留 Python 3.12.13 补丁冻结）。
- 剩余限制：仅静态导入/签名契约；未验证运行时行为（持久图、推理、模型代理留给 V02/V03）。

### 2026-09-11 V02 验证持久图桥接和两类推理（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：e89d5a22da5ba42e01b11a281043b4248c064663（V01 台账提交）。
- 修改文件：semantic/experiments/{fixtures/controlled_graph.json,test_graph_bridge.py,test_reasoning_bridge.py,bridge_probe.py}、docs/superpowers/plans/semantica/bridge-evidence.md、本台账、00 计划勾选。
- RED：`uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q` 退出码 2，两文件 ModuleNotFoundError: No module named 'bridge_probe'（目标 probe 未实现；同环境 V01 契约测试 1 passed 证明测试环境本身正常）。
- GREEN：同命令退出码 0（10 passed）。
- 评审修复 RED：`... -m pytest semantic/experiments/test_reasoning_bridge.py -q` 退出码 1，test_model_probe_reports_failure_for_broken_approved_entry 复现质量评审 BLOCKER（AssertionError: 'completed' == 'failed'，配置失败的模型入口被伪装成成功）。
- 评审修复 GREEN：两文件合跑退出码 0（终态 13 passed，含新增失败入口、生成失败存根、全行溯源用例；模块级夹具后约 5s）。
- 回归：`uv run --project semantic/experiments python -m pytest semantic/experiments/ -q` 13 passed；9 次全目录合跑中 1 次在解释器退出阶段出现 libc++ recursive_mutex 原生崩溃（发生在 13 passed 打印之后，单文件/成对命令稳定），记入限制。
- 环境：macOS arm64；uv 0.9.30；Python 3.12.13；semantica 0.6.8（neo4j driver 6.3.0）；隔离 Neo4j 5.26-community 容器 semantica-v02-neo4j（127.0.0.1:17687，口令经环境变量注入，证据文件不含明文口令）。
- 证据：`uv run --project semantic/experiments python semantic/experiments/bridge_probe.py evidence --output docs/superpowers/plans/semantica/bridge-evidence.md` 退出码 0。实测：restart_verified=true（独立写进程退出后新连接读回）、attribution_fidelity_ok=true（含 d3/d4 全部 4 边 4 节点对照 fixture）、evidence_ids=[e-d1,e-d2]、甲→乙→丙两跳路径 + ContextRetriever 两跳扩展、规则正例 controls(a,c)/负例空推导/中文 controls(甲公司,丙公司)、冲突并存（d1/d3 同存各自溯源）、模型 unverified（无受批准凭据，未调用，无伪造用量；证据全文无"松柏"泄漏）。
- review：规格符合性 PASS（9 项通过；2 项 MINOR：证据缺复现命令——已补，默认实验口令入码——记录接受）。代码质量首轮 FAIL（BLOCKER：配置失败入口被伪装 completed；另有 8 项 MINOR）→ 按 TDD 修复（先新增复现测试 RED，再修复，再 GREEN）→ 复审 PASS（BLOCKER 三条路径实测确认解决；attribution_fidelity_ok 经 DB 级篡改判别实验证明有效；复审新增 2 项 MINOR：证据口令明文——已改环境变量引用，生成失败分支缺回归测试——已补存根测试）。
- 提交 SHA：fb13414（feat(semantic): v02 验证持久图桥接和两类推理）。
- 剩余限制：模型推断保持 unverified（无受批准模型入口凭据，Q03/O03 真实模型验收仍需凭据）；ContextRetriever 向量语义检索路径未验证；全目录合跑约 1/9 概率退出阶段原生崩溃（不影响测试结果，O03 CI 门禁需回访）；probe 写入端无跨进程锁需串行运行。

### 2026-09-11 V03 中文质量与上线阈值评估基线（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：ab25417（V02 台账提交）。
- 修改文件：semantic/experiments/{evaluate.py,test_evaluate.py,fixtures/questions.jsonl,fixtures/evaluation_corpus.json}、docs/superpowers/plans/semantica/{semantica-results.jsonl,native-results.jsonl,evaluation-baseline.md,acceptance-policy.json}、本台账、00 计划勾选。
- RED：`uv run --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q` 退出码 2，ModuleNotFoundError: No module named 'evaluate'（评分器未实现）。
- GREEN：同命令退出码 0（5 passed）；评审修复 RED：新增 4 个行为测试失败（leak 大小写敏感、数据集无校验、空数据集静默通过），`... -m pytest semantic/experiments/test_evaluate.py -q` 4 failed/8 passed；修复后 12 passed 退出码 0。
- semantica 实测：`uv run --project semantic/experiments python semantic/experiments/evaluate.py --backend semantica --dataset semantic/experiments/fixtures/questions.jsonl --output docs/superpowers/plans/semantica/semantica-results.jsonl` 退出码 0（30 题：正确率 0.767，fact/conflict/zh_locate/permission 1.0、multihop 0.4、unanswerable 0.2，source_precision 均值 0.628、source_recall 非空均值 0.925，p50 冷/热 0.032/0.015ms，p95 0.063/0.020ms，索引 741.4ms，tokens 0，泄漏 0）。
- native 尝试：`... evaluate.py --backend native ... --output docs/superpowers/plans/semantica/native-results.jsonl` 退出码 2，30 条错误行+blocked_reason 全部落盘（未配置隔离测试端点；未连任何生产环境）。
- 回归：四个实验测试文件合跑 26 passed 退出码 0（V01 1 + V02 13 + V03 12）。
- 环境：macOS arm64；Python 3.12.13；semantica 0.6.8；隔离 Neo4j 5.26（semantica-v02-neo4j，127.0.0.1:17687）。
- review：规格符合性 PASS（9 项通过；独立复算全部汇总统计与结果文件一致；3 项 MINOR：字段名与计划接口差异——已改 source_precision/source_recall/unanswerable_correct，台账勾选滞后——本次完成，leak 子串匹配备注——Q01 回访）。代码质量 PASS（0 BLOCKER、12 MINOR；已修复：大小写不敏感泄漏门+去重、数据集类别/必填校验、空数据集退出、argparse 用法错误退出码 1、逐案例错误行、recall 均值剔除空预期题（0.95→0.925 修正后重跑）、泄漏负载加入命中内容对象、document_revision 读自 fixture、别名清理、native available 语义与空延迟置 None；保留：共享 probe 标签串行约束已在 baseline 限制中记录）。
- 阈值：acceptance-policy.json approved=false；泄漏 hard gate proposed_max=0（实测 0）；multihop/unanswerable 阈值=当前实测值（诚实下限，Q01/Q03 后必须上调）；native 与对照门禁 unmeasured。
- 提交 SHA：2443e61（feat(semantic): v03 中文质量与上线阈值评估基线）。
- 剩余限制：native 对照与模型模式质量/用量门槛待隔离测试部署+真实模型凭据；受控语料规模不代表生产规模；评测需串行运行（与 V02 共享 probe 标签）。

### 2026-09-11 C01 版本化协议和跨语言领域类型（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：6da1df7（V03 台账提交）。
- 修改文件：semantic/{pyproject.toml,uv.lock,.python-version}、semantic/proto/{semantic.proto,semantic.pb.go,semantic_grpc.pb.go}、semantic/semantic_service/{__init__.py,contracts.py,proto/*}、semantic/scripts/generate_proto.sh、semantic/tests/{test_contract.py,fixtures/contract-v1.json}、internal/types/semantic.go、internal/types/interfaces/semantic.go、internal/infrastructure/semantic/{mapping.go,contract_test.go}、本台账、01 计划勾选。
- RED（Python）：`uv run --project semantic python -m pytest semantic/tests/test_contract.py -q` 退出码 2，ImportError: cannot import name 'contracts'（先建好 pyproject+包骨架使环境可用）。
- RED（Go）：`go test ./internal/infrastructure/semantic -run TestContract -count=1` 退出码 1，no required module provides package .../semantic/proto（proto 生成物不存在）。
- 评审修复 RED/GREEN（两轮）：第一轮 RED=Go 映射面缺失（undefined: ChunkToWire 等，编译失败）→ 实现全 14 DTO+全 5 枚举表映射 → GREEN；第二轮 RED=TestContractInvalidPurposeNeverDropsAccessScope（签名编译失败）→ SearchRequestToWire 改双返回值并传播授权信封错误 → GREEN。
- GREEN（终态）：`uv run --project semantic python -m pytest semantic/tests/test_contract.py -q` 12 passed 退出码 0；`go test ./internal/infrastructure/semantic -run TestContract -count=1` ok 退出码 0。
- 生成与幂等：`bash semantic/scripts/generate_proto.sh` 退出码 0，二次生成 sha256 逐文件一致；生成器锁定 grpcio-tools 1.80.0 / protoc-gen-go v1.36.11（=go.mod protobuf）/ protoc-gen-go-grpc v1.5.1（插件经 GOPROXY 代理安装，属授权依赖安装）。
- 覆盖：契约 fixture 18 键双向驱动（domain-JSON→wire→bytes→domain）；最大 uint64 不经 float64（Go ,string 标签+自定义 Evidence 编组，Python to/from_json 十进制字符串）；空 span 保持 nil/None；中文逐字节；未知 wire 枚举→unspecified、未知领域枚举→错误（含嵌套 AccessScope 不得静默丢弃的专项测试）；非空 error_code 保留。
- 环境：macOS arm64；Python 3.12.13；Go 1.26.3；grpcio 1.80.0/protobuf 6.33.x（uv.lock 锁定）。
- review：规格符合性 PASS（11 项：4.1 全部字段逐字对齐、7 RPC、tag 纪律、DTO 约定、接口签名、核心断言、JSON 边界、同源生成、无 protobuf 穿透、包卫生、文档——其中能力协商失败行为已按建议补记 proto+接口注释）。代码质量首轮 FAIL（BLOCKER：Go 映射面仅 5/14 DTO、2/5 枚举表）→ RED-first 补全 → 复审发现修复引入的新 BLOCKER（SearchRequestToWire 丢弃 AccessScope 错误=授权信封静默丢弃，评审员实证）→ 再次 RED-first 修复 → 终审 PASS（评审员独立探针验证错误传播、边界值 2^64-1 接受/2^64 拒绝、全部 MINOR 关闭）。
- 提交 SHA：3d4fa79（feat(semantic): c01 版本化协议和跨语言领域类型）。
- 剩余限制：QueryLimits uint32 字段 Python 侧构造期未校验（protobuf to_wire 拒绝越界，Go json 原生拒绝；C02 观察项）；.python-version 因仓库 .gitignore 点号规则需 git add -f（沿用 V01 先例）；semantica==0.6.8 生产依赖按计划在首个导入它的任务（I03/A03）进入本包锁。

### 2026-09-11 C02 认证服务骨架和Go客户端（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：12a1f8b（C01 台账提交）。
- 修改文件：semantic/semantic_service/{server.py,auth.py,config.py}、semantic/tests/{conftest.py,test_rpc_auth.py}、semantic/{pyproject.toml,uv.lock}（新增 grpcio-health-checking==1.80.0、trustme==1.2.1 dev）、internal/infrastructure/semantic/{client.go,client_test.go}、internal/config/config.go 与 internal/container/container.go（范围化增量）、本台账、01 计划勾选。
- RED：`uv run --project semantic python -m pytest semantic/tests/test_rpc_auth.py -q` 收集错误 ModuleNotFoundError: semantic_service.config（server 模块不存在；pytest/依赖环境已就绪）。
- GREEN：`uv run --project semantic python -m pytest semantic/tests/ -q` 21 passed 退出码 0（契约 12 + 认证 9）；`go test ./internal/infrastructure/semantic -count=1` ok（含 -race）；`go vet ./internal/...`、gofmt、`go build ./cmd/...` 全部通过。
- 评审修复循环（三轮，行为修复均 RED-first）：①容器 Invoke 在 Provide 之前导致 dig 即刻解析失败、服务无法启动（BLOCKER）→ 调整顺序并在两处新增行为测试（Get 重试端到端、重试退避中 ctx 到期须报 DEADLINE_EXCEEDED）；②修复引入的新 BLOCKER：ReadOnlyRetryAttempts<0 使 RPC 完全不被调用（评审员实证 nil capabilities）→ 先加 TestSemanticClientNegativeRetryStillInvokesOnce 复现（RED）→ 负值钳为 0（恰好一次调用）→ GREEN；③16 项 MINOR 全部修复（通道泄漏+fixture 工厂、try/finally 清理、单遍校验、allow_plaintext 更名修正语义、死代码删除、字节级常量时间比较+预编码、流式拒绝回归测试、元数据单值语义两端对齐、Close 幂等、重试语义文档化等）。
- 实测行为：缺令牌/错令牌（含 TLS 已信）→UNAUTHENTICATED；匿名健康 Check 与流式 Watch 均拒绝；已认证未实现方法（GetCapabilities/Apply）→UNIMPLEMENTED 非假成功；未知健康服务名→NOT_FOUND；客户端 deadline→DEADLINE_EXCEEDED、取消→CANCELED、连接失败→UNAVAILABLE、错误逐字透传；仅 Capabilities/Get 透明重试（实测 2 次调用），Apply 恒不自动重试（实测 1 次调用）；调用方携带旧令牌被剥离（单一 x-semantic-token 值端到端断言）。
- 生产边界：create_server 双重 fail-closed（config.validate+拒绝明文端口）；allow_plaintext 仅本地测试；enabled=false 默认返回 nil client 原系统不变；client 构造≠服务 ready（三处注释+接口文档）；容器清理经 ResourceCleaner 注册（nil 守卫）。
- 计划偏差记录：计划示意片段 verified_service_identity(context)/time_remaining() 服务端截止检查并入拦截器与 gRPC 核心机制（拦截先于 handler；核心强制 deadline），未单独保留死函数；semantic_service 生产锁暂不含 semantica==0.6.8（I03 首个导入任务进入，C01 已记录）。
- review：规格符合性 PASS（10 项：核心断言真实服务、fixture 真实端口+测试证书、fail-closed、UNIMPLEMENTED、健康真实、客户端契约、身份随行、可选配置、标准错误码、卫生）；代码质量终审 PASS（原 BLOCKER+16 MINOR+N1-N3 全部实证关闭；遗留非阻断 nits：closed-port TOCTOU、_free_port 窗口、deadline 测试通道进程退出关闭）。
- 提交 SHA：（本记录与代码同批提交后补记）
- 剩余限制：业务 RPC 全部 UNIMPLEMENTED（后续任务实现）；流式业务 RPC 落地时拒绝路径需流式处理器配套；首次真实部署（O01）前服务从未在本机外暴露。

## 当前边界

- V01–V03、C01–C02 已完成（verified）；后续 19 个任务未开始。
- V01精确版本已冻结（semantica 0.6.8）；真实模型证据须在后续任务补齐，不是已经通过的前提。
- V02 结论边界：持久图桥接/授权子图重建/注册规则推导已验证；模型推断 unverified（无凭据，未调用）；向量检索路径未验证。
- V03 结论边界：semantica 模式检索质量/延迟为受控语料实测；native 对照与模型用量门槛未测（阻断记录见上）；上线门禁 approved=false 待用户确认。
- 未创建GitHub Issue或外部发布；没有分配虚构Issue编号。
