export interface EmbedResourceLike {
  name?: unknown;
  agent_id?: unknown;
  enabled?: unknown;
  allowed_origins?: unknown;
  welcome_message?: unknown;
  rate_limit_per_minute?: unknown;
  rate_limit_per_day?: unknown;
  primary_color?: unknown;
  page_title?: unknown;
  header_title_mode?: unknown;
  show_suggested_questions?: unknown;
  widget_position?: unknown;
  allow_web_search?: unknown;
  allow_file_upload?: unknown;
  default_locale?: unknown;
  webhook_url?: unknown;
}

export function buildEmbedUpdatePayload(channel: EmbedResourceLike, nextName: string): Record<string, unknown> {
  const origins = Array.isArray(channel.allowed_origins) ? channel.allowed_origins.filter((value): value is string => typeof value === 'string') : [];
  if (origins.length === 0) throw new Error('Embed channel must keep at least one allowed origin when updating.');
  return {
    name: nextName.trim(),
    allowed_origins: origins,
    enabled: channel.enabled !== false,
    welcome_message: typeof channel.welcome_message === 'string' ? channel.welcome_message : '',
    rate_limit_per_minute: typeof channel.rate_limit_per_minute === 'number' ? channel.rate_limit_per_minute : 30,
    rate_limit_per_day: typeof channel.rate_limit_per_day === 'number' ? channel.rate_limit_per_day : 10000,
    primary_color: typeof channel.primary_color === 'string' ? channel.primary_color : '',
    page_title: typeof channel.page_title === 'string' ? channel.page_title : '',
    header_title_mode: typeof channel.header_title_mode === 'string' ? channel.header_title_mode : 'channel',
    show_suggested_questions: channel.show_suggested_questions !== false,
    widget_position: typeof channel.widget_position === 'string' ? channel.widget_position : 'bottom-right',
    allow_web_search: channel.allow_web_search === true,
    allow_file_upload: channel.allow_file_upload === true,
    ...(typeof channel.default_locale === 'string' ? { default_locale: channel.default_locale } : {}),
    ...(typeof channel.webhook_url === 'string' ? { webhook_url: channel.webhook_url } : {}),
    ...(typeof channel.agent_id === 'string' ? { agent_id: channel.agent_id } : {}),
  };
}
