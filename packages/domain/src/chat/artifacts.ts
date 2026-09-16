export interface ChatArtifact {
  index: number;
  fileName: string;
  handle?: string;
  fileType?: string;
  fileSize?: number;
  version?: string;
  expiresAt?: string;
  modTime?: string;
  createdAt?: string;
}

type UnknownRecord = Record<string, unknown>;
type ArtifactReference = { kind: 'handle'; value: string } | { kind: 'name'; value: string };

// The backend uses opaque handles whose length may vary by deployment/version.
// Validate the safe handle alphabet, not one observed encoding length.
const RESOURCE_HANDLE = /^resource:\/\/[A-Za-z0-9_-]+$/;
const SANDBOX_REFERENCE = /^sandbox:(?:\/\/)?(.+)$/i;

function record(value: unknown): UnknownRecord | null {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
    ? value as UnknownRecord
    : null;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value.trim() : '';
}

function publicHandle(value: unknown): string | undefined {
  const candidate = text(value);
  return RESOURCE_HANDLE.test(candidate) ? candidate : undefined;
}

function dateTime(value: unknown): string | undefined {
  const candidate = text(value);
  if (!candidate) return undefined;
  const parsed = new Date(candidate);
  return Number.isNaN(parsed.getTime()) ? undefined : parsed.toISOString();
}

function parseArtifactReference(value: unknown): ArtifactReference | null {
  const candidate = text(value);
  if (!candidate) return null;
  if (RESOURCE_HANDLE.test(candidate)) return { kind: 'handle', value: candidate };
  const match = candidate.match(SANDBOX_REFERENCE);
  if (!match) return null;
  let name = match[1]!.trim();
  try { name = decodeURIComponent(name); } catch { /* Keep malformed legacy names readable. */ }
  name = name.split('/').pop()?.trim() ?? '';
  return name ? { kind: 'name', value: name } : null;
}

/** Strip storage URLs and sandbox paths while retaining public artifact identity. */
export function normalizeArtifactMetadata(value: unknown): ChatArtifact | null {
  const input = record(value);
  if (!input) return null;
  const index = input.index;
  const fileName = text(input.file_name ?? input.fileName);
  if (!Number.isInteger(index) || (index as number) < 0 || !fileName) return null;

  const handle = publicHandle(input.handle) ?? publicHandle(input.url);
  const fileType = text(input.file_type ?? input.fileType);
  const size = input.file_size ?? input.fileSize;
  const fileSize = typeof size === 'number' && Number.isFinite(size) && size >= 0 ? size : undefined;
  const rawVersion = input.version;
  const version = typeof rawVersion === 'string' || typeof rawVersion === 'number'
    ? String(rawVersion).trim()
    : '';
  const expiresAt = dateTime(input.expires_at ?? input.expiresAt);
  const modTime = dateTime(input.mod_time ?? input.modTime);
  const createdAt = dateTime(input.created_at ?? input.createdAt);

  return {
    index: index as number,
    ...(handle ? { handle } : {}),
    fileName,
    ...(fileType ? { fileType } : {}),
    ...(fileSize !== undefined ? { fileSize } : {}),
    ...(version ? { version } : {}),
    ...(expiresAt ? { expiresAt } : {}),
    ...(modTime ? { modTime } : {}),
    ...(createdAt ? { createdAt } : {}),
  };
}

export function normalizeArtifactList(values: readonly unknown[] | null | undefined): ChatArtifact[] {
  if (!Array.isArray(values)) return [];
  return values.flatMap((value) => {
    const artifact = normalizeArtifactMetadata(value);
    return artifact ? [artifact] : [];
  });
}

/** Keep server-owned display names safe when passed to a browser or local filesystem sink. */
export function safeArtifactFileName(value: string): string {
  const sanitized = value
    .replace(/[\\/]/g, '_')
    .replace(/[\u0000-\u001f\u007f]/g, '')
    .trim();
  return sanitized || 'artifact';
}

export function resolveArtifactReference(
  reference: unknown,
  artifacts: readonly ChatArtifact[] | null | undefined,
): ChatArtifact | null {
  const parsed = parseArtifactReference(reference);
  if (!parsed || !artifacts) return null;
  if (parsed.kind === 'handle') {
    return artifacts.find((artifact) => artifact.handle === parsed.value) ?? null;
  }
  return artifacts.find((artifact) => artifact.fileName === parsed.value) ?? null;
}

export function artifactDownloadPath(sessionId: string, messageId: string, index: number): string {
  if (!sessionId.trim()) throw new Error('session id must not be empty');
  if (!messageId.trim()) throw new Error('message id must not be empty');
  if (!Number.isInteger(index) || index < 0) throw new Error('artifact index must be a non-negative integer');
  return `/api/v1/sessions/${encodeURIComponent(sessionId)}/messages/${encodeURIComponent(messageId)}/artifacts/${index}/download`;
}

export function isArtifactExpired(artifact: ChatArtifact, now: Date = new Date()): boolean {
  if (!artifact.expiresAt) return false;
  const expiresAt = new Date(artifact.expiresAt).getTime();
  return Number.isFinite(expiresAt) && expiresAt <= now.getTime();
}
