// Port of frontend/src/utils/passwordPolicy.ts and Login.vue form rules.
// Returns i18n message KEYS (mapped to copy at render time) so the shared
// validation semantics stay identical across locales.

export const PASSWORD_SPECIAL_CHARS = '!@#$%^&*()_+-=[]{}|;:,.<>?';
export const PASSWORD_SPECIAL_CHAR_REGEX = /[!@#$%^&*()_+\-=\[\]{}|;:,.<>?]/;
export const USERNAME_PATTERN = /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/;
export const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export interface FieldErrors { [field: string]: string[] }

function add(errors: FieldErrors, field: string, key: string): void {
  (errors[field] ??= []).push(key);
}

export function validateLogin(email: string, password: string): FieldErrors {
  const errors: FieldErrors = {};
  if (!email.trim()) add(errors, 'email', 'auth.emailRequired');
  else if (!EMAIL_PATTERN.test(email)) add(errors, 'email', 'auth.emailInvalid');
  if (!password) add(errors, 'password', 'auth.passwordRequired');
  else {
    if (password.length < 8) add(errors, 'password', 'auth.passwordMinLength');
    if (password.length > 32) add(errors, 'password', 'auth.passwordMaxLength');
  }
  return errors;
}

export function validatePassword(password: string, complexEnabled: boolean): string[] {
  const errors: string[] = [];
  if (!password) return ['auth.passwordRequired'];
  if (password.length < 8) errors.push('auth.passwordMinLength');
  if (password.length > 32) errors.push('auth.passwordMaxLength');
  if (complexEnabled) {
    if (!/[a-z]/.test(password)) errors.push('auth.passwordMustContainLowercaseLetter');
    if (!/[A-Z]/.test(password)) errors.push('auth.passwordMustContainUppercaseLetter');
    if (!/\d/.test(password)) errors.push('auth.passwordMustContainNumber');
    if (!PASSWORD_SPECIAL_CHAR_REGEX.test(password)) errors.push('auth.passwordMustContainSpecialChar');
  } else {
    if (!/[a-zA-Z]/.test(password)) errors.push('auth.passwordMustContainLetter');
    if (!/\d/.test(password)) errors.push('auth.passwordMustContainNumber');
  }
  return errors;
}

export function validateRegister(
  input: { username: string; email: string; password: string; confirmPassword: string },
  complexEnabled: boolean,
): FieldErrors {
  const errors: FieldErrors = {};
  const username = input.username.trim();
  if (!username) add(errors, 'username', 'auth.usernameRequired');
  else {
    if (username.length < 2) add(errors, 'username', 'auth.usernameMinLength');
    if (username.length > 20) add(errors, 'username', 'auth.usernameMaxLength');
    if (!USERNAME_PATTERN.test(username)) add(errors, 'username', 'auth.usernameInvalid');
  }
  if (!input.email.trim()) add(errors, 'email', 'auth.emailRequired');
  else if (!EMAIL_PATTERN.test(input.email)) add(errors, 'email', 'auth.emailInvalid');
  for (const key of validatePassword(input.password, complexEnabled)) add(errors, 'password', key);
  if (!input.confirmPassword) add(errors, 'confirmPassword', 'auth.confirmPasswordRequired');
  else if (input.confirmPassword !== input.password) add(errors, 'confirmPassword', 'auth.passwordMismatch');
  return errors;
}
