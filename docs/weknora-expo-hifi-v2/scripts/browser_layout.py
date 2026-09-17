"""Render actual self-contained HTML using set_content (file/localhost navigation is restricted here)."""
from pathlib import Path
import json,os,time
from playwright.sync_api import sync_playwright
R=Path(__file__).resolve().parent.parent
results=[];errors=[];requests=[]
with sync_playwright() as p:
 b=p.chromium.launch(executable_path=os.getenv('CHROMIUM','/usr/bin/chromium'),headless=True,args=['--no-sandbox'])
 page=b.new_page(viewport={'width':1440,'height':1050},device_scale_factor=1)
 page.on('pageerror',lambda e:errors.append(str(e)));page.on('request',lambda q:requests.append(q.url))
 page.set_content((R/'index.html').read_text());page.wait_for_timeout(100)
 screens=page.evaluate('WorkbenchReview.pages')
 for theme in ['light','dark']:
  page.evaluate('(x)=>WorkbenchReview.setTheme(x)',theme)
  for width in [320,360,390,430,768,1440]:
   page.set_viewport_size({'width':width,'height':900 if width<720 else 1050})
   for screen in screens:
    page.evaluate('(x)=>WorkbenchReview.go(x)',screen['id'])
    m=page.evaluate('''()=>{const e=document.querySelector('#phone #screen-content'),ph=document.querySelector('#phone');return {document:[document.documentElement.scrollWidth,window.innerWidth],content:[e.scrollWidth,e.clientWidth],phone:[ph.scrollWidth,ph.clientWidth]}}''')
    results.append({'page':screen['no'],'theme':theme,'width':width,'font_scale':1,'measurements':m,'pass':all(a<=c+1 for a,c in m.values())})
    if width==1440:
     page.locator('#phone').screenshot(path=str(R/'screenshots'/f'{screen["no"]}-{screen["id"]}-{theme}.png'))
  page.set_viewport_size({'width':390,'height':900});page.evaluate('WorkbenchReview.setLarge(true)')
  for screen in screens:
   page.evaluate('(x)=>WorkbenchReview.go(x)',screen['id'])
   m=page.evaluate('''()=>{const e=document.querySelector('#phone #screen-content'),ph=document.querySelector('#phone');return {document:[document.documentElement.scrollWidth,window.innerWidth],content:[e.scrollWidth,e.clientWidth],phone:[ph.scrollWidth,ph.clientWidth]}}''')
   results.append({'page':screen['no'],'theme':theme,'width':390,'font_scale':1.25,'measurements':m,'pass':all(a<=c+1 for a,c in m.values())})
  page.evaluate('WorkbenchReview.setLarge(false)')
 page.set_viewport_size({'width':1440,'height':1050});page.evaluate("WorkbenchReview.setTheme('light');WorkbenchReview.go('home')")
 page.screenshot(path=str(R/'screenshots/studio-desktop.png'),full_page=True)
 page.evaluate("WorkbenchReview.setMode('guide')");page.screenshot(path=str(R/'screenshots/design-system-light.png'),full_page=True)
 page.evaluate("WorkbenchReview.setTheme('dark')");page.screenshot(path=str(R/'screenshots/design-system-dark.png'),full_page=True)
 # Standalone files must initialize the correct page when their actual bytes are injected.
 standalones=[]
 for screen in screens:
  page.close();page=b.new_page(viewport={'width':1440,'height':1050})
  page.on('pageerror',lambda e:errors.append(str(e)));page.on('request',lambda q:requests.append(q.url))
  page.set_content((R/'pages'/f'{screen["no"]}-{screen["id"]}.html').read_text())
  standalones.append({'page':screen['no'],'pass':page.evaluate('WorkbenchReview.snapshot().page')==screen['id']})
 page.close();page=b.new_page(viewport={'width':1440,'height':1050})
 page.on('pageerror',lambda e:errors.append(str(e)))
 page.set_content((R/'design-system.html').read_text());kit=page.locator('#guide').is_visible()
 b.close()
report={'scope':'Chromium set_content render only; not local-file navigation, native rendering, hosted deployment or reload persistence','layout_cases':results,'standalone_initialization':standalones,'design_system_default_visible':kit,'js_errors':errors,'network_requests':requests,'passed':sum(x['pass'] for x in results),'failed':sum(not x['pass'] for x in results)}
(R/'verification/browser-layout.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
print(json.dumps({'layout_passed':report['passed'],'layout_failed':report['failed'],'failures':[x for x in results if not x['pass']],'standalone_passed':sum(x['pass'] for x in standalones),'guide':kit,'js_errors':errors,'requests':requests},ensure_ascii=False))
raise SystemExit(1 if report['failed'] or errors or not kit or not all(x['pass'] for x in standalones) else 0)
