/* Local demonstrator only. No authentication, network, worker or billing implementation. */
const WorkbenchModel = (() => {
  const space = (populated) => ({draft:populated?'汇总本周客户反馈，提炼主要问题，并生成一份处理建议。':'',budget:'100',agent:'general',target:'platform',knowledge:false,attachment:false,attachmentReady:false,approval:'pending',revision:4,run:'waiting_user',execution:'connected',settlement:'pending',populated,messages:[],title:'客户反馈分析',starred:false,connected:true,voiceText:'',voiceStatus:'idle'});
  const initial=()=>({version:2,page:'home',space:'team',scenario:'normal',theme:'light',large:false,spaces:{team:space(true),personal:space(false)},requests:[],decisions:[],generation:1,notifications:true,message:'',counter:0});
  const cp=s=>JSON.parse(JSON.stringify(s));
  const current=s=>s.spaces[s.space];
  const writable=s=>!['offline','forbidden','loading','error'].includes(s.scenario);
  function submit(input){const s=cp(input),v=current(s);s.message='';if(!writable(s)){s.message='当前无法提交，草稿已保留。';return s;}const unresolved=s.requests.find(r=>r.space===s.space&&r.status==='unknown');if(unresolved){s.page='run';s.message='原请求仍在核实，没有创建新请求。';return s;}
    if(!v.draft.trim()){s.message='请填写任务描述。';return s;}if(!Number.isSafeInteger(Number(v.budget))||Number(v.budget)<=0){s.message='预算上限须为正整数。';return s;}if(v.attachment&&!v.attachmentReady){s.message='附件校验完成后才能提交。';return s;}
    const num=++s.counter; s.requests.push({id:`demo-request-${String(num).padStart(3,'0')}`,runId:`demo-run-${String(num).padStart(3,'0')}`,space:s.space,status:s.scenario==='unknown'?'unknown':'admitted',text:v.draft,budget:Number(v.budget)});v.populated=true;v.title=v.draft.trim().slice(0,18);v.run=s.scenario==='unknown'?'unknown':'waiting_user';v.approval='pending';v.revision=4;s.page=s.scenario==='unknown'?'run':'conversation';return s;}
  function reconcile(input){const s=cp(input);if(!writable(s))return s;const r=s.requests.find(r=>r.space===s.space&&r.status==='unknown');if(r){r.status='admitted';current(s).run='waiting_user';}s.scenario='normal';s.message='演示：已查到原 Run，未重复启动。';return s;}
  function switchSpace(input,next){const s=cp(input);if(!s.spaces[next])return s;s.generation++;s.space=next;s.page='home';s.message='已切换空间；服务端任务不会因此取消。';return s;}
  function decide(input,action,revision){const s=cp(input),v=current(s);if(!writable(s))return s;if(s.scenario==='conflict'||revision!==v.revision){s.message='内容版本已改变，请刷新后重新核对。';return s;}if(v.approval!=='pending')return s;if(!['approve','reject'].includes(action))return s;v.approval=action==='approve'?'approved':'rejected';v.revision++;v.run=action==='approve'?'reconciling':'waiting_user';s.decisions.push({space:s.space,action,revision});s.message=action==='approve'?'已记录模拟批准；不代表外部写入已成功。':'已拒绝这项具体操作。';return s;}
  function cancel(input){const s=cp(input);if(!writable(s))return s;current(s).run='cancel_requested';current(s).execution='stop_pending';s.message='停止请求已提交，仍待执行方确认。';return s;}
  return {initial,current,writable,submit,reconcile,switchSpace,decide,cancel};
})();
if (typeof module !== 'undefined' && module.exports) module.exports=WorkbenchModel;
