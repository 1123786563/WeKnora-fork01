import type { OrderView, PurchaseView } from '@weknora/contracts';
export function orderMessage(order:OrderView):string {
 // (#84 / AC4) 付款异常是独立闭合状态（spec L169 operator attention）：
 // 后端不变量保证 attention 只与 payment=pending 成对出现（与已付款/
 // 已生效互斥），分支置于最前以防御后端误发错位组合。文案不含平台词汇
 // （spec L210）。
 if(order.fulfillment==='attention') return '付款异常（资金事实已记录，待处理）';
 if(order.fulfillment==='fulfilled') return '权益已生效';
 if(order.payment==='paid') return '已付款，权益处理中';
 if(order.payment==='closed') return '订单已关闭';
 return '等待付款';
}

// (#82 Task 15 / D15-f) The ONE shared purchase-state vocabulary (spec L169
// closed product set): the Billing row suffix and any future purchase-state
// surface consume this map — no page keeps a private copy of the words
// (absent renders no suffix by construction: callers skip the empty label).
export const PURCHASE_STATE_LABEL: Record<PurchaseView['state'], string> = {
 awaiting_payment: '待付款（权益未开放）',
 paid_awaiting_activation: '已付款待激活',
 active: '已生效',
 canceled: '该购买已取消，请重新发起购买',
 absent: '',
};
