import assert from 'node:assert/strict';
import test from 'node:test';
import { JSDOM } from 'jsdom';
import React from 'react';
import { renderToStaticMarkup } from '../../../apps/web/node_modules/react-dom/server.js';
import { Alert } from './alert.tsx';
import { Button } from './button.tsx';
import { Dialog, getDialogFocusableElements } from './dialog.tsx';
import { Input } from './input.tsx';
import { Select } from './select.tsx';
import { Tabs, TabsList, TabsTrigger } from './tabs.tsx';
import { Menu, MenuContent, MenuItem, MenuTrigger } from './dropdown-menu.tsx';

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
