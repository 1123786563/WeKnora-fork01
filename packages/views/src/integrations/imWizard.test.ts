import assert from 'node:assert/strict';
import test from 'node:test';

// Vue baseline: frontend/src/components/IMChannelPanel.vue (4-step SettingDrawer
// wizard, per-platform credential field tables, WeChat QR binding flow).
import {
  IM_WIZARD_STEPS,
  applyImPlatformChange,
  applyWeChatConfirmedCredentials,
  buildImCreatePayload,
  buildImUpdatePayload,
  createImWizardForm,
  imCallbackUrl,
  imConsoleLink,
  imCredentialFields,
  imPlatformSupportsThread,
  isWeChatBound,
  normalizeYunzhijiaCredentials,
  validateImWizardSave,
  validateImWizardStep,
  wechatQrImageUrl,
} from './imWizard.ts';

const zhDefaults = (platform: string): string => ({
  wecom: '企业微信',
  feishu: '飞书',
  lark: 'Lark（飞书国际版）',
  slack: 'Slack',
  telegram: 'Telegram',
  dingtalk: '钉钉',
  mattermost: 'Mattermost',
  wechat: '微信',
  qqbot: 'QQBot',
  yunzhijia: '云之家',
} as Record<string, string>)[platform] ?? platform;

test('wizard keeps the Vue 4-step structure and order', () => {
  assert.deepEqual(IM_WIZARD_STEPS.map((step) => step.key), ['basic', 'connection', 'knowledge', 'credentials']);
  assert.deepEqual(IM_WIZARD_STEPS.map((step) => step.titleKey), [
    'agentEditor.im.stepBasic',
    'agentEditor.im.stepConnection',
    'agentEditor.im.stepKnowledge',
    'agentEditor.im.stepCredentials',
  ]);
});

test('default form mirrors Vue resetForm (wecom, websocket, stream, user)', () => {
  const form = createImWizardForm();
  assert.equal(form.targetAgentId, '');
  assert.equal(form.platform, 'wecom');
  assert.equal(form.name, '');
  assert.equal(form.mode, 'websocket');
  assert.equal(form.outputMode, 'stream');
  assert.equal(form.sessionMode, 'user');
  assert.equal(form.knowledgeBaseId, '');
  assert.deepEqual(form.credentials, {});
});

test('thread session mode is limited to the Vue platform list', () => {
  assert.equal(imPlatformSupportsThread('slack'), true);
  assert.deepEqual(['mattermost', 'feishu', 'lark', 'telegram', 'yunzhijia'].map(imPlatformSupportsThread), [true, true, true, true, true]);
  assert.deepEqual(['wecom', 'dingtalk', 'wechat', 'qqbot'].map(imPlatformSupportsThread), [false, false, false, false]);
});

test('platform switch applies the Vue default mode/output/credentials', () => {
  const base = createImWizardForm();
  const wechat = applyImPlatformChange(base, 'wechat', { channelNameTouched: false, defaultNameFor: zhDefaults });
  assert.equal(wechat.mode, 'longpoll');
  assert.equal(wechat.outputMode, 'full');
  assert.deepEqual(wechat.credentials, {});

  const mattermost = applyImPlatformChange(base, 'mattermost', { channelNameTouched: false, defaultNameFor: zhDefaults });
  assert.equal(mattermost.mode, 'webhook');
  assert.equal(mattermost.outputMode, 'stream');
  assert.equal(mattermost.credentials.post_to_main, false);

  const yunzhijia = applyImPlatformChange(base, 'yunzhijia', { channelNameTouched: false, defaultNameFor: zhDefaults });
  assert.equal(yunzhijia.mode, 'webhook');
  assert.equal(yunzhijia.credentials.timeout_seconds, 10);
  assert.equal(yunzhijia.credentials.allowed_webhook_host_suffix, 'yunzhijia.com');

  const feishu = applyImPlatformChange(base, 'feishu', { channelNameTouched: false, defaultNameFor: zhDefaults });
  assert.equal(feishu.mode, 'websocket');
  assert.equal(feishu.outputMode, 'stream');
});

test('platform switch renames untouched channel names and keeps touched ones', () => {
  const base = createImWizardForm();
  const renamed = applyImPlatformChange(base, 'feishu', { channelNameTouched: false, defaultNameFor: zhDefaults });
  assert.equal(renamed.name, '飞书');
  const kept = applyImPlatformChange({ ...base, name: '客服机器人' }, 'feishu', { channelNameTouched: true, defaultNameFor: zhDefaults });
  assert.equal(kept.name, '客服机器人');
});

test('platform switch drops unsupported thread session modes', () => {
  const base = { ...createImWizardForm(), sessionMode: 'thread' as const };
  const wecom = applyImPlatformChange(base, 'wecom', { channelNameTouched: true, defaultNameFor: zhDefaults });
  assert.equal(wecom.sessionMode, 'user');
  const slack = applyImPlatformChange(base, 'slack', { channelNameTouched: true, defaultNameFor: zhDefaults });
  assert.equal(slack.sessionMode, 'thread');
});

test('credential fields mirror the Vue wecom tables per mode', () => {
  const ws = imCredentialFields('wecom', 'websocket').map((field) => field.key);
  assert.deepEqual(ws, ['bot_id', 'bot_secret', 'ws_endpoint']);
  assert.equal(imCredentialFields('wecom', 'websocket')[1].type, 'password');
  const hook = imCredentialFields('wecom', 'webhook').map((field) => field.key);
  assert.deepEqual(hook, ['corp_id', 'agent_secret', 'token', 'encoding_aes_key', 'corp_agent_id', 'api_base_url']);
  const numbers = imCredentialFields('wecom', 'webhook').filter((field) => field.type === 'number');
  assert.deepEqual(numbers.map((field) => field.key), ['corp_agent_id']);
});

test('credential fields mirror the Vue feishu/lark and slack tables per mode', () => {
  for (const platform of ['feishu', 'lark'] as const) {
    const ws = imCredentialFields(platform, 'websocket').map((field) => field.key);
    assert.deepEqual(ws, ['app_id', 'app_secret', 'api_base_url'], platform);
    const hook = imCredentialFields(platform, 'webhook').map((field) => field.key);
    assert.deepEqual(hook, ['app_id', 'app_secret', 'api_base_url', 'verification_token', 'encrypt_key'], platform);
  }
  assert.deepEqual(imCredentialFields('slack', 'websocket').map((field) => field.key), ['app_token', 'bot_token']);
  assert.deepEqual(imCredentialFields('slack', 'webhook').map((field) => field.key), ['bot_token', 'signing_secret']);
});

test('credential fields mirror the Vue telegram, dingtalk, qqbot and mattermost tables', () => {
  assert.deepEqual(imCredentialFields('telegram', 'websocket').map((field) => field.key), ['bot_token']);
  assert.deepEqual(imCredentialFields('telegram', 'webhook').map((field) => field.key), ['bot_token', 'secret_token']);
  assert.deepEqual(imCredentialFields('dingtalk', 'websocket').map((field) => field.key), ['client_id', 'client_secret', 'card_template_id']);
  assert.deepEqual(imCredentialFields('qqbot', 'websocket').map((field) => field.key), ['app_id', 'client_secret', 'api_base_url', 'gateway_url']);
  const mattermost = imCredentialFields('mattermost', 'webhook');
  assert.deepEqual(mattermost.map((field) => field.key), ['site_url', 'bot_token', 'outgoing_token', 'bot_user_id', 'post_to_main']);
  assert.equal(mattermost[mattermost.length - 1].type, 'switch');
});

test('credential fields mirror the Vue yunzhijia table with required flags', () => {
  const fields = imCredentialFields('yunzhijia', 'webhook');
  assert.deepEqual(fields.map((field) => field.key), ['send_msg_url', 'secret', 'app_id', 'app_secret', 'timeout_seconds', 'allowed_webhook_host_suffix']);
  const required = fields.filter((field) => field.required).map((field) => field.key);
  assert.deepEqual(required, ['send_msg_url', 'allowed_webhook_host_suffix']);
  const timeout = fields.find((field) => field.key === 'timeout_seconds');
  assert.equal(timeout?.type, 'number');
  assert.equal(timeout?.min, 1);
  assert.equal(timeout?.max, 60);
});

test('wechat has no credential input fields (QR binding flow instead)', () => {
  assert.deepEqual(imCredentialFields('wechat', 'longpoll'), []);
});

test('console links match the Vue open-platform hints', () => {
  assert.deepEqual(imConsoleLink('wecom'), { url: 'https://work.weixin.qq.com/', labelKey: 'agentEditor.im.wecomConsole' });
  assert.deepEqual(imConsoleLink('feishu'), { url: 'https://open.feishu.cn/', labelKey: 'agentEditor.im.feishuConsole' });
  assert.deepEqual(imConsoleLink('lark'), { url: 'https://open.larksuite.com/', labelKey: 'agentEditor.im.larkConsole' });
  assert.equal(imConsoleLink('slack')?.url, 'https://api.slack.com/apps');
  assert.equal(imConsoleLink('telegram')?.url, 'https://t.me/BotFather');
  assert.equal(imConsoleLink('dingtalk')?.url, 'https://open.dingtalk.com/');
  assert.equal(imConsoleLink('qqbot')?.url, 'https://q.qq.com/');
  assert.equal(imConsoleLink('mattermost')?.url, 'https://developers.mattermost.com/integrate/webhooks/outgoing/');
  assert.equal(imConsoleLink('yunzhijia'), undefined);
  assert.equal(imConsoleLink('wechat'), undefined);
});

test('step validation only guards the bound agent like Vue validateWizardStep', () => {
  assert.equal(validateImWizardStep(createImWizardForm(), 0), 'integrations.selectAgentHint');
  assert.equal(validateImWizardStep({ ...createImWizardForm(), targetAgentId: 'agent-1' }, 0), null);
  assert.equal(validateImWizardStep(createImWizardForm(), 1), null);
  assert.equal(validateImWizardStep(createImWizardForm(), 3), null);
});

test('yunzhijia save normalization fills the Vue defaults before validating', () => {
  const form = { ...createImWizardForm(), platform: 'yunzhijia' as const, credentials: { send_msg_url: 'https://www.yunzhijia.com/gateway/robot/webhook/send?yzjtoken=x' } };
  assert.equal(validateImWizardSave(form), null);
  assert.equal(form.credentials.allowed_webhook_host_suffix, 'yunzhijia.com');
  assert.equal(form.credentials.timeout_seconds, 10);
});

test('yunzhijia save fails without a send message url', () => {
  const form = { ...createImWizardForm(), platform: 'yunzhijia' as const, credentials: {} };
  assert.equal(validateImWizardSave(form), 'agentEditor.im.yunzhijiaSendMsgUrlRequired');
});

test('wechat save fails until the QR binding produced a bot token', () => {
  const form = { ...createImWizardForm(), platform: 'wechat' as const, mode: 'longpoll' as const, outputMode: 'full' as const, credentials: {} };
  assert.equal(validateImWizardSave(form), 'agentEditor.im.wechatScanBind');
  const bound = { ...form, credentials: { bot_token: 't', ilink_bot_id: 'b', ilink_user_id: 'u' } };
  assert.equal(validateImWizardSave(bound), null);
});

test('other platforms save without extra credential validation', () => {
  const form = { ...createImWizardForm(), targetAgentId: 'agent-1' };
  assert.equal(validateImWizardSave(form), null);
});

test('create payload mirrors Vue createIMChannel body', () => {
  const form = {
    ...createImWizardForm(),
    targetAgentId: 'agent-1',
    platform: 'feishu' as const,
    name: '',
    credentials: { app_id: 'cli_x', app_secret: 's' },
  };
  assert.deepEqual(buildImCreatePayload(form, '飞书'), {
    platform: 'feishu',
    name: '飞书',
    mode: 'websocket',
    output_mode: 'stream',
    session_mode: 'user',
    knowledge_base_id: '',
    credentials: { app_id: 'cli_x', app_secret: 's' },
  });
});

test('create payload keeps explicit names and knowledge base ids', () => {
  const form = { ...createImWizardForm(), name: '客服', knowledgeBaseId: 'kb-9', platform: 'wecom' as const };
  const payload = buildImCreatePayload(form, '企业微信');
  assert.equal(payload.name, '客服');
  assert.equal(payload.knowledge_base_id, 'kb-9');
});

test('update payload mirrors Vue updateIMChannel body with enabled and agent passthrough', () => {
  const form = {
    ...createImWizardForm(),
    targetAgentId: 'agent-2',
    platform: 'slack' as const,
    name: 'Ops',
    mode: 'webhook' as const,
    outputMode: 'full' as const,
    sessionMode: 'thread' as const,
    knowledgeBaseId: 'kb-1',
    credentials: { bot_token: 'xoxb-' },
  };
  assert.deepEqual(buildImUpdatePayload(form, false, 'Slack'), {
    name: 'Ops',
    mode: 'webhook',
    output_mode: 'full',
    session_mode: 'thread',
    knowledge_base_id: 'kb-1',
    credentials: { bot_token: 'xoxb-' },
    enabled: false,
    agent_id: 'agent-2',
  });
  const noAgent = buildImUpdatePayload({ ...form, targetAgentId: '' }, true, 'Slack');
  assert.equal(noAgent.agent_id, undefined);
  assert.equal('platform' in noAgent, false);
});

test('callback url matches the Vue /api/v1/im/callback shape', () => {
  assert.equal(imCallbackUrl('ch-1', 'https://demo.example.com'), 'https://demo.example.com/api/v1/im/callback/ch-1');
});

test('wechat QR image url uses the same external generator as Vue', () => {
  const target = 'https://work.weixin.qq.com/ca/cawc';
  assert.equal(wechatQrImageUrl(target), 'https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=' + encodeURIComponent(target));
});

test('wechat bound detection matches the Vue bot_token + ilink_bot_id check', () => {
  assert.equal(isWeChatBound({}), false);
  assert.equal(isWeChatBound({ bot_token: 't' }), false);
  assert.equal(isWeChatBound({ bot_token: 't', ilink_bot_id: 'b' }), true);
});

test('confirmed QR polling payload maps onto the Vue credential triple', () => {
  assert.deepEqual(
    applyWeChatConfirmedCredentials({ status: 'confirmed', bot_token: 't', ilink_bot_id: 'b', ilink_user_id: 'u' }),
    { bot_token: 't', ilink_bot_id: 'b', ilink_user_id: 'u' },
  );
});
