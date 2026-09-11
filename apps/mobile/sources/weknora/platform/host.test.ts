import { expect, test } from 'vitest';
import { createMobileHost } from './host';

test('requires an explicit product origin', () => {
  expect(() => createMobileHost('')).toThrow('SERVER_REQUIRED');
  expect(createMobileHost('https://example.test/')).toEqual({ backend: 'weknora', origin: 'https://example.test' });
});

test('rejects unsafe server URLs', () => {
  for (const origin of [
    'ftp://example.test',
    'https://user:pass@example.test',
    'https://example.test/?token=secret',
    'https://example.test/#fragment',
    'not a URL',
  ]) {
    expect(() => createMobileHost(origin)).toThrow('INVALID_SERVER');
  }
});
