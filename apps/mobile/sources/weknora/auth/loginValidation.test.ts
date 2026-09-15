import { describe, expect, test } from 'vitest';
import { validateLoginCredentials } from './loginValidation';

describe('validateLoginCredentials', () => {
  test('matches the Vue login form rules for malformed email and short password', () => {
    expect(validateLoginCredentials('person@example', 'short')).toEqual({
      email: 'Enter a valid email address.',
      password: 'Password must be at least 8 characters.',
    });
  });

  test('accepts an email and password within the Vue limits', () => {
    expect(validateLoginCredentials(' person@example.com ', '12345678')).toEqual({
      email: null,
      password: null,
    });
  });

  test('rejects passwords longer than the Vue maximum', () => {
    expect(validateLoginCredentials('person@example.com', 'a'.repeat(33))).toEqual({
      email: null,
      password: 'Password must be at most 32 characters.',
    });
  });
});
