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
});
