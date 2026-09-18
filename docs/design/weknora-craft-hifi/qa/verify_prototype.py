"""Offline prototype behavior checks. Not production React/Go integration tests.
The environment blocks file:// navigation, so Chromium renders the exact bundled
HTML via set_content. No server, remote assets, credentials or API calls.
"""
from pathlib import Path
from playwright.sync_api import sync_playwright
import json, time, traceback, os, shutil
R=Path(__file__).resolve().parents[1]
HTML=(R/'index.html').read_text()
results=[]; errors=[]; network=[]
def snap(p): return p.evaluate('window.CraftDemo.snapshot()')
def click(p,action): p.locator(f'[data-action="{action}"]').filter(visible=True).first.click()
def shot(p,name):
 p.wait_for_timeout(100);p.screenshot(path=str(R/'screenshots'/name),full_page=True)
def check(name,fn):
 t=time.monotonic()
 try:
  fn();results.append(dict(name=name,status='passed',seconds=round(time.monotonic()-t,3)))
 except Exception as e:
  results.append(dict(name=name,status='failed',error=str(e),seconds=round(time.monotonic()-t,3)))
  print('FAILED',name,str(e)[:300])
with sync_playwright() as pw:
 browser=pw.chromium.launch(executable_path=os.environ.get('CHROMIUM_PATH') or shutil.which('chromium'),headless=True,args=['--no-sandbox'])
 context=browser.new_context(viewport={'width':1440,'height':1000},device_scale_factor=1,accept_downloads=True)
 def page(width=1440,height=1000):
  p=context.new_page();p.set_viewport_size({'width':width,'height':height});p.set_default_timeout(4000)
  p.on('pageerror',lambda e:errors.append(str(e)));p.on('request',lambda r:network.append(r.url))
  p.set_content(HTML,wait_until='load');p.wait_for_timeout(30);return p
 def work(p,kind='web'):
  ids={'web':'p1','document':'p2','spreadsheet':'p3','slides':'p4'}
  if kind=='slides': p.locator('.nav [data-route="library"]').click()
  p.locator('.artifact-card[data-id="'+ids[kind]+'"]').click()
 def scenario(p,which):
  p.locator('.topbar [data-action="scenario"]').click();p.locator(f'#drawer [data-scenario="{which}"]').click()
 def isolated(name,fn):
  def run():
   p=page()
   try:fn(p)
   finally:p.close()
  check(name,run)
 def assert_(condition,message='assertion failed'):
  if not condition:raise AssertionError(message)
 isolated('01 首页主体与继承品牌颜色',lambda p:(assert_(p.locator('h1').inner_text()=='把知识，变成作品。'),assert_(p.locator('.brand').evaluate("e=>getComputedStyle(e).color")=='rgb(0, 82, 217)'),shot(p,'01-home.png')))
 isolated('02 空目标不可创建Run',lambda p:(click(p,'create'),assert_(snap(p)['route']=='home'),assert_('请先描述' in p.locator('#toast').inner_text())))
 isolated('03 模板填充目标与类型但不执行',lambda p:(p.locator('.template-item[data-kind="document"]').click(),assert_(snap(p)['kind']=='document'),assert_('企业知识库' in p.locator('#home-prompt').input_value()),assert_(snap(p)['route']=='home')))
 def create(p):
  p.locator('#home-prompt').fill('生成一份产品经营报告');click(p,'create');assert_(snap(p)['versions']==[]);assert_(snap(p)['scenario']=='running');assert_(p.locator('.topbar [data-action="export"]').is_disabled());click(p,'finish');assert_(snap(p)['baseVersion']==1);assert_(len(snap(p)['versions'])==1)
 isolated('04 首次创建无虚构历史且交付v1',create)
 def filters(p):
  p.locator('.nav [data-route="library"]').click();assert_(p.locator('.artifact-card').count()==6);p.locator('[data-filter="document"]').click();assert_(p.locator('.artifact-card').count()==2);p.locator('#project-search').fill('不存在的名称');assert_(p.locator('.empty').count()==1);click(p,'clear-filter');assert_(p.locator('.artifact-card').count()==6);shot(p,'02-library.png')
 isolated('05 作品库过滤搜索与空态清除',filters)
 def modify(p):
  work(p);v=snap(p)['versions'];p.locator('#composer').fill('补充区域筛选');click(p,'send');assert_(snap(p)['runCount']==2);assert_(len(snap(p)['versions'])==3);click(p,'finish');s=snap(p);assert_(s['baseVersion']==4 and len(s['versions'])==4);assert_(s['versions'][1:]==v)
 isolated('06 修改仅新建一轮并保留历史记录',modify)
 def historical(p):
  work(p);a=snap(p);p.locator('.topbar [data-action="versions"]').click();p.locator('#drawer [data-action="view-version"][data-version="1"]').click();b=snap(p);assert_(b['viewVersion']==1 and b['baseVersion']==3);assert_(b['workspaceRevision']==a['workspaceRevision'] and b['runCount']==a['runCount']);shot(p,'10-history-preview.png')
 isolated('07 查看v1不改变基线或运行',historical)
 def restore(p):
  work(p);a=snap(p);p.locator('.topbar [data-action="versions"]').click();shot(p,'09-versions.png');p.locator('#drawer [data-action="restore"][data-version="1"]').click();assert_(p.locator('#confirm').is_visible());click(p,'confirm-restore');b=snap(p);assert_(b['baseVersion']==1);assert_(b['workspaceRevision']==a['workspaceRevision']+1);assert_(b['versions']==a['versions'] and b['runCount']==a['runCount'])
 isolated('08 恢复显式确认且不新发布版本',restore)
 def missing(p):
  work(p);p.locator('.topbar [data-action="scenario"]').click();click(p,'missing-snapshot');assert_(p.locator('#drawer [data-action="restore"][data-version="1"]').is_disabled());assert_(p.locator('#drawer [data-action="view-version"][data-version="1"]').is_enabled())
 isolated('09 缺恢复源时仍可查看但禁止恢复',missing)
 def readonly(p):
  work(p);scenario(p,'readonly');assert_(p.locator('#composer').is_disabled());p.locator('.topbar [data-action="versions"]').click();assert_(all(x.is_disabled() for x in p.locator('#drawer [data-action="restore"]').all()))
 isolated('10 只读禁止编辑与恢复',readonly)
 def running_restore(p):
  work(p);scenario(p,'running');p.locator('.topbar [data-action="versions"]').click();assert_(all(x.is_disabled() for x in p.locator('#drawer [data-action="restore"]').all()))
 isolated('11 活动任务禁止恢复工作区',running_restore)
 def approve(p):
  work(p);scenario(p,'approval');n=snap(p)['runCount'];shot(p,'11-approval.png');click(p,'approve');assert_(snap(p)['decision']=='recorded' and snap(p)['scenario']=='approval');assert_('等待远端确认' in p.locator('.approval').inner_text());click(p,'ack');assert_(snap(p)['decision']=='delivered' and snap(p)['scenario']=='running');assert_(snap(p)['runCount']==n)
 isolated('12 审批区分记录和远端送达',approve)
 isolated('13 拒绝审批不授予权限',lambda p:(work(p),scenario(p,'approval'),click(p,'reject'),assert_(snap(p)['decision']=='rejected' and snap(p)['scenario']=='failed')))
 isolated('14 回答问题不形成许可决定',lambda p:(work(p),scenario(p,'question'),click(p,'answer'),assert_(snap(p)['scenario']=='running' and snap(p)['decision']=='pending')))
 def reconnect(p):
  work(p);before=snap(p);scenario(p,'reconnecting');click(p,'reconnect');after=snap(p);assert_(after['runCount']==before['runCount'] and after['versions']==before['versions'])
 isolated('15 重连只恢复原任务读取',reconnect)
 def expired(p):
  work(p);before=snap(p);scenario(p,'expired');click(p,'refresh-preview');after=snap(p);assert_(after['runCount']==before['runCount'] and after['versions']==before['versions'])
 isolated('16 预览续期不重生成',expired)
 def cancel(p):
  work(p);v=snap(p)['versions'];scenario(p,'running');click(p,'stop');click(p,'close-confirm');assert_(snap(p)['scenario']=='running');click(p,'stop');click(p,'confirm-stop');assert_(snap(p)['scenario']=='canceled' and snap(p)['versions']==v)
 isolated('17 取消二次确认并保留历史',cancel)
 isolated('18 本轮失败不替换已交付内容',lambda p:(work(p),scenario(p,'failed'),assert_(len(snap(p)['versions'])==3),assert_('本轮未发布' in p.locator('.run-card').inner_text())))
 isolated('19 结果不明没有自动重发入口',lambda p:(work(p),scenario(p,'unknown'),assert_(p.locator('[data-action="send"]').count()==0),assert_(p.locator('[data-action="finish"]').count()==0)))
 def revoke(p):
  work(p);p.locator('.topbar [data-action="sources"]').click();shot(p,'08-sources.png');click(p,'revoke');assert_('权限已撤销' in p.locator('#drawer').inner_text());assert_('产品分类、业务指标口径' not in p.locator('#drawer').inner_text());p.locator('#drawer [data-action="source-open"][data-source="S2"]').click();assert_('403' in p.locator('#toast').inner_text())
 isolated('20 来源撤权清除原文与拒绝再读',revoke)
 def source_version(p):
  work(p);a=snap(p)['versions'];p.locator('.topbar [data-action="sources"]').click();p.locator('#drawer [data-action="choose-sources"]').click();p.locator('input[value="S2"]').uncheck();click(p,'apply-sources');assert_(snap(p)['selectedSources']==['S1']);assert_(snap(p)['versions']==a);p.locator('#composer').fill('只用数据');click(p,'send');click(p,'finish');assert_(snap(p)['versions'][0]['sourceIds']==['S1']);assert_(snap(p)['versions'][1]['sourceIds']==['S1','S2'])
 isolated('21 下一轮来源变化不回写历史版本',source_version)
 def upload(p):
  click(p,'upload');p.locator('#local-files').set_input_files({'name':'<img src=x onerror=alert(1)>.csv','mimeType':'text/csv','buffer':b'x,y\n1,2'});assert_(p.locator('#upload-results img').count()==0);assert_('<img' in p.locator('#upload-results').inner_text());click(p,'close-drawer');assert_(len(snap(p)['uploaded'])==1)
 isolated('22 本地文件名转义且只读取名称大小',upload)
 def download(p):
  work(p);p.locator('.topbar [data-action="export"]').click()
  with p.expect_download() as info:click(p,'download')
  d=info.value;assert_(d.suggested_filename.endswith('-demo.html'));path=R/'qa/export-sample.html';d.save_as(str(path));assert_('原型导出示例' in path.read_text());assert_('非真实业务交付文件' in path.read_text())
 isolated('23 导出明确是HTML示例而非Office',download)
 def file_tabs(p):
  work(p);p.locator('[data-action="tab"][data-tab="files"]').click();click(p,'file');assert_('不伪造校验值' in p.locator('#confirm').inner_text());click(p,'close-confirm');p.locator('[data-action="tab"][data-tab="checks"]').click();assert_('未运行' in p.locator('.artifact-scroll').inner_text())
 isolated('24 文件元数据与未执行检查不伪造',file_tabs)
 def web_preview(p):
  work(p);before=p.locator('.metric strong').first.inner_text();p.locator('#region').select_option('east');after=p.locator('.metric strong').first.inner_text();assert_(before!=after);p.locator('#region').select_option('all');shot(p,'03-workbench-web.png');p.locator('[data-action="viewport"][data-viewport="mobile"]').click();assert_(p.locator('.report.narrow').count()==1)
 isolated('25 网页筛选与窄屏预览可交互',web_preview)
 def doc(p):
  work(p,'document');assert_(snap(p)['kind']=='document');assert_(p.locator('.document-paper').count()>0);shot(p,'04-workbench-document.png')
 isolated('26 文档内容预览',doc)
 def sheet(p):
  work(p,'spreadsheet');assert_(snap(p)['kind']=='spreadsheet');assert_(p.locator('.spreadsheet').count()>0);shot(p,'05-workbench-spreadsheet.png')
 isolated('27 表格只读预览与公式说明',sheet)
 def slides(p):
  work(p,'slides');assert_(p.locator('[data-action="prev-slide"]').is_disabled());click(p,'next-slide');assert_(snap(p)['slide']==2);p.locator('[data-action="slide"][data-slide="4"]').click();assert_(p.locator('[data-action="next-slide"]').is_disabled());p.locator('[data-action="slide"][data-slide="1"]').click();shot(p,'06-workbench-slides.png')
 isolated('28 演示稿分页与边界',slides)
 isolated('29 设计规范页色板组件',lambda p:(p.locator('[data-route="design"]').click(),assert_(p.locator('.swatch').count()==8),shot(p,'07-design-system.png')))
 def escape(p):
  work(p);b=p.locator('.topbar [data-action="versions"]');b.click();p.keyboard.press('Escape');assert_(not p.locator('#drawer').is_visible());assert_(b.evaluate('e=>e===document.activeElement'))
 isolated('30 Escape关闭并恢复触发焦点',escape)
 def ime(p):
  work(p);p.locator('#composer').fill('测试输入法');before=snap(p)['runCount'];p.locator('#composer').dispatch_event('keydown',{'key':'Enter','isComposing':True});assert_(snap(p)['runCount']==before)
 isolated('31 IME组合Enter不误发送',ime)
 def mobile():
  p=page(390,844)
  try:
   shot(p,'12-mobile-home.png');assert_(p.evaluate('document.documentElement.scrollWidth<=innerWidth'))
   work(p);assert_(p.locator('.mobile-tabs').is_visible());shot(p,'13-mobile-chat.png');p.locator('[data-action="mobile-tab"][data-tab="preview"]').click();assert_(p.locator('.artifact-pane').is_visible());shot(p,'14-mobile-preview.png');assert_(p.evaluate('document.documentElement.scrollWidth<=innerWidth'));p.locator('.mobile-tabs [data-action="sources"]').click();assert_(p.locator('#drawer').is_visible());shot(p,'15-mobile-sources.png')
  finally:p.close()
 check('32 390px首页对话作品与来源切换',mobile)
 for w in [768,1024,1280,1440]:
  def responsive(w=w):
   p=page(w,1000)
   try:
    assert_(p.evaluate('document.documentElement.scrollWidth<=innerWidth'));work(p);assert_(p.evaluate('document.documentElement.scrollWidth<=innerWidth'));shot(p,f'qa-workbench-{w}.png')
   finally:p.close()
  check(f'33-{w} 页面无水平溢出',responsive)
 def reduced():
  p=page()
  try:p.emulate_media(reduced_motion='reduce');assert_(p.evaluate('getComputedStyle(document.body).getPropertyValue("--craft-motion-base").trim()')=='0ms')
  finally:p.close()
 check('34 减少动效偏好生效',reduced)
 check('35 无浏览器未捕获异常',lambda:assert_(not errors,str(errors)))
 check('36 无远程资产/API请求',lambda:assert_(not [x for x in network if x.startswith(('http:','https:'))],str(network)))
 context.close();browser.close()
summary={'scope':'offline standalone HTML prototype ONLY','render_method':'Chromium set_content of exact index.html; file:// navigation blocked by environment policy','tests':results,'passed':sum(r['status']=='passed' for r in results),'failed':sum(r['status']=='failed' for r in results),'page_errors':errors,'remote_requests':[x for x in network if x.startswith(('http:','https:'))]}
(R/'qa/behavior-results.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2))
print('RESULT',summary['passed'],'passed;',summary['failed'],'failed')
raise SystemExit(bool(summary['failed']))
