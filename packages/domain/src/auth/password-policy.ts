export const PASSWORD_SPECIAL_CHARS = '!@#$%^&*()_+-=[]{}|;:,.<>?';
export const PASSWORD_SPECIAL_CHAR_REGEX = /[!@#$%^&*()_+\-=\[\]{}|;:,.<>?]/;

export type PasswordViolationCode =
  | 'required'
  | 'minLength'
  | 'maxLength'
  | 'lowercase'
  | 'uppercase'
  | 'number'
  | 'specialChar'
  | 'letter';

export interface PasswordRuleResult {
  readonly valid: boolean;
  readonly violations: readonly PasswordViolationCode[];
}

function baseViolations(password: string): PasswordViolationCode[] {
  const violations: PasswordViolationCode[] = [];
  if (!password) violations.push('required');
  if (password.length > 0 && password.length < 8) violations.push('minLength');
  if (password.length > 32) violations.push('maxLength');
  return violations;
}

export function validatePassword(password: string, complexEnabled: boolean): PasswordRuleResult {
  const violations = baseViolations(password);
  if (complexEnabled) {
    if (!/[a-z]/.test(password)) violations.push('lowercase');
    if (!/[A-Z]/.test(password)) violations.push('uppercase');
    if (!/\d/.test(password)) violations.push('number');
    if (!PASSWORD_SPECIAL_CHAR_REGEX.test(password)) violations.push('specialChar');
  } else {
    if (!/[a-zA-Z]/.test(password)) violations.push('letter');
    if (!/\d/.test(password)) violations.push('number');
  }
  return { valid: violations.length === 0, violations };
}

export function firstPasswordViolation(password: string, complexEnabled: boolean): PasswordViolationCode | undefined {
  return validatePassword(password, complexEnabled).violations[0];
}
