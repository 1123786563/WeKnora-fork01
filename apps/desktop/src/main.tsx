// Install Wails-only behavior before loading the shared renderer. A static
// import would evaluate the Web entry first and make the desktop bridge too
// late for API/deep-link/bootstrap decisions.
import { installDesktopRuntime } from './platform/runtime.ts';

installDesktopRuntime();
void import('../../web/src/main.tsx');
