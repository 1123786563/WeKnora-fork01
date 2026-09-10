import { useState } from 'react';
import { INTEGRATION_SECTIONS, type IntegrationKey } from './registry.ts';

export interface IntegrationResource {
  id: string;
  name?: string;
  platform?: string;
  agent_id?: string;
  enabled?: boolean;
  [key: string]: unknown;
}

export interface IntegrationsPageProps {
  embedChannels: readonly IntegrationResource[];
  imChannels: readonly IntegrationResource[];
  apiBaseUrl: string;
  initialTab?: IntegrationKey;
  loading?: boolean;
  error?: string;
  onReload?: () => void;
  onOpenEmbed?: (channel: IntegrationResource) => void;
}

function labelFor(key: IntegrationKey): string {
  return ({ im: 'IM channels', embed: 'Embed channels', api: 'API access', cli: 'CLI', chrome: 'Browser extension', claw: 'Claw skill' })[key];
}

export function IntegrationsPage({ embedChannels, imChannels, apiBaseUrl, initialTab = 'embed', loading = false, error, onReload, onOpenEmbed }: IntegrationsPageProps) {
  const [tab, setTab] = useState<IntegrationKey>(initialTab);
  const section = INTEGRATION_SECTIONS.find((item) => item.key === tab)!;
  return (
    <main className="wk-integrations-page">
      <header className="wk-integrations-header">
        <div><p className="wk-eyebrow">React migration seam</p><h1>Integrations</h1><p className="wk-muted">Manage visitor channels and connect external clients.</p></div>
        {onReload ? <button className="wk-button" type="button" onClick={onReload}>Reload</button> : null}
      </header>
      <nav className="wk-integrations-tabs" aria-label="Integrations">
        {INTEGRATION_SECTIONS.map((item) => <button type="button" key={item.key} className={item.key === tab ? 'is-active' : ''} onClick={() => setTab(item.key)}>{labelFor(item.key)}</button>)}
      </nav>
      <section className="wk-integrations-panel">
        <div className="wk-integrations-panel-heading"><div><h2>{labelFor(section.key)}</h2><p>{section.external ? 'Open the maintained integration guide or marketplace listing.' : `Owned by the ${section.apiDomain} API domain.`}</p></div>{section.minRole === 'owner' ? <span className="wk-role-badge">Owner</span> : null}</div>
        {loading ? <p className="wk-status">Loading integrations…</p> : null}
        {error ? <p className="wk-status wk-status-error" role="alert">{error}</p> : null}
        {!loading && !error && tab === 'embed' ? <ResourceList items={embedChannels} empty="No Embed channels configured." actionLabel="Open" onAction={onOpenEmbed} /> : null}
        {!loading && !error && tab === 'im' ? <ResourceList items={imChannels} empty="No IM channels configured." /> : null}
        {!loading && !error && tab === 'api' ? <div className="wk-integration-copy"><p>API base URL</p><code>{apiBaseUrl}</code><a href={`${apiBaseUrl.replace(/\/$/, '')}/docs`} target="_blank" rel="noreferrer">Open API documentation</a></div> : null}
        {!loading && !error && section.external ? <div className="wk-integration-copy"><p>This entry is intentionally an external guide so the main app does not embed third-party credentials or runtimes.</p><a className="wk-button" href={section.externalUrl} target="_blank" rel="noreferrer">Open {labelFor(section.key)} guide</a></div> : null}
      </section>
    </main>
  );
}

function ResourceList({ items, empty, actionLabel, onAction }: { items: readonly IntegrationResource[]; empty: string; actionLabel?: string; onAction?: (item: IntegrationResource) => void }) {
  if (items.length === 0) return <p className="wk-status">{empty}</p>;
  return <div className="wk-integration-list">{items.map((item) => <article className="wk-integration-card" key={item.id}><div><strong>{item.name || item.platform || item.id}</strong><span>{item.id}</span></div><div className="wk-integration-card-actions"><span className={item.enabled === false ? 'wk-disabled' : 'wk-enabled'}>{item.enabled === false ? 'Disabled' : 'Enabled'}</span>{onAction && actionLabel ? <button className="wk-button" type="button" onClick={() => onAction(item)}>{actionLabel}</button> : null}</div></article>)}</div>;
}
