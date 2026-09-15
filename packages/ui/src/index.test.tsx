import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { Alert, Button, Dialog, Input, Menu, Select, Sheet, Tabs } from './index.tsx';

const styles = readFileSync(new URL('./styles.css', import.meta.url), 'utf8');
const compactStyles = styles.replace(/\s*([{}:;])\s*/g, '$1');

test('Button and Input expose Vue-sized semantic states', () => {
  const markup = renderToStaticMarkup(<><Button variant="primary" size="small" loading>Save</Button><Input invalid aria-label="Name" /></>);
  assert.match(markup, /wk-button/);
  assert.match(markup, /wk-button-size-small/);
  assert.match(markup, /aria-busy="true"/);
  assert.match(markup, /wk-input/);
  assert.match(markup, /aria-invalid="true"/);
});

test('Input keeps Vue focus rings for normal and invalid keyboard focus', () => {
  assert.match(compactStyles, /\.wk-input:focus\{[^}]*border-color:var\(--wk-color-brand,#07c05f\)[^}]*box-shadow:0 0 0 2px var\(--wk-color-brand-focus/);
  assert.match(compactStyles, /\.wk-input\[aria-invalid=true\]:focus\{[^}]*border-color:var\(--wk-color-error,#e34d59\)[^}]*box-shadow:0 0 0 2px var\(--wk-color-error-focus/);
  assert.match(compactStyles, /\.wk-input:focus\{[^}]*outline:none/);
});

test('overlay and menu primitives render portal-ready accessible contracts', () => {
  const markup = renderToStaticMarkup(<>
    <Dialog open title="Confirm"><p>Body</p></Dialog>
    <Sheet open side="right" title="Settings"><p>Body</p></Sheet>
    <Menu open label="More"><Menu.Item>Rename</Menu.Item></Menu>
    <Select value="a" aria-label="Choice"><Select.Option value="a">A</Select.Option></Select>
    <Tabs value="one"><Tabs.List><Tabs.Trigger value="one">One</Tabs.Trigger></Tabs.List><Tabs.Content value="one">Content</Tabs.Content></Tabs>
    <Alert tone="error">Problem</Alert>
  </>);
  assert.match(markup, /role="dialog"/);
  assert.match(markup, /aria-modal="true"/);
  assert.match(markup, /data-side="right"/);
  assert.match(markup, /role="menu"/);
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
  const titleIds = [...markup.matchAll(/<h2 id="([^"]+)">/g)].map(match => match[1]);

  assert.equal(labelledBy.length, 2);
  assert.equal(titleIds.length, 2);
  assert.equal(new Set(titleIds).size, 2);
  assert.deepEqual(labelledBy, titleIds);
});
