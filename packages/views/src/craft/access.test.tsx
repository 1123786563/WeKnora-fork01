import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from '../../../../apps/web/node_modules/react-dom/server.node.js';
import { CraftAccess } from './access.tsx';

test('Craft access projects owner controls but keeps Viewer read only', () => {
  const members = [{ user_id: 'owner', role: 'owner' as const }, { user_id: 'viewer', role: 'viewer' as const }];
  const owner = renderToStaticMarkup(<CraftAccess role="owner" members={members} onGrant={() => {}} onRevoke={() => {}} />);
  assert.match(owner, /Add member/);
  assert.match(owner, /Revoke/);
  const viewer = renderToStaticMarkup(<CraftAccess role="viewer" members={members} onGrant={() => {}} onRevoke={() => {}} />);
  assert.match(viewer, /viewer/);
  assert.doesNotMatch(viewer, /Add member|Revoke/);
});
