# V02 持久图桥接与推理证据

V02 是预生产能力实验。它不构成 A03 模型网关、授权/预算结算、生产 ACL、抽取质量或 native 对比验收。

## 实际成功证据

- 每次运行使用带随机 nonce 的专用 `semantica-v02-<nonce>` Neo4j `2025.10.1` 容器和独立
  同 nonce 卷；写入、删除、重启和清理前均核验 container ID、V02/nonce labels、固定镜像、
  running 状态和唯一的 `127.0.0.1:17687` Bolt 映射。镜像固定为
  `neo4j@sha256:155c8aad10d5c838bc3bbc476c0418779086547822acb214ec5e3d49ba336907`，
  Bolt 仅绑定 `127.0.0.1:17687`。
- `GraphStore(backend="neo4j")` 写入受控夹具的三条关系，写客户端关闭后仅重启
  该容器；新 `GraphStore` 客户端读回三条持久行。D1/D2 授权投影在构造内存图前完成，
  保留 assertion、文档 revision、证据 ID 与中文 quote，仅输出 `e-d1`、`e-d2`，不含
  隐藏关系 `e-hidden` 或“隐藏组件”。
- 实际 Semantica `0.6.8` `Reasoner` 注册了唯一的两前提 `depends_on` 规则。两前提得到
  `indirectly_depends_on(a,c)`；缺少第二前提没有结论。含 `not_depends_on(a,b)` 的真实
  `forward_chain` 输出被保留，但冻结 API 没有可注册的 contradiction/conflict operation，
  所以 conflict capability 明确标为 `actual-runtime/unavailable`，不再把字符串标记冒充上游能力。
- 实际 Semantica `GraphReasoner` 注册的 provider 仅通过 `127.0.0.1:18092` Go 实验入口
  调用现有 WeKnora Ollama `ChatStream` adapter；固定模型 `qwen2.5:0.5b`、单次请求 20 秒
  deadline、96 token cap，不接受任意模型、URL 或 tools。URL 固定精确解析为
  `http://127.0.0.1:18092`，拒绝 userinfo、其它 host/port、IPv6、query 和 fragment；严格 JSON
  只接受单个 `{prompt}`。stream chunk 的 UTF-8 byte cap 超限会取消并以失败返回，绝不标为完成。
  V02 使用 streaming 原始计数，未采用已知可疑的非流式 completion subtraction。
- 组合验收把真实 `probe_roundtrip(...)["result"]` 直接交给 `probe_model`。provider 记录实际
  prompt SHA-256，并验证 D1/D2 的 assertion/revision/quote 被输入，hidden assertion/evidence/quote
  不在 prompt。完整失败/成功调用和原始总量都保留在 JSON，不能作为商业结算证据。

完整机器可读产物：[bridge-run.json](evidence/2026-09-20/v02/bridge-run.json)。

## 验证

```text
UV_PROJECT_ENVIRONMENT=/tmp/semantica-v01-clean-final uv run --project semantic/experiments python -m pytest semantic/experiments/test_graph_bridge.py semantic/experiments/test_reasoning_bridge.py -q
exit 0; 12 passed in 192.57s

go test ./semantic/experiments/model_gateway
exit 0
```

RED 先于实现：Python bridge 测试因 `bridge_probe` 不存在而在收集阶段失败；本轮 URL 严格解析
测试因 `validate_gateway_url` 不存在而失败；Go boundary 测试因 `decodePrompt`、`validUsage` 和
`appendCapped` 不存在而编译失败。它们均以最小实现转绿。

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
