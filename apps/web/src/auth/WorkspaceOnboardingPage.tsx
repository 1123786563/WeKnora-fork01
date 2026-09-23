import { useEffect, useState } from 'react';
import type { TenantInvitation, WeKnoraClient } from '@weknora/api-client';
// S6 换装（T15 前置）：表单栈离开 packages/ui 旧栈——Button/Input/Textarea 换
// tdesign（playbook §1：onChange 改 (value) 签名、maxLength → maxlength）；
// Card/Status 无 TDesign 对应走 shared/wk-legacy；两个弹窗同走 WkDialog
// （DOM 同构旧栈：本页为 React 独有表面、无扫描锚点，tdesign Dialog 的
// Portal+CSSTransition 在 node/jsdom 下退场计时器不触发会造成测试假挂，
// WkDialog 渲染树与迁移前逐节点一致）。
import { Button as TButton, Input as TInput, Textarea as TTextarea } from 'tdesign-react';
import { WkCard as Card, WkDialog as TDialog, WkStatus as Status } from '../shared/wk-legacy.tsx';
import type { WebScopeRuntime } from '../platform/scope-runtime.ts';
import { onboardingView, validateCreateTenant, type OnboardingPolicyInput } from './onboarding.ts';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import './auth-u.css';

const LOCALE_STORAGE_KEY = 'locale';
function readInitialLocale(): Locale {
  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  return stored && isLocale(stored) ? stored : 'zh-CN';
}
const msg = (locale: Locale, key: string, values?: Record<string, string | number>): string => formatMessage(locale, key, values);

export interface WorkspaceOnboardingPageProps {
  client: WeKnoraClient;
  scopeRuntime: WebScopeRuntime;
  onLogout: () => Promise<void> | void;
}

export function WorkspaceOnboardingPage({ client, scopeRuntime, onLogout }: WorkspaceOnboardingPageProps) {
  const [locale] = useState<Locale>(readInitialLocale);
  const [policy, setPolicy] = useState<OnboardingPolicyInput | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const [createVisible, setCreateVisible] = useState(false);
  const [invitationsVisible, setInvitationsVisible] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string[]>>({});
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState('');
  const [invitations, setInvitations] = useState<TenantInvitation[] | null>(null);
  const [invitationError, setInvitationError] = useState('');
  const [invitationNotice, setInvitationNotice] = useState('');
  const [respondingId, setRespondingId] = useState<number | null>(null);

  async function loadPolicy() {
    setLoadFailed(false);
    try {
      const authMe = await client.auth.me();
      scopeRuntime.hydrate(authMe);
      let pending = 0;
      try { pending = (await client.identity.tenants.invitations.pendingCount()).pendingCount; } catch { pending = 0; }
      setPolicy({
        authenticated: true,
        hasTenant: !scopeRuntime.requiresWorkspace(),
        canCreateTenant: scopeRuntime.can('can_create_tenant'),
        pendingInvitationCount: pending,
      });
    } catch {
      setLoadFailed(true);
      setPolicy({ authenticated: false, hasTenant: false, canCreateTenant: false, pendingInvitationCount: 0 });
    }
  }

  useEffect(() => { void loadPolicy(); /* eslint-disable-next-line react-hooks/exhaustive-deps */ }, []);
  const view = onboardingView(policy);
  useEffect(() => { if (view.kind === 'redirect') window.location.assign('/platform/knowledge-bases'); }, [view.kind]);

  async function createTenant() {
    const errors = validateCreateTenant({ name, description });
    setFieldErrors(errors);
    if (Object.keys(errors).length) return;
    setCreating(true);
    setCreateError('');
    try {
      await client.identity.tenants.admin.create({ name: name.trim(), description: description.trim() || undefined });
      const authMe = await client.auth.me();
      scopeRuntime.hydrate(authMe);
      window.location.assign('/platform/knowledge-bases');
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : msg(locale, 'auth.workspaceOnboarding.workspaceCreationFailed'));
    } finally {
      setCreating(false);
    }
  }

  async function loadInvitations() {
    setInvitationError('');
    setInvitationNotice('');
    try {
      const page = await client.identity.tenants.invitations.listMine();
      setInvitations(page.items.filter((item) => item.status === 'pending'));
    } catch (error) {
      setInvitationError(error instanceof Error ? error.message : msg(locale, 'auth.workspaceOnboarding.invitationsLoadFailed'));
    }
  }

  async function respond(invitation: TenantInvitation, accept: boolean) {
    if (respondingId !== null) return;
    setRespondingId(invitation.id);
    setInvitationError('');
    setInvitationNotice('');
    try {
      if (accept) await client.identity.tenants.invitations.accept(invitation.id);
      else await client.identity.tenants.invitations.decline(invitation.id);
      setInvitations((current) => current?.filter((item) => item.id !== invitation.id) ?? null);
      setPolicy((current) => current ? { ...current, pendingInvitationCount: Math.max(0, current.pendingInvitationCount - 1) } : current);
      if (accept) {
        const authMe = await client.auth.me();
        scopeRuntime.hydrate(authMe);
        if (!scopeRuntime.requiresWorkspace()) window.location.assign('/platform/knowledge-bases');
      }
      setInvitationNotice(msg(locale, accept ? 'tenantInvitation.myInbox.acceptSuccess' : 'tenantInvitation.myInbox.declineSuccess', { tenant: invitation.tenant_name || msg(locale, 'auth.workspaceOnboarding.workspaceFallback', { id: invitation.tenant_id }) }));
    } catch (error) {
      setInvitationError(error instanceof Error ? error.message : msg(locale, 'auth.workspaceOnboarding.invitationsActionFailed'));
    } finally {
      setRespondingId(null);
    }
  }

  const ready = view.kind === 'ready';
  return <main className="wk-page wk-page--std"><Card>
    <div data-testid="workspace-mark" aria-hidden="true" className="wk-onb-1">
      <svg viewBox="0 0 24 24" width="30" height="30" aria-hidden="true">
        <rect x="3" y="3" width="8" height="8" rx="1" fill="currentColor" />
        <rect x="13" y="3" width="8" height="8" rx="1" fill="currentColor" opacity="0.55" />
        <rect x="3" y="13" width="8" height="8" rx="1" fill="currentColor" opacity="0.55" />
        <rect x="13" y="13" width="8" height="8" rx="1" fill="currentColor" />
      </svg>
    </div>
    <h1 className="wk-onb-2">{ready && view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.title') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyTitle')}</h1>
    <p className="wk-muted wk-onb-3">{ready && view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.description') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyDescription')}</p>
    {view.kind === 'loading-policy' || (ready && false) ? <Status>{msg(locale, 'auth.workspaceOnboarding.loadingPolicy')}</Status> : null}
    {view.kind === 'policy-error' || loadFailed ? <div role="alert"><Status tone="error">{msg(locale, 'auth.workspaceOnboarding.policyLoadFailed')}</Status>
      <TButton type="button" onClick={() => void loadPolicy()}>{msg(locale, 'auth.workspaceOnboarding.retry')}</TButton></div> : null}
    {ready ? <>
      {view.inviteOnly ? <Status>{msg(locale, 'auth.workspaceOnboarding.inviteOnlyNotice')}</Status> : null}
      <div className="wk-actions">
        {view.canCreateTenant ? <TButton type="button" onClick={() => setCreateVisible(true)}>{msg(locale, 'auth.workspaceOnboarding.create')}</TButton> : null}
        <TButton type="button" onClick={() => { setInvitationsVisible(true); void loadInvitations(); }}>
          {msg(locale, 'auth.workspaceOnboarding.invitations')}{view.pendingInvitationCount > 0 ? ` (${view.pendingInvitationCount})` : ''}
        </TButton>
      </div>
      <p className="wk-muted wk-onb-3">{view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.help') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyHelp')}</p>
    </> : null}
    <TButton type="button" onClick={() => void onLogout()}>{msg(locale, 'auth.logout')}</TButton>

    <TDialog
      open={createVisible}
      title={<span className="wk-onb-4"><svg className="wk-onb-5" viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><rect x="3" y="3" width="8" height="8" rx="1" fill="currentColor" /><rect x="13" y="3" width="8" height="8" rx="1" fill="currentColor" opacity="0.55" /><rect x="3" y="13" width="8" height="8" rx="1" fill="currentColor" opacity="0.55" /><rect x="13" y="13" width="8" height="8" rx="1" fill="currentColor" /></svg>{msg(locale, 'tenant.create.dialogTitle')}</span>}
      onClose={() => { if (!creating) { setCreateVisible(false); setName(''); setDescription(''); setFieldErrors({}); setCreateError(''); } }}
      className="wk-onb-6"
    >
      <p className="wk-muted wk-onb-3">{msg(locale, 'tenant.create.dialogSubtitle')}</p>
      <form className="wk-form wk-onb-7" onSubmit={(event) => { event.preventDefault(); void createTenant(); }}>
        <label className="wk-onb-8">{msg(locale, 'tenant.create.nameLabel')}
          <TInput className="wk-onb-9" value={name} onChange={(value) => setName(String(value))} maxlength={128} autofocus disabled={creating} placeholder={msg(locale, 'tenant.create.namePlaceholder')} />
          {(fieldErrors.name ?? []).map((key) => <Status key={key} tone="error">{msg(locale, key)}</Status>)}
        </label>
        <label className="wk-onb-8">{msg(locale, 'tenant.create.descriptionLabel')}
          <TTextarea className="wk-onb-10" value={description} onChange={(value) => setDescription(String(value))} maxlength={512} rows={3} disabled={creating} placeholder={msg(locale, 'tenant.create.descriptionPlaceholder')} />
          {(fieldErrors.description ?? []).map((key) => <Status key={key} tone="error">{msg(locale, key)}</Status>)}
        </label>
        {createError ? <Status tone="error">{createError}</Status> : null}
        <div className="wk-actions">
          <TButton type="submit" disabled={creating}>{creating ? msg(locale, 'auth.workspaceOnboarding.creating') : msg(locale, 'tenant.create.submit')}</TButton>
          <TButton type="button" onClick={() => { setCreateVisible(false); setName(''); setDescription(''); setFieldErrors({}); setCreateError(''); }}>{msg(locale, 'tenant.create.cancel')}</TButton>
        </div>
      </form>
    </TDialog>

    <TDialog
      open={invitationsVisible}
      title={msg(locale, 'auth.workspaceOnboarding.invitations')}
      onClose={() => setInvitationsVisible(false)}
      className="wk-onb-11"
    >
      {invitationError ? <Status tone="error">{invitationError}</Status> : null}
      {invitationNotice ? <Status tone="success">{invitationNotice}</Status> : null}
      {invitations === null ? <Status>{msg(locale, 'auth.workspaceOnboarding.loadingInvitations')}</Status> : invitations.length === 0 ? <Status>{msg(locale, 'tenantInvitation.myInbox.empty')}</Status> : (
        <ul>{invitations.map((invitation) => <li key={invitation.id}>
          <strong>{invitation.tenant_name || msg(locale, 'auth.workspaceOnboarding.workspaceFallback', { id: invitation.tenant_id })}</strong> — {invitation.role}
          <TButton type="button" disabled={respondingId !== null} onClick={() => void respond(invitation, true)}>{msg(locale, 'tenantInvitation.myInbox.acceptButton')}</TButton>
          <TButton type="button" disabled={respondingId !== null} onClick={() => void respond(invitation, false)}>{msg(locale, 'tenantInvitation.myInbox.declineButton')}</TButton>
        </li>)}</ul>
      )}
      <TButton type="button" onClick={() => setInvitationsVisible(false)}>{msg(locale, 'auth.workspaceOnboarding.close')}</TButton>
    </TDialog>
  </Card></main>;
}
