import type { OrderView } from '@weknora/contracts';
export function orderMessage(order:OrderView):string {
 if(order.fulfillment==='fulfilled') return '权益已生效';
 if(order.payment==='paid') return '已付款，权益处理中';
 if(order.payment==='closed') return '订单已关闭';
 return '等待付款';
}
