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
  const transport = new PaseoPersonalNodeTransport({
    ...input.transport,
    bearer: input.credentials.read(),
  });
  const connector = new PersonalNodeConnector(input.registrationClient, transport, input.connectorOptions);
  if (!input.lifecycle && !input.credentials) return connector;
  const revoke = connector.revoke.bind(connector);
  connector.revoke = async () => {
    await revoke();
    await input.credentials.clear();
    await input.lifecycle?.onRevoked?.();
  };
  return connector;
}

export type { PersonalNodeConnectorType };
