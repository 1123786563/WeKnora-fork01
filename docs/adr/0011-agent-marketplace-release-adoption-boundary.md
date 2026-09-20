# Agent Marketplace 分发 Release，Tenant 引入后生成本地变体

Agent Marketplace 的分发单位是脱敏、不可变并带 Manifest 与 Dependency Lock 的 Agent Release，而不是运行中的 Agent 或会自动漂移的模板；Built-in、Public Marketplace 和 Tenant Catalog 共用这一模型。Tenant 通过 Agent Adoption 接受 Listing，再以固定 Release 派生一个或多个本地 Agent Variant，完成本地能力映射、测试和 Agent Version 发布后才可用于 Task。这样牺牲“一键即运行”，换取 Tenant 隔离、权限本地化、显式升级、可复现执行与跨来源一致治理。
