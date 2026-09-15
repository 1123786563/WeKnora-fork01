import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath, URL } from 'node:url';

const DEV_PROXY_TARGET =
  process.env.VITE_DEV_PROXY_TARGET ||
  process.env.FRONTEND_BACKEND_URL ||
  'http://localhost:8080';

const backendProxy = {
  target: DEV_PROXY_TARGET,
  changeOrigin: true,
  secure: false,
  ws: true,
};

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': backendProxy,
      '/files': backendProxy,
    },
  },
  preview: {
    proxy: {
      '/api': backendProxy,
      '/files': backendProxy,
    },
  },
  resolve: {
    alias: {
      '@weknora/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@weknora/contracts': fileURLToPath(new URL('../../packages/contracts/src/index.ts', import.meta.url)),
      '@weknora/core/craft/controller': fileURLToPath(new URL('../../packages/core/src/craft/controller.ts', import.meta.url)),
      '@weknora/domain/scope': fileURLToPath(new URL('../../packages/domain/src/scope.ts', import.meta.url)),
      '@weknora/domain/craft/state': fileURLToPath(new URL('../../packages/domain/src/craft/state.ts', import.meta.url)),
      '@weknora/domain/craft/reconnect': fileURLToPath(new URL('../../packages/domain/src/craft/reconnect.ts', import.meta.url)),
      '@weknora/domain': fileURLToPath(new URL('../../packages/domain/src/query-key.ts', import.meta.url)),
      '@weknora/views/craft/home': fileURLToPath(new URL('../../packages/views/src/craft/home.tsx', import.meta.url)),
      '@weknora/views/craft/workbench': fileURLToPath(new URL('../../packages/views/src/craft/workbench.tsx', import.meta.url)),
      '@weknora/views/craft/preview': fileURLToPath(new URL('../../packages/views/src/craft/preview.tsx', import.meta.url)),
      '@weknora/views/craft/files': fileURLToPath(new URL('../../packages/views/src/craft/files.tsx', import.meta.url)),
      '@weknora/views/craft/presentation': fileURLToPath(new URL('../../packages/views/src/craft/presentation.ts', import.meta.url)),
      '@weknora/views/craft/interaction': fileURLToPath(new URL('../../packages/views/src/craft/interaction.tsx', import.meta.url)),
      '@weknora/ui': fileURLToPath(new URL('../../packages/ui/src/index.tsx', import.meta.url)),
    },
  },
});
