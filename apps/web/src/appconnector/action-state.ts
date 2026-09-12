export function actionMessage(state:string):string {
 if(state==='unknown') return '正在核对外部执行结果';
 if(state==='awaiting_approval') return '等待操作批准';
 if(state==='succeeded') return '操作已完成';
 return '操作处理中';
}
