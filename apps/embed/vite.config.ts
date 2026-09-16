import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url';

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // The Embed entry is copied below web/embed/ for Lite and served behind
  // /embed/. Keeping the base explicit prevents it from resolving assets
  // against the main Web SPA.
  base: '/embed/',
  resolve: {
    alias: {
      '@weknora/api-client/embed': fileURLToPath(new URL('../../packages/api-client/src/embed/client.ts', import.meta.url)),
      '@weknora/api-client/transport': fileURLToPath(new URL('../../packages/api-client/src/transport/json.ts', import.meta.url)),
      '@weknora/contracts': fileURLToPath(new URL('../../packages/contracts/src/index.ts', import.meta.url)),
      '@weknora/views/embed/bridge': fileURLToPath(new URL('../../packages/views/src/embed/bridge.ts', import.meta.url)),
      '@weknora/views/chat/markdown': fileURLToPath(new URL('../../packages/views/src/chat/markdown.ts', import.meta.url)),
      '@weknora/views': fileURLToPath(new URL('../../packages/views/src/index.ts', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
