import { parseExecutionEvent, type ExecutionEvent } from '@weknora/contracts';
import { parseStreamControl, type StreamControl } from '@weknora/domain/mobile';

export interface StreamRequest {
  url: string;
  headers: Record<string, string>;
  signal: AbortSignal;
}

/** v2 联合帧：业务帧携带完整 envelope；控制帧（control/legacy error）不占业务 seq。 */
export type ParsedExecutionFrame =
  | { kind: 'business'; id?: string; event: string; data: ExecutionEvent }
  | { kind: 'control'; event: string; control: StreamControl };

export interface StreamTransport {
  open(request: StreamRequest, onEvent: (frame: ParsedExecutionFrame) => Promise<void> | void): Promise<void>;
}

function parseFrame(raw: string): ParsedExecutionFrame | undefined {
  if (raw.trim() === '' || raw.split(/\r?\n/).every((line) => line.startsWith(':'))) return undefined;
  let id: string | undefined;
  let event = 'message';
  const data: string[] = [];
  for (const line of raw.split(/\r?\n/)) {
    if (line === '' || line.startsWith(':')) continue;
    const colon = line.indexOf(':');
    const field = colon < 0 ? line : line.slice(0, colon);
    const value = colon < 0 ? '' : line.slice(colon + 1).replace(/^ /, '');
    if (field === 'id') id = value;
    else if (field === 'event') event = value || 'message';
    else if (field === 'data') data.push(value);
  }
  if (data.length === 0) return undefined;
  let decoded: unknown;
  try { decoded = JSON.parse(data.join('\n')) as unknown; } catch { throw new Error('invalid execution SSE JSON'); }
  // 控制帧（v2 显式 control / v1 legacy error）不携带业务 envelope：单独解析，绝不推进业务 cursor
  if (event === 'control' || event === 'error') {
    const control = parseStreamControl(decoded);
    if (id !== undefined) {
      throw new Error('control frames must not carry a business id');
    }
    return { kind: 'control', event, control };
  }
  const parsed = parseExecutionEvent(decoded);
  if (id !== undefined) {
    if (!/^\d+$/.test(id) || !Number.isSafeInteger(Number(id)) || Number(id) < 0 || String(parsed.seq) !== id) {
      throw new Error('execution SSE id must equal payload.seq');
    }
  }
  return { kind: 'business', ...(id === undefined ? {} : { id }), event, data: parsed };
}

/** Incremental SSE parser: CRLF may be split between any two byte chunks. */
export class ExecutionSSEParser {
  private text = '';
  private decoder = new TextDecoder();

  push(bytes: Uint8Array, final = false): ParsedExecutionFrame[] {
    this.text += this.decoder.decode(bytes, { stream: !final });
    const frames: ParsedExecutionFrame[] = [];
    let boundary: number;
    while ((boundary = this.text.search(/\r\n\r\n|\n\n|\r\r/)) >= 0) {
      const match = this.text.slice(boundary).match(/^(\r\n\r\n|\n\n|\r\r)/)?.[1] ?? '\n\n';
      const raw = this.text.slice(0, boundary);
      this.text = this.text.slice(boundary + match.length);
      const frame = parseFrame(raw);
      if (frame) frames.push(frame);
    }
    if (final) {
      this.text += this.decoder.decode();
      const frame = parseFrame(this.text);
      if (frame) frames.push(frame);
      this.text = '';
    }
    return frames;
  }
}

export function createFetchStreamTransport(fetchImpl: typeof fetch = fetch): StreamTransport {
  return {
    async open(request, onEvent) {
      const response = await fetchImpl(request.url, { method: 'GET', headers: request.headers, signal: request.signal });
      if (!response.ok) throw new Error(`execution stream HTTP ${response.status}`);
      if (!response.body) throw new Error('execution stream has no readable body');
      const reader = response.body.getReader();
      const parser = new ExecutionSSEParser();
      try {
        while (true) {
          const chunk = await reader.read();
          if (chunk.done) {
            for (const frame of parser.push(new Uint8Array(), true)) await onEvent(frame);
            return;
          }
          for (const frame of parser.push(chunk.value)) await onEvent(frame);
        }
      } finally {
        reader.releaseLock();
      }
    },
  };
}
