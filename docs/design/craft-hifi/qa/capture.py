import os, shutil
from pathlib import Path
from playwright.sync_api import sync_playwright
R=Path(__file__).resolve().parents[1]
with sync_playwright() as p:
 browser=p.chromium.launch(executable_path=os.environ.get('CHROMIUM_PATH') or shutil.which('chromium'),headless=True,args=['--no-sandbox'])
 page=browser.new_page(viewport={'width':1440,'height':1000},device_scale_factor=1)
 page.on('pageerror',lambda e:print('PAGEERROR',e))
 page.set_content((R/'index.html').read_text(),wait_until='load');page.wait_for_timeout(400)
 page.screenshot(path=str(R/'screenshots/01-home.png'),full_page=True)
 page.locator('.artifact-card[data-id="p1"]').click();page.wait_for_timeout(300)
 page.screenshot(path=str(R/'screenshots/03-workbench-web.png'),full_page=True)
 print('overflow',page.evaluate('document.documentElement.scrollWidth>innerWidth'))
 browser.close()
