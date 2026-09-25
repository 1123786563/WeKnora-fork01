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
  const refuseV4 = (a, b) => {
    if (a === 127 || a === 10 || a === 0 || a >= 224) throw new Error('loopback/private/reserved IPv4 is not allowed');
    if (a === 172 && b >= 16 && b <= 31) throw new Error('private IPv4 is not allowed');
    if (a === 192 && b === 168) throw new Error('private IPv4 is not allowed');
    if (a === 169 && b === 254) throw new Error('link-local IPv4 is not allowed');
    if (a === 100 && b >= 64 && b <= 127) throw new Error('CGNAT IPv4 is not allowed');
  };
  if (v4) refuseV4(Number(v4[1]), Number(v4[2]));
  const lower = host.toLowerCase();
  // fc/fd（ULA fc00::/7）与 fe80（链路本地 fe80::/10 全距 fe[89ab]）只对 IPv6 字面量判定：
  // 以这些前缀开头的公网域名（fda.gov 等）是合法主机，不得按前缀巧合误拒（最终审查修复 F1）。
  if (host.includes(':')) {
    if (lower === '::1' || /^f[cd]/.test(lower) || /^fe[89ab]/.test(lower)) throw new Error('IPv6 loopback/ULA/link-local is not allowed');
    // v4-mapped（::ffff:0:0/96）：WHATWG URL 把 ::ffff:a.b.c.d 规范化为十六进制
    //（::ffff:10.0.0.1 → ::ffff:a00:1），按内嵌 IPv4 的同一规则判定，不得借道字面量绕过私网检查。
    const mapped = ipv4OfV4Mapped(lower);
    if (mapped) refuseV4(mapped[0], mapped[1]);
  }
}

/** 展开规范化 IPv6 字面量为 8 段 hextext；v4-mapped 返回其内嵌 IPv4 的四字节，否则 undefined。 */
function ipv4OfV4Mapped(literal) {
  const sections = literal.split('::');
  if (sections.length > 2) return undefined;
  const head = sections[0] === '' ? [] : sections[0].split(':');
  const tail = sections.length === 2 ? (sections[1] === '' ? [] : sections[1].split(':')) : [];
  if (head.some(part => /^[0-9a-f]{1,4}$/.test(part) === false) || tail.some(part => /^[0-9a-f]{1,4}$/.test(part) === false)) return undefined;
  const fill = 8 - head.length - tail.length;
  if (fill < 0 || (sections.length === 1 && fill !== 0)) return undefined;
  const pieces = [...head, ...Array(fill).fill('0'), ...tail].map(part => parseInt(part, 16));
  if (pieces.slice(0, 5).every(piece => piece === 0) && pieces[5] === 0xffff) {
    return [pieces[6] >> 8, pieces[6] & 0xff, pieces[7] >> 8, pieces[7] & 0xff];
  }
  return undefined;
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

// 最终审查修复 F1 回归：fc/fd/fe80 前缀检查必须限定 IPv6 字面量——以这些前缀开头的
// 合法公网域名（fda.gov 等）不得被误拒。纯校验器检查，不依赖部署环境，永不 skip。
test('deployment origin guard: prefix-colliding public domains are admitted; loopback/private targets are still refused', () => {
  const allowed = origin => assertAllowedDeploymentOrigin(origin);
  const refused = origin => assert.throws(() => assertAllowedDeploymentOrigin(origin), error => error instanceof Error, origin);
  // 以 fc/fd/fe80 开头的公网域名：前缀巧合不得触发 IPv6 私有段拒绝
  allowed('https://fda.gov/');
  allowed('https://fcbarcelona.example/');
  allowed('https://fe80systems.example/');
  allowed('https://FDA.gov/');
  allowed('https://example.com/');
  // 私网/环回/链路本地：HTTPS 字面量仍拒绝
  refused('http://fda.gov/');
  refused('https://localhost/');
  refused('https://api.example.test/path');
  refused('https://127.0.0.1/');
  refused('https://10.1.2.3/');
  refused('https://172.16.0.1/');
  refused('https://192.168.1.1/');
  refused('https://169.254.3.4/');
  refused('https://100.64.0.1/');
  refused('https://0.0.0.0/');
  refused('https://224.0.0.1/');
  refused('https://[::1]/');
  refused('https://[fc00::1]/');
  refused('https://[FD12:3456::1]/');
  refused('https://[fe80::1]/');
  refused('https://[febf::1]/', 'fe80::/10 上界（fe[89ab]）同属链路本地');
  refused('https://[::ffff:10.0.0.1]/', 'v4-mapped 必须按其内嵌 v4 规则拒绝（WHATWG 规范化为 ::ffff:a00:1 后仍命中）');
  allowed('https://[::ffff:8.8.8.8]/');
});

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
