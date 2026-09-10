import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@weknora/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@weknora/contracts': fileURLToPath(new URL('../../packages/contracts/src/index.ts', import.meta.url)),
      '@weknora/domain': fileURLToPath(new URL('../../packages/domain/src/query-key.ts', import.meta.url)),
      '@weknora/domain/scope': fileURLToPath(new URL('../../packages/domain/src/scope.ts', import.meta.url)),
      '@weknora/ui': fileURLToPath(new URL('../../packages/ui/src/index.tsx', import.meta.url)),
    },
  },
});
