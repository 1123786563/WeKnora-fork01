import {
  PaseoPersonalNodeTransport,
  PersonalNodeConnector,
  type NodeRegistrationClient,
  type NodeConnectorOptions,
  type PersonalNodeConnector as PersonalNodeConnectorType,
  type PersonalNodeTransportConfig,
} from './node-connector.ts';

/**
 * The platform owns the credential lifecycle. Mobile and desktop callers
 * provide an OS-backed store (Expo SecureStore, Keychain, or a desktop
 * bridge); this adapter never persists a bearer in ordinary storage.
 */
export interface PersonalNodeCredentialStore {
  read(): string | undefined;
  clear(): Promise<void>;
}

export interface PersonalNodeLifecycle {
  onRevoked?(): Promise<void> | void;
}

export interface PersonalNodeComposition {
  registrationClient: NodeRegistrationClient;
  transport: Omit<PersonalNodeTransportConfig, 'bearer'>;
  credentials: PersonalNodeCredentialStore;
  lifecycle?: PersonalNodeLifecycle;
  connectorOptions?: NodeConnectorOptions;
}

/**
 * Production composition seam for the configured Paseo bridge. The endpoint
 * policy is still enforced by PaseoPersonalNodeTransport, while the bearer
 * is read once from secure storage and is cleared when the server revokes the
 * registration. Tests can inject fake fetch and stores through this same seam.
 */
export function createPersonalNodeConnector(input: PersonalNodeComposition): PersonalNodeConnector {
  if (!input.credentials) throw new Error('PASEO_CREDENTIAL_STORE_MISSING');
  const bearer = input.credentials.read();
  if (!bearer || !bearer.trim()) throw new Error('PASEO_CREDENTIAL_MISSING');
  const transport = new PaseoPersonalNodeTransport({
    ...input.transport,
    bearer,
  });
  const connector = new PersonalNodeConnector(input.registrationClient, transport, input.connectorOptions);
  const revoke = connector.revoke.bind(connector);
  connector.revoke = async () => {
    let failure: unknown;
    try { await revoke(); } catch (cause) { failure = cause; }
    try { await input.credentials.clear(); } catch (cause) { if (!failure) failure = cause; }
    try { await input.lifecycle?.onRevoked?.(); } catch (cause) { if (!failure) failure = cause; }
    if (failure) throw failure;
  };
  return connector;
}

export type { PersonalNodeConnectorType };
