import { useEffect, useRef, useState } from 'react';
// S6 抽屉收编：集成向导抽屉离开 @weknora/ui（T15 硬前置），换 tdesign。
import { Alert as TAlert, Button as TButton, Drawer as TDrawer, Input as TInput } from 'tdesign-react';
import { WkCard as Card, WkStatus as Status } from '../shared/wk-legacy.tsx';
import { buildIntegrationPath, integrationLabels, integrationTabs, type IntegrationRoute } from './route.ts';
import { drawerTabs, nextTab, visibleChannels, type DrawerTab, type IntegrationChannel, type IntegrationLoadState, type IntegrationSnapshot } from './state.ts';

export interface IntegrationsPageProps {
  route: IntegrationRoute;
  onNavigate(path: string): void;
  canView?: boolean;
  canEdit?: boolean;
  load?: () => Promise<IntegrationSnapshot>;
  initialData?: IntegrationSnapshot;
  onSave?: (channel: IntegrationChannel) => Promise<void>;
  runPlaygroundRequest?: () => Promise<void>;
}

const copy: Record<IntegrationRoute['tab'], { title: string; description: string }> = {
  im: { title: 'IM Web integrations', description: 'Configure IM Web publishing channels for Agents.' },
  embed: { title: 'Web embed', description: 'Configure the Agent web embed channel and publish code.' },
  api: { title: 'API integration', description: 'Manage API settings, keys, and the API playground.' },
  cli: { title: 'WeKnora CLI', description: 'Manage knowledge bases and documents from your terminal.' },
  chrome: { title: 'Chrome extension', description: 'Connect the WeKnora browser extension.' },
  claw: { title: 'Claw Skill', description: 'Connect OpenClaw to WeKnora.' },
};

const emptySnapshot: IntegrationSnapshot = { channels: [], apiKeys: [] };

function ChannelCards({ tab, snapshot, agentId, canEdit, onOpen }: { tab: 'im' | 'embed'; snapshot: IntegrationSnapshot; agentId: string | null; canEdit: boolean; onOpen(channel: IntegrationChannel): void }) {
  const channels = visibleChannels(snapshot, tab, agentId);
  return <section aria-label={`${integrationLabels[tab]} channels`}>
    <div className="wk-integration-toolbar"><span>{channels.length} channels</span>{canEdit ? <TButton type="button" size="small" onClick={() => onOpen({ id: '', name: '', enabled: true, kind: tab })}>Add channel</TButton> : null}</div>
    {channels.length === 0 ? <Status>No {tab === 'im' ? 'IM Web' : 'embed'} channels configured.</Status> : <div className="wk-integration-cards">{channels.map((channel) => <button key={channel.id} type="button" className="wk-integration-card" onClick={() => onOpen(channel)}><strong>{channel.name || 'Unnamed channel'}</strong><span>{channel.agentId || 'All agents'} · {channel.enabled ? 'Enabled' : 'Disabled'}</span></button>)}</div>}
  </section>;
}

function DrawerTabs({ active, onChange }: { active: DrawerTab; onChange(tab: DrawerTab): void }) {
  const refs = useRef(new Map<DrawerTab, HTMLButtonElement>());
  return <div role="tablist" aria-label="Integration drawer sections">{drawerTabs.map((tab) => <button key={tab} ref={(node) => { if (node) refs.current.set(tab, node); else refs.current.delete(tab); }} type="button" role="tab" id={`integration-drawer-tab-${tab}`} aria-selected={active === tab} aria-controls="integration-drawer-panel" tabIndex={active === tab ? 0 : -1} onClick={() => onChange(tab)} onKeyDown={(event) => { const next = nextTab(drawerTabs, tab, event.key); if (!next) return; event.preventDefault(); onChange(next); refs.current.get(next)?.focus(); }}>{tab === 'configuration' ? 'Configuration' : tab === 'playground' ? 'API playground' : 'Embed code'}</button>)}</div>;
}

export function IntegrationsPage({ route, onNavigate, canView = true, canEdit = false, load, initialData = emptySnapshot, onSave, runPlaygroundRequest }: IntegrationsPageProps) {
  const [snapshot, setSnapshot] = useState(initialData);
  const [loadState, setLoadState] = useState<IntegrationLoadState>(load ? { status: 'loading' } : { status: 'ready' });
  const [drawer, setDrawer] = useState<IntegrationChannel | null>(null);
  const [drawerTab, setDrawerTab] = useState<DrawerTab>('configuration');
  const [saveState, setSaveState] = useState<'idle' | 'saving' | 'error'>('idle');
  const [playgroundState, setPlaygroundState] = useState<'idle' | 'loading' | 'error' | 'ready'>('idle');
  const trigger = useRef<HTMLButtonElement | null>(null);
  const tabRefs = useRef(new Map<IntegrationRoute['tab'], HTMLButtonElement>());
  const active = copy[route.tab];

  const refresh = () => { if (!load) return; setLoadState({ status: 'loading' }); void load().then((data) => { setSnapshot(data); setLoadState({ status: 'ready' }); }).catch((error: unknown) => setLoadState({ status: 'error', message: error instanceof Error ? error.message : 'Unable to load integration settings.' })); };
  useEffect(() => { refresh(); }, [load]);
  useEffect(() => { if (!drawer) trigger.current?.focus(); }, [drawer]);

  const openDrawer = (channel: IntegrationChannel) => { setDrawer(channel); setDrawerTab('configuration'); setSaveState('idle'); };
  const save = () => { if (!drawer || !onSave) { setDrawer(null); return; } setSaveState('saving'); void onSave(drawer).then(() => { setSaveState('idle'); setDrawer(null); }).catch(() => setSaveState('error')); };
  const runPlayground = () => { setPlaygroundState('loading'); void (runPlaygroundRequest ? runPlaygroundRequest() : Promise.resolve()).then(() => setPlaygroundState('ready')).catch(() => setPlaygroundState('error')); };

  return <main className="wk-page" aria-label="Integrations settings">
    <header className="wk-header"><div><p className="wk-eyebrow">Settings / Integrations</p><h1>Integrations</h1><p className="wk-muted">Vue-compatible integration sections and legacy entry points.</p></div></header>
    <Card>
      <nav aria-label="Integration sections" role="tablist" className="wk-integration-tabs">{integrationTabs.map((tab) => <button key={tab} ref={(node) => { if (node) tabRefs.current.set(tab, node); else tabRefs.current.delete(tab); }} type="button" role="tab" aria-selected={tab === route.tab} tabIndex={tab === route.tab ? 0 : -1} onClick={() => onNavigate(buildIntegrationPath(tab, route.agentId ?? undefined))} onKeyDown={(event) => { const next = nextTab(integrationTabs, tab, event.key); if (!next) return; event.preventDefault(); onNavigate(buildIntegrationPath(next, route.agentId ?? undefined)); tabRefs.current.get(next)?.focus(); }}>{integrationLabels[tab]}</button>)}</nav>
      {!canView ? <TAlert theme="error" message="You do not have permission to view integration settings." /> : loadState.status === 'loading' ? <Status>Loading integration settings…</Status> : loadState.status === 'error' ? <TAlert theme="error" message={loadState.message} operation={<TButton type="button" size="small" onClick={refresh}>Retry</TButton>} /> : <section aria-labelledby="integration-title"><p className="wk-eyebrow">{integrationLabels[route.tab]}</p><h2 id="integration-title">{active.title}</h2><p>{active.description}</p>{route.agentId ? <p className="wk-muted">Agent filter: {route.agentId}</p> : null}{route.tab === 'im' || route.tab === 'embed' ? <ChannelCards tab={route.tab} snapshot={snapshot} agentId={route.agentId} canEdit={canEdit} onOpen={(channel) => { trigger.current = document.activeElement as HTMLButtonElement; openDrawer(channel); }} /> : route.tab === 'api' ? <section aria-label="API playground"><Status>API base URL: {snapshot.apiBaseUrl || 'Not configured'}</Status>{snapshot.apiKeys?.length ? <ul aria-label="API keys">{snapshot.apiKeys.map((key) => <li key={key.id}>{key.name}: <code>{key.masked}</code></li>)}</ul> : <Status>No API keys configured.</Status>}<TButton type="button" onClick={runPlayground} loading={playgroundState === 'loading'}>Run API playground</TButton>{playgroundState === 'error' ? <Status tone="error">API playground request failed. Retry.</Status> : playgroundState === 'ready' ? <Status tone="success">API playground request completed.</Status> : null}</section> : <Status>Integration configuration remains on the existing backend contract.</Status>}</section>}
    </Card>
    <TDrawer footer={false} visible={drawer !== null} header={drawer?.id ? `Edit ${drawer.name || 'channel'}` : 'Add channel'} onClose={() => setDrawer(null)}>
      {drawer ? <><DrawerTabs active={drawerTab} onChange={setDrawerTab} /><div id="integration-drawer-panel" role="tabpanel" aria-labelledby={`integration-drawer-tab-${drawerTab}`}><label>Channel name<TInput value={drawer.name} disabled={!canEdit || saveState === 'saving'} onChange={(value) => setDrawer({ ...drawer, name: String(value) })} /></label>{drawerTab === 'playground' ? <p>Use this panel to verify the configured API request.</p> : drawerTab === 'embed' ? <pre>{`<iframe src="/embed/${drawer.id || 'new'}" title="WeKnora" />`}</pre> : <p>{canEdit ? 'Changes are editable by administrators.' : 'Read-only access.'}</p>}{saveState === 'error' ? <Status tone="error">Unable to save this channel. Retry.</Status> : null}</div><div className="wk-drawer-actions"><TButton type="button" variant="text" onClick={() => setDrawer(null)}>Cancel</TButton>{canEdit ? <TButton type="button" onClick={save} loading={saveState === 'saving'}>Save</TButton> : null}</div></> : null}
    </TDrawer>
  </main>;
}
