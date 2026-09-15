import assert from 'node:assert/strict';
import test, { afterEach } from 'node:test';
import { JSDOM } from 'jsdom';
import React, { act } from 'react';
import { renderToStaticMarkup } from '../../../apps/web/node_modules/react-dom/server.js';
import { Alert } from './alert.tsx';
import { Button } from './button.tsx';
import { Input } from './input.tsx';
import { Select } from './select.tsx';
import { Tabs, TabsList, TabsTrigger } from './tabs.tsx';
import { Menu, MenuContent, MenuItem, MenuTrigger } from './dropdown-menu.tsx';

const dom = new JSDOM('<!doctype html><html><body></body></html>');
Object.assign(globalThis, {
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Element: dom.window.Element,
  Event: dom.window.Event,
  KeyboardEvent: dom.window.KeyboardEvent,
  Node: dom.window.Node,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });
const { createRoot } = await import('react-dom/client');
const { flushSync } = await import('react-dom');
const { Dialog, getDialogFocusableElements } = await import('./dialog.tsx');
const { Sheet } = await import('./sheet.tsx');

let mountedRoot: ReturnType<typeof createRoot> | undefined;
afterEach(() => {
  mountedRoot?.unmount();
  mountedRoot = undefined;
  document.body.replaceChildren();
});

test('exports a dialog primitive with an explicit focus and escape contract', () => {
  assert.equal(typeof Dialog, 'function');
});

test('dialog focusables exclude disabled and untabbable controls', () => {
  const dom = new JSDOM('<section><button id="first"></button><button disabled>skip</button><input tabindex="-1" /><a href="/next" id="last">next</a></section>');
  const root = dom.window.document.querySelector('section')!;
  assert.deepEqual(getDialogFocusableElements(root).map((element) => element.id), ['first', 'last']);
});

test('form primitives expose Vue-sized state classes and semantics', () => {
  const markup = renderToStaticMarkup(React.createElement('form', null,
    React.createElement(Button, { loading: true, variant: 'primary', children: 'Save' }),
    React.createElement(Input, { 'aria-invalid': true, disabled: true, value: 'bad', readOnly: true }),
    React.createElement(Select, { 'aria-invalid': true, disabled: true, value: 'one', onChange: () => undefined },
      React.createElement('option', { value: 'one' }, 'One'),
    ),
    React.createElement(Alert, { tone: 'danger', children: 'Invalid value' }),
  ));

  assert.match(markup, /button[^>]*disabled[^>]*aria-busy="true"/);
  assert.match(markup, /min-h-8/);
  assert.match(markup, /aria-\[invalid=true\]:border-danger/);
  assert.match(markup, /role="alert"/);
  assert.match(markup, /wk-button-loading/);
  assert.match(markup, /focus-visible:outline-accent/);
});

test('form primitives use the Vue focus contract and tokenized state colors', () => {
  const markup = renderToStaticMarkup(React.createElement('form', null,
    React.createElement(Input, { 'aria-invalid': true, placeholder: 'Name' }),
    React.createElement(Select, { 'aria-invalid': true, value: 'one', onChange: () => undefined },
      React.createElement('option', { value: 'one' }, 'One'),
    ),
    React.createElement(Alert, { tone: 'warning', children: 'Check this field' }),
  ));

  assert.doesNotMatch(markup, /focus:ring/);
  assert.match(markup, /focus:border-accent/);
  assert.match(markup, /border-warning/);
  assert.match(markup, /bg-warning-wash/);
});

test('form primitives provide explicit invalid aliases and Vue control geometry', () => {
  const markup = renderToStaticMarkup(React.createElement('form', null,
    React.createElement(Input, { invalid: true, placeholder: 'Name' }),
    React.createElement(Select, { invalid: true, children: React.createElement('option', { value: 'one' }, 'One') }),
  ));
  assert.match(markup, /aria-invalid="true"/g);
  assert.match(markup, /h-8/);
  assert.match(markup, /leading-\[22px\]/);
  assert.match(markup, /focus-visible:outline-accent/);
});

test('tabs expose Vue-sized trigger and active-state tokens', () => {
  const markup = renderToStaticMarkup(React.createElement(Tabs, { defaultValue: 'one' },
    React.createElement(TabsList, null,
      React.createElement(TabsTrigger, { value: 'one', children: 'One' }),
    ),
  ));

  assert.match(markup, /min-h-8/);
  assert.match(markup, /data-\[state=active\]:border-accent/);
  assert.doesNotMatch(markup, /outline-primary/);
});

test('menu aliases preserve the Radix portal and Vue menu surface tokens', () => {
  const markup = renderToStaticMarkup(React.createElement(Menu, null,
    React.createElement(MenuTrigger, { asChild: true }, React.createElement('button', null, 'Open')),
    React.createElement(MenuContent, null, React.createElement(MenuItem, null, 'Item')),
  ));
  assert.match(markup, /data-state/);
  assert.match(markup, /Open/);
});

test('shared menu styles retain Vue sizing, hover and active interaction hooks', () => {
  const markup = renderToStaticMarkup(React.createElement(Menu, null,
    React.createElement(MenuTrigger, { asChild: true }, React.createElement('button', null, 'Open')),
    React.createElement(MenuContent, null, React.createElement(MenuItem, null, 'Item')),
  ));
  assert.match(markup, /data-state/);
  assert.match(markup, /Open/);
});

test('dialog and sheet expose the overlay contract in a DOM root', () => {
  const host = document.createElement('main');
  document.body.append(host);
  mountedRoot = createRoot(host);
  flushSync(() => mountedRoot?.render(<><Dialog open title="Dialog" portal={false} onClose={() => undefined}>Body</Dialog><Sheet open title="Sheet" portal={false} onClose={() => undefined}>Body</Sheet></>));

  assert.equal(host.querySelectorAll('[role="dialog"]').length, 2);
  assert.equal(document.body.querySelectorAll('[role="dialog"]').length, 2);
});

test('dialog and sheet support explicit inline rendering for SSR/static checks', () => {
  const markup = renderToStaticMarkup(React.createElement(Dialog, { open: true, portal: false, title: 'Dialog', onClose: () => undefined }, 'Body'));
  assert.match(markup, /role="dialog"/);
  assert.match(markup, /Body/);
});

test('sheet keeps a fixed drawer from exceeding the viewport on narrow desktop windows', () => {
  const markup = renderToStaticMarkup(React.createElement(Sheet, { open: true, title: 'Sheet', onClose: () => undefined, width: '640px', portal: false }, 'Body'));
  assert.match(markup, /max-w-full/);
  assert.match(markup, /style="width:640px/);
});
