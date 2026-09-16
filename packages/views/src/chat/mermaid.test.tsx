import assert from 'node:assert/strict';
import test from 'node:test';

import { hydrateMermaidBlocks, MERMAID_RENDER_CONFIG, mermaidSource } from './mermaid.ts';

test('extracts Mermaid source from the controlled Markdown code block', () => {
  const block = {
    querySelector: (selector: string) => selector === 'code' ? { textContent: 'graph TD; A-->B' } : null,
  } as unknown as HTMLElement;

  assert.equal(mermaidSource(block), 'graph TD; A-->B');
});

test('requires strict Mermaid rendering and disables automatic page startup', () => {
  assert.equal(MERMAID_RENDER_CONFIG.securityLevel, 'strict');
  assert.equal(MERMAID_RENDER_CONFIG.startOnLoad, false);
});

test('hydrates only after the SVG passes through the sanitizer', async () => {
  const block = {
    dataset: {},
    querySelector: () => ({ textContent: 'graph TD; A-->B' }),
    replaceWith: (value: unknown) => { replaced = value; },
  };
  let replaced: unknown;
  const root = { querySelectorAll: () => [block] } as unknown as HTMLElement;
  const initialized: unknown[] = [];
  const rendered: string[] = [];
  const engine = {
    initialize: (config: typeof MERMAID_RENDER_CONFIG) => initialized.push(config),
    render: async (id: string, source: string) => {
      rendered.push(`${id}:${source}`);
      return { svg: '<svg onload="bad()"><path /></svg>' };
    },
  };
  const originalDocument = globalThis.document;
  Object.defineProperty(globalThis, 'document', {
    configurable: true,
    value: { createElement: () => ({
      className: '',
      setAttribute: () => undefined,
      innerHTML: '',
    }) },
  });
  try {
    await hydrateMermaidBlocks(root, engine, (svg) => {
      assert.match(svg, /onload/);
      return '<svg><path /></svg>';
    });
  } finally {
    Object.defineProperty(globalThis, 'document', { configurable: true, value: originalDocument });
  }
  assert.equal(initialized.length, 1);
  assert.deepEqual(rendered, ['wk-chat-mermaid-1:graph TD; A-->B']);
  assert.equal((replaced as { innerHTML: string }).innerHTML, '<svg><path /></svg>');
});

test('keeps the escaped source block when Mermaid rendering fails', async () => {
  const block = {
    dataset: {},
    querySelector: () => ({ textContent: 'graph TD; A-->B' }),
    replaceWith: () => { throw new Error('should keep fallback'); },
  };
  const root = { querySelectorAll: () => [block] } as unknown as HTMLElement;
  await hydrateMermaidBlocks(root, {
    initialize: () => undefined,
    render: async () => { throw new Error('invalid diagram'); },
  }, (svg) => svg);
  assert.equal(block.dataset.mermaidError, 'true');
});
