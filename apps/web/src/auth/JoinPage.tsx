import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import { LoginPage } from './LoginPage.tsx';
import { readInviteToken } from './join.ts';

const storedLocale = (): Locale => {
  const stored = window.localStorage.getItem('locale');
  return stored && isLocale(stored) ? stored : 'zh-CN';
};

export interface JoinPageProps {
  client: WeKnoraClient;
  onAuthenticated?: (session: Awaited<ReturnType<WeKnoraClient['auth']['registerByInvite']>>) => void;
}

export function JoinPage({ client, onAuthenticated }: JoinPageProps) {
  const token = readInviteToken(window.location.search);
  if (token) return <LoginPage client={client} onAuthenticated={onAuthenticated} initialMode="login" inviteToken={token} />;
  return <main className="wk-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12"><Card>
    <h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{formatMessage(storedLocale(), 'auth.join.title')}</h1>
    <Status tone="error">{formatMessage(storedLocale(), 'auth.join.invitationMissingToken')}</Status>
  </Card></main>;
}
