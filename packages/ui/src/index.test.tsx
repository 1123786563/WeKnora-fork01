import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import * as nodeModule from 'node:module';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const styles = readFileSync(new URL('./styles.css', import.meta.url), 'utf8');
const compactStyles = styles.replace(/\s*([{}:;])\s*/g, '$1');
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });
const { Alert, Button, Dialog, Input, Menu, MenuContent, MenuItem, Select, Sheet, Status, Tabs, TabsContent, TabsList, TabsTrigger } = await import('./index.tsx');

test('Button and Input expose Vue-sized semantic states', () => {
  const markup = renderToStaticMarkup(<><Button variant="primary" size="small" loading>Save</Button><Input invalid aria-label="Name" /></>);
  assert.match(markup, /wk-button/);
  assert.match(markup, /wk-button-size-small/);
  assert.match(markup, /aria-busy="true"/);
  assert.match(markup, /wk-input/);
  assert.match(markup, /aria-invalid="true"/);
});

test('Input keeps Vue border-only focus for normal keyboard focus', () => {
  assert.match(compactStyles, /\.wk-input:focus\{[^}]*border-color:var\(--wk-color-brand\)[^}]*box-shadow:none/);
  assert.match(compactStyles, /\.wk-input:focus\{[^}]*outline:none/);
});

test('semantic token CSS exposes the Vue states consumed by shared UI', () => {
  for (const token of [
    '--wk-color-brand-hover',
    '--wk-color-brand-active',
    '--wk-color-brand-focus',
    '--wk-color-surface-hover',
    '--wk-color-surface-active',
    '--wk-color-text-placeholder',
    '--wk-color-text-disabled',
    '--wk-color-error-light',
    '--wk-color-success-light',
    '--wk-color-warning',
    '--wk-font-size-body',
    '--wk-line-height-body',
    '--wk-radius-panel',
    '--wk-shadow-panel',
  ]) assert.match(compactStyles, new RegExp(`${token.replaceAll('-', '\\-')}:`));
});

test('shared controls expose Vue hover, active, disabled, and status surfaces', () => {
  assert.match(compactStyles, /\.wk-button-primary:active:not\(:disabled\)\{[^}]*background:var\(--wk-color-brand-active/);
  assert.match(compactStyles, /\.wk-button-primary:hover:not\(:disabled\)\{[^}]*background:var\(--wk-color-brand-hover/);
  assert.match(compactStyles, /\.wk-button-danger:hover:not\(:disabled\)\{[^}]*background:var\(--wk-color-error-light/);
  assert.match(compactStyles, /\.wk-input:hover:not\(:disabled\)\{[^}]*border-color:var\(--wk-color-text-secondary/);
  assert.match(compactStyles, /\.wk-input:disabled\{[^}]*color:var\(--wk-color-text-disabled/);
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

test('Dialog gives each SSR instance a unique labelled title', () => {
  const markup = renderToStaticMarkup(<>
    <Dialog open title="First">One</Dialog>
    <Dialog open title="Second">Two</Dialog>
  </>);
  const labelledBy = [...markup.matchAll(/aria-labelledby="([^"]+)"/g)].map(match => match[1]);
  const titleIds = [...markup.matchAll(/<h2 id="([^"]+)"/g)].map(match => match[1]);

  assert.equal(labelledBy.length, 2);
  assert.equal(titleIds.length, 2);
  assert.equal(new Set(titleIds).size, 2);
  assert.deepEqual(labelledBy, titleIds);
});

test('Sheet gives its title an accessible labelled relationship', () => {
  const markup = renderToStaticMarkup(<Sheet open title="Settings">Body</Sheet>);
  const labelledBy = markup.match(/aria-labelledby="([^"]+)"/u)?.[1];
  const titleId = markup.match(/<h2 id="([^"]+)"[^>]*>Settings<\/h2>/u)?.[1];
  assert.ok(labelledBy);
  assert.equal(labelledBy, titleId);
});

test('Status renders the semantic live-region contract for each tone', () => {
  const markup = renderToStaticMarkup(<><Status>Loading</Status><Status tone="success">Saved</Status><Status tone="error">Failed</Status></>);
  assert.match(markup, /Loading.{0,200}role="status"/s);
  assert.match(markup, /Saved.{0,200}role="status"/s);
  assert.match(markup, /Failed.{0,200}role="alert"/s);
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
