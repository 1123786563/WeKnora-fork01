// Port of the IM channel wizard from frontend/src/components/IMChannelPanel.vue
// (SettingDrawer, lines 72-579 + script lines 583-1106): a 4-step drawer
// (basic -> connection -> file knowledge base -> credentials) with per-platform
// credential field tables, mode/output/session defaults per platform, WeChat QR
// code binding, and the create/update payloads of handleSave() (lines 1017-1073).
// Pure logic only — the React renderer lives in page.tsx.

export type ImPlatform =
  | 'wecom' | 'feishu' | 'lark' | 'slack' | 'telegram'
  | 'dingtalk' | 'mattermost' | 'wechat' | 'qqbot' | 'yunzhijia';

export type ImConnectionMode = 'websocket' | 'webhook' | 'longpoll';
export type ImOutputMode = 'stream' | 'full';
export type ImSessionMode = 'user' | 'thread';

export interface ImWizardForm {
  targetAgentId: string;
  platform: ImPlatform;
  name: string;
  mode: ImConnectionMode;
  outputMode: ImOutputMode;
  sessionMode: ImSessionMode;
  knowledgeBaseId: string;
  credentials: Record<string, unknown>;
}

/** Vue stepTitles (IMChannelPanel.vue lines 651-656): order is part of parity. */
export interface ImWizardStep { key: 'basic' | 'connection' | 'knowledge' | 'credentials'; titleKey: string }

export const IM_WIZARD_STEPS: readonly ImWizardStep[] = [
  { key: 'basic', titleKey: 'agentEditor.im.stepBasic' },
  { key: 'connection', titleKey: 'agentEditor.im.stepConnection' },
  { key: 'knowledge', titleKey: 'agentEditor.im.stepKnowledge' },
  { key: 'credentials', titleKey: 'agentEditor.im.stepCredentials' },
] as const;

/** Vue resetForm defaults (lines 995-1015). */
export function createImWizardForm(): ImWizardForm {
  return {
    targetAgentId: '',
    platform: 'wecom',
    name: '',
    mode: 'websocket',
    outputMode: 'stream',
    sessionMode: 'user',
    knowledgeBaseId: '',
    credentials: {},
  };
}

/** Vue platformSupportsThread (lines 784-786). */
const THREAD_PLATFORMS: readonly string[] = ['slack', 'mattermost', 'feishu', 'lark', 'telegram', 'yunzhijia'];

export function imPlatformSupportsThread(platform: string): boolean {
  return THREAD_PLATFORMS.includes(platform);
}

export interface ImCredentialField {
  key: string;
  /** Verbatim Vue label; either literal text or an i18n key (labelKey). */
  label?: string;
  labelKey?: string;
  type: 'text' | 'password' | 'number' | 'switch';
  required?: boolean;
  placeholder?: string;
  placeholderKey?: string;
  hintKey?: string;
  /** Optional doc/console link rendered inside the hint line (yunzhijia). */
  hintLink?: { url: string; labelKey: string };
  min?: number;
  max?: number;
}

const field = (key: string, label: string, extra: Partial<ImCredentialField> = {}): ImCredentialField => ({ key, label, type: 'text', ...extra });
const secret = (key: string, label: string, extra: Partial<ImCredentialField> = {}): ImCredentialField => ({ key, label, type: 'password', ...extra });

// Vue credentials template (IMChannelPanel.vue lines 246-531), flattened into a
// per-platform/per-mode field table. Order matches the Vue DOM order exactly.
const WECom_WS: readonly ImCredentialField[] = [
  field('bot_id', 'Bot ID'),
  secret('bot_secret', 'Bot Secret'),
  field('ws_endpoint', 'WebSocket Endpoint', { placeholder: 'wss://openws.work.weixin.qq.com', hintKey: 'agentEditor.im.wecomWSEndpointHint' }),
];
const WECOM_WEBHOOK: readonly ImCredentialField[] = [
  field('corp_id', 'Corp ID'),
  secret('agent_secret', 'Agent Secret'),
  field('token', 'Token'),
  field('encoding_aes_key', 'EncodingAESKey'),
  field('corp_agent_id', 'Corp Agent ID', { type: 'number' }),
  field('api_base_url', 'API Base URL', { placeholder: 'https://qyapi.weixin.qq.com', hintKey: 'agentEditor.im.wecomAPIBaseURLHint' }),
];
const FEISHU_BASE: readonly ImCredentialField[] = [
  field('app_id', 'App ID'),
  secret('app_secret', 'App Secret'),
  field('api_base_url', 'Base URL', { placeholder: 'https://open.feishu.cn', hintKey: 'agentEditor.im.feishuAPIBaseURLHint' }),
];
const FEISHU_WEBHOOK: readonly ImCredentialField[] = [
  field('verification_token', 'Verification Token'),
  secret('encrypt_key', 'Encrypt Key'),
];
const YUNZHIJIA: readonly ImCredentialField[] = [
  field('send_msg_url', undefined as unknown as string, {
    required: true, labelKey: 'agentEditor.im.yunzhijiaSendMsgUrl',
    placeholder: 'https://www.yunzhijia.com/gateway/robot/webhook/send?yzjtype=0&yzjtoken=...',
    hintKey: 'agentEditor.im.yunzhijiaSendMsgUrlHint', hintLink: { url: 'https://www.yunzhijia.com/opendocs/docs.html#/guide/im/robot', labelKey: 'agentEditor.im.yunzhijiaRobotDoc' },
  }),
  secret('secret', undefined as unknown as string, { labelKey: 'agentEditor.im.yunzhijiaSecret', placeholderKey: 'agentEditor.im.yunzhijiaSecretPlaceholder', hintKey: 'agentEditor.im.yunzhijiaSecretHint' }),
  field('app_id', undefined as unknown as string, { labelKey: 'agentEditor.im.yunzhijiaAppId', placeholderKey: 'agentEditor.im.yunzhijiaAppIdPlaceholder', hintKey: 'agentEditor.im.yunzhijiaAppCredentialHint', hintLink: { url: 'https://www.yunzhijia.com/developers/', labelKey: 'agentEditor.im.yunzhijiaImageDoc' } }),
  secret('app_secret', undefined as unknown as string, { labelKey: 'agentEditor.im.yunzhijiaAppSecret', placeholderKey: 'agentEditor.im.yunzhijiaAppSecretPlaceholder' }),
  field('timeout_seconds', undefined as unknown as string, { type: 'number', labelKey: 'agentEditor.im.yunzhijiaTimeout', placeholder: '10', min: 1, max: 60, hintKey: 'agentEditor.im.yunzhijiaTimeoutHint' }),
  field('allowed_webhook_host_suffix', undefined as unknown as string, { required: true, labelKey: 'agentEditor.im.yunzhijiaAllowedHostSuffix', placeholder: 'yunzhijia.com', hintKey: 'agentEditor.im.yunzhijiaAllowedHostSuffixHint' }),
];
const MATTERMOST: readonly ImCredentialField[] = [
  field('site_url', 'Site URL', { placeholder: 'https://mattermost.example.com' }),
  secret('bot_token', 'Bot Token'),
  secret('outgoing_token', 'Outgoing Webhook Token', { placeholder: 'Token from Outgoing Webhook' }),
  field('bot_user_id', 'Bot User ID', { placeholder: 'Optional — filter bot self-messages' }),
  { key: 'post_to_main', type: 'switch', labelKey: 'agentEditor.im.mattermostPostToMain', hintKey: 'agentEditor.im.mattermostPostToMainHint' },
];

function wecomFields(mode: ImConnectionMode): readonly ImCredentialField[] {
  return mode === 'websocket' ? WECom_WS : WECOM_WEBHOOK;
}

function feishuFields(mode: ImConnectionMode): readonly ImCredentialField[] {
  return mode === 'webhook' ? [...FEISHU_BASE, ...FEISHU_WEBHOOK] : FEISHU_BASE;
}

function slackFields(mode: ImConnectionMode): readonly ImCredentialField[] {
  return mode === 'websocket'
    ? [secret('app_token', 'App Token', { placeholder: 'xapp-...' }), secret('bot_token', 'Bot Token', { placeholder: 'xoxb-...' })]
    : [secret('bot_token', 'Bot Token', { placeholder: 'xoxb-...' }), secret('signing_secret', 'Signing Secret')];
}

function telegramFields(mode: ImConnectionMode): readonly ImCredentialField[] {
  const base = [secret('bot_token', 'Bot Token', { placeholder: '123456789:AABBccdd...' })];
  return mode === 'webhook' ? [...base, secret('secret_token', 'Secret Token', { placeholder: 'Secret Token (optional)' })] : base;
}

const DINGTALK: readonly ImCredentialField[] = [
  field('client_id', 'Client ID (AppKey)', { placeholder: 'Client ID / AppKey' }),
  secret('client_secret', 'Client Secret (AppSecret)', { placeholder: 'Client Secret / AppSecret' }),
  field('card_template_id', undefined as unknown as string, { labelKey: 'agentEditor.im.dingtalkCardTemplateId', placeholder: 'xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx.schema', hintKey: 'agentEditor.im.dingtalkCardTemplateIdHint' }),
];

const QQBOT: readonly ImCredentialField[] = [
  field('app_id', 'App ID', { placeholder: 'QQBot App ID' }),
  secret('client_secret', 'App Secret', { placeholder: 'QQBot App Secret' }),
  field('api_base_url', 'API Base URL', { placeholder: 'https://api.sgroup.qq.com', hintKey: 'agentEditor.im.qqbotAPIBaseURLHint' }),
  field('gateway_url', 'Gateway URL', { placeholder: 'wss://api.sgroup.qq.com/websocket/', hintKey: 'agentEditor.im.qqbotGatewayURLHint' }),
];

/** Credential inputs for one platform+mode; wechat returns [] (QR binding instead). */
export function imCredentialFields(platform: ImPlatform | string, mode: ImConnectionMode): readonly ImCredentialField[] {
  switch (platform) {
    case 'wecom': return wecomFields(mode);
    case 'feishu':
    case 'lark': return feishuFields(mode);
    case 'slack': return slackFields(mode);
    case 'telegram': return telegramFields(mode);
    case 'dingtalk': return DINGTALK;
    case 'qqbot': return QQBOT;
    case 'mattermost': return MATTERMOST;
    case 'yunzhijia': return YUNZHIJIA;
    case 'wechat': return [];
    default: return [];
  }
}

export interface ImConsoleLink { url: string; labelKey: string }

/** Vue console hint rows (lines 249-450) + openPlatformConsole (lines 679-683). */
export function imConsoleLink(platform: ImPlatform | string): ImConsoleLink | undefined {
  switch (platform) {
    case 'wecom': return { url: 'https://work.weixin.qq.com/', labelKey: 'agentEditor.im.wecomConsole' };
    case 'feishu': return { url: 'https://open.feishu.cn/', labelKey: 'agentEditor.im.feishuConsole' };
    case 'lark': return { url: 'https://open.larksuite.com/', labelKey: 'agentEditor.im.larkConsole' };
    case 'slack': return { url: 'https://api.slack.com/apps', labelKey: 'agentEditor.im.slackConsole' };
    case 'telegram': return { url: 'https://t.me/BotFather', labelKey: 'agentEditor.im.telegramConsole' };
    case 'dingtalk': return { url: 'https://open.dingtalk.com/', labelKey: 'agentEditor.im.dingtalkConsole' };
    case 'qqbot': return { url: 'https://q.qq.com/', labelKey: 'agentEditor.im.qqbotConsole' };
    case 'mattermost': return { url: 'https://developers.mattermost.com/integrate/webhooks/outgoing/', labelKey: 'agentEditor.im.mattermostConsole' };
    default: return undefined;
  }
}

export interface ImPlatformChangeOptions {
  channelNameTouched: boolean;
  /** Vue defaultChannelName(platform): the localized platform label. */
  defaultNameFor: (platform: string) => string;
}

/** Vue onPlatformChange (lines 810-838) + platform watch (lines 788-801). */
export function applyImPlatformChange(form: ImWizardForm, nextPlatform: ImPlatform, options: ImPlatformChangeOptions): ImWizardForm {
  const next: ImWizardForm = { ...form, platform: nextPlatform, credentials: {} };
  if (nextPlatform === 'wechat') {
    next.mode = 'longpoll';
    next.outputMode = 'full';
  } else if (nextPlatform === 'mattermost' || nextPlatform === 'yunzhijia') {
    next.mode = 'webhook';
    next.outputMode = 'stream';
    if (nextPlatform === 'yunzhijia') {
      next.credentials = { timeout_seconds: 10, allowed_webhook_host_suffix: 'yunzhijia.com' };
    } else {
      next.credentials = { post_to_main: false };
    }
  } else {
    next.mode = 'websocket';
    next.outputMode = 'stream';
  }
  if (!imPlatformSupportsThread(nextPlatform)) {
    next.sessionMode = 'user';
  }
  if (!options.channelNameTouched) {
    next.name = options.defaultNameFor(nextPlatform);
  }
  return next;
}

/** Vue normalizeYunzhijiaCredentials (lines 840-848); mutates form.credentials in place. */
export function normalizeYunzhijiaCredentials(form: ImWizardForm): void {
  if (form.platform !== 'yunzhijia') return;
  if (!form.credentials.allowed_webhook_host_suffix) {
    form.credentials.allowed_webhook_host_suffix = 'yunzhijia.com';
  }
  if (!form.credentials.timeout_seconds) {
    form.credentials.timeout_seconds = 10;
  }
}

/** Vue validateWizardStep (lines 692-698): only the bound agent is a step gate. */
export function validateImWizardStep(form: ImWizardForm, step: number): string | null {
  if (step === 0 && !form.targetAgentId) return 'integrations.selectAgentHint';
  return null;
}

/** Vue handleSave guards (lines 1017-1033). Returns a warning message key, or null. */
export function validateImWizardSave(form: ImWizardForm): string | null {
  if (form.platform === 'wechat' && !form.credentials.bot_token) {
    return 'agentEditor.im.wechatScanBind';
  }
  if (form.platform === 'yunzhijia') {
    normalizeYunzhijiaCredentials(form);
    if (!String(form.credentials.send_msg_url || '').trim()) {
      return 'agentEditor.im.yunzhijiaSendMsgUrlRequired';
    }
  }
  return null;
}

const trimCredentials = (credentials: Record<string, unknown>): Record<string, unknown> => {
  const output: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(credentials)) output[key] = value;
  return output;
};

/** Vue createIMChannel body (lines 1053-1061); name falls back to the platform label. */
export function buildImCreatePayload(form: ImWizardForm, defaultName: string): Record<string, unknown> {
  return {
    platform: form.platform,
    name: form.name.trim() || defaultName,
    mode: form.mode,
    output_mode: form.outputMode,
    session_mode: form.sessionMode,
    knowledge_base_id: form.knowledgeBaseId,
    credentials: trimCredentials(form.credentials),
  };
}

/** Vue updateIMChannel body (lines 1036-1045); agent_id only when present. */
export function buildImUpdatePayload(form: ImWizardForm, enabled: boolean, defaultName: string): Record<string, unknown> {
  return {
    name: form.name.trim() || defaultName,
    mode: form.mode,
    output_mode: form.outputMode,
    session_mode: form.sessionMode,
    knowledge_base_id: form.knowledgeBaseId,
    credentials: trimCredentials(form.credentials),
    enabled,
    ...(form.targetAgentId ? { agent_id: form.targetAgentId } : {}),
  };
}

/** Vue getCallbackUrl (lines 940-943). */
export function imCallbackUrl(channelId: string, origin: string): string {
  return origin + '/api/v1/im/callback/' + channelId;
}

// --- WeChat QR binding (Vue lines 533-575, 716-723, 802-807, 850-919) ---

/** Vue startWeChatBinding line 865: image generated by the public qrserver API. */
export function wechatQrImageUrl(qrcodeUrl: string): string {
  return 'https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=' + encodeURIComponent(qrcodeUrl);
}

/** Vue wechatBound computed (lines 802-807). */
export function isWeChatBound(credentials: Record<string, unknown>): boolean {
  return Boolean(credentials.bot_token) && Boolean(credentials.ilink_bot_id);
}

export interface WeChatQrStatusPayload {
  status: 'wait' | 'scaned' | 'confirmed' | 'expired' | string;
  bot_token?: string;
  ilink_bot_id?: string;
  ilink_user_id?: string;
}

/** Vue pollOnce confirmed branch (lines 888-893). */
export function applyWeChatConfirmedCredentials(payload: WeChatQrStatusPayload): Record<string, unknown> {
  return { bot_token: payload.bot_token, ilink_bot_id: payload.ilink_bot_id, ilink_user_id: payload.ilink_user_id };
}

/** Prefill from an existing channel, mirroring Vue editChannel (lines 961-993). */
export function imWizardFormFromChannel(channel: {
  agent_id?: unknown; platform?: unknown; name?: unknown; mode?: unknown; output_mode?: unknown;
  session_mode?: unknown; knowledge_base_id?: unknown; credentials?: unknown;
}): ImWizardForm {
  const form = createImWizardForm();
  form.targetAgentId = typeof channel.agent_id === 'string' ? channel.agent_id : '';
  if (typeof channel.platform === 'string' && channel.platform) form.platform = channel.platform as ImPlatform;
  form.name = typeof channel.name === 'string' ? channel.name : '';
  if (channel.mode === 'websocket' || channel.mode === 'webhook' || channel.mode === 'longpoll') form.mode = channel.mode;
  if (channel.output_mode === 'stream' || channel.output_mode === 'full') form.outputMode = channel.output_mode;
  if (channel.session_mode === 'user' || channel.session_mode === 'thread') form.sessionMode = channel.session_mode;
  form.knowledgeBaseId = typeof channel.knowledge_base_id === 'string' ? channel.knowledge_base_id : '';
  form.credentials = channel.credentials && typeof channel.credentials === 'object' && !Array.isArray(channel.credentials)
    ? { ...(channel.credentials as Record<string, unknown>) }
    : {};
  return form;
}