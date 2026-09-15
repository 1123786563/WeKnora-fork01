import { describe, expect, it } from 'vitest';
import { bytesToBase64, previewText } from './file';

describe('mobile knowledge file helpers', () => {
  it('encodes binary response bytes for native file sharing', () => {
    expect(bytesToBase64(new Uint8Array([72, 105]).buffer)).toBe('SGk=');
  });

  it('only decodes text-like previews', () => {
    const bytes = new TextEncoder().encode('hello').buffer;
    expect(previewText(bytes, 'text/plain')).toBe('hello');
    expect(previewText(bytes, 'application/pdf')).toBeNull();
  });
});
