// Hermes lacks globalThis.crypto / TextEncoder / btoa, which the OIDC PKCE chain
// (randomBytes → codeChallenge → base64Url) needs: without them beginOidc always
// throws OIDC_RANDOM/OIDC_CRYPTO on device (T01 #31 live round). expo-crypto fills
// the native gap; every installer is a no-op where the global already exists.

export function btoaForHermes(input: string): string {
  const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  let out = '';
  for (let i = 0; i < input.length; i += 3) {
    const bytes = [input.charCodeAt(i), input.charCodeAt(i + 1), input.charCodeAt(i + 2)];
    out += ALPHABET[bytes[0] >> 2];
    out += ALPHABET[((bytes[0] & 0x03) << 4) | ((bytes[1] >> 4) & 0x0f)];
    out += input.length > i + 1 ? ALPHABET[((bytes[1] & 0x0f) << 2) | ((bytes[2] >> 6) & 0x03)] : '=';
    out += input.length > i + 2 ? ALPHABET[bytes[2] & 0x3f] : '=';
  }
  return out;
}

// ponytail: Latin-1 truncation is fine for the ASCII-only strings the OIDC chain
// feeds it; swap in a real UTF-8 encoder if non-ASCII input ever lands here.
export class TextEncoderForHermes {
  encode(input: string): Uint8Array {
    const out = new Uint8Array(input.length);
    for (let i = 0; i < input.length; i++) out[i] = input.charCodeAt(i) & 0xff;
    return out;
  }
}

function installHermesCryptoShimIfMissing(): void {
  if (typeof globalThis.crypto?.getRandomValues === 'function') return;
  type ExpoCrypto = { getRandomBytes: (n: number) => Uint8Array; digestStringAsync: (alg: 'SHA-256', s: string, o: { encoding: 'hex' }) => Promise<string> };
  let expoCrypto: ExpoCrypto | undefined;
  try { expoCrypto = require('expo-crypto') as ExpoCrypto; } catch { expoCrypto = undefined; }
  if (expoCrypto === undefined) return;
  const hexToBytes = (hex: string): Uint8Array => { const out = new Uint8Array(hex.length / 2); for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16); return out; };
  (globalThis as { crypto?: unknown }).crypto = {
    ...(globalThis.crypto ?? {}),
    getRandomValues: (buffer: Uint8Array): Uint8Array => { buffer.set(expoCrypto!.getRandomBytes(buffer.byteLength)); return buffer; },
    subtle: {
      digest: async (_algorithm: string, data: Uint8Array): Promise<Uint8Array> => {
        let binary = '';
        for (const byte of data) binary += String.fromCharCode(byte);
        const hex = await expoCrypto!.digestStringAsync('SHA-256', binary, { encoding: 'hex' });
        return hexToBytes(hex);
      },
    },
  };
}

installHermesCryptoShimIfMissing();
if (typeof (globalThis as { TextEncoder?: unknown }).TextEncoder === 'undefined') {
  (globalThis as { TextEncoder?: unknown }).TextEncoder = TextEncoderForHermes;
}
if (typeof (globalThis as { btoa?: unknown }).btoa === 'undefined') {
  (globalThis as { btoa?: unknown }).btoa = btoaForHermes;
}
