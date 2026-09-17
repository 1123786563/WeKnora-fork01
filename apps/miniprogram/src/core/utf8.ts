/** Incremental, strict UTF-8. Works without a browser TextDecoder. */
export class Utf8Decoder {
  private pending: number[] = [];
  push(input: ArrayBuffer | Uint8Array): string {
    const bytes = input instanceof Uint8Array ? input : new Uint8Array(input);
    const data = [...this.pending, ...bytes]; this.pending = [];
    const out: string[] = [];
    for (let i = 0; i < data.length;) {
      const first = data[i]!;
      const count = first <= 0x7f ? 1 : first >= 0xc2 && first <= 0xdf ? 2 : first >= 0xe0 && first <= 0xef ? 3 : first >= 0xf0 && first <= 0xf4 ? 4 : 0;
      if (count === 0) throw new TypeError('Invalid UTF-8 leading byte');
      // Check all available continuation bytes, including truncated sequences.
      for (let j = 1; j < count && i + j < data.length; j++) if ((data[i + j]! & 0xc0) !== 0x80) throw new TypeError('Invalid UTF-8 continuation byte');
      if (i + count > data.length) { this.pending = data.slice(i); break; }
      let code = count === 1 ? first : first & (0x7f >> count);
      for (let j = 1; j < count; j++) code = code * 64 + (data[i + j]! & 0x3f);
      const minimum = [0, 0, 0x80, 0x800, 0x10000][count]!;
      if (code < minimum || code > 0x10ffff || (code >= 0xd800 && code <= 0xdfff)) throw new TypeError('Invalid UTF-8 scalar');
      out.push(String.fromCodePoint(code)); i += count;
    }
    return out.join('');
  }
  finish(): string { if (this.pending.length) throw new TypeError('Truncated UTF-8 sequence'); return ''; }
}
