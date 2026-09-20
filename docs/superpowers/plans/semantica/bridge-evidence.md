# V02 持久图桥接与推理证据

V02 是预生产能力实验。它不构成 A03 模型网关、授权/预算结算、生产 ACL、抽取质量或 native 对比验收。

## 实际成功证据

- 存储使用专用 `semantica-v02-bridge` Neo4j `2025.10.1` 容器和独立
  `semantica-v02-bridge-data` 卷，镜像固定为
  `neo4j@sha256:155c8aad10d5c838bc3bbc476c0418779086547822acb214ec5e3d49ba336907`，
  Bolt 仅绑定 `127.0.0.1:17687`。
- `GraphStore(backend="neo4j")` 写入受控夹具的三条关系，写客户端关闭后仅重启
  该容器；新 `GraphStore` 客户端读回三条持久行。D1/D2 授权投影在构造内存图前完成，
  保留 assertion、文档 revision、证据 ID 与中文 quote，仅输出 `e-d1`、`e-d2`，不含
  隐藏关系 `e-hidden` 或“隐藏组件”。
- 实际 Semantica `0.6.8` `Reasoner` 注册了唯一的两前提 `depends_on` 规则。两前提得到
  `indirectly_depends_on(a,c)`；缺少第二前提没有结论；显式 `not_depends_on(a,b)` 报告
  `conflicting_evidence`，不会伪造相反结论。
- 实际 Semantica `GraphReasoner` 注册的 provider 仅通过 `127.0.0.1:18092` Go 实验入口
  调用现有 WeKnora Ollama `ChatStream` adapter；固定模型 `qwen2.5:0.5b`、单次请求 20 秒
  deadline、96 token cap，不接受任意模型、URL 或 tools。中文问题得到“构建依赖发布。”，
  raw provider 计数为 166 prompt、5 completion、171 total。V02 使用 streaming 的原始
  计数，未采用已知可疑的非流式 completion subtraction。协调窗口共三次有界调用（模型
  契约、保留输出、完整验收），原始总计 480 prompt、15 completion、495 total；逐次明细
  见 JSON，不能作为商业结算证据。

完整机器可读产物：[bridge-run.json](evidence/2026-09-20/v02/bridge-run.json)。

## 验证

```text
UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q
exit 0; 6 passed in 40.61s

go test ./semantic/experiments/model_gateway
exit 0
```

RED 先于实现：Python bridge 测试因 `bridge_probe` 不存在而在收集阶段失败；Go gateway
边界测试因 `decodePrompt` 与 `validUsage` 不存在而编译失败。两项随后以最小实现转绿。

## 限制与未验证项

- 选用 dedicated 实例，因为它给实验提供可观测的完整容器/卷边界；未对 shared-isolated
  account/database/schema/collection/prefix 进行等价实现或生产拓扑选择。
- 这是授权投影的合成夹具检查，不能证明 tenant/KB ACL、撤权、删除屏障、generation 或生产
  读路径。
- Go loopback 是 V02 特许的 live-local-provider 证据，不是 A03 `SemanticModelGateway`，没有
  capability token、`ExecutionGate.Begin/Finish`、预算结算、重试对账或远程 provider 支持。
- `GraphReasoner` 返回 plain text；它没有产生结构化结论或证据 DAG。证据 ID 是上游已授权
  子图的显式投影，不应被视为模型生成的引用。
- 夹具为手工种子，不表示实体/关系自动抽取质量；V03 必须单独消费这些可重复候选函数并区分该边界。
