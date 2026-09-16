export const EMBED_HOST_SOURCE = 'weknora-host';
export const EMBED_MESSAGE_SOURCE = 'weknora-embed';

export interface EmbedMessageEvent {
  sourceIsParent: boolean;
  origin: string;
  data: unknown;
}

function validOrigin(value: string): boolean {
  if (!value || value === 'null') return false;
  try {
    const url = new URL(value);
    return (url.protocol === 'http:' || url.protocol === 'https:') && url.origin === value;
  } catch {
    return false;
  }
}

export function createEmbedBridgeGuard(parentOriginHint = '') {
  let pinnedOrigin = validOrigin(parentOriginHint) ? parentOriginHint : '';
  return {
    accept(event: EmbedMessageEvent): boolean {
      if (!event.sourceIsParent || !validOrigin(event.origin)) return false;
      if (!event.data || typeof event.data !== 'object' || Array.isArray(event.data)) return false;
      if ((event.data as { source?: unknown }).source !== EMBED_HOST_SOURCE) return false;
      if (pinnedOrigin && event.origin !== pinnedOrigin) return false;
      pinnedOrigin ||= event.origin;
      return true;
    },
    targetOrigin(): string { return pinnedOrigin || '*'; },
    canSendSensitive(): boolean { return Boolean(pinnedOrigin); },
  };
}
