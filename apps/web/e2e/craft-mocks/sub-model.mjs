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

// The D03 slides fixture generator (python3 stdlib only, deterministic
// bytes: fixed zip timestamps). Writes output/report.pptx (slides + notes
// + embedded chart picture), output/report.pdf (fixture-render placeholder
// PDF, same 5-page shape), output/pages/page-N.svg (real per-page visual
// content: Chinese titles/bullets, a bar chart on page 3), preview.json,
// manifest.json and the static deck viewer output/index.html. Variants:
// proposal (round 1) and edit3 (round 2 changes ONLY page 3's conclusion
// line — every other stored part is byte-identical across rounds, which is
// exactly what the single-page-modification acceptance asserts). The host
// has no python-pptx/LibreOffice: this generator writes the same storage
// shape as the image-verified chain (python-pptx 1.0.2 deck + impress
// impress_pdf_Export PDF + pdftocairo page SVGs, see the D03 report), the
// way the D02 generator simulated recalculated cache values.
const SLIDES_GENERATOR = String.raw`import html, json, struct, sys, zipfile

variant = sys.argv[1]
CONCLUSION = {
    'proposal': '结论：分三期交付，第一期完成知识接入与权限打通',
    'edit3': '结论：首期直接上线智能问答，一次性交付见效',
}
BARS = [('第一季度', 120), ('第二季度', 150), ('第三季度', 180), ('第四季度', 210)]
SOURCES = [
    ('kc_customer', '客户背景资料'),
    ('kc_sales', '销售数据摘要'),
    ('kc_market', '市场分析摘录'),
    ('kc_plan', '实施计划素材'),
]
PAGES = [
    {'index': 1, 'title': '智绘云图客户方案：面向制造业的智能知识中台首期建设建议', 'notes': '', 'sources': [], 'min_font_pt': 28, 'images': 0},
    {'index': 2, 'title': '客户背景', 'notes': '来源：kc_customer', 'sources': ['kc_customer'], 'min_font_pt': 18, 'images': 0},
    {'index': 3, 'title': '核心方案', 'notes': '来源：kc_sales kc_market', 'sources': ['kc_sales', 'kc_market'], 'min_font_pt': 18, 'images': 1},
    {'index': 4, 'title': '实施计划', 'notes': '来源：kc_plan', 'sources': ['kc_plan'], 'min_font_pt': 18, 'images': 0},
    {'index': 5, 'title': '来源', 'notes': '来源页', 'sources': [s[0] for s in SOURCES], 'min_font_pt': 14, 'images': 0},
]
BODY = {
    2: ['客户为华东地区大型制造集团，下辖 12 家工厂', '现有知识分散在 30+ 个系统，检索平均耗时 15 分钟', '2026 年上半年销售数据：东区 100、西区 200，合计 300'],
    3: ['建设统一知识中台，接入 12 家工厂的文档与数据', '智能问答将检索耗时从 15 分钟压缩到 30 秒', CONCLUSION[variant]],
    4: ['第一期（1-2 月）：知识接入与权限打通', '第二期（3-4 月）：智能问答上线', '第三期（5-6 月）：决策驾驶舱'],
    5: [sid + ' ' + title for sid, title in SOURCES],
}
A = 'http://schemas.openxmlformats.org/drawingml/2006/main'
R = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'
P = 'http://schemas.openxmlformats.org/presentationml/2006/main'

def esc(text):
    return text.replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;').replace('"', '&quot;')

def svg_page(page):
    idx = page['index']
    out = ['<?xml version="1.0" encoding="UTF-8"?>',
        '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1280 720" font-family="Noto Sans CJK SC,PingFang SC,Microsoft YaHei,sans-serif">',
        '<rect width="1280" height="720" fill="#ffffff"/>',
        '<rect width="1280" height="12" fill="#1a5fb4"/>']
    title = page['title']
    if idx == 1:
        cut = 11
        out.append('<text x="64" y="150" font-size="42" font-weight="700" fill="#1a1a2e">' + esc(title[:cut]) + '</text>')
        out.append('<text x="64" y="212" font-size="42" font-weight="700" fill="#1a1a2e">' + esc(title[cut:]) + '</text>')
        out.append('<text x="64" y="330" font-size="24" fill="#444444">汇报人：Craft 团队</text>')
        out.append('<text x="64" y="374" font-size="24" fill="#444444">2026 年 9 月</text>')
        out.append('<text x="64" y="418" font-size="24" fill="#444444">本方案基于客户提供的背景与销售数据分析得出</text>')
    else:
        out.append('<text x="64" y="96" font-size="36" font-weight="700" fill="#1a1a2e">' + esc(title) + '</text>')
        y = 190
        size = 26 if idx != 5 else 22
        for item in BODY[idx]:
            out.append('<text x="80" y="' + str(y) + '" font-size="' + str(size) + '" fill="#333333">' + esc(item) + '</text>')
            y += 56
    if idx == 3:
        base_y, max_h, x0 = 600, 250, 700
        out.append('<text x="700" y="296" font-size="22" fill="#333333">季度收入（万元）</text>')
        for i, (label, value) in enumerate(BARS):
            h = int(max_h * value / 210.0)
            x = x0 + i * 120
            out.append('<rect x="' + str(x) + '" y="' + str(base_y - h) + '" width="72" height="' + str(h) + '" fill="#3584e4"/>')
            out.append('<text x="' + str(x + 36) + '" y="' + str(base_y + 28) + '" font-size="18" fill="#555555" text-anchor="middle">' + esc(label) + '</text>')
            out.append('<text x="' + str(x + 36) + '" y="' + str(base_y - h - 10) + '" font-size="18" fill="#555555" text-anchor="middle">' + str(value) + '</text>')
        out.append('<line x1="' + str(x0 - 10) + '" y1="' + str(base_y) + '" x2="1180" y2="' + str(base_y) + '" stroke="#999999"/>')
    out.append('<text x="64" y="692" font-size="16" fill="#888888">' + str(idx) + ' / ' + str(len(PAGES)) + '</text>')
    out.append('</svg>')
    return ('\n'.join(out) + '\n').encode('utf-8')

def slide_xml(page):
    idx = page['index']
    title_sz = 3200 if idx == 1 else 2800
    body_sz = 1400 if idx == 5 else 1800
    shapes = ['<p:sp><p:nvSpPr><p:cNvPr id="2" name="标题 ' + str(idx) + '"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>'
        '<p:spPr><a:xfrm><a:off x="457200" y="365738"/><a:ext cx="11277600" cy="1257300"/></a:xfrm>'
        '<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>'
        '<p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p><a:r><a:rPr lang="zh-CN" sz="' + str(title_sz) + '" b="1"/>'
        '<a:t>' + esc(page['title']) + '</a:t></a:r></a:p></p:txBody></p:sp>']
    if idx in BODY:
        runs = ''.join('<a:r><a:rPr lang="zh-CN" sz="' + str(body_sz) + '"/><a:t>' + esc(item) + '</a:t></a:r>' for item in BODY[idx])
        shapes.append('<p:sp><p:nvSpPr><p:cNvPr id="3" name="正文 ' + str(idx) + '"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>'
            '<p:spPr><a:xfrm><a:off x="548640" y="1828800"/><a:ext cx="11033760" cy="4114800"/></a:xfrm>'
            '<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>'
            '<p:txBody><a:bodyPr wrap="square"/><a:lstStyle/><a:p>' + runs + '</a:p></p:txBody></p:sp>')
    if idx == 3:
        shapes.append('<p:pic><p:nvPicPr><p:cNvPr id="4" name="季度收入图表"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>'
            '<p:blipFill><a:blip r:embed="rIdChart"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>'
            '<p:spPr><a:xfrm><a:off x="6400800" y="2286000"/><a:ext cx="4800600" cy="2971800"/></a:xfrm>'
            '<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>')
    return ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
        '<p:sld xmlns:a="' + A + '" xmlns:r="' + R + '" xmlns:p="' + P + '"><p:cSld><p:spTree>'
        '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>'
        + ''.join(shapes) + '</p:spTree></p:cSld><p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sld>').encode('utf-8')

THEME = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
    '<a:theme xmlns:a="' + A + '" name="Craft"><a:themeElements>'
    '<a:clrScheme name="Craft"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>'
    '<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2>'
    '<a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1>'
    '<a:accent2><a:srgbClr val="ED7D31"/></a:accent2><a:accent3><a:srgbClr val="A5A5A5"/></a:accent3>'
    '<a:accent4><a:srgbClr val="FFC000"/></a:accent4><a:accent5><a:srgbClr val="5B9BD5"/></a:accent5>'
    '<a:accent6><a:srgbClr val="70AD47"/></a:accent6><a:hlink><a:srgbClr val="0563C1"/></a:hlink>'
    '<a:folHlink><a:srgbClr val="954F72"/></a:folHlink></a:clrScheme>'
    '<a:fontScheme name="Craft"><a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>'
    '<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont></a:fontScheme>'
    '<a:fmtScheme name="Craft"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>'
    '<a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>'
    '<a:lnStyleLst><a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln><a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln>'
    '<a:ln><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>'
    '<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle><a:effectStyle><a:effectLst/></a:effectStyle>'
    '<a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>'
    '<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill><a:solidFill><a:schemeClr val="phClr"/></a:solidFill>'
    '<a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst></a:fmtScheme></a:themeElements></a:theme>').encode('utf-8')

MASTER = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
    '<p:sldMaster xmlns:a="' + A + '" xmlns:r="' + R + '" xmlns:p="' + P + '"><p:cSld><p:spTree>'
    '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>'
    '<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" '
    'accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/>'
    '<p:sldLayoutIdLst><p:sldLayoutId id="2147483649" r:id="rId1"/></p:sldLayoutIdLst></p:sldMaster>').encode('utf-8')

LAYOUT = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
    '<p:sldLayout xmlns:a="' + A + '" xmlns:r="' + R + '" xmlns:p="' + P + '" type="blank"><p:cSld name="Blank"><p:spTree>'
    '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>'
    '<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr></p:sldLayout>').encode('utf-8')

NOTES_MASTER = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
    '<p:notesMaster xmlns:a="' + A + '" xmlns:r="' + R + '" xmlns:p="' + P + '"><p:cSld><p:spTree>'
    '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>'
    '<p:clrMap bg1="lt1" tx1="dk1" bg2="lt2" tx2="dk2" accent1="accent1" accent2="accent2" accent3="accent3" '
    'accent4="accent4" accent5="accent5" accent6="accent6" hlink="hlink" folHlink="folHlink"/></p:notesMaster>').encode('utf-8')

NOTES3 = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
    '<p:notes xmlns:a="' + A + '" xmlns:r="' + R + '" xmlns:p="' + P + '"><p:cSld><p:spTree>'
    '<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/>'
    '<p:sp><p:nvSpPr><p:cNvPr id="2" name="备注"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>'
    '<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="5486400" cy="4114800"/></a:xfrm>'
    '<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>'
    '<p:txBody><a:bodyPr/><a:lstStyle/><a:p><a:r><a:rPr lang="zh-CN" sz="1200"/>'
    '<a:t>' + esc('来源：kc_sales kc_market。' + CONCLUSION[variant]) + '</a:t></a:r></a:p></p:txBody></p:sp>'
    '</p:spTree></p:cSld></p:notes>').encode('utf-8')

def png_bytes():
    def chunk(tag, data):
        return struct.pack('>I', len(data)) + tag + data + struct.pack('>I', zlib_crc32(tag + data))
    ihdr = struct.pack('>IIBBBBB', 8, 8, 8, 2, 0, 0, 0)
    raw = b''.join(b'\x00' + bytes([40 + i * 8, 90, 160]) for i in range(8))
    return b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', ihdr) + chunk(b'IDAT', zlib_compress(raw)) + chunk(b'IEND', b'')

import zlib as _zlib
zlib_crc32 = lambda d: _zlib.crc32(d) & 0xffffffff
zlib_compress = lambda d: _zlib.compress(d, 9)

def pdf_bytes(pages):
    objs = [b'<< /Type /Catalog /Pages 2 0 R >>']
    kids = ' '.join(str(3 + i) + ' 0 R' for i in range(pages))
    objs.append(('<< /Type /Pages /Kids [' + kids + '] /Count ' + str(pages) + ' >>').encode('utf-8'))
    for _ in range(pages):
        objs.append(b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 960 540] /Resources << >> >>')
    out = bytearray(b'%PDF-1.4\n')
    offsets = []
    for n, body in enumerate(objs, start=1):
        offsets.append(len(out))
        out += (str(n) + ' 0 obj\n').encode('utf-8') + body + b'\nendobj\n'
    xref_at = len(out)
    out += ('xref\n0 ' + str(len(objs) + 1) + '\n').encode('utf-8')
    out += b'0000000000 65535 f \n'
    for off in offsets:
        out += (('%010d 00000 n \n' % off)).encode('utf-8')
    out += ('trailer\n<< /Size ' + str(len(objs) + 1) + ' /Root 1 0 R >>\nstartxref\n' + str(xref_at) + '\n%%EOF\n').encode('utf-8')
    return bytes(out)

CT = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
    '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
    '<Default Extension="xml" ContentType="application/xml"/>'
    '<Default Extension="png" ContentType="image/png"/>'
    '<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>'
    + ''.join('<Override PartName="/ppt/slides/slide' + str(i) + '.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>' for i in range(1, 6))
    + '<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>'
    + '<Override PartName="/ppt/slideLayouts/slideLayout1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>'
    + '<Override PartName="/ppt/notesMasters/notesMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesMaster+xml"/>'
    + '<Override PartName="/ppt/notesSlides/notesSlide3.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/>'
    + '<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/></Types>').encode('utf-8')

RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="' + R + '/officeDocument" Target="ppt/presentation.xml"/></Relationships>').encode('utf-8')

PRES = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>\n'
    '<p:presentation xmlns:a="' + A + '" xmlns:r="' + R + '" xmlns:p="' + P + '">'
    '<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId6"/></p:sldMasterIdLst>'
    '<p:notesMasterIdLst><p:notesMasterId r:id="rId7"/></p:notesMasterIdLst>'
    '<p:sldIdLst>' + ''.join('<p:sldId id="' + str(255 + i) + '" r:id="rId' + str(i) + '"/>' for i in range(1, 6)) + '</p:sldIdLst>'
    '<p:sldSz cx="12192000" cy="6858000"/><p:notesSz cx="6858000" cy="9144000"/></p:presentation>').encode('utf-8')

PRES_RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    + ''.join('<Relationship Id="rId' + str(i) + '" Type="' + P + '/slide" Target="slides/slide' + str(i) + '.xml"/>' for i in range(1, 6))
    + '<Relationship Id="rId6" Type="' + P + '/slideMaster" Target="slideMasters/slideMaster1.xml"/>'
    + '<Relationship Id="rId7" Type="' + P + '/notesMaster" Target="notesMasters/notesMaster1.xml"/></Relationships>').encode('utf-8')

MASTER_RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="' + P + '/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>'
    '<Relationship Id="rId2" Type="' + R + '/theme" Target="../theme/theme1.xml"/></Relationships>').encode('utf-8')

LAYOUT_RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="' + P + '/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>').encode('utf-8')

NOTES_MASTER_RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="' + R + '/theme" Target="../theme/theme1.xml"/></Relationships>').encode('utf-8')

SLIDE3_RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rIdChart" Type="' + R + '/image" Target="../media/chart-1.png"/>'
    '<Relationship Id="rIdSlide" Type="' + P + '/slide" Target="../slides/slide3.xml"/></Relationships>').encode('utf-8')

NOTES3_RELS = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
    '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
    '<Relationship Id="rId1" Type="' + P + '/notesMaster" Target="../notesMasters/notesMaster1.xml"/></Relationships>').encode('utf-8')

parts = {
    '[Content_Types].xml': CT,
    '_rels/.rels': RELS,
    'ppt/presentation.xml': PRES,
    'ppt/_rels/presentation.xml.rels': PRES_RELS,
    'ppt/slideMasters/slideMaster1.xml': MASTER,
    'ppt/slideMasters/_rels/slideMaster1.xml.rels': MASTER_RELS,
    'ppt/slideLayouts/slideLayout1.xml': LAYOUT,
    'ppt/slideLayouts/_rels/slideLayout1.xml.rels': LAYOUT_RELS,
    'ppt/theme/theme1.xml': THEME,
    'ppt/notesMasters/notesMaster1.xml': NOTES_MASTER,
    'ppt/notesMasters/_rels/notesMaster1.xml.rels': NOTES_MASTER_RELS,
    'ppt/slides/_rels/slide3.xml.rels': SLIDE3_RELS,
    'ppt/notesSlides/notesSlide3.xml': NOTES3,
    'ppt/notesSlides/_rels/notesSlide3.xml.rels': NOTES3_RELS,
    'ppt/media/chart-1.png': png_bytes(),
}
for page in PAGES:
    parts['ppt/slides/slide' + str(page['index']) + '.xml'] = slide_xml(page)

order = ['[Content_Types].xml', '_rels/.rels', 'ppt/presentation.xml', 'ppt/_rels/presentation.xml.rels',
    'ppt/slideMasters/slideMaster1.xml', 'ppt/slideMasters/_rels/slideMaster1.xml.rels',
    'ppt/slideLayouts/slideLayout1.xml', 'ppt/slideLayouts/_rels/slideLayout1.xml.rels',
    'ppt/theme/theme1.xml', 'ppt/notesMasters/notesMaster1.xml', 'ppt/notesMasters/_rels/notesMaster1.xml.rels',
    'ppt/slides/slide1.xml', 'ppt/slides/slide2.xml', 'ppt/slides/slide3.xml', 'ppt/slides/slide4.xml', 'ppt/slides/slide5.xml',
    'ppt/slides/_rels/slide3.xml.rels', 'ppt/notesSlides/notesSlide3.xml', 'ppt/notesSlides/_rels/notesSlide3.xml.rels',
    'ppt/media/chart-1.png']

with zipfile.ZipFile('output/report.pptx', 'w') as zf:
    for name in order:
        info = zipfile.ZipInfo(name, date_time=(2026, 1, 1, 0, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = 0o600 << 16
        zf.writestr(info, parts[name])

with open('output/report.pdf', 'wb') as fh:
    fh.write(pdf_bytes(len(PAGES)))

import os
os.makedirs('output/pages', exist_ok=True)
for page in PAGES:
    with open('output/pages/page-' + str(page['index']) + '.svg', 'wb') as fh:
        fh.write(svg_page(page))

preview = {
    'kind': 'slides',
    'slide_count': len(PAGES),
    'pages': [{'index': p['index'], 'title': p['title'], 'image': 'pages/page-' + str(p['index']) + '.svg',
        'notes': p['notes'], 'sources': p['sources']} for p in PAGES],
    'overflow_pages': [],
}
with open('output/preview.json', 'w', encoding='utf-8') as fh:
    json.dump(preview, fh, ensure_ascii=False, indent=1)

manifest = {
    'kind': 'slides',
    'pptx': 'report.pptx',
    'pdf': 'report.pdf',
    'preview': 'preview.json',
    'pptx_ref': 'resource://report.pptx',
    'page_refs': ['resource://pages/page-' + str(p['index']) + '.svg' for p in PAGES],
    'slide_count': len(PAGES),
    'overflow_pages': [],
    'pages': [{'index': p['index'], 'title': p['title'], 'notes': p['notes'], 'sources': p['sources'],
        'min_font_pt': p['min_font_pt'], 'images': p['images']} for p in PAGES],
    'sources': [{'id': sid, 'title': title} for sid, title in SOURCES],
    'checks': [
        {'name': 'render', 'status': 'passed', 'detail': 'fixture render: impress render + pdftocairo export are proven in the image acceptance'},
        {'name': 'pages', 'status': 'passed', 'detail': '5/5 页边界/字号/图表齐备'},
        {'name': 'sources', 'status': 'passed', 'detail': '每页引用均可解析到来源页与讲者备注的 kc_ id'},
        {'name': 'visual', 'status': 'not_run', 'detail': '排版美观/视觉层次需要人工验收'},
    ],
}
with open('output/manifest.json', 'w', encoding='utf-8') as fh:
    json.dump(manifest, fh, ensure_ascii=False, indent=1)

deck_title = PAGES[0]['title']
pages_js = json.dumps(preview['pages'], ensure_ascii=False)
page = '''<!DOCTYPE html>
<html lang="zh">
<head><meta charset="utf-8"><title>__TITLE__</title><style>
body{font-family:'Noto Sans CJK SC','PingFang SC',sans-serif;margin:0;background:#f4f5f7}
header{display:flex;align-items:center;gap:12px;padding:8px 16px;background:#fff;border-bottom:1px solid #ddd;flex-wrap:wrap}
header h1{font-size:16px;margin:0;flex:1 1 320px}
.stage{display:flex;flex-direction:column;align-items:center;padding:12px}
.stage img.slide{max-width:100%;height:auto;border:1px solid #ccc;background:#fff}
.slide-title{font-size:14px;color:#333;margin-top:6px}
.thumbs{display:flex;gap:8px;padding:8px 16px;overflow-x:auto}
.slide-thumb{display:flex;flex-direction:column;align-items:center;border:2px solid transparent;background:#fff;padding:2px;cursor:pointer}
.slide-thumb[aria-selected=true]{border-color:#1a5fb4}
.slide-thumb img{width:120px;height:67px;object-fit:contain;border:1px solid #eee}
.nav{display:flex;align-items:center;gap:12px;padding:8px 16px}
a.download{margin-left:8px}
</style></head>
<body>
<header><h1>__TITLE_HTML__</h1>
<span class="page-number" id="slide-page-number" data-testid="slide-page-number">第 1/5 页</span>
<a class="download" data-testid="slide-download-pptx" href="report.pptx" download>下载 PPTX</a>
<a class="download" data-testid="slide-download-pdf" href="report.pdf" download>下载 PDF</a>
<button type="button" id="slide-modify" data-testid="slide-modify" data-page="1">修改此页</button>
</header>
<div class="stage"><img class="slide" id="slide-image" data-testid="slide-image" alt=""><p class="slide-title" id="slide-title" data-testid="slide-title"></p></div>
<div class="thumbs" id="slide-thumbs" data-testid="slide-thumbs"></div>
<div class="nav">
<button type="button" id="slide-prev" data-testid="slide-prev">上一页</button>
<button type="button" id="slide-next" data-testid="slide-next">下一页</button>
<span class="hint">← → 键翻页</span>
</div>
<script>
var PAGES = __PAGES__;
(function () {
  var now = 0;
  var img = document.getElementById('slide-image');
  var title = document.getElementById('slide-title');
  var number = document.getElementById('slide-page-number');
  var modify = document.getElementById('slide-modify');
  var thumbs = document.getElementById('slide-thumbs');
  PAGES.forEach(function (p, i) {
    var b = document.createElement('button');
    b.type = 'button'; b.className = 'slide-thumb';
    b.setAttribute('data-testid', 'slide-thumb');
    b.setAttribute('data-page', String(p.index));
    var im = document.createElement('img'); im.src = p.image; im.alt = p.index + '. ' + p.title;
    var s = document.createElement('span'); s.textContent = String(p.index);
    b.appendChild(im); b.appendChild(s);
    b.addEventListener('click', function () { show(i); });
    thumbs.appendChild(b);
  });
  function show(i) {
    now = Math.min(Math.max(i, 0), PAGES.length - 1);
    var p = PAGES[now];
    img.src = p.image; img.alt = p.title;
    title.textContent = p.title + (p.sources.length ? ' · 来源: ' + p.sources.join(' ') : '');
    number.textContent = '第 ' + (now + 1) + '/' + PAGES.length + ' 页';
    modify.setAttribute('data-page', String(p.index));
    Array.prototype.forEach.call(thumbs.children, function (el, j) { el.setAttribute('aria-selected', String(j === now)); });
    document.getElementById('slide-prev').disabled = now === 0;
    document.getElementById('slide-next').disabled = now === PAGES.length - 1;
  }
  document.getElementById('slide-prev').addEventListener('click', function () { show(now - 1); });
  document.getElementById('slide-next').addEventListener('click', function () { show(now + 1); });
  modify.addEventListener('click', function () {
    window.parent.postMessage({ type: 'craft:slides:modify', page: Number(modify.getAttribute('data-page')) }, '*');
  });
  document.addEventListener('keydown', function (event) {
    if (event.key === 'ArrowLeft') show(now - 1);
    if (event.key === 'ArrowRight') show(now + 1);
  });
  show(0);
})();
</script></body></html>'''.replace('__TITLE__', html.escape(deck_title)).replace('__TITLE_HTML__', html.escape(deck_title)).replace('__PAGES__', pages_js)
with open('output/index.html', 'w', encoding='utf-8') as fh:
    fh.write(page)
print('slides round files written:', variant, 'pages', len(PAGES))`;

// The D01 document fixture generator (python3 stdlib only, deterministic
// bytes: fixed zip timestamps). Writes output/report.md (the editable
// source) and output/report.docx (the same-run export: Title/Heading1
// styled OOXML with the CJK font declared in styles.xml, an 8-row
// customer table and the two kc_ citations) plus output/manifest.json with
// the four document gate checks (the 'broken' variant stores a corrupt
// export and an honest export=failed manifest for the failure
// acceptance). NO index.html: the document kind's preview surface is the
// workbench component, not a static page. The citation ids come from argv
// - the dispatch extracted any kc_ ids the staged knowledge manifest
// actually carried, else the deterministic fixture-shaped fallback (the
// python-docx rendering itself is proven inside the craft image, see the
// D01 report).
const DOCUMENT_GENERATOR = String.raw`import datetime, json, sys, zipfile

variant = sys.argv[1]
cit1, cit2 = sys.argv[2], sys.argv[3]
CJK = 'Noto Sans CJK SC'
ROWS = [
    ['客户', '行业', '部署周期', '满意度'],
    ['华东制造集团', '制造业', '6 周', '96%'],
    ['南方电网某供电局', '能源', '6 周', '96%'],
    ['西部物流股份', '物流', '5 周', '95%'],
    ['京津联合银行', '金融', '7 周', '97%'],
    ['半岛文旅集团', '文旅', '6 周', '96%'],
    ['中原医院联盟', '医疗', '8 周', '94%'],
    ['远东零售连锁', '零售', '6 周', '96%'],
]
FAQ = [
    ('数据如何保留？', '客户数据全部留在客户自有环境，平台不做二次留存。'),
    ('能否扩容？', '支持按知识库扩容，已有 12 家客户平滑扩容经验。'),
]

def esc(text):
    return text.replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;').replace('"', '&quot;')

headings = ['公司概况', '产品能力', '实施效果']
if variant == 'faq':
    headings.append('常见问题')

md = ['# 智绘云图客户介绍', '',
      '摘要：面向企业知识中台建设的客户介绍，覆盖公司概况、产品能力与实施效果，全部数字来自两份知识资料（' + cit1 + '、' + cit2 + '）。', '']
md += ['## 公司概况', '', '智绘云图已服务 12 家企业客户，平均满意度 96%（' + cit1 + '）。', '']
md += ['## 产品能力', '', '知识入库、混合检索与受控生成一体化，标准交付周期 6 周（' + cit2 + '）。', '']
md += ['## 实施效果', '', '代表性客户实施周期与满意度如下：', '',
       '| ' + ' | '.join(ROWS[0]) + ' |', '| ' + ' | '.join(['---'] * 4) + ' |']
for r in ROWS[1:]:
    md.append('| ' + ' | '.join(r) + ' |')
md.append('')
if variant == 'faq':
    md += ['## 常见问题', '']
    for q, a in FAQ:
        md += ['- ' + q + ' ' + a]
    md.append('')
md += ['## 来源', '', '- ' + cit1 + ' 公司介绍资料', '- ' + cit2 + ' 客户案例资料', '']
with open('output/report.md', 'w', encoding='utf-8') as fh:
    fh.write('\n'.join(md))

if variant == 'broken':
    # The failure-acceptance round: the stored export is NOT a real OOXML
    # package, and the manifest says so honestly (export=failed). The
    # server-side gate refuses the whole round - no version may publish.
    with open('output/report.docx', 'wb') as fh:
        fh.write('corrupt docx bytes for the failure acceptance (not a zip)'.encode('utf-8'))
    manifest = {
        'kind': 'document', 'markdown': 'report.md', 'docx': 'report.docx',
        'markdown_ref': 'resource://report.md', 'docx_ref': 'resource://report.docx',
        'headings': ['智绘云图客户介绍'] + headings + ['来源'], 'citation_ids': [cit1, cit2],
        'checks': [
            {'name': 'generate', 'status': 'passed', 'detail': 'report.md authored'},
            {'name': 'modify', 'status': 'passed', 'detail': 'numbers kept verbatim'},
            {'name': 'preview', 'status': 'passed', 'detail': 'markdown loadable'},
            {'name': 'export', 'status': 'failed', 'detail': 'stored report.docx is not a ZIP package (corrupt export)'},
        ],
    }
    with open('output/manifest.json', 'w', encoding='utf-8') as fh:
        json.dump(manifest, fh, ensure_ascii=False, indent=1)
    print('document round files written: broken (export failed on purpose)')
    sys.exit(0)

def para(text, style=None):
    ppr = '<w:pPr><w:pStyle w:val="%s"/></w:pPr>' % style if style else ''
    return '<w:p>' + ppr + '<w:r><w:t xml:space="preserve">' + esc(text) + '</w:t></w:r></w:p>'

body = [para('智绘云图客户介绍', 'Title'),
        para('摘要：面向企业知识中台建设的客户介绍，覆盖公司概况、产品能力与实施效果，全部数字来自两份知识资料（' + cit1 + '、' + cit2 + '）。')]
body.append(para('公司概况', 'Heading1'))
body.append(para('智绘云图已服务 12 家企业客户，平均满意度 96%（' + cit1 + '）。'))
body.append(para('产品能力', 'Heading1'))
body.append(para('知识入库、混合检索与受控生成一体化，标准交付周期 6 周（' + cit2 + '）。'))
body.append(para('实施效果', 'Heading1'))
body.append(para('代表性客户实施周期与满意度如下：'))
tbl = ['<w:tbl><w:tblPr><w:tblStyle w:val="TableGrid"/><w:tblW w:w="0" w:type="auto"/><w:tblBorders>' + ''.join('<w:%s w:val="single" w:sz="4" w:space="0" w:color="auto"/>' % e for e in ('top','left','bottom','right','insideH','insideV')) + '</w:tblBorders></w:tblPr>']
for row in ROWS:
    tbl.append('<w:tr>' + ''.join('<w:tc><w:tcPr/>' + para(cell) + '</w:tc>' for cell in row) + '</w:tr>')
tbl.append('</w:tbl>')
body.append(''.join(tbl))
if variant == 'faq':
    body.append(para('常见问题', 'Heading1'))
    for q, a in FAQ:
        body.append(para('问：' + q + ' 答：' + a))
body.append(para('来源', 'Heading1'))
body.append(para('本介绍引用的知识资料：' + cit1 + '（公司介绍资料）、' + cit2 + '（客户案例资料）。'))
document = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>'
            + ''.join(body) + '<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr></w:body></w:document>')

def style(style_id, name, size, bold):
    return ('<w:style w:type="paragraph" w:styleId="%s"><w:name w:val="%s"/><w:qFormat/>'
            '<w:pPr><w:spacing w:before="120" w:after="120"/></w:pPr>'
            '<w:rPr><w:rFonts w:ascii="%s" w:hAnsi="%s" w:eastAsia="%s"/>%s<w:sz w:val="%d"/></w:rPr></w:style>'
            ) % (style_id, name, CJK, CJK, CJK, '<w:b/>' if bold else '', size)
styles = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
          '<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
          + style('Normal', 'Normal', 21, False)
          + style('Title', 'Title', 44, True)
          + style('Heading1', 'heading 1', 28, True)
          + style('TableGrid', 'Table Grid', 21, False)
          + '</w:styles>')
content_types = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
                 '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
                 '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
                 '<Default Extension="xml" ContentType="application/xml"/>'
                 '<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>'
                 '<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>'
                 '<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>'
                 '<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>'
                 '</Types>')
rels = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
        '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>'
        '<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>'
        '<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>'
        '</Relationships>')
doc_rels = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            '<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>'
            '</Relationships>')
core = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        '<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/">'
        '<dc:title>智绘云图客户介绍</dc:title><dc:creator>craft-document fixture</dc:creator></cp:coreProperties>')
app = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
       '<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"><Application>craft-document-fixture</Application></Properties>')

parts = {'[Content_Types].xml': content_types, '_rels/.rels': rels,
         'word/document.xml': document, 'word/styles.xml': styles,
         'word/_rels/document.xml.rels': doc_rels,
         'docProps/core.xml': core, 'docProps/app.xml': app}
with zipfile.ZipFile('output/report.docx', 'w') as zf:
    for name in sorted(parts):
        info = zipfile.ZipInfo(name, date_time=(2026, 9, 13, 0, 0, 0))
        info.compress_type = zipfile.ZIP_DEFLATED
        info.external_attr = 0o600 << 16
        zf.writestr(info, parts[name].encode('utf-8'))

manifest_headings = ['智绘云图客户介绍'] + headings + ['来源']
manifest = {
    'kind': 'document', 'markdown': 'report.md', 'docx': 'report.docx',
    'markdown_ref': 'resource://report.md', 'docx_ref': 'resource://report.docx',
    'headings': manifest_headings, 'citation_ids': [cit1, cit2],
    'checks': [
        {'name': 'generate', 'status': 'passed', 'detail': 'report.md authored from the two staged materials: title/abstract/sections/8-row table/sources'},
        {'name': 'modify', 'status': 'passed', 'detail': 'markdown is the continuation source; docx keeps 12/96%/6 周 verbatim'},
        {'name': 'preview', 'status': 'passed', 'detail': 'markdown + manifest loadable by the document view (safe renderer)'},
        {'name': 'export', 'status': 'passed', 'detail': 'stored report.docx verified against the markdown (OOXML read-back proven in the craft image)'},
    ],
}
with open('output/manifest.json', 'w', encoding='utf-8') as fh:
    json.dump(manifest, fh, ensure_ascii=False, indent=1)
print('document round files written:', variant, 'citations', cit1, cit2)`;

// One bash tool call runs the embedded generator for the requested variant;
// the runtime repoints the serve-relative output/ pointer at the active
// delegation workspace, so plain output/ paths are session-scoped.
function spreadsheetGeneratorCommand(variant) {
  return 'python3 - ' + variant + ' <<' + String.fromCharCode(39) + 'D02GEN' + String.fromCharCode(39) + String.fromCharCode(10)
    + SPREADSHEET_GENERATOR + String.fromCharCode(10) + 'D02GEN';
}
function slidesGeneratorCommand(variant) {
  return 'python3 - ' + variant + ' <<' + String.fromCharCode(39) + 'D03GEN' + String.fromCharCode(39) + String.fromCharCode(10)
    + SLIDES_GENERATOR + String.fromCharCode(10) + 'D03GEN';
}
// The document generator takes the two citation ids as argv: the dispatch
// extracts any kc_ ids the staged knowledge manifest actually carried from
// the delegation prompt and falls back to the deterministic fixture-shaped
// ids otherwise (the python-docx rendering chain is proven inside the image).
function documentGeneratorCommand(variant, cit1, cit2) {
  return 'python3 - ' + variant + ' ' + cit1 + ' ' + cit2 + ' <<' + String.fromCharCode(39) + 'D01GEN' + String.fromCharCode(39) + String.fromCharCode(10)
    + DOCUMENT_GENERATOR + String.fromCharCode(10) + 'D01GEN';
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
  // D03 slides acceptance: a presentation/PPT goal switches the fixture to
  // the slides round — ONE bash tool call running the embedded stdlib-only
  // python generator (the host has no python-pptx/LibreOffice: the generator
  // writes the deck OOXML, a fixture-render PDF and per-page SVG previews
  // directly, same storage shape as the image-verified chain).
  const slides = lastUser.includes('演示') || lastUser.toLowerCase().includes('ppt') || lastUser.includes('幻灯');
  if (slides) {
    const variant = (lastUser.includes('第3页') || lastUser.includes('第三页') || lastUser.includes('结论')) ? 'edit3' : 'proposal';
    return {
      kind: 'slides-' + variant,
      writes,
      bash: slidesGeneratorCommand(variant),
      final: variant === 'edit3'
        ? '已只修改第 3 页结论并重新渲染，其余页保持不变。'
        : '已生成 5 页客户方案演示稿（PPTX + PDF + 逐页预览与来源页）。',
    };
  }
  // D01 document acceptance: a 客户介绍/DOCX goal switches the fixture to
  // the document round - ONE bash tool call running the embedded stdlib-only
  // python generator (the host has no python-docx: the generator writes the
  // DOCX OOXML directly in the same storage shape as the image-verified
  // python-docx chain). Citations reuse kc_ ids from the staged knowledge
  // manifest when the prompt carries them; numbers 12/96%/6 周 stay
  // verbatim across the FAQ round; the 损坏 goal is the failure
  // acceptance (corrupt export, honest export=failed manifest).
  const document = lastUser.includes('客户介绍') || lastUser.toLowerCase().includes('docx') || lastUser.toLowerCase().includes('word') || lastUser.includes('损坏');
  if (document) {
    const ids = [...lastUser.matchAll(/kc_[0-9a-f]{24}/g)].map((m) => m[0]);
    const cit1 = ids[0] ?? 'kc_8833411d564eabdf5b60e012';
    const cit2 = ids[1] ?? 'kc_9d6cf870db056f0bb909a0d6';
    const variant = lastUser.includes('损坏') ? 'broken' : (lastUser.includes('FAQ') || lastUser.includes('常见问题')) ? 'faq' : 'intro';
    return {
      kind: 'document-' + variant,
      writes,
      bash: documentGeneratorCommand(variant, cit1, cit2),
      final: variant === 'broken'
        ? '本轮导出失败：存储的 DOCX 不是有效包，manifest 如实记录 export=failed，本轮不得发布版本。'
        : variant === 'faq'
        ? '已在客户介绍中新增常见问题 FAQ 章节，原有数字保持不变（12 家客户 / 96% / 6 周），DOCX 已同步导出。'
        : '已用两份知识资料生成客户介绍（3 个章节 + 8 行客户表格 + 2 个可点引用），DOCX 已导出。',
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
