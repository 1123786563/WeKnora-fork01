import assert from 'node:assert/strict';
import test from 'node:test';
import * as nodeModule from 'node:module';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
const { Alert, Button, Dialog, Input, Menu, MenuContent, MenuItem, Select, Sheet, Tabs, TabsContent, TabsList, TabsTrigger } = await import('./index.tsx');

test('Button and Input expose Vue-sized semantic states', () => {
  const markup = renderToStaticMarkup(<><Button variant="primary" size="small" loading>Save</Button><Input invalid aria-label="Name" /></>);
  assert.match(markup, /wk-button/);
  assert.match(markup, /wk-button-size-small/);
  assert.match(markup, /aria-busy="true"/);
  assert.match(markup, /wk-input/);
  assert.match(markup, /aria-invalid="true"/);
});

test('overlay and menu primitives render portal-ready accessible contracts', () => {
  const markup = renderToStaticMarkup(<>
    <Dialog open title="Confirm"><p>Body</p></Dialog>
    <Sheet open side="right" title="Settings"><p>Body</p></Sheet>
    <Menu open><MenuContent><MenuItem>Rename</MenuItem></MenuContent></Menu>
    <Select value="a" aria-label="Choice"><option value="a">A</option></Select>
    <Tabs value="one"><TabsList><TabsTrigger value="one">One</TabsTrigger></TabsList><TabsContent value="one">Content</TabsContent></Tabs>
    <Alert tone="error">Problem</Alert>
  </>);
  assert.match(markup, /role="dialog"/);
  assert.match(markup, /aria-modal="true"/);
  assert.match(markup, /data-side="right"/);
  assert.match(markup, /z-\[var\(--wk-overlay-settings-z\)\]/);
  assert.match(markup, /z-\[calc\(var\(--wk-overlay-settings-z\)\+1\)\]/);
  // Radix menu content is intentionally portalled and therefore omitted by
  // React's static SSR renderer; its DOM contract is covered by interaction.test.tsx.
  assert.match(markup, /role="combobox"/);
  assert.match(markup, /role="tablist"/);
  assert.match(markup, /role="tab" aria-selected="true"/);
  assert.match(markup, /role="alert"/);
});

test('shared overlay primitives retain Vue-derived style hooks', () => {
  const markup = renderToStaticMarkup(<>
    <Dialog open title="Confirm">Body</Dialog>
    <Sheet open side="right">Body</Sheet>
  </>);
  assert.match(markup, /class="wk-dialog/);
  assert.match(markup, /data-side="right"/);
  assert.match(markup, /shadow-\[0_20px_60px_rgba\(23,32,51,0\.2\)\]/);
});
