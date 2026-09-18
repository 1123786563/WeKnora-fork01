import type { Locale } from '@weknora/i18n/runtime';

// Vue parity references (frontend/src/api/embed/index.ts):
// - EMBED_MSG_SOURCE / EMBED_HOST_SOURCE (L439-440)
// - inbound host message handlers onEmbedHostToken/Context/Locale/OpenWithQuery (L727-771)
// - outbound postToParent payloads postEmbedReady/BootstrapRequest/MessageSent/MessageReceived (L497-552)
// Vue message names are part of the public host contract — keep them byte-identical.
export const EMBED_MESSAGE_SOURCE = 'weknora-embed';
export const EMBED_HOST_SOURCE = 'weknora-host';

export type EmbedHostEvent =
  | { type: 'provide_token'; token: string; channelId?: string }
  | { type: 'set_context'; payload: Record<string, unknown> }
  | { type: 'set_locale'; locale: string }
  | { type: 'open_with_query'; query: string };

function objectPayload(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function stringField(record: Record<string, unknown>, nestedKey: string, flatKey: string): string {
  const nested = objectPayload(record[nestedKey]);
  const raw = nested[flatKey] ?? record[flatKey];
  return String(raw ?? '').trim();
}

/**
 * Parse one host postMessage data payload. Mirrors the per-type guards of the
 * Vue onEmbedHost* listeners: unknown sources/types and empty required fields
 * are ignored rather than acted on.
 */
export function parseEmbedHostMessage(data: unknown): EmbedHostEvent | null {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null;
  const record = data as Record<string, unknown>;
  if (record.source !== EMBED_HOST_SOURCE) return null;
  switch (record.type) {
    case 'provide_token': {
      const token = String(record.token ?? '').trim();
      if (!token) return null;
      const channelId = typeof record.channel_id === 'string' && record.channel_id.trim() ? record.channel_id.trim() : undefined;
      return { type: 'provide_token', token, channelId };
    }
    case 'set_context':
      return { type: 'set_context', payload: objectPayload(record.payload) };
    case 'set_locale': {
      const locale = stringField(record, 'payload', 'locale');
      if (!locale) return null;
      return { type: 'set_locale', locale };
    }
    case 'open_with_query': {
      const query = stringField(record, 'payload', 'query');
      if (!query) return null;
      return { type: 'open_with_query', query };
    }
    default:
      return null;
  }
}

/** Vue postEmbedBootstrapRequest (L503-505): token handoff request to the host. */
export function embedBootstrapRequestPayload(channelId: string): Record<string, unknown> {
  return { source: EMBED_MESSAGE_SOURCE, type: 'bootstrap_request', channel_id: channelId };
}

/** Vue postEmbedReady (L498-500): widget/boot completion notice to the host. */
export function embedReadyPayload(channelId: string): Record<string, unknown> {
  return { source: EMBED_MESSAGE_SOURCE, type: 'ready', channel_id: channelId };
}

/** Vue postEmbedMessageSent (L508-518) — sensitive conversation content. */
export function embedMessageSentPayload(channelId: string, sessionId: string, query: string): Record<string, unknown> {
  return { source: EMBED_MESSAGE_SOURCE, type: 'message_sent', channel_id: channelId, session_id: sessionId, query };
}

/** Vue postEmbedMessageReceived (L542-552) — sensitive conversation content. */
export function embedMessageReceivedPayload(channelId: string, sessionId: string, content: string): Record<string, unknown> {
  return { source: EMBED_MESSAGE_SOURCE, type: 'message_received', channel_id: channelId, session_id: sessionId, content };
}

// Vue EmbedPage.vue pageStyle/badgeStyle: the channel primary_color is injected
// as a CSS custom property (--embed-primary) and reused for the header badge
// via color-mix. The React embed scope keeps the same variable name so channel
// theming (not a shadcn default) drives the accent color.
export function embedThemeVars(primaryColor?: string): Record<string, string> {
  const color = primaryColor?.trim();
  if (!color) return {};
  return { '--embed-primary': color };
}

export function embedBadgeStyle(primaryColor?: string): Record<string, string> {
  const color = primaryColor?.trim();
  if (!color) return {};
  return { background: `color-mix(in srgb, ${color} 12%, transparent)`, color };
}

const SUPPORTED_LOCALES = ['zh-CN', 'en-US', 'ja-JP', 'ko-KR', 'ru-RU'] as const;

/**
 * Normalize a host-provided locale tag onto the app's supported locales
 * (Vue EmbedLocaleTag, frontend/src/api/embed/index.ts:50). Accepts exact tags
 * and falls back to the primary BCP-47 subtag; unknown locales stay unpinned so
 * the channel default_locale applies.
 */
export function normalizeEmbedLocale(raw: string): Locale | '' {
  const value = raw.trim();
  if (!value) return '';
  const lower = value.toLowerCase();
  const exact = SUPPORTED_LOCALES.find((locale) => locale.toLowerCase() === lower);
  if (exact) return exact;
  const primary = lower.split(/[-_]/)[0];
  const match = SUPPORTED_LOCALES.find((locale) => locale.toLowerCase().startsWith(primary));
  return match ?? '';
}

// Vue api/embed/index.ts:59-66 — short-lived session token prefix and the
// per-channel localStorage keys for visitor id / persisted chat session.
export const EMBED_SESSION_TOKEN_PREFIX = 'ems_';

export function isEmbedSessionToken(token: string): boolean {
  return typeof token === 'string' && token.trim().startsWith(EMBED_SESSION_TOKEN_PREFIX);
}

export function embedVisitorStorageKey(channelId: string): string {
  return `weknora-embed-visitor:${channelId}`;
}

export function embedChatSessionStorageKey(channelId: string): string {
  return `weknora-embed-session:${channelId}`;
}
