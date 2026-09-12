# Semantica 实施台账

状态：V01–C03、I01–I04、A01、A03 verified；其余 12 个任务未开始。总计划：[实施入口](../2026-09-11-semantica-implementation.md)。

状态值 pending / in_progress / blocked / implemented / verified。每项验证记录必须包含commit SHA、精确命令、退出码、环境、产物路径、失败/限制；无真实证据不标记verified。执行前记录实际基线与已有脏文件。

| 任务 | 名称 | 依赖 | 状态 | 证据/提交/阻断原因 |
|---|---|---|---|---|
| V01 | 冻结版本与最小安装契约 | 无 | verified | semantica==0.6.8（+graph-neo4j extra）冻结于上游 commit f73f599a；Python 3.12.13；见运行记录 2026-09-11 V01 与 capability-evidence.json |
| V02 | 验证持久图桥接和两类推理 | V01 | verified | Neo4j 5.26 真实持久化+进程重启溯源通过；规则正/负/中文推导通过；模型推断无受批准凭据保持 unverified（未发起调用）；见运行记录 2026-09-11 V02 与 bridge-evidence.md |
| V03 | 中文质量与上线阈值评估基线 | V02 | verified | semantica 模式 30 题实测（正确率 0.767、泄漏 0、recall 0.925）；native 对照阻断于隔离测试部署+模型凭据，approved=false；见运行记录 2026-09-11 V03 与 evaluation-baseline.md |
| C01 | 版本化协议和跨语言领域类型 | V01 | verified | proto 全 14 DTO+7RPC 双语言同源生成（幂等）；Go 全量映射+全 5 枚举表；uint64 十进制字符串边界；未知枚举不映射成功；见运行记录 2026-09-11 C01 |
| C02 | 认证服务骨架和Go客户端 | C01 | verified | 真实 TLS+内部令牌 gRPC 服务（缺身份→UNAUTHENTICATED、未实现→UNIMPLEMENTED、健康真实）；Go 客户端 deadline/取消/错误映射+仅读操作重试；enabled=false 默认可启动；见运行记录 2026-09-11 C02 |
| C03 | 事实与证据校验模型 | C01,V02 | verified | codepoint 半开区间 span+原文校验、来源/推导分离、跨 scope 拒绝、循环 DAG 拒绝、多来源支持永不塌缩；见运行记录 2026-09-11 C03 |
| I01 | 持久操作、幂等与worker租约 | C02 | verified | 真实 PG（隔离容器）：幂等 accept/冲突检测、SKIP LOCKED 单胜领取、fencing token 失联接管、原子终态、取消 CAS 双向竞争；见运行记录 2026-09-11 I01 |
| I02 | 业务revision、outbox与授权版本 | C01 | verified | PG96/SQLite17 六表；WithSemanticMutation 单事务（CAS+outbox+deny+epoch）；原子领取/确认/退避；同事务 BumpSemanticEpochTx 供 A01；见运行记录 2026-09-11 I02 |
| I03 | 有来源的构图与generation原子发布 | C03,I01,I02,A03 | verified | 真实 PG：manifest 持久化/CAS 原子发布（首发布 INSERT ON CONFLICT、同 base 条件 UPDATE，真线程单胜实证）/read lease+续期/staged|publishing 过期可回收；见运行记录 2026-09-11 I03 |
| I04 | 删除屏障、支持撤销和清理receipt | I03,A01 | verified | 真实 PG：单调墓碑（deny 先于物理清理）/多来源撤销（合取前提递归失效）/分存储 receipt（backup 不伪装）/GC 窗口+清单闭包守卫；恢复模式重放归 I05；见运行记录 2026-09-11 I04 |
| I05 | 文档任务、attempt与终态协调 | I04 | pending | 尚未执行 |
| A01 | 可信AccessScope与权限变更屏障 | C02,I02 | verified | 11 条 ACL 接线+盘点修正（临时文档豁免实证）；短钥/伪造/漂移/过期均拒绝；完成评审 PASS；见运行记录 2026-09-11 A01（两段）与 acl-write-inventory.md |
| A02 | 授权事实子图与缓存隔离 | A01,I03,I04 | pending | 尚未执行 |
| A03 | 模型代理、原始用量与预算 | C02,I01,A01 | verified | 原子预算准入/幂等台账/unknown对账/新ID重试/受控入口（PG并发与死上下文实证）；无凭据真实调用保持未通过；见运行记录 2026-09-11 A03 |
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
- 提交 SHA：6738280（feat(semantic): c02 认证服务骨架和Go客户端）。
- 剩余限制：业务 RPC 全部 UNIMPLEMENTED（后续任务实现）；流式业务 RPC 落地时拒绝路径需流式处理器配套；首次真实部署（O01）前服务从未在本机外暴露。

### 2026-09-11 C03 事实与证据校验模型（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：4ed4310（C02 台账提交）。
- 修改文件：semantic/semantic_service/{evidence.py,facts.py}、semantic/tests/{test_evidence.py,test_facts.py}、semantic/semantic_service/contracts.py（跨文件增量，见下）、本台账、01 计划勾选。
- 跨文件披露：contracts.py（C01 所有）新增 7 行 \`ASSERTION_KINDS = frozenset(...)\`（由既有 wire 枚举表派生，unspecified 排除），为 facts.py 提供唯一事实来源、避免枚举漂移；纯增量无行为变化，随本任务提交并在提交信息说明。
- RED：`uv run --project semantic python -m pytest semantic/tests/test_evidence.py semantic/tests/test_facts.py -q` 收集错误（semantic_service.evidence/facts 不存在）。
- 评审修复 RED（两批）：①规格 BLOCKER——upsert 按内容去重忽略支持集，第二文档对同一断言的支持被静默丢弃（违反规格 §4 多来源支持与必测场景 4）→ 先写 test_multi_source_support_is_never_dropped 复现（RED）→ 仅当内容+支持集全同才去重；②质量 MINOR 批次 9 项行为修复全部 RED-first（同 id 异内容报错、跨租户并存、迭代三色 DFS 3000+ 深链、DAG 重复 id 报错、枚举单源、空 object/value 拒绝、推导不得直引证据、kb_id 空白拒绝、空/空白引文拒绝）。
- GREEN：`uv run --project semantic python -m pytest semantic/tests/ -q` 60 passed 退出码 0（契约 12+认证 9+证据 14+事实 25）。
- 覆盖：中文/emoji codepoint span、越界/倒序/零宽、两端同现同缺、篡改引文/哈希/chunk、revision=0、悬空 evidence/premise、跨租户与跨 KB premise、自环/深埋环/重复 id、多来源支持并存、同 subject/predicate 异值并存、实体名称/别名为有来源断言。
- 计划偏差记录：计划片段 extract_quote/validate_span 按原文实现；推导（rule/model）不得直接引用证据（支持经 premise 传递），为规格 §4/§7 推导语义的收紧；evidence 跨 scope 校验因 C01 Evidence DTO 无 scope 字段而归属装载层（I03），已在 docstring 与测试记录。
- review：规格符合性 PASS（10 项；终审确认多来源支持路径全部符合规格 §4 与场景 4）；代码质量 PASS（18 项发现全部独立探针复核；新增对抗探针：菱形 DAG、500 premise 扇出、深度 2000 埋环、50000 深链、NBSP 引文等全部正确；遗留 2 项外观 nits 无需处理）。
- 提交 SHA：1a6137a（feat(semantic): c03 事实与证据校验模型）。
- 剩余限制：upsert 为校验时辅助函数，批量摄取性能归 I03；同 id 同内容不同支持的并存条目由调用方在 generation 间对账（I03 范围）。

### 2026-09-11 I01 持久操作、幂等与worker租约（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：4afdc09（C03 台账修正提交）。
- 修改文件：semantic/migrations/001_operations.sql、semantic/semantic_service/{operations.py,worker.py}、semantic/tests/{test_operations.py,conftest.py（I01 扩展 fixture）}、semantic/{pyproject.toml,uv.lock}（psycopg[binary]==3.2.9）、本台账、02 计划勾选。
- RED：`uv run --project semantic python -m pytest semantic/tests/test_operations.py -q` 收集错误（semantic_service.operations 不存在）。
- 评审修复 RED（两批）：①规格 BLOCKER——状态表缺 (publishing,superseded)（规格 §6 发布 CAS 失败路径）→ 回归测试先行；②质量 BLOCKER——mark_result 两段事务存在崩溃窗口（终态已定但 result/租约未清且不可恢复）→ 改为单条守卫 UPDATE（原子终态+错误码+结果+租约释放）；另 8 项行为 MINOR（未领取 accepted CAS 分支、Not Found 异常类型、取消有界循环+scope 谓词、4MiB 内联上限、advisory lock 迁移器、accept 单往返、worker 边界、负例测试）与复审新增 2 项（renew 幽灵租约、RequestTooLargeError 语义）全部修复，多数先写失败测试。
- GREEN：`uv run --project semantic python -m pytest semantic/tests/test_operations.py -q` 22 passed；全量 `uv run --project semantic python -m pytest semantic/tests/ -q` 82 passed 退出码 0（真实 PG）。
- 验收实测：重复投递同 operation_id（含新实例"进程重启"）；同键异 payload→OperationConflictError；双线程双连接并发 claim 恰一胜（真实 PG FOR UPDATE SKIP LOCKED）；租约过期→新 worker 接管 token+1、旧 token 全部写入被拒（计划核心断言逐字通过）；取消/发布双向竞争恰一终态；终态不可逆；Cancel succeeded→FAILED_PRECONDITION；空库 claim None；未知/跨 scope 操作→OperationNotFoundError。
- 环境：隔离容器 semantica-i01-pg（postgres:16-alpine，127.0.0.1:15432，semantic/semantic，库 semantic_test；DSN 可经 SEMANTIC_TEST_PG_DSN 覆盖）；缺环境时 fixture 显式失败并给出启动指引（实测坏 DSN→14 errors exit 1，无任何 skip）。
- 迁移器：pg_advisory_lock(835471001) 串行化；001 作幂等引导并记入 semantic.schema_migrations（实测重复运行仅 1 行）。
- 计划偏差记录：CAS 守卫在计划 SQL 基础上增加 (state='accepted' AND lease_until IS NULL) 分支使未领取操作可被 superseded/取消路径触达（评审确认过期租约必为 running，该分支不可能服务失联 worker）；内联请求 4MiB 上限对应规格 §7 manifest_ref 大文档路径。
- review：规格符合性 PASS（9 项；两核心断言逐字、接口签名、schema、租约纪律、状态机、四项验收扩展、worker、无 skip、卫生）；代码质量 PASS（原 BLOCKER+10 MINOR+复审新增 2 MINOR 全部实证关闭；遗留 nits：迁移引导注释措辞、重复测试已清理）。
- 提交 SHA：8af2691（feat(semantic): i01 持久操作、幂等与worker租约）。
- 剩余限制：renew 未校验租约存活（计划仅要求匹配 token；fencing 安全，评审确认）；连接为逐调用建立（池化归 O01）；retry_after 列预留 I05。

### 2026-09-11 I02 业务revision、outbox与授权版本（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：b9fe468（I01 台账提交）。
- 修改文件：migrations/versioned/000096_semantic_control.{up,down}.sql、migrations/sqlite/000017_semantic_control.{up,down}.sql、internal/types/semantic_control.go、internal/application/repository/{semantic_outbox.go,semantic_outbox_test.go}、internal/database/semantic_migration_test.go、go.mod/go.sum（tidy：lib/pq+mattn 转直接依赖）、本台账、02 计划勾选。
- 迁移号核查：实施前实测 PG 最高 000095、SQLite 最高 000016 → 采纳计划建议 PG96/SQLite17，未改写任何已执行迁移。
- RED：`go test ./internal/application/repository -run TestSemanticMutation -count=1` 编译失败（NewSemanticControlRepository/SemanticMutation 未定义）。
- GREEN：`go test ./internal/application/repository -run 'TestSemantic|TestRevision|TestDelete|TestNonDelete|TestBump|TestOutbox|TestConcurrent' -count=1 -race` ok（12 项）；`go test ./internal/database -run TestSemanticMigration -count=1` ok（PG 3.5s + SQLite）；go vet/build/tidy -diff 全净。
- 实测验收：回滚零逃逸（outbox 0 行+revision 回滚后重放成功，计划核心片段逐字）；CAS 过期 expected→ErrSemanticRevisionConflict（三种形态）；并发同文档恰一胜（-race ×10）；删除同事务写 deny+epoch 提升（普通更新不动 epoch）；并发首次 epoch 提升均成功（单语句 upsert，断言 epoch==2）；BumpSemanticEpochTx 随调用方事务回滚；outbox 原子领取（条件 UPDATE+RowsAffected）、租约持有/过期回收/退避/终态确认；迁移合同 up/down/up+双唯一键拒绝+事务失败零孤立事件（真实 PG 全 96 迁移链 + SQLite）。
- 环境：业务测试库在隔离容器 semantica-i01-pg（127.0.0.1:15432）内另建 semantic_business_test，并设 `ALTER DATABASE ... SET app.skip_embedding='true'` 走无扩展路径（vector/pg_search 迁移均有该官方开关；曾试 pgvector 镜像仍缺 pg_search，弃用）；DSN 可经 SEMANTIC_TEST_BUSINESS_PG_DSN 覆盖；dropAllPublicTables 仅接受库名含 test 的 DSN（url 解析校验）；缺环境 t.Fatalf 不 skip。
- 评审修复：规格 PASS（10 项，6 MINOR）+ 质量 PASS（无 BLOCKER，13 MINOR：1/2/3/4/5/6/7/8/9/10/12 已修复——唯一冲突检测收紧、原子领取、同事务 Bump 变体、epoch 单语句 upsert、payload_hash 归一化语义注记、领取计次、drop 守卫、I05 receipts 注记、真实重复插入断言、go mod tidy、本记录随提交；11/13 PG 路径仓储测试与时间戳来源统一留作后续；复审确认全部修复并 -race 复跑）。
- 计划偏差记录：BumpSemanticEpoch 同时提供无 tx 版本（独立权限事件）与 Tx 版本（A01 同事务接线，计划原文要求）；receipts 表按 I05 契约注记将由其迁移扩展 (attempt, operation_id)。
- 提交 SHA：3619e12（feat(semantic): i02 业务revision、outbox与授权版本）。
- 剩余限制：PG 路径仓储行为仅经迁移合同覆盖（库级 SQL 与 sqlite 驱动同构；PG 专属仓储测试留后续）；outbox 领取谓词部分索引与确认事件清理归 I03；backend_states 列为 W03/I03 预留。

### 2026-09-11 A01 可信AccessScope与权限变更屏障（implemented）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：22f2ca1（I02 台账提交）。
- 修改文件：internal/application/service/{semantic_scope.go,semantic_scope_test.go}、internal/handler/{semantic_internal.go,semantic_internal_test.go}、internal/application/repository/{semantic_outbox.go(读API+3个Bump助手),tenant_member.go,kbshare.go,organization.go(同事务epoch接线)}、internal/config/config.go(ScopeIssuingKey≥32/ResolveToken/ScopeTTL≤15m)、internal/container/container.go、internal/router/{router.go,routes_infra.go}、internal/database/migration_sqlite_versioned_schema_test.go(迁移计数13→17修正)、docs/superpowers/plans/semantica/acl-write-inventory.md、本台账、03 计划勾选（步骤3/6 按实留空）。
- RED（服务核心）：`go test ./internal/application/service -run TestSemanticScope -count=1` 编译失败（SemanticScopeService 未定义）；RED（内部入口）：`go test ./internal/handler -run TestSemanticInternal -count=1`（RegisterSemanticInternalRoutes 未定义）；评审修复 RED×3：组织表名错误（knowledge_base_shares→kb_shares，TestOrgMemberRemovalBumpsSharedKBEpochs 先失败）；快照漂移（TestSemanticScopeDriftWithoutEpochStillInvalidates 先失败）；短密钥漏洞（TestSemanticScopeShortKeyDisablesService/EmptyKeyForgedRefsNeverVerify 先失败——nil-and-continue 使空公钥伪造通过，评审员实证）。
- GREEN：service 9 项（计划核心逐字：epoch 变更→ErrSemanticScopeChanged；另过期时钟/伪造/篡改字段/Resolve 快照（撤后旧 scope 拒绝+新 scope 排除+墓碑 revision）/保留旧版/无 epoch 漂移仍失效/短密钥禁用/空钥伪造拒绝）；handler 4 项（缺/错身份 401、快照 200、变更 409、伪造 403）；repository 全量含组织成员移除 epoch 回归；database 迁移测试修复后全绿；build/vet 净。
- ACL 接线（同事务 epoch）：tenant_member UpdateRole/SoftDelete/Demote/RemoveOwner（RowsAffected 门控）；kbshare Update/Delete（源租户+KB）；organization RemoveTenantMember/UpdateTenantMemberRole（组织全部被分享 KB）；文档语义删除经 I02 WithSemanticMutation（deny+epoch 同事务，计划核心测试即经此路径）。
- 内部入口：/api/v1/internal/semantic/scope/resolve，常量时间令牌（X-Semantic-Internal-Token），无令牌不挂载（容器双重 fail-closed：密钥<32B→nil 服务→nil handler→无路由）；仅返回授权快照不含知识内容；401/403/409 语义。
- review：规格 PASS（9 项；条件：①组织删除补入清单——已补为第三条待接线；②Issue 调用方必须经 resolveKBReadTenant 取 owner tenant——已记入清单已知缺口，Q01 前强制；③敏感替换隐藏旧版建模归 I05——已记）；代码质量终审 PASS（原 BLOCKER kb_shares 表名+评审修复中自引入的短密钥 BLOCKER 均实证关闭；迁移计数预存失败 13→17 修复；遗留 follow-up：按 subject 快照+owner tenant 校验（Q01 前强制）、testutil 合并、少量边界测试）。
- 提交 SHA：d0ffa3f（feat(semantic): a01 可信AccessScope与权限变更屏障）。
- 剩余限制（verified 前必须完成）：三条 ACL 接线——knowledge_transfer 克隆（源+目标 KB）、temporary_document 到期清理、organization DeleteOrganization（撤分享前）；Issue 尚无生产调用方（Q04 接线时必须走 resolveKBReadTenant）；快照当前 KB 级非 subject 级（per-subject 过滤归 A02/Q01 接线）；内部入口部署形态（网络隔离）归 O01。

### 2026-09-11 A01 收尾：三条待接线（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线：d4f250a（A01 首段台账提交 d0ffa3f 之后）。
- 修改文件：internal/application/repository/{kbshare.go(DeleteByOrganizationID tx+bump),semantic_outbox.go(BumpKBSemanticEpochs 服务层方法),semantic_outbox_test.go}、internal/application/service/{knowledge.go(semanticEpochs 字段+构造参数),semantic_scope.go(SemanticEpochBumper 接口),semantic_scope_test.go(TestTransferBumps+短钥/空钥两测),knowledge_transfer.go(bumpTransferEpochs×2 调用点)}、internal/container/container.go(dig 适配器)、acl-write-inventory.md（11 接线+盘点修正+无漏接结论）、本台账、03 计划勾选（步骤 3/6 补勾）。
- 接线（补齐三条）：
  1. 组织删除：kbshare.DeleteByOrganizationID 同事务**先 bump 后删**（顺序关键——bump 读取被删行）；RED-first（TestOrgShareStrippingBumpsSharedKBEpochs 在旧代码失败 epoch 0≠1，评审员 /tmp 变异复现：先删后 bump 同样失败）。
  2. 文档克隆：executeKnowledgeClone 删除批后+克隆批后 bump 源+目标 KB（SemanticEpochBumper 接口经 dig 适配注入 knowledgeService；可恢复多文档流程无单一事务，取自有短事务；nil 安全）。诚实记录：TestTransferBumpsSourceAndTargetEpochs 为接线后补写（确认性测试），其判别力经评审员 no-op 变异验证（失败 0≠1）；两调用点位置经代码审读确认（端到端克隆流测试列为可选后续）。
  3. 临时文档到期：**盘点修正而非接线**——TemporaryDocument 无 KnowledgeBaseID（会话级，types/temporary_document.go:24-48），从不进入知识库/语义索引，到期清理不影响语义可见性；清单结论改为"11 条已接线+1 条豁免+3 条归属后续"。
- 评审（完成评审，聚焦增量）：PASS 6/6（组织删除顺序变异验证；transfer 判别力变异验证；临时文档豁免对照类型定义实证；清单完备；诚实测试注记成立；全电池绿）。遗留 nits：计数 10→11 与行号 24-38→24-48 已当场修复。
- GREEN：repository 全量 ok；service 12 项（含 TestTransferBumps）；handler/database 不变绿；build/vet 净。
- 提交 SHA：598f3be（feat(semantic): a01 收尾ACL接线与盘点修正）。
- 剩余限制：Issue 生产调用方必须经 resolveKBReadTenant 取 owner tenant（Q04 接线强制，已在清单）；快照为 KB 级非 subject 级（A02/Q01 接线时细化）；端到端克隆流 bump 调用点测试为可选后续；内部入口网络隔离归 O01。

### 2026-09-11 A03 模型代理、原始用量与预算（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：ce425d2（A01 收尾台账提交）。
- 修改文件：migrations/{versioned/000097_semantic_invocations.{up,down}.sql,sqlite/000018_semantic_invocations.{up,down}.sql}、internal/types/semantic_model.go、internal/application/service/{semantic_model.go,semantic_model_test.go}、internal/handler/{semantic_model_internal.go,semantic_model_gateway_server_test.go}、internal/infrastructure/semantic/model_provider.go、internal/config/config.go(ModelProvider/BaseURL/APIKey[json:"-"]/ModelName)、internal/container/container.go、internal/router/router.go、internal/database/{migration_sqlite_versioned_schema_test.go(18),semantic_migration_test.go(steps -2)}、semantic/semantic_service/model_gateway.py、semantic/tests/{test_model_gateway.py,conftest.py(go_model_server/model_gateway fixtures)}、本台账、03 计划勾选。
- 迁移号：实施前实测 PG 最高 000096、SQLite 最高 000017 → 采用 000097/000018；合同测试 up/(-2)/up 双方言；迁移计数 17→18。
- RED：`go test ./internal/application/service -run TestSemanticModel -count=1`（类型未定义）与 `uv run --project semantic python -m pytest semantic/tests/test_model_gateway.py -q`（模块缺失）。
- 评审修复 RED 四批：①provider 失败被吞成 200 假成功（502 死代码）→TestSemanticModelProviderFailureSurfacesNotFakeSuccess 先失败；②PG 并发预算超限（10×40 vs 100 全部准入）→ 原子条件 UPDATE + TestSemanticModelConcurrentBudgetNeverOvershoots（评审员 overlay 探针实证修复前后）；③死上下文台账写丢失（真实 deadline 场景行滞留 in_flight→500）→ WithoutCancel+10s + TestSemanticModelRealDeadlineRecordsUnknown（provider 阻塞至真实 ctx 到期，先失败）；④预算拒绝滞留 in_flight → 删除可重试 + TestSemanticModelBudgetRefusalDoesNotStrandInFlight（先失败）。
- GREEN：`go test ./internal/application/service -run TestSemanticModel -count=1` 11 passed（含 -race -count=3 稳定）；`uv run --project semantic python -m pytest semantic/tests/test_model_gateway.py -q` 3 passed（真实编译 Go HTTP：go test -c 服务 + 受控 provider + 持久台账计数；计划核心断言逐字通过 first==second && count==1）；handler/database/repository 全量 ok；build/vet 净。
- 关键语义：claim 幂等（new/completed/in_flight/unknown；failed/reconciled 同 ID 阻断→409）；同 ID 重试返回已存结果（provider 恰一次）；预算准入先于 provider（拒绝时 0 调用；HTTP 402）；预占/实际/unknown 分离（finalize 退款、reconciling 保留对账、Reconcile 按观察用量结算并退款）；重试新 ID 关联 parent（无聚合重复计费）；受控入口唯一（OpenAI-compatible 适配器，凭据仅 Go config、json:"-" 不落盘不落日志；Python 仅经内部 HTTP + 服务身份令牌）。
- review：规格首轮 FAIL（BLOCKER：provider 失败假成功）→ 修复+终审 PASS（含终审新 MINOR：Reconcile 幂等性——退款已加状态守卫建议，记录为后续）；质量首轮 FAIL（3 BLOCKER：PG 并发超限/死上下文台账/滞留 in_flight，均 overlay 探针实证）→ 三项修复后 PASS（评审员独立复跑：PG ok=2/spent=80、-race ×3、Python 3/3）。遗留 MINOR follow-up：claim 并发 PK 冲突→409、finalize 超支记账、messages JSON 编解码、Retry 父态守卫+返回体、deadline 头解析加固、provider 响应限长+缺用量→unknown、502 不泄露 base URL、Python 客户端 deadline 头/异常分类、路由前缀对齐、PG 服务层测试、BYOK 用量测试。
- 提交 SHA：25dedb8（feat(semantic): a03 模型代理、原始用量与预算）。
- 剩余限制：无真实模型凭据——真实模型调用/上游无旁路直连证明保持未通过（计划允许：记录阻断继续他项）；预算为任务本地 semantic_budgets（未接商业结算，未伪称）；模型能力凭据为静态受控入口（短期按操作凭据归 Q03/A02 接线强化）。

### 2026-09-11 I03 有来源的构图与generation原子发布（verified）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：62fc527（A03 台账提交）。
- 修改文件：semantic/migrations/002_generations.sql（generations/active_generations/read_leases，幂等）、semantic/semantic_service/indexing/{__init__,manifest,builder,store,publisher}.py、semantic/tests/{test_generation_publish.py,conftest.py(index_store/service_pg_dsn/complete_manifest/清洁 fixture)}、本台账、02 计划勾选。
- RED：`uv run --project semantic python -m pytest semantic/tests/test_generation_publish.py -q` 收集错误（semantic_service.indexing 不存在）。
- 评审修复 RED 两批：①BLOCKER——crash 于 staged→publishing 后操作永久滞留（claim/recoverable 仅回收 accepted/running）→ 先加 test_publishing/staged_operation_with_expired_lease_is_reclaimable（RED：返回 None）→ claim/recoverable 候选集扩至 staged/publishing（fencing token 递增，旧 worker 写入必被拒）；②save_manifest 同 generation 异载荷静默丢弃 → test_save_manifest_conflict_on_divergent_payload（RED）→ RETURNING+归一化 JSON 比对 raise；③CAS 失败语义修正 failed→superseded（规格 §6 line 92；测试先行更新断言）。终审遗留 MINOR A–E 亦已折叠：陈旧重复测试删除、真实双线程同 base CAS 竞争测试（单胜实证）、swap 期间 pin 旧代可读测试、builder 三项测试（scope 不匹配/继承+新产物/空产物拒绝）、renew() 冗余导入清理。
- GREEN：`uv run --project semantic python -m pytest semantic/tests/test_generation_publish.py -q` 16 passed；全量 `semantic/tests/` 101 passed 退出码 0（真实服务 PG：semantica-v02-i01 容器 127.0.0.1:15432/semantic_test）。
- 实测验收：不完整 manifest 永不 active（计划核心断言逐字）；未持久化 complete 先行拒绝；CAS——首发布 INSERT ON CONFLICT 恰一胜 + 同 base 条件 UPDATE 真线程恰一胜（评审员双连接探针独立复现）；查询只见完整旧版或完整新版（active 指针仅指向已持久化 complete）；read lease pin 当前 active/到期感知 holds/release/renew（过期不可复活）；manifest 重启等值恢复（新 store 实例）；publisher 三段式（fenced staged→publishing→CAS→succeeded/superseded，LostLeaseError 类型化，三处返回值检查）；builder scope 校验/未变文档继承既有不可变产物/变更文档新产物/非删除空产物拒绝。
- 计划偏差记录：publisher 为四段事务（fenced 转换/manifest 持久化/CAS/终态），计划伪代码的单事务 assert_live_operation+assert_not_deleted+assert_artifacts_verified+CAS 归 I04/I05 接线（store lease_token 占位注释已注明）；抽取适配器（Neo4j/向量）为 Protocol 接缝，实际接线归 I03 后续+Q01（builder 文档已如实限定）；claim 候选集扩展使过期 staged/publishing 可被回收并经 fencing 防旧 worker 复活（对 reclaim 的 publishing 操作须先对账 active 指针——publisher 文档已述）。
- review：规格 PASS（11 项；5 项 MINOR 全部折叠进终版：fencing 事务差距记录/续期已补/builder 已测/docstring 已改写/台账本记录）；质量首轮 FAIL（BLOCKER 滞留窗口）→ 修复+终审 PASS（BLOCKER 回收测试判别力实证、CAS 真线程复验、fencing 分析确认无永久滞留与旧 worker 安全）。
- 提交 SHA：6585ecc（feat(semantic): i03 有来源的构图与generation原子发布）。
- 剩余限制：图/向量实际抽取适配未接线（Q01/A02 消费 manifest 闭包）；GC 未实现（read lease 保护 API 就绪，I04 落地清理）；operations.py claim/recoverable 扩展影响 I01 行为（回收更多状态——回归 I01 全量测试确认无破坏）。

### 2026-09-11 I04 删除屏障、支持撤销和清理receipt（verified，步骤 6 延后）

- 工作区：.worktrees/semantica（分支 codex/semantica）；基线 SHA：3718a29（I03 回收扩展提交）。
- 修改文件：semantic/migrations/003_deletion_receipts.sql、semantic/semantic_service/indexing/{deletion.py,gc.py}、semantic/tests/{test_delete_races.py,conftest.py(clean_generations 容缺失表+deletion 清理)}、本台账、02 计划勾选（步骤 6 保持未勾）。
- RED：`uv run --project semantic python -m pytest semantic/tests/test_delete_races.py -q`（fixture NameError/收集失败→业务断言失败）。
- 评审修复 RED 三批（全部评审员探针复现→先测后修）：①支持行 PK 漏 revision（同文档高版本支持被 ON CONFLICT 静默丢弃，删除旧版误杀活事实）→ PK 加 support_revision + test_reapplied_higher_revision_restores_visibility；②合取前提语义（A∧B→C 杀 A 后 C 存活——NOT EXISTS any-visible 错）→ EXISTS-a-dead-premise + test_conjunctive_rule_dies_when_any_premise_dies（推导断言由第三方文档支持以隔离前提路径）；③GC 清墓碑复活（仅按时间窗清除，未重建的 active generation 仍供已删行）→ 清除条件加"无任何持久化 manifest 引用该 (doc,revision<=墓碑)"（回滚安全：覆盖全部持久化 generation）+ test_tombstone_sweep_requires_rebuilt_generation（stale manifest 在→0 清除；generation 行删→1 清除）。终审新 MINOR 亦折叠：死后插入的推导断言不再 born-visible（insert 同事务跑 fixpoint + test_derived_inserted_after_premise_death_is_born_invisible）。
- 其余折叠：visible_assertion 单 SQL 折叠墓碑检查（去 N+1 连接）；重放感知插入（墓碑内支持行存 INVISIBLE——重放事件不可复活已删数据，test_replayed_support_after_delete_stores_invisible）；GCConfig 强制配置（无默认值）；gc.py 文档不再宣称未实现的产物删除；protect() 负例+租约分支测试（pin 后移指针：租约保护→释放即不保护）；实体自有来源删除不牵连他处事实；误导命名测试改名（late apply admitted-but-denied）；_scope 私有属性与死导入清除。
- GREEN：`uv run --project semantic python -m pytest semantic/tests/test_delete_races.py -q` 17 passed；全量 semantic/tests/ 118 passed 退出码 0（真实服务 PG）。
- 实测验收：核心断言逐字（删 d1 留 shared-fact、删 d2 去 shared-fact+derived-fact）；墓碑单调（高版本拒低版本、迟到低墓碑不降栏、重建高版本不拒）；多来源（单源删留事实、末源删失效、同名实体他事实存活——含实体自有来源损失场景）；合取前提递归失效（评审员加测深 2 链收敛、多支持前提部分删不过度失效）；receipt 分存储独立推进（半失败 pending 可独立重试、四 store 齐 completed_at、backup 恒 retention_pending 不伪装、mark 幂等 True→False）；GC 窗口 floor=max(retention,replay)且清单闭包守卫（跨租户 manifest 不钉他域墓碑——评审员探针）；重放写入不复活。
- 计划偏差/延后记录（I05 接线）：①步骤 6 恢复模式（默认不 ready+从 Go 重放 deny/epoch 再开放查询）未实现——checkbox 保持未勾，归 I05 服务编排；②删除操作路由（apply 产生 accepted 操作可被索引 worker 领取；须打标并驱动终态）归 I05；③I03 publisher 的 assert_not_deleted（发布时墓碑拒绝）归 I05；④I01 预存 bug：operations.py:362 load_request 对已解码 JSONB dict 再 json.loads 会崩（评审员探针发现，worker 管道端到端未跑过）归 I05 首要修复。
- review：规格 PASS（11 项；条件：台账+延后记录+共享 PG 并发写风险——本记录即为；推荐项全折叠：GCConfig 强制/protect 负例+sweep 测试/实体测试加强/…）；质量首轮 FAIL（3 BLOCKER：PK 缺列/合取语义/墓碑清除复活）→ 修复后终审 PASS（三项以原复现场景独立复验+四场景边缘探针；两项新 MINOR：born-visible 已当场折叠修复，共享 DB 修复为带外操作——提交信息注明"已跑过旧 003 的开发库需 DROP semantic.assertions 重建"）；注意共享隔离 PG 有并发写风险（评审期间发现外部 kb-race 命名空间行，运行套件时独占）。
- 提交 SHA：（本记录与代码同批提交后补记）

## 当前边界

- V01–C03、I01–I04、A01、A03 verified；后续 12 个任务未开始。
- V01精确版本已冻结（semantica 0.6.8）；真实模型证据须在后续任务补齐，不是已经通过的前提。
- V02 结论边界：持久图桥接/授权子图重建/注册规则推导已验证；模型推断 unverified（无凭据，未调用）；向量检索路径未验证。
- V03 结论边界：semantica 模式检索质量/延迟为受控语料实测；native 对照与模型用量门槛未测（阻断记录见上）；上线门禁 approved=false 待用户确认。
- 未创建GitHub Issue或外部发布；没有分配虚构Issue编号。
