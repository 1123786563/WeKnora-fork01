import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';
import { createAgentMarketplaceApi } from './agent-marketplace-api.ts';
import { AgentVersionActions } from './AgentVersionActions.tsx';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
const { createRoot } = await import('react-dom/client');

const version = { id: 'version-1', agent_id: 'agent-1', version_number: 1, source_sha256: 'a'.repeat(64), frozen_by: 'author-1', created_at: '2026-09-21T00:00:00Z' };
const submission = { id: 'submission-1', tenant_id: 7, listing_id: 'listing-1', agent_version_id: 'version-1', source_agent_id: 'agent-1', author_id: 'author-1', semantic_version: '1.0.0', bundle_digest: 'b'.repeat(64), manifest: { display_name: 'Helpful agent', license_id: 'MIT' }, dependency_lock: { dependencies: [] }, payload: { system_prompt: 'fixed prompt', allowed_tools: [], skills: [], subagents: [], starter_prompts: [] }, status: 'pending_review', created_at: '2026-09-21T00:00:00Z' };

test('freezes an Agent Version, selects it, and submits that fixed version through the shared API client', async () => {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/' });
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, HTMLTextAreaElement: dom.window.HTMLTextAreaElement, Event: dom.window.Event });
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createAgentMarketplaceApi({ request: async (input) => {
    calls.push(input);
    if (input.path === '/api/v1/agents/agent-1/versions') return { success: true, data: version };
    if (input.path === '/api/v1/marketplace/tenant/release-submissions') return { success: true, data: submission };
    throw new Error(`Unexpected request ${input.method} ${input.path}`);
  } });
  const host = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(host);
  const root = createRoot(host);
  try {
    await act(async () => { root.render(React.createElement(AgentVersionActions, { api, agentId: 'agent-1', agentName: 'Helpful agent' })); });
    const freeze = host.querySelector('[data-author-freeze]') as HTMLButtonElement;
    assert.ok(freeze);
    await act(async () => { freeze.click(); });
    assert.match(host.textContent ?? '', /version-1/);
    assert.match(host.textContent ?? '', /a{64}/);

    const values: Record<string, string> = {
      semantic_version: '1.0.0',
      display_name: 'Helpful agent',
      summary: 'Helps answer support questions',
      supported_languages: 'en, zh',
      use_cases: 'support, knowledge questions',
      minimum_weknora_capability: '1',
      license_id: 'MIT',
    };
    for (const [name, value] of Object.entries(values)) {
      const input = host.querySelector(`[name="${name}"]`) as HTMLInputElement | HTMLTextAreaElement;
      assert.ok(input, `field ${name} exists`);
      const proto = input instanceof dom.window.HTMLTextAreaElement ? dom.window.HTMLTextAreaElement.prototype : dom.window.HTMLInputElement.prototype;
      const setter = Object.getOwnPropertyDescriptor(proto, 'value')?.set;
      setter?.call(input, value);
      await act(async () => { input.dispatchEvent(new dom.window.Event('input', { bubbles: true })); });
      await act(async () => { input.dispatchEvent(new dom.window.Event('change', { bubbles: true })); });
    }
    const submit = host.querySelector('[data-author-submit]') as HTMLButtonElement;
    assert.ok(submit);
    await act(async () => { submit.click(); });

    assert.equal(calls[0]?.path, '/api/v1/agents/agent-1/versions');
    assert.equal(calls[1]?.path, '/api/v1/marketplace/tenant/release-submissions');
    assert.deepEqual(calls[1]?.body, {
      agent_version_id: 'version-1',
      metadata: {
        semantic_version: '1.0.0',
        display_name: 'Helpful agent',
        summary: 'Helps answer support questions',
        supported_languages: ['en', 'zh'],
        use_cases: ['support', 'knowledge questions'],
        minimum_weknora_capability: '1',
        license_id: 'MIT',
      },
    });
    assert.match(host.textContent ?? '', /submission-1/);
  } finally {
    await act(async () => { root.unmount(); });
    Object.assign(globalThis, { window: previousWindow, document: previousDocument });
    dom.window.close();
  }
});
