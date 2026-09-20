import assert from 'node:assert/strict';
import { createRequire } from 'node:module';
import test from 'node:test';
import React from 'react';
import { act } from 'react';
import { createAgentMarketplaceApi } from './agent-marketplace-api.ts';
import { TenantReleaseReview } from './TenantReleaseReview.tsx';

const { JSDOM } = createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
Object.assign(globalThis, { React, IS_REACT_ACT_ENVIRONMENT: true });
const { createRoot } = await import('react-dom/client');

const digest = 'd'.repeat(64);
const submission = {
  id: 'submission-1', tenant_id: 7, listing_id: 'listing-1', agent_version_id: 'version-1', source_agent_id: 'agent-1', author_id: 'author-1',
  semantic_version: '1.0.0', bundle_digest: digest, manifest: { display_name: 'Helpful agent', summary: 'Support', license_id: 'MIT' },
  dependency_lock: { dependencies: [{ type: 'skill', version: '2.0.0', digest: 'e'.repeat(64), license_id: 'Apache-2.0' }] },
  status: 'pending_review', created_at: '2026-09-21T00:00:00Z',
};
const release = { id: 'release-1', listing_id: 'listing-1', submission_id: 'submission-1', agent_version_id: 'version-1', source_agent_id: 'agent-1', release_number: 1, semantic_version: '1.0.0', bundle_digest: digest, manifest: submission.manifest, dependency_lock: submission.dependency_lock, published_by: 'reviewer-1', created_at: '2026-09-21T00:01:00Z' };

async function render(role: 'viewer' | 'contributor' | 'admin' | 'owner', reviewResponse?: unknown) {
  const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test/platform/market' });
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  Object.assign(globalThis, { window: dom.window, document: dom.window.document, HTMLElement: dom.window.HTMLElement, HTMLInputElement: dom.window.HTMLInputElement, HTMLTextAreaElement: dom.window.HTMLTextAreaElement, Event: dom.window.Event });
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createAgentMarketplaceApi({ request: async (input) => {
    calls.push(input);
    if (input.path.endsWith('/review-queue')) return { success: true, data: [submission] };
    if (input.path.endsWith('/review')) return reviewResponse ?? { success: true, data: { review: { id: 'review-1', submission_id: 'submission-1', reviewer_id: 'reviewer-1', reviewed_digest: digest, decision: 'rejected', reason: 'Needs a clearer safety boundary', created_at: '2026-09-21T00:01:00Z' }, release: null } };
    throw new Error(`Unexpected request ${input.method} ${input.path}`);
  } });
  const host = dom.window.document.createElement('div');
  dom.window.document.body.appendChild(host);
  const root = createRoot(host);
  await act(async () => { root.render(React.createElement(TenantReleaseReview, { api, role })); await new Promise((resolve) => setImmediate(resolve)); });
  return {
    host, calls,
    click: async (selector: string) => { const button = host.querySelector(selector) as HTMLButtonElement; assert.ok(button, `button ${selector} exists`); await act(async () => { button.click(); }); },
    cleanup: async () => { await act(async () => { root.unmount(); }); Object.assign(globalThis, { window: previousWindow, document: previousDocument }); dom.window.close(); },
  };
}

test('Tenant review controls are available only to Admin and Owner roles', async () => {
  const member = await render('contributor');
  try {
    assert.equal(member.host.querySelector('[data-review-decision]'), null);
    assert.equal(member.calls.length, 0, 'unauthorized UI does not request the review queue');
  } finally { await member.cleanup(); }

  for (const role of ['admin', 'owner'] as const) {
    const reviewer = await render(role);
    try {
      assert.ok(reviewer.host.querySelector('[data-review-decision="approved"]'));
      assert.ok(reviewer.host.querySelector('[data-review-decision="rejected"]'));
      assert.ok(reviewer.host.querySelector('[data-review-decision="changes_requested"]'));
    } finally { await reviewer.cleanup(); }
  }
});

test('reject requires a reason and reviews the exact displayed digest', async () => {
  const reviewer = await render('admin');
  try {
    assert.match(reviewer.host.textContent ?? '', new RegExp(digest));
    assert.match(reviewer.host.textContent ?? '', /MIT/);
    assert.match(reviewer.host.textContent ?? '', /Apache-2\.0/);
    await reviewer.click('[data-review-decision="rejected"]');
    const reason = reviewer.host.querySelector('[data-review-reason]') as HTMLTextAreaElement;
    assert.ok(reason);
    const setter = Object.getOwnPropertyDescriptor(reviewer.host.ownerDocument.defaultView!.HTMLTextAreaElement.prototype, 'value')?.set;
    setter?.call(reason, '   ');
    await act(async () => { reason.dispatchEvent(new reviewer.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    const submit = reviewer.host.querySelector('[data-review-submit]') as HTMLButtonElement;
    assert.ok(submit);
    assert.equal(submit.disabled, true, 'blank reason cannot be submitted');
    assert.equal(reviewer.calls.length, 1, 'only the queue request has run');
    setter?.call(reason, 'Needs a clearer safety boundary');
    await act(async () => { reason.dispatchEvent(new reviewer.host.ownerDocument.defaultView!.Event('input', { bubbles: true })); });
    assert.equal(submit.disabled, false);
    await act(async () => { submit.click(); });
    assert.equal(reviewer.calls[1]?.body && (reviewer.calls[1]?.body as { expected_digest: string }).expected_digest, digest);
    assert.equal((reviewer.calls[1]?.body as { decision: string }).decision, 'rejected');
    assert.equal((reviewer.calls[1]?.body as { reason: string }).reason, 'Needs a clearer safety boundary');
    assert.match(reviewer.host.textContent ?? '', /review-1/);
  } finally { await reviewer.cleanup(); }
});

test('approval displays the immutable Release returned by the atomic review response', async () => {
  const response = { success: true, data: { review: { id: 'review-2', submission_id: 'submission-1', reviewer_id: 'reviewer-1', reviewed_digest: digest, decision: 'approved', reason: '', created_at: '2026-09-21T00:01:00Z' }, release } };
  const reviewer = await render('owner', response);
  try {
    assert.equal(reviewer.host.querySelector('[data-review-reason]'), null, 'approval does not ask for a rejection reason');
    await reviewer.click('[data-review-decision="approved"]');
    await reviewer.click('[data-review-submit]');
    assert.equal((reviewer.calls[1]?.body as { expected_digest: string }).expected_digest, digest);
    assert.equal((reviewer.calls[1]?.body as { decision: string }).decision, 'approved');
    assert.match(reviewer.host.textContent ?? '', /release-1/);
    assert.match(reviewer.host.textContent ?? '', new RegExp(digest));
  } finally { await reviewer.cleanup(); }
});
