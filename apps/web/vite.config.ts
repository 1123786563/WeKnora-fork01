import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { fileURLToPath, URL } from 'node:url';
import { execSync } from 'node:child_process';
import { readFileSync } from 'node:fs';

// UI 版本 for the system info panel（frontend/vite.config.ts:14-30 parity）。
// T12c：版本号与 Vue 端同源——直接读 frontend/package.json（apps/web 自己的
// package.json version=0.0.0，与 Vue 端 0.8.0 不同源，system 分区前端版本行
// 像素对比因此必差）；commit 短哈希长度也对齐 Vue 端 `--short`（auto）而非
// 固定 8 位，两端在同一 HEAD 启动 dev server 时字符串完全一致。
const FRONTEND_VERSION = (() => {
  try {
    const frontendPkg = JSON.parse(readFileSync(new URL('../../frontend/package.json', import.meta.url), 'utf8')) as { version?: string };
    return frontendPkg.version ?? 'unknown';
  } catch {
    return 'unknown';
  }
})();

function resolveFrontendCommit(): string {
  const fromEnv = process.env.VITE_FRONTEND_COMMIT || process.env.GITHUB_SHA;
  if (fromEnv) {
    // Vue frontend/vite.config.ts:22-25 parity: env-supplied hashes shorten to 7.
    return fromEnv.slice(0, 7);
  }
  try {
    return execSync('git rev-parse --short HEAD', { stdio: ['ignore', 'pipe', 'ignore'] }).toString().trim();
  } catch {
    return 'unknown';
  }
}

const FRONTEND_COMMIT = resolveFrontendCommit();

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
  plugins: [react(), tailwindcss()],
  server: { proxy: { '/api': backendProxy, '/files': backendProxy } },
  preview: { proxy: { '/api': backendProxy, '/files': backendProxy } },
  define: {
    __FRONTEND_VERSION__: JSON.stringify(FRONTEND_VERSION),
    __FRONTEND_COMMIT__: JSON.stringify(FRONTEND_COMMIT),
  },
  resolve: {
    alias: {
      '@weknora/api-client': fileURLToPath(new URL('../../packages/api-client/src/index.ts', import.meta.url)),
      '@weknora/contracts': fileURLToPath(new URL('../../packages/contracts/src/index.ts', import.meta.url)),
      '@weknora/core/craft/controller': fileURLToPath(new URL('../../packages/core/src/craft/controller.ts', import.meta.url)),
      '@weknora/core/craft/command-bridge': fileURLToPath(new URL('../../packages/core/src/craft/command-bridge.ts', import.meta.url)),
      '@weknora/domain/craft/capabilities': fileURLToPath(new URL('../../packages/domain/src/craft/capabilities.ts', import.meta.url)),
      '@weknora/domain/auth/onboarding': fileURLToPath(new URL('../../packages/domain/src/auth/onboarding.ts', import.meta.url)),
      '@weknora/domain/auth/password-policy': fileURLToPath(new URL('../../packages/domain/src/auth/password-policy.ts', import.meta.url)),
      '@weknora/domain/settings/local-preferences': fileURLToPath(new URL('../../packages/domain/src/settings/local-preferences.ts', import.meta.url)),
      '@weknora/domain/settings/theme': fileURLToPath(new URL('../../packages/domain/src/settings/theme.ts', import.meta.url)),
      '@weknora/domain/scope': fileURLToPath(new URL('../../packages/domain/src/scope.ts', import.meta.url)),
      '@weknora/domain/chat/draft': fileURLToPath(new URL('../../packages/domain/src/chat/draft.ts', import.meta.url)),
      '@weknora/domain/chat/reducer': fileURLToPath(new URL('../../packages/domain/src/chat/reducer.ts', import.meta.url)),
      '@weknora/domain/chat/message-extras': fileURLToPath(new URL('../../packages/domain/src/chat/message-extras.ts', import.meta.url)),
      '@weknora/domain/chat/session-state': fileURLToPath(new URL('../../packages/domain/src/chat/session-state.ts', import.meta.url)),
      '@weknora/domain/chat/message-timestamps': fileURLToPath(new URL('../../packages/domain/src/chat/message-timestamps.ts', import.meta.url)),
      '@weknora/domain/chat/copy-answer': fileURLToPath(new URL('../../packages/domain/src/chat/copy-answer.ts', import.meta.url)),
      '@weknora/domain/chat/session-grouping': fileURLToPath(new URL('../../packages/domain/src/chat/session-grouping.ts', import.meta.url)),
      '@weknora/domain/chat/artifacts': fileURLToPath(new URL('../../packages/domain/src/chat/artifacts.ts', import.meta.url)),
      '@weknora/domain/chat/references': fileURLToPath(new URL('../../packages/domain/src/chat/references.ts', import.meta.url)),
      '@weknora/domain/chat/tool-results': fileURLToPath(new URL('../../packages/domain/src/chat/tool-results.ts', import.meta.url)),
      '@weknora/domain/knowledge/folders': fileURLToPath(new URL('../../packages/domain/src/knowledge/folders.ts', import.meta.url)),
      '@weknora/domain/knowledge/processing': fileURLToPath(new URL('../../packages/domain/src/knowledge/processing.ts', import.meta.url)),
      '@weknora/domain/knowledge/preview': fileURLToPath(new URL('../../packages/domain/src/knowledge/preview.ts', import.meta.url)),
      '@weknora/domain/sandbox/terminal': fileURLToPath(new URL('../../packages/domain/src/sandbox/terminal.ts', import.meta.url)),
      '@weknora/domain/sandbox/skill-install': fileURLToPath(new URL('../../packages/domain/src/sandbox/skill-install.ts', import.meta.url)),
      '@weknora/domain/wiki/diff': fileURLToPath(new URL('../../packages/domain/src/wiki/diff.ts', import.meta.url)),
      '@weknora/domain/craft/state': fileURLToPath(new URL('../../packages/domain/src/craft/state.ts', import.meta.url)),
      '@weknora/domain/craft/reconnect': fileURLToPath(new URL('../../packages/domain/src/craft/reconnect.ts', import.meta.url)),
      '@weknora/domain': fileURLToPath(new URL('../../packages/domain/src/query-key.ts', import.meta.url)),
      '@weknora/ui/button': fileURLToPath(new URL('../../packages/ui/src/button.tsx', import.meta.url)),
      '@weknora/ui/checkbox': fileURLToPath(new URL('../../packages/ui/src/checkbox.tsx', import.meta.url)),
      '@weknora/ui/input': fileURLToPath(new URL('../../packages/ui/src/input.tsx', import.meta.url)),
      '@weknora/ui/textarea': fileURLToPath(new URL('../../packages/ui/src/textarea.tsx', import.meta.url)),
      '@weknora/ui': fileURLToPath(new URL('../../packages/ui/src/index.tsx', import.meta.url)),
      '@weknora/views/chat/page': fileURLToPath(new URL('../../packages/views/src/chat/page.tsx', import.meta.url)),
      '@weknora/views/chat/chat-copy': fileURLToPath(new URL('../../packages/views/src/chat/chat-copy.ts', import.meta.url)),
      '@weknora/views/chat/composer': fileURLToPath(new URL('../../packages/views/src/chat/composer.tsx', import.meta.url)),
      '@weknora/views/chat/agent-readiness': fileURLToPath(new URL('../../packages/views/src/chat/agent-readiness.ts', import.meta.url)),
      '@weknora/views/chat/agent-selector': fileURLToPath(new URL('../../packages/views/src/chat/agent-selector.tsx', import.meta.url)),
      '@weknora/views/chat/markdown': fileURLToPath(new URL('../../packages/views/src/chat/markdown.ts', import.meta.url)),
      '@weknora/views/chat/live-thinking': fileURLToPath(new URL('../../packages/views/src/chat/live-thinking.ts', import.meta.url)),
      '@weknora/views/chat/fork-point': fileURLToPath(new URL('../../packages/views/src/chat/fork-point.ts', import.meta.url)),
      '@weknora/views/chat/mermaid': fileURLToPath(new URL('../../packages/views/src/chat/mermaid.ts', import.meta.url)),
      '@weknora/views/chat/web-search': fileURLToPath(new URL('../../packages/views/src/chat/web-search.ts', import.meta.url)),
      '@weknora/views/embed/bridge': fileURLToPath(new URL('../../packages/views/src/embed/bridge.ts', import.meta.url)),
      '@weknora/views/guides/contextual-guides': fileURLToPath(new URL('../../packages/views/src/guides/contextual-guides.ts', import.meta.url)),
      '@weknora/views/integrations/registry': fileURLToPath(new URL('../../packages/views/src/integrations/registry.ts', import.meta.url)),
      '@weknora/views/integrations/page': fileURLToPath(new URL('../../packages/views/src/integrations/page.tsx', import.meta.url)),
      '@weknora/views/integrations/apiKeys': fileURLToPath(new URL('../../packages/views/src/integrations/apiKeys.ts', import.meta.url)),
      '@weknora/views/integrations/settings-route': fileURLToPath(new URL('../../packages/views/src/integrations/settings-route.ts', import.meta.url)),
      '@weknora/views/integrations/swagger': fileURLToPath(new URL('../../packages/views/src/integrations/swagger.ts', import.meta.url)),
      '@weknora/views/settings/registry': fileURLToPath(new URL('../../packages/views/src/settings/registry.ts', import.meta.url)),
      '@weknora/views/craft/home': fileURLToPath(new URL('../../packages/views/src/craft/home.tsx', import.meta.url)),
      '@weknora/views/craft/library': fileURLToPath(new URL('../../packages/views/src/craft/library.tsx', import.meta.url)),
      '@weknora/views/craft/templates': fileURLToPath(new URL('../../packages/views/src/craft/templates.tsx', import.meta.url)),
      '@weknora/views/craft/workbench': fileURLToPath(new URL('../../packages/views/src/craft/workbench.tsx', import.meta.url)),
      '@weknora/views/craft/preview': fileURLToPath(new URL('../../packages/views/src/craft/preview.tsx', import.meta.url)),
      '@weknora/views/craft/files': fileURLToPath(new URL('../../packages/views/src/craft/files.tsx', import.meta.url)),
      '@weknora/views/craft/spreadsheet': fileURLToPath(new URL('../../packages/views/src/craft/spreadsheet.tsx', import.meta.url)),
      '@weknora/views/craft/document': fileURLToPath(new URL('../../packages/views/src/craft/document.tsx', import.meta.url)),
      '@weknora/views/craft/slides': fileURLToPath(new URL('../../packages/views/src/craft/slides.tsx', import.meta.url)),
      '@weknora/views/craft/presentation': fileURLToPath(new URL('../../packages/views/src/craft/presentation.ts', import.meta.url)),
      '@weknora/views/craft/interaction': fileURLToPath(new URL('../../packages/views/src/craft/interaction.tsx', import.meta.url)),
      '@weknora/i18n/runtime': fileURLToPath(new URL('../../packages/i18n/src/runtime.ts', import.meta.url)),
      '@weknora/views': fileURLToPath(new URL('../../packages/views/src/index.ts', import.meta.url)),
    },
  },
});
