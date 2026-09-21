import assert from 'node:assert/strict';
import test from 'node:test';

import { CHAT_COPY } from './chat-copy.ts';
import {
  browserActionLabel,
  browserPageAddress,
  browserToolContent,
  browserToolIncomplete,
  browserToolSummary,
  browserToolTitle,
  type BrowserToolEvent,
} from './browser-tool-display.ts';

// Upstream browserToolDisplay.test.ts contract, ported to the flattened
// ChatCopyTable keys.

test('browserPageAddress strips credentials, query and rejects non-http schemes', () => {
  assert.equal(browserPageAddress('https://user:pass@example.com/path?token=x#frag'), 'https://example.com/path');
  assert.equal(browserPageAddress('http://example.com'), 'http://example.com/');
  assert.equal(browserPageAddress('javascript:alert(1)'), '');
  assert.equal(browserPageAddress('not a url'), '');
});

test('browserActionLabel maps every documented method to its copy key', () => {
  assert.equal(browserActionLabel(CHAT_COPY, 'navigate'), CHAT_COPY.browserToolOpenPage);
  assert.equal(browserActionLabel(CHAT_COPY, 'request_help'), CHAT_COPY.browserToolNeedHelp);
  assert.equal(browserActionLabel(CHAT_COPY, 'unknown-method'), CHAT_COPY.browserToolBrowserAction);
});

test('browserToolIncomplete matches navigation timeouts and explicit failures only', () => {
  const timeout: BrowserToolEvent = { arguments: { method: 'navigate' }, output: { reached: 'timeout' } };
  const loaded: BrowserToolEvent = { arguments: { method: 'navigate' }, output: { reached: 'domcontentloaded' } };
  assert.equal(browserToolIncomplete(timeout), true);
  assert.equal(browserToolIncomplete(loaded), false);
  assert.equal(browserToolIncomplete({ arguments: { method: 'observe' }, output: { text: 'timeout' } }), false);
  assert.equal(browserToolIncomplete({ success: false }), true);
});

test('browserToolTitle carries method label, target and failure marker', () => {
  const base: BrowserToolEvent = { arguments: { method: 'navigate', url: 'https://example.com/page' }, pending: true };
  assert.equal(browserToolTitle(CHAT_COPY, base),
    `${CHAT_COPY.browserToolLocal} · ${CHAT_COPY.browserToolOpenPage} · example.com…`);
  const failed: BrowserToolEvent = {
    arguments: { method: 'navigate', url: 'https://example.com' },
    output: { reached: 'timeout' },
  };
  assert.ok(browserToolTitle(CHAT_COPY, failed).includes(CHAT_COPY.browserToolActionFailed));
  const press: BrowserToolEvent = { arguments: { method: 'press', key: 'Enter' } };
  assert.ok(browserToolTitle(CHAT_COPY, press).includes('Enter'));
});

test('browserToolSummary classifies pending, completion and failure families', () => {
  assert.equal(browserToolSummary(CHAT_COPY, { pending: true }), CHAT_COPY.browserToolActionPending);
  assert.equal(browserToolSummary(CHAT_COPY, { success: true }), CHAT_COPY.browserToolActionCompleted);
  assert.equal(browserToolSummary(CHAT_COPY, {}), CHAT_COPY.browserToolActionRecorded);
  assert.equal(
    browserToolSummary(CHAT_COPY, { success: false, error: 'unfinished command in flight' }),
    CHAT_COPY.browserToolCommandBusy,
  );
  assert.equal(
    browserToolSummary(CHAT_COPY, { success: false, error: 'Parameter validation failed: url' }),
    CHAT_COPY.browserToolInvalidArguments,
  );
  assert.equal(
    browserToolSummary(CHAT_COPY, { success: false, error: 'task is paused' }),
    CHAT_COPY.browserToolCommandInterrupted,
  );
  assert.equal(
    browserToolSummary(CHAT_COPY, { success: false, error: 'browser is offline; connect BrowserSkill' }),
    CHAT_COPY.browserToolReconnectHint,
  );
  assert.equal(
    browserToolSummary(CHAT_COPY, { success: false, error: 'something else' }),
    CHAT_COPY.browserToolActionFailedHint,
  );
});

test('browserToolContent renders console and network diagnostics', () => {
  const console = browserToolContent({
    arguments: { method: 'console' },
    tool_data: {
      entries: [
        { level: 'error', text: 'boom', url: 'https://example.com/app.js', line: 3, column: 9,
          stack_trace: [{ function_name: 'main', url: 'https://example.com/app.js', line: 42 }] },
      ],
    },
  });
  assert.ok(console.text.includes('[error] boom'));
  assert.ok(console.text.includes('https://example.com/app.js:3:9'));
  assert.ok(console.text.includes('main https://example.com/app.js:42'));
  assert.equal(console.empty, false);

  const network = browserToolContent({
    arguments: { method: 'network' },
    tool_data: { entries: [{ method: 'GET', status: 200, url: 'https://example.com/a?tok=1' }] },
  });
  assert.ok(network.text.includes('GET 200 https://example.com/a'));

  const empty = browserToolContent({ arguments: { method: 'network' }, tool_data: { entries: [] } });
  assert.equal(empty.empty, true);
});

test('browserToolContent bounds screenshots into the data channel', () => {
  // 1x1 transparent PNG.
  const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aTioAAAAASUVORK5CYII=';
  const shot = browserToolContent({ arguments: { method: 'screenshot' }, output: { image_base64: png, format: 'png' } });
  assert.equal(shot.image, `data:image/png;base64,${png}`);
  assert.equal(shot.text, '');

  const bad = browserToolContent({ arguments: { method: 'screenshot' }, output: { image_base64: '####', format: 'png' } });
  assert.equal(bad.image, '');
});

test('browserToolContent surfaces help prompts and page identity', () => {
  const help = browserToolContent({
    arguments: { method: 'request_help', prompt: '请扫码登录' },
    output: { title: '登录', final_url: 'https://example.com/login?session=x' },
  });
  assert.equal(help.prompt, '请扫码登录');
  assert.equal(help.title, '登录');
  assert.equal(help.address, 'https://example.com/login');
});

test('browserToolContent truncates oversized page text', () => {
  const long = browserToolContent({
    arguments: { method: 'observe' },
    output: { text: 'x'.repeat(13000) },
  });
  assert.equal(long.text.length, 12000);
  assert.equal(long.truncated, true);
});
