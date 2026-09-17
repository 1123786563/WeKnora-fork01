/**
 * W30 product transcription call: the server-side endpoint behind the
 * product authSession. This is the injection that consumes W29's
 * TRANSCRIBE_ENDPOINT_PENDING_W30 seam — the dictation port's `transcribe`
 * gains a real backend.
 *
 * Transport policy follows the W25 upload precedent: the platform local
 * file port carries the multipart capture with the live credential's
 * bearer attached (the auth session does not wrap sendMultipartFile yet),
 * and a rotated credential re-runs this factory (route memo) so a stale
 * bearer fails the upload visibly. The client never holds a provider model
 * key — auth and budget stay server-side behind the product authSession.
 * The optional signal (M-3) aborts a timed-out or cancelled upload.
 */
import { createJsonTransport, type BearerCredential, type ProductAuthSession } from '@weknora/api-client';
import { File } from 'expo-file-system';
import { DictationError } from './dictation';

export interface ProductVoiceTranscriberInput {
  origin: string;
  credential: BearerCredential;
  authSession: ProductAuthSession;
}

interface TranscriptionEnvelope {
  success?: unknown;
  data?: { text?: unknown };
}

function newRequestID(): string {
  const bytes = new Uint8Array(16);
  globalThis.crypto?.getRandomValues?.(bytes);
  let id = '';
  for (const b of bytes) id += b.toString(16).padStart(2, '0');
  return id || `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`;
}

/** Reads the capture size, clamped defensively for a missing stat. */
function fileSizeOf(uri: string): number {
  try {
    const file = new File(uri);
    return file.exists ? Number(file.size) || 0 : 0;
  } catch {
    return 0;
  }
}

export function createProductVoiceTranscriber(input: ProductVoiceTranscriberInput): (uri: string, signal?: AbortSignal) => Promise<string> {
  const localFilePort = createJsonTransport(fetch);
  const base = input.origin.replace(/\/+$/, '');
  return async (uri: string, signal?: AbortSignal): Promise<string> => {
    if (signal?.aborted) throw new DictationError('TRANSCRIBE_FAILED', 'TRANSCRIBE_ABORTED');
    const result = await localFilePort.sendMultipartFile!({
      method: 'POST',
      url: `${base}/api/v1/mobile/voice/transcriptions`,
      headers: { authorization: `Bearer ${input.credential.accessToken}`, accept: 'application/json' },
      file: { uri, name: 'capture.m4a', type: 'audio/mp4', size: fileSizeOf(uri) },
      fields: { request_id: newRequestID() },
      signal,
    });
    if (signal?.aborted) throw new DictationError('TRANSCRIBE_FAILED', 'TRANSCRIBE_ABORTED');
    if (result.status < 200 || result.status >= 300) throw new DictationError('TRANSCRIBE_FAILED', `TRANSCRIBE_HTTP_${result.status}`);
    const root = result.body as TranscriptionEnvelope | undefined;
    if (!root || typeof root !== 'object' || root.success !== true || !root.data || typeof root.data !== 'object') {
      throw new DictationError('TRANSCRIBE_FAILED', 'TRANSCRIBE_RESPONSE_INVALID');
    }
    const text = root.data.text;
    if (typeof text !== 'string' || text === '') throw new DictationError('TRANSCRIBE_FAILED', 'TRANSCRIBE_EMPTY');
    return text;
  };
}
