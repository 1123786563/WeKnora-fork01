import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';
import test, { afterEach, beforeEach } from 'node:test';
import * as React from 'react';
import { act } from 'react';
import type { Root } from 'react-dom/client';
import { renderToStaticMarkup } from 'react-dom/server';
import type { WeKnoraClient } from '@weknora/api-client';

const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => unknown }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { JSDOM } = nodeModule.createRequire(import.meta.url)('jsdom') as { JSDOM: new (html: string, options: { url: string }) => { window: Window & typeof globalThis } };
const dom = new JSDOM('<!doctype html><html><body></body></html>', { url: 'https://weknora.test' });
Object.assign(globalThis, {
  React,
  window: dom.window,
  document: dom.window.document,
  HTMLElement: dom.window.HTMLElement,
  Event: dom.window.Event,
  IS_REACT_ACT_ENVIRONMENT: true,
});
Object.defineProperty(globalThis, 'navigator', { configurable: true, value: dom.window.navigator });

const { createRoot } = await import('react-dom/client');
const { formatMessage } = await import('@weknora/i18n');
const {
  SandboxSettingsPanel,
  DEFAULT_DOCKER_IMAGE,
  canDeepCheckOnStep,
  canJumpToStep,
  checkScopeKey,
  collectNetworkPolicy,
  collectSandboxConfig,
  denyOutRowCoversAllIPv4,
  domainAllowNeedsDenyAll,
  initialEditorForm,
  missingRequiredFields,
  pendingCheckItems,
  primaryTextKey,
  reportedChecks,
  sandboxT,
  templateStatusKey,
  wizardStepKeys,
} = await import('/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient/apps/web/src/settings/SandboxSettingsPanel.tsx');

const t = sandboxT('zh-CN');

// ---- fixtures ---------------------------------------------------------------

const e2bRecord = {
  id: 'sandbox-1',
  name: 'E2B production',
  description: 'Remote execution',
  sandbox_type: 'e2b',
  // No api_key and no template yet: the card surfaces both warnings.
  config: { sandbox_type: 'e2b', e2b: { api_url: 'https://api.e2b.app' } },
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
} as never;

const cubeRecord = {
  id: 'sandbox-2',
  name: 'Cube cluster',
  description: '',
  sandbox_type: 'cube',
  config: { sandbox_type: 'cube', cube: { api_url: 'http://cube.internal:33000', proxy_url: 'http://cube.internal:80', sandbox_domain: 'cube.app', template_id: 'tpl-base' } },
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
} as never;

const cubeNoTemplateRecord = {
  id: 'sandbox-4',
  name: 'Cube without template',
  sandbox_type: 'cube',
  config: { sandbox_type: 'cube', cube: { api_url: 'http://cube.internal:33000', proxy_url: 'http://cube.internal:80', sandbox_domain: 'cube.app' } },
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
} as never;

const legacyRecord = {
  id: 'sandbox-3',
  name: 'Legacy local sandbox',
  sandbox_type: 'local',
  config: { sandbox_type: 'local' },
  created_at: '2030-01-01T00:00:00Z',
  updated_at: '2030-01-01T00:00:00Z',
} as never;

// ---- client harness ---------------------------------------------------------

interface RoutedRequest { method: string; path: string; body?: unknown }

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((next) => { resolve = next; });
  return { promise, resolve };
}

type RequestHandler = (request: RoutedRequest) => unknown;

function makeClient(routes: RequestHandler, listItems: unknown[] = []): { client: WeKnoraClient; requests: RoutedRequest[] } {
  const requests: RoutedRequest[] = [];
  const client = {
    request: async (input: RoutedRequest) => {
      requests.push(input);
      return routes(input);
    },
    // Mirrors the real client: one typed GET per session, envelope parsed to data.
    sessions: {
      get: async (sessionId: string) => {
        const envelope = await routes({ method: 'GET', path: `/api/v1/sessions/${encodeURIComponent(sessionId)}` }) as { success?: boolean; data?: unknown } | null;
        if (!envelope || envelope.success !== true) throw new Error('session request failed');
        return envelope.data;
      },
    },
    sandboxConfigurations: {
      list: async () => ({ items: listItems, workspaceScriptsDisabled: false }),
      create: async () => ({}),
      update: async () => ({}),
      remove: async () => undefined,
      inventory: async () => ({ sandboxCount: 0, sessionIds: [], agentNames: [], unverifiable: false }),
      setWorkspacePolicy: async () => ({ workspaceScriptsDisabled: false }),
    },
  };
  return { client: client as unknown as WeKnoraClient, requests };
}

const okCatalog = (templates: unknown[], standardTemplateId = '', provisioned = false) => ({
  success: true, data: { templates, standard_template_id: standardTemplateId, provisioned },
});
const okCheck = (ok: boolean, checks: unknown[]) => ({ success: true, data: { ok, provider: 'test', checks } });

let mountedRoot: Root | undefined;
let confirmCalls: string[] = [];

afterEach(async () => {
  if (mountedRoot) await act(async () => mountedRoot?.unmount());
  mountedRoot = undefined;
  document.body.replaceChildren();
  window.localStorage.clear();
  confirmCalls = [];
});

async function mount(client: WeKnoraClient, props: { role?: 'viewer' | 'admin' | 'owner'; initialData?: { items: unknown[]; workspaceScriptsDisabled: boolean } } = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<SandboxSettingsPanel
      client={client}
      role={props.role ?? 'admin'}
      {...(props.initialData === undefined ? {} : { initialData: props.initialData as never })}
    />);
  });
  return container;
}

async function openCreateEditor(container: HTMLElement) {
  const addButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.addConfig'));
  assert.ok(addButton, 'the add button should render for admins');
  await act(async () => addButton!.click());
  const editor = container.querySelector<HTMLElement>('[data-testid="sandbox-editor"]');
  assert.ok(editor, 'the editor drawer should open');
  return editor;
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLInputElement.prototype, 'value')?.set;
  setValue?.call(input, value);
  input.dispatchEvent(new dom.window.Event('input', { bubbles: true }));
}

function setSelectValue(select: HTMLSelectElement, value: string) {
  const setValue = Object.getOwnPropertyDescriptor(dom.window.HTMLSelectElement.prototype, 'value')?.set;
  setValue?.call(select, value);
  select.dispatchEvent(new dom.window.Event('change', { bubbles: true }));
}

function submitEditor(editor: HTMLElement) {
  return act(async () => {
    editor.querySelector('form')!.dispatchEvent(new dom.window.Event('submit', { bubbles: true, cancelable: true }));
  });
}

function fillCubeConnection(editor: HTMLElement) {
  const inputs = Array.from(editor.querySelectorAll('input')) as HTMLInputElement[];
  const nameInput = inputs.find((input) => input.placeholder === t('settings.sandbox.configNamePlaceholder'))!;
  setInputValue(nameInput, 'New cube config');
  const byPlaceholder = (placeholder: string) => inputs.find((input) => input.placeholder === placeholder)!;
  setInputValue(byPlaceholder('http://cube.example.com:33000'), 'http://cube.example.com:33000');
  setInputValue(byPlaceholder('http://cube.example.com:80'), 'http://cube.example.com:80');
  setInputValue(byPlaceholder('cube.app'), 'cube.app');
}

// ---- inventory session titles (SandboxSettings.vue:140-206, 312-344, 511-528) -

async function openInventory(container: HTMLElement): Promise<void> {
  const viewButton = Array.from(container.querySelectorAll('button'))
    .find((button) => button.textContent === t('settings.sandbox.viewSandboxes'));
  assert.ok(viewButton, 'cube cards offer the inventory entry');
  await act(async () => viewButton!.click());
}
test('inventory stays hidden while session titles resolve, then renders them (SandboxSettings.vue:515-527)', async () => {
  const pending = deferred<{ id: string; title: string; is_pinned: boolean }>();
  const { client } = makeClient(() => ({}), [cubeRecord]);
  (client.sandboxConfigurations as { inventory: unknown }).inventory = async () => ({
    sandboxCount: 1, sessionIds: ['session-a'], agentNames: [],
  });
  (client.sessions as { get: (id: string) => Promise<unknown> }).get = () => pending.promise;
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  console.log('probe-a mounted');
  await openInventory(container);
  console.log('probe-b clicked');

  // Vue keeps the drawer inside t-loading until the titles land (515-527),
  // so the list must not render raw ids first.
  assert.equal(container.querySelector('.wk-sandbox-inventory'), null, 'no inventory list before titles resolve');
  console.log('probe-c asserted hidden');

  await act(async () => pending.resolve({ id: 'session-a', title: '巡检机器人', is_pinned: false }));
  const text = container.textContent ?? '';
  assert.match(text, /巡检机器人/);
