import assert from 'node:assert/strict';
import test from 'node:test';

// Vue baseline: frontend/src/components/AgentEmbedChannelPanel.vue (6-step
// SettingDrawer wizard) + frontend/src/utils/embedAllowedOrigins.ts +
// frontend/src/api/embed/index.ts snippet builders.
import {
  EMBED_WIZARD_STEPS,
  WEKNORA_BRAND_COLOR,
  buildEmbedCreatePayload,
  buildEmbedUpdatePayload,
  createEmbedWizardForm,
  embedChannelKeyDisplay,
  embedChannelUrl,
  embedIframeSnippet,
  embedOriginsTextFromChannel,
  embedSecureServerGoExample,
  embedSecureServerNodeExample,
  embedSecureWidgetSnippet,
  embedSnippetScenarioKey,
  embedWidgetSnippet,
  embedWizardFormFromChannel,
  embedWizardSteps,
  mapEmbedOriginsApiError,
  parseEmbedAllowedOrigins,
  validateEmbedAllowedOrigins,
  validateEmbedWizardStep,
} from './embedWizard.ts';

test('wizard keeps the Vue 5-step create structure and the edit-only deploy step', () => {
  assert.deepEqual(EMBED_WIZARD_STEPS.map((step) => step.key), [
    'channel', 'security', 'capabilities', 'appearance', 'webhook',
  ]);
  assert.deepEqual(EMBED_WIZARD_STEPS.map((step) => step.titleKey), [
    'embedPublish.stepChannel',
    'embedPublish.stepSecurity',
    'embedPublish.stepCapabilities',
    'embedPublish.stepAppearance',
    'embedPublish.stepWebhook',
  ]);
  assert.deepEqual(embedWizardSteps(false).map((step) => step.key), [
    'channel', 'security', 'capabilities', 'appearance', 'webhook',
  ]);
  // Vue stepTitles (lines 601-611) appends the deploy step only while editing.
  assert.deepEqual(embedWizardSteps(true).map((step) => step.key), [
    'channel', 'security', 'capabilities', 'appearance', 'webhook', 'deploy',
  ]);
  assert.equal(embedWizardSteps(true)[5].titleKey, 'embedPublish.stepDeploy');
});

test('default form mirrors Vue defaultForm (lines 502-517)', () => {
  const form = createEmbedWizardForm();
  assert.equal(form.agentId, '');
  assert.equal(form.name, '');
  assert.equal(form.welcomeMessage, '');
  assert.equal(form.rateLimitPerMinute, 30);
  assert.equal(form.rateLimitPerDay, 10000);
  assert.equal(form.primaryColor, WEKNORA_BRAND_COLOR);
  assert.equal(WEKNORA_BRAND_COLOR, '#07C05F');
  assert.equal(form.pageTitle, '');
  assert.equal(form.headerTitleMode, 'channel');
  assert.equal(form.showSuggestedQuestions, true);
  assert.equal(form.widgetPosition, 'bottom-right');
  assert.equal(form.allowWebSearch, false);
  assert.equal(form.allowFileUpload, false);
  assert.equal(form.defaultLocale, '');
  assert.equal(form.webhookUrl, '');
  assert.equal(form.webhookSecret, '');
  assert.equal(createEmbedWizardForm('#123456').primaryColor, '#123456');
});

test('origins parsing mirrors Vue parseAllowedOrigins (split/trim/filter)', () => {
  assert.deepEqual(parseEmbedAllowedOrigins(' https://a.example.com \n https://b.example.com\n'), [
    'https://a.example.com', 'https://b.example.com',
  ]);
  assert.deepEqual(parseEmbedAllowedOrigins('   '), []);
});

test('origins validation mirrors the backend rules (required / wildcard_prod / invalid)', () => {
  const empty = validateEmbedAllowedOrigins([]);
  assert.equal(empty.ok, false);
  if (!empty.ok) assert.deepEqual(empty.error, { code: 'required' });

  assert.deepEqual(validateEmbedAllowedOrigins(['*'], false), { ok: true, origins: ['*'] });
  const prodWildcard = validateEmbedAllowedOrigins(['*'], true);
  if (!prodWildcard.ok) assert.deepEqual(prodWildcard.error, { code: 'wildcard_prod' });
  assert.equal(prodWildcard.ok, false);

  const invalid = validateEmbedAllowedOrigins(['https://ok.example.com', 'not a url']);
  assert.equal(invalid.ok, false);
  if (!invalid.ok) assert.deepEqual(invalid.error, { code: 'invalid', origin: 'not a url' });

  // Vue prefixes '*.' hosts with https:// before the URL check.
  assert.deepEqual(validateEmbedAllowedOrigins(['*.example.com']), { ok: true, origins: ['*.example.com'] });
  assert.equal(validateEmbedAllowedOrigins(['ftp://example.com']).ok, false);
  assert.deepEqual(validateEmbedAllowedOrigins(['http://localhost:8080']), { ok: true, origins: ['http://localhost:8080'] });
});

test('step 0 blocks Next without a bound agent (Vue validateWizardStep line 620)', () => {
  assert.deepEqual(validateEmbedWizardStep({ agentId: '' }, '', 0), { key: 'integrations.selectAgentHint' });
  assert.equal(validateEmbedWizardStep({ agentId: 'agent-1' }, '', 0), null);
});

test('step 1 validates origins and passes clean text (Vue validateWizardStep lines 624-632)', () => {
  assert.deepEqual(validateEmbedWizardStep({ agentId: 'a' }, '', 1), { key: 'embedPublish.originsRequired' });
  assert.deepEqual(validateEmbedWizardStep({ agentId: 'a' }, 'https://ok\nbroken', 1), {
    key: 'embedPublish.originsInvalid', values: { origin: 'broken' },
  });
  assert.deepEqual(validateEmbedWizardStep({ agentId: 'a' }, '*', 1, true), { key: 'embedPublish.originsWildcardProd' });
  assert.equal(validateEmbedWizardStep({ agentId: 'a' }, 'https://shop.example.com', 1), null);
});

test('API origin errors map to the Vue messages (mapOriginsApiError lines 882-894)', () => {
  assert.deepEqual(mapEmbedOriginsApiError('at least one allowed origin is required'), { key: 'embedPublish.originsRequired' });
  assert.deepEqual(mapEmbedOriginsApiError("wildcard origin '*' is not allowed in production"), { key: 'embedPublish.originsWildcardProd' });
  assert.deepEqual(
    mapEmbedOriginsApiError('invalid allowed origin: "https://bad.example.com"'),
    { key: 'embedPublish.originsInvalid', values: { origin: 'https://bad.example.com' } },
  );
  assert.equal(mapEmbedOriginsApiError('channel not found'), null);
  assert.equal(mapEmbedOriginsApiError(''), null);
});

test('create payload mirrors Vue saveForm (lines 911-929) field by field', () => {
  const form = { ...createEmbedWizardForm(), agentId: 'agent-1' };
  const payload = buildEmbedCreatePayload(form, {
    origins: ['https://shop.example.com'],
    defaultName: '知识助手 · 网页嵌入',
  });
  assert.deepEqual(payload, {
    name: '知识助手 · 网页嵌入',
    welcome_message: '',
    allowed_origins: ['https://shop.example.com'],
    rate_limit_per_minute: 30,
    rate_limit_per_day: 10000,
    primary_color: WEKNORA_BRAND_COLOR,
    page_title: '',
    header_title_mode: 'channel',
    show_suggested_questions: true,
    widget_position: 'bottom-right',
    allow_web_search: false,
    allow_file_upload: false,
    default_locale: '',
    webhook_url: '',
    enabled: true,
    agent_id: 'agent-1',
  });
});

test('create payload keeps an explicit name and drops the empty webhook secret key', () => {
  const form = { ...createEmbedWizardForm(), agentId: 'a', name: ' 官网客服 ' };
  const payload = buildEmbedCreatePayload(form, { origins: ['*'], defaultName: '网页嵌入' });
  assert.equal(payload.name, '官网客服');
  assert.equal(Object.prototype.hasOwnProperty.call(payload, 'webhook_secret'), false);
  const withSecret = buildEmbedCreatePayload(
    { ...form, defaultLocale: 'en-US', webhookUrl: 'https://hooks.example.com', webhookSecret: 's3cret' },
    { origins: ['*'], defaultName: '网页嵌入' },
  );
  assert.equal(withSecret.webhook_secret, 's3cret');
  assert.equal(withSecret.default_locale, 'en-US');
  assert.equal(withSecret.webhook_url, 'https://hooks.example.com');
});

test('update payload passes enabled through and always carries agent_id (Vue lines 927-928)', () => {
  const form = { ...createEmbedWizardForm(), agentId: 'agent-9', name: '客服' };
  const payload = buildEmbedUpdatePayload(form, {
    origins: ['https://a.example.com'],
    defaultName: '网页嵌入',
    enabled: false,
  });
  assert.deepEqual(payload, {
    name: '客服',
    welcome_message: '',
    allowed_origins: ['https://a.example.com'],
    rate_limit_per_minute: 30,
    rate_limit_per_day: 10000,
    primary_color: WEKNORA_BRAND_COLOR,
    page_title: '',
    header_title_mode: 'channel',
    show_suggested_questions: true,
    widget_position: 'bottom-right',
    allow_web_search: false,
    allow_file_upload: false,
    default_locale: '',
    webhook_url: '',
    enabled: false,
    agent_id: 'agent-9',
  });
});

test('prefill mirrors Vue fillFormFromChannel (lines 813-837) with its fallbacks', () => {
  const form = embedWizardFormFromChannel({
    id: 'ch-1', agent_id: 'agent-1', name: '客服渠道', enabled: false,
    welcome_message: 'hi', rate_limit_per_minute: 0, rate_limit_per_day: undefined,
    primary_color: '', page_title: undefined, header_title_mode: undefined,
    show_suggested_questions: undefined, widget_position: 'top-left',
    allow_web_search: true, allow_file_upload: undefined, default_locale: 'ko-KR',
    webhook_url: 'https://hooks.example.com',
  }, '#123456');
  assert.deepEqual(form, {
    agentId: 'agent-1',
    name: '客服渠道',
    welcomeMessage: 'hi',
    rateLimitPerMinute: 30,
    rateLimitPerDay: 10000,
    primaryColor: '#123456',
    pageTitle: '',
    headerTitleMode: 'channel',
    showSuggestedQuestions: true,
    widgetPosition: 'top-left',
    allowWebSearch: true,
    allowFileUpload: false,
    defaultLocale: 'ko-KR',
    webhookUrl: 'https://hooks.example.com',
    webhookSecret: '',
  });
  assert.equal(embedOriginsTextFromChannel({ allowed_origins: ['https://a.com', 'https://b.com'] }), 'https://a.com\nhttps://b.com');
  assert.equal(embedOriginsTextFromChannel({ allowed_origins: null }), '');
});

test('channel key display masks with bullets until revealed (displayChannelKey lines 734-742)', () => {
  assert.equal(embedChannelKeyDisplay('abc123', false), '••••••');
  assert.equal(embedChannelKeyDisplay('abc123', true), 'abc123');
  assert.equal(embedChannelKeyDisplay(undefined, false), '');
});

test('embed channel URL mirrors Vue buildEmbedURL (locale, refresh key, token hash)', () => {
  assert.equal(
    embedChannelUrl('ch 1', 'tok', { origin: 'https://weknora.test' }),
    'https://weknora.test/embed/ch%201#token=tok',
  );
  assert.equal(
    embedChannelUrl('ch-1', 'tok', { origin: 'https://weknora.test', locale: 'zh-CN', refreshKey: 3 }),
    'https://weknora.test/embed/ch-1?locale=zh-CN&r=3#token=tok',
  );
  assert.equal(embedChannelUrl('ch-1', undefined, { origin: 'https://weknora.test' }), 'https://weknora.test/embed/ch-1');
});

test('iframe and widget snippets mirror the Vue snippet builders', () => {
  const iframe = embedIframeSnippet('ch-1', 'tok', 'https://weknora.test');
  assert.equal(
    iframe,
    '<iframe src="https://weknora.test/embed/ch-1#token=tok" style="width:400px;height:600px;border:none;border-radius:12px" allow="clipboard-write"></iframe>',
  );
  const widget = embedWidgetSnippet('ch-1', 'tok', 'https://weknora.test', { primaryColor: '#07C05F', title: 'Support', position: 'top-left' });
  assert.ok(widget.startsWith('<script src="https://weknora.test/weknora-widget.js"'));
  assert.ok(widget.includes('data-channel="ch-1"'));
  assert.ok(widget.includes('data-token="tok"'));
  assert.ok(widget.includes('data-position="top-left"'));
  assert.ok(widget.includes('data-primary-color="#07C05F"'));
  assert.ok(widget.includes('data-title="Support"'));
  const bare = embedWidgetSnippet('ch-1', 'tok', 'https://weknora.test');
  assert.ok(bare.includes('data-position="bottom-right"'));
  assert.ok(!bare.includes('data-title'));
  // HTML-dangerous values are attribute-escaped like Vue escapeHtmlAttr.
  assert.ok(embedWidgetSnippet('ch-1', 'to"k', 'https://weknora.test').includes('data-token="to&quot;k"'));
});

test('secure snippet points at the integrator token endpoint; server examples embed the exchange URL', () => {
  const secure = embedSecureWidgetSnippet('ch-1', 'https://weknora.test');
  assert.ok(secure.includes('data-token-endpoint="https://your-backend.example.com/weknora/embed-token"'));
  assert.ok(!secure.includes('data-token='));
  const node = embedSecureServerNodeExample('ch-1', 'https://weknora.test');
  assert.ok(node.includes('WEKNORA_PUBLISH_TOKEN'));
  assert.ok(node.includes('https://weknora.test/api/v1/embed/ch-1/exchange'));
  const go = embedSecureServerGoExample('ch-1', 'https://weknora.test');
  assert.ok(go.includes('embedTokenHandler'));
  assert.ok(go.includes('https://weknora.test/api/v1/embed/ch-1/exchange'));
  assert.equal(embedSnippetScenarioKey('iframe'), 'embedPublish.embedIframeDesc');
  assert.equal(embedSnippetScenarioKey('widget'), 'embedPublish.embedWidgetDesc');
  assert.equal(embedSnippetScenarioKey('secure'), 'embedPublish.embedSecureDesc');
});