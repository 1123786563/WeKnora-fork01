#!/usr/bin/env node
import { existsSync, readFileSync, readdirSync } from 'node:fs';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('..', import.meta.url));
const sourceExtensions = new Set(['.ts', '.tsx', '.js', '.jsx']);
const failures = [];

function filesUnder(directory) {
  if (!existsSync(directory)) return [];
  const result = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name);
    if (entry.name === 'node_modules' || entry.name === 'dist' || entry.name === 'coverage') continue;
    if (entry.isDirectory()) result.push(...filesUnder(path));
    else if (sourceExtensions.has(path.slice(path.lastIndexOf('.')))) result.push(path);
  }
  return result;
}

function assertNo(pattern, directories, label) {
  for (const directory of directories) {
    for (const path of filesUnder(join(root, directory))) {
      const source = readFileSync(path, 'utf8');
      if (pattern.test(source)) failures.push(`${label}: ${relative(root, path)}`);
    }
  }
}

assertNo(/from\s+['"](?:react|react-dom|vue|pinia)(?:\/|['"])/, ['packages/contracts', 'packages/api-client', 'packages/domain', 'packages/design-tokens', 'packages/i18n'], 'shared package imports a UI runtime');
assertNo(/(?:frontend\/src|\.\.\/\.\.\/frontend)/, ['packages', 'apps'], 'React surface reaches the legacy frontend source');
assertNo(/(?:^|[^A-Za-z])(window|document|localStorage|sessionStorage)\s*[.[]/, ['packages/contracts', 'packages/api-client', 'packages/domain', 'packages/design-tokens', 'packages/i18n'], 'shared package reads a host global');

const dockerIgnore = readFileSync(join(root, '.dockerignore'), 'utf8');
for (const requiredPath of ['frontend/', 'packages/', 'apps/']) {
  if (new RegExp(`^${requiredPath.replace('/', '\\/')}$`, 'm').test(dockerIgnore)) {
    failures.push(`root .dockerignore excludes ${requiredPath}; root-context React/Docker builds cannot see it`);
  }
}

const dockerfile = readFileSync(join(root, 'frontend/Dockerfile'), 'utf8');
if (!dockerfile.includes('COPY frontend/package.json') || !dockerfile.includes('COPY frontend .')) {
  failures.push('frontend/Dockerfile is not root-context compatible');
}

const compose = readFileSync(join(root, 'docker-compose.yml'), 'utf8');
if (!compose.includes('context: .') || !compose.includes('dockerfile: frontend/Dockerfile')) {
  failures.push('docker-compose.yml must use the repository root context with frontend/Dockerfile');
}

const mobileWorkflow = readFileSync(join(root, '.github/workflows/mobile.yml'), 'utf8');
for (const platform of ['ios', 'android']) {
  if (!mobileWorkflow.includes(`expo export --platform ${platform}`)) {
    failures.push(`mobile CI must export the ${platform} JavaScript bundle`);
  }
}

const nginx = readFileSync(join(root, 'frontend/nginx.conf'), 'utf8');
for (const requiredRule of [
  'location /api/',
  'location = /files',
  'location ^~ /embed/assets/',
  'location ^~ /embed/',
  'public, max-age=31536000, immutable',
  'proxy_buffering off',
  'proxy_set_header Upgrade $http_upgrade',
  'try_files $uri $uri/ /index.html',
]) {
  if (!nginx.includes(requiredRule)) failures.push(`frontend/nginx.conf is missing ${requiredRule}`);
}

if (failures.length) {
  console.error(failures.map((failure) => `FAIL ${failure}`).join('\n'));
  process.exitCode = 1;
} else {
  console.log('React boundary checks passed');
}
