import { useEffect, useState } from 'react';
import type { AuthSession, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';

export interface LoginPageProps {
  client: WeKnoraClient;
  onAuthenticated?: (session: AuthSession) => void;
  apiBaseUrl?: string;
  initialError?: string;
}

export function LoginPage({ client, onAuthenticated, apiBaseUrl, initialError }: LoginPageProps) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [state, setState] = useState<'idle' | 'loading' | 'error' | 'success'>(initialError ? 'error' : 'idle');
  const [message, setMessage] = useState(initialError ?? '');
  const [registrationMode, setRegistrationMode] = useState('self_serve');
  const [oidcEnabled, setOIDCEnabled] = useState(false);
  const [oidcProvider, setOIDCProvider] = useState('SSO');
  const [oidcLoading, setOIDCLoading] = useState(false);

  useEffect(() => {
    let active = true;
    void Promise.allSettled([client.auth.registrationConfig(), client.auth.oidcConfig()]).then(([registration, oidc]) => {
      if (!active) return;
      if (registration.status === 'fulfilled') setRegistrationMode(registration.value.registrationMode);
      if (oidc.status === 'fulfilled') {
        setOIDCEnabled(oidc.value.enabled);
        if (oidc.value.providerDisplayName) setOIDCProvider(oidc.value.providerDisplayName);
      }
    });
    return () => { active = false; };
  }, [client]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setState('loading');
    setMessage('');
    try {
      if (mode === 'register') {
        await client.auth.register({ username, email, password });
        setMode('login');
        setState('success');
        setMessage('Account created. Sign in to continue.');
      } else {
        const session = await client.auth.login({ email, password });
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
      window.location.assign(result.authorizationUrl);
    } catch (error) {
      setState('error');
      setMessage(error instanceof Error ? error.message : 'Unable to start SSO login');
    } finally {
      setOIDCLoading(false);
    }
  }

  return <main className="wk-page"><Card><form onSubmit={submit} aria-label="Login form">
    <h1>{mode === 'login' ? 'Sign in' : 'Create account'}</h1>
    {mode === 'register' ? <label>Username<input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" required minLength={2} /></label> : null}
    <label>Email<input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="username" required /></label>
    <label>Password<input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" required /></label>
    {state === 'error' ? <Status tone="error">{message}</Status> : null}
    {state === 'success' ? <Status tone="success">{message}</Status> : null}
    <Button type="submit" disabled={state === 'loading'}>{state === 'loading' ? mode === 'login' ? 'Signing in…' : 'Creating account…' : mode === 'login' ? 'Sign in' : 'Create account'}</Button>
    {mode === 'login' && registrationMode !== 'invite_only' ? <Button type="button" onClick={() => { setMode('register'); setState('idle'); setMessage(''); }}>Create account</Button> : null}
    {mode === 'register' ? <Button type="button" onClick={() => { setMode('login'); setState('idle'); setMessage(''); }}>Back to sign in</Button> : null}
    {mode === 'login' && oidcEnabled ? <Button type="button" disabled={oidcLoading} onClick={() => void startOIDC()}>{oidcLoading ? 'Redirecting…' : `Continue with ${oidcProvider}`}</Button> : null}
  </form></Card></main>;
}
