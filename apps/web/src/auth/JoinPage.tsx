import { useEffect, useState } from 'react';
import type { InvitationLookup, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Input, Status } from '@weknora/ui';
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
  const [message, setMessage] = useState(token ? '' : formatMessage(storedLocale(), 'auth.join.invitationMissingToken'));
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const renderErrors = (field: string) => (fieldErrors[field] ?? []).map((key) => <Status key={key} tone="error">{formatMessage(storedLocale(), key)}</Status>);

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
      setMessage(error instanceof Error ? error.message : formatMessage(storedLocale(), 'auth.join.invitationInvalid'));
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
      const session = await client.auth.registerByInvite({ token, email: email.trim(), username: username.trim(), password });
      onAuthenticated?.(session);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : formatMessage(storedLocale(), 'auth.join.invitationRegistrationFailed'));
    } finally {
      setSubmitting(false);
    }
  }

  return <main className="wk-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12"><Card>
    <h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{formatMessage(storedLocale(), 'auth.join.title')}</h1>
    {lookupState === 'loading' ? <Status>{formatMessage(storedLocale(), 'auth.join.checkingInvitation')}</Status> : null}
    {lookupState === 'error' ? <Status tone="error">{message}</Status> : null}
    {lookupState === 'ready' && lookup ? <>
      <p>{formatMessage(storedLocale(), 'auth.join.joinPrefix')} <strong>{lookup.tenantName || formatMessage(storedLocale(), 'auth.join.workspaceFallback', { id: lookup.tenantId })}</strong>{formatMessage(storedLocale(), 'auth.join.asRole', { role: lookup.role })}</p>
      <form className="wk-form mb-4 flex flex-wrap items-end gap-3" onSubmit={submit}>
        <label className="grid gap-1">{formatMessage(storedLocale(), 'auth.join.username')}<Input className="rounded-control border border-line-strong p-[0.55rem]" value={username} onChange={(event) => setUsername(event.target.value)} required minLength={2} aria-invalid={Boolean(fieldErrors.username?.length)} aria-describedby={fieldErrors.username?.length ? 'join-username-error' : undefined} />{fieldErrors.username?.length ? <span id="join-username-error">{renderErrors('username')}</span> : null}</label>
        <label className="grid gap-1">{formatMessage(storedLocale(), 'auth.join.email')}<Input className="rounded-control border border-line-strong p-[0.55rem]" value={email} onChange={(event) => setEmail(event.target.value)} type="email" required aria-invalid={Boolean(fieldErrors.email?.length)} aria-describedby={fieldErrors.email?.length ? 'join-email-error' : undefined} />{fieldErrors.email?.length ? <span id="join-email-error">{renderErrors('email')}</span> : null}</label>
        <label className="grid gap-1">{formatMessage(storedLocale(), 'auth.join.password')}<Input className="rounded-control border border-line-strong p-[0.55rem]" value={password} onChange={(event) => setPassword(event.target.value)} type="password" required minLength={8} maxLength={32} disabled={submitting} />
        {renderErrors('password')}</label>
      <label className="grid gap-1">{formatMessage(storedLocale(), 'auth.join.confirmPassword')}<Input className="rounded-control border border-line-strong p-[0.55rem]" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" required disabled={submitting} />
        {renderErrors('confirmPassword')}</label>
        {message ? <Status tone="error">{message}</Status> : null}
        <Button type="submit" disabled={submitting}>{submitting ? formatMessage(storedLocale(), 'auth.join.creatingAccount') : formatMessage(storedLocale(), 'auth.join.createAccountAndJoin')}</Button>
      </form>
    </> : null}
  </Card></main>;
}
