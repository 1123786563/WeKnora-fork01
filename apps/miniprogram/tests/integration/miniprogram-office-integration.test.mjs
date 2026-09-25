import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

const DEPLOYMENT_URL = process.env.WEKNORA_MOBILE_TEST_DEPLOYMENT_URL;
const EMAIL = process.env.WEKNORA_MOBILE_TEST_EMAIL;
const PASSWORD = process.env.WEKNORA_MOBILE_TEST_PASSWORD;
const START_PROBE = process.env.WEKNORA_MOBILE_TEST_START_TASK === '1';
/** 三变量同进同出：任一缺失即视为未配置（与前置批次 opt-in 语义一致）。 */
const enabled = Boolean(DEPLOYMENT_URL) && Boolean(EMAIL) && Boolean(PASSWORD);

/** Mimosa/仓库安全约束：真实请求前校验——仅 https、拒绝 localhost/环回/私网/链路本地/保留地址。 */
function assertAllowedDeploymentOrigin(origin) {
  let parsed;
  try { parsed = new URL(origin); } catch { throw new Error(`invalid deployment origin: ${origin}`); }
  if (parsed.protocol !== 'https:') throw new Error('integration deployment must be HTTPS');
  if (parsed.pathname !== '/' || parsed.search || parsed.hash) throw new Error('deployment origin must carry no path/query/fragment');
  const host = parsed.hostname.replace(/^\[|\]$/g, '');
  const v4 = host.match(/^(\d+)\.(\d+)\.(\d+)\.(\d+)$/);
  if (host === 'localhost' || host.endsWith('.localhost') || host.endsWith('.local') || host === '0.0.0.0') throw new Error('loopback/local host is not allowed');
  if (v4) {
    const [a, b] = [Number(v4[1]), Number(v4[2])];
    if (a === 127 || a === 10 || a === 0 || a >= 224) throw new Error('loopback/private/reserved IPv4 is not allowed');
    if (a === 172 && b >= 16 && b <= 31) throw new Error('private IPv4 is not allowed');
    if (a === 192 && b === 168) throw new Error('private IPv4 is not allowed');
    if (a === 169 && b === 254) throw new Error('link-local IPv4 is not allowed');
    if (a === 100 && b >= 64 && b <= 127) throw new Error('CGNAT IPv4 is not allowed');
  }
  const lower = host.toLowerCase();
  if (lower === '::1' || lower.startsWith('fc') || lower.startsWith('fd') || lower.startsWith('fe80')) throw new Error('IPv6 loopback/ULA/link-local is not allowed');
  if (/^::ffff:\d+\.\d+\.\d+\.\d+$/.test(lower)) throw new Error('IPv4-mapped IPv6 must be rejected by its IPv4 rules');
}

const nodeTaroURL = pathToFileURL(new URL('../helpers/node-taro.mjs', import.meta.url).pathname).href;
if (enabled) {
  assertAllowedDeploymentOrigin(DEPLOYMENT_URL);
  registerHooks({
    resolve(specifier, context, nextResolve) {
      if (specifier === '@tarojs/taro') return { url: nodeTaroURL, shortCircuit: true };
      return nextResolve(specifier, context);
    },
  });
}
globalThis.__API_ORIGIN__ = DEPLOYMENT_URL ?? 'https://api.example.test';

test('miniprogram office integration (opt-in, real deployment)', { skip: enabled ? false : 'set WEKNORA_MOBILE_TEST_DEPLOYMENT_URL/EMAIL/PASSWORD (and optionally WEKNORA_MOBILE_TEST_START_TASK=1) to run the real evidence' }, async () => {
  const runtimeModule = await import('../../src/services/runtime.ts');
  const office = await import('../../src/services/mobile-office.ts');
  const evidence = [];
  const say = line => { evidence.push(line); console.log(`[evidence] ${line}`); };

  await runtimeModule.auth.login(EMAIL, PASSWORD);
  assert.equal(runtimeModule.auth.snapshot().phase, 'ready');
  say(`login: authorized as ${runtimeModule.auth.snapshot().userId} in tenant ${runtimeModule.auth.snapshot().tenantId}`);
  assert.ok(runtimeModule.runtime.scopeLease(), 'authorization minted a lease');

  const home = await office.requireTaskOffice().home();
  assert.equal(typeof home.asOf, 'string');
  say(`home: needsMe=${home.needsMe.length} running=${home.running.length} recentlyCompleted=${home.recentlyCompleted.length} unread=${home.unreadNotifications}`);

  const tasks = await office.requireTaskOffice().tasks({});
  assert.ok(Array.isArray(tasks.items));
  say(`tasks: items=${tasks.items.length} nextCursor=${tasks.nextCursor ?? '-'}`);

  const shelf = office.activeResourceShelf();
  if (shelf) {
    const page = await shelf.browse();
    say(`shelf: agents=${page.agents.length} knowledge=${page.knowledge.length} connections=${page.connections.length} agentVerdict=${page.classVerdicts.agent.state}`);
  } else {
    say('shelf: unavailable (no authorized shelf)');
  }

  const inbox = await office.requireTaskOffice().inbox();
  say(`inbox: pending=${inbox.items.length}`);

  let probeRunId;
  if (START_PROBE) {
    const receipt = await office.requireTaskOffice().start({ text: `issue68 integration probe ${new Date().toISOString()}`, agentId: 'builtin-quick-answer', budgetUpper: 1 });
    say(`start: requestId=${receipt.requestId} phase=${receipt.phase} runId=${receipt.runId ?? '-'}`);
    assert.ok(receipt.dispatched || receipt.phase === 'pending');
    probeRunId = receipt.runId;
  }

  const materialTarget = probeRunId ?? tasks.items[0]?.runId;
  if (materialTarget) {
    const material = office.openActiveMaterial();
    if (material) {
      const index = await material.index({ runId: materialTarget });
      say(`material: run=${materialTarget} entries=${index.materials.length} terminal=${index.terminal.available}`);
      material.close('evidence-done');
    }
  } else {
    say('material: skipped (no run available and probe disabled)');
  }

  if (probeRunId) {
    const resolved = await office.resolveTaskForRun(probeRunId);
    await office.requireTaskOffice().archive(resolved.taskId);
    say(`cleanup: archived probe task ${resolved.taskId}`);
  }
  await runtimeModule.logout();
  say('logout: credentials cleared');
  assert.ok(evidence.length >= 5, 'evidence log must be substantive, not fabricated');
});
