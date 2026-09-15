export interface LoginForm {
  email: string;
  password: string;
}

export interface RegisterForm {
  username: string;
  email: string;
  password: string;
  confirmPassword: string;
}

export type AuthField = 'email' | 'password' | 'username' | 'confirmPassword';
export type ValidationErrors = Partial<Record<AuthField, string>>;

const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const usernamePattern = /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/;

export function validateLoginForm(form: LoginForm): ValidationErrors {
  const errors: ValidationErrors = {};
  if (!form.email.trim()) errors.email = 'auth.emailRequired';
  else if (!emailPattern.test(form.email.trim())) errors.email = 'auth.emailInvalid';
  if (!form.password) errors.password = 'auth.passwordRequired';
  else if (form.password.length < 8) errors.password = 'auth.passwordMinLength';
  else if (form.password.length > 32) errors.password = 'auth.passwordMaxLength';
  return errors;
}

export function validateRegisterForm(form: RegisterForm, complexPassword = false): ValidationErrors {
  const errors: ValidationErrors = {};
  const username = form.username.trim();
  if (!username) errors.username = 'auth.usernameRequired';
  else if (username.length < 2) errors.username = 'auth.usernameMinLength';
  else if (username.length > 20) errors.username = 'auth.usernameMaxLength';
  else if (!usernamePattern.test(username)) errors.username = 'auth.usernameInvalid';
  if (!form.email.trim()) errors.email = 'auth.emailRequired';
  else if (!emailPattern.test(form.email.trim())) errors.email = 'auth.emailInvalid';
  if (!form.password) errors.password = 'auth.passwordRequired';
  else if (form.password.length < 8) errors.password = 'auth.passwordMinLength';
  else if (form.password.length > 32) errors.password = 'auth.passwordMaxLength';
  else if (complexPassword && !(/[a-z]/.test(form.password) && /[A-Z]/.test(form.password) && /\d/.test(form.password) && /[^A-Za-z0-9]/.test(form.password))) {
    errors.password = 'auth.passwordComplexity';
  }
  if (!form.confirmPassword) errors.confirmPassword = 'auth.confirmPasswordRequired';
  else if (form.confirmPassword !== form.password) errors.confirmPassword = 'auth.passwordMismatch';
  return errors;
}

export type OnboardingStatus = 'loading' | 'error' | 'ready';
export type OnboardingPresentation =
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'create-and-invite'; pendingInvitationCount: number }
  | { kind: 'invite-only'; pendingInvitationCount: number };

export function getOnboardingPresentation(input: { status: OnboardingStatus; canCreateTenant: boolean; pendingInvitationCount: number }): OnboardingPresentation {
  if (input.status === 'loading') return { kind: 'loading' };
  if (input.status === 'error') return { kind: 'error' };
  const pendingInvitationCount = Math.max(0, Math.floor(input.pendingInvitationCount));
  return input.canCreateTenant ? { kind: 'create-and-invite', pendingInvitationCount } : { kind: 'invite-only', pendingInvitationCount };
}
