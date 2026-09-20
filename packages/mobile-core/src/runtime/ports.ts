/** Exact credential fields returned by Task 2's `passwordLogin` adapter. */
export interface StoredCredential {
  token: string;
  refreshToken: string;
}

export interface CredentialStore {
  read(deployment: string): Promise<StoredCredential | undefined>;
  write(deployment: string, credential: StoredCredential): Promise<void>;
  clear(deployment: string): Promise<void>;
}

/** Structural subset of `createMobileRuntimeRemote`; mobile-core remains adapter-independent. */
export interface RuntimeRemote {
  passwordLogin(input: { email: string; password: string }): Promise<StoredCredential>;
  me(accessToken: string): Promise<{ user: { id: unknown }; tenant?: { id: unknown } | null }>;
  deploymentCapabilities(accessToken: string): Promise<unknown>;
}

/** Declared here for Task 4, which owns persistence and one-time callback consumption. */
export interface PendingOidcStore {
  savePending(input: PendingOidc): Promise<void>;
  loadPending(): Promise<PendingOidc | undefined>;
  consumePending(): Promise<PendingOidc | undefined>;
}

export interface PendingOidc {
  deploymentOrigin: string;
  state: string;
  codeVerifier: string;
  redirectUri: string;
}

/** Declared here for Task 4's native browser/deep-link adapter. */
export interface OidcBrowserPort {
  open(authorizationUrl: string): Promise<string>;
}

/** Declared here for Task 4's lifecycle-aware OIDC recovery. */
export interface AppLifecyclePort {
  subscribe(listener: (state: 'active' | 'background') => void): () => void;
}

/** `remoteFor` receives a normalized HTTPS origin, binding transport and credential scope. */
export interface MobileRuntimePorts {
  credentialStore: CredentialStore;
  remoteFor(deployment: string): RuntimeRemote;
  clientVersion: number;
  pendingOidcStore?: PendingOidcStore;
  oidcBrowser?: OidcBrowserPort;
  lifecycle?: AppLifecyclePort;
}
