// Install Wails-only behavior before loading the shared renderer. A static
// import would evaluate the Web entry first and make the desktop bridge too
// late for API/deep-link/bootstrap decisions.
import { installDesktopRuntime } from './platform/runtime.ts';
import { readWailsBridge } from './platform/wails.ts';
import { resolveDesktopPersonalNode } from './platform/bootstrap.ts';

const wailsApp = readWailsBridge(typeof window === 'undefined' ? undefined : (window as Window & { go?: { main?: { App?: unknown } } }).go?.main?.App);
const personalNode = await resolveDesktopPersonalNode(wailsApp);
await installDesktopRuntime({ personalNode });
await import('../../web/src/main.tsx');
