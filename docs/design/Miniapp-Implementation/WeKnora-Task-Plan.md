# 05｜任务拆分与并行实施计划

> 面向开发执行者：逐任务读取对应详细设计、依赖交付物和验收条件；采用测试先行与独立评审。建议使用 superpowers 的 subagent-driven-development 或 executing-plans 工作流。此文只交付计划，未向仓库创建分支、Issue、提交或PR。

**Goal：**为现有WeKnora SaaS增加可用、可恢复、可控的微信小程序入口。

**Architecture：**新增apps/miniprogram；复用contracts/api-client/domain与现有Go真值。只补微信端适配、身份桥接和缺失的聚合读模型；支付渠道在资格核验后按现有订单体系扩展。

**Tech Stack：**Taro React、TypeScript、SCSS、现有Go/Gin与共享工作区；具体兼容补丁由MP00验证锁定。

**Spec：**01前端详细设计、02后端与共享契约详细设计、03视觉与组件规范、04页面详细设计。

## 1. 编号、状态与复用边界

保留原方案12个MP父任务，细拆为59个子任务；例如MP01-T02-04隶属原MP01-T02。前后端共用一套编号，不各自再建立冲突任务。

所有子任务当前为planned，PR、证据和合并提交为空；HTML原型的测试不能将生产任务标为完成。执行状态采用planned → ready → in_progress → review → verified → merged，阻塞使用blocked并附原因。使用tasks/task-index.json作为机器可读台账。

全局约束：不重建身份、租户、Agent循环、钱包或计量账本；不将Web/Expo节点直接搬入Taro；不私改既有StartExecutionInput；不把颜色或角色名当权限；不以客户端回调作为支付或权益真值；不运行模型生成HTML/JS。

## 2. 分轨并行与文件锁

先冻结接口，再并行编码。推荐轨道：A端工程/设计系统，B共享SDK与恢复，C Go身份，D Go任务查询，E业务页面，F商业化，G QA/发布。一个人可以兼任多个轨道，轨道不是强制团队规模。

`pnpm-lock.yaml`、根workspace导出、router总装配、shared ports与client.ts采用单集成人串行合并。不同子文件也可能共享同一个锁：contracts.mini、sdk.mini、payment.callback、domain.execution-cache等以任务卡为准。一个PR仅实现自己的模块；需要修改他人锁定文件时先输出接口变更请求，不能直接覆盖。

表中波次仅是硬依赖的拓扑深度，不表示日期或人天；相同波次仍要校验锁不冲突。UI可在契约冻结后使用受控fixture提前开工，但联调验收必须等待真实依赖，不能把mock页面当后端已完成。

## 3. 任务总表

| 子任务 | 交付 | Track | 硬依赖 | 波次 |
|---|---|---|---|---|
| MP00-T01-01 | 固定源码与差异台账 | BASE | 无 | W01 |
| MP00-T01-02 | Taro工作区与版本矩阵 | BASE | MP00-T01-01 | W02 |
| MP00-T01-03 | 真机流传输能力实验 | SDK | MP00-T01-02 | W03 |
| MP00-T01-04 | 共享导出与依赖边界 | SDK | MP00-T01-01 | W02 |
| MP00-T02-01 | 身份、成员与资源授权核查 | BE | MP00-T01-01 | W02 |
| MP00-T02-02 | 支付回调鉴权接线核验 | PAY | MP00-T01-01 | W02 |
| MP00-T02-03 | 支付资格与商品分类门禁 | OPS | MP00-T01-01 | W02 |
| MP01-T01-01 | 设计令牌与构建生成 | DESIGN | MP00-T01-01 | W02 |
| MP01-T01-02 | 基础组件与状态样式 | FE | MP01-T01-01, MP00-T01-02 | W03 |
| MP01-T01-03 | 四Tab、导航与安全区 | FE | MP01-T01-02, MP00-T01-02 | W04 |
| MP01-T01-04 | 路由守卫与六类异常 | FE | MP01-T01-02, MP00-T02-01 | W04 |
| MP01-T01-05 | 首页与Agent浏览 | FE | MP01-T01-03, MP01-T01-04, MP01-T02-01 | W05 |
| MP01-T02-01 | 普通HTTP适配 | SDK | MP00-T01-03, MP00-T01-04 | W04 |
| MP01-T02-02 | SSE跨块分帧修复 | SDK | MP00-T01-04 | W03 |
| MP01-T02-03 | 流式端口与生命周期 | SDK | MP00-T01-03, MP01-T02-02 | W04 |
| MP01-T02-04 | NativeFile上传提前分流 | SDK | MP01-T02-01, MP00-T01-04 | W05 |
| MP01-T02-05 | 凭证与刷新单飞 | SDK | MP01-T02-01, MP00-T02-01 | W05 |
| MP01-T02-06 | 原子切租户与旧回包隔离 | SDK | MP01-T02-03, MP01-T02-05 | W06 |
| MP01-T02-07 | 有界缓存与待确认意图存储 | SDK | MP01-T02-06 | W07 |
| MP01-T03-01 | 微信外部身份唯一性 | BE | MP00-T02-01 | W03 |
| MP01-T03-02 | 微信code登录桥接 | BE | MP01-T03-01, MP00-T02-01 | W04 |
| MP01-T03-03 | 双侧验证账号绑定 | BE | MP01-T03-01, MP01-T03-02 | W05 |
| MP01-T03-04 | 登录绑定和空间选择UI | FE | MP01-T01-03, MP01-T03-02, MP01-T03-03, MP01-T02-06 | W07 |
| MP01-T03-05 | 邀请接受与加入 | FE | MP01-T03-04, MP01-T01-04 | W08 |
| MP02-T01-01 | 普通聊天用例控制器 | FE | MP01-T02-03, MP01-T02-06 | W07 |
| MP02-T01-02 | 对话渲染与引用卡 | FE | MP02-T01-01, MP01-T01-02 | W08 |
| MP02-T01-03 | 临时附件历史与键盘输入 | FE | MP02-T01-01, MP01-T02-04, MP01-T01-03 | W08 |
| MP02-T01-04 | 聊天弱网安全回归 | QA | MP02-T01-02, MP02-T01-03 | W09 |
| MP02-T02-01 | 知识库文档列表与权限 | FE | MP01-T02-01, MP01-T02-06, MP01-T01-03 | W07 |
| MP02-T02-02 | 文件与URL知识导入 | FE | MP02-T02-01, MP01-T02-04 | W08 |
| MP02-T02-03 | 解析状态与重入刷新 | FE | MP02-T02-02, MP01-T02-06 | W09 |
| MP02-T02-04 | 引用定位与受控预览 | FE | MP02-T02-01, MP01-T02-04 | W08 |
| MP03-T01-01 | 本人任务读模型与索引 | BE | MP00-T02-01 | W03 |
| MP03-T01-02 | 任务列表API与客户端契约 | BE | MP03-T01-01, MP00-T01-04 | W04 |
| MP03-T01-03 | 待办与能力聚合 | BE | MP03-T01-01, MP00-T02-01 | W04 |
| MP03-T01-04 | 授权启动选项与预算单位 | BE | MP00-T02-01, MP03-T01-02 | W05 |
| MP03-T02-01 | 一次业务意图发起与对账 | FE | MP03-T01-04, MP01-T02-07, MP01-T01-05 | W08 |
| MP03-T02-02 | 快照安装与水位提交 | SDK | MP00-T01-04, MP01-T02-07, MP03-T01-02 | W08 |
| MP03-T02-03 | 前后台恢复同一执行 | FE | MP03-T02-02, MP01-T02-03, MP01-T02-06 | W09 |
| MP03-T02-04 | 任务中心与详情页面 | FE | MP03-T01-02, MP03-T01-03, MP03-T02-03, MP01-T01-03 | W10 |
| MP03-T02-05 | 一次审批与版本冲突 | FE | MP03-T01-03, MP03-T02-04 | W11 |
| MP03-T02-06 | 取消与追加指令 | FE | MP03-T02-04 | W11 |
| MP03-T02-07 | 任务产物安全预览 | FE | MP01-T02-04, MP03-T02-04 | W11 |
| MP03-T02-08 | 恢复、审批与幂等集成验收 | QA | MP03-T02-01, MP03-T02-05, MP03-T02-06, MP03-T02-07 | W12 |
| MP04-T01-01 | 商业摘要与套餐只读页 | FE | MP01-T02-01, MP01-T02-06, MP01-T01-03 | W07 |
| MP04-T01-02 | 通道能力与严格支付契约 | PAY | MP00-T02-03, MP04-T01-01 | W08 |
| MP04-T01-03 | 获准微信渠道服务端适配 | PAY | MP04-T01-02, MP01-T03-02 | W09 |
| MP04-T01-04 | 报价支付与订单状态页 | FE | MP04-T01-02, MP04-T01-03, MP04-T01-01 | W10 |
| MP04-T01-05 | 回调对账与幂等履约 | PAY | MP00-T02-02, MP04-T01-03 | W10 |
| MP04-T01-06 | 支付异常与旧渠道回归 | QA | MP04-T01-04, MP04-T01-05 | W11 |
| MP04-T02-01 | 订阅授权与通知偏好 | FE | MP01-T03-04, MP03-T02-04 | W11 |
| MP04-T02-02 | 去重出站通知投递 | BE | MP04-T02-01, MP03-T01-03 | W12 |
| MP04-T02-03 | 通知邀请深链安全回跳 | FE | MP01-T01-04, MP01-T03-05, MP03-T02-04 | W11 |
| MP04-T03-01 | 三端契约与构建回归 | QA | MP02-T01-04, MP02-T02-03, MP03-T02-08, MP04-T01-06 | W13 |
| MP04-T03-02 | 跨租户与秘密数据验收 | QA | MP03-T02-08, MP04-T01-06 | W13 |
| MP04-T03-03 | 视觉还原和可访问性验收 | QA | MP02-T01-04, MP02-T02-04, MP03-T02-08, MP04-T01-06 | W13 |
| MP04-T03-04 | 真机性能弱网与后台 | QA | MP04-T03-01, MP04-T03-03 | W14 |
| MP04-T03-05 | 隐私内容和上线资格 | OPS | MP00-T02-03, MP04-T03-02 | W14 |
| MP04-T03-06 | 灰度发布与回滚演练 | OPS | MP04-T03-04, MP04-T03-05, MP04-T02-02, MP04-T02-03 | W15 |

## 4. 阶段出口

| 阶段 | 子任务数 | 出口 |
|---|---:|---|
| MP00 | 7 | 源码/兼容/真机能力与支付门禁均有证据 |
| MP01 | 17 | 登录、端适配、作用域、导航与UI基础可联调 |
| MP02 | 8 | 聊天、临时附件、知识导入和引用可以真实使用 |
| MP03 | 12 | 任务从提交到恢复、审批、命令和产物闭环 |
| MP04 | 15 | 商业化、通知、真机、安全、资格与回滚验收 |

## 5. 发布分支与可选能力

推荐先交付登录、知识问答、导入、任务和审批；支付及通知可由能力开关延后公开。完整59任务发布出口依赖通知投递和深链，不允许遗漏后台通知服务而假称通知可用。若拆分里程碑，须记录显式裁剪清单并关闭相关入口，不删除安全回归。

任务列表固定created_at+id排序；单条updated_at只作展示。snapshot/事件水位、预算单位、微信通道参数是必须以源码/实机证据锁定的门禁；本方案不编造原接口已含的字段。

## 6. 每个PR的共同审查内容

提交差异、契约变化、测试结果、页面截图、租户权限影响、迁移回滚、安全日志、能力开关、共享包兼容。没有对应真实证据，状态不得从review推进到verified。前端merge不能提前让对应后端任务自动完成。

## 7. 独立任务卡

完整卡片位于tasks/，每卡包含目标、角色、仓库、模块、文件范围、消费者/产出、依赖、排他锁、步骤、三条验收与完成门禁。以下汇总可用于单文件评审。

# MP00-T01-01｜固定源码与差异台账

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付固定commit、目录/接口E-A-N-V台账。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T01 / MP00 |
| Track / 角色 | BASE / 技术负责人 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `baseline` |
| File Lock Key | `baseline` |
| 硬依赖 | 无 |
| 拓扑波次 | W01；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**上一版技术方案、源码核查与当前只读仓库基线

**Produces：**固定commit、目录/接口E-A-N-V台账

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: docs/miniprogram/baseline.md`
- `N: docs/miniprogram/source-diff.json`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 取得只读工作副本并记录HEAD。
- [ ] 对照上版24个源码索引。
- [ ] 逐项记录路由、DTO、装配与依赖差异。
- [ ] 冻结此次实施基线。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 所有现有能力能定位固定源码。
2. 新增接口不标为已实现。
3. 阻断差异有负责人和对应任务。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP00-T01-01-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP00-T01-02｜Taro工作区与版本矩阵

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付可编译最小工程与build/typecheck/test脚本。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T01 / MP00 |
| Track / 角色 | BASE / 技术负责人 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.config` |
| File Lock Key | `workspace.root` / `mini.config` |
| 硬依赖 | MP00-T01-01 |
| 拓扑波次 | W02；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-01 → 固定commit、目录/接口E-A-N-V台账

**Produces：**可编译最小工程与build/typecheck/test脚本

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/package.json`
- `N: apps/miniprogram/config/index.ts`
- `A: pnpm-workspace.yaml`
- `A: pnpm-lock.yaml`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 使用官方React模板锁定兼容组合。
- [ ] 只新增小程序workspace。
- [ ] 注册固定环境与最小页面。
- [ ] 运行Web/Expo回归和Taro启动实验。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 全部@tarojs包补丁一致。
2. 不整体降级Web/Expo。
3. 包中无AppSecret或租户API Key。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP00-T01-02-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP00-T01-03｜真机流传输能力实验

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付HTTP状态、headers、chunk、abort能力矩阵。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T01 / MP00 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.probe` |
| File Lock Key | `mini.probe` / `sdk.ports` |
| 硬依赖 | MP00-T01-02 |
| 拓扑波次 | W03；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-02 → 可编译最小工程与build/typecheck/test脚本

**Produces：**HTTP状态、headers、chunk、abort能力矩阵

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/tests/device/transport-probe.ts`
- `N: docs/miniprogram/device-probe.md`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 准备SSE和401/403测试响应。
- [ ] 观察headers/status可得时点。
- [ ] 分片发送中文emoji并取消请求。
- [ ] 锁定真实元信息扩展或有界降级方案。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不伪造HTTP200。
2. 401正文不变成模型输出。
3. 首事件前后重试行为分开验证。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP00-T01-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP00-T01-04｜共享导出与依赖边界

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付允许导入清单与包体基线。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T01 / MP00 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `shared.exports` |
| File Lock Key | `shared.exports` |
| 硬依赖 | MP00-T01-01 |
| 拓扑波次 | W02；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-01 → 固定commit、目录/接口E-A-N-V台账

**Produces：**允许导入清单与包体基线

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: packages/api-client/package.json`
- `A: packages/domain/package.json`
- `N: apps/miniprogram/tests/boundary.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 扫描DOM/RN全局与副作用。
- [ ] 补必要深路径导出。
- [ ] 阻止大barrel导入。
- [ ] 生成主包/分包依赖报告。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 小程序无Radix/RN运行节点。
2. domain不依赖Taro。
3. 可定位每个大依赖来源。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP00-T01-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP00-T02-01｜身份、成员与资源授权核查

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付tenantless与owner权限测试矩阵。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T02 / MP00 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `auth.audit` |
| File Lock Key | `auth.audit` |
| 硬依赖 | MP00-T01-01 |
| 拓扑波次 | W02；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-01 → 固定commit、目录/接口E-A-N-V台账

**Produces：**tenantless与owner权限测试矩阵

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: internal/middleware/auth.go`
- `A: internal/router/routes_auth_tenant.go`
- `N: internal/handler/mini_auth_scope_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 核对匿名/tenantless入口。
- [ ] 创建双租户三用户测试数据。
- [ ] 校验成员、角色、资源所有者。
- [ ] 冻结403与TENANT_REQUIRED映射。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 伪造tenant不能授权。
2. Owner不默认看到他人私有任务。
3. 无空间用户可走me/邀请/创建。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP00T0201 的测试组
go test ./internal/... -run '^TestMiniMP00T0201' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP00-T02-02｜支付回调鉴权接线核验

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付最小公开回调入口与真实Router测试。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T02 / MP00 |
| Track / 角色 | PAY / 商业化工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `payment.callback` |
| File Lock Key | `router.root` / `auth.middleware` / `payment.callback` |
| 硬依赖 | MP00-T01-01 |
| 拓扑波次 | W02；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-01 → 固定commit、目录/接口E-A-N-V台账

**Produces：**最小公开回调入口与真实Router测试

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: internal/router/routes_commercial.go`
- `A: internal/middleware/auth.go`
- `N: internal/router/mini_callback_auth_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 构造无JWT合法签名回调。
- [ ] 构造伪签名和普通商业写请求。
- [ ] 仅修精确公开路径。
- [ ] 回归provider验签和普通API鉴权。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 合法签名到达处理器。
2. 伪签名无账务副作用。
3. 不开放整个commercial路由。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP00T0202 的测试组
go test ./internal/... -run '^TestMiniMP00T0202' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP00-T02-03｜支付资格与商品分类门禁

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付逐商品逐终端的通道可用矩阵。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP00-T02 / MP00 |
| Track / 角色 | OPS / 发布/运营负责人 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `payment.qualification` |
| File Lock Key | `payment.qualification` |
| 硬依赖 | MP00-T01-01 |
| 拓扑波次 | W02；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-01 → 固定commit、目录/接口E-A-N-V台账

**Produces：**逐商品逐终端的通道可用矩阵

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: docs/miniprogram/payment-qualification.md`
- `N: docs/miniprogram/channel-decision.json`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 收集真实主体和商品描述。
- [ ] 核对官方规则与后台开通状态。
- [ ] 记录审核证据和复核日期。
- [ ] 未核验通道保持关闭。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 未获准不能调起购买。
2. 无外部跳转绕过限制。
3. Native二维码不被当作小程序支付。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP00-T02-03-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T01-01｜设计令牌与构建生成

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付106个令牌及CSS/SCSS/TS同源输出。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T01 / MP01 |
| Track / 角色 | DESIGN / 设计系统工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `design.tokens` |
| File Lock Key | `design.tokens` |
| 硬依赖 | MP00-T01-01 |
| 拓扑波次 | W02；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-01 → 固定commit、目录/接口E-A-N-V台账

**Produces：**106个令牌及CSS/SCSS/TS同源输出

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: packages/design-tokens/src/miniapp.tokens.json`
- `N: packages/design-tokens/scripts/build-miniapp-tokens.ts`
- `N: apps/miniprogram/src/styles/_tokens.scss`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 导入本交付token源。
- [ ] 解析别名并验证无环。
- [ ] 逻辑375到750输入尺寸单次换算。
- [ ] 生成组件基线及对比度结果。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 16逻辑px只转为32设计px。
2. 安全区运行值不翻倍。
3. 主要文字/状态组合达到项目对比度目标。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T01-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T01-02｜基础组件与状态样式

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付Button/Field/Badge/Notice/EmptyState。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T01 / MP01 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.primitives` |
| File Lock Key | `mini.primitives` |
| 硬依赖 | MP01-T01-01, MP00-T01-02 |
| 拓扑波次 | W03；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T01-01 → 106个令牌及CSS/SCSS/TS同源输出；MP00-T01-02 → 可编译最小工程与build/typecheck/test脚本

**Produces：**Button/Field/Badge/Notice/EmptyState

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/components/primitives/Button.tsx`
- `N: apps/miniprogram/src/components/primitives/FormField.tsx`
- `N: apps/miniprogram/src/components/primitives/StatusBadge.tsx`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 先写默认禁用加载错误用例。
- [ ] 用Taro原生节点实现。
- [ ] 补label与操作反馈。
- [ ] 处理长中文及字号变化。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 禁用按钮不触发操作。
2. 状态含文字及图形。
3. 组件中无wx.request或DOM依赖。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T01-02.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T01-03｜四Tab、导航与安全区

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付MiniNav/AppFrame/SafeBottom。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T01 / MP01 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.layout` |
| File Lock Key | `mini.navigation` / `mini.layout` |
| 硬依赖 | MP01-T01-02, MP00-T01-02 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T01-02 → Button/Field/Badge/Notice/EmptyState；MP00-T01-02 → 可编译最小工程与build/typecheck/test脚本

**Produces：**MiniNav/AppFrame/SafeBottom

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/app.config.ts`
- `N: apps/miniprogram/src/components/layout/MiniNav.tsx`
- `N: apps/miniprogram/src/platform/metrics.ts`
- `N: apps/miniprogram/src/custom-tab-bar/index.tsx`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 登记四Tab与业务分包。
- [ ] 读取真实胶囊/窗口尺寸。
- [ ] 区分页面滚动和固定操作区。
- [ ] 校验键盘与前后台布局。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 业务操作不覆盖胶囊。
2. 聊天无Tab重叠。
3. 320到430宽度无横向溢出。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T01-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T01-04｜路由守卫与六类异常

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付resolveEntry/guardCapability/errorToState。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T01 / MP01 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.guards` |
| File Lock Key | `mini.guards` |
| 硬依赖 | MP01-T01-02, MP00-T02-01 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T01-02 → Button/Field/Badge/Notice/EmptyState；MP00-T02-01 → tenantless与owner权限测试矩阵

**Produces：**resolveEntry/guardCapability/errorToState

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/navigation.ts`
- `N: apps/miniprogram/src/features/system/capability.ts`
- `N: apps/miniprogram/src/subpackages/system/pages/state/index.tsx`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 建立六类异常输入输出。
- [ ] 依次校验登录租户能力。
- [ ] 保存白名单返回目标。
- [ ] 恢复动作不产生隐式写请求。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 403不显示空数据。
2. 未装配不显示暂无任务。
3. 登录回跳不允许任意URL。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T01-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T01-05｜首页与Agent浏览

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P03/P04与AgentCard/AttentionCard。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T01 / MP01 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.home` |
| File Lock Key | `mini.home` / `mini.agents` |
| 硬依赖 | MP01-T01-03, MP01-T01-04, MP01-T02-01 |
| 拓扑波次 | W05；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T01-03 → MiniNav/AppFrame/SafeBottom；MP01-T01-04 → resolveEntry/guardCapability/errorToState；MP01-T02-01 → createMiniHttpTransport().send

**Produces：**P03/P04与AgentCard/AttentionCard

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/pages/home/index.tsx`
- `N: apps/miniprogram/src/subpackages/agents/pages/list/index.tsx`
- `N: apps/miniprogram/src/features/agents/view-model.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 读取授权Agent和近期会话。
- [ ] 实现意图草稿与搜索筛选。
- [ ] 待办卡定位可处理资源。
- [ ] 各卡片独立加载和降级。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不出现API Key配置表单。
2. 意图先确认再执行。
3. 单卡错误不拖垮整页。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T01-05.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-01｜普通HTTP适配

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付createMiniHttpTransport().send。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.http` |
| File Lock Key | `mini.http` |
| 硬依赖 | MP00-T01-03, MP00-T01-04 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-03 → HTTP状态、headers、chunk、abort能力矩阵；MP00-T01-04 → 允许导入清单与包体基线

**Produces：**createMiniHttpTransport().send

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/http.ts`
- `N: apps/miniprogram/tests/platform/http.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 覆盖JSON与字符串body。
- [ ] 归一header与ApiError。
- [ ] 映射timeout/AbortSignal。
- [ ] 仅明确幂等读取允许自动重试。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 非2xx不丢错误码。
2. 普通POST不自动重放。
3. 取消只影响当前请求。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-02｜SSE跨块分帧修复

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付跨chunk一致的SSE parser。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `sdk.chat.parser` |
| File Lock Key | `sdk.chat.parser` |
| 硬依赖 | MP00-T01-04 |
| 拓扑波次 | W03；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-04 → 允许导入清单与包体基线

**Produces：**跨chunk一致的SSE parser

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: packages/api-client/src/chat/stream.ts`
- `N: packages/api-client/src/chat/stream.boundary.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 先复现CR与LF拆块错误。
- [ ] 保留pendingCR与行缓存。
- [ ] 覆盖多data/注释/空id/EOF。
- [ ] 任意字节分片回归两类消费者。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 分片方式不改变事件序列。
2. 中文emoji无乱码。
3. 心跳不生成业务消息。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-02.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-03｜流式端口与生命周期

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付sendStream和可验证元信息。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.stream` |
| File Lock Key | `sdk.ports` / `mini.stream` |
| 硬依赖 | MP00-T01-03, MP01-T02-02 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-03 → HTTP状态、headers、chunk、abort能力矩阵；MP01-T02-02 → 跨chunk一致的SSE parser

**Produces：**sendStream和可验证元信息

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/stream.ts`
- `N: apps/miniprogram/src/platform/utf8.ts`
- `A: packages/api-client/src/ports.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 实现增量解码与有界队列。
- [ ] 绑定headers/chunk/fail/complete。
- [ ] 绑定abort并彻底释放监听。
- [ ] 非SSE和未知状态走明确错误路径。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 首帧无需等待连接结束。
2. 缓冲溢出停止并提示恢复。
3. onHide不发送cancel任务命令。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-04｜NativeFile上传提前分流

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付native multipart/binary下载适配。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.files` |
| File Lock Key | `sdk.client` / `mini.files` |
| 硬依赖 | MP01-T02-01, MP00-T01-04 |
| 拓扑波次 | W05；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-01 → createMiniHttpTransport().send；MP00-T01-04 → 允许导入清单与包体基线

**Produces：**native multipart/binary下载适配

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: packages/api-client/src/client.ts`
- `N: apps/miniprogram/src/platform/files.ts`
- `N: apps/miniprogram/tests/platform/files.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 先写无FormData环境失败测试。
- [ ] native分支前移。
- [ ] 映射filePath/formname/进度。
- [ ] 封装授权下载与临时文件预览。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 原生分支不构造FormData。
2. Web上传回归通过。
3. 无权文件不进入系统预览。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-05｜凭证与刷新单飞

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付CredentialAdapter和refresh coordinator。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.credentials` |
| File Lock Key | `mini.credentials` |
| 硬依赖 | MP01-T02-01, MP00-T02-01 |
| 拓扑波次 | W05；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-01 → createMiniHttpTransport().send；MP00-T02-01 → tenantless与owner权限测试矩阵

**Produces：**CredentialAdapter和refresh coordinator

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/credentials.ts`
- `N: apps/miniprogram/src/platform/refresh.ts`
- `N: apps/miniprogram/tests/platform/refresh.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 建立唯一凭证读写入口。
- [ ] 同scope并发刷新single-flight。
- [ ] 刷新失败清理并触发重新登录。
- [ ] 脱敏日志并限制存储范围。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 并发401只刷新一次。
2. 旧scope刷新不能覆盖新token。
3. 日志无token/session_key/AppSecret。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-05.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-06｜原子切租户与旧回包隔离

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ScopeCoordinator.switchTenant。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.scope` |
| File Lock Key | `mini.scope` |
| 硬依赖 | MP01-T02-03, MP01-T02-05 |
| 拓扑波次 | W06；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-03 → sendStream和可验证元信息；MP01-T02-05 → CredentialAdapter和refresh coordinator

**Produces：**ScopeCoordinator.switchTenant

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/scope.ts`
- `N: apps/miniprogram/tests/platform/scope.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 冻结写请求并结束旧订阅。
- [ ] 请求现有switch-tenant。
- [ ] 原子安装token/scope并递增generation。
- [ ] 失败回滚并恢复旧合法视图。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 旧流和旧回包不能落到新空间。
2. 失败不丢原空间。
3. 缓存键包含origin/user/tenant。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-06.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T02-07｜有界缓存与待确认意图存储

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ScopedStore与PendingIntentStore。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T02 / MP01 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.storage` |
| File Lock Key | `mini.storage` |
| 硬依赖 | MP01-T02-06 |
| 拓扑波次 | W07；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-06 → ScopeCoordinator.switchTenant

**Produces：**ScopedStore与PendingIntentStore

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/storage.ts`
- `N: apps/miniprogram/src/features/execution/intent-store.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 定义存储记录schema和TTL。
- [ ] 验证容量与写入结果。
- [ ] 发送前保存request_id及scope摘要。
- [ ] 退出撤权过期均可清理。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 存储失败不声称可恢复。
2. 缓存不含永久下载地址。
3. 不同用户租户键不碰撞。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T02-07.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T03-01｜微信外部身份唯一性

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ExternalIdentity repository与唯一索引。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T03 / MP01 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `identity.schema` |
| File Lock Key | `identity.schema` / `migrations.identity` |
| 硬依赖 | MP00-T02-01 |
| 拓扑波次 | W03；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T02-01 → tenantless与owner权限测试矩阵

**Produces：**ExternalIdentity repository与唯一索引

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/identity/wechat_identity.go`
- `N: migrations/mini_external_identity.sql`
- `N: internal/identity/wechat_identity_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 优先扩展现有provider模型。
- [ ] 定义appid/openid/user唯一性。
- [ ] 测试并发插入绑定冲突。
- [ ] 设计可回滚迁移与审计字段。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 同openid不能绑定两个用户。
2. 无unionid仍可登录。
3. session_key不明文存身份表。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP01T0301 的测试组
go test ./internal/... -run '^TestMiniMP01T0301' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T03-02｜微信code登录桥接

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付POST auth/wechat/mini/login。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T03 / MP01 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `auth.wechat` |
| File Lock Key | `auth.wechat` / `router.auth` |
| 硬依赖 | MP01-T03-01, MP00-T02-01 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T03-01 → ExternalIdentity repository与唯一索引；MP00-T02-01 → tenantless与owner权限测试矩阵

**Produces：**POST auth/wechat/mini/login

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/handler/auth_wechat_mini.go`
- `A: internal/router/routes_auth_tenant.go`
- `N: internal/handler/auth_wechat_mini_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 限流并固定服务端appid配置。
- [ ] 用code交换真实微信身份。
- [ ] 复用平台用户与token签发服务。
- [ ] 返回登录态或短期绑定挑战。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 客户端自报openid不可信。
2. 无空间返回tenant-required。
3. 微信机密不进入客户端响应。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP01T0302 的测试组
go test ./internal/... -run '^TestMiniMP01T0302' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T03-03｜双侧验证账号绑定

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付一次性绑定挑战消费与冲突响应。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T03 / MP01 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `auth.binding` |
| File Lock Key | `auth.binding` |
| 硬依赖 | MP01-T03-01, MP01-T03-02 |
| 拓扑波次 | W05；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T03-01 → ExternalIdentity repository与唯一索引；MP01-T03-02 → POST auth/wechat/mini/login

**Produces：**一次性绑定挑战消费与冲突响应

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/handler/auth_wechat_bind.go`
- `N: internal/identity/binding_challenge.go`
- `N: internal/handler/auth_wechat_bind_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 挑战绑定微信subject及有效期。
- [ ] 校验平台已有登录或近期证明。
- [ ] 事务写入唯一身份映射。
- [ ] 消费挑战并记录脱敏审计。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不能凭同邮箱自动合并。
2. 挑战重放不能二次绑定。
3. 并发冲突不改变原账号归属。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP01T0303 的测试组
go test ./internal/... -run '^TestMiniMP01T0303' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T03-04｜登录绑定和空间选择UI

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P01/P02与AuthStateMachine。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T03 / MP01 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.auth` |
| File Lock Key | `mini.auth` |
| 硬依赖 | MP01-T01-03, MP01-T03-02, MP01-T03-03, MP01-T02-06 |
| 拓扑波次 | W07；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T01-03 → MiniNav/AppFrame/SafeBottom；MP01-T03-02 → POST auth/wechat/mini/login；MP01-T03-03 → 一次性绑定挑战消费与冲突响应；MP01-T02-06 → ScopeCoordinator.switchTenant

**Produces：**P01/P02与AuthStateMachine

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/auth/pages/login/index.tsx`
- `N: apps/miniprogram/src/subpackages/auth/pages/workspace/index.tsx`
- `N: apps/miniprogram/src/features/auth/controller.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 接协议阅读与主动登录。
- [ ] 分流已绑定/需绑定/tenantless。
- [ ] 读取真实membership列表。
- [ ] 显式选择并进入空间。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不强制手机号作为登录前提。
2. 无任意origin输入。
3. 切换失败保持原合法空间。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T03-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP01-T03-05｜邀请接受与加入

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P19与邀请状态流。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP01-T03 / MP01 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.invitations` |
| File Lock Key | `mini.invitations` |
| 硬依赖 | MP01-T03-04, MP01-T01-04 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T03-04 → P01/P02与AuthStateMachine；MP01-T01-04 → resolveEntry/guardCapability/errorToState

**Produces：**P19与邀请状态流

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/account/pages/invitations/index.tsx`
- `N: apps/miniprogram/src/features/invitations/controller.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 读取本人邀请。
- [ ] 匿名先登录再校验token。
- [ ] 接受拒绝均确认当前身份。
- [ ] 刷新membership后让用户选空间。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 过期撤销邀请不可接受。
2. 他人邀请不可操作。
3. 接受不自动授权私有任务。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP01-T03-05.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T01-01｜普通聊天用例控制器

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ChatViewModel与send/stop。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T01 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.chat.domain` |
| File Lock Key | `mini.chat.domain` |
| 硬依赖 | MP01-T02-03, MP01-T02-06 |
| 拓扑波次 | W07；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-03 → sendStream和可验证元信息；MP01-T02-06 → ScopeCoordinator.switchTenant

**Produces：**ChatViewModel与send/stop

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/features/chat/controller.ts`
- `N: apps/miniprogram/src/features/chat/view-model.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 读取session及消息分页。
- [ ] 分开知识/Agent聊天协议。
- [ ] 投影文本引用和脱敏工具摘要。
- [ ] 中断消息明确标记未完成。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不隐式重复POST。
2. 旧scope消息不写新页。
3. 普通聊天不承诺durable恢复。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T01-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T01-02｜对话渲染与引用卡

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P06与受限Markdown渲染。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T01 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.chat.ui` |
| File Lock Key | `mini.chat.ui` |
| 硬依赖 | MP02-T01-01, MP01-T01-02 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T01-01 → ChatViewModel与send/stop；MP01-T01-02 → Button/Field/Badge/Notice/EmptyState

**Produces：**P06与受限Markdown渲染

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/chat/pages/thread/index.tsx`
- `N: apps/miniprogram/src/components/chat/AssistantMessage.tsx`
- `N: apps/miniprogram/src/components/chat/CitationCard.tsx`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 流阶段节流更新轻文本。
- [ ] 完成后受限节点格式化。
- [ ] 引用跳内部授权资源。
- [ ] 长对话分页保持锚点。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不执行模型HTML/JS。
2. 不显示私有推理或秘密工具参数。
3. 阅读历史时不强制跳底。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T01-02.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T01-03｜临时附件历史与键盘输入

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付附件状态与历史选择器。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T01 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.chat.composer` |
| File Lock Key | `mini.chat.composer` |
| 硬依赖 | MP02-T01-01, MP01-T02-04, MP01-T01-03 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T01-01 → ChatViewModel与send/stop；MP01-T02-04 → native multipart/binary下载适配；MP01-T01-03 → MiniNav/AppFrame/SafeBottom

**Produces：**附件状态与历史选择器

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/components/chat/ChatComposer.tsx`
- `N: apps/miniprogram/src/features/chat/attachments.ts`
- `N: apps/miniprogram/src/features/chat/history.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 按session上传临时附件。
- [ ] 显示失败进度和移除。
- [ ] 切会话处理草稿与附件归属。
- [ ] 真机检查键盘与安全区。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 临时附件不自动入库。
2. 不同session不串附件。
3. 停止输出不等于取消任务。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T01-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T01-04｜聊天弱网安全回归

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付流中断与撤权证据。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T01 / MP02 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.chat` |
| File Lock Key | `qa.chat` |
| 硬依赖 | MP02-T01-02, MP02-T01-03 |
| 拓扑波次 | W09；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T01-02 → P06与受限Markdown渲染；MP02-T01-03 → 附件状态与历史选择器

**Produces：**流中断与撤权证据

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/tests/chat/chat.integration.test.ts`
- `N: docs/miniprogram/evidence/chat.md`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 使用中文emoji多行响应。
- [ ] 首帧前后注入断网和401。
- [ ] 撤销引用/会话权限重入。
- [ ] 检查请求计数和监听释放。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 无乱码重复消息。
2. 引用撤权不读旧缓存。
3. 未发生隐式第二次模型调用。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP02-T01-04-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T02-01｜知识库文档列表与权限

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P11/P12与KnowledgeViewModel。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T02 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.knowledge.list` |
| File Lock Key | `mini.knowledge.list` |
| 硬依赖 | MP01-T02-01, MP01-T02-06, MP01-T01-03 |
| 拓扑波次 | W07；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-01 → createMiniHttpTransport().send；MP01-T02-06 → ScopeCoordinator.switchTenant；MP01-T01-03 → MiniNav/AppFrame/SafeBottom

**Produces：**P11/P12与KnowledgeViewModel

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/pages/knowledge/index.tsx`
- `N: apps/miniprogram/src/subpackages/documents/pages/kb/index.tsx`
- `N: apps/miniprogram/src/features/knowledge/list.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 按scope读取库和文档。
- [ ] 映射我的空间共享筛选。
- [ ] 基于capabilities显示操作。
- [ ] 缓存带时点且不可作授权真值。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 共享只读无上传入口。
2. 接口错误不显示空库。
3. 分页搜索保留scope。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T02-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T02-02｜文件与URL知识导入

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P13与UploadStateMachine。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T02 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.knowledge.import` |
| File Lock Key | `mini.knowledge.import` |
| 硬依赖 | MP02-T02-01, MP01-T02-04 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T02-01 → P11/P12与KnowledgeViewModel；MP01-T02-04 → native multipart/binary下载适配

**Produces：**P13与UploadStateMachine

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/documents/pages/import/index.tsx`
- `N: apps/miniprogram/src/features/knowledge/import.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 展示目标及授权。
- [ ] 校验允许类型和地址格式。
- [ ] 复用upload/import记录knowledge_id。
- [ ] 未知结果按既有幂等约定对账。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 上传成功不等于ready。
2. 不偷偷再建重复条目。
3. 服务端最终防护URL抓取。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T02-02.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T02-03｜解析状态与重入刷新

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ParsingStateMapper与前台轮询。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T02 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.knowledge.processing` |
| File Lock Key | `mini.knowledge.processing` |
| 硬依赖 | MP02-T02-02, MP01-T02-06 |
| 拓扑波次 | W09；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T02-02 → P13与UploadStateMachine；MP01-T02-06 → ScopeCoordinator.switchTenant

**Produces：**ParsingStateMapper与前台轮询

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/features/knowledge/processing.ts`
- `N: apps/miniprogram/tests/knowledge/processing.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 映射真实处理枚举。
- [ ] 前台可见列表有界轮询。
- [ ] 隐藏停止回前台重读。
- [ ] 失败提示真实原因和授权重试。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 退出页面不重新上传。
2. ready只由服务器确认。
3. 换空间释放旧轮询。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T02-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP02-T02-04｜引用定位与受控预览

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P14与AuthorizedDocumentPreview。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP02-T02 / MP02 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.knowledge.preview` |
| File Lock Key | `mini.knowledge.preview` |
| 硬依赖 | MP02-T02-01, MP01-T02-04 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T02-01 → P11/P12与KnowledgeViewModel；MP01-T02-04 → native multipart/binary下载适配

**Produces：**P14与AuthorizedDocumentPreview

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/documents/pages/detail/index.tsx`
- `N: apps/miniprogram/src/features/knowledge/reference.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 校验内部doc/reference参数。
- [ ] 重读授权原文及位置。
- [ ] 展示真实来源或位置缺失。
- [ ] 获准文件调用系统预览。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不伪造页码。
2. 撤权不以缓存绕过。
3. 模型URL不直接当可信下载。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP02-T02-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T01-01｜本人任务读模型与索引

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ListOwnedExecutions和稳定游标。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T01 / MP03 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `go.execution.query` |
| File Lock Key | `go.execution.query` / `migrations.execution` |
| 硬依赖 | MP00-T02-01 |
| 拓扑波次 | W03；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T02-01 → tenantless与owner权限测试矩阵

**Produces：**ListOwnedExecutions和稳定游标

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/application/workbench/list_executions.go`
- `N: internal/infrastructure/repository/execution_list.go`
- `N: migrations/mini_execution_list_index.sql`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 从现有任务投影摘要。
- [ ] 以created_at加id稳定排序。
- [ ] 按当前tenant和owner过滤。
- [ ] 校验查询计划和跨scope游标。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不信任前端owner参数。
2. 相同时间无漏项重复。
3. 不以updated_at可变排序制造漂移。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP03T0101 的测试组
go test ./internal/... -run '^TestMiniMP03T0101' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T01-02｜任务列表API与客户端契约

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付GET workbench/executions及TaskPage。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T01 / MP03 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `go.execution.list` |
| File Lock Key | `go.execution.list` / `contracts.mini` / `sdk.mini` |
| 硬依赖 | MP03-T01-01, MP00-T01-04 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T01-01 → ListOwnedExecutions和稳定游标；MP00-T01-04 → 允许导入清单与包体基线

**Produces：**GET workbench/executions及TaskPage

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/handler/workbench_list.go`
- `A: internal/router/routes_workbench.go`
- `N: packages/contracts/src/miniprogram/execution-list.ts`
- `N: packages/api-client/src/miniprogram/execution-list.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 定义cursor/status/limit校验。
- [ ] 调用查询与现有权限服务。
- [ ] 校验响应schema并新增client。
- [ ] 覆盖分页错误和兼容样例。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 新字段不冒充旧ExecutionDTO。
2. limit有上限。
3. 统计列表权限范围一致。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP03T0102 的测试组
go test ./internal/... -run '^TestMiniMP03T0102' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T01-03｜待办与能力聚合

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付GET workbench/inbox与能力原因码。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T01 / MP03 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `go.inbox` |
| File Lock Key | `go.inbox` / `capabilities` / `contracts.mini` |
| 硬依赖 | MP03-T01-01, MP00-T02-01 |
| 拓扑波次 | W04；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T01-01 → ListOwnedExecutions和稳定游标；MP00-T02-01 → tenantless与owner权限测试矩阵

**Produces：**GET workbench/inbox与能力原因码

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/handler/workbench_inbox.go`
- `A: internal/router/deployment_capabilities.go`
- `N: packages/contracts/src/miniprogram/capabilities.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 查询本人可处理交互。
- [ ] 投影脱敏摘要及版本。
- [ ] 组合部署租户权限终端能力。
- [ ] 输出不可用原因并保留服务端校验。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不可审批者看不到敏感待办。
2. 未装配返回unavailable。
3. 能力声明不是写操作授权凭证。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP03T0103 的测试组
go test ./internal/... -run '^TestMiniMP03T0103' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T01-04｜授权启动选项与预算单位

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付GET workbench/launch-options?agent_id=。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T01 / MP03 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `go.launch` |
| File Lock Key | `go.launch` / `contracts.mini` / `sdk.mini` |
| 硬依赖 | MP00-T02-01, MP03-T01-02 |
| 拓扑波次 | W05；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T02-01 → tenantless与owner权限测试矩阵；MP03-T01-02 → GET workbench/executions及TaskPage

**Produces：**GET workbench/launch-options?agent_id=

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/handler/workbench_launch_options.go`
- `N: packages/contracts/src/miniprogram/launch-options.ts`
- `N: packages/api-client/src/miniprogram/launch-options.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 解析授权Agent。
- [ ] 返回target/workspace组合及时点。
- [ ] 锁定budget显示单位和换算。
- [ ] 提交时再次校验选项有效性。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 任填target不能获权。
2. 积分换算有精确合同测试。
3. 选项失效不能静默换执行目标。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP03T0104 的测试组
go test ./internal/... -run '^TestMiniMP03T0104' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-01｜一次业务意图发起与对账

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P05和LaunchStateMachine。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.execution.launch` |
| File Lock Key | `mini.execution.launch` |
| 硬依赖 | MP03-T01-04, MP01-T02-07, MP01-T01-05 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T01-04 → GET workbench/launch-options?agent_id=；MP01-T02-07 → ScopedStore与PendingIntentStore；MP01-T01-05 → P03/P04与AgentCard/AttentionCard

**Produces：**P05和LaunchStateMachine

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/agents/pages/detail/index.tsx`
- `N: apps/miniprogram/src/features/execution/launch.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 用户确认后生成request_id。
- [ ] 发送前持久化最小意图。
- [ ] 提交现有StartExecutionInput。
- [ ] 超时查询原request并使用同ID重试。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 双击不生成两份意图。
2. 不私加attachment_ids。
3. unknown不等于可以换ID。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-02｜快照安装与水位提交

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付installSnapshot与有序提交。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | SDK / 共享SDK工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `domain.execution-cache` |
| File Lock Key | `domain.execution-cache` |
| 硬依赖 | MP00-T01-04, MP01-T02-07, MP03-T01-02 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T01-04 → 允许导入清单与包体基线；MP01-T02-07 → ScopedStore与PendingIntentStore；MP03-T01-02 → GET workbench/executions及TaskPage

**Produces：**installSnapshot与有序提交

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: packages/domain/src/mobile/execution-cache.ts`
- `N: packages/domain/src/mobile/execution-cache.snapshot.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 核实watermark保留窗口语义。
- [ ] 验证scope/run/schema后安装快照。
- [ ] 投影与游标一条记录原子提交。
- [ ] 测试重复gap未知schema与存储失败。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不可对空缓存直接commit大于1序号。
2. 游标不先于投影。
3. 未知schema停流并同步。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-02.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-03｜前后台恢复同一执行

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付ExecutionRecoveryController。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.execution.recovery` |
| File Lock Key | `mini.execution.recovery` |
| 硬依赖 | MP03-T02-02, MP01-T02-03, MP01-T02-06 |
| 拓扑波次 | W09；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T02-02 → installSnapshot与有序提交；MP01-T02-03 → sendStream和可验证元信息；MP01-T02-06 → ScopeCoordinator.switchTenant

**Produces：**ExecutionRecoveryController

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/features/execution/recovery.ts`
- `N: apps/miniprogram/src/platform/lifecycle.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 进入页先快照后流。
- [ ] 隐藏断开订阅保存最小状态。
- [ ] 回前台按原run恢复。
- [ ] gap/过期游标回快照并有界降级。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 恢复不调用startExecution。
2. 列表卡片不各开长流。
3. 连接状态和任务状态分离。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-04｜任务中心与详情页面

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P07/P08与TaskCard/Timeline。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.execution.ui` |
| File Lock Key | `mini.execution.ui` |
| 硬依赖 | MP03-T01-02, MP03-T01-03, MP03-T02-03, MP01-T01-03 |
| 拓扑波次 | W10；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T01-02 → GET workbench/executions及TaskPage；MP03-T01-03 → GET workbench/inbox与能力原因码；MP03-T02-03 → ExecutionRecoveryController；MP01-T01-03 → MiniNav/AppFrame/SafeBottom

**Produces：**P07/P08与TaskCard/Timeline

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/pages/tasks/index.tsx`
- `N: apps/miniprogram/src/subpackages/execution/pages/detail/index.tsx`
- `N: apps/miniprogram/src/components/execution/Timeline.tsx`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 列表分页筛选前台刷新。
- [ ] 渲染脱敏执行步骤。
- [ ] 独立显示连接执行结算。
- [ ] 按单run capabilities开放操作。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不编造AI进度百分比。
2. 缓存状态有时点。
3. 未开通无权空态区分。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-05｜一次审批与版本冲突

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P09和DecisionController。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.execution.decisions` |
| File Lock Key | `mini.execution.decisions` |
| 硬依赖 | MP03-T01-03, MP03-T02-04 |
| 拓扑波次 | W11；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T01-03 → GET workbench/inbox与能力原因码；MP03-T02-04 → P07/P08与TaskCard/Timeline

**Produces：**P09和DecisionController

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/execution/pages/approval/index.tsx`
- `N: apps/miniprogram/src/features/execution/decisions.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 展示动作对象与影响。
- [ ] 确认前核对有效性版本。
- [ ] 按真实decision类型提交。
- [ ] 冲突刷新并禁止旧决定。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 无永久允许默认项。
2. 双击或他端处理不重复写入。
3. 充值不自动批准外部动作。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-05.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-06｜取消与追加指令

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付sendCancel/sendSteer。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.execution.commands` |
| File Lock Key | `mini.execution.commands` |
| 硬依赖 | MP03-T02-04 |
| 拓扑波次 | W11；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T02-04 → P07/P08与TaskCard/Timeline

**Produces：**sendCancel/sendSteer

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/features/execution/commands.ts`
- `N: apps/miniprogram/src/components/execution/CommandSheet.tsx`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 确认取消不撤销既有副作用。
- [ ] 校验steer并带当前revision。
- [ ] 409刷新后重新由用户确认。
- [ ] 仅合法非终态开放命令。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. onHide不发送cancel。
2. 终态不重发命令。
3. 冲突不能自动带新版本重试危险动作。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-06.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-07｜任务产物安全预览

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P10与SafeArtifactViewer。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.execution.artifacts` |
| File Lock Key | `mini.execution.artifacts` |
| 硬依赖 | MP01-T02-04, MP03-T02-04 |
| 拓扑波次 | W11；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-04 → native multipart/binary下载适配；MP03-T02-04 → P07/P08与TaskCard/Timeline

**Produces：**P10与SafeArtifactViewer

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/execution/pages/artifact/index.tsx`
- `N: apps/miniprogram/src/features/execution/artifacts.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 核实run到session/message产物关联。
- [ ] 限制可渲染类型。
- [ ] 下载前重新鉴权。
- [ ] 清除临时文件及过期地址。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 生成HTML不运行。
2. 不同租户产物被拒。
3. 无永久公开下载缓存。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP03-T02-07.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP03-T02-08｜恢复、审批与幂等集成验收

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付服务端证明同意图同run的证据。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP03-T02 / MP03 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.execution` |
| File Lock Key | `qa.execution` |
| 硬依赖 | MP03-T02-01, MP03-T02-05, MP03-T02-06, MP03-T02-07 |
| 拓扑波次 | W12；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T02-01 → P05和LaunchStateMachine；MP03-T02-05 → P09和DecisionController；MP03-T02-06 → sendCancel/sendSteer；MP03-T02-07 → P10与SafeArtifactViewer

**Produces：**服务端证明同意图同run的证据

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/tests/execution/recovery.integration.test.ts`
- `N: internal/handler/mini_execution_security_test.go`
- `N: docs/miniprogram/evidence/execution.md`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 提交响应丢失后杀进程重入。
- [ ] 注入重复gap与快照变化。
- [ ] 另一设备处理同一审批。
- [ ] 检查外部工具及用量ledger次数。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 同request只对应同run。
2. 冲突不重复执行外部写。
3. 取消和结算各自正确。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP03-T02-08-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T01-01｜商业摘要与套餐只读页

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P15/P16与BalanceDisplay。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T01 / MP04 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.commercial.read` |
| File Lock Key | `mini.commercial.read` |
| 硬依赖 | MP01-T02-01, MP01-T02-06, MP01-T01-03 |
| 拓扑波次 | W07；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T02-01 → createMiniHttpTransport().send；MP01-T02-06 → ScopeCoordinator.switchTenant；MP01-T01-03 → MiniNav/AppFrame/SafeBottom

**Produces：**P15/P16与BalanceDisplay

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/pages/me/index.tsx`
- `N: apps/miniprogram/src/subpackages/account/pages/usage/index.tsx`
- `N: apps/miniprogram/src/features/commercial/view-model.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 按权限读取summary/plans/usage。
- [ ] 金额字符串确定性格式化。
- [ ] 分开展示available/held/refund_locked。
- [ ] 无购买权限保持只读。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. stale与as_of可见。
2. 预占不被当可用余额。
3. 示例价格不进入生产常量。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP04-T01-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T01-02｜通道能力与严格支付契约

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付CheckoutAvailability与获准渠道DTO。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T01 / MP04 |
| Track / 角色 | PAY / 商业化工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `payment.policy` |
| File Lock Key | `payment.policy` / `contracts.checkout` / `sdk.checkout` |
| 硬依赖 | MP00-T02-03, MP04-T01-01 |
| 拓扑波次 | W08；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T02-03 → 逐商品逐终端的通道可用矩阵；MP04-T01-01 → P15/P16与BalanceDisplay

**Produces：**CheckoutAvailability与获准渠道DTO

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/payment/mini_checkout_policy.go`
- `N: packages/contracts/src/miniprogram/checkout.ts`
- `N: packages/api-client/src/miniprogram/checkout.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 组合商品终端主体权限。
- [ ] 默认unavailable并提供原因。
- [ ] 仅注册经核验渠道精确参数。
- [ ] 兼容旧Native/Web消费者。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 未核验前后端均拒买。
2. 没有any强转执行载荷。
3. 旧订单契约仍可读。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP04T0102 的测试组
go test ./internal/... -run '^TestMiniMP04T0102' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T01-03｜获准微信渠道服务端适配

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付经核验provider及参数校验器。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T01 / MP04 |
| Track / 角色 | PAY / 商业化工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `payment.mini-provider` |
| File Lock Key | `payment.mini-provider` |
| 硬依赖 | MP04-T01-02, MP01-T03-02 |
| 拓扑波次 | W09；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP04-T01-02 → CheckoutAvailability与获准渠道DTO；MP01-T03-02 → POST auth/wechat/mini/login

**Produces：**经核验provider及参数校验器

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/payment/wechat_mini_provider.go`
- `N: internal/payment/wechat_mini_provider_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 从绑定身份取正确appid下openid。
- [ ] 按获准通道下单并签名。
- [ ] 保留业务订单与provider幂等关系。
- [ ] 严格校验返回参数和签名一致性。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不拿code_url代替小程序参数。
2. AppKey等机密不外泄。
3. 未知结果对账原订单。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP04T0103 的测试组
go test ./internal/... -run '^TestMiniMP04T0103' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T01-04｜报价支付与订单状态页

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付P17/P18与PaymentAdapter。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T01 / MP04 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.commercial.checkout` |
| File Lock Key | `mini.commercial.checkout` |
| 硬依赖 | MP04-T01-02, MP04-T01-03, MP04-T01-01 |
| 拓扑波次 | W10；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP04-T01-02 → CheckoutAvailability与获准渠道DTO；MP04-T01-03 → 经核验provider及参数校验器；MP04-T01-01 → P15/P16与BalanceDisplay

**Produces：**P17/P18与PaymentAdapter

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/subpackages/account/pages/checkout/index.tsx`
- `N: apps/miniprogram/src/subpackages/account/pages/order/index.tsx`
- `N: apps/miniprogram/src/platform/payment.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 服务端报价确认空间金额有效期。
- [ ] quote_id和稳定幂等键下单。
- [ ] 严格分发获准渠道参数。
- [ ] 收银返回只查询原订单。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. SDK成功不加余额。
2. 过期必须重报价。
3. 取消超时不自动再建单。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP04-T01-04.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T01-05｜回调对账与幂等履约

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付回调/主动查单/履约一致性。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T01 / MP04 |
| Track / 角色 | PAY / 商业化工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `commercial.fulfillment` |
| File Lock Key | `payment.callback` / `commercial.fulfillment` |
| 硬依赖 | MP00-T02-02, MP04-T01-03 |
| 拓扑波次 | W10；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T02-02 → 最小公开回调入口与真实Router测试；MP04-T01-03 → 经核验provider及参数校验器

**Produces：**回调/主动查单/履约一致性

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `A: internal/router/routes_commercial.go`
- `N: internal/payment/mini_reconcile_test.go`
- `N: internal/commercial/mini_fulfillment_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 校验订单金额币种和渠道身份。
- [ ] 支付事实交现有订单服务。
- [ ] paid未履约保留并可靠重试。
- [ ] 测试重复乱序回调退款回收。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 重复通知只发一次权益。
2. 延迟回调可主动对账恢复。
3. 金额不符即使签名正确也不履约。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP04T0105 的测试组
go test ./internal/... -run '^TestMiniMP04T0105' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T01-06｜支付异常与旧渠道回归

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付支付负向矩阵与证据。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T01 / MP04 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.payment` |
| File Lock Key | `qa.payment` |
| 硬依赖 | MP04-T01-04, MP04-T01-05 |
| 拓扑波次 | W11；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP04-T01-04 → P17/P18与PaymentAdapter；MP04-T01-05 → 回调/主动查单/履约一致性

**Produces：**支付负向矩阵与证据

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/tests/commercial/payment.integration.test.ts`
- `N: docs/miniprogram/evidence/payment-matrix.md`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 无资格无权终端不支持。
- [ ] 过期取消未知结果。
- [ ] 延迟回调和履约失败恢复。
- [ ] 重复通知退款后校验权益。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 无资格不能真实收银。
2. 客户端不是权益真值。
3. 旧Web/Native无退化。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T01-06-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T02-01｜订阅授权与通知偏好

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付最小授权记录与用户偏好。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T02 / MP04 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.notifications` |
| File Lock Key | `mini.notifications` |
| 硬依赖 | MP01-T03-04, MP03-T02-04 |
| 拓扑波次 | W11；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T03-04 → P01/P02与AuthStateMachine；MP03-T02-04 → P07/P08与TaskCard/Timeline

**Produces：**最小授权记录与用户偏好

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/notifications.ts`
- `N: apps/miniprogram/src/features/notifications/preferences.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 用户主动动作后请求订阅。
- [ ] 区分允许拒绝失败。
- [ ] 记录模板范围及有效语义。
- [ ] 撤销过期不再假定可发送。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 启动页不批量索权。
2. 拒绝不阻断执行。
3. 本地偏好不等于微信授权额度。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP04-T02-01.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T02-02｜去重出站通知投递

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付授权通知outbox与失败追踪。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T02 / MP04 |
| Track / 角色 | BE / Go后端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `go.notifications` |
| File Lock Key | `go.notifications` |
| 硬依赖 | MP04-T02-01, MP03-T01-03 |
| 拓扑波次 | W12；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP04-T02-01 → 最小授权记录与用户偏好；MP03-T01-03 → GET workbench/inbox与能力原因码

**Produces：**授权通知outbox与失败追踪

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/notification/mini_dispatch.go`
- `N: internal/notification/mini_outbox.go`
- `N: internal/notification/mini_dispatch_test.go`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 权威事件写现有可靠队列或outbox。
- [ ] 用户租户事件模板版本去重。
- [ ] 发送前重核授权和资源。
- [ ] 失败按分类重试或关闭。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 通知失败不回滚任务。
2. 重复事件不重复发送。
3. 正文不含敏感文档工具参数。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# 在已实现本任务测试后，匹配名为 TestMiniMP04T0202 的测试组
go test ./internal/... -run '^TestMiniMP04T0202' -count=1
```

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T02-03｜通知邀请深链安全回跳

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付parseEntryIntent/resolveEntryTarget。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T02 / MP04 |
| Track / 角色 | FE / Taro前端工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `mini.deeplink` |
| File Lock Key | `mini.deeplink` |
| 硬依赖 | MP01-T01-04, MP01-T03-05, MP03-T02-04 |
| 拓扑波次 | W11；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP01-T01-04 → resolveEntry/guardCapability/errorToState；MP01-T03-05 → P19与邀请状态流；MP03-T02-04 → P07/P08与TaskCard/Timeline

**Produces：**parseEntryIntent/resolveEntryTarget

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/src/platform/deep-link.ts`
- `N: apps/miniprogram/tests/platform/deep-link.test.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 白名单解析内部资源目标。
- [ ] 拒绝任意origin及脚本URL。
- [ ] 登录后选择有权空间。
- [ ] 服务端重新验证资源或邀请。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 跨租户不先展示缓存。
2. 任意外部URL不进入路由。
3. 拒绝邀请不留授权。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

```bash
# MP00-T01-02创建实际工作区脚本；测试文件由本任务建立
pnpm --filter @weknora/miniprogram test -- --run tests/tasks/MP04-T02-03.test.ts
pnpm --filter @weknora/miniprogram typecheck
```
共享包改动还需运行其自身测试，以及Web/Expo兼容用例；不可仅运行页面测试。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T03-01｜三端契约与构建回归

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付Web/Expo/Taro兼容报告。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T03 / MP04 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.shared` |
| File Lock Key | `qa.shared` / `ci.shared` |
| 硬依赖 | MP02-T01-04, MP02-T02-03, MP03-T02-08, MP04-T01-06 |
| 拓扑波次 | W13；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T01-04 → 流中断与撤权证据；MP02-T02-03 → ParsingStateMapper与前台轮询；MP03-T02-08 → 服务端证明同意图同run的证据；MP04-T01-06 → 支付负向矩阵与证据

**Produces：**Web/Expo/Taro兼容报告

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: tests/miniprogram/shared-contract-regression.test.ts`
- `N: .github/workflows/miniprogram-shared-regression.yml`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 保留既有DTO回归样例。
- [ ] 运行parser及两条上传分支。
- [ ] 三端typecheck与构建。
- [ ] 校验导出和可选字段兼容。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 旧客户端可忽略新增字段。
2. 原上传流消费者不退化。
3. CI命令对应真实脚本。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T03-01-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T03-02｜跨租户与秘密数据验收

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付接口级多租户安全矩阵。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T03 / MP04 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.security` |
| File Lock Key | `qa.security` |
| 硬依赖 | MP03-T02-08, MP04-T01-06 |
| 拓扑波次 | W13；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP03-T02-08 → 服务端证明同意图同run的证据；MP04-T01-06 → 支付负向矩阵与证据

**Produces：**接口级多租户安全矩阵

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: internal/handler/mini_security_matrix_test.go`
- `N: docs/miniprogram/evidence/security.md`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 替换tenant/run/session/file/order ID。
- [ ] 切空间延迟旧异步回包。
- [ ] 扫描包日志缓存中的秘密。
- [ ] 测试绑定重放撤权下载。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 请求头不能自行授权。
2. Owner不默认读私有资源。
3. 清缓存不是唯一安全控制。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T03-02-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T03-03｜视觉还原和可访问性验收

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付20页真实Taro截图与差异清单。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T03 / MP04 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.visual` |
| File Lock Key | `qa.visual` |
| 硬依赖 | MP02-T01-04, MP02-T02-04, MP03-T02-08, MP04-T01-06 |
| 拓扑波次 | W13；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP02-T01-04 → 流中断与撤权证据；MP02-T02-04 → P14与AuthorizedDocumentPreview；MP03-T02-08 → 服务端证明同意图同run的证据；MP04-T01-06 → 支付负向矩阵与证据

**Produces：**20页真实Taro截图与差异清单

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: apps/miniprogram/tests/visual/page-matrix.json`
- `N: docs/miniprogram/evidence/visual.md`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 逐页正常和关键异常截图。
- [ ] 320/375/390/430及大字体。
- [ ] 测触点对比度截断安全区。
- [ ] 核对token与危险按钮语义。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 不拿截图当整页实现。
2. 状态不只靠颜色。
3. 胶囊键盘不遮挡主操作。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T03-03-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T03-04｜真机性能弱网与后台

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付设备/基础库/性能证据。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T03 / MP04 |
| Track / 角色 | QA / 测试工程师 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `qa.device` |
| File Lock Key | `qa.device` |
| 硬依赖 | MP04-T03-01, MP04-T03-03 |
| 拓扑波次 | W14；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP04-T03-01 → Web/Expo/Taro兼容报告；MP04-T03-03 → 20页真实Taro截图与差异清单

**Produces：**设备/基础库/性能证据

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: docs/miniprogram/evidence/device-matrix.md`
- `N: apps/miniprogram/tests/device/scenarios.json`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 固定冷启动与首帧测量条件。
- [ ] 跑长对话大列表文件弱网。
- [ ] 隐藏杀进程重新登录恢复。
- [ ] 记录包体内存监听队列。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 流缓冲不无限增长。
2. 返回前台不重启任务。
3. 阈值由基线设定并实际测量通过。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T03-04-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T03-05｜隐私内容和上线资格

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付正式条款、审核证据与开关。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T03 / MP04 |
| Track / 角色 | OPS / 发布/运营负责人 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `ops.compliance` |
| File Lock Key | `ops.compliance` |
| 硬依赖 | MP00-T02-03, MP04-T03-02 |
| 拓扑波次 | W14；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP00-T02-03 → 逐商品逐终端的通道可用矩阵；MP04-T03-02 → 接口级多租户安全矩阵

**Produces：**正式条款、审核证据与开关

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: docs/miniprogram/release-compliance.md`
- `N: apps/miniprogram/src/config/legal.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 核实主体类目备案及适用运营要求。
- [ ] 上线真实隐私服务删除举报入口。
- [ ] 验证按需权限用途。
- [ ] 支付未获资格保持关闭。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 占位条款不进入生产。
2. 拒绝非必要授权仍可基本使用。
3. 不确定资格关闭而非绕过。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T03-05-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。


# MP04-T03-06｜灰度发布与回滚演练

> 状态：planned。本文件描述后续生产开发，不代表本次已实现或已测试。

**目标：**交付受控发布包与回滚手册。

| 调度项 | 约束 |
|---|---|
| 父任务 / 阶段 | MP04-T03 / MP04 |
| Track / 角色 | OPS / 发布/运营负责人 |
| Repository | 1123786563/WeKnora-fork01 |
| Module Key | `ops.release` |
| File Lock Key | `ops.release` / `ci.mini` |
| 硬依赖 | MP04-T03-04, MP04-T03-05, MP04-T02-02, MP04-T02-03 |
| 拓扑波次 | W15；不是工期 |
| 并行约束 | 依赖已合并且文件锁不冲突才可合并推进；契约冻结后可提前用独立fixture开发UI |

## 输入与输出

**Consumes：**MP04-T03-04 → 设备/基础库/性能证据；MP04-T03-05 → 正式条款、审核证据与开关；MP04-T02-02 → 授权通知outbox与失败追踪；MP04-T02-03 → parseEntryIntent/resolveEntryTarget

**Produces：**受控发布包与回滚手册

## 文件范围

A表示基线中的适配目标；N为本方案建议新增路径。新目录最终按MP00源码差异确认，不自动创建第二套同名服务。

- `N: .github/workflows/miniprogram-release.yml`
- `N: docs/miniprogram/release-runbook.md`
- `N: apps/miniprogram/src/config/environment.ts`

## 步骤与验证闭环

- [ ] 领取文件锁，核对依赖的合并提交；阅读本任务对应详细设计与页面。
- [ ] 将下方三条验收条件写成有明确输入、操作、结果的失败测试；保留首次失败证据。
- [ ] 隔离dev/staging/prod固定域名。
- [ ] 检查SSE缓冲和空闲超时。
- [ ] 小范围灰度观察错误恢复指标。
- [ ] 演练关闭写功能与保留对账。
- [ ] 跑本任务测试、同模块回归与类型/构建检查，保存完整命令、退出码和日志。
- [ ] 核对权限、错误映射、状态及命名；提交独立PR，不顺带做无关重构。

## 验收条件

1. 机密只在服务端受控CI。
2. 关购买不关已付订单对账。
3. 回滚兼容已有端与进行中任务。

## 测试定位

这些是实施时应新增的测试目标；不是本交付包已经存在的生产测试。

交付命名为 `MP04-T03-06-evidence.md` 的证据，包含输入环境、可复現步骤、期望/实际结果、日志和判定人；纯截图不能代替服务端安全或账务证据。

## 完成门禁

所有验收通过；依赖和锁记录准确；无新增机密泄露；无未解释兼容变化；接口/页面/测试台账同步。只有取得真实测试证据并合并后，才把状态改为 `verified` / `merged`。

变更范围外的问题登记到差异台账，不使用“顺手修复”覆盖其他任务的文件。
