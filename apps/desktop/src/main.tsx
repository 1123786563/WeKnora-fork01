// Install Wails-only behavior before loading the shared renderer. A static
// import would evaluate the Web entry first and make the desktop bridge too
// late for API/deep-link/bootstrap decisions.
import { installDesktopRuntime } from './platform/runtime.ts';
import { createDesktopAppPersonalNode } from './platform/personalNodeApp.ts';

const wailsApp = (typeof window === 'undefined' ? undefined : (window as Window & { go?: { main?: { App?: Parameters<typeof createDesktopAppPersonalNode>[0] } } }).go?.main?.App);
const credentialBridge = wailsApp ? {
  ...wailsApp,
  readCredential: (key: string) => wailsApp.GetCredential?.(key) ?? null,
  removeCredential: (key: string) => wailsApp.DeleteCredential?.(key),
} : undefined;
const apiBaseURL = typeof window === 'undefined' ? undefined : (window as Window & { __WEKNORA_API_BASE__?: string }).__WEKNORA_API_BASE__;
await installDesktopRuntime({ personalNode: credentialBridge ? createDesktopAppPersonalNode(credentialBridge, { apiBaseURL }) : undefined });
await import('../../web/src/main.tsx');
