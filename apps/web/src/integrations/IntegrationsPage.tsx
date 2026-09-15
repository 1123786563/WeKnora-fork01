import { Button, Card, Status } from '@weknora/ui';
import { buildIntegrationPath, integrationLabels, integrationTabs, type IntegrationRoute } from './route.ts';

export interface IntegrationsPageProps {
  route: IntegrationRoute;
  onNavigate(path: string): void;
  canView?: boolean;
}

const copy: Record<IntegrationRoute['tab'], { title: string; description: string }> = {
  im: { title: 'IM integrations', description: 'Configure publishing channels for Agents.' },
  embed: { title: 'Web embed', description: 'Configure the Agent embed channel.' },
  api: { title: 'API integration', description: 'Manage API integration settings and keys.' },
  cli: { title: 'WeKnora CLI', description: 'Manage knowledge bases and documents from your terminal.' },
  chrome: { title: 'Chrome extension', description: 'Connect the WeKnora browser extension.' },
  claw: { title: 'Claw Skill', description: 'Connect OpenClaw to WeKnora.' },
};

export function IntegrationsPage({ route, onNavigate, canView = true }: IntegrationsPageProps) {
  const active = copy[route.tab];
  return (
    <main className="wk-page" aria-label="Integrations settings">
      <header className="wk-header">
        <div><p className="wk-eyebrow">Settings / Integrations</p><h1>Integrations</h1><p className="wk-muted">Vue-compatible integration sections and legacy entry points.</p></div>
      </header>
      <Card>
        <nav aria-label="Integration sections" style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem', marginBottom: '1.25rem' }}>
          {integrationTabs.map((tab) => <Button key={tab} type="button" aria-current={tab === route.tab ? 'page' : undefined} onClick={() => onNavigate(buildIntegrationPath(tab, route.agentId ?? undefined))}>{integrationLabels[tab]}</Button>)}
        </nav>
        {!canView ? <Status tone="error">You do not have permission to view integration settings.</Status> : <section aria-labelledby="integration-title"><p className="wk-eyebrow">{integrationLabels[route.tab]}</p><h2 id="integration-title">{active.title}</h2><p>{active.description}</p>{route.agentId ? <p className="wk-muted">Agent filter: {route.agentId}</p> : null}<Status>Integration configuration remains on the existing backend contract.</Status></section>}
      </Card>
    </main>
  );
}
