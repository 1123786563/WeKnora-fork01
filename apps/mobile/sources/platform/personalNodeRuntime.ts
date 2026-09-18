import { createPersonalNodeRuntime, type PersonalNodeComposition, type PersonalNodeConnector } from '@weknora/paseo-adapter';

export interface MobileSecureCredentialStore {
  getItemAsync(key: string): Promise<string | null>;
  deleteItemAsync(key: string): Promise<void>;
}

export type MobilePersonalNodeRuntimeInput = Omit<PersonalNodeComposition, 'credentials'> & {
  secureStore: MobileSecureCredentialStore;
  credentialKey?: string;
};

/**
 * Mobile's authenticated runtime composition. The caller supplies an
 * Expo-SecureStore-backed credential implementation after login; this module
 * is the app-owned entry point rather than a test-only adapter import.
 */
export async function createMobilePersonalNodeRuntime(input: MobilePersonalNodeRuntimeInput): Promise<PersonalNodeConnector> {
  const credentialKey = input.credentialKey ?? 'weknora.mobile.personal-node-credential';
  const bearer = await input.secureStore.getItemAsync(credentialKey);
  // Keep the adapter fail-closed: an absent SecureStore value never creates a
  // connector, even if the caller supplied a valid endpoint and registration client.
  if (!bearer?.trim()) throw new Error('PASEO_CREDENTIAL_MISSING');
  return createPersonalNodeRuntime({
    ...input,
    credentials: {
      read: () => bearer,
      clear: () => input.secureStore.deleteItemAsync(credentialKey),
    },
  });
}
