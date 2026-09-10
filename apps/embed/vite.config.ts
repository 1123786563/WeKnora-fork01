import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@weknora/api-client/embed': fileURLToPath(new URL('../../packages/api-client/src/embed/client.ts', import.meta.url)),
      '@weknora/api-client/transport': fileURLToPath(new URL('../../packages/api-client/src/transport/json.ts', import.meta.url)),
      '@weknora/contracts': fileURLToPath(new URL('../../packages/contracts/src/index.ts', import.meta.url)),
      '@weknora/views': fileURLToPath(new URL('../../packages/views/src/index.ts', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
