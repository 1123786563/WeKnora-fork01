import { createPersonalNodeRuntime, type PersonalNodeComposition, type PersonalNodeConnector } from '@weknora/paseo-adapter';

/**
 * Mobile's authenticated runtime composition. The caller supplies an
 * Expo-SecureStore-backed credential implementation after login; this module
 * is the app-owned entry point rather than a test-only adapter import.
 */
export function createMobilePersonalNodeRuntime(input: PersonalNodeComposition): PersonalNodeConnector {
  return createPersonalNodeRuntime(input);
}
