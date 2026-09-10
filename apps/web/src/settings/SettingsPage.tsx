import { useEffect, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import { Card, Status } from '@weknora/ui';
import { SETTINGS_SECTIONS } from '@weknora/views';
import { settingsOperationLabel, settingsRoleLabel, settingsScopeLabel, settingsSectionMeta, settingsValueEntries } from './surface.ts';

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

export function SettingsPage({ client }: { client: WeKnoraClient }) {
  const [selectedKey, setSelectedKey] = useState(initialSection);
  const [payload, setPayload] = useState<unknown>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const section = settingsSectionMeta(selectedKey)!;

  async function load() {
    setLoading(true); setError(null);
    try { setPayload(await readSettingsSection(client, selectedKey)); }
    catch (reason) { setPayload(null); setError(errorText(reason, `Unable to load ${section.title}`)); }
    finally { setLoading(false); }
  }

  useEffect(() => { void load(); }, [client, selectedKey]);

  function select(key: string) {
    setSelectedKey(key);
    window.history.replaceState(null, '', `/platform/settings?section=${encodeURIComponent(key)}`);
  }

  return <main className="wk-page wk-settings-page"><header className="wk-header"><div><p className="wk-eyebrow">Platform settings</p><h1>Settings and runtime configuration</h1><p className="wk-muted">Every section is connected to its typed API seam. Scope, minimum role, and supported operations stay visible.</p></div><button type="button" className="wk-settings-tab" onClick={() => void load()} disabled={loading}>Reload</button></header><div className="wk-settings-layout"><nav aria-label="Settings sections" className="wk-settings-nav"><Card><h2>Sections</h2><ul className="wk-list">{SETTINGS_SECTIONS.map((item) => { const meta = settingsSectionMeta(item.key)!; return <li key={item.key} className={item.key === selectedKey ? 'is-selected' : ''}><button type="button" onClick={() => select(item.key)}>{meta.title}<small>{settingsScopeLabel(meta.scope)} · {settingsRoleLabel(meta.minRole)}</small></button></li>; })}</ul></Card></nav><section className="wk-settings-section" aria-live="polite"><Card><div className="wk-settings-panel-heading"><div><p className="wk-eyebrow">{settingsScopeLabel(section.scope)} · minimum {settingsRoleLabel(section.minRole)}</p><h2>{section.title}</h2><p className="wk-muted">{section.description}</p></div><div className="wk-settings-operation-list">{section.operations.map((operation) => <span key={operation} className={operation === 'unavailable' ? 'wk-disabled' : 'wk-role-badge'}>{settingsOperationLabel(operation)}</span>)}</div></div>{error ? <Status tone="error">{error}</Status> : loading ? <Status>Loading from {section.apiDomain}…</Status> : <><p className="wk-settings-read-note">Read result received from the server. This inventory view does not turn unsupported save, reset, test, or delete operations into a generic editor.</p><dl className="wk-settings-values">{settingsValueEntries(payload).map(([key, value]) => <div key={key}><dt>{key}</dt><dd>{value}</dd></div>)}</dl></>}</Card></section></div></main>;
}
