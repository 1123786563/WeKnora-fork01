import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Button, Card, Status } from '@weknora/ui';
import { SETTINGS_SECTIONS } from '@weknora/views';
import { profilePasswordPatch, settingsOperationLabel, settingsRoleLabel, settingsScopeLabel, settingsSectionMeta, settingsValueEntries, tenantEditState, tenantPatch } from './surface.ts';
import { MemoryWorkspacePanel, PersonalMemoryPanel, PersonalMemorySettingsPanel } from './PersonalMemoryPanel.tsx';
import { ResourceSettingsPanel } from './ResourceSettingsPanel.tsx';

function errorText(error: unknown, fallback: string): string { return error instanceof Error ? error.message : fallback; }

export async function readSettingsSection(client: WeKnoraClient, key: string): Promise<unknown> {
  switch (key) {
    case 'general': return client.settings.preferences.get();
    case 'tenant': return client.settings.tenant.get();
    case 'userprofile': return client.settings.profile.get();
    case 'ollama': return Promise.all([client.settings.ollama.status(), client.settings.ollama.models()]).then(([status, models]) => ({ status, models }));
    case 'parser': return client.settings.parser.engines();
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
    default: throw new Error(`No read operation is registered for settings section: ${key}`);
  }
}

function initialSection(): string {
  const requested = new URLSearchParams(window.location.search).get('section');
  return requested && settingsSectionMeta(requested) ? requested : SETTINGS_SECTIONS[0]!.key;
}

export function SettingsPage({ client, tenantId }: { client: WeKnoraClient; tenantId: number }) {
  const [selectedKey, setSelectedKey] = useState(initialSection);
  const [payload, setPayload] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [tenantDraft, setTenantDraft] = useState({ name: '', description: '' });
  const [passwordDraft, setPasswordDraft] = useState({ oldPassword: '', newPassword: '', confirmation: '' });
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);
  const section = settingsSectionMeta(selectedKey)!;

  async function load() {
    setLoading(true); setError(null); setNotice(null);
    try { const next = await readSettingsSection(client, selectedKey); setPayload(next); if (selectedKey === 'tenant') setTenantDraft(tenantEditState(next)); }
    catch (reason) { setPayload(null); setError(errorText(reason, `Unable to load ${section.title}`)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, [client, selectedKey]);

  function select(key: string) {
    setSelectedKey(key);
    window.history.replaceState(null, '', `/platform/settings?section=${encodeURIComponent(key)}`);
  }

  async function saveTenant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError(null); setNotice(null);
    try { const next = await client.settings.tenant.update(tenantId, tenantPatch(tenantDraft.name, tenantDraft.description)); setPayload(next); setTenantDraft(tenantEditState(next)); }
    catch (reason) { setError(errorText(reason, 'Unable to save tenant information; the server value was kept.')); }
    finally { setSaving(false); }
  }

  async function changePassword(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault(); setSaving(true); setError(null); setNotice(null);
    try {
      await client.settings.profile.changePassword(profilePasswordPatch(passwordDraft.oldPassword, passwordDraft.newPassword, passwordDraft.confirmation));
      setPasswordDraft({ oldPassword: '', newPassword: '', confirmation: '' });
      setNotice('Password changed. Existing sessions may be signed out by the server.');
    } catch (reason) { setError(errorText(reason, 'Unable to change password; your current credentials were kept.')); }
    finally { setSaving(false); }
  }

  const resourcePanel = selectedKey === 'storage' || selectedKey === 'vectorstore' || selectedKey === 'websearch'
    ? <ResourceSettingsPanel client={client} section={selectedKey} initialValue={payload} />
    : null;
  return <main className="wk-page wk-settings-page"><header className="wk-header"><div><p className="wk-eyebrow">Platform settings</p><h1>Settings and runtime configuration</h1><p className="wk-muted">Every section is connected to its typed API seam. Scope, minimum role, and supported operations stay visible.</p></div><button type="button" className="wk-settings-tab" onClick={() => void load()} disabled={loading}>Reload</button></header><div className="wk-settings-layout"><nav aria-label="Settings sections" className="wk-settings-nav"><Card><h2>Sections</h2><ul className="wk-list">{SETTINGS_SECTIONS.map((item) => { const meta = settingsSectionMeta(item.key)!; return <li key={item.key} className={item.key === selectedKey ? 'is-selected' : ''}><button type="button" onClick={() => select(item.key)}>{meta.title}<small>{settingsScopeLabel(meta.scope)} · {settingsRoleLabel(meta.minRole)}</small></button></li>; })}</ul></Card></nav><section className="wk-settings-section" aria-live="polite"><Card><div className="wk-settings-panel-heading"><div><p className="wk-eyebrow">{settingsScopeLabel(section.scope)} · minimum {settingsRoleLabel(section.minRole)}</p><h2>{section.title}</h2><p className="wk-muted">{section.description}</p></div><div className="wk-settings-operation-list">{section.operations.map((operation) => <span key={operation} className={operation === 'unavailable' ? 'wk-disabled' : 'wk-role-badge'}>{settingsOperationLabel(operation)}</span>)}</div></div>{error ? <Status tone="error">{error}</Status> : loading ? <Status>Loading from {section.apiDomain}…</Status> : <>{notice ? <Status tone="success">{notice}</Status> : null}{resourcePanel ?? (selectedKey === 'tenant' ? <form className="wk-settings-editor" onSubmit={(event) => void saveTenant(event)}><label>Name<input required value={tenantDraft.name} onChange={(event) => setTenantDraft((current) => ({ ...current, name: event.target.value }))} /></label><label>Description<textarea rows={3} value={tenantDraft.description} onChange={(event) => setTenantDraft((current) => ({ ...current, description: event.target.value }))} /></label><Button type="submit" loading={saving}>Save tenant information</Button></form> : selectedKey === 'userprofile' ? <form className="wk-settings-editor" onSubmit={(event) => void changePassword(event)}><p className="wk-muted">Profile identity fields are server-owned. Change your password only after entering the current credential and confirming the new one.</p><label>Current password<input required type="password" autoComplete="current-password" value={passwordDraft.oldPassword} onChange={(event) => setPasswordDraft((current) => ({ ...current, oldPassword: event.target.value }))} /></label><label>New password<input required type="password" autoComplete="new-password" value={passwordDraft.newPassword} onChange={(event) => setPasswordDraft((current) => ({ ...current, newPassword: event.target.value }))} /></label><label>Confirm new password<input required type="password" autoComplete="new-password" value={passwordDraft.confirmation} onChange={(event) => setPasswordDraft((current) => ({ ...current, confirmation: event.target.value }))} /></label><Button type="submit" loading={saving}>Change password</Button></form> : selectedKey === 'memory' ? <div className="wk-settings-memory"><MemoryWorkspacePanel client={client} initialConfig={((payload as Record<string, unknown> | null)?.workspace)} /><PersonalMemorySettingsPanel client={client} initialSettings={((payload as Record<string, unknown> | null)?.personal)} /></div> : selectedKey === 'mymemory' ? <PersonalMemoryPanel client={client} initialItems={payload} /> : <p className="wk-settings-read-note">Read result received from the server. This inventory view does not turn unsupported save, reset, test, or delete operations into a generic editor.</p>)}<dl className="wk-settings-values">{settingsValueEntries(payload).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></>}</Card></section></div></main>;
}
