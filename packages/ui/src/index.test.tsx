import assert from 'node:assert/strict';
import test from 'node:test';
import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { Alert, Button, Dialog, Input, Menu, Select, Sheet, Tabs } from './index.tsx';

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
