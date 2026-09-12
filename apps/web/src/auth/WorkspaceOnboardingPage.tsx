import { useEffect, useState } from 'react';
import type { TenantInvitation, WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import type { WebScopeRuntime } from '../platform/scope-runtime.ts';
import { onboardingView, validateCreateTenant, type OnboardingPolicyInput } from './onboarding.ts';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';

const LOCALE_STORAGE_KEY = 'locale';
function readInitialLocale(): Locale {
  const stored = window.localStorage.getItem(LOCALE_STORAGE_KEY);
  return stored && isLocale(stored) ? stored : 'zh-CN';
}
const msg = (locale: Locale, key: string): string => formatMessage(locale, key);

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
      setCreateError(error instanceof Error ? error.message : 'Workspace creation failed.');
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
      setInvitationError(error instanceof Error ? error.message : 'Unable to load invitations.');
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
      setInvitationError(error instanceof Error ? error.message : 'Invitation action failed.');
    }
  }

  const ready = view.kind === 'ready';
  return <main className="wk-page"><Card>
    <h1>{ready && view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.title') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyTitle')}</h1>
    <p className="wk-muted">{ready && view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.description') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyDescription')}</p>
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
      <p className="wk-muted">{view.canCreateTenant ? msg(locale, 'auth.workspaceOnboarding.help') : msg(locale, 'auth.workspaceOnboarding.inviteOnlyHelp')}</p>
    </> : null}
    <Button type="button" onClick={() => void onLogout()}>{msg(locale, 'auth.logout')}</Button>

    {createVisible ? <Card>
      <h2>{msg(locale, 'auth.workspaceOnboarding.create')}</h2>
      <form className="wk-form" onSubmit={(event) => { event.preventDefault(); void createTenant(); }}>
        <label>{msg(locale, 'tenant.create.nameLabel')}
          <input value={name} onChange={(event) => setName(event.target.value)} maxLength={128} autoFocus disabled={creating} placeholder={msg(locale, 'tenant.create.namePlaceholder')} />
          {(fieldErrors.name ?? []).map((key) => <Status key={key} tone="error">{msg(locale, key)}</Status>)}
        </label>
        <label>{msg(locale, 'tenant.create.descriptionLabel')}
          <input value={description} onChange={(event) => setDescription(event.target.value)} maxLength={512} disabled={creating} />
          {(fieldErrors.description ?? []).map((key) => <Status key={key} tone="error">{msg(locale, key)}</Status>)}
        </label>
        {createError ? <Status tone="error">{createError}</Status> : null}
        <div className="wk-actions">
          <Button type="submit" disabled={creating}>{creating ? 'Creating…' : msg(locale, 'auth.workspaceOnboarding.create')}</Button>
          <Button type="button" onClick={() => { setCreateVisible(false); setName(''); setDescription(''); setFieldErrors({}); setCreateError(''); }}>Cancel</Button>
        </div>
      </form>
    </Card> : null}

    {invitationsVisible ? <Card>
      <h2>{msg(locale, 'auth.workspaceOnboarding.invitations')}</h2>
      {invitationError ? <Status tone="error">{invitationError}</Status> : null}
      {invitations === null ? <Status>Loading…</Status> : invitations.length === 0 ? <Status>{msg(locale, 'tenantInvitation.myInbox.empty')}</Status> : (
        <ul>{invitations.map((invitation) => <li key={invitation.id}>
          <strong>{invitation.tenant_name || `workspace ${invitation.tenant_id}`}</strong> — {invitation.role}
          <Button type="button" onClick={() => void respond(invitation, true)}>{msg(locale, 'tenantInvitation.myInbox.accept')}</Button>
          <Button type="button" onClick={() => void respond(invitation, false)}>{msg(locale, 'tenantInvitation.myInbox.decline')}</Button>
        </li>)}</ul>
      )}
      <Button type="button" onClick={() => setInvitationsVisible(false)}>Close</Button>
    </Card> : null}
  </Card></main>;
}
