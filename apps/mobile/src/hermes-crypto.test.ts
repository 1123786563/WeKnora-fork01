import test from 'node:test';
import assert from 'node:assert/strict';
import { btoaForHermes, TextEncoderForHermes } from './hermes-crypto.ts';

test('btoa polyfill matches Buffer base64 across padding shapes', () => {
  for (const input of ['', 'm', 'ma', 'man', 'hello world', '0123456789abcdef']) {
    assert.equal(btoaForHermes(input), Buffer.from(input, 'latin1').toString('base64'), `input=${JSON.stringify(input)}`);
  }
});

test('TextEncoder polyfill encodes ASCII identically to UTF-8', () => {
  const encoder = new TextEncoderForHermes();
  for (const input of ['', 'abc', 'https://192-168-3-33.nip.io:8443']) {
    assert.deepEqual(encoder.encode(input), new Uint8Array(Buffer.from(input, 'utf8')), `input=${JSON.stringify(input)}`);
  }
});
