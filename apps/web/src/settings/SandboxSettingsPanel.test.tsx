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
} = await import('./SandboxSettingsPanel.tsx');

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

async function mount(client: WeKnoraClient, props: { role?: 'viewer' | 'admin' | 'owner'; initialData?: { items: unknown[]; workspaceScriptsDisabled: boolean }; onOpenSession?: (sessionId: string) => void } = {}) {
  const container = document.createElement('div');
  document.body.append(container);
  mountedRoot = createRoot(container);
  await act(async () => {
    mountedRoot?.render(<SandboxSettingsPanel
      client={client}
      role={props.role ?? 'admin'}
      {...(props.initialData === undefined ? {} : { initialData: props.initialData as never })}
      {...(props.onOpenSession ? { onOpenSession: props.onOpenSession } : {})}
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

beforeEach(() => {
  (dom.window as unknown as { confirm: (message: string) => boolean }).confirm = (message: string) => {
    confirmCalls.push(message);
    return true;
  };
});

// ---- list surface (SandboxSettings.vue) -------------------------------------

test('sandbox list keeps the Vue loading copy before the first load resolves', () => {
  const html = renderToStaticMarkup(<SandboxSettingsPanel client={{} as never} role="admin" />);
  assert.match(html, new RegExp(t('settings.sandbox.loading')));
});

test('sandbox list keeps viewers read-only with the Vue empty state copy', () => {
  const html = renderToStaticMarkup(<SandboxSettingsPanel
    client={{} as never}
    role="viewer"
    initialData={{ items: [], workspaceScriptsDisabled: false } as never}
  />);
  assert.match(html, new RegExp(t('settings.sandbox.title')));
  assert.match(html, new RegExp(t('settings.sandbox.noConfigs')));
  assert.doesNotMatch(html, new RegExp(t('settings.sandbox.addConfig')));
  assert.doesNotMatch(html, new RegExp(t('settings.sandbox.scriptPolicyLabel')));
  assert.doesNotMatch(html, new RegExp(t('common.edit')));
});

test('sandbox list renders Vue backend tabs with named-only counts, card warnings, target summary, and gated inventory', async () => {
  const { client } = makeClient(() => ({}), [e2bRecord, cubeRecord, cubeNoTemplateRecord, legacyRecord]);
  // Without initialData the panel loads through the mocked list endpoint.
  const container = await mount(client, { initialData: undefined });
  const text = container.textContent ?? '';
  // Vue backendLabel + countByType (SandboxSettings.vue:58-63, 286-287).
  assert.match(text, /全部 \(4\)/);
  assert.match(text, /CubeSandbox \(2\)/);
  assert.match(text, /E2B \(1\)/);
  assert.match(text, /Docker \(0\)/);
  // targetSummary (SandboxSettings.vue:401-406): the remote host tells configs apart.
  assert.match(text, /api.e2b.app/);
  // buildCardWarnings (SandboxSettings.vue:426-459): cube without a template,
  // e2b without a credential.
  assert.match(text, new RegExp(t('settings.sandbox.templateNotConfigured')));
  assert.match(text, new RegExp(t('settings.sandbox.cardCredentialMissing')));
  // Legacy rows are marked and offer no edit/inventory entries (SandboxSettings.vue:91-93, 292-304).
  assert.match(text, new RegExp(t('settings.sandbox.legacyConfig')));
  const legacyCard = Array.from(container.querySelectorAll('.wk-sandbox-card')).find((card) => card.textContent?.includes('Legacy local sandbox'))!;
  assert.ok(legacyCard);
  assert.equal(Array.from(legacyCard.querySelectorAll('button')).find((button) => button.textContent === t('common.edit')), undefined);
  assert.equal(Array.from(legacyCard.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.viewSandboxes')), undefined);
});

test('non-legacy sandbox cards open the editor from the card surface like Vue', async () => {
  const { client } = makeClient(() => okCatalog([]), [cubeRecord]);
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  const card = container.querySelector<HTMLElement>('.wk-sandbox-card');
  assert.ok(card, 'the configured sandbox card should render');
  assert.equal(card?.getAttribute('role'), 'button');

  await act(async () => card?.click());

  assert.ok(container.querySelector('[data-testid="sandbox-editor"]'), 'clicking the card should open the editor');
});

test('script policy disable requires confirmation while enable applies directly (SandboxSettings.vue:47-55)', () => {
  const enabledHtml = renderToStaticMarkup(<SandboxSettingsPanel
    client={{} as never}
    role="admin"
    initialData={{ items: [], workspaceScriptsDisabled: false } as never}
  />);
  // Enabled renders the one-way switch on; the warning popconfirm only exists
  // after the user clicks the switch (Vue t-popconfirm on t-switch).
  assert.match(enabledHtml, /role="switch" aria-checked="true"/);
  assert.doesNotMatch(enabledHtml, /data-confirm="disable-scripts"/);
  const disabledHtml = renderToStaticMarkup(<SandboxSettingsPanel
    client={{} as never}
    role="admin"
    initialData={{ items: [], workspaceScriptsDisabled: true } as never}
  />);
  assert.match(disabledHtml, /role="switch" aria-checked="false"/);
  assert.doesNotMatch(disabledHtml, /data-confirm="disable-scripts"/);
});

// ---- wizard structure (drawer.vue:34-57, 953-1006) --------------------------

test('create wizard for a remote backend renders the Vue step rail and per-step primary copy', async () => {
  const { client } = makeClient(() => okCatalog([]));
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  assert.equal(editor.getAttribute('aria-label'), t('settings.sandbox.createTitle'));
  const rail = editor.querySelector('nav[aria-label="' + t('settings.sandbox.setupProgress') + '"]');
  assert.ok(rail, 'the step rail is labelled with setupProgress');
  const steps = Array.from(rail!.querySelectorAll('.wk-sandbox-step'));
  assert.deepEqual(Array.from(rail!.querySelectorAll('.wk-sandbox-step__title')).map((node) => node.textContent), [
    t('settings.sandbox.stepConnection'),
    t('settings.sandbox.stepTemplate'),
    t('settings.sandbox.stepRuntime'),
  ]);
  assert.equal(steps[0]!.textContent, '1' + t('settings.sandbox.stepConnection'), 'the pending marker shows the step number');
  assert.equal(steps[0]!.getAttribute('aria-current'), 'step');
  // Creation mode: future steps are plain markers, not buttons (drawer.vue:1003-1006).
  assert.equal(steps[0]!.tagName, 'DIV');
  assert.equal(steps[1]!.tagName, 'DIV');
  assert.equal(steps[2]!.tagName, 'DIV');
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.stepDescriptions.connection')));
  const primary = editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-editor-primary"]')!;
  assert.equal(primary.textContent, t('settings.sandbox.connectAndContinue'));
  // The deep-check action waits for the template step on remote backends (drawer.vue:973-977).
  assert.equal(editor.querySelector('[data-testid="sandbox-deep-check"]'), null);
  assert.equal(editor.textContent?.includes(t('settings.sandbox.back')), false);
});

test('docker wizard drops the template step and runs the shallow check on the connection step (drawer.vue:953-977)', async () => {
  const checks: unknown[] = [];
  const { client, requests } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') {
      checks.push(request.body);
      return okCheck(true, [{ name: 'client_build', ok: true }]);
    }
    return okCatalog([]);
  });
  const container = await mount(client);
  // Vue createPresetType (SandboxSettings.vue:251-255): the active tab presets the backend.
  const dockerTab = Array.from(container.querySelectorAll<HTMLButtonElement>('.wk-model-tabs button')).find((button) => button.textContent!.startsWith('Docker'))!;
  await act(async () => dockerTab.click());
  const editor = await openCreateEditor(container);
  const rail = editor.querySelector('nav')!;
  assert.deepEqual(Array.from(rail.querySelectorAll('.wk-sandbox-step__title')).map((node) => node.textContent), [
    t('settings.sandbox.stepConnection'),
    t('settings.sandbox.stepRuntime'),
  ]);
  // Docker shows the WeKnora image card and risk note (drawer.vue:217-246).
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.weknoraDockerImage')));
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.dockerHostRisk')));
  // The deep check is available on the connection step for docker (drawer.vue:973-977).
  assert.ok(editor.querySelector('[data-testid="sandbox-deep-check"]'));
  const inputs = Array.from(editor.querySelectorAll('input')) as HTMLInputElement[];
  setInputValue(inputs.find((input) => input.placeholder === t('settings.sandbox.configNamePlaceholder'))!, 'Local docker');
  await submitEditor(editor);
  const checkBody = checks[0] as { deep: boolean; config: { sandbox_type: string; docker: { image?: string } } };
  assert.equal(checkBody.deep, false);
  assert.equal(checkBody.config.sandbox_type, 'docker');
  // The default image mirrors the server default (drawer.vue:796, 1139-1141).
  assert.equal(checkBody.config.docker.image, 'wechatopenai/weknora-sandbox:main');
  // Advancing kicks the background standard pull (drawer.vue:1583-1586).
  const templateQuery = requests.find((request) => request.path === '/api/v1/sandbox-configs/templates/query') as { body: { ensure_standard: boolean } };
  assert.ok(templateQuery);
  assert.equal(templateQuery.body.ensure_standard, true);
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.stepDescriptions.runtime')));
});

test('edit mode unlocks every wizard step as a direct jump (drawer.vue:998-1006)', async () => {
  const { client } = makeClient(() => okCatalog([]), [cubeRecord]);
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  const editButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === t('common.edit'))!;
  await act(async () => editButton.click());
  const editor = container.querySelector<HTMLElement>('[data-testid="sandbox-editor"]')!;
  assert.equal(editor.getAttribute('aria-label'), t('settings.sandbox.editTitle'));
  const steps = Array.from(editor.querySelectorAll('nav .wk-sandbox-step'));
  // The current step stays a plain marker; every other step is a direct jump.
  assert.equal(steps[0]!.tagName, 'DIV');
  assert.equal(steps[0]!.getAttribute('aria-current'), 'step');
  assert.equal(steps[1]!.tagName, 'BUTTON');
  assert.equal(steps[2]!.tagName, 'BUTTON');
  // Jumping straight to the template step loads the catalog (drawer.vue:1015-1019).
  const templateStep = () => Array.from(editor.querySelectorAll('nav .wk-sandbox-step'))[1] as HTMLButtonElement;
  await act(async () => templateStep().click());
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.stepDescriptions.template')));
  const connectionStep = () => Array.from(editor.querySelectorAll('nav .wk-sandbox-step'))[0] as HTMLButtonElement;
  await act(async () => connectionStep().click());
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.stepDescriptions.connection')));
});

test('visited steps stay clickable during creation while future ones do not (drawer.vue:998-1006)', async () => {
  const checkGate = deferred<unknown>();
  const { client } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') return checkGate.promise;
    return okCatalog([]);
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(true, [{ name: 'client_build', ok: true }])));
  await submitting;
  const steps = Array.from(editor.querySelectorAll('nav .wk-sandbox-step'));
  // Now on the template step: connection is a clickable way back, runtime is not.
  assert.equal(steps[0]!.tagName, 'BUTTON');
  assert.equal(steps[1]!.getAttribute('aria-current'), 'step');
  assert.equal(steps[2]!.tagName, 'DIV');
  const back = Array.from(editor.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.back'));
  assert.ok(back, 'the back button appears after the first step (drawer.vue:20-22)');
});

// ---- validation (drawer.vue:1061-1109, 1234-1248, 1562-1569) ----------------

test('connection validation mirrors Vue required fields, copy, and clearing triggers', async () => {
  const { client } = makeClient(() => okCatalog([]));
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  await submitEditor(editor);
  // An empty name short-circuits before the field matrix runs (drawer.vue:1573-1574).
  let alerts = Array.from(editor.querySelectorAll('[role="alert"]')).map((node) => node.textContent);
  assert.deepEqual(alerts, [t('settings.sandbox.configNameRequired')]);
  const inputs = Array.from(editor.querySelectorAll('input')) as HTMLInputElement[];
  setInputValue(inputs.find((input) => input.placeholder === t('settings.sandbox.configNamePlaceholder'))!, 'New cube config');
  await submitEditor(editor);
  // With a name, every cube connection field is flagged (drawer.vue:1064-1109).
  alerts = Array.from(editor.querySelectorAll('[role="alert"]')).map((node) => node.textContent);
  assert.equal(alerts.filter((text) => text === t('settings.sandbox.fieldRequired')).length, 3);
  // Typing into one field clears only its error (drawer.vue:1079-1087).
  setInputValue(inputs.find((input) => input.placeholder === 'http://cube.example.com:33000')!, 'http://cube.example.com:33000');
  await act(async () => {});
  const remaining = Array.from(editor.querySelectorAll('[role="alert"]')).map((node) => node.textContent);
  assert.equal(remaining.filter((text) => text === t('settings.sandbox.fieldRequired')).length, 2);
  // Switching backend clears carried errors (drawer.vue:1692-1697).
  const backendSelect = editor.querySelector('select')!;
  setSelectValue(backendSelect, 'e2b');
  await act(async () => {});
  assert.equal(Array.from(editor.querySelectorAll('[role="alert"]')).length, 0);
});

test('e2b requires only the API key on the connection step (drawer.vue:1064-1068)', async () => {
  const { client } = makeClient(() => okCatalog([]));
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  setSelectValue(editor.querySelector('select')!, 'e2b');
  await act(async () => {});
  const inputs = Array.from(editor.querySelectorAll('input')) as HTMLInputElement[];
  setInputValue(inputs.find((input) => input.placeholder === t('settings.sandbox.configNamePlaceholder'))!, 'E2B config');
  await submitEditor(editor);
  const alerts = Array.from(editor.querySelectorAll('[role="alert"]')).map((node) => node.textContent);
  assert.deepEqual(alerts.filter((text) => text === t('settings.sandbox.fieldRequired')), [t('settings.sandbox.fieldRequired')]);
});

// ---- template catalog (drawer.vue:260-364, 1389-1448) -----------------------

const catalogTemplates = [
  { id: 'tpl-ready', name: 'weknora-standard', status: 'ready', image: 'ghcr.io/weknora/sandbox:main', version: 'v3', created_at: '2030-01-02T03:04:05Z', standard: true },
  { id: 'tpl-building', name: 'weknora-next', status: 'building', standard: false },
  { id: 'tpl-failed', name: 'broken', status: 'failed', error: 'registry auth rejected', standard: false },
  { id: 'tpl-untagged', name: 'untagged-build', status: 'untagged', standard: false },
  { id: 'tpl-mystery', name: 'odd-status', status: 'weird', standard: false },
];

test('template step lists the cluster catalog with Vue statuses, fields, and selection semantics', async () => {
  const checkGate = deferred<unknown>();
  const { client, requests } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') return checkGate.promise;
    if (request.method === 'POST' && request.path === '/api/v1/sandbox-configs/templates/query') {
      return okCatalog(catalogTemplates, 'tpl-ready');
    }
    return okCatalog([]);
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(true, [{ name: 'client_build', ok: true }])));
  await submitting;
  const query = requests.find((request) => request.path === '/api/v1/sandbox-configs/templates/query') as { body: { config: { sandbox_type: string }; config_id?: string; ensure_standard: boolean } };
  assert.equal(query.body.config.sandbox_type, 'cube');
  assert.equal(query.body.ensure_standard, false);
  assert.equal(query.body.config_id, undefined);

  const text = editor.textContent ?? '';
  // Status labels (drawer.vue:1368-1374).
  assert.match(text, /已就绪/);
  assert.match(text, /构建中/);
  assert.match(text, /失败/);
  assert.match(text, new RegExp(t('settings.sandbox.templateStatuses.untagged')));
  assert.match(text, new RegExp(t('settings.sandbox.templateStatuses.unknown')));
  // Field rows (drawer.vue:1287-1327).
  assert.match(text, /ghcr\.io\/weknora\/sandbox:main/);
  assert.match(text, /v3/);
  // Provider failure reason verbatim (drawer.vue:1354-1358).
  assert.match(text, new RegExp(t('settings.sandbox.templateFailedReason', { reason: 'registry auth rejected' }).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  // Untagged hint (drawer.vue:341-343).
  assert.match(text, new RegExp(t('settings.sandbox.templateUntaggedHint')));
  // Auto-selection picks the ready standard template (drawer.vue:1423-1425).
  const rows = Array.from(editor.querySelectorAll<HTMLElement>('[role="radio"]'));
  const selected = rows.find((row) => row.getAttribute('aria-checked') === 'true')!;
  assert.ok(selected.textContent!.includes('weknora-standard'));
  // Pending rows are disabled and cannot be selected (drawer.vue:1262-1266, 1329-1333).
  const pendingRow = rows.find((row) => row.textContent!.includes('weknora-next'))!;
  assert.equal(pendingRow.getAttribute('aria-disabled'), 'true');
  await act(async () => pendingRow.click());
  assert.equal(rows.find((row) => row.getAttribute('aria-checked') === 'true')!.textContent!.includes('weknora-standard'), true);
  // The primary stays enabled because the ready standard is selected.
  assert.equal(editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-editor-primary"]')!.disabled, false);
  // The standard template offers the rebuild action (drawer.vue:1268-1270, 323-333).
  const rebuild = Array.from(editor.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.replaceStandardTemplate'));
  assert.ok(rebuild, 'the ready standard template exposes the rebuild action');
  await act(async () => rebuild!.click());
  assert.equal(confirmCalls[0], t('settings.sandbox.replaceStandardTemplateConfirm'));
  const replaceQuery = requests.filter((request) => request.path === '/api/v1/sandbox-configs/templates/query').pop() as { body: { replace_standard: boolean } };
  assert.equal(replaceQuery.body.replace_standard, true);
});

test('a catalog without any ready template keeps the wizard gated with the Vue empty copy', async () => {
  const checkGate = deferred<unknown>();
  const { client } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') return checkGate.promise;
    return okCatalog([{ id: 'tpl-building', name: 'still-building', status: 'building', standard: true }], 'tpl-building');
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(true, [{ name: 'client_build', ok: true }])));
  await submitting;
  // The pending standard is auto-selected but blocks continuing (drawer.vue:979-985).
  const primary = editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-editor-primary"]')!;
  assert.equal(primary.disabled, true);
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.templateBuildingHint')));
  // The create-standard offer is hidden because a standard entry exists (drawer.vue:950-952).
  assert.equal(Array.from(editor.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.createStandardTemplate')), undefined);
});

test('an empty catalog offers creating the WeKnora standard template with ensure_standard (drawer.vue:276-292, 1441-1443)', async () => {
  const checkGate = deferred<unknown>();
  const { client, requests } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') return checkGate.promise;
    return okCatalog([], '', true);
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(true, [{ name: 'client_build', ok: true }])));
  await submitting;
  const create = Array.from(editor.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.createStandardTemplate'));
  assert.ok(create, 'the offer row appears when no standard template exists');
  await act(async () => create!.click());
  const ensureQuery = requests.filter((request) => request.path === '/api/v1/sandbox-configs/templates/query').pop() as { body: { ensure_standard: boolean } };
  assert.equal(ensureQuery.body.ensure_standard, true);
  // provisioned feedback (drawer.vue:1426-1430).
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.standardTemplateProvisioning')));
});

// ---- deep check (drawer.vue:19-32, 649-676, 712-735) ------------------------

test('deep check confirms, posts deep=true, and renders the structured result with scope hints', async () => {
  const checkGate = deferred<unknown>();
  const deepGate = deferred<unknown>();
  let deepCalls = 0;
  const { client } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') {
      const body = request.body as { deep: boolean };
      if (body.deep) { deepCalls += 1; return deepGate.promise; }
      return checkGate.promise;
    }
    return okCatalog(catalogTemplates, 'tpl-ready');
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(true, [{ name: 'client_build', ok: true, latency_ms: 12 }])));
  await submitting;
  const deepButton = editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-deep-check"]')!;
  assert.equal(deepButton.textContent, t('settings.sandbox.deepCheck'));
  await act(async () => deepButton.click());
  assert.equal(confirmCalls[0], t('settings.sandbox.deepCheckConfirm'), 'the deep check asks before consuming real sandbox time');
  await act(async () => deepGate.resolve(okCheck(true, [
    { name: 'client_build', ok: true, latency_ms: 8 },
    { name: 'api_url_reachable', ok: true, latency_ms: 41 },
    { name: 'credential_valid', ok: true },
    { name: 'template_exists', ok: null, reason: 'needs_deep_check' },
    { name: 'sandbox_exec', ok: null, reason: 'needs_deep_check' },
    { name: 'egress_available', ok: null, reason: 'needs_deep_check' },
  ])));
  const result = editor.querySelector('[data-testid="sandbox-check-result"]')!;
  const text = result.textContent ?? '';
  assert.match(text, new RegExp(t('settings.sandbox.checkPassed')));
  // Scope hint: pending probes mean the verdict covered the control plane only (drawer.vue:1046-1050).
  assert.match(text, new RegExp(t('settings.sandbox.checkScopeConnection')));
  // Reported checks are labelled from checks.* and keep their latency (drawer.vue:722-731).
  assert.match(text, new RegExp(t('settings.sandbox.checks.client_build')));
  assert.match(text, /41 ms/);
  // Deferred probes are explained once (drawer.vue:732-734).
  assert.match(text, new RegExp(t('settings.sandbox.checkPendingHint', {
    names: [t('settings.sandbox.checks.template_exists'), t('settings.sandbox.checks.sandbox_exec'), t('settings.sandbox.checks.egress_available')].join('、'),
  })));
  // After a deep run the action flips to "recheck" (drawer.vue:29, 811).
  assert.equal(editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-deep-check"]')!.textContent, t('settings.sandbox.recheck'));
  assert.equal(deepCalls, 1);
});

test('a failing shallow check keeps the wizard on the connection step (drawer.vue:1571-1576)', async () => {
  const checkGate = deferred<unknown>();
  const { client, requests } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') return checkGate.promise;
    return okCatalog([]);
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(false, [
    { name: 'client_build', ok: true },
    { name: 'api_url_reachable', ok: false, message: 'dial tcp: connection refused' },
  ])));
  await submitting;
  // Still on connection: templates were never queried.
  assert.equal(requests.find((request) => request.path === '/api/v1/sandbox-configs/templates/query'), undefined);
  assert.match(editor.textContent ?? '', new RegExp(t('settings.sandbox.stepDescriptions.connection')));
});

test('an egress-restricted deep check reports the policy-restricted scope (drawer.vue:1038-1050)', async () => {
  const result = {
    ok: true, checks: [
      { name: 'client_build', ok: true },
      { name: 'egress_available', ok: null, reason: 'egress_restricted_by_policy' },
    ],
  } as never;
  assert.equal(checkScopeKey(result), 'settings.sandbox.checkScopePolicyRestricted');
  assert.deepEqual(pendingCheckItems(result), []);
  assert.deepEqual(reportedChecks(result).map((item) => item.name), ['client_build', 'egress_available']);
  assert.equal(checkScopeKey({ ok: true, checks: [{ name: 'x', ok: true }] } as never), 'settings.sandbox.checkScopeFull');
});

// ---- save flow (drawer.vue:1610-1647) ---------------------------------------

test('runtime save submits the exact Vue payload shape and closes on success', async () => {
  let updateInput: unknown;
  const { client } = makeClient(() => okCatalog(catalogTemplates, 'tpl-ready'), [cubeRecord]);
  (client as unknown as { sandboxConfigurations: { update: (id: string, input: unknown) => Promise<unknown> } }).sandboxConfigurations.update = async (_id, input) => {
    updateInput = input;
    return {};
  };
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  const editButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === t('common.edit'))!;
  await act(async () => editButton.click());
  const editor = container.querySelector<HTMLElement>('[data-testid="sandbox-editor"]')!;
  // Jump straight to runtime — every step is open while editing.
  const runtimeStep = Array.from(editor.querySelectorAll('nav .wk-sandbox-step'))[2] as HTMLButtonElement;
  await act(async () => runtimeStep.click());
  const primary = editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-editor-primary"]')!;
  assert.equal(primary.textContent, t('common.save'));
  await submitEditor(editor);
  const payload = updateInput as { name: string; description: string; config: Record<string, unknown> & { cube: Record<string, unknown>; network: Record<string, unknown>; env_vars: Record<string, string> } };
  assert.equal(payload.name, 'Cube cluster');
  // Only the selected backend block is sent (drawer.vue:1473-1477).
  assert.equal(payload.config.sandbox_type, 'cube');
  assert.ok(payload.config.cube);
  assert.equal(payload.config.e2b, undefined);
  assert.equal(payload.config.docker, undefined);
  assert.deepEqual(payload.config.env_vars, {});
  assert.deepEqual(payload.config.network, {});
  // The drawer closes and the list refreshes (drawer.vue:1626-1632).
  assert.equal(container.querySelector('[data-testid="sandbox-editor"]'), null);
  assert.match(container.textContent ?? '', new RegExp(t('common.saveSuccess')));
});

test('a sandboxes_still_live save refusal keeps the drawer open with the Vue conflict alert (drawer.vue:63-81, 1634-1641)', async () => {
  const refusal = Object.assign(new Error('sandboxes still live'), {
    code: 'sandboxes_still_live',
    details: { sandbox_count: 2, session_ids: ['s1', 's2'], agent_names: ['Helper'], unverifiable: false },
  });
  const { client } = makeClient(() => okCatalog(catalogTemplates, 'tpl-ready'), [cubeRecord]);
  (client as unknown as { sandboxConfigurations: { update: (id: string, input: unknown) => Promise<unknown> } }).sandboxConfigurations.update = async () => {
    throw refusal;
  };
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  const editButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === t('common.edit'))!;
  await act(async () => editButton.click());
  const editor = container.querySelector<HTMLElement>('[data-testid="sandbox-editor"]')!;
  const runtimeStep = Array.from(editor.querySelectorAll('nav .wk-sandbox-step'))[2] as HTMLButtonElement;
  await act(async () => runtimeStep.click());
  await submitEditor(editor);
  const alert = editor.querySelector('[data-testid="sandbox-editor-conflict"]')!;
  const text = alert.textContent ?? '';
  assert.match(text, new RegExp(t('settings.sandbox.sandboxesStillLive', { count: 2 }).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  assert.match(text, new RegExp(t('settings.sandbox.affectedSessions', { count: 2 }).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  assert.match(text, new RegExp(t('settings.sandbox.affectedAgents', { names: 'Helper' }).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')));
  assert.match(text, new RegExp(t('settings.sandbox.blockedHint')));
  assert.ok(container.querySelector('[data-testid="sandbox-editor"]'), 'the drawer stays open with the form intact');
});

// ---- egress warning (drawer.vue:912-932) ------------------------------------

test('a domain allow-list without deny-all surfaces the Vue warning, pure rules included', async () => {
  assert.equal(denyOutRowCoversAllIPv4('0.0.0.0/0'), true);
  assert.equal(denyOutRowCoversAllIPv4('1.2.3.4/0'), true);
  assert.equal(denyOutRowCoversAllIPv4('10.0.0.0/8'), false);
  assert.equal(domainAllowNeedsDenyAll(['*.example.com'], [], false), true);
  assert.equal(domainAllowNeedsDenyAll(['*.example.com'], ['1.2.3.4/0'], false), false);
  assert.equal(domainAllowNeedsDenyAll(['*.example.com'], [], true), false);
  assert.equal(domainAllowNeedsDenyAll(['203.0.113.9/32'], [], false), false);

  const checkGate = deferred<unknown>();
  const { client } = makeClient((request) => {
    if (request.method === 'POST' && request.path === '/api/v1/system/sandbox-check') return checkGate.promise;
    return okCatalog(catalogTemplates, 'tpl-ready');
  });
  const container = await mount(client);
  const editor = await openCreateEditor(container);
  fillCubeConnection(editor);
  const submitting = submitEditor(editor);
  await act(async () => checkGate.resolve(okCheck(true, [{ name: 'client_build', ok: true }])));
  await submitting;
  const primary = editor.querySelector<HTMLButtonElement>('[data-testid="sandbox-editor-primary"]')!;
  await act(async () => primary.click());
  const runtimeStep = Array.from(editor.querySelectorAll('nav .wk-sandbox-step'))[2] as HTMLButtonElement;
  await act(async () => runtimeStep.click());
  assert.equal(editor.querySelector('[data-testid="domain-allow-warning"]'), null);
  // Add a domain allow target without deny-all (drawer.vue:478-480).
  const addTarget = Array.from(editor.querySelectorAll('button')).find((button) => button.textContent === t('settings.sandbox.addTarget'))!;
  await act(async () => addTarget.click());
  const allowInput = editor.querySelector<HTMLInputElement>('.wk-net-row input[placeholder="' + t('settings.sandbox.allowOutPlaceholder') + '"]')!;
  setInputValue(allowInput, '*.example.com');
  await act(async () => {});
  assert.match(editor.querySelector('[data-testid="domain-allow-warning"]')!.textContent ?? '', new RegExp(t('settings.sandbox.domainAllowNeedsDenyAll')));
});

// ---- pure helpers mapped to Vue ---------------------------------------------

test('wizard helpers mirror the Vue step order, gating, and primary copy', () => {
  assert.deepEqual(wizardStepKeys('cube'), ['connection', 'template', 'runtime']);
  assert.deepEqual(wizardStepKeys('e2b'), ['connection', 'template', 'runtime']);
  assert.deepEqual(wizardStepKeys('docker'), ['connection', 'runtime']);
  assert.equal(canJumpToStep(2, 0, false), false);
  assert.equal(canJumpToStep(0, 1, false), true);
  assert.equal(canJumpToStep(2, 0, true), true);
  assert.equal(canDeepCheckOnStep('connection', 'docker'), true);
  assert.equal(canDeepCheckOnStep('connection', 'cube'), false);
  assert.equal(canDeepCheckOnStep('template', 'cube'), true);
  assert.equal(canDeepCheckOnStep('runtime', 'cube'), false);
  assert.equal(primaryTextKey('connection'), 'settings.sandbox.connectAndContinue');
  assert.equal(primaryTextKey('template'), 'common.next');
  assert.equal(primaryTextKey('runtime'), 'common.save');
  assert.equal(templateStatusKey({ id: 'x', standard: false, status: 'READY' }), 'ready');
  assert.equal(templateStatusKey({ id: 'x', standard: false, status: 'waiting' }), 'building');
  assert.equal(templateStatusKey({ id: 'x', standard: false, status: 'untagged' }), 'untagged');
  assert.equal(templateStatusKey({ id: 'x', standard: false, status: 'canceled' }), 'failed');
  assert.equal(templateStatusKey({ id: 'x', standard: false, status: 'weird' }), 'unknown');
});

test('collectSandboxConfig reproduces the Vue payload rules for secrets and network policy', () => {
  const record = {
    id: 'sandbox-x',
    name: 'Cube',
    sandbox_type: 'cube',
    config: {
      sandbox_type: 'cube',
      env_vars: { KEEP: '***', PLAIN: 'visible' },
      cube: { api_url: 'http://cube:33000', proxy_url: 'http://cube:80', sandbox_domain: 'cube.app', template_id: 'tpl-1', api_key: '***' },
      network: {
        allow_out: [' *.example.com ', ''],
        deny_out: [' 10.0.0.0/8'],
        cube_rules: [{
          name: 'pay',
          scheme: 'https',
          host: 'api.pay.com',
          methods: ['POST'],
          deny: false,
          inject: [{ header: 'Authorization', secret: '***', format: 'Bearer ${SECRET}' }],
        }, {
          name: 'block',
          deny: true,
          inject: [{ header: 'X', secret: 'new' }],
        }],
        e2b_host_rules: [{ host: 'e2b.example.com', headers: { Token: '***', '': 'dropped' } }],
      },
    },
    created_at: '2030-01-01T00:00:00Z',
    updated_at: '2030-01-01T00:00:00Z',
  } as never;
  const form = initialEditorForm(record, '');
  // Blank-but-stored secrets keep the placeholder on submit (drawer.vue:1452-1455).
  assert.equal(form.storedCubeKey, true);
  const config = collectSandboxConfig(form);
  const network = collectNetworkPolicy(form);
  assert.equal(config.cube?.api_key, '***');
  assert.deepEqual(config.env_vars, { KEEP: '***', PLAIN: 'visible' });
  // Rows are trimmed and empties dropped (drawer.vue:1493-1496).
  assert.deepEqual(network.allow_out, ['*.example.com']);
  assert.deepEqual(network.deny_out, ['10.0.0.0/8']);
  // Methods are uppercased; deny rules drop inject entries (drawer.vue:1499-1533).
  const allowRule = network.cube_rules?.[0]!;
  assert.equal(allowRule.name, 'pay');
  assert.equal(allowRule.methods?.[0], 'POST');
  assert.equal(allowRule.inject?.[0]?.secret, '***');
  assert.equal(network.cube_rules?.[1]?.inject, undefined);
  // e2b rules are serialized only for an e2b config (drawer.vue:1534-1535).
  assert.equal(network.e2b_host_rules, undefined);
  const e2bForm = initialEditorForm({
    id: 'sandbox-e2b',
    name: 'E2B',
    sandbox_type: 'e2b',
    config: {
      sandbox_type: 'e2b',
      e2b: { api_key: '***', api_url: 'https://api.e2b.app', template_id: 'tpl-9' },
      network: { e2b_host_rules: [{ host: 'e2b.example.com', headers: { Token: '***', '': 'dropped' } }] },
    },
    created_at: '2030-01-01T00:00:00Z',
    updated_at: '2030-01-01T00:00:00Z',
  } as never, '');
  // Stored e2b header secrets survive only under their original host (drawer.vue:1534-1553).
  assert.deepEqual(collectNetworkPolicy(e2bForm).e2b_host_rules, [{ host: 'e2b.example.com', headers: { Token: '***' } }]);
  // Numbers left empty stay unset (drawer.vue:1466-1467).
  assert.equal(config.default_timeout_sec, undefined);
  assert.equal(config.allow_private_endpoints, undefined);
});

test('missingRequiredFields mirrors the Vue per-backend field matrix', () => {
  const empty = initialEditorForm(null, '');
  assert.deepEqual(missingRequiredFields({ ...empty, backend: 'cube' }, false).sort(), ['api_url', 'proxy_url', 'sandbox_domain']);
  assert.deepEqual(missingRequiredFields({ ...empty, backend: 'e2b' }, false), ['api_key']);
  // template_id is required only once the wizard reaches it (drawer.vue:1098).
  assert.deepEqual(missingRequiredFields({ ...empty, backend: 'cube', cube: { api_url: 'a', proxy_url: 'b', sandbox_domain: 'c' } }, true), ['template_id']);
  assert.deepEqual(missingRequiredFields({ ...empty, backend: 'docker', docker: { image: DEFAULT_DOCKER_IMAGE } }), []);
});

// ---- localization -----------------------------------------------------------

test('shared settings keys localize per locale through the settingsT pattern', async () => {
  const zhTitle = formatMessage('zh-CN', 'settings.sandbox.title');
  const enTitle = formatMessage('en-US', 'settings.sandbox.title');
  assert.equal(t('settings.sandbox.title'), zhTitle);
  assert.equal(sandboxT('en-US')('settings.sandbox.title'), enTitle);
  assert.notEqual(zhTitle, enTitle, 'the shared package must localize the panel title');
  // Fallback-only drawer keys resolve to the Vue zh-CN copy verbatim.
  assert.equal(t('settings.sandbox.stepConnection'), '连接');
  assert.equal(t('settings.sandbox.fieldRequired'), '必填');
  assert.equal(t('settings.sandbox.checks.client_build'), '客户端构建');
  assert.equal(t('settings.sandbox.templateNotReady'), '所选模板尚未构建完成，请刷新并等待状态就绪');
  assert.equal(t('settings.sandbox.checkPendingHint', { names: '模板存在性' }), '模板存在性 需要「完整验证」才能确认：会真实创建一个临时沙箱、执行一次脚本再销毁。');
  assert.equal(t('settings.sandbox.backends.docker'), 'Docker');
});

test('the panel renders English copy when the stored locale is en-US', async () => {
  window.localStorage.setItem('locale', 'en-US');
  const { client } = makeClient(() => ({}), []);
  const container = await mount(client);
  const heading = container.querySelector('h3')!;
  assert.equal(heading.textContent, formatMessage('en-US', 'settings.sandbox.title'));
  assert.match(container.textContent ?? '', new RegExp(formatMessage('en-US', 'common.all')));
});

// ---- inventory session titles (SandboxSettings.vue:140-206, 312-344, 511-528) -

async function openInventory(container: HTMLElement): Promise<void> {
  const viewButton = Array.from(container.querySelectorAll('button'))
    .find((button) => button.textContent === t('settings.sandbox.viewSandboxes'));
  assert.ok(viewButton, 'cube cards offer the inventory entry');
  await act(async () => viewButton!.click());
}

test('inventory resolves session ids into titles with Vue fallbacks for failed lookups (SandboxSettings.vue:171-178, 323-339)', async () => {
  const requested: string[] = [];
  const { client } = makeClient(() => ({}), [cubeRecord]);
  (client.sandboxConfigurations as { inventory: unknown }).inventory = async () => ({
    sandboxCount: 3,
    // A duplicate and a blank id must not trigger extra lookups (324-330).
    sessionIds: ['session-a', 'session-b', 'session-a', '   '],
    agentNames: [],
  });
  (client.sessions as { get: (id: string) => Promise<unknown> }).get = async (id: string) => {
    requested.push(id);
    if (id === 'session-a') return { id, title: '  季度盘点助手  ', is_pinned: false };
    throw new Error('session gone');
  };
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  await openInventory(container);

  const text = container.textContent ?? '';
  assert.ok(container.querySelector('.wk-sandbox-inventory-overlay'), 'inventory should mount in a Vue-style drawer overlay');
  assert.ok(container.querySelector('.wk-sandbox-inventory-drawer'), 'inventory should use a right-side drawer surface');
  // Loaded titles render trimmed (SandboxSettings.vue:333).
  assert.match(text, /季度盘点助手/);
  // A failed lookup reads as "untitled" instead of breaking the list (334-336, 317-321).
  assert.match(text, new RegExp(t('settings.sandbox.inventoryUntitledSession')));
  // The raw id stays reachable as the row tooltip (173 :title="id").
  const titledRow = container.querySelector('.wk-sandbox-inventory li strong[title="session-a"]');
  assert.equal(titledRow?.getAttribute('title'), 'session-a');
  // Blank ids are dropped and duplicates collapse into one lookup per id.
  assert.deepEqual(requested, ['session-a', 'session-b']);
});

test('inventory stays hidden while session titles resolve, then renders them (SandboxSettings.vue:515-527)', async () => {
  const { client } = makeClient(() => ({}), [cubeRecord]);
  (client.sandboxConfigurations as { inventory: unknown }).inventory = async () => ({
    sandboxCount: 1, sessionIds: ['session-a'], agentNames: [],
  });
  let releaseTitle!: (session: { id: string; title: string; is_pinned: boolean }) => void;
  const titleGate = new Promise<{ id: string; title: string; is_pinned: boolean }>((resolve) => { releaseTitle = resolve; });
  (client.sessions as { get: (id: string) => Promise<unknown> }).get = () => titleGate;
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  await openInventory(container);

  // Vue keeps the drawer inside t-loading until the titles land (515-527),
  // so the list must not render raw ids first.
  const listBeforeTitles = container.querySelector('.wk-sandbox-inventory');
  releaseTitle({ id: 'session-a', title: '巡检机器人', is_pinned: false });
  await act(async () => { await titleGate; });
  const textAfterTitles = container.textContent ?? '';

  // Assertions run only after the flow has fully settled.
  assert.equal(listBeforeTitles === null, true, 'the inventory list must not render raw ids before titles resolve');
  assert.equal(textAfterTitles.includes('巡检机器人'), true, 'the loaded title renders once titles resolve');
  assert.equal(textAfterTitles.includes(t('settings.sandbox.inventorySessionKind')), true, 'the row keeps the session-kind label');
  const text = container.textContent ?? '';
  assert.match(text, /巡检机器人/);
  assert.match(text, new RegExp(t('settings.sandbox.inventorySessionKind')));
});

// ---- openSession navigation (SandboxSettings.vue:171, 341-344) ---------------

test('inventory session rows open the chat session through onOpenSession (SandboxSettings.vue:171, 341-344)', async () => {
  const { client } = makeClient(() => ({}), [cubeRecord]);
  (client.sandboxConfigurations as { inventory: unknown }).inventory = async () => ({
    sandboxCount: 2, sessionIds: ['session-a', 'session-b'], agentNames: [],
  });
  const opened: string[] = [];
  const container = await mount(client, {
    initialData: { items: [cubeRecord], workspaceScriptsDisabled: false },
    onOpenSession: (sessionId) => { opened.push(sessionId); },
  });
  await openInventory(container);
  const rowButtons = Array.from(container.querySelectorAll<HTMLElement>('.wk-sandbox-inventory li button'));
  if (rowButtons[0]) await act(async () => rowButtons[0].click());

  // Assertions run only after the flow has fully settled.
  assert.deepEqual(opened, ['session-a'], 'clicking a row reports its session id exactly once');
  assert.equal(container.querySelector('.wk-sandbox-inventory-overlay'), null, 'Vue closes the occupancy drawer before opening the chat session');
  assert.equal(rowButtons.length, 2, 'every session row is a click target');
});

test('inventory rows render inert without onOpenSession so embeds stay click-safe (SandboxSettings.vue:171)', async () => {
  const { client } = makeClient(() => ({}), [cubeRecord]);
  (client.sandboxConfigurations as { inventory: unknown }).inventory = async () => ({
    sandboxCount: 1, sessionIds: ['session-a'], agentNames: [],
  });
  const container = await mount(client, { initialData: { items: [cubeRecord], workspaceScriptsDisabled: false } });
  await openInventory(container);

  const text = container.textContent ?? '';
  assert.equal(container.querySelectorAll('.wk-sandbox-inventory li button').length, 0, 'no row renders a click affordance');
  assert.ok(container.querySelector('.wk-sandbox-inventory li strong[title="session-a"]'), 'the static row keeps the raw-id tooltip');
  assert.match(text, new RegExp(t('settings.sandbox.inventoryUntitledSession')));
});
