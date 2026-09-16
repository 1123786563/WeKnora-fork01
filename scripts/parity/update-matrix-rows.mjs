#!/usr/bin/env node
// Update status + gap cells for specific matrix rows.
import { readFileSync, writeFileSync } from 'node:fs';
const path = 'docs/migrations/react/vue-react-parity-matrix.md';
const updates = JSON.parse(readFileSync(process.argv[2], 'utf8'));
const lines = readFileSync(path, 'utf8').split('\n');
outer: for (const [rowId, { status, gap }] of Object.entries(updates)) {
  for (let i = 0; i < lines.length; i++) {
    if (!lines[i].startsWith('| ' + rowId + ' | ')) continue;
    const cells = lines[i].split(' | ');
    // cells[cells.length-1] ends with ' |' (trailing); status is second-to-last
    const last = cells.length - 1;
    cells[last - 1] = status;
    cells[last] = gap + ' |';
    lines[i] = cells.join(' | ');
    console.log('updated', rowId);
    continue outer;
  }
  console.log('ROW NOT FOUND:', rowId);
}
writeFileSync(path, lines.join('\n'));
