export function refundMessage(state:string):string {
 if(state==='completed') return '退款与权益调整已完成';
 if(state==='revocation_pending') return '退款已完成，权益调整处理中';
 if(state==='refund_unknown') return '退款结果核对中';
 if(state==='rejected') return '退款申请未通过';
 return '退款处理中';
}
