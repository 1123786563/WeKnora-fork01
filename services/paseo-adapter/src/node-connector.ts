export interface NodeGrant {
  targetID: string;
  epoch: number;
  expiresAt: number;
  operations: string[];
}

export function validateNodeGrant(
  grant: NodeGrant,
  expected: { targetID: string; epoch: number },
  operation: string,
  now = Date.now(),
): void {
  if (grant.targetID !== expected.targetID || grant.epoch !== expected.epoch || !Number.isSafeInteger(grant.epoch) || grant.expiresAt <= now || !grant.operations.includes(operation)) {
    throw new Error('NODE_GRANT');
  }
}

export interface NodeChallenge {
  challenge_id: string;
  runtime_id: string;
  external_target_id: string;
  public_key: string;
  nonce: string;
  expires_at: string;
}

export interface NodeRegistration {
  id: string;
  runtime_id: string;
  external_target_id: string;
  public_key: string;
  credential_version: number;
  state: 'active' | 'revoked';
}

export interface NodeRegistrationClient {
  createChallenge(input: { runtime_id: string; external_target_id: string; public_key: string }): Promise<NodeChallenge>;
  complete(input: { challenge_id: string; nonce: string; runtime_id: string; external_target_id: string; public_key: string; signature: string; idempotency_key: string; confirmed: boolean }): Promise<{ registration: NodeRegistration; grant: NodeGrant }>;
  revoke(registrationID: string): Promise<void>;
}

export interface PersonalNodeCommand {
  operation: 'start' | 'observe' | 'stop';
  payload: unknown;
}

export interface PersonalNodeTransport {
  send(command: PersonalNodeCommand, grant: NodeGrant): Promise<{ accepted: boolean; commandID?: string }>;
  close(): Promise<void>;
}

export interface PersonalNodeTransportConfig {
  /** Server-provided, pinned Paseo bridge URL. It is never accepted from a command payload. */
  baseURL: string;
  fetchImpl?: typeof fetch;
  bearer?: string;
}

export interface PaseoNodeHeartbeat { nodeID: string; credentialVersion: number; at: number }
export interface PaseoNodeLog { level: 'debug' | 'info' | 'warn' | 'error'; message: string; at: number }

/**
 * Concrete configured-platform transport. The bridge protocol is intentionally
 * small and explicit: commands have an acknowledgement, heartbeats prove the
 * credential version, logs are sent to the platform sink, and rotation replaces
 * the short-lived credential. No caller can redirect it to an arbitrary daemon.
 */
export class PaseoPersonalNodeTransport implements PersonalNodeTransport {
  private readonly baseURL: URL;
  private readonly fetchImpl: typeof fetch;
  constructor(config: PersonalNodeTransportConfig) {
    this.baseURL = new URL(config.baseURL);
    if (this.baseURL.protocol !== 'https:' && this.baseURL.hostname !== 'localhost' && this.baseURL.hostname !== '127.0.0.1') throw new Error('PASEO_ENDPOINT_FORBIDDEN');
    this.fetchImpl = config.fetchImpl ?? fetch;
    this.bearer = config.bearer;
  }
  private bearer?: string;
  private async request(path: string, body?: unknown, method = 'POST'): Promise<any> {
    const response = await this.fetchImpl(new URL(path, this.baseURL), { method, headers: { 'content-type': 'application/json', ...(this.bearer ? { authorization: `Bearer ${this.bearer}` } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
    if (!response.ok) throw new Error(`PASEO_HTTP_${response.status}`);
    return response.status === 204 ? undefined : response.json();
  }
  async send(command: PersonalNodeCommand, grant: NodeGrant): Promise<{ accepted: boolean; commandID?: string }> {
    const response = await this.request('/v1/commands', { operation: command.operation, payload: command.payload, grant });
    if (typeof response?.accepted !== 'boolean') throw new Error('PASEO_ACK_INVALID');
    return { accepted: response.accepted, commandID: typeof response.commandID === 'string' ? response.commandID : undefined };
  }
  async heartbeat(input: PaseoNodeHeartbeat): Promise<void> { await this.request('/v1/heartbeat', input); }
  async appendLog(log: PaseoNodeLog): Promise<void> { await this.request('/v1/logs', log); }
  async rotateCredential(credential: string): Promise<void> { await this.request('/v1/credentials/rotate', { credential }); this.bearer = credential; }
  async close(): Promise<void> { await this.request('/v1/close', undefined); }
}

export interface NodeConnectorOptions {
  now?: () => number;
  backoffMs?: number[];
  sleep?: (ms: number) => Promise<void>;
}

// This connector holds only the public registration projection and short-lived
// grants. Long-lived platform bearers/private keys stay in the platform or
// OS secure storage and are never sent to a personal node.
export class PersonalNodeConnector {
  private registration?: NodeRegistration;
  private grant?: NodeGrant;
  private challengeIDs = new Set<string>();
  private readonly now: () => number;

  constructor(private readonly client: NodeRegistrationClient, private readonly transport: PersonalNodeTransport, options: NodeConnectorOptions = {}) {
    this.now = options.now ?? (() => Date.now());
  }

  async enroll(input: { runtime_id: string; external_target_id: string; public_key: string; signature: string; idempotency_key: string }): Promise<NodeRegistration> {
    if (this.registration?.state === 'active') throw new Error('NODE_ALREADY_ENROLLED');
    const challenge = await this.client.createChallenge({ runtime_id: input.runtime_id, external_target_id: input.external_target_id, public_key: input.public_key });
    if (this.challengeIDs.has(challenge.challenge_id) || Date.parse(challenge.expires_at) <= this.now()) throw new Error('REGISTRATION_CHALLENGE');
    this.challengeIDs.add(challenge.challenge_id);
    const result = await this.client.complete({ ...input, challenge_id: challenge.challenge_id, nonce: challenge.nonce, confirmed: true });
    if (result.registration.state !== 'active' || result.registration.credential_version < 1) throw new Error('REGISTRATION_CREDENTIAL');
    this.registration = result.registration;
    this.grant = result.grant;
    return result.registration;
  }

  async send(command: PersonalNodeCommand): Promise<{ accepted: boolean; commandID?: string }> {
    if (!this.registration || this.registration.state !== 'active' || !this.grant) throw new Error('NODE_REVOKED');
    validateNodeGrant(this.grant, { targetID: this.grant.targetID, epoch: this.registration.credential_version }, command.operation, this.now());
    return this.transport.send(command, this.grant);
  }

  async revoke(): Promise<void> {
    if (!this.registration) return;
    await this.client.revoke(this.registration.id);
    this.registration = { ...this.registration, state: 'revoked', credential_version: this.registration.credential_version + 1 };
    this.grant = undefined;
    await this.transport.close();
  }
}

export async function connectWithBackoff<T>(connect: () => Promise<T>, options: NodeConnectorOptions = {}): Promise<T> {
  const delays = options.backoffMs ?? [250, 1_000, 5_000];
  const sleep = options.sleep ?? ((ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms)));
  let lastError: unknown;
  for (let attempt = 0; attempt <= delays.length; attempt += 1) {
    try { return await connect(); } catch (error) {
      lastError = error;
      if (attempt === delays.length) break;
      await sleep(delays[attempt]);
    }
  }
  throw lastError instanceof Error ? lastError : new Error('NODE_UNAVAILABLE');
}
