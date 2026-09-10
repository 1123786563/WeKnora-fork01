import { useState } from 'react';
import type { AuthSession, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';

export interface LoginPageProps {
  client: WeKnoraClient;
  onAuthenticated?: (session: AuthSession) => void;
}

export function LoginPage({ client, onAuthenticated }: LoginPageProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [state, setState] = useState<'idle' | 'loading' | 'error'>('idle');
  const [message, setMessage] = useState('');

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setState('loading');
    setMessage('');
    try {
      const session = await client.auth.login({ email, password });
      onAuthenticated?.(session);
      setState('idle');
    } catch (error) {
      setState('error');
      setMessage(error instanceof Error ? error.message : 'Login failed');
    }
  }

  return <main className="wk-page"><Card><form onSubmit={submit} aria-label="Login form">
    <h1>Sign in</h1>
    <label>Email<input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="username" required /></label>
    <label>Password<input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" required /></label>
    {state === 'error' ? <Status tone="error">{message}</Status> : null}
    <Button type="submit" disabled={state === 'loading'}>{state === 'loading' ? 'Signing in…' : 'Sign in'}</Button>
  </form></Card></main>;
}
