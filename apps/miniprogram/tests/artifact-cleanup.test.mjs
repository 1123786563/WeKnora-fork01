import test from 'node:test';
import assert from 'node:assert/strict';
import { registerHooks } from 'node:module';
import { pathToFileURL } from 'node:url';

// D3（docs/plans/issue-140/task-6-live-validation.md）：openProtectedDocument 直接
// unlinkSync 下载临时文件（http://tmp/…），真实 DevTools permission denied，403 字节
// 临时文件事后仍存在，而 UI 承诺“临时打开后立即清理”。本文件在替身复现该平台契约
// （见 helpers/taro-stub.mjs 的 isRuntimeTempPath）后，验证受保护产物打开的文件生命周期。
const stubURL = pathToFileURL(new URL('./helpers/taro-stub.mjs', import.meta.url).pathname).href;
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === '@tarojs/taro') return { url: stubURL, shortCircuit: true };
    return nextResolve(specifier, context);
  },
});
globalThis.__API_ORIGIN__ = 'https://api.example.test';
const { stub } = await import('./helpers/taro-stub.mjs');
const runtime = await import('../src/services/runtime.ts');
const files = await import('../src/platform/files.ts');

const ORIGIN = 'https://api.example.test';
const TEMP_PDF = 'http://tmp/wkdl-report.pdf';

function freshLogin() {
  stub.reset();
  const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
  stub.use(call => {
    if (call.kind === 'request') {
      const path = new URL(call.options.url).pathname;
      if (path === '/api/v1/auth/login') { stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }); return; }
      if (path === '/api/v1/auth/me') { stub.succeed(call, { data: me() }); return; }
      call.options.fail({ errMsg: `no route for ${path}` });
      return;
    }
    call.options.success({ statusCode: 200, tempFilePath: TEMP_PDF, header: {} });
  });
  stub.state.fileContents.set(TEMP_PDF, 'PDF-CONTENT');
  return runtime.auth.login('u@example.test', 'pw');
}
const DOWNLOAD = '/api/v1/workbench/artifacts/download?signature=grant-1';

test('D3: 受保护产物从可写的私有副本打开，且副本在打开后立即删除', async () => {
  await freshLogin();
  await files.openProtectedDocument(DOWNLOAD, 'report.pdf', true);
  assert.equal(stub.state.copies.length, 1, '必须先把下载临时文件复制进可写目录');
  const { srcPath, destPath } = stub.state.copies[0];
  assert.equal(srcPath, TEMP_PDF, '复制源必须是 downloadFile 落下的临时文件');
  assert.ok(destPath.startsWith('wxfile://usr/'), `副本必须位于 USER_DATA_PATH（当前：${destPath}）`);
  assert.ok(!isRuntimeTemp(destPath), '副本不能仍在运行时临时目录（那里 unlink 被 deny）');
  assert.deepEqual(stub.state.openedDocuments, [{ filePath: destPath, showMenu: true }], 'openDocument 必须打开私有副本');
  assert.deepEqual(stub.state.removedFiles, [destPath], '打开结束后副本必须被删除');
  assert.equal(stub.state.fileContents.has(destPath), false, '副本内容必须真的被清掉');
  assert.equal(stub.state.fileContents.has(TEMP_PDF), true, '运行时临时文件不由应用管理（我们不承诺清理它）');
});

test('D3: 预览失败时私有副本仍被清理，原始错误不被清理失败掩盖', async () => {
  await freshLogin();
  stub.state.openDocumentError = Object.assign(new Error('openDocument:fail'), { errMsg: 'openDocument:fail file type not supported' });
  await assert.rejects(files.openProtectedDocument(DOWNLOAD, 'report.pdf'), /openDocument/);
  assert.equal(stub.state.removedFiles.length, 1, '预览失败也要删除私有副本');
});

test('D3: 副本清理失败必须如实报错，而不是静默违背“立即清理”的承诺', async () => {
  await freshLogin();
  stub.state.unlinkError = { errMsg: 'unlink:fail no such file or directory' };
  await assert.rejects(files.openProtectedDocument(DOWNLOAD, 'report.pdf'), error => {
    assert.match(error instanceof Error ? error.message : String(error), /清理/, '清理失败必须抛出可读错误');
    return true;
  });
  assert.equal(stub.state.openedDocuments.length, 1, '文档确实已打开（清理失败发生在打开之后）');
});

test('D3: 授权过期（401）的下载不产生任何私有副本，也没有可清理对象', async () => {
  stub.reset();
  const me = () => ({ success: true, data: { user: { id: 'u1', username: 'Lin' }, tenant: { id: 1, name: 'Space' }, memberships: [] } });
  stub.use(call => {
    if (call.kind === 'request') {
      const path = new URL(call.options.url).pathname;
      if (path === '/api/v1/auth/login') { stub.succeed(call, { data: { success: true, data: { token: 't1', refresh_token: 'r1' } } }); return; }
      if (path === '/api/v1/auth/me') { stub.succeed(call, { data: me() }); return; }
      call.options.fail({ errMsg: `no route for ${path}` });
      return;
    }
    stub.state.fileContents.set(TEMP_PDF, JSON.stringify({ success: false, code: 'artifact_grant_expired' }));
    call.options.success({ statusCode: 401, tempFilePath: TEMP_PDF, header: {} });
  });
  await runtime.auth.login('u@example.test', 'pw');
  await assert.rejects(files.openProtectedDocument(DOWNLOAD, 'report.pdf'), error => error.code === 'ARTIFACT_GRANT_EXPIRED');
  assert.equal(stub.state.copies.length, 0, '失败的下载不应创建私有副本');
  assert.deepEqual(stub.state.removedFiles, [], '没有任何应用管理的文件需要清理');
});

function isRuntimeTemp(p) { return p.startsWith('http://tmp/') || p.startsWith('wxfile://tmp') || p.startsWith('/tmp/'); }
