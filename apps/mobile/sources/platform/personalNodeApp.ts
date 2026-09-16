import * as SecureStore from 'expo-secure-store';
import type { MobilePersonalNodeRuntimeInput } from './personalNodeRuntime.ts';
import { createRegistrationClient } from './registrationClient.ts';

type RuntimeEnv = { EXPO_PUBLIC_API_BASE_URL?: string; EXPO_PUBLIC_PASEO_URL?: string; EXPO_PUBLIC_PASEO_ALLOWED_ORIGINS?: string; EXPO_PUBLIC_PASEO_RUNTIME_ID?: string; EXPO_PUBLIC_PASEO_TARGET_ID?: string };

function env(): RuntimeEnv { return process.env as RuntimeEnv; }
function origins(raw: string | undefined): string[] { return (raw ?? '').split(',').map((value) => value.trim()).filter(Boolean); }

/** Build the authenticated mobile composition from deployment-owned configuration. */
export function createMobileAppPersonalNode(): MobilePersonalNodeRuntimeInput | undefined {
  const config = env();
  return createMobileAppPersonalNodeFromConfig({
    apiBaseURL: config.EXPO_PUBLIC_API_BASE_URL,
    paseoURL: config.EXPO_PUBLIC_PASEO_URL,
    allowedOrigins: origins(config.EXPO_PUBLIC_PASEO_ALLOWED_ORIGINS),
  });
}

export function createMobileAppPersonalNodeFromConfig(config: { apiBaseURL?: string; paseoURL?: string; allowedOrigins: string[] }): MobilePersonalNodeRuntimeInput | undefined {
  const baseURL = config.paseoURL?.trim();
  const allowedOrigins = config.allowedOrigins;
  const apiBaseURL = config.apiBaseURL?.trim();
  if (!baseURL || !apiBaseURL || allowedOrigins.length === 0) return undefined;
  return {
    secureStore: SecureStore,
    transport: { baseURL, allowedOrigins },
    registrationClient: createRegistrationClient({
      baseURL: apiBaseURL,
      accessToken: () => SecureStore.getItemAsync('weknora.mobile.access-token'),
    }),
    connectorOptions: {
      // Runtime identity is deployment-issued; enrollment remains an explicit user action.
    },
  } as MobilePersonalNodeRuntimeInput;
}
