const REQUIRED = ['id', 'route', 'source', 'interaction', 'destination', 'service', 'task'];
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
const STATUSES = new Set(['pending', 'accepted', 'blocked']);

export function fixedRoutePaths(upstream, commit) {
  return execFileSync('git', ['-C', upstream, 'ls-tree', '-r', '--name-only', commit, 'packages/happy-app/sources/app/(app)'], { encoding: 'utf8' })
    .trim().split('\n').filter(Boolean).filter(p => !p.includes('/dev/'))
    .map(p => p.replace('packages/happy-app/', ''));
}

export function validateCommittedInventory({ upstream, commit, matrixPath }) {
  const matrix = JSON.parse(fs.readFileSync(matrixPath, 'utf8'));
  if (matrix.commit !== commit) throw new Error(`matrix commit mismatch: ${matrix.commit}`);
  checkInventory(matrix.rows, fixedRoutePaths(upstream, commit));
  return matrix.rows.length;
}

export function checkInventory(rows, routes) {
  if (!Array.isArray(rows) || !Array.isArray(routes)) throw new TypeError('rows and routes must be arrays');
  const ids = new Set();
  for (const row of rows) {
    if (ids.has(row.id)) throw new Error(`duplicate ${row.id}`);
    ids.add(row.id);
    for (const key of REQUIRED) {
      if (typeof row[key] !== 'string' || !row[key].trim()) throw new Error(`missing ${key}`);
    }
    if (!STATUSES.has(row.status)) throw new Error('invalid status');
  }
  for (const route of routes) {
    if (!rows.some(row => row.route === route)) throw new Error(`unmapped ${route}`);
  }
}
