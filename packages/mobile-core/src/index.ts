export { createMobileRuntime } from './runtime/mobile-runtime.ts';
export { createInMemoryCredentialStore } from './runtime/in-memory-adapters.ts';
export type { AppLifecyclePort, CredentialStore, DeploymentStore, MobileRuntimePorts, OidcBrowserPort, PendingOidc, PendingOidcStore, RuntimeRemote, StoredCredential } from './runtime/ports.ts';
export type { Deployment, DeploymentInput, MobileRuntime, RuntimeReason, RuntimeSnapshot, RuntimeSurface, ScopeLease } from './runtime/types.ts';
