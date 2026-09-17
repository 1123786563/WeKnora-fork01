import { createHash, createHmac, timingSafeEqual } from 'node:crypto';
import { constants as fsConstants } from 'node:fs';
import { lstat, open as fsOpen, realpath } from 'node:fs/promises';
import path from 'node:path';
import { Readable } from 'node:stream';
import { BridgeError } from './protocol.ts';

/**
 * The artifact export boundary of the Paseo bridge (W26).
 *
 * A command names an authorized workspace by its opaque server-side reference
 * plus a relative file reference. Host absolute paths are never accepted. The
 * bridge resolves the workspace root itself through a trusted resolver and
 * walks the file reference segment by segment, refusing symlinks at every
 * step (verified with lstat plus an O_NOFOLLOW final open and a post-open
 * realpath containment re-check — not a string startsWith guess), refusing
 * non-regular files (devices, sockets, fifos, directories), and stopping
 * reads that exceed the size cap.
 */

export interface ArtifactExportCommand {
  commandID: string;
  runID: string;
  targetID: string;
  workspaceRef: string;
  /** Relative path inside the authorized workspace root. */
  fileRef: string;
  versionID: string;
  expiresAt: number;
}

export interface ArtifactExportResult {
  fileRef: string;
  size: number;
  mime: string;
  /** sha256 hex digest of the exported bytes. */
  digest: string;
}

export interface ArtifactExportStream {
  stream: Readable;
  summary: { fileRef: string; size: number; mime: string };
}

export interface ArtifactExportOptions {
  maxBytes?: number;
  signal?: AbortSignal;
  now?: number;
}

/** Upload capabilities only ever grant PUT. */
export type UploadCapabilityMethod = 'PUT';

export interface UploadCapabilityRequest {
  objectKey: string;
  method: UploadCapabilityMethod;
  maxBytes: number;
  expiresAt: number;
}

export interface SignedUploadCapability extends UploadCapabilityRequest {
  serviceIdentity: string;
  signature: string;
}

export interface UploadCapabilityVerifier {
  serviceIdentity: string;
  signingSecret: string;
}

export interface UploadCapabilityCheck {
  objectKey: string;
  method: string;
  contentLength?: number;
}

/** Default read ceiling for one exported artifact (512 MiB). */
export const DEFAULT_MAX_ARTIFACT_BYTES = 512 * 1024 * 1024;

const READ_CHUNK = 1024 * 1024;

const MIME_BY_EXTENSION: Record<string, string> = {
  '.csv': 'text/csv',
  '.txt': 'text/plain',
  '.md': 'text/plain',
  '.json': 'application/json',
  '.pdf': 'application/pdf',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.gif': 'image/gif',
  '.webp': 'image/webp',
  '.svg': 'image/svg+xml',
  '.zip': 'application/zip',
};

function mimeForFileRef(fileRef: string): string {
  return MIME_BY_EXTENSION[path.extname(fileRef).toLowerCase()] ?? 'application/octet-stream';
}

export function validateArtifactExportCommand(command: ArtifactExportCommand): void {
  const fields: Array<keyof ArtifactExportCommand> = ['commandID', 'runID', 'targetID', 'workspaceRef', 'fileRef', 'versionID'];
  for (const field of fields) {
    if (typeof command[field] !== 'string' || command[field].trim() === '') {
      throw new BridgeError('INVALID_COMMAND', `missing ${field}`);
    }
  }
  if (!Number.isFinite(command.expiresAt) || command.expiresAt <= 0) {
    throw new BridgeError('INVALID_COMMAND', 'invalid expiry');
  }
  if ([command.commandID, command.runID, command.targetID, command.workspaceRef, command.versionID].some(value => value.length > 512)) {
    throw new BridgeError('INVALID_COMMAND', 'identifier exceeds size limit');
  }
  if (command.fileRef.length > 1024) {
    throw new BridgeError('INVALID_COMMAND', 'file reference exceeds size limit');
  }
}

/**
 * Split a relative file reference into path segments. Absolute paths, null
 * bytes, backslashes, drive letters, and `.`/`..`/empty segments are
 * malformed and rejected before the filesystem is touched. Percent-encoded
 * names such as `%2e%2e` are treated literally: the bridge never URL-decodes.
 */
export function parseFileRef(fileRef: string): string[] {
  if (fileRef === '' || fileRef.includes('\0') || fileRef.includes('\\')) {
    throw new BridgeError('INVALID_COMMAND', 'malformed file reference');
  }
  if (fileRef.startsWith('/') || /^[A-Za-z]:/.test(fileRef)) {
    throw new BridgeError('INVALID_COMMAND', 'file reference must be relative');
  }
  const segments = fileRef.split('/');
  for (const segment of segments) {
    if (segment === '' || segment === '.' || segment === '..') {
      throw new BridgeError('INVALID_COMMAND', 'malformed file reference segment');
    }
  }
  return segments;
}

interface OpenedArtifact {
  handle: Awaited<ReturnType<typeof fsOpen>>;
  absolutePath: string;
  size: number;
  mime: string;
}

async function resolveArtifactRoot(command: ArtifactExportCommand, resolveRoot: (ref: string) => Promise<string>): Promise<string> {
  let root: string;
  try {
    root = await resolveRoot(command.workspaceRef);
  } catch {
    throw new BridgeError('WORKSPACE_FORBIDDEN');
  }
  if (typeof root !== 'string' || root === '' || root.includes('\0') || !root.startsWith('/') || root.includes('\\')) {
    throw new BridgeError('WORKSPACE_FORBIDDEN');
  }
  return root;
}

/**
 * Open the referenced file entirely inside the authorized root. Every path
 * segment is checked with lstat (any symlink escapes), the final component is
 * opened O_NOFOLLOW, the opened descriptor is re-verified to be a regular
 * file, and realpath containment is re-checked after the open so a swapped
 * intermediate directory cannot smuggle the read outside the root.
 */
async function openArtifactFile(command: ArtifactExportCommand, resolveRoot: (ref: string) => Promise<string>, maxBytes: number): Promise<OpenedArtifact> {
  const segments = parseFileRef(command.fileRef);
  let current = await resolveArtifactRoot(command, resolveRoot);
  for (let i = 0; i < segments.length; i++) {
    current = path.join(current, segments[i]);
    let stat: Awaited<ReturnType<typeof lstat>>;
    try {
      stat = await lstat(current);
    } catch {
      throw new BridgeError('ARTIFACT_NOT_FOUND', current === command.fileRef ? undefined : 'segment missing');
    }
    if (stat.isSymbolicLink()) {
      throw new BridgeError('ARTIFACT_ESCAPE', 'symlink in export boundary');
    }
    const isLast = i === segments.length - 1;
    if (!isLast && !stat.isDirectory()) {
      throw new BridgeError('ARTIFACT_NOT_FOUND', 'path segment is not a directory');
    }
    if (isLast && !stat.isFile()) {
      throw new BridgeError('ARTIFACT_NOT_REGULAR');
    }
  }
  let handle: Awaited<ReturnType<typeof fsOpen>>;
  try {
    handle = await fsOpen(current, fsConstants.O_RDONLY | fsConstants.O_NOFOLLOW);
  } catch (error) {
    const code = (error as NodeJS.ErrnoException)?.code;
    if (code === 'ELOOP') throw new BridgeError('ARTIFACT_ESCAPE', 'symlink in export boundary');
    if (code === 'ENOENT' || code === 'ENOTDIR') throw new BridgeError('ARTIFACT_NOT_FOUND');
    throw new BridgeError('ARTIFACT_UNREADABLE', String(code ?? 'open failed'));
  }
  try {
    const opened = await handle.stat();
    if (!opened.isFile()) {
      throw new BridgeError('ARTIFACT_NOT_REGULAR');
    }
    if (opened.size > maxBytes) {
      throw new BridgeError('ARTIFACT_TOO_LARGE', `size ${opened.size} exceeds cap ${maxBytes}`);
    }
    const [realPath, realRoot] = await Promise.all([realpath(current), realpath(await resolveArtifactRoot(command, resolveRoot))]);
    if (realPath !== realRoot && !realPath.startsWith(realRoot + path.sep)) {
      throw new BridgeError('ARTIFACT_ESCAPE', 'resolved path left the workspace root');
    }
    return { handle, absolutePath: current, size: opened.size, mime: mimeForFileRef(command.fileRef) };
  } catch (error) {
    await handle.close().catch(() => {});
    throw error;
  }
}

function exportOptions(now: number | undefined, maxBytes: number | undefined, signal: AbortSignal | undefined): { now: () => number; maxBytes: number } {
  if (signal?.aborted) throw new BridgeError('BRIDGE_CANCELLED');
  const cap = maxBytes ?? DEFAULT_MAX_ARTIFACT_BYTES;
  if (!Number.isSafeInteger(cap) || cap <= 0) throw new BridgeError('INVALID_COMMAND', 'invalid maxBytes');
  return { now: () => (now ?? Date.now()), maxBytes: cap };
}

/** Export one artifact: validate the boundary, then read and digest the file. */
export async function exportArtifact(
  command: ArtifactExportCommand,
  resolveRoot: (ref: string) => Promise<string>,
  options: ArtifactExportOptions = {},
): Promise<ArtifactExportResult> {
  validateArtifactExportCommand(command);
  const { now, maxBytes } = exportOptions(options.now, options.maxBytes, options.signal);
  if (command.expiresAt <= now()) throw new BridgeError('COMMAND_EXPIRED');
  const opened = await openArtifactFile(command, resolveRoot, maxBytes);
  try {
    const hash = createHash('sha256');
    const buffer = Buffer.alloc(Math.min(READ_CHUNK, maxBytes + 1));
    let total = 0;
    for (;;) {
      const { bytesRead } = await opened.handle.read(buffer, 0, buffer.length, null);
      if (bytesRead === 0) break;
      total += bytesRead;
      if (total > maxBytes) throw new BridgeError('ARTIFACT_TOO_LARGE', 'read exceeded the size cap');
      hash.update(buffer.subarray(0, bytesRead));
    }
    if (total !== opened.size) throw new BridgeError('ARTIFACT_TOO_LARGE', 'file changed size during read');
    return { fileRef: command.fileRef, size: opened.size, mime: opened.mime, digest: hash.digest('hex') };
  } finally {
    await opened.handle.close().catch(() => {});
  }
}

/**
 * Open a bounded readable stream for one artifact. The stream reads from the
 * already-validated descriptor, so replacing the file on disk mid-stream does
 * not change the served bytes; reading is lazy (nothing is pulled until the
 * consumer attaches) and exceeding the cap destroys the stream with
 * ARTIFACT_TOO_LARGE instead of streaming unbounded data.
 */
export async function openArtifactStream(
  command: ArtifactExportCommand,
  resolveRoot: (ref: string) => Promise<string>,
  options: ArtifactExportOptions = {},
): Promise<ArtifactExportStream> {
  validateArtifactExportCommand(command);
  const { now, maxBytes } = exportOptions(options.now, options.maxBytes, options.signal);
  if (command.expiresAt <= now()) throw new BridgeError('COMMAND_EXPIRED');
  const opened = await openArtifactFile(command, resolveRoot, maxBytes);
  let delivered = 0;
  const stream = new Readable({
    read() {
      void (async () => {
        try {
          const buffer = Buffer.alloc(Math.min(READ_CHUNK, maxBytes + 1));
          const { bytesRead } = await opened.handle.read(buffer, 0, buffer.length, null);
          if (bytesRead === 0) {
            stream.push(null);
            return;
          }
          delivered += bytesRead;
          if (delivered > maxBytes) {
            stream.destroy(new BridgeError('ARTIFACT_TOO_LARGE', 'read exceeded the size cap'));
            return;
          }
          stream.push(buffer.subarray(0, bytesRead));
        } catch (error) {
          stream.destroy(error instanceof Error ? error : new BridgeError('ARTIFACT_UNREADABLE'));
        }
      })();
    },
  });
  stream.on('close', () => {
    void opened.handle.close().catch(() => {});
  });
  return { stream, summary: { fileRef: command.fileRef, size: opened.size, mime: opened.mime } };
}

function canonicalCapability(capability: UploadCapabilityRequest): string {
  return JSON.stringify({
    objectKey: capability.objectKey, method: capability.method, maxBytes: capability.maxBytes, expiresAt: capability.expiresAt,
  });
}

function capabilitySignature(capability: UploadCapabilityRequest, secret: string): string {
  return `hmac-sha256:${createHmac('sha256', secret).update(canonicalCapability(capability)).digest('hex')}`;
}

/**
 * Mint the upload capability for one object key. The capability binds a single
 * key, the PUT method, a byte ceiling, and an expiry; it is the signed URL the
 * remote producer uploads against.
 */
export function mintUploadCapability(
  capability: UploadCapabilityRequest,
  serviceIdentity: string,
  signingSecret: string,
): SignedUploadCapability {
  if (capability.method !== 'PUT') throw new BridgeError('INVALID_CAPABILITY', 'only PUT may be granted');
  if (typeof capability.objectKey !== 'string' || capability.objectKey === '' || capability.objectKey.includes('\0')) {
    throw new BridgeError('INVALID_CAPABILITY', 'invalid object key');
  }
  if (!Number.isSafeInteger(capability.maxBytes) || capability.maxBytes <= 0) {
    throw new BridgeError('INVALID_CAPABILITY', 'invalid maxBytes');
  }
  if (!Number.isFinite(capability.expiresAt) || capability.expiresAt <= 0) {
    throw new BridgeError('INVALID_CAPABILITY', 'invalid expiry');
  }
  if (serviceIdentity === '' || signingSecret === '') {
    throw new BridgeError('INVALID_CAPABILITY', 'service identity and secret are required');
  }
  return { ...capability, serviceIdentity, signature: capabilitySignature(capability, signingSecret) };
}

/**
 * Verify an upload capability against a concrete upload request. Signature
 * mismatches are checked first so expired forgeries still fail as forbidden.
 */
export function verifyUploadCapability(
  signed: SignedUploadCapability,
  verifier: UploadCapabilityVerifier,
  request: UploadCapabilityCheck,
  now: number,
): void {
  if (signed.serviceIdentity !== verifier.serviceIdentity) throw new BridgeError('CAPABILITY_FORBIDDEN', 'unknown service identity');
  const expected = capabilitySignature(signed, verifier.signingSecret);
  const a = Buffer.from(signed.signature ?? '');
  const b = Buffer.from(expected);
  if (a.length !== b.length || !timingSafeEqual(a, b)) throw new BridgeError('CAPABILITY_FORBIDDEN', 'signature mismatch');
  if (!Number.isFinite(now) || signed.expiresAt <= now) throw new BridgeError('CAPABILITY_EXPIRED');
  if (request.objectKey !== signed.objectKey) throw new BridgeError('CAPABILITY_KEY_MISMATCH');
  if (request.method !== signed.method || signed.method !== 'PUT') throw new BridgeError('CAPABILITY_METHOD_MISMATCH');
  if (request.contentLength !== undefined && request.contentLength > signed.maxBytes) throw new BridgeError('CAPABILITY_TOO_LARGE');
}
