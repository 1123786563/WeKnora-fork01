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

// T25 (#55) AC2：移动装配图结构性无终端输入通道——src 源树（除测试）零命中
// terminal 门票签发与 WS 终端装配词。移动端的终端面只有材料域的只读投影
// （MATERIAL_TERMINAL_READ_ONLY，task-material.ts:227）。
test('the mobile source tree wires no terminal ticket or interactive terminal channel (T25 #55 AC2)', () => {
  for (const file of tsFiles(here)) {
    const source = readFileSync(file, 'utf8');
    for (const marker of ['createSandboxTerminalApi', 'issueTicket', 'sandbox/terminal-ticket']) {
      assert.equal(source.includes(marker), false, `${file} must not reference ${marker}: the mobile terminal is read-only by construction`);
    }
  }
});

test('the composition wires the delivery recovery interface (T25 #55 AC1)', () => {
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /createDeliveryRecovery/, 'composition must wire the delivery recovery module');
  assert.match(composition, /activeDeliveryRecovery/, 'composition must expose activeDeliveryRecovery for the detail route');
});
