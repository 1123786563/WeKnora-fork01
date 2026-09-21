# V03 离线评估基线

本产物仅提供确定性的离线评分基础：合成中文 cases、预录 observation JSONL 的严格加载、逐 case 评分与汇总。所有 fixtures 和 example observations 都标为 `synthetic`；它们只用于 parser/scorer smoke coverage，不是查询运行结果。

指标定义：`source_precision` 是预期证据与返回证据交集除以返回证据数；`source_recall` 是该交集除以预期证据数。无预期证据的 recall 为 1；可回答 case 的空预测 precision 为 0。`permission_leak` 独立于这些指标，命中 forbidden evidence 或出现 access violation 即为硬失败。requested/actual mode 不一致也是硬失败，不能把 GraphRAG 静默降级或升级为 Reason。不可回答 case 仅在 `insufficient_evidence` 且没有证据或结论时得分正确。

离线复现命令：

```sh
uv run --locked --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q
uv run --locked --project semantic/experiments python semantic/experiments/evaluate.py --dataset semantic/experiments/fixtures/questions.jsonl --observations semantic/experiments/fixtures/evaluation-example-observations.jsonl --output /tmp/semantica-v03-synthetic-report.json
```

本基线未包含 native-vs-Semantica 查询对照、controlled-provider、live-model、授权企业语料、延迟/Token 测量或生产切换。缺失测量以 `null` 保存，绝不记为零。上线策略见 `acceptance-policy.json`，其 `approved` 必须保持 `false`，直到有可比的授权测量与产品负责人明确确认门槛。
