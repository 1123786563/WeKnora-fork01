// W06 controlled MAIN-agent model endpoint (OpenAI-compatible).
//
// The main agent of a craft run must reliably decide to delegate: this
// scripted endpoint answers every /chat/completions call with exactly one
// craft_delegate tool call on the first round and a final text answer once a
// tool result is present. It is the browser-acceptance counterpart of R05's
// scriptedCraftModel fixture: the sub-executor stays the pinned OpenCode
// chain (mock or real free model), never this endpoint.
//
// Credentials: none. It binds 127.0.0.1 only and is started/ stopped by the
// W06 stack harness with a recorded PID.
import { createServer } from 'node:http';

const port = Number(process.argv[2] ?? 41871);
let hits = 0;

function lastUserText(messages) {
  for (let i = messages.length - 1; i >= 0; i -= 1) {
    const message = messages[i];
    if (message.role === 'user') {
      if (typeof message.content === 'string') return message.content;
      if (Array.isArray(message.content)) {
        const text = message.content.filter((part) => part && part.type === 'text').map((part) => part.text).join('\n');
        if (text !== '') return text;
      }
      return '';
    }
  }
  return '';
}

function sawToolResult(messages) {
  for (const message of messages) {
    if (message.role === 'tool') return true;
    // openai-compatible tool results sometimes arrive as user-role tool-result blocks
    if (message.role === 'user' && Array.isArray(message.content) && message.content.some((part) => part && part.type === 'tool-result')) return true;
  }
  return false;
}

function goalFor(userText) {
  return '在 Craft 子执行器中完成用户本轮目标：' + userText +
    '。产出文件写入当前工作目录的 output/ 下（网页入口 output/index.html）。';
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
    const model = typeof body.model === 'string' ? body.model : 'craft-main-fixture';
    const id = 'chatcmpl-main-' + hits;
    const toolRound = !sawToolResult(messages);
    const userText = lastUserText(messages);
    const encode = (payload) => JSON.stringify(payload);

    const finish = () => { response.end(); };

    if (!streaming) {
      const message = toolRound
        ? {
            role: 'assistant',
            content: '',
            tool_calls: [{
              id: 'call-main-' + hits,
              type: 'function',
              function: { name: 'craft_delegate', arguments: JSON.stringify({ goal: goalFor(userText) }) },
            }],
          }
        : { role: 'assistant', content: '本轮目标已委派完成，产出已发布为版本，可在预览面板查看。' };
      const choice = { index: 0, message, finish_reason: toolRound ? 'tool_calls' : 'stop' };
      response.writeHead(200, { 'content-type': 'application/json' });
      response.end(encode({ id, object: 'chat.completion', created: Math.floor(Date.now() / 1000), model, choices: [choice] }));
      return;
    }

    response.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache', connection: 'keep-alive' });
    const send = (delta, finishReason) => {
      const choice = { index: 0, delta };
      if (finishReason !== undefined) choice.finish_reason = finishReason;
      response.write('data: ' + encode({ id, object: 'chat.completion.chunk', created: Math.floor(Date.now() / 1000), model, choices: [choice] }) + '\n\n');
    };
    send({ role: 'assistant', content: '' }, undefined);
    if (toolRound) {
      send({
        tool_calls: [{
          index: 0,
          id: 'call-main-' + hits,
          type: 'function',
          function: { name: 'craft_delegate', arguments: JSON.stringify({ goal: goalFor(userText) }) },
        }],
      }, 'tool_calls');
    } else {
      send({ content: '本轮目标已委派完成，产出已发布为版本，可在预览面板查看。' }, 'stop');
    }
    response.write('data: [DONE]\n\n');
    finish();
  });
});

server.listen(port, '127.0.0.1', () => {
  process.stdout.write('main-model fixture listening on 127.0.0.1:' + port + '\n');
});
