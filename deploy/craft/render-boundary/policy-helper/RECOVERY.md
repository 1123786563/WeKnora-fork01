# T14 policy-helper 源码恢复记录（2026-10-02）

来源：本地镜像层提取 + 检查点拷贝，逐文件 SHA-256 对照档案链。

## 镜像

| 镜像 | digest | 用途 |
| --- | --- | --- |
| `craft-t14-policy-helper:2026-09-24` | `sha256:c0cfa88b4db1940b7cbc978f0b43e713c22ce4af0ebcdeb2ec7db9a14bf1f2bb` | helper.py 提取（`docker create` + `docker cp /policy-helper.py`，只读） |
| `craft-t14-current-reviewed:2026-09-24` | `7e3c24469815` | 排查性检查（无 policy-helper 构建上下文） |
| base | `python:3.12-alpine@sha256:4c47124a8391cb7a9f571164147d154777cf012a4ece5f86097130d7a4478111` | build.sh 锚定（取自镜像 label + inspect） |

## 恢复文件与哈希链判词

| 文件 | SHA-256 | 链上预期 | 判词 |
| --- | --- | --- | --- |
| `helper.py` | `c8d30121927350e3cea5fd3b5823ca2308f1b4f550af1d730d29ac7e92fd87c2` | fix1-final（fix1-report §1535、fix2-plan §7、ledger §42、live-test-run.json source_checkpoint 四处一致） | **MATCH** |
| `tests/test_controller_integration.py` | `9c95492af5f2f72c78c1f95f742a82aed5d1e03c3269561be97007586a3c0bf8` | fix3 after（checkpoints/2026-09-29-fix3/hashes.txt + live-test-run.json） | **MATCH** |

版本归属核实：sockdiag-fix3 计划明确只改 `tests/test_controller_integration.py`，helper.py 保持 fix1 终态——与镜像内提取件哈希一致，无冲突。

注：恢复派工单所写 helper 哈希 `c8d30121927350e3cea5fd3b5823ca230b1f4f55…` 为笔误；四处权威文档一致为 `c8d30121927350e3cea5fd3b5823ca2308f1b4f550af1d730d29ac7e92fd87c2`，提取件与后者吻合。

## 未恢复件（不可恢复，留档哈希）

原目录（fix1-report §80 文件清单）还有三件，无任何可恢复载体（镜像层/BuildKit 缓存/worktree/SDD 检查点均无）：

| 原文件 | 档案 SHA-256（fix1-report §88） | 状态 |
| --- | --- | --- |
| `controller.py` | `9e6840357d22e4f2da1d4f79d52a01f5953991766fe9a30b9433973902f0a439` | 2026-10-02 Task 2 以重写件替代（`4dcad2b1…`）：宿主侧编排器，满足恢复测试文件全部 API；活体验收复跑七判据全过（`docs/testing/craft/t14/2026-10-02-live-acceptance-rerun/`） |
| `barrier_adapter.py` | `598502bb9015d440a7696a399b486b33111a1982487b8798cdb3d2763004b1ec` | 2026-10-02 Task 2 以重写件替代（`7c9969ba…`）：发现脚本 + 七字段 flow join |
| `Dockerfile` | `d9a1e8f450f831ba6cd364014c934beb62069b08b3928f89f89fc22d74804deb` | 字节不可推导（128 种 history 兼容格式组合穷举无一命中）。以 `build.sh` 代替：按 `docker history` 四层 + 精确 base digest 复现镜像 |

## 离线检查

- `python3 -m py_compile helper.py tests/test_controller_integration.py` — 通过
- `tests/ $ python3 -m unittest test_controller_integration.PolicyTargetCounterUnitTests` — 通过（纯单测，仅加载 helper.py）
- 其余测试类（StopFailure/BoundedEvidence/Invocation/Integration）依赖缺失的 `controller.py` 或 Docker，本轮未跑
