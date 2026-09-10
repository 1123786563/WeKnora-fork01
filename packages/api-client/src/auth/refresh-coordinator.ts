import type { BearerCredential, Credential, CredentialAdapter } from '../ports.ts';

export type RefreshResponse = unknown;

export interface RefreshCoordinatorOptions {
  credentials: CredentialAdapter;
  refresh(refreshToken: string): Promise<RefreshResponse>;
}

export type AuthErrorCode = 'AUTH_NOT_REFRESHABLE' | 'AUTH_REFRESH_INVALID' | 'AUTH_INVALIDATED';

export class AuthError extends Error {
  readonly code: AuthErrorCode;

  constructor(code: AuthErrorCode, message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'AuthError';
    this.code = code;
  }
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}

function parseRefreshCredential(response: RefreshResponse, previous: BearerCredential): BearerCredential {
  if (typeof response !== 'object' || response === null) {
    throw new AuthError('AUTH_REFRESH_INVALID', 'Refresh response did not contain valid tokens');
  }
  const record = response as Record<string, unknown>;
  if (record.success !== true || !isNonEmptyString(record.access_token)) {
    throw new AuthError('AUTH_REFRESH_INVALID', 'Refresh response did not contain a valid access token');
  }
  if (record.refresh_token !== undefined && !isNonEmptyString(record.refresh_token)) {
    throw new AuthError('AUTH_REFRESH_INVALID', 'Refresh response contained an invalid refresh token');
  }
  return {
    kind: 'bearer',
    accessToken: record.access_token,
    refreshToken: record.refresh_token === undefined ? previous.refreshToken : record.refresh_token,
  };
}

function isBearer(credential: Credential): credential is BearerCredential {
  return credential.kind === 'bearer';
}

export function createRefreshCoordinator(options: RefreshCoordinatorOptions) {
  let generation = 0;
  let inFlight: Promise<BearerCredential> | undefined;

  async function clearBearerIfCurrent(startGeneration: number): Promise<void> {
    if (generation !== startGeneration) return;
    const current = await options.credentials.read();
    if (generation === startGeneration && isBearer(current)) await options.credentials.clear();
  }

  async function performRefresh(startGeneration: number): Promise<BearerCredential> {
    const current = await options.credentials.read();
    if (!isBearer(current) || !current.refreshToken) {
      throw new AuthError('AUTH_NOT_REFRESHABLE', 'The current credential cannot be refreshed');
    }

    try {
      const refreshed = parseRefreshCredential(await options.refresh(current.refreshToken), current);
      if (generation !== startGeneration) {
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated during refresh');
      }
      await options.credentials.write(refreshed);
      return refreshed;
    } catch (error: unknown) {
      if (generation !== startGeneration) {
        if (error instanceof AuthError && error.code === 'AUTH_INVALIDATED') throw error;
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated during refresh', { cause: error });
      }
      await clearBearerIfCurrent(startGeneration);
      throw error;
    }
  }

  function refresh(): Promise<BearerCredential> {
    if (inFlight) return inFlight;
    const startGeneration = generation;
    const task = performRefresh(startGeneration);
    let shared!: Promise<BearerCredential>;
    shared = task.finally(() => {
      if (inFlight === shared) inFlight = undefined;
    });
    inFlight = shared;
    return shared;
  }

  async function invalidate(): Promise<void> {
    generation += 1;
    await options.credentials.clear();
  }

  return {
    refresh,
    invalidate,
    logout: invalidate,
    getGeneration: () => generation,
  };
}

export type RefreshCoordinator = ReturnType<typeof createRefreshCoordinator>;
