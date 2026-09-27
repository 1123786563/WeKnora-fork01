import type { OrderView, PurchaseView } from '@weknora/contracts';
export function orderMessage(order:OrderView):string {
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
