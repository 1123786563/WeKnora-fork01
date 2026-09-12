import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test from 'node:test';
import * as React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
(globalThis as typeof globalThis & { React: typeof React }).React = React;
const { SkillSettingsPanel } = await import('./SkillSettingsPanel.tsx');
const client = {} as never;

test('skill settings keeps viewer state read-only', () => {
  const html = renderToStaticMarkup(React.createElement(SkillSettingsPanel, { client, role: 'viewer', initialSkills: [{ id: 's1', name: 'PDF', description: 'Read PDFs' }] }));
  assert.match(html, /Installed skills are managed/);
  assert.match(html, /PDF/);
  assert.doesNotMatch(html, /<button/);
});

test('skill settings routes admin to the existing server-backed catalog operations', () => {
  const html = renderToStaticMarkup(React.createElement(SkillSettingsPanel, { client, role: 'admin' }));
  assert.match(html, /Skill catalog and files/);
  assert.match(html, /Register skill source/);
  assert.match(html, /Sandbox configuration IDs/);
});
