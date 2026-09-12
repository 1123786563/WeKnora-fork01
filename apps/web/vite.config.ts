import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@weknora/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@weknora/contracts': fileURLToPath(new URL('../../packages/contracts/src/index.ts', import.meta.url)),
      '@weknora/domain/scope': fileURLToPath(new URL('../../packages/domain/src/scope.ts', import.meta.url)),
      '@weknora/domain/chat/draft': fileURLToPath(new URL('../../packages/domain/src/chat/draft.ts', import.meta.url)),
      '@weknora/domain/chat/reducer': fileURLToPath(new URL('../../packages/domain/src/chat/reducer.ts', import.meta.url)),
      '@weknora/domain/chat/session-state': fileURLToPath(new URL('../../packages/domain/src/chat/session-state.ts', import.meta.url)),
      '@weknora/domain/chat/artifacts': fileURLToPath(new URL('../../packages/domain/src/chat/artifacts.ts', import.meta.url)),
      '@weknora/domain/chat/references': fileURLToPath(new URL('../../packages/domain/src/chat/references.ts', import.meta.url)),
      '@weknora/domain/chat/tool-results': fileURLToPath(new URL('../../packages/domain/src/chat/tool-results.ts', import.meta.url)),
      '@weknora/domain/knowledge/folders': fileURLToPath(new URL('../../packages/domain/src/knowledge/folders.ts', import.meta.url)),
      '@weknora/domain/knowledge/processing': fileURLToPath(new URL('../../packages/domain/src/knowledge/processing.ts', import.meta.url)),
      '@weknora/domain/knowledge/preview': fileURLToPath(new URL('../../packages/domain/src/knowledge/preview.ts', import.meta.url)),
      '@weknora/domain/sandbox/terminal': fileURLToPath(new URL('../../packages/domain/src/sandbox/terminal.ts', import.meta.url)),
      '@weknora/domain/wiki/diff': fileURLToPath(new URL('../../packages/domain/src/wiki/diff.ts', import.meta.url)),
      '@weknora/domain': fileURLToPath(new URL('../../packages/domain/src/query-key.ts', import.meta.url)),
      '@weknora/ui': fileURLToPath(new URL('../../packages/ui/src/index.tsx', import.meta.url)),
      '@weknora/views': fileURLToPath(new URL('../../packages/views/src/index.ts', import.meta.url)),
    },
  },
});
