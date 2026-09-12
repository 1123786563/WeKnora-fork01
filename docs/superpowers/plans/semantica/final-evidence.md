# Semantica 最终四层证据汇总（O03）

分支：codex/semantica @ 53cad94（最新实质提交）（各任务提交见 progress.md 逐条 SHA）

## 四层验证状态（如实）

| 任务 | 代码实现 | 单元测试 | 真实存储集成 | 浏览器流程 | 真实模型 |
|---|---|---|---|---|---|
| V01 | ✅ 0.6.8 冻结 | ✅ | n/a（实验） | n/a | ❌ 无凭据 |
| V02 | ✅ | ✅ | ✅ 真实 Neo4j 5.26 | n/a | ❌ |
| V03 | ✅ | ✅ | ✅ 受控语料 | n/a | ❌ |
| C01–C03 | ✅ | ✅ | n/a（proto/TLS gRPC/单测合同） | n/a | n/a |
| I01–I05 | ✅ | ✅ | ✅ 真实 PG | n/a | n/a |
| A01 | ✅ | ✅ | ✅ | n/a | n/a |
| A02 | ✅ | ✅ | ✅ 真实 PG | n/a | n/a |
| A03 | ✅ | ✅ | ✅ 受控 provider | n/a | ❌ 无凭据（受控桩全链路） |
| Q01 | ✅ | ✅ | ✅ 真实 PG+真实 gRPC | n/a | n/a |
| Q02 | ✅ | ✅ | ✅ | n/a | n/a |
| Q03 | ✅ | ✅ | ✅ | n/a | ❌（模型模式=受控桩验证） |
| Q04 | ✅ | ✅ -race | ✅ | n/a | n/a |
| W01 | ✅ | ✅ TS+Go | n/a | n/a | n/a |
| W02 | ✅ | ✅ TS | n/a | ❌ 阻断（无服务栈） | n/a |
| W03 | ✅ | ✅ -race | ✅ PG 实证 CAS | n/a | n/a |
| O01 | ✅ | ✅ | ✅ 活体探针 | n/a | n/a |
| O02 | ✅ | ✅ | ✅ 真实 PG 8 场景 | n/a | n/a |
| O03 | ✅ 门禁 7 测 | ✅ | ✅ | ❌（同 W02 阻断） | ❌ |

## 已知限制（上线前必须知悉）

1. **真实模型验证未通过**（无凭据）：V02/V03 模型推断、A03 真实调用、Q03 模型模式端到端——门禁 `missing_live_model` **必然阻断 release**（正确行为）。
2. **浏览器 E2E 未执行**（无服务栈）：W02/O03 的 Playwright 场景就绪（SEMANTIC_E2E_READY=1 门控）——`missing_browser` 阻断。
3. Q04 消化项：chat/agent 入口统一（facade 就绪 fail-closed）、真向量 seam（noop 诚实）。
4. O02 进程级故障注入与对象存储级快照恢复未演练。
5. I05 删除操作路由打标+终态驱动（协调器就绪）。
6. 部署级启动验证（compose up/helm install 实启）未执行（授权禁部署）。

## Release 判定（check_acceptance 实测）

`uv run --project semantic python scripts/semantic/check_acceptance.py '{"approved": false}' '{"security_leaks": 0}'`
→ 输出 `policy_not_approved`、`missing_contract`…、`missing_live_model`、`missing_browser` 等——**BLOCKED**（如实）。
