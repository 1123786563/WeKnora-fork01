"""Rebuild all standalone pages. Python stdlib only; no external assets."""
from pathlib import Path
import json,re
R=Path(__file__).resolve().parent
base=(R/'src/wireframe-base.js').read_text()

# Initialize each standalone page explicitly rather than falling back to Home.
base=base.replace("$('#stage-label')", "$('#page-name')")
base=base.replace("document.body.dataset.initial||'home'", "window.INITIAL_PAGE||document.body.dataset.initial||'home'")
base=base.replace("state.selection==='我的个人空间'?'Owner':'Member'", "state.selection==='我的个人空间'?'所有者':'成员'")
base=base.replace('<div class="modal"><h3>', '<div class="modal" role="dialog" aria-modal="true" aria-labelledby="dialog-title"><h3 id="dialog-title">')
base=base.replace("state.invitation='accepted';state.selection='示例团队';go('workspace');toast('模拟接受邀请；正式版需后端验证');", "state.invitation='accepted';go('workspace');toast('邀请已接受（模拟）；请选择空间。');")

hf=(R/'src/high-fidelity.js').read_text()
hf=hf.replace('statusInfo(status)','hfStatus(status)').replace('statusInfo(state.status)','hfStatus(state.status)').replace('data-filter=','data-value=').replace("state.approval==='conflict'","state.approval==='stale'").replace("state.network?'disconnect':'reconnect'","'network'").replace("'','danger','cancel'","'','danger','cancel-task'").replace('data-act="complete"','data-act="demo-finish"').replace('data-act="approval-conflict"','data-act="approval-stale"')
hf=hf.replace("[['我的个人空间','只属于你的资料与任务','me','所有者'],['青禾产品团队','团队知识与日常协作','users','成员']]","hfWorkspaceOptions()")
hf=hf.replace('<h1 style=\"font-size:22px;margin-top:20px\">青禾产品团队</h1>','<h1 style=\"font-size:22px;margin-top:20px\">云杉创新项目</h1>')
hf+='\nfunction hfWorkspaceOptions(){const a=[[\"我的个人空间\",\"只属于你的资料与任务\",\"me\",\"所有者\"],[\"青禾产品团队\",\"团队知识与日常协作\",\"users\",\"成员\"]];if(state.invitation===\"accepted\")a.push([\"云杉创新项目\",\"刚刚接受邀请的工作空间\",\"users\",\"成员\"]);return a}\n'
base=base.replace('const views=',hf+'\nconst views=',1)
base=base.replace('示例团队','青禾产品团队').replace('整理本周销售周报，汇总重点进展和待跟进事项。','整理本周项目进展，生成包含风险与下周计划的团队周报。').replace('离线线框','高保真原型').replace('小程序线框','小程序高保真').replace('本线框','本原型')
base=base.replace("switch(name){","""switch(name){
case 'home-preset':$('#home-prompt').value=el.dataset.text;$('#home-prompt').focus();break;
case 'kb-filter':state.kbFilter=el.dataset.value;render('knowledge');break;
case 'quote-toggle':state.quoteExpired=!state.quoteExpired;render('checkout');break;
case 'parse-complete':state.uploadStage='ready';render('upload');break;
""",1)
base=base.replace("case 'pay':if(!state.purchaseEnabled)","case 'pay':if(!state.purchaseEnabled||state.quoteExpired)")
base=base.replace("state.taskText=text;state.status='running';", "state.taskText=text;state.requestId=state.requestId||'demo-intent-001';state.launchCount=(state.launchCount||0)+1;state.status='running';")
base=base.replace("case 'approve':modal(","case 'approve':if(state.approval!=='pending'){toast('审批已处理，请刷新');break}modal(")
base=base.replace("case 'approve-confirm':state.approval=", "case 'approve-confirm':if(state.approval!=='pending'){toast('版本已变化，请重新读取审批');break}state.approval=")
# Replace demonstration download explanation with an actual local-only sample file.
old="case 'download':modal('授权文件下载','正式版经现有授权资源接口下载到微信临时文件，再使用支持的预览能力。本原型不下载真实文档。');break;"
new="case 'download':{const u=URL.createObjectURL(new Blob(['# 示例资料\\n\\n本文件为虚构演示，不包含任何真实用户资料。\\n'],{type:'text/markdown;charset=utf-8'}));const a=document.createElement('a');a.href=u;a.download='WeKnora-示例资料.md';a.click();setTimeout(()=>URL.revokeObjectURL(u),1000);toast('已生成本地示例文件，未访问真实资源。');break;}"
base=base.replace(old,new)
# Member switch clears local demonstration states and pending output.
base=base.replace("case 'enter-space':state.tenant=", "case 'enter-space':clearInterval(typingTimer);typingTimer=null;state.requestId=null;state.launchCount=0;state.uploadSelected=false;state.quoteExpired=false;state.tenant=")
base=base.replace("case 'logout-confirm':state.userMessages=[];state.assistantMessage='';go('login');break;", "case 'logout-confirm':clearInterval(typingTimer);typingTimer=null;state.userMessages=[];state.assistantMessage='';state.requestId=null;state.launchCount=0;state.purchaseEnabled=false;state.uploadSelected=false;state.uploadStage='idle';state.orderPayment='pending';state.orderFulfillment='pending';go('login');break;")
# Safe free-text search for refined cards, plus modal keyboard trap.
extra="""
document.addEventListener('input',e=>{if(e.target.id==='agent-search'){$('#agent-cards').innerHTML=agentCards(e.target.value.trim())}if(e.target.id==='knowledge-search'){$('#hf-kb-cards').innerHTML=hfKnowledgeCards(e.target.value.trim())}});
document.addEventListener('keydown',e=>{let m=$('#modal .modal');if(!m)return;if(e.key==='Escape'){$('#modal').innerHTML='';return}if(e.key==='Tab'){let q=[...m.querySelectorAll('button:not([disabled]),input,textarea')],f=q[0],l=q.at(-1);if(e.shiftKey&&document.activeElement===f){e.preventDefault();l.focus()}else if(!e.shiftKey&&document.activeElement===l){e.preventDefault();f.focus()}}});
window.__prototype={getState:()=>JSON.parse(JSON.stringify(state)),navigate:go};
"""
mapping=json.loads((R/'developer-mapping.json').read_text()) if (R/'developer-mapping.json').exists() else {}
extra+='\nconst hfMapping='+json.dumps(mapping,ensure_ascii=False)+';\nconst originalRender=render;render=function(id){originalRender(id);let m=hfMapping[id];if(m){$(\"notes\".startsWith(\"#\")?\"notes\":\"#notes\").insertAdjacentHTML(\"beforeend\",\"<h3>开发映射</h3><div class=hf-route>\"+m.route+\"</div><p>\"+m.components+\"</p><div class=hf-tasks-ref>\"+m.tasks.join(\"<br>\")+\"</div><h3>设计令牌</h3><p>主题 Quiet Work · color.action.primary<br>radius.lg / space.16 / font.size.body</p>\")}document.querySelectorAll(\".modal\").forEach(x=>{x.setAttribute(\"role\",\"dialog\");x.setAttribute(\"aria-modal\",\"true\")})};\n'
base=base.replace('let lastGroup=',extra+'\nlet lastGroup=',1)
(R/'src/app.js').write_text(base)
css=(R.parent/'design/tokens.css').read_text()+(R/'src/wireframe-base.css').read_text()+(R/'src/high-fidelity.css').read_text()+'.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap}\n'
(R/'src/app.css').write_text(css)
pages=json.loads((R/'page-manifest.json').read_text())
def page(id):
 return '<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover"><title>WeKnora 高保真原型</title><style>'+css+'</style></head><body><header class="top"><div class="brand"><div class="brandmark">W</div><div><h1>WeKnora · Quiet Work</h1><p>微信小程序高保真设计 · 20 个页面 · 全部为虚构示例数据</p></div></div><span class="pillnote">设计交付 v1.0 · 非生产应用</span></header><div class="studio"><aside class="nav" id="nav" aria-label="20页原型导航"></aside><section class="stage"><div class="stage-caption"><strong id="page-name"></strong><span id="page-count"></span></div><div class="phone" id="phone"></div><p class="bottomcaption">可点击交互原型 · 未连接真实后端<br>微信状态栏和胶囊仅作位置占位，实机需读取真实尺寸。</p></section><aside class="notes" id="notes" aria-label="开发标注"></aside></div><script>window.INITIAL_PAGE='+json.dumps(id)+';\n'+base.replace('</script','<\\/script')+'</script></body></html>'
(R/'index.html').write_text(page('home'))
for p in pages:
 p['file']=p['n']+'-'+p['id']+'.html'
 (R/'pages'/p['file']).write_text(page(p['id']))
(R/'page-manifest.json').write_text(json.dumps(pages,ensure_ascii=False,indent=2)+'\n')
(R.parent/'WeKnora-HiFi.html').write_text(page('home'))
print('20 independently initialized pages + review index generated')
