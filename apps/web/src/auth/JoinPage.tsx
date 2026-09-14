import { useEffect, useState } from 'react';
import type { InvitationLookup, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { readInviteToken } from './join.ts';
import { validateRegister } from './validation.ts';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';

const storedLocale = (): Locale => {
  const stored = window.localStorage.getItem('locale');
  return stored && isLocale(stored) ? stored : 'zh-CN';
};

export interface JoinPageProps {
  client: WeKnoraClient;
  onAuthenticated?: (session: Awaited<ReturnType<WeKnoraClient['auth']['registerByInvite']>>) => void;
}

export function JoinPage({ client, onAuthenticated }: JoinPageProps) {
  const [token] = useState(() => readInviteToken(window.location.search));
  const [lookup, setLookup] = useState<InvitationLookup | null>(null);
  const [lookupState, setLookupState] = useState<'loading' | 'ready' | 'error'>(token ? 'loading' : 'error');
  const [message, setMessage] = useState(token ? '' : 'This invitation link is missing its token.');
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    let active = true;
    void client.auth.registrationConfig().then((config) => {
      if (active) setComplexPasswordEnabled(config.complexPasswordEnabled);
    }).catch(() => { /* fail open like Vue loadAuthConfig */ });
    return () => { active = false; };
  }, [client]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    void client.auth.lookupInvitation(token).then((next) => {
      if (!active) return;
      setLookup(next);
      setLookupState('ready');
    }).catch((error: unknown) => {
      if (!active) return;
      setLookupState('error');
      setMessage(error instanceof Error ? error.message : 'This invitation link is invalid or expired.');
    });
    return () => { active = false; };
  }, [client, token]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) return;
    const errors = validateRegister({ username, email, password, confirmPassword }, complexPasswordEnabled);
    setFieldErrors(errors);
    if (Object.keys(errors).length) return;
    setSubmitting(true);
    setMessage('');
    try {
      const session = await client.auth.registerByInvite({ token, email, username, password });
      onAuthenticated?.(session);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : formatMessage(storedLocale(), 'auth.join.invitationRegistrationFailed'));
    } finally {
      setSubmitting(false);
    }
  }

  return <main className="wk-page"><Card>
    <h1>{formatMessage(storedLocale(), 'auth.join.title')}</h1>
    {lookupState === 'loading' ? <Status>{formatMessage(storedLocale(), 'auth.join.checkingInvitation')}</Status> : null}
    {lookupState === 'error' ? <Status tone="error">{message}</Status> : null}
    {lookupState === 'ready' && lookup ? <>
      <p>{formatMessage(storedLocale(), 'auth.join.joinPrefix')} <strong>{lookup.tenantName || formatMessage(storedLocale(), 'auth.join.workspaceFallback', { id: lookup.tenantId })}</strong>{formatMessage(storedLocale(), 'auth.join.asRole', { role: lookup.role })}</p>
      <form className="wk-form" onSubmit={submit}>
        <label>{formatMessage(storedLocale(), 'auth.join.username')}<input value={username} onChange={(event) => setUsername(event.target.value)} required minLength={2} /></label>
        <label>{formatMessage(storedLocale(), 'auth.join.email')}<input value={email} onChange={(event) => setEmail(event.target.value)} type="email" required /></label>
        <label>{formatMessage(storedLocale(), 'auth.join.password')}<input value={password} onChange={(event) => setPassword(event.target.value)} type="password" required minLength={8} maxLength={32} disabled={submitting} />
        {(fieldErrors.password ?? []).map((key) => <Status key={key} tone="error">{formatMessage(storedLocale(), key)}</Status>)}</label>
      <label>{formatMessage(storedLocale(), 'auth.join.confirmPassword')}<input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" required disabled={submitting} />
        {(fieldErrors.confirmPassword ?? []).map((key) => <Status key={key} tone="error">{formatMessage(storedLocale(), key)}</Status>)}</label>
        {message ? <Status tone="error">{message}</Status> : null}
        <Button type="submit" disabled={submitting}>{submitting ? formatMessage(storedLocale(), 'auth.join.creatingAccount') : formatMessage(storedLocale(), 'auth.join.createAccountAndJoin')}</Button>
      </form>
    </> : null}
  </Card></main>;
}
