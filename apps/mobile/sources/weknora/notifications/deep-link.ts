/**
 * The only data a notification may carry into the product is a navigation
 * reference.  It is deliberately parsed separately from the notification
 * payload so a provider cannot smuggle a command, bearer token, or server
 * override into the native router.
 */
export interface NotificationLink {
  tenantID: string;
  runID: string;
}

const MAX_REFERENCE_LENGTH = 128;
const REFERENCE = /^[A-Za-z0-9][A-Za-z0-9._~:-]{0,127}$/;

function invalid(): never {
  throw new Error('INVALID_LINK');
}

function reference(value: string | null): string {
  if (value === null || value.length === 0 || value.length > MAX_REFERENCE_LENGTH || !REFERENCE.test(value)) invalid();
  return value;
}

/**
 * Parse only `weknora://execution?tenant=<id>&run=<id>`.
 *
 * The raw prefix is checked as well as URL's normalized fields. URL parsing
 * lowercases hostnames, so checking only `url.hostname` would accept a
 * differently-cased authority and make the accepted grammar less explicit.
 */
export function parseNotificationLink(raw: string): NotificationLink {
  if (typeof raw !== 'string' || raw.length === 0 || raw.length > 512 || !/^weknora:\/\/execution\?/.test(raw)) invalid();

  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    invalid();
  }

  if (
    url.protocol !== 'weknora:' ||
    url.hostname !== 'execution' ||
    url.port !== '' ||
    url.pathname !== '' ||
    url.hash !== '' ||
    url.username !== '' ||
    url.password !== ''
  ) invalid();

  const keys = [...url.searchParams.keys()];
  if (keys.length !== 2 || keys[0] === keys[1] || !keys.includes('tenant') || !keys.includes('run')) invalid();
  if (url.searchParams.getAll('tenant').length !== 1 || url.searchParams.getAll('run').length !== 1) invalid();

  return {
    tenantID: reference(url.searchParams.get('tenant')),
    runID: reference(url.searchParams.get('run')),
  };
}

export function notificationLinkKey(link: NotificationLink): string {
  return `${link.tenantID}:${link.runID}`;
}
