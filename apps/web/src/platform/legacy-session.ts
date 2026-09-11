import type { Credential } from '@weknora/api-client';

export interface LegacyPlatformSession {
  credential: Credential;
  tenantId: string | null;
}

export interface ReactPlatformState extends LegacyPlatformSession {
  preferences: Record<string, string>;
}

export const REACT_SESSION_STORAGE_KEY = 'weknora_react_session_v1';
export const REACT_LEGACY_IMPORT_MARKER_KEY = 'weknora_react_legacy_import_v1';
export const REACT_LEGACY_FALLBACK_STORAGE_KEY = 'weknora_react_legacy_fallback_v1';

const LEGACY_PREFERENCE_KEYS = [
  'weknora_selected_tenant_name',
  'weknora_lite_mode',
  'locale',
  'sidebar_collapsed',
  'sandbox_panel_width',
] as const;

type BrowserStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>;

function safeGetItem(storage: Pick<Storage, 'getItem'>, key: string): string | null {
  try {
    return storage.getItem(key);
  } catch {
    return null;
  }
}

export interface LegacyPlatformAdapter {
  read(): LegacyPlatformSession;
}

/** Migration-only seam for the Vue-era browser keys. The shared client never reads storage. */
export function createLegacyPlatformAdapter(storage: Pick<Storage, 'getItem'>): LegacyPlatformAdapter {
  return {
    read(): LegacyPlatformSession {
      const token = safeGetItem(storage, 'weknora_token')?.trim() ?? '';
      const tenantId = safeGetItem(storage, 'weknora_selected_tenant_id')?.trim() || null;
      if (!token) return { credential: { kind: 'anonymous' }, tenantId };

      if (/^embed\s/i.test(token)) {
        return { credential: { kind: 'embed', token }, tenantId };
      }
      const refreshToken = safeGetItem(storage, 'weknora_refresh_token')?.trim() || undefined;
      return {
        credential: refreshToken
          ? { kind: 'bearer', accessToken: token, refreshToken }
          : { kind: 'bearer', accessToken: token },
        tenantId,
      };
    },
  };
}

export function readLegacyPlatformSession(): LegacyPlatformSession {
  return createLegacyPlatformAdapter(window.localStorage).read();
}

function anonymousState(): ReactPlatformState {
  return { credential: { kind: 'anonymous' }, tenantId: null, preferences: {} };
}

function isCredential(value: unknown): value is Credential {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false;
  const candidate = value as Record<string, unknown>;
  if (candidate.kind === 'anonymous') return true;
  if (candidate.kind === 'bearer') {
    return typeof candidate.accessToken === 'string' && candidate.accessToken.trim().length > 0
      && (candidate.refreshToken === undefined || typeof candidate.refreshToken === 'string');
  }
  if (candidate.kind === 'embed') {
    return typeof candidate.token === 'string' && candidate.token.trim().length > 0;
  }
  return false;
}

function parseState(value: unknown): ReactPlatformState | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const candidate = value as Record<string, unknown>;
  if (!isCredential(candidate.credential)) return null;
  if (candidate.tenantId !== null && typeof candidate.tenantId !== 'string') return null;
  if (!candidate.preferences || typeof candidate.preferences !== 'object' || Array.isArray(candidate.preferences)) return null;
  const preferences: Record<string, string> = {};
  for (const [key, item] of Object.entries(candidate.preferences)) {
    if (typeof item !== 'string') return null;
    preferences[key] = item;
  }
  return {
    credential: candidate.credential,
    tenantId: candidate.tenantId,
    preferences,
  };
}

function readJson(storage: Pick<Storage, 'getItem'>, key: string): unknown | null {
  const raw = safeGetItem(storage, key);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
}

function readFallback(storage: Pick<Storage, 'getItem'>): ReactPlatformState | null {
  const parsed = readJson(storage, REACT_LEGACY_FALLBACK_STORAGE_KEY);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null;
  return parseState((parsed as Record<string, unknown>).state);
}

/** Read the React record, falling back to the durable pre-migration snapshot. */
export function readReactPlatformState(storage: Pick<Storage, 'getItem'>): ReactPlatformState | null {
  const canonical = parseState(readJson(storage, REACT_SESSION_STORAGE_KEY));
  return canonical ?? readFallback(storage);
}

function collectLegacyPreferences(storage: Pick<Storage, 'getItem'>): Record<string, string> {
  const preferences: Record<string, string> = {};
  for (const key of LEGACY_PREFERENCE_KEYS) {
    const value = safeGetItem(storage, key);
    if (value !== null) preferences[key] = value;
  }
  return preferences;
}

/**
 * Copy Vue-era browser state once. The old keys deliberately remain available
 * to the legacy artifact; the marker prevents React from treating them as a
 * live session source after the first successful import.
 */
export function importLegacyPlatformState(
  storage: BrowserStorage,
  now: () => string = () => new Date().toISOString(),
): ReactPlatformState {
  const marker = safeGetItem(storage, REACT_LEGACY_IMPORT_MARKER_KEY);
  const saved = readReactPlatformState(storage);
  if (marker === 'complete') return saved ?? anonymousState();
  if (marker === 'pending' && saved) return saved;

  const legacy = createLegacyPlatformAdapter(storage).read();
  const state: ReactPlatformState = {
    ...legacy,
    preferences: collectLegacyPreferences(storage),
  };
  const fallback = JSON.stringify({ version: 1, importedAt: now(), state });

  // The fallback is written first so a partially completed migration can be
  // recovered even if the canonical record or marker write is interrupted.
  try {
    storage.setItem(REACT_LEGACY_FALLBACK_STORAGE_KEY, fallback);
    storage.setItem(REACT_LEGACY_IMPORT_MARKER_KEY, 'pending');
    storage.setItem(REACT_SESSION_STORAGE_KEY, JSON.stringify(state));
    storage.setItem(REACT_LEGACY_IMPORT_MARKER_KEY, 'complete');
  } catch {
    // Keep the old session usable for this boot. A later boot can retry from
    // the fallback record without ever logging or exposing token contents.
  }
  return state;
}

export function persistReactPlatformState(storage: BrowserStorage, state: ReactPlatformState): void {
  storage.setItem(REACT_SESSION_STORAGE_KEY, JSON.stringify(state));
}

export function persistSelectedTenant(storage: BrowserStorage, tenantId: string | null): void {
  if (tenantId) storage.setItem('weknora_selected_tenant_id', tenantId);
  else storage.removeItem('weknora_selected_tenant_id');

  const state = readReactPlatformState(storage);
  if (state) persistReactPlatformState(storage, { ...state, tenantId });
}

export function authorizationHeader(credential: Credential): string | undefined {
  if (credential.kind === 'bearer') return `Bearer ${credential.accessToken}`;
  if (credential.kind === 'embed') return credential.token;
  return undefined;
}
