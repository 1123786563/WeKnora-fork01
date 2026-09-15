import { useEffect, useState } from 'react';
import type { TenantInvitation, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Dialog, Input, Status, Textarea } from '@weknora/ui';
import type { WebScopeRuntime } from '../platform/scope-runtime.ts';
import { onboardingView, validateCreateTenant, type OnboardingPolicyInput } from './onboarding.ts';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';

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
      await client.identity.tenants.admin.create({ name, description: description || undefined });
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
    try {
      const page = await client.identity.tenants.invitations.listMine();
      setInvitations(page.items.filter((item) => item.status === 'pending'));
    } catch (error) {
      setInvitationError(error instanceof Error ? error.message : msg(locale, 'auth.workspaceOnboarding.invitationsLoadFailed'));
    }
  }

  async function respond(invitation: TenantInvitation, accept: boolean) {
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
    } catch (error) {
      setInvitationError(error instanceof Error ? error.message : msg(locale, 'auth.workspaceOnboarding.invitationsActionFailed'));
    }
  }

  const ready = view.kind === 'ready';
  return <main className="wk-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12"><Card>
    <h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">{ready && view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.title') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyTitle')}</h1>
    <p className="wk-muted text-muted">{ready && view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.description') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyDescription')}</p>
    {view.kind === 'loading-policy' || (ready && false) ? <Status>{msg(locale, 'auth.workspaceOnboarding.loadingPolicy')}</Status> : null}
    {view.kind === 'policy-error' || loadFailed ? <div role="alert"><Status tone="error">{msg(locale, 'auth.workspaceOnboarding.policyLoadFailed')}</Status>
      <Button type="button" onClick={() => void loadPolicy()}>{msg(locale, 'auth.workspaceOnboarding.retry')}</Button></div> : null}
    {ready ? <>
      {view.inviteOnly ? <Status>{msg(locale, 'auth.workspaceOnboarding.inviteOnlyNotice')}</Status> : null}
      <div className="wk-actions">
        {view.canCreateTenant ? <Button type="button" onClick={() => setCreateVisible(true)}>{msg(locale, 'auth.workspaceOnboarding.create')}</Button> : null}
        <Button type="button" onClick={() => { setInvitationsVisible(true); void loadInvitations(); }}>
          {msg(locale, 'auth.workspaceOnboarding.invitations')}{view.pendingInvitationCount > 0 ? ` (${view.pendingInvitationCount})` : ''}
        </Button>
      </div>
      <p className="wk-muted text-muted">{view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.help') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyHelp')}</p>
    </> : null}
    <Button type="button" onClick={() => void onLogout()}>{msg(locale, 'auth.logout')}</Button>

    <Dialog
      open={createVisible}
      title={<span className="inline-flex items-center gap-2"><svg className="text-primary shrink-0" viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><rect x="3" y="3" width="8" height="8" rx="1" fill="currentColor" /><rect x="13" y="3" width="8" height="8" rx="1" fill="currentColor" opacity="0.55" /><rect x="3" y="13" width="8" height="8" rx="1" fill="currentColor" opacity="0.55" /><rect x="13" y="13" width="8" height="8" rx="1" fill="currentColor" /></svg>{msg(locale, 'tenant.create.dialogTitle')}</span>}
      onClose={() => { if (!creating) { setCreateVisible(false); setName(''); setDescription(''); setFieldErrors({}); setCreateError(''); } }}
      className="w-[min(480px,100%)]!"
    >
      <p className="wk-muted text-muted">{msg(locale, 'tenant.create.dialogSubtitle')}</p>
      <form className="wk-form mb-4 flex flex-wrap items-end gap-3" onSubmit={(event) => { event.preventDefault(); void createTenant(); }}>
        <label className="grid gap-1">{msg(locale, 'tenant.create.nameLabel')}
          <Input className="rounded-control border border-line-strong p-[0.55rem]" value={name} onChange={(event) => setName(event.target.value)} maxLength={128} autoFocus disabled={creating} placeholder={msg(locale, 'tenant.create.namePlaceholder')} />
          {(fieldErrors.name ?? []).map((key) => <Status key={key} tone="error">{msg(locale, key)}</Status>)}
        </label>
        <label className="grid gap-1">{msg(locale, 'tenant.create.descriptionLabel')}
          <Textarea className="box-border w-full resize-y px-[0.6rem] py-[0.5rem] [font:inherit]" value={description} onChange={(event) => setDescription(event.target.value)} maxLength={512} rows={3} disabled={creating} placeholder={msg(locale, 'tenant.create.descriptionPlaceholder')} />
          {(fieldErrors.description ?? []).map((key) => <Status key={key} tone="error">{msg(locale, key)}</Status>)}
        </label>
        {createError ? <Status tone="error">{createError}</Status> : null}
        <div className="wk-actions">
          <Button type="submit" disabled={creating}>{creating ? msg(locale, 'auth.workspaceOnboarding.creating') : msg(locale, 'tenant.create.submit')}</Button>
          <Button type="button" onClick={() => { setCreateVisible(false); setName(''); setDescription(''); setFieldErrors({}); setCreateError(''); }}>{msg(locale, 'tenant.create.cancel')}</Button>
        </div>
      </form>
    </Dialog>

    {invitationsVisible ? <Card>
      <h2>{msg(locale, 'auth.workspaceOnboarding.invitations')}</h2>
      {invitationError ? <Status tone="error">{invitationError}</Status> : null}
      {invitations === null ? <Status>{msg(locale, 'auth.workspaceOnboarding.loadingInvitations')}</Status> : invitations.length === 0 ? <Status>{msg(locale, 'tenantInvitation.myInbox.empty')}</Status> : (
        <ul>{invitations.map((invitation) => <li key={invitation.id}>
          <strong>{invitation.tenant_name || msg(locale, 'auth.workspaceOnboarding.workspaceFallback', { id: invitation.tenant_id })}</strong> — {invitation.role}
          <Button type="button" onClick={() => void respond(invitation, true)}>{msg(locale, 'tenantInvitation.myInbox.accept')}</Button>
          <Button type="button" onClick={() => void respond(invitation, false)}>{msg(locale, 'tenantInvitation.myInbox.decline')}</Button>
        </li>)}</ul>
      )}
      <Button type="button" onClick={() => setInvitationsVisible(false)}>{msg(locale, 'auth.workspaceOnboarding.close')}</Button>
    </Card> : null}
  </Card></main>;
}
