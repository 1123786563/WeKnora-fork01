# Semantica 实施台账

状态：V01 已完成并验证，其余 23 个任务未开始。总计划：[实施入口](../2026-09-11-semantica-implementation.md)。

状态值 pending / in_progress / blocked / implemented / verified。每项验证记录必须包含commit SHA、精确命令、退出码、环境、产物路径、失败/限制；无真实证据不标记verified。执行前记录实际基线与已有脏文件。

| 任务 | 名称 | 依赖 | 状态 | 证据/提交/阻断原因 |
|---|---|---|---|---|
| V01 | 冻结版本与最小安装契约 | 无 | verified | semantica==0.6.8（+graph-neo4j extra）冻结于上游 commit f73f599a；Python 3.12.13；见运行记录 2026-09-11 V01 与 capability-evidence.json |
| V02 | 验证持久图桥接和两类推理 | V01 | pending | 尚未执行 |
| V03 | 中文质量与上线阈值评估基线 | V02 | pending | 尚未执行 |
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

## 当前边界

- V01 已完成（verified）；V02/V03 及后续 22 个任务未开始。
- V01精确版本已冻结（semantica 0.6.8）；V03数值门槛、真实模型证据须在后续任务补齐，不是已经通过的前提。
- 未创建GitHub Issue或外部发布；没有分配虚构Issue编号。
