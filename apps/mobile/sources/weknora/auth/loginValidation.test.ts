import assert from 'node:assert/strict';
import test from 'node:test';
import { validateLoginCredentials } from './loginValidation';

test('matches the Vue login form rules for malformed email and short password', () => {
    assert.deepEqual(validateLoginCredentials('person@example', 'short'), {
      email: 'Enter a valid email address.',
      password: 'Password must be at least 8 characters.',
    });
});

test('accepts an email and password within the Vue limits', () => {
    assert.deepEqual(validateLoginCredentials(' person@example.com ', '12345678'), {
      email: null,
      password: null,
    });
});

test('rejects passwords longer than the Vue maximum', () => {
    assert.deepEqual(validateLoginCredentials('person@example.com', 'a'.repeat(33)), {
      email: null,
      password: 'Password must be at most 32 characters.',
    });
});
