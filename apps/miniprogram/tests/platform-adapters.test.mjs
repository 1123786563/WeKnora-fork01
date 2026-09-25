import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const transport = await import('../src/platform/transport.ts');
const credentialStore = await import('../src/platform/credential-store.ts');
const channels = await import('../src/platform/authorized-channels.ts');
const intentLog = await import('../src/platform/intent-log.ts');

const memoryStore = () => {
  const map = new Map();
  return { read: k => map.get(k), write: (k, v) => map.set(k, v), remove: k => map.delete(k), keys: () => [...map.keys()] };
};

test('credential store: runtime write/read/clear roundtrip on the canonical key', async () => {
  const store = memoryStore();
  const cs = credentialStore.createTaroCredentialStore(store, 'https://api.example.test');
  assert.equal(await cs.read('https://api.example.test'), undefined, 'empty store reads undefined');
  await cs.write('https://api.example.test', { token: 't1', refreshToken: 'r1' });
  assert.deepEqual(await cs.read('https://api.example.test'), { token: 't1', refreshToken: 'r1' });
  await cs.clear('https://api.example.test');
  assert.equal(await cs.read('https://api.example.test'), undefined);
  assert.equal(store.keys().includes('wk:auth:https://api.example.test'), false, 'clear removes the key');
});

test('credential store: just-in-time readers see a rotation immediately (single writer is the runtime)', async () => {
  const store = memoryStore();
  const cs = credentialStore.createTaroCredentialStore(store, 'https://api.example.test');
  await cs.write('https://api.example.test', { token: 't1', refreshToken: 'r1' });
  assert.equal(credentialStore.readStoredCredential(store, 'https://api.example.test').token, 't1');
  // Runtime 轮换（refresh 后 persistCredential 写回）
  await cs.write('https://api.example.test', { token: 't2', refreshToken: 'r2' });
  assert.equal(credentialStore.readStoredCredential(store, 'https://api.example.test').token, 't2');
});

test('credential store: legacy bearer shape and host-case variants are adopted exactly once (D7 semantics)', () => {
  const store = memoryStore();
  store.write('wk:auth:https://API.example.test', { kind: 'bearer', accessToken: 'legacy', refreshToken: 'legacy-r' });
  store.write('wk:auth:https://api.example.test ', { kind: 'garbage' });
  store.write('wk:auth:https://other.example.test', { kind: 'bearer', accessToken: 'other', refreshToken: 'other-r' });
  credentialStore.adoptLegacyCredentials(store, 'https://api.example.test');
  assert.deepEqual(credentialStore.readStoredCredential(store, 'https://api.example.test'), { token: 'legacy', refreshToken: 'legacy-r' });
  assert.equal(store.keys().includes('wk:auth:https://API.example.test'), false, 'case variant removed after adoption');
  assert.equal(store.keys().includes('wk:auth:https://api.example.test '), false, 'whitespace-malformed variant key is folded away, never left behind (final-review fix F4)');
  assert.deepEqual(credentialStore.readStoredCredential(store, 'https://other.example.test'), { token: 'other', refreshToken: 'other-r' }, 'other origin untouched');
});

// 最终审查修复 F4 回归：normalizeApiOrigin 不归一空白，尾随/前导空白畸形变体键原先被当作
// 不同 origin 原样残留——收养比较前先折叠空白，畸形变体与大小写变体同语义（收养+移除）。
test('credential store: whitespace-malformed variant keys are adopted and removed like any variant', () => {
  const store = memoryStore();
  store.write('wk:auth:https://api.example.test ', { kind: 'bearer', accessToken: 'ws', refreshToken: 'ws-r' });
  credentialStore.adoptLegacyCredentials(store, 'https://api.example.test');
  assert.deepEqual(credentialStore.readStoredCredential(store, 'https://api.example.test'), { token: 'ws', refreshToken: 'ws-r' }, 'valid credential under a whitespace-malformed key is adopted');
  assert.equal(store.keys().includes('wk:auth:https://api.example.test '), false, 'malformed key removed after adoption');
  // canonical 已有凭据时：畸形变体只移除、不覆盖（与大小写变体同一纪律）。
  const store2 = memoryStore();
  store2.write('wk:auth:https://api.example.test', { token: 'canonical', refreshToken: 'canonical-r' });
  store2.write('wk:auth:https://api.example.test ', { kind: 'bearer', accessToken: 'stale', refreshToken: 'stale-r' });
  credentialStore.adoptLegacyCredentials(store2, 'https://api.example.test');
  assert.deepEqual(credentialStore.readStoredCredential(store2, 'https://api.example.test'), { token: 'canonical', refreshToken: 'canonical-r' }, 'canonical credential is never overwritten by a malformed variant');
  assert.equal(store2.keys().includes('wk:auth:https://api.example.test '), false);
});

test('authorized stream channel: pre-stream non-2xx rejects with a real ApiError shape (runtime 401 retry depends on it)', async () => {
  const network = {
    request(options) { return stub.dispatch('request', options); },
    uploadFile(options) { return stub.dispatch('uploadFile', options); },
  };
  stub.reset();
  stub.use(call => {
    if (call.kind !== 'request') { call.options.fail({ errMsg: 'unexpected' }); return; }
    if (call.options.enableChunked) { stub.emitHeaders(call, {}, 401); return; }
    call.options.fail({ errMsg: 'unexpected route' });
  });
  const stream = channels.createAuthorizedStreamChannel(network, 'https://api.example.test');
  await assert.rejects(
    stream({ method: 'GET', path: '/api/v1/workbench/executions/run-1/events' }, 'tok', () => {}),
    error => error.name === 'ApiError' && error.status === 401 && error.code === 'HTTP_401',
  );
});

test('authorized stream channel: chunks are forwarded as decoded text', async () => {
  const network = { request: options => stub.dispatch('request', options), uploadFile: options => stub.dispatch('uploadFile', options) };
  stub.reset();
  stub.use(call => {
    stub.emitHeaders(call, { 'Content-Type': 'text/event-stream; charset=utf-8' }, 200);
    stub.emitChunk(call, new TextEncoder().encode('data: x\n\n').buffer);
    stub.succeed(call, { statusCode: 200, header: { 'Content-Type': 'text/event-stream' }, data: '' });
  });
  const stream = channels.createAuthorizedStreamChannel(network, 'https://api.example.test');
  let text = '';
  await stream({ method: 'GET', path: '/api/v1/workbench/executions/run-1/events' }, 'tok', chunk => { text += chunk; });
  assert.equal(text, 'data: x\n\n');
  assert.equal(stub.lastCall('request').options.header.authorization, 'Bearer tok');
  assert.equal(stub.lastCall('request').options.header.accept, 'text/event-stream');
});

test('blob fetch: refuses non-http(s) schemes and fetches arraybuffer bytes over the network seam', async () => {
  const network = { request: options => stub.dispatch('request', options), uploadFile: options => stub.dispatch('uploadFile', options) };
  const blob = channels.createTaroBlobFetch(network);
  await assert.rejects(blob.fetch('ftp://api.example.test/file.bin'), /BLOB_FETCH/);
  stub.reset();
  stub.use(call => {
    assert.equal(call.options.responseType, 'arraybuffer');
    stub.succeed(call, { statusCode: 200, header: { 'content-type': 'text/plain' }, data: new TextEncoder().encode('hello').buffer });
  });
  const result = await blob.fetch('https://cdn.example.test/file.txt');
  assert.equal(new TextDecoder().decode(result.bytes), 'hello');
  assert.equal(result.mime, 'text/plain');
});

test('intent log: save/load/listScope/remove roundtrip; corrupt storage reads as empty', async () => {
  const store = memoryStore();
  const log = intentLog.createTaroIntentLog(store);
  const scope = { origin: 'https://api.example.test', tenantID: '1', userID: 'u1' };
  const record = { requestId: 'req-1', sessionId: 's-1', goal: { text: '写周报', agentId: 'a1', budgetUpper: 100 }, scope, persistedAt: '2026-09-24T00:00:00Z' };
  await log.save(record);
  assert.deepEqual(await log.load('req-1'), record);
  assert.equal((await log.listScope(scope)).length, 1);
  assert.equal((await log.listScope({ ...scope, tenantID: '2' })).length, 0, 'scope mismatch is filtered out');
  await log.save({ ...record, requestId: 'req-1', sessionId: 's-2' });
  assert.equal((await log.listScope(scope)).length, 1, 'same requestId overwrites, never duplicates');
  await log.remove('req-1');
  assert.equal(await log.load('req-1'), undefined);
  store.write('wk:mini:intents.v1', 'not-json-object');
  assert.deepEqual(await log.listScope(scope), [], 'corrupt storage reads as an empty table');
});
