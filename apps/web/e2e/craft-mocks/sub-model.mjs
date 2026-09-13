// W06 controlled SUB-executor model endpoint (OpenAI-compatible) — the
// browser-acceptance counterpart of R07's craftMockProvider. The pinned
// OpenCode serve points its provider here; every generation streams one
// round of write-tool calls that produce the web report files inside the
// session workspace directory, then the final text once tool results are
// back. The figures mirror e2e/fixtures/craft-sales.csv: 2026-01 东区 100,
// 2026-02 西区 200, total 300.
import { createServer } from 'node:http';

const port = Number(process.argv[2] ?? 41872);
// The main app origin the malicious fixture tries to fetch (embedded into the
// generated probe page; replaced again at request time from the env).
const probeAppOrigin = process.env.CRAFT_PROBE_APP_ORIGIN ?? 'http://127.0.0.1:41876';
let hits = 0;

function turnView(messages) {
  let lastUser = '';
  let toolDone = false;
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i];
    const raw = typeof message.content === 'string' ? message.content : JSON.stringify(message.content ?? '');
    if (message.role === 'tool' || raw.includes('tool-result')) { toolDone = true; continue; }
    if (message.role !== 'user') continue;
    if (Array.isArray(message.content)) {
      const text = message.content.filter((part) => part && part.type === 'text').map((part) => part.text).join('\n');
      lastUser = text;
    } else if (typeof message.content === 'string') {
      lastUser = message.content;
    }
    break;
  }
  return { lastUser, toolDone };
}

// The runtime repoints the serve-relative output/ pointer at the active
// delegation's workspace before each dispatch, so plain relative output
// paths are session-scoped by the server, not by the model.

function monthlyPage() {
  return `<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>按月销售报告</title></head>
<body>
<h1>按月销售报告</h1>
<p id="total-line">总额：<strong id="total-value">300</strong></p>
<label>地区筛选 <select id="region-filter">
<option value="all">全部</option>
<option value="east">东区</option>
<option value="west">西区</option>
</select></label>
<table id="sales">
<thead><tr><th scope="col">月份</th><th scope="col">地区</th><th scope="col">收入</th></tr></thead>
<tbody>
<tr data-region="east"><td>2026-01</td><td>东区</td><td>100</td></tr>
<tr data-region="west"><td>2026-02</td><td>西区</td><td>200</td></tr>
</tbody>
</table>
<script>
(function () {
  var rows = Array.prototype.slice.call(document.querySelectorAll('#sales tbody tr'));
  var filter = document.getElementById('region-filter');
  var total = document.getElementById('total-value');
  function apply() {
    var sum = 0;
    rows.forEach(function (row) {
      var visible = filter.value === 'all' || row.getAttribute('data-region') === filter.value;
      row.style.display = visible ? '' : 'none';
      if (visible) sum += parseInt(row.cells[2].textContent, 10) || 0;
    });
    total.textContent = String(sum);
  }
  filter.addEventListener('change', apply);
  apply();
})();
</script>
</body>
</html>
`;
}

function quarterlyPage() {
  return `<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>季度销售汇总</title></head>
<body>
<h1>季度销售汇总</h1>
<p id="total-line">总额：<strong id="total-value">300</strong></p>
<label>地区筛选 <select id="region-filter">
<option value="all">全部</option>
<option value="east">东区</option>
<option value="west">西区</option>
</select></label>
<table id="sales">
<thead><tr><th scope="col">季度</th><th scope="col">地区</th><th scope="col">收入</th></tr></thead>
<tbody>
<tr data-region="east"><td>2026-Q1</td><td>东区</td><td>100</td></tr>
<tr data-region="west"><td>2026-Q1</td><td>西区</td><td>200</td></tr>
</tbody>
</table>
<script>
(function () {
  var rows = Array.prototype.slice.call(document.querySelectorAll('#sales tbody tr'));
  var filter = document.getElementById('region-filter');
  var total = document.getElementById('total-value');
  function apply() {
    var sum = 0;
    rows.forEach(function (row) {
      var visible = filter.value === 'all' || row.getAttribute('data-region') === filter.value;
      row.style.display = visible ? '' : 'none';
      if (visible) sum += parseInt(row.cells[2].textContent, 10) || 0;
    });
    total.textContent = String(sum);
  }
  filter.addEventListener('change', apply);
  apply();
})();
</script>
</body>
</html>
`;
}

function maliciousPage() {
  const page = `<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>安全探测页面</title></head>
<body>
<h1>安全探测</h1>
<div id="probe-results">pending</div>
<script>
(function () {
  var out = {};
  try { out.cookieRead = document.cookie; } catch (error) { out.cookieRead = 'BLOCKED:' + error.name; }
  try { document.cookie = 'craft_probe=1'; out.cookieWrite = 'OK'; } catch (error) { out.cookieWrite = 'BLOCKED:' + error.name; }
  try { out.parentTitle = parent.document.title; } catch (error) { out.parentTitle = 'BLOCKED:' + error.name; }
  try { out.parentCookie = parent.document.cookie; } catch (error) { out.parentCookie = 'BLOCKED:' + error.name; }
  try { out.parentDom = parent.document.body ? 'OK' : 'OK_EMPTY'; } catch (error) { out.parentDom = 'BLOCKED:' + error.name; }
  var appOrigin = '__CRAFT_PROBE_APP_ORIGIN__';
  function render() { document.getElementById('probe-results').textContent = JSON.stringify(out); }
  render();
  // Probe 1: fetch the MAIN app origin (reachable without CSP; must be blocked here).
  fetch(appOrigin + '/api/v1/health').then(function () { out.fetchApp = 'OK'; render(); }, function (error) { out.fetchApp = 'BLOCKED:' + (error && error.name ? error.name : 'TypeError'); render(); });
  // Probe 2: fetch a foreign origin.
  fetch('https://example.com/').then(function () { out.fetchExternal = 'OK'; render(); }, function (error) { out.fetchExternal = 'BLOCKED:' + (error && error.name ? error.name : 'TypeError'); render(); });
})();
</script>
</body>
</html>
`;
  return page.replace('__CRAFT_PROBE_APP_ORIGIN__', probeAppOrigin);
}

function reportMarkdown(kind) {
  if (kind === 'quarterly') return '# 季度汇总\n\n- 2026-Q1 总计 300（东区 100 + 西区 200）\n';
  return '# 按月收入报告\n\n- 2026-01 东区 100\n- 2026-02 西区 200\n';
}

// The D02 spreadsheet fixture generator (python3 stdlib only, deterministic
// bytes: fixed zip timestamps). Writes output/report.xlsx (formulas WITH
// cached <v> values, date/thousands-typed cells, guarded CSV text),
// output/preview.json, output/manifest.json and the static escaped preview
// page output/index.html. Variant totals: monthly/quarterly 300, extra 350.
const SPREADSHEET_GENERATOR = String.raw`import datetime, html, json, sys, zipfile

variant = sys.argv[1]
DATA = {
    'monthly':   {'sheet': '月度销售', 'title': '按月销售表格', 'rows': [('2026-01-01', '东区', 100), ('2026-02-01', '西区', 200)]},
    'quarterly': {'sheet': '季度销售', 'title': '季度销售汇总表格', 'rows': [('2026-Q1', '东区', 100), ('2026-Q1', '西区', 200)]},
    'extra':     {'sheet': '月度销售', 'title': '按月销售表格（含3月）', 'rows': [('2026-01-01', '东区', 100), ('2026-02-01', '西区', 200), ('2026-03-01', '中区', 50)]},
}
cfg = DATA[variant]
S1 = cfg['sheet']
S2 = '汇总'
TOTAL = sum(r[2] for r in cfg['rows'])
NROWS = len(cfg['rows'])
LAST = 1 + NROWS
TOTALROW = LAST + 1
RANGE = 'C2:C%d' % LAST
EPOCH = datetime.date(1899, 12, 30)

strings = []
def sidx(text):
    if text not in strings:
        strings.append(text)
    return strings.index(text)

def esc(text):
    return text.replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;').replace('"', '&quot;')

def col(n):
    name = ''
    while n:
        n, r = divmod(n - 1, 26)
        name = chr(65 + r) + name
    return name

def serial(iso):
    y, m, d = (int(x) for x in iso.split('-'))
    return (datetime.date(y, m, d) - EPOCH).days

GUARD = "=cmd|'/c calc'!A0"
headers = ['月份', '地区', '收入', '备注']
rows1 = [[('s', h) for h in headers] + [('n', 1234, 2), ('s', GUARD)]]
display1 = [[{'text': h, 'type': 'text'} for h in headers] + [{'text': '1,234', 'type': 'number'}, {'text': GUARD, 'type': 'text'}]]
for label, region, amount in cfg['rows']:
    is_date = label.count('-') == 2
    row = []
    disp = []
    if is_date:
        row.append(('d', label))
        disp.append({'text': label, 'type': 'date'})
    else:
        row.append(('s', label))
        disp.append({'text': label, 'type': 'text'})
    row += [('s', region), ('n', amount), None]
    disp += [{'text': region, 'type': 'text'}, {'text': str(amount), 'type': 'number'}, {'text': '', 'type': 'empty'}]
    rows1.append(row)
    display1.append(disp)
rows1.append([('s', '合计'), None, ('f', 'SUM(%s)' % RANGE, TOTAL), None])
display1.append([{'text': '合计', 'type': 'text'}, {'text': '', 'type': 'empty'}, {'text': str(TOTAL), 'type': 'number'}, {'text': '', 'type': 'empty'}])

rows2 = [[('s', '项目'), ('s', '数值')]]
display2 = [[{'text': '项目', 'type': 'text'}, {'text': '数值', 'type': 'text'}]]
rows2.append([('s', '总额'), ('f', '%s!C%d' % (S1, TOTALROW), TOTAL)])
display2.append([{'text': '总额', 'type': 'text'}, {'text': str(TOTAL), 'type': 'number'}])
rows2.append([('s', '数据行'), ('f', 'COUNT(%s!%s)' % (S1, RANGE), NROWS)])
display2.append([{'text': '数据行', 'type': 'text'}, {'text': str(NROWS), 'type': 'number'}])

def sheet_xml(rows):
    out = ['<?xml version="1.0" encoding="UTF-8" standalone="yes"?>']
    out.append('<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>')
    for r, row in enumerate(rows, start=1):
        out.append('<row r="%d">' % r)
        for c, cell in enumerate(row, start=1):
            if cell is None:
                continue
            ref = '%s%d' % (col(c), r)
            kind = cell[0]
            if kind == 's':
                out.append('<c r="%s" t="s"><v>%d</v></c>' % (ref, sidx(cell[1])))
            elif kind == 'n':
                style = cell[2] if len(cell) > 2 else 0
                out.append('<c r="%s" s="%d"><v>%d</v></c>' % (ref, style, cell[1]))
            elif kind == 'd':
                out.append('<c r="%s" s="1"><v>%d</v></c>' % (ref, serial(cell[1])))
            else:
                out.append('<c r="%s" t="n"><f>%s</f><v>%d</v></c>' % (ref, esc(cell[1]), cell[2]))
        out.append('</row>')
    out.append('</sheetData></worksheet>')
    return ''.join(out).encode('utf-8')

def fixed(name, data):
    info = zipfile.ZipInfo(name, date_time=(2026, 1, 1, 0, 0, 0))
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = 0o600 << 16
    return info, data

parts = {}
parts['[Content_Types].xml'] = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
    '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
    '<Default Extension="xml" ContentType="application/xml"/>'
    '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
    '<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
    '<Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
    '<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>'
    '<Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>').encode('utf-8')
parts['_rels/.rels'] = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>').encode('utf-8')
parts['xl/workbook.xml'] = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">'
    '<sheets><sheet name="%s" sheetId="1" r:id="rId1"/><sheet name="%s" sheetId="2" r:id="rId2"/></sheets></workbook>' % (esc(S1), esc(S2))).encode('utf-8')
parts['xl/_rels/workbook.xml.rels'] = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>'
    '<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet2.xml"/>'
    '<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/>'
    '<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>').encode('utf-8')
parts['xl/worksheets/sheet1.xml'] = sheet_xml(rows1)
parts['xl/worksheets/sheet2.xml'] = sheet_xml(rows2)
parts['xl/styles.xml'] = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">'
    '<numFmts count="1"><numFmt numFmtId="164" formatCode="yyyy\-mm\-dd"/></numFmts>'
    '<fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts>'
    '<fills count="1"><fill><patternFill patternType="none"/></fill></fills>'
    '<borders count="1"><border/></borders>'
    '<cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>'
    '<cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>'
    '<xf numFmtId="164" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>'
    '<xf numFmtId="3" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/></cellXfs>'
    '<cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles></styleSheet>').encode('utf-8')

_ = sidx(GUARD)
shared = ['<?xml version="1.0" encoding="UTF-8" standalone="yes"?>',
    '<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="%d" uniqueCount="%d">' % (len(strings), len(strings))]
for text in strings:
    shared.append('<si><t xml:space="preserve">%s</t></si>' % esc(text))
shared.append('</sst>')
parts['xl/sharedStrings.xml'] = ''.join(shared).encode('utf-8')

with zipfile.ZipFile('output/report.xlsx', 'w') as zf:
    for name in ['[Content_Types].xml', '_rels/.rels', 'xl/workbook.xml', 'xl/_rels/workbook.xml.rels', 'xl/worksheets/sheet1.xml', 'xl/worksheets/sheet2.xml', 'xl/styles.xml', 'xl/sharedStrings.xml']:
        info, data = fixed(name, parts[name])
        zf.writestr(info, data)

preview = {
    'kind': 'spreadsheet',
    'sheets': [
        {'name': S1, 'rows': len(rows1), 'columns': 4, 'total': str(TOTAL), 'cells': display1},
        {'name': S2, 'rows': len(rows2), 'columns': 2, 'total': str(TOTAL), 'cells': display2},
    ],
    'page_rows': 1000,
    'page_count': 1,
}
with open('output/preview.json', 'w', encoding='utf-8') as fh:
    json.dump(preview, fh, ensure_ascii=False, indent=1)

manifest = {
    'kind': 'spreadsheet',
    'xlsx': 'report.xlsx',
    'preview': 'preview.json',
    'sheets': [
        {'name': S1, 'rows': len(rows1), 'columns': 4, 'formula_errors': [], 'recalculated': True},
        {'name': S2, 'rows': len(rows2), 'columns': 2, 'formula_errors': [], 'recalculated': True},
    ],
    'totals': {S1: str(TOTAL), S2: str(TOTAL)},
    'preview_totals': {S1: str(TOTAL), S2: str(TOTAL)},
    'checks': [
        {'name': 'recalc', 'status': 'passed', 'detail': 'fixture cache: LibreOffice recalc is proven in the image acceptance'},
        {'name': 'preview', 'status': 'passed', 'detail': 'preview.json totals match the workbook cache'},
        {'name': 'parse', 'status': 'passed', 'detail': 'OOXML parsed: formulas + cached values present'},
    ],
}
with open('output/manifest.json', 'w', encoding='utf-8') as fh:
    json.dump(manifest, fh, ensure_ascii=False, indent=1)

def table(cells):
    body = []
    for row in cells:
        tds = ''.join('<td data-type="%s">%s</td>' % (c['type'], html.escape(c['text'])) for c in row)
        body.append('<tr>%s</tr>' % tds)
    return '<table><tbody>%s</tbody></table>' % ''.join(body)

sections = []
tabs = []
for sheet in preview['sheets']:
    tabs.append('<button type="button" class="sheet-tab" data-sheet="%s">%s</button>' % (html.escape(sheet['name']), html.escape(sheet['name'])))
    sections.append('<section class="sheet-section" data-sheet="%s"><h2>%s</h2>'
        '<p class="sheet-meta">\u884c\u6570 <span class="sheet-rows">%d</span> \u00b7 \u5217\u6570 %d \u00b7 \u8ba1\u7b97\u603b\u989d <strong class="sheet-total">%s</strong></p>'
        '%s</section>' % (html.escape(sheet['name']), html.escape(sheet['name']), sheet['rows'], sheet['columns'], html.escape(sheet['total']), table(sheet['cells'])))
page = '''<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>%s</title><style>body{font-family:sans-serif;margin:1rem}table{border-collapse:collapse}td,th{border:1px solid #ccc;padding:2px 8px}td[data-type=number],td[data-type=currency],td[data-type=date]{text-align:right}.sheet-tab[aria-selected=true]{font-weight:700}.sheet-section[hidden]{display:none}</style></head>
<body>
<h1>%s</h1>
<div id="sheet-tabs" role="tablist">%s</div>
%s
<p><a class="xlsx-download" href="report.xlsx" download>下载 XLSX</a></p>
<script>
(function(){var tabs=[].slice.call(document.querySelectorAll('.sheet-tab'));
function apply(active){tabs.forEach(function(t){t.setAttribute('aria-selected',String(t.getAttribute('data-sheet')===active));});
document.querySelectorAll('.sheet-section').forEach(function(s){s.hidden=s.getAttribute('data-sheet')!==active;});}
tabs.forEach(function(t){t.addEventListener('click',function(){apply(t.getAttribute('data-sheet'));});});
apply(tabs[0].getAttribute('data-sheet'));})();
</script></body></html>''' % (html.escape(cfg['title']), html.escape(cfg['title']), ''.join(tabs), ''.join(sections))
with open('output/index.html', 'w', encoding='utf-8') as fh:
    fh.write(page)
print('spreadsheet round files written:', variant, 'total', TOTAL)`;

// One bash tool call runs the embedded generator for the requested variant;
// the runtime repoints the serve-relative output/ pointer at the active
// delegation workspace, so plain output/ paths are session-scoped.
function spreadsheetGeneratorCommand(variant) {
  return 'python3 - ' + variant + ' <<' + String.fromCharCode(39) + 'D02GEN' + String.fromCharCode(39) + String.fromCharCode(10)
    + SPREADSHEET_GENERATOR + String.fromCharCode(10) + 'D02GEN';
}
function plan(lastUser) {
  const malicious = lastUser.includes('安全探测') || lastUser.includes('恶意');
  const quarterly = lastUser.includes('季度');
  const writes = [];
  if (malicious) {
    writes.push({ filePath: 'output/index.html', content: maliciousPage() });
    return { kind: 'malicious', writes, final: '已生成安全探测页面 index.html。' };
  }
  // D02 spreadsheet acceptance: a table/XLSX goal switches the fixture to the
  // spreadsheet round, which emits ONE bash tool call running the embedded
  // stdlib-only python generator (the host has no openpyxl/LibreOffice: the
  // generator writes the OOXML directly, formulas carry fixture-simulated
  // cached values exactly like the image-verified LibreOffice recalc output).
  const spreadsheet = lastUser.includes('表格') || lastUser.toLowerCase().includes('xlsx');
  if (spreadsheet) {
    const variant = lastUser.includes('季度') ? 'quarterly'
      : (lastUser.includes('50') || lastUser.includes('加一行') || lastUser.includes('3月')) ? 'extra' : 'monthly';
    const total = variant === 'extra' ? 350 : 300;
    return {
      kind: 'spreadsheet-' + variant,
      writes,
      bash: spreadsheetGeneratorCommand(variant),
      final: variant === 'extra'
        ? '已更新 XLSX 数据表格（新增一行 50，重算总额 350）。'
        : '已生成 XLSX 数据表格（重算总额 ' + total + '，含 汇总 工作表）。',
    };
  }
  const page = quarterly ? quarterlyPage() : monthlyPage();
  writes.push({ filePath: 'output/index.html', content: page });
  writes.push({ filePath: 'output/report.md', content: reportMarkdown(quarterly ? 'quarterly' : 'monthly') });
  return {
    kind: quarterly ? 'quarterly' : 'monthly',
    writes,
    final: quarterly ? '已生成季度汇总网页报告（总额 300）。' : '已生成按月网页报告（总额 300，东区 100 + 西区 200）。',
  };
}

const server = createServer((request, response) => {
  if (request.method !== 'POST' || !request.url.endsWith('/chat/completions')) {
    response.writeHead(404);
    response.end('not found');
    return;
  }
  const chunks = [];
  request.on('data', (chunk) => chunks.push(chunk));
  request.on('end', () => {
    hits += 1;
    let body = {};
    try { body = JSON.parse(Buffer.concat(chunks).toString('utf8') || '{}'); } catch { /* keep {} */ }
    const messages = Array.isArray(body.messages) ? body.messages : [];
    const streaming = body.stream === true;
    const model = typeof body.model === 'string' ? body.model : 'mock/mock-model';
    const id = 'chatcmpl-sub-' + hits;
    const { lastUser, toolDone } = turnView(messages);
    const round = plan(lastUser);

    // The spreadsheet round carries a bash command instead of write payloads;
    // either way the first turn emits tool calls and the post-tool turn the
    // final text.
    const toolCalls = round.writes.map((write, index) => ({
      id: 'toolu-sub-' + hits + '-' + index,
      type: 'function',
      function: { name: 'write', arguments: JSON.stringify(write) },
    }));
    if (round.bash !== undefined) {
      toolCalls.push({
        id: 'toolu-sub-' + hits + '-bash',
        type: 'function',
        function: { name: 'bash', arguments: JSON.stringify({ command: round.bash }) },
      });
    }
    const wantsTools = !toolDone && toolCalls.length > 0;

    if (!streaming) {
      const message = wantsTools
        ? { role: 'assistant', content: '', tool_calls: toolCalls }
        : { role: 'assistant', content: round.final };
      const choice = { index: 0, message, finish_reason: wantsTools ? 'tool_calls' : 'stop' };
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(JSON.stringify({ id, object: 'chat.completion', created: Math.floor(Date.now() / 1000), model, choices: [choice] }));
      return;
    }

    response.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache', connection: 'keep-alive' });
    const send = (delta, finishReason) => {
      const choice = { index: 0, delta };
      if (finishReason !== undefined) choice.finish_reason = finishReason;
      response.write('data: ' + JSON.stringify({ id, object: 'chat.completion.chunk', created: Math.floor(Date.now() / 1000), model, choices: [choice] }) + '\n\n');
    };
    send({ role: 'assistant', content: '' }, undefined);
    if (wantsTools) {
      send({
        tool_calls: toolCalls.map((call, index) => ({ index, ...call })),
      }, 'tool_calls');
    } else {
      send({ content: round.final }, 'stop');
    }
    response.write('data: [DONE]\n\n');
    response.end();
  });
});

server.listen(port, '127.0.0.1', () => {
  process.stdout.write('sub-model fixture listening on 127.0.0.1:' + port + '\n');
});
