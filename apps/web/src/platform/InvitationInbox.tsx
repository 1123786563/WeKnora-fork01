import { useCallback, useEffect, useMemo, useState } from 'react';
import type { TenantInvitation, WeKnoraClient } from '@weknora/api-client';
import { formatMessage, type Locale } from '@weknora/i18n';
// S6 换装（T15 前置）：packages/ui 旧栈 表单栈离开——Button 换 tdesign
// （playbook §1：variant="primary" → theme="primary"；size small 直译）；
// Status 无 TDesign 对应，走 shared/wk-legacy（.wk-status 族，渲染不变）。
// 弹层同走 WkDialog（DOM 同构旧栈：铃铛弹层为点击后表面、不在七页扫描锚点
// 内，tdesign Dialog 的 Portal+CSSTransition 在 node/jsdom 下退场计时器
// 不触发会造成测试假挂，WkDialog 渲染树与迁移前逐节点一致）。布局不动。
import { Button as TButton } from 'tdesign-react';
import { WkDialog as Dialog, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { usePreferredLocale } from '../locale.ts';
import './platform-u.css';

type Client = WeKnoraClient;

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
  const locale = usePreferredLocale();
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
    {pendingCount > 0 ? <span className="wk-inv-1" data-testid="global-invitation-bell-wrap">
      <button
        type="button"
        data-testid="global-invitation-bell"
        aria-label={message(locale, 'auth.workspaceOnboarding.invitations')}
        title={message(locale, 'auth.workspaceOnboarding.invitations')}
        className="wk-inv-bell wk-inv-2"
        onClick={() => setOpen(true)}
      >
        <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
          <path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9" /><path d="M10 21h4" />
        </svg>
        <span className="wk-inv-3" aria-label={`${pendingCount}`}>{pendingCount > 99 ? '99+' : pendingCount}</span>
      </button>
    </span> : null}
    <Dialog open={open} title={message(locale, 'auth.workspaceOnboarding.invitations')} onClose={closeDialog} closeLabel={message(locale, 'auth.workspaceOnboarding.close')} className="wk-inv-4">
      {invitations === null && !loadError ? <Status>{message(locale, 'auth.workspaceOnboarding.loadingInvitations')}</Status> : null}
      {loadError ? <div className="wk-inv-5"><Status tone="error">{loadError}</Status><TButton type="button" data-action="retry-invitations" onClick={() => void loadInvitations()}>{message(locale, 'auth.workspaceOnboarding.retry')}</TButton></div> : null}
      {actionError ? <Status tone="error">{actionError}</Status> : null}
      {notice ? <Status tone="success">{notice}</Status> : null}
      {invitations && invitations.length === 0 && !loadError ? <Status>{message(locale, 'tenantInvitation.myInbox.empty')}</Status> : null}
      {invitations && invitations.length > 0 ? <ul className="wk-inv-6">
        {invitations.map((invitation) => <li key={invitation.id} data-testid="invitation-row" className="wk-inv-7">
          <div className="wk-inv-8"><strong className="wk-inv-9">{invitation.tenant_name || `#${invitation.tenant_id}`}</strong><span className="wk-inv-10">{inviterDisplay(invitation)} · {invitation.role}</span>{invitation.message ? <span className="wk-inv-11">{invitation.message}</span> : null}</div>
          <div className="wk-inv-12"><TButton type="button" theme="primary" size="small" data-action="accept" loading={actingId === invitation.id} disabled={actingId !== null} onClick={() => void respond(invitation, true)}>{message(locale, 'tenantInvitation.myInbox.acceptButton')}</TButton><TButton type="button" size="small" loading={actingId === invitation.id} disabled={actingId !== null} data-action="decline" onClick={() => void respond(invitation, false)}>{message(locale, 'tenantInvitation.myInbox.declineButton')}</TButton></div>
        </li>)}
      </ul> : null}
    </Dialog>
  </>;
}
