import type { CredentialStore, StoredCredential } from './ports.ts';

export function createInMemoryCredentialStore(initial: Record<string, StoredCredential | undefined> = {}): CredentialStore {
  const values = new Map(Object.entries(initial));
  return {
    async read(deployment) { return values.get(deployment); },
    async write(deployment, credential) { values.set(deployment, { ...credential }); },
    async clear(deployment) { values.delete(deployment); },
  };
}
