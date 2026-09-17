export function errorMessage(error:unknown):string{
  if(error instanceof Error&&error.name==='AbortError')return '操作已中断。离开页面不会取消后台任务。';
  const e=error!==null&&typeof error==='object'?error as {status?:number;code?:string}:{};
  if(e.status===401)return '登录已失效，请重新登录。';
  if(e.status===403)return '你没有访问当前空间资源的权限。';
  if(e.status===404||e.status===501||e.status===503)return '此资源不存在，或当前部署尚未开放该能力。';
  if(e.status===409)return '状态已被其他操作更新，请刷新后重新确认。';
  if(e.status===410)return '当前操作已过期，请重新获取。';
  if(e.status===429)return '请求较多，请稍后重试。';
  if(e.code==='TIMEOUT'||e.code==='STREAM_TIMEOUT')return '等待超时，请先查询原操作状态，不要重复提交。';
  if(error instanceof Error&&error.message==='API_ORIGIN_MISSING')return '服务地址未配置，请由维护人员完成小程序构建配置。';
  if(error instanceof Error&&error.message==='AUTH_REQUIRED')return '请先登录，再访问你的工作空间。';
  if(error instanceof Error&&error.message==='SCOPE_CHANGED')return '工作空间已切换，本次旧请求已丢弃。';
  return '操作未完成，请检查网络或刷新状态后重试。';
}
