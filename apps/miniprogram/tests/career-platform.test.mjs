import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// 平台边界替换：与 assembly.test.mjs 相同的契约级 Taro 替身（storage 可用）。
const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const platform = await import('../src/adapters/career-platform.ts');

test('P1: readSharedEntry returns the shared JD payload from the entry params', () => {
  const adapter = platform.createCareerPlatform({ entryParams: () => ({ jd: '岗位：Go 工程师\n要求：三年经验', from: 'share' }) });
  const entry = adapter.readSharedEntry();
  assert.ok(entry, 'payload must be present');
  assert.equal(entry.text, '岗位：Go 工程师\n要求：三年经验');
  assert.equal(typeof entry.sourceLabel, 'string');
  assert.ok(entry.sourceLabel.length > 0);
});

test('P2: readSharedEntry reports a missing payload instead of inventing one', () => {
  const adapter = platform.createCareerPlatform({ entryParams: () => ({}) });
  assert.equal(adapter.readSharedEntry(), undefined, 'no demo data for a missing share payload');
  const blank = platform.createCareerPlatform({ entryParams: () => ({ jd: '   ' }) });
  assert.equal(blank.readSharedEntry(), undefined, 'blank payload counts as missing');
});

test('P3: prepareSharedImport builds a verifiable preview without any network write', () => {
  stub.reset();
  const callsBefore = stub.state.calls.length;
  const jd = '职位描述\n'.repeat(60); // 360 字，超过预览截断阈值
  const draft = platform.prepareSharedImport(jd, '微信分享');
  assert.equal(stub.state.calls.length, callsBefore, 'preparing a preview must not touch the network');
  assert.equal(draft.rawText, jd.trim(), 'draft keeps the verbatim payload for submission');
  assert.ok(draft.preview.excerpt.length < jd.length, 'excerpt is truncated for narrow screens');
  assert.ok(draft.preview.excerpt.length > 0);
  assert.equal(draft.preview.fullLength, jd.trim().length);
  assert.equal(draft.sourceLabel, '微信分享');
  assert.ok(typeof draft.requestId === 'string' && draft.requestId.length > 0);
  const short = platform.prepareSharedImport('后端开发', '粘贴');
  assert.equal(short.preview.excerpt, '后端开发');
  assert.equal(short.preview.fullLength, 4);
});

test('P4: prepareSharedImport refuses an empty payload with a recoverable error', () => {
  assert.throws(() => platform.prepareSharedImport('   '), error => error?.code === 'share_payload_missing' && error?.recoverable === true);
});

test('P5: controlled career store is scoped under wk:career: and cleared with the private cache', async () => {
  const { clearPrivateCache } = await import('../src/platform/storage.ts');
  stub.reset();
  const store = platform.createControlledStore();
  assert.throws(() => store.read('wk:other:key'), /wk:career:/, 'store rejects keys outside the controlled prefix');
  store.write('wk:career:probe', { a: 1 });
  assert.deepEqual(store.read('wk:career:probe'), { a: 1 });
  assert.ok([...stub.state.storage.keys()].every(key => key.startsWith('wk:career:')), 'no key escapes the prefix');
  clearPrivateCache();
  assert.equal(store.read('wk:career:probe'), undefined, 'logout clears career-controlled state');
});

test('P6: chooseResume delegates to the injected file seam and forwards the native file', async () => {
  const file = { uri: 'wxfile://tmp-resume.pdf', name: '简历.pdf', type: 'application/pdf', size: 2048 };
  const adapter = platform.createCareerPlatform({ chooseFile: async () => file });
  assert.deepEqual(await adapter.chooseResume(), file);
});

test('P7: percent-encoded share payloads are decoded, literal % text is preserved', () => {
  const adapter = platform.createCareerPlatform({ entryParams: () => ({ jd: encodeURIComponent('岗位：Go 工程师\n要求：三年经验') }) });
  const entry = adapter.readSharedEntry();
  assert.ok(entry, 'encoded payload must yield an entry');
  assert.equal(entry.text, '岗位：Go 工程师\n要求：三年经验', 'percent-encoding is decoded once');
  const literal = platform.createCareerPlatform({ entryParams: () => ({ jd: '增长率 100% 未达成' }) });
  assert.equal(literal.readSharedEntry().text, '增长率 100% 未达成', 'plain text with a bare % is not mangled');
  const broken = platform.createCareerPlatform({ entryParams: () => ({ jd: 'broken %E4%B8' }) });
  assert.equal(broken.readSharedEntry().text, 'broken %E4%B8', 'undecodable text is kept verbatim');
});
