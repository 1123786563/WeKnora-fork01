"""Browser-only checks for the delivered HTML wireframe, NOT the Expo/Go app.
Run: python verification/check_prototype.py
Requires Playwright and Chromium. Override CHROMIUM_PATH when needed.
The document is injected via set_content; it has no external dependencies.
"""
from pathlib import Path
from datetime import datetime, timezone
import json, os
from playwright.sync_api import sync_playwright

ROOT=Path(__file__).resolve().parents[1]
results=[]
def check(name, condition, detail=''):
    ok=bool(condition)
    results.append({'name':name,'passed':ok,'detail':detail})
    print(('PASS' if ok else 'FAIL')+' '+name+(' | '+detail if detail else ''))

with sync_playwright() as p:
    browser=p.chromium.launch(executable_path=os.environ.get('CHROMIUM_PATH','/usr/bin/chromium'),headless=True,args=['--no-sandbox'])
    page=browser.new_page(viewport={'width':1440,'height':1050},device_scale_factor=1)
    errors=[]; requests=[]
    page.on('pageerror',lambda error:errors.append(str(error)))
    page.on('request',lambda request:requests.append(request.url))
    page.set_content((ROOT/'wireframes.html').read_text(encoding='utf-8'),wait_until='load')
    api='window.WireframeReview'
    def snapshot(): return page.evaluate(api+'.snapshot()')
    def reset(): page.evaluate(api+'.reset()')
    def go(where): page.evaluate('(id)=>window.WireframeReview.go(id)',where)
    def click(action): page.locator('#phone [data-act="'+action+'"]').click()
    def scenario(value): page.locator('#scenario').select_option(value)
    screens=page.evaluate(api+'.pages')
    check('18 screen definitions and navigation entries',len(screens)==18 and page.locator('#sidenav button').count()==18)
    check('desktop document has no horizontal overflow',page.evaluate('document.documentElement.scrollWidth <= innerWidth'))
    overflows=[]
    for width in [1440,390,360]:
        page.set_viewport_size({'width':width,'height':1050 if width==1440 else 1000})
        for screen in screens:
            go(screen['id'])
            dims=page.locator('#phone .app-scroll').evaluate('(el)=>({client:el.clientWidth,scroll:el.scrollWidth})')
            if dims['scroll']>dims['client']+1 or not page.evaluate('document.documentElement.scrollWidth <= innerWidth+1'):
                overflows.append({'width':width,'screen':screen['id'],'dimensions':dims})
        check(f'all 18 screens fit horizontally at {width}px',not any(o['width']==width for o in overflows))
    page.set_viewport_size({'width':1440,'height':1050}); reset()
    go('new')
    page.locator('#phone [data-field="prompt"]').fill('')
    click('submit-task')
    check('empty prompt blocked',snapshot()['page']=='new' and '任务描述' in snapshot()['toast'])
    page.locator('#phone [data-field="prompt"]').fill('整理本周反馈 · 原型回归')
    page.locator('#phone [data-field="budget"]').fill('-2');click('submit-task')
    check('invalid budget blocked',snapshot()['page']=='new' and '正整数' in snapshot()['toast'])
    page.locator('#phone [data-field="budget"]').fill('80')
    page.locator('#phone [data-go="agents"]').click()
    page.locator('#phone [data-agent="知识助手"]').click()
    check('Agent chooser returns with selection',snapshot()['page']=='new' and snapshot()['agent']=='知识助手')
    click('submit-task')
    check('task text and budget carried into conversation',snapshot()['page']=='conversation' and snapshot()['budget']=='80' and '整理本周反馈' in page.locator('#phone').inner_text())
    page.locator('#phone [data-field="draft"]').fill('请保留我的草稿')
    scenario('offline')
    check('offline disables send but retains draft',page.locator('#phone [data-act="send-message"]').is_disabled() and page.locator('#phone [data-field="draft"]').input_value()=='请保留我的草稿')
    scenario('normal');click('send-message')
    check('editable follow-up appears in local message model','请保留我的草稿' in snapshot()['messages'])
    reset();go('approval');click('approve')
    check('approval requires confirmation first',page.locator('#phone [role="dialog"]').count()==1 and snapshot()['approval']=='pending')
    page.keyboard.press('Escape')
    check('Escape dismisses without approval',snapshot()['modal'] is None and snapshot()['approval']=='pending')
    click('approve');click('confirm-approve')
    check('approval recorded without declaring task succeeded',snapshot()['approval']=='approved' and snapshot()['run']!='succeeded')
    reset();go('approval');scenario('conflict')
    check('conflict view hides stale approve button',page.locator('#phone [data-act="approve"]').count()==0 and '另一设备' in page.locator('#phone').inner_text())
    click('refresh-approval')
    check('conflict refresh loads existing decision',snapshot()['approval']=='approved' and snapshot()['scenario']=='normal')
    reset();go('approval');scenario('forbidden')
    check('permission denial hides sensitive target/account','project-bot' not in page.locator('#phone').inner_text() and '暂时无法访问' in page.locator('#phone').inner_text())
    reset();go('new');scenario('unknown');click('submit-task')
    before=snapshot();go('new');click('submit-task');after=snapshot()
    check('uncertain start reuses original request',before['request']==after['request'] and before['requestCount']==after['requestCount'] and after['page']=='run')
    click('reconcile')
    check('reconcile keeps request identity',snapshot()['request']==before['request'] and snapshot()['run']!='unknown')
    click('clear-toast');click('cancel-run');click('confirm-cancel')
    check('cancel shows pending stop rather than confirmed stop',snapshot()['run']=='cancel_requested' and '停止待确认' in page.locator('#phone').inner_text())
    reset();go('spaces');page.locator('#phone [data-space="个人空间"]').click()
    check('space change produces isolated empty state',snapshot()['space']=='个人空间' and '客户反馈摘要' not in page.locator('#phone').inner_text())
    go('artifact')
    check('old artifact not exposed after space switch','客户反馈分析摘要' not in page.locator('#phone').inner_text())
    reset();go('voice');click('record')
    check('voice generates editable draft',bool(snapshot()['voiceText']))
    page.locator('#phone [data-field="voiceText"]').fill('这是确认后编辑的语音内容')
    click('send-voice')
    check('voice sends edited text, not original transcript',snapshot()['page']=='conversation' and snapshot()['messages'][-1]=='这是确认后编辑的语音内容')
    reset();go('connector');click('revoke');click('confirm-revoke')
    check('revoke requires confirmation and updates demo state',not snapshot()['connected'])
    reset();go('profile');click('toggle-notifications')
    check('notification preference independent of task state',not snapshot()['notices'] and snapshot()['run']=='waiting_user')
    reset();page.locator('[data-act="gallery"]').click()
    check('all-page gallery contains 18 wireframes',page.locator('#gallery .gallery-card').count()==18 and snapshot()['board'])
    page.locator('#gallery .gallery-card > header [data-go="new"]').click()
    check('gallery links back to interactive screen',snapshot()['page']=='new' and not snapshot()['board'])
    reset();go('new')
    page.locator('#phone [data-field="prompt"]').fill('<img src=x onerror=alert(1)> 文本')
    click('submit-task')
    check('user text escaped, not interpreted as markup',page.locator('#phone img').count()==0 and '<img' in page.locator('#phone .bubble').first.inner_text())
    reset();go('login');click('demo-login')
    check('demo login only navigates without credentials',snapshot()['page']=='spaces')
    reset();go('profile');click('logout')
    check('logout returns to login',snapshot()['page']=='login')
    reset()
    screenshot_files=[]
    for screen in screens:
        reset();go(screen['id'])
        file=ROOT/'screenshots'/f"{screen['n']}-{screen['id']}.png"
        page.locator('#phone').screenshot(path=str(file))
        screenshot_files.append(file.name)
    reset();page.screenshot(path=str(ROOT/'verification/review-desktop.png'),full_page=True)
    scenario('unknown');go('run');page.locator('#phone').screenshot(path=str(ROOT/'screenshots/state-unknown.png'))
    reset();go('approval');scenario('conflict');page.locator('#phone').screenshot(path=str(ROOT/'screenshots/state-conflict.png'))
    reset();go('approval');scenario('forbidden');page.locator('#phone').screenshot(path=str(ROOT/'screenshots/state-forbidden.png'))
    reset();go('conversation');scenario('offline');page.locator('#phone').screenshot(path=str(ROOT/'screenshots/state-offline.png'))
    reset();go('approval');click('approve');page.locator('#phone').screenshot(path=str(ROOT/'screenshots/state-approval-confirm.png'))
    check('no runtime JavaScript page errors',not errors,str(errors))
    check('no network requests from self-contained prototype',not requests,str(requests))
    check('18 core screen screenshots generated',len(screenshot_files)==18)
    version=browser.version
    browser.close()
report={'artifact':'wireframes.html','scope':'HTML wireframe only; no Expo/Go/backend/Provider verification','method':'Chromium Playwright set_content of self-contained HTML; file URL navigation is restricted in this container','checked_at':datetime.now(timezone.utc).isoformat(),'browser':version,'passed':all(r['passed'] for r in results),'checks':results,'horizontal_overflows':overflows,'page_errors':errors,'network_requests':requests,'screenshots':screenshot_files}
(ROOT/'verification/prototype-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
print(f"RESULT {sum(r['passed'] for r in results)}/{len(results)} passed")
raise SystemExit(0 if report['passed'] else 1)
