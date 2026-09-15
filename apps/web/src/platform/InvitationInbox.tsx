import { useCallback, useEffect, useMemo, useState } from 'react';
import type { TenantInvitation, WeKnoraClient } from '@weknora/api-client';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import { Button, Dialog, Status } from '@weknora/ui';

type Client = WeKnoraClient;

function resolveLocale(): Locale {
  const language = typeof navigator !== 'undefined' ? navigator.language : 'en-US';
  return isLocale(language) ? language : isLocale(language.split('-')[0] ?? '') ? language.split('-')[0] as Locale : 'en-US';
}

function message(locale: Locale, key: string, values?: Record<string, string | number>): string {
  return formatMessage(locale, key, values);
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

function inviterDisplay(invitation: TenantInvitation): string {
  return invitation.inviter_name?.trim() || invitation.inviter_email?.trim() || invitation.invited_by || '—';
}

export interface InvitationInboxProps {
  client: Client;
}

export function InvitationInbox({ client }: InvitationInboxProps) {
  const locale = useMemo(resolveLocale, []);
  const [pendingCount, setPendingCount] = useState(0);
  const [open, setOpen] = useState(false);
  const [invitations, setInvitations] = useState<TenantInvitation[] | null>(null);
  const [loadError, setLoadError] = useState('');
  const [actionError, setActionError] = useState('');
  const [notice, setNotice] = useState('');
  const [actingId, setActingId] = useState<number | null>(null);

  const refreshCount = useCallback(async () => {
    try {
      const result = await client.identity.tenants.invitations.pendingCount();
      setPendingCount(Math.max(0, result.pendingCount));
    } catch {
      // Vue keeps the last known badge value when the lightweight poll fails.
    }
  }, [client]);

  const loadInvitations = useCallback(async () => {
    setInvitations(null);
    setLoadError('');
    setActionError('');
    setNotice('');
    try {
      const page = await client.identity.tenants.invitations.listMine();
      const pending = page.items.filter((item) => item.status === 'pending');
      setInvitations(pending);
      setPendingCount(pending.length);
    } catch (error) {
      setLoadError(errorMessage(error, message(locale, 'common.error')));
    }
  }, [client, locale]);

  useEffect(() => {
    void refreshCount();
    // Use the ambient timer so Node/jsdom can unref the poller while browsers
    // keep the same interval semantics. This prevents a mounted shell test
    // from being held open by a two-minute background refresh.
    const timer = globalThis.setInterval(() => { void refreshCount(); }, 2 * 60 * 1000);
    (timer as unknown as { unref?: () => void }).unref?.();
    return () => globalThis.clearInterval(timer);
  }, [refreshCount]);

  useEffect(() => {
    if (open) void loadInvitations();
  }, [loadInvitations, open]);

  const respond = async (invitation: TenantInvitation, accept: boolean) => {
    if (actingId !== null) return;
    setActingId(invitation.id);
    setActionError('');
    setNotice('');
    try {
      if (accept) {
        await client.identity.tenants.invitations.accept(invitation.id);
        // Rehydrate auth/me after joining so tenant membership state follows
        // the same post-accept boundary as Vue refreshFromAuthMe().
        await client.auth.me();
      } else {
        await client.identity.tenants.invitations.decline(invitation.id);
      }
      setInvitations((current) => current?.filter((item) => item.id !== invitation.id) ?? current);
      setPendingCount((current) => Math.max(0, current - 1));
      setNotice(message(locale, accept ? 'tenantInvitation.myInbox.acceptSuccess' : 'tenantInvitation.myInbox.declineSuccess', {
        tenant: invitation.tenant_name || `#${invitation.tenant_id}`,
      }));
    } catch (error) {
      setActionError(errorMessage(error, message(locale, 'common.error')));
    } finally {
      setActingId(null);
    }
  };

  const closeDialog = useCallback(() => {
    if (actingId === null) setOpen(false);
  }, [actingId]);

  return <>
    {pendingCount > 0 ? <span className="fixed right-4 top-3 z-[100] inline-flex" data-testid="global-invitation-bell-wrap">
      <button
        type="button"
        data-testid="global-invitation-bell"
        aria-label={message(locale, 'auth.workspaceOnboarding.invitations')}
        title={message(locale, 'auth.workspaceOnboarding.invitations')}
        className="relative inline-flex h-8 w-8 items-center justify-center rounded-[10px] border border-line bg-surface text-muted shadow-[0_2px_6px_rgba(0,0,0,0.04)] hover:text-primary focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary"
        onClick={() => setOpen(true)}
      >
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" />
        </svg>
        <span className="absolute -right-2 -top-2 inline-flex min-w-5 items-center justify-center rounded-full bg-danger px-1 text-[11px] leading-5 text-white" aria-label={`${pendingCount}`}>{pendingCount > 99 ? '99+' : pendingCount}</span>
      </button>
    </span> : null}
    <Dialog open={open} title={message(locale, 'auth.workspaceOnboarding.invitations')} onClose={closeDialog} closeLabel={message(locale, 'auth.workspaceOnboarding.close')} className="w-[min(560px,100%)]!">
      {invitations === null && !loadError ? <Status>{message(locale, 'auth.workspaceOnboarding.loadingInvitations')}</Status> : null}
      {loadError ? <div className="grid gap-2"><Status tone="error">{loadError}</Status><Button type="button" data-action="retry-invitations" onClick={() => void loadInvitations()}>{message(locale, 'auth.workspaceOnboarding.retry')}</Button></div> : null}
      {actionError ? <Status tone="error">{actionError}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {invitations && invitations.length === 0 && !loadError ? <Status>{message(locale, 'tenantInvitation.myInbox.empty')}</Status> : null}
      {invitations && invitations.length > 0 ? <ul className="m-0 grid max-h-[60vh] list-none gap-2 overflow-y-auto p-0">
        {invitations.map((invitation) => <li key={invitation.id} data-testid="invitation-row" className="flex items-center justify-between gap-3 rounded-control border border-line bg-surface p-3">
          <div className="min-w-0"><strong className="block">{invitation.tenant_name || `#${invitation.tenant_id}`}</strong><span className="text-xs text-muted">{inviterDisplay(invitation)} · {invitation.role}</span>{invitation.message ? <span className="mt-1 block text-xs text-muted">{invitation.message}</span> : null}</div>
          <div className="flex shrink-0 flex-col gap-1"><Button type="button" variant="primary" size="small" data-action="accept" loading={actingId === invitation.id} disabled={actingId !== null} onClick={() => void respond(invitation, true)}>{message(locale, 'tenantInvitation.myInbox.acceptButton')}</Button><Button type="button" size="small" loading={actingId === invitation.id} disabled={actingId !== null} data-action="decline" onClick={() => void respond(invitation, false)}>{message(locale, 'tenantInvitation.myInbox.declineButton')}</Button></div>
        </li>)}
      </ul> : null}
    </Dialog>
  </>;
}
