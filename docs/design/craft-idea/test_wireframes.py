"""Local wireframe acceptance checks; not a test of WeKnora's real backend."""
from pathlib import Path
import json
import os
import shutil
from playwright.sync_api import sync_playwright
ROOT=Path(__file__).resolve().parent
HTML=ROOT/'craft-wireframes.html'
assert HTML.exists(), 'Wireframe deliverable has not been created'
checks=[]
with sync_playwright() as p:
    browser=p.chromium.launch(executable_path=os.environ.get('CHROMIUM_PATH') or shutil.which('chromium') or shutil.which('google-chrome'),headless=True,args=['--no-sandbox'])
    page=browser.new_page(viewport={'width':1440,'height':1000},device_scale_factor=1)
    errors=[]
    page.on('pageerror', lambda error: errors.append(str(error)))
    # Browser policy in this environment blocks file:// navigation.
    # Load the same self-contained HTML bytes directly, with no network.
    page.set_content(HTML.read_text(), wait_until='load')
    page.get_by_test_id('home-heading').wait_for()
    checks.append('Home renders')
    page.screenshot(path=str(ROOT/'01-home.png'),full_page=True)
    page.get_by_test_id('open-demo').first.click()
    page.get_by_test_id('workbench').wait_for()
    checks.append('Recent work opens the workbench')
    page.screenshot(path=str(ROOT/'02-workbench.png'),full_page=True)
    page.get_by_test_id('tab-files').click()
    assert page.get_by_test_id('file-list').is_visible()
    checks.append('Version-bound files panel')
    page.get_by_test_id('tab-checks').click()
    assert page.get_by_test_id('checks-panel').is_visible()
    checks.append('Verification facts panel')
    page.get_by_test_id('tab-preview').click()
    page.get_by_test_id('sources').click()
    assert page.get_by_test_id('source-panel').is_visible()
    page.get_by_test_id('close-panel').click()
    checks.append('Knowledge sources panel')
    page.get_by_test_id('versions').click()
    page.get_by_test_id('view-v1').click()
    assert 'v1' in page.get_by_test_id('version-label').inner_text()
    checks.append('Historic version selection')
    page.get_by_test_id('versions').click()
    page.get_by_test_id('restore-v1').click()
    page.get_by_test_id('confirm-restore').click()
    assert page.evaluate('window.demoState.workspaceRevision') == 8
    assert page.evaluate('window.demoState.versions.length') == 3
    checks.append('Restoring changes workspace revision, not immutable published versions')
    before=page.evaluate('window.demoState.runCount')
    page.get_by_test_id('scenario').select_option('reconnecting')
    assert page.get_by_test_id('sync-banner').is_visible()
    assert page.evaluate('window.demoState.runCount')==before
    checks.append('Disconnect simulation does not create a run')
    page.get_by_test_id('scenario').select_option('approval')
    assert page.get_by_test_id('approval-card').is_visible()
    page.screenshot(path=str(ROOT/'03-approval.png'),full_page=True)
    page.get_by_test_id('approve-once').click()
    assert '已记录' in page.get_by_test_id('approval-card').inner_text()
    checks.append('Approval recorded separately from delivery')
    page.get_by_test_id('scenario').select_option('failed')
    assert page.get_by_test_id('preview-canvas').is_visible()
    checks.append('Failed run preserves the last published preview')
    page.get_by_test_id('scenario').select_option('ready')
    page.get_by_test_id('composer').fill('增加季度说明')
    page.get_by_test_id('send').click()
    assert page.evaluate('window.demoState.runCount')==before+1
    page.get_by_test_id('finish-demo').click()
    checks.append('Explicit send and explicit demo completion')
    page.locator('#toast').wait_for(state='hidden', timeout=7000)
    page.set_viewport_size({'width':390,'height':844})
    page.get_by_test_id('mobile-preview').click()
    page.screenshot(path=str(ROOT/'04-mobile.png'),full_page=True)
    assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth')
    checks.append('390px layout has no document overflow')
    assert not errors,errors
    checks.append('No JavaScript page errors')
    browser.close()
(ROOT/'verification.json').write_text(json.dumps({'scope':'Standalone wireframes only; no real backend or model calls', 'passed':len(checks),'checks':checks},ensure_ascii=False,indent=2))
print(json.dumps({'passed':len(checks),'checks':checks},ensure_ascii=False,indent=2))
