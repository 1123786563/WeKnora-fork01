import { useCallback, useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { SettingsRole } from '@weknora/views';
import { roleAtLeast, SETTINGS_SECTIONS, settingsSectionsForRole } from '@weknora/views';
import { Button, Card, Status } from '@weknora/ui';
import { formatMessage } from '@weknora/i18n';
import { profilePasswordPatch, settingsOperationLabel, settingsRoleLabel, settingsScopeLabel, settingsSectionMeta, settingsValueEntries, tenantEditState, tenantPatch } from './surface.ts';
import { MemoryWorkspacePanel, PersonalMemoryPanel, PersonalMemorySettingsPanel } from './PersonalMemoryPanel.tsx';
import { ResourceSettingsPanel } from './ResourceSettingsPanel.tsx';
import { ConfigSettingsPanel, type SettingsModelOption } from './ConfigSettingsPanel.tsx';
import { OllamaSettingsPanel } from './OllamaSettingsPanel.tsx';
import { CloudSettingsPanel } from './CloudSettingsPanel.tsx';
import { EnvVarSettingsPanel } from './EnvVarSettingsPanel.tsx';
import { LiveSectionsPanel, PortedSectionsPanel, readInitialLocale, settingsT } from './PortedSectionsPanel.tsx';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

const PARTIALLY_PORTED_SECTIONS = new Set(['models', 'members', 'mcp', 'sandbox', 'skills', 'system-global', 'runtime-queues', 'platform-api-keys', 'system-audit-log']);

export async function readSettingsSection(client: WeKnoraClient, key: string, tenantId: number): Promise<unknown> {
  switch (key) {
    case 'general': return client.settings.preferences.get();
    case 'tenant': return client.settings.tenant.get();
    case 'userprofile': return client.settings.profile.get();
    case 'ollama': return Promise.all([client.settings.ollama.status(), client.settings.ollama.models()]).then(([status, models]) => ({ status, models }));
    case 'parser': return Promise.all([client.settings.parser.engines(), client.settings.parser.config.get()]).then(([engines, config]) => ({ engines, config }));
    case 'retrieval': return client.settings.retrieval.get();
    case 'memory': return Promise.all([client.settings.memory.workspace.get(), client.settings.memory.personal.settings()]).then(([workspace, personal]) => ({ workspace, personal }));
    case 'mymemory': return client.settings.memory.personal.items.list({ limit: 50 });
    case 'envvars': return client.settings.envVars.list();
    case 'storage': return Promise.all([client.settings.storage.backends.list(), client.settings.storage.legacy.status()]).then(([backends, legacy]) => ({ backends, legacy }));
    case 'vectorstore': return client.settings.vectorStores.list();
    case 'websearch': return client.settings.webSearch.providers.list();
    case 'chathistory': return Promise.all([client.settings.chatHistory.config.get(), client.settings.chatHistory.stats()]).then(([config, stats]) => ({ config, stats }));
    case 'system': return client.settings.system.info();
    case 'weknoracloud': return client.settings.weknoraCloud.status();
    case 'models': return client.configuration.models.list();
    case 'members': return client.identity.tenants.members.list(tenantId, { pageSize: 50 });
    case 'mcp': return client.configuration.mcp.list();
    case 'skills': return client.configuration.skills.list();
    case 'sandbox': return null;
    case 'system-global': return client.administration.settings.list();
    case 'runtime-queues': return client.administration.runtime.queues();
    case 'platform-api-keys': return client.administration.apiKeys.list();
    case 'system-audit-log': return client.administration.auditLog.list({ limit: 50 });
    default: throw new Error(`No read operation is registered for settings section: ${key}`);
  }
}

function requestedSection(search: string): string {
  const requested = new URLSearchParams(search).get('section');
  return requested && settingsSectionMeta(requested) ? requested : SETTINGS_SECTIONS[0]!.key;
}

export function SettingsPage({ client, tenantId, role = 'owner' }: { client: WeKnoraClient; tenantId: number; role?: SettingsRole }) {
  const locale = readInitialLocale();
  const t = settingsT(locale);
  const [selectedKey, setSelectedKey] = useState(() => requestedSection(window.location.search));
  const [payload, setPayload] = useState<unknown>(null);
  const [models, setModels] = useState<readonly SettingsModelOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tenantDraft, setTenantDraft] = useState({ name: '', description: '' });
  const [passwordDraft, setPasswordDraft] = useState({ oldPassword: '', newPassword: '', confirmation: '' });
  const [complexPasswordEnabled, setComplexPasswordEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const section = settingsSectionMeta(selectedKey)!;
  const visibleSections = settingsSectionsForRole(role);
  const roleDenied = !roleAtLeast(role, section.minRole);

  async function load() {
    if (roleDenied) { setPayload(null); setError(null); setLoading(false); return; }
    setLoading(true); setError(null); setNotice(null);
    try {
      const next = await readSettingsSection(client, selectedKey, tenantId); setPayload(next);
      if (selectedKey === 'tenant') setTenantDraft(tenantEditState(next));
      if (selectedKey === 'retrieval' || selectedKey === 'chathistory') {
        try { setModels(await client.configuration.models.list()); } catch { setModels([]); }
      }
    }
    catch (reason) { setPayload(null); setError(errorText(reason, `Unable to load ${section.title}`)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, [client, selectedKey, role]);
  useEffect(() => { void client.auth.registrationConfig().then((config) => setComplexPasswordEnabled(config.complexPasswordEnabled)).catch(() => setComplexPasswordEnabled(false)); }, [client]);

  useEffect(() => {
    function onPopState() { setSelectedKey(requestedSection(window.location.search)); }
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const select = useCallback((key: string) => {
    const next = settingsSectionMeta(key) ? key : SETTINGS_SECTIONS[0]!.key;
    setSelectedKey(next);
    window.history.pushState(null, '', `/platform/settings?section=${encodeURIComponent(next)}`);
  }, []);

  async function saveTenant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError(null); setNotice(null);
    try { const next = await client.settings.tenant.update(tenantId, tenantPatch(tenantDraft.name, tenantDraft.description)); setPayload(next); setTenantDraft(tenantEditState(next)); }
    catch (reason) { setError(errorText(reason, 'Unable to save tenant information; the server value was kept.')); }
    finally { setSaving(false); }
  }

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError(null); setNotice(null);
    try {
      await client.settings.profile.changePassword(profilePasswordPatch(passwordDraft.oldPassword, passwordDraft.newPassword, passwordDraft.confirmation, { complexPasswordEnabled }));
      setPasswordDraft({ oldPassword: '', newPassword: '', confirmation: '' });
      setNotice('Password changed. Existing sessions may be signed out by the server.');
    } catch (reason) { setError(errorText(reason, 'Unable to change password; your current credentials were kept.')); }
    finally { setSaving(false); }
  }

  const resourcePanel = selectedKey === 'storage' || selectedKey === 'vectorstore' || selectedKey === 'websearch'
    ? <ResourceSettingsPanel client={client} section={selectedKey} initialValue={payload} />
    : null;
  const configPanel = selectedKey === 'retrieval'
    ? <ConfigSettingsPanel client={client} section="retrieval" initialValue={payload} models={models} />
    : selectedKey === 'chathistory'
      ? <ConfigSettingsPanel
          client={client}
          section="chathistory"
          initialValue={((payload as Record<string, unknown> | null)?.config)}
          models={models}
          embeddingLocked={((payload as Record<string, unknown> | null)?.stats as Record<string, unknown> | undefined)?.has_indexed_messages === true}
          onSaved={() => void load()}
        />
      : selectedKey === 'parser'
        ? <ConfigSettingsPanel client={client} section="parser" initialValue={((payload as Record<string, unknown> | null)?.config)} />
        : null;
  const ollamaPanel = selectedKey === 'ollama' ? <OllamaSettingsPanel client={client} initialValue={payload} /> : null;
  const cloudPanel = selectedKey === 'weknoracloud' ? <CloudSettingsPanel client={client} initialValue={payload} /> : null;
  const envVarPanel = selectedKey === 'envvars' ? <EnvVarSettingsPanel client={client} initialPayload={payload} onMutated={() => void load()} /> : null;
  const portedPanel = PARTIALLY_PORTED_SECTIONS.has(selectedKey)
    ? (selectedKey === 'sandbox'
        ? <PortedSectionsPanel section={selectedKey} />
        : <LiveSectionsPanel client={client} section={selectedKey} payload={payload} />)
    : null;
  const deniedPanel = roleDenied
    ? <div data-testid="role-denied-panel"><Status tone="error">{t('settings.roleDenied.title')}</Status><p className="wk-muted">{t('settings.roleDenied.desc')}</p></div>
    : null;
  return <main className="wk-page wk-settings-page"><header className="wk-header"><div><p className="wk-eyebrow">Platform settings</p><h1>Settings and runtime configuration</h1><p className="wk-muted">Every section is connected to its typed API seam. Scope, minimum role, and supported operations stay visible.</p></div><button type="button" className="wk-settings-tab" onClick={() => void load()} disabled={loading}>Reload</button></header><div className="wk-settings-layout"><nav aria-label="Settings sections" className="wk-settings-nav"><Card><h2>Sections</h2><ul className="wk-list">{visibleSections.map((item) => { const meta = settingsSectionMeta(item.key)!; return <li key={item.key} className={item.key === selectedKey ? 'is-selected' : ''}><button type="button" onClick={() => select(item.key)}>{meta.title}<small>{settingsScopeLabel(meta.scope)} · {settingsRoleLabel(meta.minRole)}</small></button></li>; })}</ul></Card></nav><section className="wk-settings-section" aria-live="polite"><Card><div className="wk-settings-panel-heading"><div><p className="wk-eyebrow">{settingsScopeLabel(section.scope)} · minimum {settingsRoleLabel(section.minRole)}</p><h2>{section.title}</h2><p className="wk-muted">{section.description}</p></div><div className="wk-settings-operation-list">{section.operations.map((operation) => <span key={operation} className={operation === 'unavailable' ? 'wk-disabled' : 'wk-role-badge'}>{settingsOperationLabel(operation)}</span>)}</div></div>{deniedPanel ?? (error ? <Status tone="error">{error}</Status> : loading ? <Status>Loading from {section.apiDomain}…</Status> : <>{notice ? <Status tone="success">{notice}</Status> : null}{resourcePanel ?? configPanel ?? ollamaPanel ?? cloudPanel ?? envVarPanel ?? portedPanel ?? (selectedKey === 'tenant' ? <form className="wk-settings-editor" onSubmit={(event) => void saveTenant(event)}><label>Name<input required value={tenantDraft.name} onChange={(event) => setTenantDraft((current) => ({ ...current, name: event.target.value }))} /></label><label>Description<textarea rows={3} value={tenantDraft.description} onChange={(event) => setTenantDraft((current) => ({ ...current, description: event.target.value }))} /></label><Button type="submit" loading={saving}>Save tenant information</Button></form> : selectedKey === 'userprofile' ? <form className="wk-settings-editor" onSubmit={(event) => void changePassword(event)}><p className="wk-muted">Profile identity fields are server-owned. Change your password only after entering the current credential and confirming the new one.</p><label>Current password<input required type="password" autoComplete="current-password" value={passwordDraft.oldPassword} onChange={(event) => setPasswordDraft((current) => ({ ...current, oldPassword: event.target.value }))} /></label><label>New password<input required type="password" autoComplete="new-password" value={passwordDraft.newPassword} onChange={(event) => setPasswordDraft((current) => ({ ...current, newPassword: event.target.value }))} /></label><label>Confirm new password<input required type="password" autoComplete="new-password" value={passwordDraft.confirmation} onChange={(event) => setPasswordDraft((current) => ({ ...current, confirmation: event.target.value }))} /></label><Button type="submit" loading={saving}>Change password</Button></form> : selectedKey === 'memory' ? <div className="wk-settings-memory"><MemoryWorkspacePanel client={client} initialConfig={((payload as Record<string, unknown> | null)?.workspace)} /><PersonalMemorySettingsPanel client={client} initialSettings={((payload as Record<string, unknown> | null)?.personal)} /></div> : selectedKey === 'mymemory' ? <PersonalMemoryPanel client={client} initialItems={payload} /> : <p className="wk-settings-read-note">Read result received from the server. This inventory view does not turn unsupported save, reset, test, or delete operations into a generic editor.</p>)}<dl className="wk-settings-values">{settingsValueEntries(payload).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></>)}</Card></section></div></main>;
}
