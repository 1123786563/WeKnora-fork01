import test from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

function tsFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return name === 'node_modules' ? [] : tsFiles(full);
    return /\.(ts|tsx)$/.test(name) && !/\.test\./.test(name) ? [full] : [];
  });
}

test('a source guard finds no known mobile terminal input wiring markers (supplemental T25 #55 AC2 evidence)', () => {
  for (const file of tsFiles(here)) {
    const source = readFileSync(file, 'utf8');
    // This string scan is a supplemental guard, not exhaustive proof. The API
    // client's packages/api-client/src/sandbox/terminal-surface.test.ts separately
    // asserts that the only method exposed by that adapter is issueTicket.
    for (const marker of [
      'createSandboxTerminalApi', 'issueTicket', 'sandbox/terminal-ticket',
      'WebSocket', 'terminalSocket', 'createTerminalWebSocket', 'socket.send(',
      'sendInput(', 'writeInput(', 'sendTerminalInput(', 'writeTerminalInput(',
      'terminal-input', '/terminal/input',
    ]) {
      assert.equal(source.includes(marker), false, `${file} must not reference known terminal input marker ${marker}`);
    }
  }
});

test('the composition wires the delivery recovery interface (T25 #55 AC1)', () => {
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /createDeliveryRecovery/, 'composition must wire the delivery recovery module');
  assert.match(composition, /activeDeliveryRecovery/, 'composition must expose activeDeliveryRecovery for the detail route');
});
