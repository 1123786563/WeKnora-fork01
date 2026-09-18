import test from 'node:test';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { chmod, mkdir, mkdtemp, readFile, writeFile, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { Readable } from 'node:stream';
import {
  ArtifactExportCommand,
  DEFAULT_MAX_ARTIFACT_BYTES,
  exportArtifact,
  mintUploadCapability,
  openArtifactStream,
  verifyUploadCapability,
} from './artifact-export.ts';
import { BridgeError } from './protocol.ts';

const command = (patch: Partial<ArtifactExportCommand> = {}): ArtifactExportCommand => ({
  commandID: 'c', runID: 'r', targetID: 'n', workspaceRef: 'w',
  fileRef: 'reports/q3.csv', versionID: 'v1', expiresAt: Date.now() + 60_000, ...patch,
});

/** Match on the stable BridgeError code, not the human-readable message. */
const codeIs = (code: string) => (e: unknown): boolean => e instanceof BridgeError && e.code === code;

interface Root {
  root: string;
  outside: string;
}

async function makeRoot(): Promise<Root> {
  const base = await mkdtemp(path.join(tmpdir(), 'artifact-export-'));
  const root = path.join(base, 'root');
  const outside = path.join(base, 'root-outside'); // shares the string prefix of root
  await mkdir(path.join(root, 'reports'), { recursive: true });
  await mkdir(outside, { recursive: true });
  return { root, outside };
}

async function consume(stream: Readable): Promise<{ bytes: number; error?: Error }> {
  let bytes = 0;
  let error: Error | undefined;
  await new Promise<void>(resolve => {
    stream.on('data', (chunk: Buffer) => { bytes += chunk.length; });
    stream.on('error', err => { error = err as Error; });
    stream.on('end', () => resolve());
    stream.on('close', () => resolve());
    stream.on('errored', () => resolve());
  });
  return { bytes, error };
}

test('exports a nested relative file and returns its digest and size', async () => {
  const { root } = await makeRoot();
  const content = Buffer.from('quarter,net\n3,42\n');
  await writeFile(path.join(root, 'reports/q3.csv'), content);
  const result = await exportArtifact(command(), async () => root, { maxBytes: 1024 });
  assert.equal(result.size, content.length);
  assert.equal(result.mime, 'text/csv');
  assert.equal(result.digest, createHash('sha256').update(content).digest('hex'));
  // The same bytes stream out of openArtifactStream.
  const opened = await openArtifactStream(command(), async () => root, { maxBytes: 1024 });
  const collected: Buffer[] = [];
  for await (const chunk of opened.stream) collected.push(chunk as Buffer);
  assert.deepEqual(Buffer.concat(collected), content);
});

test('unicode and space filenames export; encoded traversal names stay literal', async () => {
  const { root } = await makeRoot();
  const weird = path.join(root, 'reports', 'ä ö 名 %2e%2e %2f.txt');
  await writeFile(weird, 'literal-name');
  const fileRef = 'reports/ä ö 名 %2e%2e %2f.txt';
  const result = await exportArtifact(command({ fileRef }), async () => root);
  assert.equal(result.size, 'literal-name'.length);
  // '%2e%2e' is a legal literal filename; the bridge never URL-decodes it.
  await assert.rejects(exportArtifact(command({ fileRef: 'reports/%2e%2e' }), async () => root), codeIs('ARTIFACT_NOT_FOUND'));
});

test('absolute paths, traversal segments, separators and null bytes never reach the filesystem', async () => {
  const { root } = await makeRoot();
  let resolverCalls = 0;
  const resolveRoot = async (ref: string): Promise<string> => { resolverCalls++; assert.equal(ref, 'w'); return root; };
  const bad = ['/etc/passwd', 'C:/win/path', 'a\\b', 'a\0b', '../outside.txt', 'reports/../../outside.txt', 'reports/./q3.csv', 'reports//q3.csv', ''];
  for (const fileRef of bad) {
    await assert.rejects(
      exportArtifact(command({ fileRef }), resolveRoot),
      (e: unknown) => e instanceof BridgeError && ['INVALID_COMMAND', 'ARTIFACT_ESCAPE'].includes(e.code),
      `fileRef ${JSON.stringify(fileRef)} must be rejected`,
    );
  }
  // Absolute traversal is rejected at command shape, before any resolver work.
  await assert.rejects(exportArtifact(command({ fileRef: '/etc/passwd' }), resolveRoot), codeIs('INVALID_COMMAND'));
  assert.equal(resolverCalls, 0, 'malformed file references must be rejected before the resolver runs');
});

test('real symlink escapes are rejected even when the string prefix matches the root', async () => {
  const { root, outside } = await makeRoot();
  const secret = path.join(outside, 'secret.txt');
  await writeFile(secret, 'SECRET');
  // The outside directory name shares the root's string prefix, so a naive
  // startsWith(root) check would pass this escape.
  await symlink(secret, path.join(root, 'reports', 'link.txt'));
  // Final-component symlink.
  await assert.rejects(
    exportArtifact(command({ fileRef: 'reports/link.txt' }), async () => root),
    (e: unknown) => e instanceof BridgeError && e.code === 'ARTIFACT_ESCAPE',
  );
  // Intermediate-directory symlink to a directory outside the root.
  await mkdir(path.join(outside, 'data'), { recursive: true });
  await writeFile(path.join(outside, 'data', 'file.txt'), 'OUTSIDE');
  await symlink(path.join(outside, 'data'), path.join(root, 'data-link'));
  await assert.rejects(
    exportArtifact(command({ fileRef: 'data-link/file.txt' }), async () => root),
    (e: unknown) => e instanceof BridgeError && e.code === 'ARTIFACT_ESCAPE',
  );
  // Even a symlink that stays inside the root is rejected: the export
  // boundary does not follow symlinks at all.
  await writeFile(path.join(root, 'inside.txt'), 'INSIDE');
  await symlink(path.join(root, 'inside.txt'), path.join(root, 'reports', 'in-link.txt'));
  await assert.rejects(
    exportArtifact(command({ fileRef: 'reports/in-link.txt' }), async () => root),
    (e: unknown) => e instanceof BridgeError && e.code === 'ARTIFACT_ESCAPE',
  );
});

test('directories and special files (fifo/device) are not regular files', async () => {
  const { root } = await makeRoot();
  await assert.rejects(
    exportArtifact(command({ fileRef: 'reports' }), async () => root),
    (e: unknown) => e instanceof BridgeError && e.code === 'ARTIFACT_NOT_REGULAR',
  );
  const fifo = path.join(root, 'fifo');
  try {
    execFileSync('mkfifo', [fifo]);
  } catch {
    console.warn('mkfifo unavailable: skipping fifo device negative');
    return;
  }
  await assert.rejects(
    exportArtifact(command({ fileRef: 'fifo' }), async () => root),
    (e: unknown) => e instanceof BridgeError && e.code === 'ARTIFACT_NOT_REGULAR',
  );
});

test('missing files read as not found', async () => {
  const { root } = await makeRoot();
  await assert.rejects(exportArtifact(command(), async () => root), codeIs('ARTIFACT_NOT_FOUND'));
});

test('replacing the file after open does not change the streamed bytes', async () => {
  const { root } = await makeRoot();
  const target = path.join(root, 'reports', 'q3.csv');
  await writeFile(target, 'ORIGINAL');
  const opened = await openArtifactStream(command(), async () => root);
  await rm(target);
  await writeFile(target, 'REPLACED');
  const collected: Buffer[] = [];
  for await (const chunk of opened.stream) collected.push(chunk as Buffer);
  assert.equal(Buffer.concat(collected).toString(), 'ORIGINAL');
});

test('oversized files are rejected and reading stops at the cap', async () => {
  const { root } = await makeRoot();
  const target = path.join(root, 'reports', 'q3.csv');
  const body = Buffer.alloc(6 * 1024 * 1024, 7);
  await writeFile(target, body);
  const options = { maxBytes: 1024 * 1024 };
  // The open-time size check rejects the digest export...
  await assert.rejects(exportArtifact(command(), async () => root, options), codeIs('ARTIFACT_TOO_LARGE'));
  // ...and the streaming export alike.
  await assert.rejects(openArtifactStream(command(), async () => root, options), codeIs('ARTIFACT_TOO_LARGE'));

  // A file that was under the cap at open time but grows past it during the
  // read must stop at the cap instead of streaming unbounded bytes. Reading
  // is lazy, so nothing is pulled before the consumer attaches.
  const growing = path.join(root, 'reports', 'growing.csv');
  await writeFile(growing, Buffer.alloc(512 * 1024, 1));
  const opened = await openArtifactStream(command({ fileRef: 'reports/growing.csv' }), async () => root, options);
  await writeFile(growing, Buffer.alloc(2 * 1024 * 1024, 2), { flag: 'a' });
  const { bytes, error } = await consume(opened.stream);
  assert.ok(error instanceof BridgeError && error.code === 'ARTIFACT_TOO_LARGE', 'stream must fail with ARTIFACT_TOO_LARGE');
  assert.ok(bytes <= options.maxBytes, `read ${bytes} bytes, cap is ${options.maxBytes}`);
});

test('unreadable permissions fail closed', async () => {
  const { root } = await makeRoot();
  const target = path.join(root, 'reports', 'q3.csv');
  await writeFile(target, 'x');
  await chmod(target, 0o000);
  await assert.rejects(exportArtifact(command(), async () => root), (e: unknown) => {
    assert.ok(!(e instanceof BridgeError && e.code === 'ARTIFACT_NOT_FOUND'));
    return true;
  });
});

test('expired export commands and forbidden roots never touch the filesystem', async () => {
  const { root } = await makeRoot();
  let resolverCalls = 0;
  const resolveRoot = async (): Promise<string> => { resolverCalls++; return root; };
  await assert.rejects(exportArtifact(command({ expiresAt: 10 }), resolveRoot, { now: 20 }), codeIs('COMMAND_EXPIRED'));
  assert.equal(resolverCalls, 0, 'expired command must not reach the resolver');
  let throwingCalls = 0;
  await assert.rejects(
    exportArtifact(command(), async () => { throwingCalls++; throw new Error('not owned'); }),
    codeIs('WORKSPACE_FORBIDDEN'),
  );
  await assert.rejects(exportArtifact(command(), async () => 'relative/path'), codeIs('WORKSPACE_FORBIDDEN'));
  assert.equal(throwingCalls, 1);
});

test('command shape is validated: versionID and identifiers are required', async () => {
  const { root } = await makeRoot();
  const resolveRoot = async () => root;
  for (const patch of [{ versionID: '' } as Partial<ArtifactExportCommand>, { runID: '' }, { targetID: '' }, { workspaceRef: '' }, { fileRef: '' }, { commandID: '' }]) {
    await assert.rejects(exportArtifact(command(patch), resolveRoot), codeIs('INVALID_COMMAND'));
  }
  await assert.rejects(exportArtifact(command({ fileRef: 'x'.repeat(1025) }), resolveRoot), codeIs('INVALID_COMMAND'));
});

test('upload capability pins key, method, size and expiry', async () => {
  const verifier = { serviceIdentity: 'weknora-execution', signingSecret: 'secret', authorizationVersion: 1 };
  const cap = { objectKey: 'artifact-versions/1/r1/' + 'a'.repeat(64), method: 'PUT' as const, maxBytes: 4096, expiresAt: Date.now() + 60_000 };
  const signed = mintUploadCapability(cap, verifier.serviceIdentity, verifier.signingSecret);
  verifyUploadCapability(signed, verifier, { objectKey: cap.objectKey, method: 'PUT', contentLength: 100 }, Date.now());
  // Wrong key.
  assert.throws(() => verifyUploadCapability(signed, verifier, { objectKey: 'artifact-versions/1/r1/' + 'b'.repeat(64), method: 'PUT' }, Date.now()), codeIs('CAPABILITY_KEY_MISMATCH'));
  // Wrong method.
  assert.throws(() => verifyUploadCapability(signed, verifier, { objectKey: cap.objectKey, method: 'GET' }, Date.now()), codeIs('CAPABILITY_METHOD_MISMATCH'));
  // Oversized body.
  assert.throws(() => verifyUploadCapability(signed, verifier, { objectKey: cap.objectKey, method: 'PUT', contentLength: 4097 }, Date.now()), codeIs('CAPABILITY_TOO_LARGE'));
  // Expired (signed URL past its deadline).
  assert.throws(() => verifyUploadCapability(signed, verifier, { objectKey: cap.objectKey, method: 'PUT' }, cap.expiresAt + 1), codeIs('CAPABILITY_EXPIRED'));
  // Tampered signature.
  assert.throws(
    () => verifyUploadCapability({ ...signed, signature: 'hmac-sha256:tampered' }, verifier, { objectKey: cap.objectKey, method: 'PUT' }, Date.now()),
    codeIs('CAPABILITY_FORBIDDEN'),
  );
  // Only PUT may be minted.
  assert.throws(() => mintUploadCapability({ ...cap, method: 'GET' as never }, verifier.serviceIdentity, verifier.signingSecret), codeIs('INVALID_CAPABILITY'));
  // Unknown service identity fails verification.
  assert.throws(
    () => verifyUploadCapability(signed, { ...verifier, serviceIdentity: 'other' }, { objectKey: cap.objectKey, method: 'PUT' }, Date.now()),
    codeIs('CAPABILITY_FORBIDDEN'),
  );
});

test('default max artifact bytes is a sane finite cap', async () => {
  assert.ok(Number.isSafeInteger(DEFAULT_MAX_ARTIFACT_BYTES) && DEFAULT_MAX_ARTIFACT_BYTES > 0);
  const { root } = await makeRoot();
  const content = await readFile(new URL(import.meta.url), 'utf8');
  await writeFile(path.join(root, 'reports', 'q3.csv'), content);
  const result = await exportArtifact(command(), async () => root);
  assert.equal(result.size, Buffer.byteLength(content));
});
