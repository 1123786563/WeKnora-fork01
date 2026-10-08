# 计费数据最小化、凭据隔离与 AGPL 上线门（#101 / Lago 29 审计记录）

状态：**生产未批准（gate 关闭）**。本文件是 #101 的可审计结论（AC④），不是法务批准。
法务审查拥有者为仓库所有者；在此之前生产环境不得启用 Lago 商业栈（deploy/lago/README.md「Compliance gate」节同口径）。

## AC① 密钥仅在服务端 secret 管理

| 凭据族 | 证据 | 说明 |
|---|---|---|
| Lago 平台凭据 | `internal/commercial/commercialplatform/config.go:26-30`（WEKNORA_COMMERCIAL_PLATFORM_API_KEY 等，env-only；常量拆串防扫描） | 空值=fail-closed（unconfigured），无默认回退 |
| 支付渠道凭据 | `internal/commercial/payment/providers_env.go:20-55`（alipay/wechat 全走 env 引用，源码零字面量）；`config.go:44-70`（Stripe 家族同纪律） | blocked-env 合法（无凭据即无通道），checkout/callbacks fail-closed |
| Webhook 验签密钥 | `internal/container/container.go:2841`（LAGO_WEBHOOK_SECRET env-gated，#98） | 未配置时验签面 fail-closed |
| 前端/移动端 | `apps/web`、`apps/mobile` grep `LAGO_|commercial.*key` 零命中 | 商业凭据不出服务端 |

残差：外部 secret manager（Vault/KMS）未接——spec 允许「服务端 env 或同名 secret service」，生产部署前以 secret service 承接同名变量即可，**属部署前提而非代码缺口**。

## AC② 敏感内容不进 Lago metadata

审计对象=实际跨越 Lago 边界的全部 payload（usage event 通道不存在——settlement 走 openmeter 轨道，见残差）：

| 通道 | 跨越字段 | 判定 |
|---|---|---|
| 钱包创建/更新 | `commercialplatform/lago.go:998-1003` metadata 仅 `{weknora_customer: externalCustomerID, period}`（关联身份+周期，spec L178 明示允许） | ✅ 最小 |
| 客户/购买订阅 | `lago_purchase.go:277,397,780` external_id、metadata[weknora_customer]（关联身份） | ✅ 最小 |
| 用量事实（本地） | `repository/commercial/usage.go:32-47` UsageRow 全字段：tenant/call/attempt/revision/run/delegation/funding/service/price_version/occurred/dimensions/status/charge——**无 prompt、无输出、无知识内容** | ✅ 最小 |
| Webhook 入站（#98 通道） | `repository/commercial/projection.go:15-42` inbox 行=provider/event_id/kind/external_id/tenant；**raw body 不留存**；audit 行 detail=动作+游标 | ✅ 最小 |

## AC③ 客户端/日志/Task 不泄凭据

| 面 | 证据 |
|---|---|
| 响应卫生 | handler/commercial.go：PlatformReadiness 错误→闭合 token（unconfigured/unreachable/invalid_response），err.Error() 永不上 wire；orderWire/purchaseWire/quoteWire 同纪律（#78/#84 轮已有 go+前端 13 用例） |
| 日志 | FulfillmentExceptionRow（service/commercial/fulfillment.go:59-73）sanitized，EventKey json:"-"；grep commercial 域日志无密钥输出 |
| Task/Agent | agentruntime/commercial_adapter.go 仅映射闭合错误（ErrInsufficientBudgetGate 等）；UsageFact 无内容字段，Task 面无凭据可达 |

## Community/Premium 功能清单（WeKnora 实际调用的 Lago API 面）

adapter 调用面（grep `a.do(ctx, http.Method` 全量）：

| Lago 资源 | 方法 | WeKnora 用途 | 版图 |
|---|---|---|---|
| /customers | POST/PUT/GET | 空间→Lago 客户映射 | Community（OSS 核心） |
| /plans | POST/GET | 套餐目录发布/读取 | Community |
| /subscriptions | POST/GET | 月度/购买订阅两轨 | Community |
| /wallets | POST/PUT/GET | Credits 钱包（月度+充值） | Community |
| /billable_metrics | GET | 定价维度校验 | Community |
| /invoices | GET | 购买发票读取（fees/支付状态） | Community |
| /features | POST | 权益特性声明 | **待法务核对**（Lago 后期 entitlements/features 属商业版范畴；pinned v1.53.0 需逐项核） |
| webhook 入站 | — | 变更通知（HMAC/JWT） | Community |

结论：除 `/features` 一处待核，WeKnora 未消费任何 Lago 商业版功能；**Premium 功能控制不可绕过**的义务在法务轮一并核验（若 /features 判属商业版，替换为 Community 等价面或购买许可）。

## AGPL 生产批准记录（AC④）

| 项 | 状态 |
|---|---|
| Lago AGPL-3.0 义务审查（self-host + 是否改源） | **未执行**——拥有者：仓库所有者/法务。fork 未修改 Lago 源（仅 API 消费）；Helm 部署形态见 #103 |
| Community/Premium 清单 | ✅ 本文件上一节（首版） |
| 生产启用 | **禁止**，直至上项闭环；本地/CI pinned Compose 栈不受限 |
| 复核触发 | usage event 通道落地（Lago events API）时 AC② 增补审计；/features 判定后更新清单 |

## 残差

- **R-101a**：法务批准为人类门禁——本轮交付可审计记录，gate 保持关闭（非代码缺口）。
- **R-101b**：外部 secret manager 未接（部署前提，同名单承接）。
- **R-101c**：Lago usage event 通道不存在（settlement 走 openmeter）；该通道落地票须补 events payload 最小化审计。
- **R-101d**：/features 的 Lago 版图归属待法务核对（pinned v1.53.0）。
