"""Behavior checks against the shipped prototype, not a backend or native test."""
from pathlib import Path
from playwright.sync_api import sync_playwright
import json,os
R=Path(__file__).resolve().parent.parent
results=[];errors=[];requests=[]
with sync_playwright() as p:
 b=p.chromium.launch(executable_path=os.getenv('CHROMIUM','/usr/bin/chromium'),headless=True,args=['--no-sandbox'])
 page=b.new_page(viewport={'width':1440,'height':1050},accept_downloads=True,reduced_motion='reduce');page.set_default_timeout(2000)
 page.on('pageerror',lambda e:errors.append(str(e)));page.on('request',lambda q:requests.append(q.url))
 page.set_content((R/'index.html').read_text())
 def go(x):page.evaluate('(x)=>WorkbenchReview.go(x)',x)
 def state():return page.evaluate('WorkbenchReview.snapshot()')
 def v():s=state();return s['spaces'][s['space']]
 def click(act):page.locator(f'#phone [data-act="{act}"]').click()
 def scenario(x):page.evaluate('(x)=>WorkbenchReview.setScenario(x)',x)
 def test(name,fn):
  page.evaluate('WorkbenchReview.reset()')
  try:fn();results.append({'name':name,'pass':True})
  except Exception as e:results.append({'name':name,'pass':False,'error':str(e)[:1400]})
 def require(x,msg='assertion failed'):
  if not x:raise AssertionError(msg)
 def newsubmit():
  go('new');page.locator('#task-prompt').fill('整理一份可交付的项目周报');page.locator('#task-budget').fill('75');click('submit');s=state();require(s['page']=='conversation');require(len(s['requests'])==1 and s['requests'][0]['budget']==75);require('整理一份' in page.locator('#phone').inner_text())
 test('edited task and budget produce one demo request',newsubmit)
 def emptydraft():go('new');page.locator('#task-prompt').fill(' ');click('submit');require(len(state()['requests'])==0)
 test('blank task is rejected',emptydraft)
 def invalidbudget():go('new');page.locator('#task-budget').fill('-1');click('submit');require(len(state()['requests'])==0 and state()['page']=='new')
 test('invalid budget does not create request',invalidbudget)
 def offline():go('new');page.locator('#task-prompt').fill('离线保留的原始草稿');scenario('offline');require(page.locator('#phone [data-act=submit]').is_disabled());scenario('normal');require(page.locator('#task-prompt').input_value()=='离线保留的原始草稿');require(len(state()['requests'])==0)
 test('offline disables submit and preserves draft',offline)
 def unknown():
  go('new');scenario('unknown');click('submit');r=state()['requests'][0]['id'];require(state()['page']=='run');go('new');click('submit');require(len(state()['requests'])==1);click('reconcile');require(state()['requests'][0]['id']==r and state()['requests'][0]['status']=='admitted');require(len(state()['requests'])==1)
 test('unknown acknowledgement reconciles same request without restart',unknown)
 def scope():
  go('new');page.locator('#task-prompt').fill('团队独有草稿');go('spaces');page.locator('#phone [data-space=personal]').click();require('发布客户反馈摘要' not in page.locator('#phone').inner_text());go('new');require(page.locator('#task-prompt').input_value()=='');page.locator('#task-prompt').fill('个人独有草稿');go('spaces');page.locator('#phone [data-space=team]').click();go('new');require(page.locator('#task-prompt').input_value()=='团队独有草稿')
 test('tenant switch isolates drafts and private home data',scope)
 def agent():go('agents');page.locator('#phone [data-agent=research]').click();require(state()['page']=='new' and v()['agent']=='research')
 test('agent selection returns to task draft',agent)
 def unavailable():go('agents');page.locator('#phone [data-agent=coding]').click();require(page.locator('#phone [role=dialog]').is_visible());require(v()['agent']=='general');page.locator('#phone').get_by_role('button',name='知道了',exact=True).click()
 test('unavailable coding capability is not enabled',unavailable)
 def target():go('targets');page.locator('#phone [data-target=managed]').click();require(state()['page']=='new' and v()['target']=='managed')
 test('target selection updates current draft',target)
 def search():go('sessions');page.locator('#phone input[data-search=sessions]').fill('周报');require(page.locator('#session-list button').count()==1);page.locator('#phone input[data-search=sessions]').fill('不存在');require('没有匹配' in page.locator('#session-list').inner_text())
 test('session search filters and provides empty result',search)
 def filter_sessions():go('sessions');page.locator('#phone [data-session-filter=running]').click();require(page.locator('#session-list button').count()==1 and '季度项目' in page.locator('#session-list').inner_text())
 test('session status filter is applied',filter_sessions)
 def resource_search():go('resources');page.locator('#phone input[data-search=resources]').fill('没有这份资料');require(page.locator('#resource-list button').count()==0)
 test('resource search stays scoped',resource_search)
 def approval():go('approval');click('approve');require(len(state()['decisions'])==0);require(page.locator('#phone .phone-base').get_attribute('inert') is not None);click('approve-confirm');require(len(state()['decisions'])==1 and v()['approval']=='approved');require(v()['run']=='reconciling');require(page.locator('#phone [data-act=approve]').count()==0)
 test('approval requires confirmation and does not claim external success',approval)
 def dismiss():go('approval');click('approve');page.keyboard.press('Escape');require(len(state()['decisions'])==0);require(page.locator('#phone [role=dialog]').count()==0);require(page.evaluate('document.activeElement.dataset.act')=='approve')
 test('escape cancels confirmation and restores focus',dismiss)
 def focus():go('approval');click('approve');page.keyboard.press('Shift+Tab');require(page.evaluate('document.activeElement.dataset.act')=='approve-confirm');page.keyboard.press('Tab');require(page.evaluate('document.activeElement.dataset.act')=='close')
 test('confirmation keyboard focus stays inside dialog',focus)
 def reject():go('approval');click('reject');click('reject-confirm');require(v()['approval']=='rejected' and state()['decisions'][0]['action']=='reject')
 test('explicit rejection is a distinct decision',reject)
 def conflict():go('approval');scenario('conflict');require(page.locator('#phone [data-act=approve]').count()==0);click('refresh-approval');require(v()['approval']=='approved' and len(state()['decisions'])==0)
 test('revision conflict refresh does not resend previous approval',conflict)
 def forbidden():
  scenario('forbidden')
  for x in ['approval','connector','artifact','knowledge','usage']:
   go(x);text=page.locator('#phone').inner_text();require('project-bot' not in text and '8,620' not in text and '本周共整理' not in text,x)
 test('forbidden pages hide sensitive payloads',forbidden)
 def cancel():go('run');click('cancel');click('cancel-confirm');require(v()['run']=='cancel_requested' and v()['execution']=='stop_pending');require(page.locator('#phone [data-act=cancel]').is_disabled())
 test('cancel means stop pending, not confirmed stop',cancel)
 def attach():go('new');click('attachment');click('attachment-confirm');require(not v()['attachmentReady']);click('submit');require(len(state()['requests'])==0);page.wait_for_timeout(1100);require(v()['attachmentReady']);click('submit');require(len(state()['requests'])==1)
 test('attachment readiness gates task submission',attach)
 def lateattach():go('new');click('attachment');click('attachment-confirm');go('spaces');page.locator('#phone [data-space=personal]').click();page.wait_for_timeout(1100);require(not v()['attachment'] and not v()['attachmentReady']);go('spaces');page.locator('#phone [data-space=team]').click();require(not v()['attachmentReady'])
 test('late upload result cannot cross scope generation',lateattach)
 def voice():
  go('voice');click('record');click('transcribe');page.locator('#voice-transcript').fill('用户最终编辑的文字');click('voice-to-draft');require(state()['page']=='conversation' and v()['chatDraft']=='用户最终编辑的文字');require(v()['messages']==[]);click('send');require(v()['messages']==['用户最终编辑的文字'] and v()['chatDraft']=='')
 test('edited voice transcript becomes unsent draft before send',voice)
 def safetext():go('conversation');page.locator('#phone textarea[data-field=chatDraft]').fill('<img src=x onerror=alert(1)>');click('send');require(page.locator('#phone img').count()==0);require('<img src=x' in page.locator('#phone').inner_text())
 test('user messages are escaped, not executable HTML',safetext)
 def conn():go('connector');click('revoke');click('revoke-confirm');require(not v()['connected']);click('connect');click('connect-confirm');require(v()['connected'])
 test('connection revoke/reconnect has explicit simulated confirmation',conn)
 def pref():go('profile');click('notifications');require(not state()['notifications']);click('theme');require(state()['theme']=='dark' and page.locator('html').get_attribute('data-theme')=='dark')
 test('theme and notification preferences are interactive',pref)
 def logout():go('profile');click('logout');click('logout-confirm');require(state()['page']=='login' and len(state()['requests'])==0)
 test('logout returns to login and clears demo state',logout)
 def read():go('inbox');click('read-all');require(len(state()['decisions'])==0 and v()['approval']=='pending')
 test('mark read does not approve a task',read)
 def recover():go('new');page.locator('#task-prompt').fill('网络错误之后保留');scenario('error');click('recover');require(page.locator('#task-prompt').input_value()=='网络错误之后保留');require(len(state()['requests'])==0)
 test('error recovery preserves draft and does not launch',recover)
 def gallery():page.evaluate("WorkbenchReview.setMode('gallery')");require(page.locator('#gallery > .gallery-item').count()==18);page.locator('#gallery > .gallery-item').first.locator(':scope > .row button').click();require(state()['page']=='login')
 test('gallery opens selected live page',gallery)
 def download():
  go('artifact')
  with page.expect_download() as event:click('download')
  d=event.value;d.save_as(str(R/'verification/downloaded-demo.md'));require('演示' in d.suggested_filename and '不是实际客户资料' in (R/'verification/downloaded-demo.md').read_text())
 test('artifact download creates only a labelled local demo file',download)
 def list_metadata():
  go('profile')
  actual=page.evaluate("""()=>{const row=document.querySelector('#phone .list-row .grow');const title=row.querySelector('strong'),sub=row.querySelector('small');return {display:getComputedStyle(sub).display,separate:sub.getBoundingClientRect().top>=title.getBoundingClientRect().bottom};}""")
  require(actual['display']=='block' and actual['separate'])
 test('list titles and metadata have distinct readable lines',list_metadata)
 # State screenshots for manual review.
 for name,screen,sc in [('offline','new','offline'),('conflict','approval','conflict'),('forbidden','approval','forbidden'),('unknown','run','unknown')]:
  page.evaluate('WorkbenchReview.reset()');go(screen);scenario(sc);page.locator('#phone').screenshot(path=str(R/'screenshots'/f'state-{name}.png'))
 page.evaluate('WorkbenchReview.reset()');go('approval');click('approve');page.locator('#phone').screenshot(path=str(R/'screenshots/state-confirmation.png'))
 b.close()
report={'scope':'Local HTML demonstrator only; no production/native auth, database, push, microphone, provider or billing verified','tests':results,'passed':sum(x['pass'] for x in results),'failed':sum(not x['pass'] for x in results),'js_errors':errors,'external_requests':[u for u in requests if u.startswith(('https:','http:'))],'all_request_urls':requests}
(R/'verification/browser-interactions.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n');print(json.dumps(report,ensure_ascii=False,indent=2))
raise SystemExit(1 if report['failed'] or errors or report['external_requests'] else 0)
