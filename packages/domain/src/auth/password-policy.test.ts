import assert from 'node:assert/strict';
import test from 'node:test';

import { PASSWORD_SPECIAL_CHARS, firstPasswordViolation, validatePassword } from './password-policy.ts';

test('simple mode requires length plus letter and number', () => {
  assert.deepEqual(validatePassword('abcd1234', false), { valid: true, violations: [] });
  assert.deepEqual(validatePassword('abcdefgh', false), { valid: false, violations: ['number'] });
  assert.deepEqual(validatePassword('12345678', false), { valid: false, violations: ['letter'] });
});

test('complex mode rejects weak passwords such as abc123 client-side', () => {
  const result = validatePassword('abc123', true);
  assert.equal(result.valid, false);
  assert.ok(result.violations.includes('minLength'));
  assert.ok(result.violations.includes('uppercase'));
  assert.ok(result.violations.includes('specialChar'));
  assert.equal(validatePassword('abc123', false).valid, false);
});

test('complex mode accepts a strong password and every rule is exercised', () => {
  assert.equal(validatePassword('Str0ng!Pass', true).valid, true);
  assert.equal(validatePassword('password', true).violations.includes('number'), true);
  assert.equal(validatePassword('PASSWORD1!', true).violations.includes('lowercase'), true);
  assert.equal(validatePassword('', true).violations[0], 'required');
  assert.equal(validatePassword('x'.repeat(33), true).violations.includes('maxLength'), true);
});

test('exposes the same special-char inventory as the Vue baseline', () => {
  assert.equal(PASSWORD_SPECIAL_CHARS, '!@#$%^&*()_+-=[]{}|;:,.<>?');
  assert.equal(firstPasswordViolation('weak', true), 'minLength');
  assert.equal(firstPasswordViolation('Str0ng!Pass', true), undefined);
});
