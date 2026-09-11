# V03 中文质量与上线阈值评估基线

生成时间：2026-09-11T16:06:16Z（semantica 实测运行）。数据集：`semantic/experiments/fixtures/questions.jsonl`（30 题，事实直答/多跳/冲突/无答案/中文定位/权限反例各 5 题，合成中文语料 `semantic/experiments/fixtures/evaluation_corpus.json`：13 文档、14 实体、13 断言，d04/d10 为受限文档）。所有数值来自实际运行，失败例保留在结果文件中未剔除。

## 复现命令

    # 1. 隔离 Neo4j（V02 已建容器复用）：
    docker run -d --name semantica-v02-neo4j -p 127.0.0.1:17687:7687 -p 127.0.0.1:17474:7474 -e NEO4J_AUTH=neo4j/$SEMANTICA_EXPERIMENT_NEO4J_PASSWORD neo4j:5.26-community

    # 2. semantica 模式（退出码 0）：
    uv run --project semantic/experiments python semantic/experiments/evaluate.py --backend semantica --dataset semantic/experiments/fixtures/questions.jsonl --output docs/superpowers/plans/semantica/semantica-results.jsonl

    # 3. native 模式（本轮退出码 2：未配置隔离测试端点，30 条错误行已写盘）：
    uv run --project semantic/experiments python semantic/experiments/evaluate.py --backend native --dataset semantic/experiments/fixtures/questions.jsonl --output docs/superpowers/plans/semantica/native-results.jsonl

    # 4. 评分器单测：
    uv run --project semantic/experiments python -m pytest semantic/experiments/test_evaluate.py -q

## semantica 模式实测（无模型、授权子图关键词检索）

| 指标 | 实测值 |
|---|---|
| 总正确率 | 0.767（23/30） |
| 事实直答 / 冲突 / 中文定位 / 权限反例 | 1.0 / 1.0 / 1.0 / 1.0 |
| 多跳 | 0.4（2/5） |
| 无答案（unanswerable_correct） | 0.2（1/5） |
| 证据 source_precision 均值 | 0.628 |
| 证据 source_recall 均值（仅非空预期题） | 0.925 |
| 权限泄漏（hard gate） | 0 次（命中内容对象/节点名/关联实体/引用原文/证据 ID 全量检查，大小写不敏感） |
| 延迟 p50（冷/热） | 0.032 ms / 0.015 ms（14 节点受控语料，仅代表实验规模） |
| 延迟 p95（冷/热） | 0.063 ms / 0.020 ms |
| 图写入（"索引"耗时） | 741.4 ms（13 文档 13 断言写 Neo4j，清空重建） |
| tokens | 0（未调用任何模型） |

指标约定：cold 含检索器构造；hot 为同一检索器两次重复查询的中位数。source_precision 将空引用记 0.0（严格方向：答对的"无答案"题也会拉低均值）；source_recall 均值只统计非空预期题（空预期的 recall 恒为 1.0，计入会虚高——评审指出后已修正，原口径 0.95 → 0.925）。

## 失败例与结论（全部为真实失败，非删除断言）

- **多跳 0.4（mh-03/04/05 失败）**：上游 ContextGraph 相关实体扩展仅沿出边方向（`_adjacency` 按源节点索引，BFS 只走 source→target）。命中汇点（如"深圳/杭州"位置节点、无出边目标）时无法回溯来源链，证据 recall 0.5。→ 生产适配（Q01）必须实现方向感知扩展。
- **无答案 0.2（ua-01..04 失败）**：关键词检索命中可见实体后按关联边引用证据，不能判定"授权范围内证据不足"，仅 ua-05（词面完全不命中）通过。→ 印证规格 §7：证据不足判定（insufficient_evidence）必须由 Q03 显式实现，不能依赖检索器自发。
- **precision 0.628**：命中节点的全部关联边都被引用（双向、不过滤谓词相关性），过度引用可见但无关的事实。→ Q01 排名与引用筛选需要改进后再收紧阈值。
- **权限反例 5/5 且 0 泄漏**：受限文档 d04/d10 的内容（松柏/青竹）、实体、证据 ID 在任何结果字段均未出现；授权过滤先于图构建与检索的顺序有效。

## native 模式：阻断（未测）

`native-results.jsonl` 30 条均为错误行（未从结果集剔除，summary available=false 并附 blocked_reason）。阻断条件：
1. 需要现有 WeKnora 图服务/图检索入口的隔离测试部署（`SEMANTICA_EXPERIMENT_NATIVE_URL` 指向隔离环境，禁止默认连生产）；
2. native 图抽取需真实模型凭据向隔离图注入同版本语料数据。
两者齐备前，native 对照、同语料比较门禁（规格 §9 切换前提）保持未测，不虚构数值。

## 阈值提议与门禁状态

数值门槛写入 `acceptance-policy.json`：`approved=false`（待用户明确验收决策）；权限泄漏为绝对 hard gate（proposed_max=0，实测 0）；semantica 各项阈值依据上表实测推导；native 与对照门禁为 unmeasured。多跳/无答案阈值即当前实测值——这是对现状的诚实下限，Q01/Q03 完成后必须上调。

## 限制

- 受控语料 14 节点/13 断言，延迟与索引耗时仅代表实验规模，不外推生产规模。
- 检索为关键词模式（无向量、无模型）；模型模式质量/延迟/用量门槛待凭据后补测。
- 数据集为合成中文语料（可公开），不含真实企业内容。
- 评测与 V02 探针共用同一 Neo4j probe 标签且无跨进程锁：串行运行安全（pytest 与 CLI 均为串行），并发运行会互相清写，需串行使用。
