// Port of the embed channel wizard from frontend/src/components/AgentEmbedChannelPanel.vue
// (SettingDrawer lines 70-386 + script lines 439-1087): a 5-step drawer
// (channel -> security -> capabilities -> appearance -> webhook) with an
// edit-only deploy step, the origins allowlist rules of
// frontend/src/utils/embedAllowedOrigins.ts, the create/update payload of
// saveForm() (lines 896-982) and the API origin-error mapping (lines 882-894).
// The deploy-step snippet builders mirror frontend/src/api/embed/index.ts with
// the origin passed explicitly (the React shell has no bundler base-URL
// injection). Pure logic only — the React renderer lives in page.tsx.

export type EmbedHeaderTitleMode = 'channel' | 'session';
export type EmbedWidgetPosition = 'bottom-right' | 'bottom-left' | 'top-right' | 'top-left';
export type EmbedLocaleTag = '' | 'zh-CN' | 'en-US' | 'ko-KR' | 'ja-JP' | 'ru-RU';

export interface EmbedWizardForm {
  /** Vue createAgentId: the bound agent select, editable while editing too. */
  agentId: string;
  name: string;
  welcomeMessage: string;
  rateLimitPerMinute: number;
  rateLimitPerDay: number;
  primaryColor: string;
  pageTitle: string;
  headerTitleMode: EmbedHeaderTitleMode;
  showSuggestedQuestions: boolean;
  widgetPosition: EmbedWidgetPosition;
  allowWebSearch: boolean;
  allowFileUpload: boolean;
  defaultLocale: EmbedLocaleTag;
  webhookUrl: string;
  webhookSecret: string;
}

export interface EmbedWizardStep {
  key: 'channel' | 'security' | 'capabilities' | 'appearance' | 'webhook' | 'deploy';
  titleKey: string;
}

const CREATE_STEPS: readonly EmbedWizardStep[] = [
  { key: 'channel', titleKey: 'embedPublish.stepChannel' },
  { key: 'security', titleKey: 'embedPublish.stepSecurity' },
  { key: 'capabilities', titleKey: 'embedPublish.stepCapabilities' },
  { key: 'appearance', titleKey: 'embedPublish.stepAppearance' },
  { key: 'webhook', titleKey: 'embedPublish.stepWebhook' },
];

export const EMBED_WIZARD_STEPS: readonly EmbedWizardStep[] = CREATE_STEPS;

/** Vue stepTitles computed (lines 601-611): the deploy step exists only while editing. */
export function embedWizardSteps(editing: boolean): readonly EmbedWizardStep[] {
  return editing
    ? [...CREATE_STEPS, { key: 'deploy', titleKey: 'embedPublish.stepDeploy' } as EmbedWizardStep]
    : CREATE_STEPS;
}

/** Vue WEKNORA_BRAND_COLOR (line 492); the CSS-var fallback of getDefaultEmbedPrimaryColor. */
export const WEKNORA_BRAND_COLOR = '#07C05F';

/** Vue defaultForm (lines 502-517). */
export function createEmbedWizardForm(defaultPrimaryColor: string = WEKNORA_BRAND_COLOR): EmbedWizardForm {
  return {
    agentId: '',
    name: '',
    welcomeMessage: '',
    rateLimitPerMinute: 30,
    rateLimitPerDay: 10000,
    primaryColor: defaultPrimaryColor,
    pageTitle: '',
    headerTitleMode: 'channel',
    showSuggestedQuestions: true,
    widgetPosition: 'bottom-right',
    allowWebSearch: false,
    allowFileUpload: false,
    defaultLocale: '',
    webhookUrl: '',
    webhookSecret: '',
  };
}

const WIDGET_POSITIONS: readonly string[] = ['bottom-right', 'bottom-left', 'top-right', 'top-left'];

function isWidgetPosition(value: unknown): value is EmbedWidgetPosition {
  return typeof value === 'string' && WIDGET_POSITIONS.includes(value);
}

const LOCALE_TAGS: readonly string[] = ['zh-CN', 'en-US', 'ko-KR', 'ja-JP', 'ru-RU'];

function isEmbedLocaleTag(value: unknown): value is EmbedLocaleTag {
  return typeof value === 'string' && (value === '' || LOCALE_TAGS.includes(value));
}

// --- Allowed origins (frontend/src/utils/embedAllowedOrigins.ts) ---

export type EmbedOriginsValidationError =
  | { code: 'required' }
  | { code: 'wildcard_prod' }
  | { code: 'invalid'; origin: string };

/** Vue parseAllowedOrigins; mirrors backend validateAllowedOrigins in internal/handler/embed_channel.go. */
export function parseEmbedAllowedOrigins(text: string): string[] {
  return text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

/** Vue validateAllowedOrigins; prod is passed explicitly (no bundler env in the views package). */
export function validateEmbedAllowedOrigins(
  origins: string[],
  prod = false,
): { ok: true; origins: string[] } | { ok: false; error: EmbedOriginsValidationError } {
  const cleaned = parseEmbedAllowedOrigins(origins.join('\n'));
  if (cleaned.length === 0) {
    return { ok: false, error: { code: 'required' } };
  }
  for (const origin of cleaned) {
    if (origin === '*') {
      if (prod) {
        return { ok: false, error: { code: 'wildcard_prod' } };
      }
      continue;
    }
    let host = origin;
    if (origin.startsWith('*.')) {
      host = 'https://' + origin.slice(2);
    }
    try {
      const url = new URL(host);
      if ((url.protocol !== 'http:' && url.protocol !== 'https:') || !url.host) {
        return { ok: false, error: { code: 'invalid', origin } };
      }
    } catch {
      return { ok: false, error: { code: 'invalid', origin } };
    }
  }
  return { ok: true, origins: cleaned };
}

/** Structured copy warning; page.tsx resolves it through the layered translator. */
export interface EmbedWizardWarning {
  key: string;
  values?: Record<string, string | number>;
}

/** Vue originsValidationMessage (lines 876-880). */
export function embedOriginsWarning(error: EmbedOriginsValidationError): EmbedWizardWarning {
  if (error.code === 'required') return { key: 'embedPublish.originsRequired' };
  if (error.code === 'wildcard_prod') return { key: 'embedPublish.originsWildcardProd' };
  return { key: 'embedPublish.originsInvalid', values: { origin: error.origin } };
}

/** Vue validateWizardStep (lines 619-634): the agent and origins step gates. */
export function validateEmbedWizardStep(
  form: Pick<EmbedWizardForm, 'agentId'>,
  originsText: string,
  step: number,
  prod = false,
): EmbedWizardWarning | null {
  if (step === 0 && !form.agentId) {
    return { key: 'integrations.selectAgentHint' };
  }
  if (step === 1) {
    const validation = validateEmbedAllowedOrigins(parseEmbedAllowedOrigins(originsText), prod);
    if (!validation.ok) {
      return embedOriginsWarning(validation.error);
    }
  }
  return null;
}

/** Vue mapOriginsApiError (lines 882-894): backend origin errors -> copy warnings. */
export function mapEmbedOriginsApiError(message: string): EmbedWizardWarning | null {
  if (message === 'at least one allowed origin is required') {
    return { key: 'embedPublish.originsRequired' };
  }
  if (message === "wildcard origin '*' is not allowed in production") {
    return { key: 'embedPublish.originsWildcardProd' };
  }
  const invalidMatch = message.match(/^invalid allowed origin: "(.+)"$/);
  if (invalidMatch) {
    return { key: 'embedPublish.originsInvalid', values: { origin: invalidMatch[1] } };
  }
  return null;
}

// --- Form <-> channel mapping ---

/** Vue fillFormFromChannel (lines 813-837) with its falsy fallbacks. */
export function embedWizardFormFromChannel(
  channel: Record<string, unknown>,
  defaultPrimaryColor: string = WEKNORA_BRAND_COLOR,
): EmbedWizardForm {
  const form = createEmbedWizardForm(defaultPrimaryColor);
  form.agentId = typeof channel.agent_id === 'string' ? channel.agent_id : '';
  form.name = typeof channel.name === 'string' ? channel.name : '';
  form.welcomeMessage = typeof channel.welcome_message === 'string' ? channel.welcome_message : '';
  const perMinute = channel.rate_limit_per_minute;
  form.rateLimitPerMinute = typeof perMinute === 'number' && perMinute ? perMinute : 30;
  const perDay = channel.rate_limit_per_day;
  form.rateLimitPerDay = typeof perDay === 'number' && perDay ? perDay : 10000;
  form.primaryColor = typeof channel.primary_color === 'string' && channel.primary_color
    ? channel.primary_color
    : defaultPrimaryColor;
  form.pageTitle = typeof channel.page_title === 'string' ? channel.page_title : '';
  form.headerTitleMode = channel.header_title_mode === 'session' ? 'session' : 'channel';
  form.showSuggestedQuestions = channel.show_suggested_questions !== false;
  form.widgetPosition = isWidgetPosition(channel.widget_position) ? channel.widget_position : 'bottom-right';
  form.allowWebSearch = channel.allow_web_search === true;
  form.allowFileUpload = channel.allow_file_upload === true;
  form.defaultLocale = isEmbedLocaleTag(channel.default_locale) ? channel.default_locale : '';
  form.webhookUrl = typeof channel.webhook_url === 'string' ? channel.webhook_url : '';
  form.webhookSecret = '';
  return form;
}

/** Vue fillFormFromChannel line 833: the origins textarea joins the array with newlines. */
export function embedOriginsTextFromChannel(channel: { allowed_origins?: unknown }): string {
  return Array.isArray(channel.allowed_origins)
    ? channel.allowed_origins.filter((value): value is string => typeof value === 'string').join('\n')
    : '';
}

// --- Save payload (Vue saveForm lines 896-982) ---

export interface EmbedWizardPayloadInput {
  /** Origins from the security step, already validated. */
  origins: string[];
  /** Vue resolvedEmbedChannelName(): trimmed form name or the localized default. */
  defaultName: string;
  /** Vue line 927: enabled keeps editingEnabled on update and is always true on create. */
  enabled: boolean;
}

/** Vue saveForm payload (lines 911-929), shared by create and update. */
export function buildEmbedWizardPayload(form: EmbedWizardForm, input: EmbedWizardPayloadInput): Record<string, unknown> {
  return {
    name: form.name.trim() || input.defaultName,
    welcome_message: form.welcomeMessage,
    allowed_origins: input.origins,
    rate_limit_per_minute: form.rateLimitPerMinute,
    rate_limit_per_day: form.rateLimitPerDay,
    primary_color: form.primaryColor,
    page_title: form.pageTitle,
    header_title_mode: form.headerTitleMode,
    show_suggested_questions: form.showSuggestedQuestions,
    widget_position: form.widgetPosition,
    allow_web_search: form.allowWebSearch,
    allow_file_upload: form.allowFileUpload,
    default_locale: form.defaultLocale || '',
    webhook_url: form.webhookUrl || '',
    ...(form.webhookSecret ? { webhook_secret: form.webhookSecret } : {}),
    enabled: input.enabled,
    agent_id: form.agentId,
  };
}

/** Create always enables the channel (Vue line 927) and always posts agent_id. */
export function buildEmbedCreatePayload(
  form: EmbedWizardForm,
  input: Omit<EmbedWizardPayloadInput, 'enabled'>,
): Record<string, unknown> {
  return buildEmbedWizardPayload(form, { ...input, enabled: true });
}

/** Update keeps the editingEnabled toggle of Vue line 927. */
export function buildEmbedUpdatePayload(
  form: EmbedWizardForm,
  input: Omit<EmbedWizardPayloadInput, 'enabled'> & { enabled: boolean },
): Record<string, unknown> {
  return buildEmbedWizardPayload(form, input);
}

// --- Channel key (Vue displayChannelKey / tokenFor, lines 732-742) ---

export function embedChannelKeyDisplay(token: string | undefined, revealed: boolean): string {
  if (!token) return '';
  if (revealed) return token;
  return '•'.repeat(token.length);
}

// --- Deploy-step snippets (frontend/src/api/embed/index.ts) ---

/** Vue escapeHtmlAttr (api/embed lines 580-586). */
function escapeHtmlAttr(value: string): string {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

/** Vue buildEmbedURL (lines 563-577) with the origin passed in. */
export function embedChannelUrl(
  channelId: string,
  token?: string,
  opts?: { locale?: string; refreshKey?: number; origin?: string },
): string {
  const base = opts?.origin || '';
  let path = base + '/embed/' + encodeURIComponent(channelId);
  const params = new URLSearchParams();
  if (opts?.locale?.trim()) params.set('locale', opts.locale.trim());
  if (opts?.refreshKey) params.set('r', String(opts.refreshKey));
  const qs = params.toString();
  if (qs) path += '?' + qs;
  if (token) path += '#token=' + encodeURIComponent(token);
  return path;
}

/** Vue buildEmbedSnippet (lines 601-606). */
export function embedIframeSnippet(channelId: string, token: string, origin: string): string {
  const url = escapeHtmlAttr(embedChannelUrl(channelId, token, { origin }));
  return '<iframe src="' + url + '" style="width:400px;height:600px;border:none;border-radius:12px" allow="clipboard-write"></iframe>';
}

function widgetScriptSnippet(
  channelId: string,
  origin: string,
  tokenOrEndpoint: { key: 'data-token' | 'data-token-endpoint'; value: string },
  opts?: { primaryColor?: string; title?: string; position?: string },
): string {
  const position = opts?.position || 'bottom-right';
  const attrs = [
    'src="' + escapeHtmlAttr(origin + '/weknora-widget.js') + '"',
    'data-channel="' + escapeHtmlAttr(channelId) + '"',
    tokenOrEndpoint.key + '="' + escapeHtmlAttr(tokenOrEndpoint.value) + '"',
    'data-position="' + escapeHtmlAttr(position) + '"',
  ];
  if (opts?.primaryColor) attrs.push('data-primary-color="' + escapeHtmlAttr(opts.primaryColor) + '"');
  if (opts?.title) attrs.push('data-title="' + escapeHtmlAttr(opts.title) + '"');
  return '<script ' + attrs.join('\n        ') + '></script>';
}

/** Vue buildWidgetSnippet (lines 608-625). */
export function embedWidgetSnippet(
  channelId: string,
  token: string,
  origin: string,
  opts?: { primaryColor?: string; title?: string; position?: string },
): string {
  return widgetScriptSnippet(channelId, origin, { key: 'data-token', value: token }, opts);
}

/** Default placeholder for the integrator's own token-minting endpoint (api/embed line 628). */
export const SECURE_TOKEN_ENDPOINT_PLACEHOLDER = 'https://your-backend.example.com/weknora/embed-token';

/** Vue buildSecureWidgetSnippet (lines 635-651): the publish token never reaches the page. */
export function embedSecureWidgetSnippet(
  channelId: string,
  origin: string,
  opts?: { primaryColor?: string; title?: string; position?: string; tokenEndpoint?: string },
): string {
  return widgetScriptSnippet(
    channelId,
    origin,
    { key: 'data-token-endpoint', value: opts?.tokenEndpoint || SECURE_TOKEN_ENDPOINT_PLACEHOLDER },
    opts,
  );
}

/** Vue buildSecureServerNodeExample (lines 658-682). */
export function embedSecureServerNodeExample(channelId: string, origin: string): string {
  const exchangeUrl = origin + '/api/v1/embed/' + channelId + '/exchange';
  return [
    '// Node/Express — keep WEKNORA_PUBLISH_TOKEN only on the server (env var).',
    "app.get('/weknora/embed-token', async (req, res) => {",
    '  // Only mint for logged-in visitors — e.g. session cookie or Bearer token.',
    "  const auth = req.headers.authorization || ''",
    '  const hasSession = Boolean(req.cookies?.session_id)',
    "  if (!hasSession && !auth.startsWith('Bearer ')) {",
    "    return res.status(401).json({ error: 'unauthorized' })",
    '  }',
    "  const r = await fetch('" + exchangeUrl + "', {",
    "    method: 'POST',",
    '    headers: {',
    "      Authorization: 'Embed ' + process.env.WEKNORA_PUBLISH_TOKEN,",
    "      Origin: 'https://your-site.example.com', // must match channel allowed_origins",
    '    },',
    '  })',
    '  const body = await r.json()',
    "  if (!body?.data?.session_token) return res.status(502).json({ error: 'mint failed' })",
    '  res.json({ token: body.data.session_token, expiresIn: body.data.expires_in })',
    '})',
  ].join('\n');
}

/** Vue buildSecureServerGoExample (lines 684-712). */
export function embedSecureServerGoExample(channelId: string, origin: string): string {
  const exchangeUrl = origin + '/api/v1/embed/' + channelId + '/exchange';
  return [
    '// Go net/http — keep WEKNORA_PUBLISH_TOKEN only on the server (env var).',
    'func embedTokenHandler(w http.ResponseWriter, r *http.Request) {',
    '  if r.Header.Get("Authorization") == "" && r.Header.Get("Cookie") == "" {',
    '    http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)',
    '    return',
    '  }',
    '  req, _ := http.NewRequest(http.MethodPost, "' + exchangeUrl + '", nil)',
    '  req.Header.Set("Authorization", "Embed "+os.Getenv("WEKNORA_PUBLISH_TOKEN"))',
    '  req.Header.Set("Origin", "https://your-site.example.com") // must match channel allowed_origins',
    '  resp, err := http.DefaultClient.Do(req)',
    '  if err != nil || resp.StatusCode >= 300 {',
    '    http.Error(w, `{"error":"mint failed"}`, http.StatusBadGateway)',
    '    return',
    '  }',
    '  defer resp.Body.Close()',
    '  var body struct { Data struct { SessionToken string `json:"session_token"` ExpiresIn int `json:"expires_in"` } `json:"data"` }',
    '  if json.NewDecoder(resp.Body).Decode(&body) != nil || body.Data.SessionToken == "" {',
    '    http.Error(w, `{"error":"mint failed"}`, http.StatusBadGateway)',
    '    return',
    '  }',
    '  w.Header().Set("Content-Type", "application/json")',
    '  json.NewEncoder(w).Encode(map[string]any{"token": body.Data.SessionToken, "expiresIn": body.Data.ExpiresIn})',
    '}',
  ].join('\n');
}

/** Vue snippetScenarioHint (lines 806-811). */
export function embedSnippetScenarioKey(tab: 'iframe' | 'widget' | 'secure'): string {
  if (tab === 'secure') return 'embedPublish.embedSecureDesc';
  return tab === 'widget' ? 'embedPublish.embedWidgetDesc' : 'embedPublish.embedIframeDesc';
}
