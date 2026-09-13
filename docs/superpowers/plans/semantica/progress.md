# Semantica 实施台账

状态：仅计划完成，24个任务均未开始。总计划：[实施入口](../2026-09-11-semantica-implementation.md)。

状态值 pending / in_progress / blocked / implemented / verified。每项验证记录必须包含commit SHA、精确命令、退出码、环境、产物路径、失败/限制；无真实证据不标记verified。执行前记录实际基线与已有脏文件。

| 任务 | 名称 | 依赖 | 状态 | 证据/提交/阻断原因 |
|---|---|---|---|---|
| V01 | 冻结版本与最小安装契约 | 无 | pending | 尚未执行 |
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

## 当前边界

- 本轮只编写计划，没有能力实验、测试运行、容器启动、模型调用或上线切换。
- V01精确版本、V03数值门槛、真实模型证据须在执行阶段补齐；这些是明确任务产物，不是已经通过的前提。
- 未创建GitHub Issue或外部发布；没有分配虚构Issue编号。
