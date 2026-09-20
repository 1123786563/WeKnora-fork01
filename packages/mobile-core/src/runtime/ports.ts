export interface StoredCredential {
  accessToken: string;
  refreshToken: string;
}

export interface CredentialStore {
  read(deployment: string): Promise<StoredCredential | undefined>;
  write(deployment: string, credential: StoredCredential): Promise<void>;
  clear(deployment: string): Promise<void>;
}

export interface RuntimeRemote {
  passwordLogin(input: { email: string; password: string }): Promise<StoredCredential>;
  me(accessToken: string): Promise<{ user: { id: unknown }; tenant?: { id: unknown } | null }>;
  deploymentCapabilities(accessToken: string): Promise<unknown>;
}

/**
 * `remoteFor` is the composition seam. It receives a normalized HTTPS origin
 * only, so its transport and the credential scope are bound to the same value.
 */
export interface MobileRuntimePorts {
  credentialStore: CredentialStore;
  remoteFor(deployment: string): RuntimeRemote;
  clientVersion: number;
}
