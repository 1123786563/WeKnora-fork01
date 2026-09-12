import { useEffect, useState } from 'react';
import type { AuthSession, InvitationLookup, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { validateLogin, validateRegister, type FieldErrors } from './validation.ts';
import { landingModeForInvite, storePendingInviteToken } from './invite-flow.ts';

export interface LoginPageProps {
  client: WeKnoraClient;
  onAuthenticated?: (session: AuthSession) => void;
  apiBaseUrl?: string;
  initialError?: string;
  initialMode?: 'login' | 'register';
  /** Share-link token from ?token= (Vue Login.vue:773-810). */
  inviteToken?: string;
  /** Called after an already-authenticated user redeems the invite token. */
  onInviteAccepted?: (result: { ok: boolean }) => void;
}

// Copy for validation message keys; moved to packages/i18n in the i18n slice.
const MESSAGES: Record<string, string> = {
  'auth.emailRequired': 'Email is required.',
  'auth.emailInvalid': 'Please enter a valid email address.',
  'auth.passwordRequired': 'Password is required.',
  'auth.passwordMinLength': 'Password must be at least 8 characters.',
  'auth.passwordMaxLength': 'Password cannot exceed 32 characters.',
  'auth.passwordMustContainLetter': 'Password must contain a letter.',
  'auth.passwordMustContainNumber': 'Password must contain a number.',
  'auth.passwordMustContainLowercaseLetter': 'Password must contain a lowercase letter.',
  'auth.passwordMustContainUppercaseLetter': 'Password must contain an uppercase letter.',
  'auth.passwordMustContainSpecialChar': 'Password must contain a special character (!@#$%^&*()_+-=[]{}|;:,.<>?).',
  'auth.usernameRequired': 'Username is required.',
  'auth.usernameMinLength': 'Username must be at least 2 characters.',
  'auth.usernameMaxLength': 'Username cannot exceed 20 characters.',
  'auth.usernameInvalid': 'Username may only contain letters, numbers, underscores and CJK characters.',
  'auth.confirmPasswordRequired': 'Please confirm your password.',
  'auth.passwordMismatch': 'The two passwords do not match.',
};

function FieldErrorsList({ errors, field }: { errors: FieldErrors; field: string }) {
  const list = errors[field];
  if (!list?.length) return null;
  return <>{list.map((key) => <Status key={key} tone="error">{MESSAGES[key] ?? key}</Status>)}</>;
}

export function LoginPage({ client, onAuthenticated, apiBaseUrl, initialError, initialMode = 'login', inviteToken = '', onInviteAccepted }: LoginPageProps) {
  const [mode, setMode] = useState<'login' | 'register'>(initialMode);
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [state, setState] = useState<'idle' | 'loading' | 'error' | 'success'>(initialError ? 'error' : 'idle');
  const [message, setMessage] = useState(initialError ?? '');
  const [registrationMode, setRegistrationMode] = useState('self_serve');
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  const [oidcEnabled, setOIDCEnabled] = useState(false);
  const [oidcProvider, setOIDCProvider] = useState('SSO');
  const [oidcLoading, setOIDCLoading] = useState(false);
  const [invite, setInvite] = useState<InvitationLookup | null>(null);
  const [inviteError, setInviteError] = useState('');

  useEffect(() => {
    let active = true;
    void Promise.allSettled([client.auth.registrationConfig(), client.auth.oidcConfig()]).then(([registration, oidc]) => {
      if (!active) return;
      if (registration.status === 'fulfilled') {
        setRegistrationMode(registration.value.registrationMode);
        setComplexPasswordEnabled(registration.value.complexPasswordEnabled);
      }
      if (oidc.status === 'fulfilled') {
        setOIDCEnabled(oidc.value.enabled);
        if (oidc.value.providerDisplayName) setOIDCProvider(oidc.value.providerDisplayName);
      }
    });
    return () => { active = false; };
  }, [client]);

  // Vue Login.vue:773-795 — resolve the invite token before any other flow.
  useEffect(() => {
    if (!inviteToken) return;
    let active = true;
    void client.auth.lookupInvitation(inviteToken)
      .then((lookup) => { if (active) setInvite(lookup); })
      .catch((error: unknown) => { if (active) setInviteError(error instanceof Error ? error.message : 'This invitation link is invalid or expired.'); });
    return () => { active = false; };
  }, [client, inviteToken]);

  async function redeemInviteAndEnter(token: string) {
    try {
      await client.auth.acceptInvitationByToken(token);
      onInviteAccepted?.({ ok: true });
    } catch {
      onInviteAccepted?.({ ok: false });
    }
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setMessage('');
    if (mode === 'register') {
      const errors = validateRegister({ username, email, password, confirmPassword }, complexPasswordEnabled);
      setFieldErrors(errors);
      if (Object.keys(errors).length) { setState('error'); return; }
    } else {
      const errors = validateLogin(email, password);
      setFieldErrors(errors);
      if (Object.keys(errors).length) { setState('error'); return; }
    }
    setState('loading');
    try {
      if (mode === 'register' && inviteToken) {
        // Vue Login.vue:716-733 — register-by-invite returns a full session.
        const session = await client.auth.registerByInvite({ token: inviteToken, username, email, password });
        onAuthenticated?.(session);
        setState('idle');
        return;
      }
      if (mode === 'register') {
        await client.auth.register({ username, email, password });
        // Vue Login.vue:744-746 — switch to login and prefill the email.
        setMode('login');
        setEmail(email);
        setUsername(''); setPassword(''); setConfirmPassword('');
        setState('success');
        setMessage('Account created. Sign in to continue.');
      } else {
        const session = await client.auth.login({ email, password });
        if (inviteToken) {
          // Vue Login.vue:686-691 — persist, redeem token, then enter.
          onAuthenticated?.(session);
          await redeemInviteAndEnter(inviteToken);
          return;
        }
        onAuthenticated?.(session);
        setState('idle');
      }
    } catch (error) {
      setState('error');
      setMessage(error instanceof Error ? error.message : mode === 'register' ? 'Registration failed' : 'Login failed');
    }
  }

  async function startOIDC() {
    setOIDCLoading(true);
    setState('idle');
    setMessage('');
    try {
      const base = apiBaseUrl || window.location.origin;
      const redirectURI = new URL('/api/v1/auth/oidc/callback', base).toString();
      const result = await client.auth.oidcUrl(redirectURI);
      // Vue Login.vue:640-643 — the IdP redirect loses the ?token=, park it.
      if (inviteToken) storePendingInviteToken(window.sessionStorage, inviteToken);
      window.location.assign(result.authorizationUrl);
    } catch (error) {
      setState('error');
      setMessage(error instanceof Error ? error.message : 'Unable to start SSO login');
    } finally {
      setOIDCLoading(false);
    }
  }

  const inviteBanner = invite
    ? <Status tone="neutral">You have been invited to join <strong>{invite.tenantName || `workspace ${invite.tenantId}`}</strong> as {invite.role}.</Status>
    : null;
  const inviteMode = inviteToken && invite ? landingModeForInvite(registrationMode) : mode;

  return <main className="wk-page"><Card>
    {inviteToken && inviteError ? <Status tone="error">{inviteError}</Status> : null}
    {inviteBanner}
    <form onSubmit={submit} aria-label="Login form">
    <h1>{inviteMode === 'login' ? 'Sign in' : 'Create account'}</h1>
    {inviteMode === 'register' ? <label>Username<input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" disabled={state === 'loading'} required minLength={2} maxLength={20} />
      <FieldErrorsList errors={fieldErrors} field="username" /></label> : null}
    <label>Email<input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="username" disabled={state === 'loading'} required />
      <FieldErrorsList errors={fieldErrors} field="email" /></label>
    <label>Password<input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" disabled={state === 'loading'} required minLength={8} maxLength={32} />
      <FieldErrorsList errors={fieldErrors} field="password" /></label>
    {inviteMode === 'register' ? <label>Confirm password<input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" disabled={state === 'loading'} required />
      <FieldErrorsList errors={fieldErrors} field="confirmPassword" /></label> : null}
    {state === 'error' && message ? <Status tone="error">{message}</Status> : null}
    {state === 'success' ? <Status tone="success">{message}</Status> : null}
    <Button type="submit" disabled={state === 'loading'}>{state === 'loading' ? (inviteMode === 'login' ? 'Signing in…' : 'Creating account…') : inviteMode === 'login' ? 'Sign in' : 'Create account'}</Button>
    {inviteMode === 'login' && !inviteToken && registrationMode !== 'invite_only' ? <Button type="button" onClick={() => { setMode('register'); setState('idle'); setMessage(''); setFieldErrors({}); }}>Create account</Button> : null}
    {inviteMode === 'register' && !inviteToken ? <Button type="button" onClick={() => { setMode('login'); setState('idle'); setMessage(''); setFieldErrors({}); }}>Back to sign in</Button> : null}
    {inviteMode === 'login' && oidcEnabled ? <Button type="button" disabled={oidcLoading} onClick={() => void startOIDC()}>{oidcLoading ? 'Redirecting…' : `Continue with ${oidcProvider}`}</Button> : null}
    </form>
  </Card></main>;
}
