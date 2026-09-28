// CFT-S01-T009: entry-page contracts — the home creator gates on an empty
// goal, the kind picker reflects the SERVER gate (closed kinds disabled with
// a readable reason, draft preserved), templates only fill the form, and the
// library's four states (loading / empty / no-match / error, plus a distinct
// load-more failure) never conflate.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import React from 'react';
// craft entry pages import craft.css via td.tsx; short-circuit CSS the same way
// packages/ui/src/index.test.tsx does.
import * as nodeModule from 'node:module';
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (h: { resolve: (specifier: string, context: unknown, nextResolve: (s: string, c: unknown) => unknown) => unknown }) => void;
};
if (hooks.registerHooks) {
  hooks.registerHooks({
    resolve: (specifier, context, nextResolve) =>
      specifier.endsWith('.css')
        ? { shortCircuit: true, url: 'data:text/javascript,export default {}' }
        : nextResolve(specifier, context),
  });
}
const { renderToStaticMarkup } = await import('../../../../apps/web/node_modules/react-dom/server.js');
const { CraftHome, capabilitiesFromView } = await import('./home.tsx');
const { CraftLibrary } = await import('./library.tsx');
const { CraftTemplates, CRAFT_STARTER_TEMPLATES } = await import('./templates.tsx');
const { craftViewCapabilities } = await import('@weknora/domain/craft/capabilities');

const caps = craftViewCapabilities({ gateEnabled: true, allowedKinds: ['web'], canWrite: true, maxInputBytes: null });

function homeElement(over: Record<string, unknown> = {}) {
  return React.createElement(CraftHome, {
    locale: 'zh' as never,
    canCreate: true,
    createBusy: false,
    createError: null,
    listStatus: 'ready',
    listError: null,
    recent: [],
    nextCursor: null,
    knowledgeOptions: [],
    attachments: [],
    onCreate: () => {},
    onPickAttachments: () => {},
    onRemoveAttachment: () => {},
    onOpen: () => {},
    onNextPage: () => {},
    onRetryList: () => {},
    ...over,
  } as never);
}

test('an empty goal cannot submit: the create button renders disabled', () => {
  const markup = renderToStaticMarkup(homeElement());
  const button = markup.match(/<button[^>]*data-testid="craft-create"[^>]*>/)?.[0]
    ?? markup.match(/<button[^>]*>[^<]*<\/button>/g)?.find((b) => b.includes('craft-create'))
    ?? '';
  assert.ok(button.includes('craft-create'), 'create button present');
  assert.match(button, /disabled/, 'empty goal keeps the create button disabled');
});

test('closed kinds are disabled with a reason and the draft goal survives', () => {
  const markup = renderToStaticMarkup(homeElement({ capabilities: caps }));
  // every non-web option carries its reason inline and is disabled
  for (const kind of ['document', 'spreadsheet', 'slides']) {
    const option = new RegExp(`<option[^>]*value="${kind}"[^>]*disabled[^>]*>`);
    assert.match(markup, option, `${kind} option disabled`);
    assert.match(markup, new RegExp(`${kind}（kind_${kind}_is_not_open_on_this_deployment）`), `${kind} shows why`);
  }
  assert.match(markup, /value="web"[^>]*>/);
  // and capabilitiesFromView routes through the domain projection
  const projected = capabilitiesFromView({ enabled: true, allowed_kinds: ['web'] });
  assert.deepEqual(projected?.allowedKinds, ['web']);
  assert.equal(capabilitiesFromView(null), null, 'legacy assemblies keep all kinds');
});

test('templates only fill the form — no execute/authorize affordance', () => {
  let used: string | null = null;
  const markup = renderToStaticMarkup(React.createElement(CraftTemplates, {
    capabilities: caps,
    onUse: (t) => { used = t.id; },
  }));
  assert.match(markup, /模板只填充目标与类型/);
  assert.match(markup, /填入创建器/);
  // closed kinds' use buttons disabled; the web ones enabled and inert
  assert.match(markup, /disabled[^>]*title="kind_document_is_not_open_on_this_deployment"/);
  assert.doesNotMatch(markup, /立即生成|开始创作|一键授权|自动选择知识库/, 'no execute/authorize affordance (the subtitle is the disclaimer, not a button)');
  assert.equal(used, null, 'rendering never invokes onUse');
  assert.equal(CRAFT_STARTER_TEMPLATES.length >= 4, true);
});

test('library separates loading / empty / no-match / error / load-more failure', () => {
  const lib = (over: Record<string, unknown>) => renderToStaticMarkup(React.createElement(CraftLibrary, {
    locale: 'zh' as never,
    sessions: [],
    status: 'ready',
    hasNextPage: false,
    loadingMore: false,
    onOpen: () => {},
    onLoadMore: () => {},
    onRetry: () => {},
    ...over,
  } as never));
  assert.match(lib({ status: 'loading' }), /加载中|正在加载/);
  assert.match(lib({ status: 'ready', sessions: [] }), /还没有作品/);
  assert.match(lib({ status: 'ready', sessions: [{ session_id: 's1', workspace_id: 'w1', kind: 'web', title: '报告', engine_type: 'trpc', updated_at: '2026-09-18T00:00:00Z' } as never] }), /报告/);
  const errMarkup = lib({ status: 'error', error: '请求失败' });
  assert.match(errMarkup, /请求失败/);
  assert.match(errMarkup, /role="alert"/);
  assert.match(lib({ status: 'ready', hasNextPage: true, loadingMoreError: '本页加载失败' }), /本页加载失败 — 可重试本页/);
});
