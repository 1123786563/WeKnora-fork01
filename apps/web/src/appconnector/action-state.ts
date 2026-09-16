export function actionMessage(state:string):string {
 if(state==='unknown') return '正在核对外部执行结果';
 if(state==='awaiting_approval') return '等待操作批准';
 if(state==='authorized') return '已批准';
 if(state==='queued') return '已入队';
 if(state==='dispatched') return '已派发';
 if(state==='succeeded') return '操作已完成';
 if(state==='failed') return '操作失败';
 return '操作处理中';
}
