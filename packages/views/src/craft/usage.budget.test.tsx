import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';

const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftBudgetPauseNotice } = await import('./usage.tsx');

test('budget pause presents a safe action without money or credentials', () => {
  const pause = { run_id: 'run-1', reason: 'exhausted', limit: 50, used: 50 };
  const owner = renderToStaticMarkup(React.createElement(CraftBudgetPauseNotice, { pause, canExtend: true }));
  assert.match(owner, /预算已用尽/);
  assert.match(owner, /申请增加预算/);
  assert.doesNotMatch(owner, /50|credit|token|secret/i);
  const member = renderToStaticMarkup(React.createElement(CraftBudgetPauseNotice, { pause, canExtend: false }));
  assert.match(member, /联系 Task Owner 或账单管理员/);
  assert.doesNotMatch(member, /申请增加预算|50|credit|token|secret/i);
});
