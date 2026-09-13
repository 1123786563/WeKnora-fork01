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

function plan(lastUser) {
  const malicious = lastUser.includes('安全探测') || lastUser.includes('恶意');
  const quarterly = lastUser.includes('季度');
  const writes = [];
  if (malicious) {
    writes.push({ filePath: 'output/index.html', content: maliciousPage() });
    return { kind: 'malicious', writes, final: '已生成安全探测页面 index.html。' };
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

    if (!streaming) {
      const message = toolDone || round.writes.length === 0
        ? { role: 'assistant', content: toolDone ? round.final : round.final }
        : {
            role: 'assistant',
            content: '',
            tool_calls: round.writes.map((write, index) => ({
              id: 'toolu-sub-' + hits + '-' + index,
              type: 'function',
              function: { name: 'write', arguments: JSON.stringify(write) },
            })),
          };
      const choice = { index: 0, message, finish_reason: !toolDone && round.writes.length > 0 ? 'tool_calls' : 'stop' };
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
    if (!toolDone && round.writes.length > 0) {
      send({
        tool_calls: round.writes.map((write, index) => ({
          index,
          id: 'toolu-sub-' + hits + '-' + index,
          type: 'function',
          function: { name: 'write', arguments: JSON.stringify(write) },
        })),
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
