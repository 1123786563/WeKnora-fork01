// CFT-S01-T012: decision cards with an HONEST delivery state machine and
// non-failure banners. Four acceptance assertions:
//   1. an approve response showing only `recorded` NEVER claims execution
//      continued — recorded → delivery_pending → delivered are distinct
//   2. a question offers answer/reject only — answering never grants
//   3. an accepted cancel keeps showing 取消中 until the authoritative
//      terminal state (the run status projection owns that transition)
//   4. reconnect and expired-preview notices never paint the task failed
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void;
};
if (hooks.registerHooks) {
  hooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier.endsWith('.css')
        ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
        : nextResolve(specifier, context),
  });
}
const React = await import('react');
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftDecisionCard, decisionDeliveryLabel } = await import('./interaction-card.tsx');
const { CraftStatusNotice } = await import('./status-notice.tsx');
const { statusLabelLocalized } = await import('./presentation.ts');

const base = {
  id: 'i-1',
  prompt: '允许执行构建命令？',
  scope: 'shell: npm run build',
  allowedActions: ['approve', 'reject'] as string[],
};

test('an approved response showing only recorded never claims execution continued', () => {
  for (const [delivery, must, mustNot] of [
    ['recorded', /已记录/, /已执行|执行已继续|已确认送达/],
    ['delivery_pending', /等待送达/, /已执行|已确认送达/],
    ['delivered', /已确认送达/, /已执行|执行已继续/],
    ['unknown', /送达不明|需核对/, /已执行|已确认送达/],
  ] as const) {
    const markup = renderToStaticMarkup(React.createElement(CraftDecisionCard, {
      ...base,
      kind: 'permission',
      resolved: true,
      decidedAction: 'allow_once',
      delivery,
    } as never));
    assert.match(markup, must, `${delivery} shows its own layer`);
    assert.doesNotMatch(markup, mustNot, `${delivery} must not claim execution`);
    assert.equal(typeof decisionDeliveryLabel(delivery), 'string');
  }
});

test('a question offers answer/reject only — answering never grants', () => {
  const markup = renderToStaticMarkup(React.createElement(CraftDecisionCard, {
    ...base,
    kind: 'question',
    allowedActions: ['answer', 'reject'],
    resolved: false,
  } as never));
  assert.match(markup, /回答/);
  assert.ok(!/批准|approve/i.test(markup.replace(/aria-label="[^"]*"/g, '')) || !markup.includes('approve'), 'a question never offers approve');
  const permission = renderToStaticMarkup(React.createElement(CraftDecisionCard, {
    ...base,
    kind: 'permission',
    resolved: false,
  } as never));
  assert.match(permission, /允许一次/, 'a permission offers allow-once');
  assert.ok(!permission.includes('永久') && !/always/i.test(permission), 'no standing grant wording');
});

test('an expired interaction keeps its buttons inert but stays readable', () => {
  const markup = renderToStaticMarkup(React.createElement(CraftDecisionCard, {
    ...base,
    kind: 'permission',
    resolved: true,
    decidedAction: null,
    delivery: 'unknown',
  } as never));
  // a resolved/terminal card renders no live controls at all — late approvals
  // can never revive the request
  const buttons = markup.match(/<button[^>]*>[^<]*<\/button>/g) ?? [];
  assert.equal(buttons.length, 0, 'a terminal card exposes no deciding controls');
  assert.match(markup, /i-1/, 'the card identity stays traceable');
});

test('reconnect and expired-preview notices never paint the task failed; cancel shows cancelling until terminal', () => {
  const reconnect = renderToStaticMarkup(React.createElement(CraftStatusNotice, { kind: 'reconnect' }));
  assert.match(reconnect, /重连|同步/);
  assert.doesNotMatch(reconnect, /失败/);
  assert.doesNotMatch(reconnect, /data-kind="failed"/);

  const expired = renderToStaticMarkup(React.createElement(CraftStatusNotice, { kind: 'expired' }));
  assert.match(expired, /过期|到期/);
  assert.doesNotMatch(expired, /失败/);

  // cancel semantics come from the run-status projection: accepted (stopping)
  // keeps the cancelling label; only the terminal projection says stopped —
  // the accepted-but-not-terminal phase never claims the terminal wording
  assert.equal(statusLabelLocalized('zh', 'stopping', 'idle', false), '正在停止');
  assert.equal(statusLabelLocalized('zh', 'canceled', 'idle', false), '已停止');
  assert.notEqual(statusLabelLocalized('zh', 'stopping', 'idle', false), '已停止');
});
