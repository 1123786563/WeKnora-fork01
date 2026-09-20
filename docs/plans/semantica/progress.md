# Semantica 实施台账

状态：V01、V02、C01 已按各自证据层和验收门槛标记为 verified；C02 已实施待 review；其余 20 个正式任务仍为 pending。先导 probe 另行留证，不替代 V03 或生产验收。总计划：[实施入口](../2026-09-11-semantica-implementation.md)，校准入口：[2026-09-20 rebaseline](../2026-09-20-semantica-rebaseline.md)。ADR-0002 与 rebaseline 优先于旧计划中的未验证建议。

状态值 pending / in_progress / blocked / implemented / verified。每项验证记录必须包含commit SHA、精确命令、退出码、环境、产物路径、失败/限制及证据层（static / actual-runtime / controlled-provider / live-model / real-storage）；无真实对应层证据不标记verified，mock 不得替代实际运行或存储。执行前记录实际基线与已有脏文件。

| 任务 | 名称 | 依赖 | 状态 | 证据/提交/阻断原因 |
|---|---|---|---|---|
| V01 | 冻结版本与最小安装契约 | 无 | verified | 2026-09-20: initial code/evidence commit `4f056bfb5097e3964fc5a0659e1386c386443805`, review-fix commit `faa23e6dc127a17f0f57af1e5e9ab581981ab192`; baseline `5382604f1538a7cef87a04c8a74e5f82f0ecda39`, Task-2 base `f632cb4a7e1eb132af3ebc6d6869549da3aa50ec`; initial working tree was clean except the untracked execution plan `docs/plans/2026-09-20-semantica-v01-recovery-and-install-contract.md`; initial RED `uv run --project semantic/experiments python -m pytest semantic/experiments/test_import_contract.py -q` exit 1 (no installed Semantica/verifier); final GREEN same command exit 0, 6 passed in 4.55s on CPython 3.12.12/macOS arm64; clean locked replay `uv run --locked --isolated --project semantic/experiments python semantic/experiments/verify_version.py --output docs/plans/semantica/evidence/2026-09-20/v01-installed-contract.json` exit 0. Lock SHA-256 `c643ce123490c93f56ed95e9d6501bbd90daf8b2cdc74b183067dccd63864c54`; evidence [v01-installed-contract.json](evidence/2026-09-20/v01-installed-contract.json), `actual-runtime`; verifier found all 7 required imports and exact runtime signatures, checks the dedicated lock stanza/wheel hash and distribution metadata/import origin, and only permits repository evidence paths. Independent review found the initial signature/provenance/path gaps; all were fixed with RED→GREEN tests. Limits: no provider/model, persistent storage, Go authorization, external service, quality, performance, budget, or production test. |
| V02 | 验证持久图桥接和两类推理 | V01 | verified | 2026-09-20 dedicated local `real-storage` evidence: Semantica `0.6.8`, V01 lock `c643ce123490c93f56ed95e9d6501bbd90daf8b2cdc74b183067dccd63864c54`, V02 lock `7cea23660879d96120a8936b44d9c23d4aaaf036b31c9a4e79812d31a4efe709`; initial writer/reader and post-container-restart reader-only recovery retain `a-d1/a-d2`, `e-d1/e-d2` and provenance while backend scope query excludes `a-foreign`. Authoritative evidence [v02-dedicated-reverified.json](evidence/2026-09-20/v02-dedicated-reverified.json), report [bridge-evidence.md](bridge-evidence.md); older `v02-dedicated.json` is superseded for restart proof. Shared-isolated Community candidate is explicitly unavailable: [v02-shared-isolated-unavailable.md](evidence/2026-09-20/v02-shared-isolated-unavailable.md); ADR fallback selects dedicated solely as V02 capability topology. Commands exited 0: V02 pytest 20 passed; dedicated capture and container restart/reader-only recovery. No provider/model, Go authorization, ACL/revocation, quality, performance, budget, production topology, or promotion claim. |
| V03 | 中文质量与上线阈值评估基线 | V02 | pending | 尚未执行 |
| C01 | 版本化协议和跨语言领域类型 | V01 | verified | 2026-09-21: final reviewed implementation head `a1721ce03a4509677aaaec55cf4d8a9b0c788784`; separate `weknora.semantic.v1` schema/generator, seven RPCs, Go/Python bidirectional mappings for RPC DTOs and component leaves, strict scope/mode/delete/numeric validation, and optional-field round trips. Fresh commands exit 0: `semantic/scripts/generate_proto.sh --check`; `go test ./semantic/proto ./internal/types -run 'TestSemantic' -count=1`; `uv run --locked --project semantic python -m pytest semantic/tests -q` (32 passed). Final independent review `review_c01_reopen_final` found no Critical/Important findings. Scope: no DocReader extension, RPC server/client implementation, model/provider call, storage implementation, or authority issuance. Owned Assertion/derivation and IndexManifest/ReadLease models remain deferred to C03/I03; opaque reference fields remain in the C01 wire DTOs. |
| C02 | 认证服务骨架和Go客户端 | C01 | implemented | 2026-09-21: commit `4b4b2b198d8bf1791721569bb20758a3af21fddf`; service is opt-in, requires explicit TLS files plus token/audience, rejects anonymous RPCs as UNAUTHENTICATED, and keeps SemanticService readiness NOT_SERVING while methods remain UNIMPLEMENTED. Go client is TLS-only, sends per-RPC identity/audience, propagates trace metadata and context deadlines/cancellation, maps gRPC status codes, and closes through ResourceCleaner; disabled config returns nil and existing container tests pass. RED: `uv run --project semantic python -m pytest semantic/tests/test_rpc_auth.py -q` failed on missing config module; GREEN Python RPC auth/health suite 3 passed. Commands exit 0: `go test ./internal/infrastructure/semantic ./internal/types/interfaces ./internal/config ./internal/container`; `uv run --locked --project semantic python -m pytest semantic/tests -q` (35 passed). Test-only ephemeral TLS certificate used; no service/container or production endpoint started. Independent review pending. |
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

## 先导 probe 记录

- 2026-09-20：[README](evidence/2026-09-20/README.md)、[REPORT](evidence/2026-09-20/REPORT.md) 与 [runtime-312.json](evidence/2026-09-20/runtime-312.json) 是有限可复现 probe 证据；不改变 24 个正式任务的 pending 状态，也不构成 V01/V02/V03 验收。

## 当前边界

- 本台账不以正在进行的 probe 代替 V01–V03、实际存储或生产验收；实验输出写入对应证据产物后再按层级判断。
- V01精确版本、V03数值门槛、真实模型证据须在执行阶段补齐；这些是明确任务产物，不是已经通过的前提。
- 未创建GitHub Issue或外部发布；没有分配虚构Issue编号。
