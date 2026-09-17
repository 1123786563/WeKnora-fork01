from pathlib import Path
from playwright.sync_api import sync_playwright
import json,os
R=Path(__file__).resolve().parent.parent
with sync_playwright() as p:
 b=p.chromium.launch(executable_path=os.getenv('CHROMIUM','/usr/bin/chromium'),headless=True,args=['--no-sandbox'])
 page=b.new_page(viewport={'width':1440,'height':1000});err=[];page.on('pageerror',lambda e:err.append(str(e)));page.set_content((R/'handoff.html').read_text())
 result=[]
 for width in [390,1440]:
  page.set_viewport_size({'width':width,'height':1000})
  for i in range(14):
   page.locator(f'aside a[data-nav="{i}"]').click()
   page.locator(f'#doc-{i}').wait_for(state='visible',timeout=3000)
   result.append({'width':width,'section':i,'visible':page.locator(f'#doc-{i}').is_visible(),'no_document_overflow':page.evaluate('document.documentElement.scrollWidth<=window.innerWidth+1')})
 page.locator('aside a[data-nav="1"]').click();page.locator('#doc-1').wait_for(state='visible');page.screenshot(path=str(R/'screenshots/handoff-desktop.png'),full_page=False)
 b.close()
 report={'scope':'injected handoff reader only','checks':result,'js_errors':err,'passed':sum(r['visible'] and r['no_document_overflow'] for r in result),'failed':sum(not (r['visible'] and r['no_document_overflow']) for r in result)}
 (R/'verification/handbook-report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
 assert not err and not report['failed'],report
 print('Handbook',report['passed'],'passed; errors',err)
