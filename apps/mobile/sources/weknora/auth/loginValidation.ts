export type LoginValidationResult = {
  email: string | null;
  password: string | null;
};

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function validateLoginCredentials(email: string, password: string): LoginValidationResult {
  const normalizedEmail = email.trim();
  return {
    email: normalizedEmail.length === 0
      ? 'Email is required.'
      : EMAIL_PATTERN.test(normalizedEmail)
        ? null
        : 'Enter a valid email address.',
    password: password.length === 0
      ? 'Password is required.'
      : password.length < 8
        ? 'Password must be at least 8 characters.'
        : password.length > 32
          ? 'Password must be at most 32 characters.'
          : null,
  };
}
