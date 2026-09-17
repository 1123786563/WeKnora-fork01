/**
 * W25 — session attachment upload, cancel and validation.
 *
 * The upload port reuses the existing session attachments route
 * (POST /api/v1/sessions/:session_id/attachments). Native content:// and
 * file:// URIs are resolved to bytes by the platform's local file port
 * (HttpTransport.sendMultipartFile) and re-uploaded as a multipart file
 * part, so the device-local path never travels to the backend as data.
 *
 * Every client-side check here (size, allowed types) is a UX-layer gate
 * only; the server re-validates request body size, real MIME, digest and
 * session ownership on every upload.
 */

/** The file the user picked, shot or received through a system share. */
export interface UploadInput {
  sessionID: string;
  /** Device-local URI (content:// on Android, file:// or ph:// elsewhere). */
  uri: string;
  name: string;
  mime: string;
  size: number;
  /**
   * Compressed local preview used for UI chips only. The original `uri`
   * always stays the upload source — a thumbnail must never be mistaken
   * for the original file.
   */
  previewUri?: string;
  previewMime?: string;
}

export function validateUpload(f: UploadInput, limit: number) {
  if (!Number.isSafeInteger(f.size) || f.size < 0 || !Number.isSafeInteger(limit) || limit <= 0) throw new Error('INVALID_UPLOAD');
  if (f.size > limit) throw new Error('UPLOAD_TOO_LARGE');
  if (!f.sessionID || !f.name || !f.uri) throw new Error('INVALID_UPLOAD');
}

/** Deployment-derived upload capability. UX layer only; the server re-checks. */
export interface UploadLimits {
  maxBytes: number;
  allowedMimes?: readonly string[];
}

/** Mirrors the backend default (MAX_FILE_SIZE_MB=50). Deployments override. */
export const defaultUploadLimits: UploadLimits = { maxBytes: 50 * 1024 * 1024 };

/** Multipart file request handled by the platform local file port. */
export interface UploadFileRequest {
  method: string;
  url: string;
  headers: Record<string, string>;
  file: { uri: string; name: string; type: string; size?: number };
  fields: Record<string, string>;
  signal?: AbortSignal;
}

export interface UploadResultLike { status: number; body?: unknown }

/**
 * Structural subset of the api-client HttpTransport. ProductAuthSession's
 * transport satisfies it, so uploads share the auth session's bearer and
 * refresh coordinator.
 */
export interface UploadTransport {
  sendMultipartFile?(request: UploadFileRequest): Promise<UploadResultLike>;
  send?(request: { method: string; url: string; headers: Record<string, string>; body?: unknown; signal?: AbortSignal }): Promise<UploadResultLike>;
}

/** Structural subset of ProductScope (capture/accept) for scope invalidation. */
export interface UploadScope {
  capture(): { generation: number; signal: AbortSignal };
  accept(generation: number): boolean;
}

export interface UploadPort {
  send(input: UploadInput, signal: AbortSignal): Promise<{ attachmentID: string }>;
}

export function createSessionUploadPort(deps: { baseURL: string; transport: UploadTransport; limits: UploadLimits }): UploadPort {
  const base = deps.baseURL.replace(/\/+$/, '');
  return {
    async send(input, signal) {
      if (signal.aborted) throw new Error('CANCELLED');
      validateUpload(input, deps.limits.maxBytes);
      if (deps.limits.allowedMimes && !deps.limits.allowedMimes.includes(input.mime)) {
        throw new Error('UPLOAD_TYPE_NOT_ALLOWED');
      }
      const sendMultipart = deps.transport.sendMultipartFile;
      if (!sendMultipart) throw new Error('UPLOAD_TRANSPORT_UNSUPPORTED');
      const result = await sendMultipart({
        method: 'POST',
        url: `${base}/api/v1/sessions/${encodeURIComponent(input.sessionID)}/attachments`,
        headers: { accept: 'application/json' },
        file: { uri: input.uri, name: input.name, type: input.mime, size: input.size },
        fields: {},
        signal,
      });
      // A signal that aborted after the response arrived is intentionally NOT
      // re-checked here: the attachment already exists server-side, and the
      // manager must see the late result so it can invalidate and clean it up.
      if (result.status < 200 || result.status >= 300) throw new Error(`UPLOAD_HTTP_${result.status}`);
      const root = result.body;
      if (!root || typeof root !== 'object' || Array.isArray(root)) throw new Error('UPLOAD_RESPONSE_INVALID');
      const envelope = root as { success?: unknown; data?: unknown };
      if (envelope.success !== true || !envelope.data || typeof envelope.data !== 'object' || Array.isArray(envelope.data)) {
        throw new Error('UPLOAD_RESPONSE_INVALID');
      }
      const data = envelope.data as { id?: unknown };
      if (typeof data.id !== 'string' || data.id.trim() === '') throw new Error('UPLOAD_RESPONSE_INVALID');
      return { attachmentID: data.id };
    },
  };
}

/** Best-effort server-side delete of an attachment (late/cancelled results). */
export function createSessionAttachmentRemover(deps: { baseURL: string; transport: UploadTransport }) {
  const base = deps.baseURL.replace(/\/+$/, '');
  return async (sessionID: string, attachmentID: string, signal: AbortSignal): Promise<void> => {
    if (!deps.transport.send) return;
    const result = await deps.transport.send({
      method: 'DELETE',
      url: `${base}/api/v1/sessions/${encodeURIComponent(sessionID)}/attachments/${encodeURIComponent(attachmentID)}`,
      headers: { accept: 'application/json' },
      signal,
    }).catch(() => undefined);
    if (result && result.status >= 400) return; // best-effort only
  };
}

// ---------------------------------------------------------------------------
// Upload manager: in-flight tracking, cancel, failure retention and scope
// invalidation.
// ---------------------------------------------------------------------------

export type SessionUploadStatus = 'uploading' | 'uploaded' | 'failed';

export interface SessionUploadRecord {
  id: string;
  input: UploadInput;
  status: SessionUploadStatus;
  attachmentID?: string;
  error?: string;
}

export interface SessionUploads {
  records(): SessionUploadRecord[];
  /** Completed attachments in attachment order, ready to reference in a send. */
  ready(): { attachmentID: string; name: string }[];
  attach(input: UploadInput): Promise<void>;
  /** Aborts an in-flight upload and clears its unfinished record. */
  cancel(id: string): void;
  /** Drops a terminal record (e.g. a failed upload the user dismissed). */
  remove(id: string): void;
  subscribe(listener: () => void): () => void;
}

export function createUploadRecordID(): string {
  if (typeof globalThis.crypto?.randomUUID === 'function') return `mobile-upload:${globalThis.crypto.randomUUID()}`;
  return `mobile-upload:${Date.now().toString(36)}:${Math.random().toString(36).slice(2)}`;
}

export function createSessionUploads(deps: {
  port: UploadPort;
  scope: UploadScope;
  removeAttachment?: (attachmentID: string, signal: AbortSignal) => Promise<void>;
}): SessionUploads {
  const records = new Map<string, { record: SessionUploadRecord; controller?: AbortController; captured: number }>();
  const listeners = new Set<() => void>();
  const notify = () => listeners.forEach((listener) => listener());
  return {
    records: () => [...records.values()].map((entry) => entry.record),
    ready: () => [...records.values()].map((entry) => entry.record).filter((record) => record.status === 'uploaded' && record.attachmentID)
      .map((record) => ({ attachmentID: record.attachmentID as string, name: record.input.name })),
    async attach(input) {
      const id = createUploadRecordID();
      const controller = new AbortController();
      const captured = deps.scope.capture();
      const onScopeAbort = () => controller.abort();
      captured.signal.addEventListener('abort', onScopeAbort, { once: true });
      const record: SessionUploadRecord = { id, input, status: 'uploading' };
      const entry = { record, controller, captured: captured.generation };
      records.set(id, entry);
      notify();
      try {
        const result = await deps.port.send(input, controller.signal);
        if (!deps.scope.accept(captured.generation)) {
          // A space switch invalidated the destination: the late result must
          // not surface, and the already-created attachment is deleted so it
          // does not linger as an orphan in the old session.
          records.delete(id);
          notify();
          await deps.removeAttachment?.(result.attachmentID, new AbortController().signal);
          return;
        }
        entry.record.status = 'uploaded';
        entry.record.attachmentID = result.attachmentID;
        notify();
      } catch (error) {
        if (controller.signal.aborted) {
          // User cancel or scope switch aborted the request: unfinished
          // records are cleaned up and the cancellation propagates.
          records.delete(id);
          notify();
          throw new Error('CANCELLED');
        }
        entry.record.status = 'failed';
        entry.record.error = error instanceof Error ? error.message : 'UPLOAD_FAILED';
        notify();
        throw error instanceof Error ? error : new Error('UPLOAD_FAILED');
      } finally {
        captured.signal.removeEventListener('abort', onScopeAbort);
      }
    },
    cancel(id) {
      const entry = records.get(id);
      if (!entry) return;
      entry.controller?.abort();
      records.delete(id);
      notify();
    },
    remove(id) {
      if (records.delete(id)) notify();
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
}

// ---------------------------------------------------------------------------
// Native entry seams: input-box picker, camera and system share. The real
// native adapters (expo-document-picker / camera / share intents) plug in
// here; device-level behaviour is covered by blocked-env manual evidence.
// ---------------------------------------------------------------------------

export interface AttachmentCandidate {
  uri: string;
  name: string;
  mime: string;
  size: number;
  previewUri?: string;
  previewMime?: string;
}

export interface NativeAttachmentSource {
  /** null means the user dismissed the picker without a selection. */
  pickDocuments(): Promise<AttachmentCandidate[] | null>;
  capturePhoto(): Promise<AttachmentCandidate | null>;
  /** Consumes one file handed over through the OS share sheet, if any. */
  consumeSharedFile(): Promise<AttachmentCandidate | null>;
}

export interface AttachmentEntryActions {
  chooseFromLibrary(): Promise<void>;
  takePhoto(): Promise<void>;
  acceptSharedFile(): Promise<void>;
}

export function createAttachmentEntryActions(deps: {
  source: NativeAttachmentSource;
  uploads: Pick<SessionUploads, 'attach'>;
  sessionID: string;
}): AttachmentEntryActions {
  const attachAll = async (candidates: AttachmentCandidate[] | null): Promise<void> => {
    if (!candidates) return;
    for (const candidate of candidates) {
      // One failed attachment must not abandon the rest of a multi-pick, and
      // cancellations are user intent, not errors: the manager's records
      // carry the per-attachment outcome for the UI.
      await deps.uploads.attach({ ...candidate, sessionID: deps.sessionID }).catch(() => undefined);
    }
  };
  return {
    async chooseFromLibrary() { await attachAll(await deps.source.pickDocuments()); },
    async takePhoto() {
      const photo = await deps.source.capturePhoto();
      await attachAll(photo ? [photo] : null);
    },
    async acceptSharedFile() {
      const shared = await deps.source.consumeSharedFile();
      await attachAll(shared ? [shared] : null);
    },
  };
}
