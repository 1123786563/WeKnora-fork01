import { describe, expect, it } from 'vitest';
import { ExecutionSSEParser } from './stream-transport';

const frame = (seq: number, type = 'future.event') => `id: ${seq}\r\nevent: ${type}\r\ndata: ${JSON.stringify({ schema_version: 1, run_id: 'r', attempt_id: 'a', seq, type, occurred_at: '2026-09-12T00:00:00Z', payload: { value: '你' } })}\r\n\r\n`;

describe('ExecutionSSEParser', () => {
  it('decodes UTF-8 split across chunks and keeps unknown events visible', () => {
    const parser = new ExecutionSSEParser();
    const bytes = new TextEncoder().encode(frame(1));
    const split = bytes.findIndex((value, index) => index > 0 && (value & 0xc0) === 0x80);
    const result = [...parser.push(bytes.slice(0, split)), ...parser.push(bytes.slice(split))];
    expect(result[0]?.event).toBe('future.event');
    expect(result[0]?.data.payload.value).toBe('你');
  });

  it('handles CRLF split across chunks and heartbeats', () => {
    const parser = new ExecutionSSEParser();
    const source = `: heartbeat\r\n\r\n${frame(2)}`;
    const bytes = new TextEncoder().encode(source);
    const out = [...parser.push(bytes.slice(0, 7)), ...parser.push(bytes.slice(7), true)];
    expect(out.map((item) => item.data.seq)).toEqual([2]);
  });

  it('rejects invalid event envelopes', () => {
    const parser = new ExecutionSSEParser();
    expect(() => parser.push(new TextEncoder().encode('data: {"schema_version":2}\n\n'))).toThrow(/SCHEMA_VERSION/);
  });

  it('rejects an id that disagrees with seq or exceeds safe integer range', () => {
    const parser = new ExecutionSSEParser();
    expect(() => parser.push(new TextEncoder().encode(`id: 2\ndata: ${JSON.stringify({ schema_version: 1, run_id: 'r', attempt_id: 'a', seq: 1, type: 'text.delta', occurred_at: '2026-09-12T00:00:00Z', payload: {} })}\n\n`))).toThrow(/id must equal/);
    expect(() => parser.push(new TextEncoder().encode(`id: 9007199254740992\ndata: ${JSON.stringify({ schema_version: 1, run_id: 'r', attempt_id: 'a', seq: 1, type: 'text.delta', occurred_at: '2026-09-12T00:00:00Z', payload: {} })}\n\n`))).toThrow(/id must equal/);
  });
});
