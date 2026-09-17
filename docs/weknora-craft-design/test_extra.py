"""Additional first-run and exceptional-state tests for the prototype only."""
from pathlib import Path
import json
import os
import shutil
from playwright.sync_api import sync_playwright
ROOT=Path(__file__).resolve().parent
checks=[]
with sync_playwright() as p:
    b=p.chromium.launch(executable_path=os.environ.get('CHROMIUM_PATH') or shutil.which('chromium') or shutil.which('google-chrome'),headless=True,args=['--no-sandbox'])
    page=b.new_page(viewport={'width':1440,'height':1000})
    errors=[]
    page.on('pageerror',lambda e:errors.append(str(e)))
    page.set_content((ROOT/'craft-wireframes.html').read_text())
    page.locator('#home-prompt').fill('用当前资料生成一份网页报告')
    page.locator('[data-action="create"]').click()
    assert page.evaluate('window.demoState.versions.length')==0, 'A new work must not inherit demo versions'
    assert page.get_by_test_id('empty-artifact').is_visible()
    checks.append('New work starts without a published version')
    page.get_by_test_id('finish-demo').click()
    assert page.evaluate('window.demoState.version')==1
    checks.append('First publication is v1')
    page.get_by_test_id('scenario').select_option('failed')
    assert '失败' in page.locator('.run-card').inner_text()
    checks.append('Failed current run is not labelled completed')
    page.get_by_test_id('scenario').select_option('running')
    page.locator('[data-action="stop"]').click()
    page.locator('[data-action="confirm-stop"]').click()
    assert page.evaluate('window.demoState.scenario')=='canceled'
    assert '已取消' in page.locator('.run-card').inner_text()
    checks.append('Cancellation has its own terminal presentation')
    before=page.evaluate('window.demoState.runCount')
    page.get_by_test_id('scenario').select_option('expired')
    page.locator('[data-action="refresh-preview"]').click()
    assert page.evaluate('window.demoState.runCount')==before
    checks.append('Preview renewal creates no run')
    page.get_by_test_id('scenario').select_option('readonly')
    assert page.get_by_test_id('send').is_disabled()
    page.get_by_test_id('versions').click()
    assert page.get_by_test_id('restore-v1').is_disabled()
    page.get_by_test_id('close-panel').click()
    checks.append('Read-only blocks send and restore controls')
    page.get_by_test_id('sources').click()
    page.locator('[data-action="revoke"]').click()
    page.locator('[data-action="source-open"][data-source="S2"]').click()
    assert '403' in page.locator('#toast').inner_text()
    page.get_by_test_id('close-panel').click()
    checks.append('Revoked citation shows denied access')
    assert not errors, errors
    b.close()
(ROOT/'verification-extra.json').write_text(json.dumps({'scope':'Standalone wireframes only', 'passed':len(checks),'checks':checks},ensure_ascii=False,indent=2))
print(json.dumps({'passed':len(checks),'checks':checks},ensure_ascii=False,indent=2))
