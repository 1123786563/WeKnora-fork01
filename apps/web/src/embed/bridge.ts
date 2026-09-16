export type EmbedHostMessage =
  | { source: 'weknora-host'; type: 'token'; token: string; channel_id?: string }
  | { source: 'weknora-host'; type: 'context'; context: Record<string, unknown> }
  | { source: 'weknora-host'; type: 'locale'; locale: string };

export type EmbedErrorState = { kind: 'exchange' | 'disabled' | 'session' | 'attachment' | 'unknown'; retryable: boolean };

export function mapEmbedError(error: unknown): EmbedErrorState {
  const message = error instanceof Error ? error.message.toLowerCase() : String(error).toLowerCase();
  if (message.includes('disabled')) return { kind: 'disabled', retryable: false };
  if (message.includes('exchange') || message.includes('token')) return { kind: 'exchange', retryable: true };
  if (message.includes('session')) return { kind: 'session', retryable: true };
  if (message.includes('attachment') || message.includes('file')) return { kind: 'attachment', retryable: true };
  return { kind: 'unknown', retryable: true };
}

function originFromReferrer(referrer?: string): string {
  if (!referrer) return '';
  try {
    return new URL(referrer).origin;
  } catch {
    return '';
  }
}

function validOrigin(value?: string): string {
  const candidate = value?.trim() || '';
  if (!candidate || candidate === '*') return '';
  try {
    const url = new URL(candidate);
    return url.protocol === 'http:' || url.protocol === 'https:' ? url.origin : '';
  } catch {
    return '';
  }
}

export function createEmbedBridge(options: { parentWindow: object; referrer?: string; parentOrigin?: string }) {
  let pinnedOrigin = validOrigin(options.parentOrigin) || validOrigin(originFromReferrer(options.referrer));

  const accept = (event: MessageEvent<EmbedHostMessage>): boolean => {
    if (event.source !== options.parentWindow || event.origin === 'null' || event.data?.source !== 'weknora-host') return false;
    if (pinnedOrigin && event.origin !== pinnedOrigin) return false;
    pinnedOrigin = event.origin;
    return true;
  };

  const targetOrigin = () => pinnedOrigin || null;

  const post = (payload: Record<string, unknown>, send: (payload: Record<string, unknown>, targetOrigin: string) => void): boolean => {
    if (!pinnedOrigin) return false;
    send(payload, pinnedOrigin);
    return true;
  };

  return { accept, targetOrigin, post };
}

export function parseEmbedToken(location: { search: string; hash: string }): string {
  const queryToken = new URLSearchParams(location.search).get('token');
  if (queryToken) return queryToken;
  const hash = location.hash.startsWith('#') ? location.hash.slice(1) : location.hash;
  return hash ? new URLSearchParams(hash).get('token') || '' : '';
}

export function isEmbedImageFile(file: { type: string; name: string }): boolean {
  if (file.type.startsWith('image/')) return true;
  return ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp'].some((ext) => file.name.toLowerCase().endsWith(ext));
}

export function fileToDataUri(file: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}
