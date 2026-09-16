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
  const isEnvelope = record.success !== undefined;
  if ((isEnvelope ? record.success !== true : false) || !isNonEmptyString(record.access_token)) {
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
  let persistenceTail: Promise<void> = Promise.resolve();
  let persistenceIntent = 0;

  function enqueuePersistence(operation: () => Promise<void>): Promise<void> {
    const task = persistenceTail.then(operation);
    persistenceTail = task.catch(() => undefined);
    return task;
  }

  async function persistRefresh(
    refreshed: BearerCredential,
    previous: BearerCredential,
    startGeneration: number,
  ): Promise<boolean> {
    const intent = ++persistenceIntent;
    return enqueuePersistence(async () => {
      if (generation !== startGeneration) return;
      await options.credentials.write(refreshed);
      if (generation !== startGeneration) {
        // An adapter write may already have started when invalidate() ran. If
        // no newer session write superseded it, restore the pre-refresh value
        // before exposing the invalidation to callers.
        if (persistenceIntent === intent) await options.credentials.write(previous);
        return;
      }
    }).then(() => generation === startGeneration && persistenceIntent === intent);
  }

  async function clearBearerIfCurrent(startGeneration: number): Promise<void> {
    await enqueuePersistence(async () => {
      if (generation !== startGeneration) return;
      const current = await options.credentials.read();
      if (generation !== startGeneration) return;
      if (isBearer(current)) await options.credentials.clear();
      // A generation change during clear is safe because the next credential
      // mutation is serialized behind this operation.
      if (generation !== startGeneration) {
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated during clear');
      }
    });
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
      if (!await persistRefresh(refreshed, current, startGeneration)) {
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated during refresh');
      }
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

  function write(value: Credential): Promise<void> {
    const startGeneration = generation;
    const intent = ++persistenceIntent;
    return enqueuePersistence(async () => {
      if (generation !== startGeneration || persistenceIntent !== intent) {
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated before it could be stored');
      }
      const previous = await options.credentials.read();
      if (generation !== startGeneration || persistenceIntent !== intent) {
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated before it could be stored');
      }
      await options.credentials.write(value);
      if (generation !== startGeneration || persistenceIntent !== intent) {
        if (persistenceIntent === intent) {
          await options.credentials.write(previous);
        }
        throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated while it was being stored');
      }
    });
  }

  async function invalidate(invalidateOptions?: { clear?: boolean }): Promise<void> {
    generation += 1;
    inFlight = undefined;
    if (invalidateOptions?.clear === false) return;
    const intent = ++persistenceIntent;
    await enqueuePersistence(async () => {
      if (persistenceIntent !== intent) return;
      await options.credentials.clear();
    });
  }

  return {
    refresh,
    write,
    /** Advance the refresh generation without deleting the current credential. */
    advanceGeneration() {
      generation += 1;
      // Do not let a subsequent request join a refresh started by the retired
      // identity. The old promise still observes the generation mismatch and
      // cannot write its result.
      inFlight = undefined;
    },
    /** Replace credentials for a newly authenticated identity. */
    async replace(value: BearerCredential): Promise<void> {
      const startGeneration = ++generation;
      inFlight = undefined;
      const intent = ++persistenceIntent;
      await enqueuePersistence(async () => {
        if (generation !== startGeneration || persistenceIntent !== intent) {
          throw new AuthError('AUTH_INVALIDATED', 'The credential was invalidated during replacement');
        }
        await options.credentials.write(value);
      });
    },
    invalidate,
    logout: invalidate,
    getGeneration: () => generation,
  };
}

export type RefreshCoordinator = ReturnType<typeof createRefreshCoordinator>;
