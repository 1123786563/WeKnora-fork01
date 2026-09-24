import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void;
};
hooks.registerHooks?.({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css')
  ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
  : nextResolve(specifier, context) });
import { renderToStaticMarkup } from '../../../../apps/web/node_modules/react-dom/server.js';
const { CraftInputDecisionPanel } = await import('./files.tsx');

test('Craft input panel distinguishes accepted opaque material from understood text', () => {
  const markup = renderToStaticMarkup(React.createElement(CraftInputDecisionPanel, {
    locale: 'en',
    inputs: [{ ref: 'opaque://1', name: 'ledger.mystery', sha256: 'a'.repeat(64), bytes: 7, citation_id: 'c1', recognition: { accepted: true, understood: false, reason: 'unrecognized_format' } }],
    onDecide: async () => {},
  }));
  assert.match(markup, /ledger\.mystery/);
  assert.match(markup, /accepted/i);
  assert.match(markup, /not understood/i);
  assert.match(markup, /continue/i);
  assert.match(markup, /cancel/i);
});
