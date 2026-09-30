# Issue #82 复验第 2 轮（reverify-2）— 第 2 轮裁定后全流程重验

- 日期：2026-09-25（第 2 轮裁定提交 `eea89716e` 之后）
- 分支/代码：`codex/issue-72-lago` HEAD=`eea89716e`（本轮 diff 仅 ledger 裁定 + docs，产品代码与 59465f8fe 相同——裁定「无产品缺陷」，行动为共享栈遗留清理与自验流程规则固化）。复验前旧二进制按 PID 终止，后端 :8092 以 HEAD 重编译启动（日志 /tmp/issue82-backend-rv2.log）
- 复验者：流程验证员-82，独立重跑（不复用修复员补跑结果）
- 环境：延续真实栈——Lago `weknora-lago-82flow`（:48889 v1.53.0 全 healthy）、前端 :5192、alipay/wechat stub :8292/:8291、sqlite `data/issue82-flow.db`
- 测试数据：**单租户 5（issue82-flow-e@verify.local）承载双腿**——遵循 ledger 保留项警告（新租户 id ≥5 且 ≠6；本轮用 5，浏览器 wechat 腿先建 ord_ee4a8f2453cc1ddc，支付宝腿后建 ord_dcfbbc0cfdb3973d）

## 第 2 轮清理效果核验（裁定行动项）

| 项 | 结果 | 证据 |
|---|---|---|
| Lago customers 读面恰余 tenant-1/2/3/4（tenant-6/probe 顾客已删、tenant-3 名回真 issue82-c's Workspace） | ✅ | `rv2-00-baseline-lago-customers.txt` |

## 全流程重验结果

| # | 断言 | 结果 | 证据 |
|---|---|---|---|
| 1 | 浏览器腿（租户 5 首购，Lago customer 全新走 create 分支）：checkout 待付款+报价明细+Billing 套餐行 | ✅ 5/5 | `rv2-01/rv2-02` 截图（browser_flow_82.mjs） |
| 2 | 支付宝下单 → 201 + checkout_url 二维码（stub 验签往返，¥99.00 一致） | ✅ | `rv2-api-02-purchase-alipay.json`（ord_dcfbbc0cfdb3973d，out_trade_no=mo_1404993ee6e52487） |
| 3 | 匿名签名 notify → 200 + 响应体恰 `success` | ✅ | `rv2-callback-anon-replay-negative.txt` |
| 4 | 订单 paid（payment=paid/state=paid）+ attempt.merchant=2088000000000000(SellerID)+succeeded | ✅ | 同上 |
| 5 | outbox 本轮订单恰 1（fulfill:ord_dcfbbc0cfdb3973d；库内另 1 条为上轮 D 单历史） | ✅ | 直查 sqlite（event_key 列表） |
| 6 | 重放同一 notify → 仍 success，outbox/fulfillment 计数零新增 | ✅ | 同上（2/1 → 2/1） |
| 7 | 伪造签名负控（篡改金额+假 sign）→ failure 401 未入账 | ✅ | 同上 |
| 8 | 付款后浏览器呈现：checkout「已付款，权益处理中」、无虚假已生效、Billing 不虚报 | ✅ 4/4 | `rv2-03/rv2-04` 截图（browser_paid_face_82.mjs） |
| 9 | Lago 四对象：订阅 incomplete、wallets=0、entitlements 404、无 succeeded payment（D2 冻结边界） | ❌ 维持冻结 | `rv2-lago-after-paid.txt`（failed=6/requires_action=6 无 succeeded） |
| 10 | GET /purchase 读面 | 说明 | 单租户双订单下 CurrentPurchaseOrder 选中未付款的 wechat 单（ord_ee4a8f2453cc1ddc，pending）——读面选择行为非缺陷；alipay 单 paid 由 GET /orders/:id 直证（同文件） |
| 11 | 回归（commercial+handler+router+middleware） | ✅ | `rv2-gotest.txt` |
| 12 | 红线四项 | ✅ | `rv2-redline.txt` |

## 结论（与第 1 轮复验一致，无回归）

三缺陷修复持续有效（首购 201 / 匿名回调 success / merchant 身份正确入账）；「可信回调入账→订单 paid→幂等」链路端到端可用。激活后半链（已生效/Credits/Entitlement/Lago Payment 关联）仍受 `d2-falsified-t10-p2-fail` 冻结约束不可达（第 2 轮裁定核实维持）。AC4 沙箱凭据披露不变。本轮无新增产品缺陷。
