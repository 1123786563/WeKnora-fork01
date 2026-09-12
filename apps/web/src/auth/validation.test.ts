import { test } from 'node:test';
import assert from 'node:assert/strict';
import { validateLogin, validateRegister, validatePassword } from './validation.ts';

test('login requires email format and password length matching the Vue rules', () => {
  const errors = validateLogin('not-an-email', 'short');
  assert.deepEqual(errors.email, ['auth.emailInvalid']);
  assert.deepEqual(errors.password, ['auth.passwordMinLength']);
  assert.deepEqual(validateLogin('', ''), { email: ['auth.emailRequired'], password: ['auth.passwordRequired'] });
  assert.deepEqual(validateLogin('a@b.co', 'x'.repeat(33)).password, ['auth.passwordMaxLength']);
  assert.deepEqual(validateLogin('a@b.co', 'abcdefgh'), {});
});

test('simple password policy requires letter and digit (Vue newPasswordRules)', () => {
  assert.deepEqual(validatePassword('abcdefgh', false), ['auth.passwordMustContainNumber']);
  assert.deepEqual(validatePassword('12345678', false), ['auth.passwordMustContainLetter']);
  assert.deepEqual(validatePassword('abcd1234', false), []);
});

test('complex password policy requires lower, upper, digit and special char', () => {
  const errors = validatePassword('Abcd1234', true);
  assert.deepEqual(errors, ['auth.passwordMustContainSpecialChar']);
  assert.deepEqual(validatePassword('Abcd123!', true), []);
  assert.ok(validatePassword('abcd123!', true).includes('auth.passwordMustContainUppercaseLetter'));
});

test('register validation enforces username rules and confirmation match', () => {
  const errors = validateRegister({ username: 'x', email: 'bad', password: 'short', confirmPassword: 'other' }, false);
  assert.ok(errors.username.includes('auth.usernameMinLength'));
  const patternErrors = validateRegister({ username: 'x!', email: 'e@m.co', password: 'abcd1234', confirmPassword: 'abcd1234' }, false);
  assert.deepEqual(patternErrors.username, ['auth.usernameInvalid']);
  assert.ok(patternErrors.email === undefined);
  assert.deepEqual(errors.email, ['auth.emailInvalid']);
  assert.deepEqual(errors.password, ['auth.passwordMinLength', 'auth.passwordMustContainNumber']);
  assert.deepEqual(errors.confirmPassword, ['auth.passwordMismatch']);
  assert.deepEqual(validateRegister({ username: '张三_01', email: 'a@b.co', password: 'abcd1234', confirmPassword: 'abcd1234' }, false), {});
});
