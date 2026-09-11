# Semantica 实施台账

状态：V01–V03 已完成并验证，其余 21 个任务未开始。总计划：[实施入口](../2026-09-11-semantica-implementation.md)。

状态值 pending / in_progress / blocked / implemented / verified。每项验证记录必须包含commit SHA、精确命令、退出码、环境、产物路径、失败/限制；无真实证据不标记verified。执行前记录实际基线与已有脏文件。

| 任务 | 名称 | 依赖 | 状态 | 证据/提交/阻断原因 |
|---|---|---|---|---|
| V01 | 冻结版本与最小安装契约 | 无 | verified | semantica==0.6.8（+graph-neo4j extra）冻结于上游 commit f73f599a；Python 3.12.13；见运行记录 2026-09-11 V01 与 capability-evidence.json |
| V02 | 验证持久图桥接和两类推理 | V01 | verified | Neo4j 5.26 真实持久化+进程重启溯源通过；规则正/负/中文推导通过；模型推断无受批准凭据保持 unverified（未发起调用）；见运行记录 2026-09-11 V02 与 bridge-evidence.md |
| V03 | 中文质量与上线阈值评估基线 | V02 | verified | semantica 模式 30 题实测（正确率 0.767、泄漏 0、recall 0.925）；native 对照阻断于隔离测试部署+模型凭据，approved=false；见运行记录 2026-09-11 V03 与 evaluation-baseline.md |
| C01 | 版本化协议和跨语言领域类型 | V01 | pending | 尚未执行 |
| C02 | 认证服务骨架和Go客户端 | C01 | pending | 尚未执行 |
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

## 当前边界

- V01–V03 已完成（verified）；后续 21 个任务未开始。
- V01精确版本已冻结（semantica 0.6.8）；真实模型证据须在后续任务补齐，不是已经通过的前提。
- V02 结论边界：持久图桥接/授权子图重建/注册规则推导已验证；模型推断 unverified（无凭据，未调用）；向量检索路径未验证。
- V03 结论边界：semantica 模式检索质量/延迟为受控语料实测；native 对照与模型用量门槛未测（阻断记录见上）；上线门禁 approved=false 待用户确认。
- 未创建GitHub Issue或外部发布；没有分配虚构Issue编号。
