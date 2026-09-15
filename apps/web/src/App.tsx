import { useEffect, useMemo, useState } from 'react';
import type { WeKnoraClient } from '@weknora/api-client';
import type { KnowledgeBase } from '@weknora/contracts';
import { createScopeController } from '@weknora/domain/scope';
import { scopedKey } from '@weknora/domain';
import { Button, Card, Status } from '@weknora/ui';
import { loadKnowledgeBases, type KnowledgeBaseListState } from './knowledge-bases/list.ts';

export type KnowledgeBaseEditorSection =
  | 'basic'
  | 'models'
  | 'vectorStore'
  | 'faq'
  | 'parser'
  | 'processing';

export interface KnowledgeBaseEditorNavItem {
  key: KnowledgeBaseEditorSection;
  label: string;
  description: string;
}

export interface KnowledgeBaseEditorNavGroup {
  key: 'basic' | 'processing';
  label: string;
  items: KnowledgeBaseEditorNavItem[];
}

export const knowledgeBaseEditorNavGroups: KnowledgeBaseEditorNavGroup[] = [
  {
    key: 'basic',
    label: 'Basic',
    items: [
      { key: 'basic', label: 'Basic', description: 'Name, type, and identity' },
      { key: 'models', label: 'Models', description: 'Embedding and generation models' },
      { key: 'vectorStore', label: 'Vector store', description: 'Vector-store binding' },
      { key: 'faq', label: 'FAQ', description: 'Question and answer indexing' },
    ],
  },
  {
    key: 'processing',
    label: 'Processing',
    items: [
      { key: 'parser', label: 'Parser', description: 'Document parser rules' },
      { key: 'processing', label: 'Processing', description: 'Ingestion and indexing status' },
    ],
  },
];

function editorSectionTitle(section: KnowledgeBaseEditorSection): string {
  return knowledgeBaseEditorNavGroups
    .flatMap((group) => group.items)
    .find((item) => item.key === section)?.label ?? 'Basic';
}

function isFaqKnowledgeBase(knowledgeBase: KnowledgeBase): boolean {
  return knowledgeBase.type?.toLowerCase() === 'faq';
}

function KnowledgeBaseEditor({ knowledgeBase }: { knowledgeBase: KnowledgeBase }) {
  const [activeSection, setActiveSection] = useState<KnowledgeBaseEditorSection>('basic');
  const faq = isFaqKnowledgeBase(knowledgeBase);
  const sectionTitle = editorSectionTitle(activeSection);

  return (
    <Card>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '1.25rem', alignItems: 'flex-start' }}>
        <aside aria-label="Knowledge base editor navigation" style={{ flex: '1 1 180px', minWidth: 180 }}>
          <p className="wk-eyebrow">Editor</p>
          <h2 style={{ fontSize: '1.2rem', margin: '0.35rem 0 1rem' }}>{knowledgeBase.name}</h2>
          {knowledgeBaseEditorNavGroups.map((group) => (
            <div key={group.key} style={{ marginBottom: '1rem' }}>
              <p className="wk-muted" style={{ fontSize: '0.76rem', fontWeight: 700, margin: '0 0 0.35rem', textTransform: 'uppercase' }}>
                {group.label}
              </p>
              <nav aria-label={`${group.label} editor sections`} style={{ display: 'grid', gap: '0.35rem' }}>
                {group.items.map((item) => (
                  <Button
                    key={item.key}
                    type="button"
                    aria-current={activeSection === item.key ? 'page' : undefined}
                    onClick={() => setActiveSection(item.key)}
                    style={{
                      background: activeSection === item.key ? '#edf3ff' : '#ffffff',
                      borderColor: activeSection === item.key ? '#2e6de6' : undefined,
                      textAlign: 'left',
                    }}
                  >
                    <span style={{ display: 'block', fontWeight: 700 }}>{item.label}</span>
                    <span className="wk-muted" style={{ display: 'block', fontSize: '0.78rem' }}>{item.description}</span>
                  </Button>
                ))}
              </nav>
            </div>
          ))}
        </aside>

        <section aria-labelledby="knowledge-base-editor-section" style={{ flex: '3 1 420px', minWidth: 280 }}>
          <p className="wk-eyebrow">{sectionTitle}</p>
          <h3 id="knowledge-base-editor-section" style={{ fontSize: '1.35rem', margin: '0.35rem 0' }}>{sectionTitle}</h3>
          <p className="wk-muted" style={{ marginTop: 0 }}>
            This React editor mirrors the Vue section boundary while keeping the current list and API contracts unchanged.
          </p>

          {activeSection === 'basic' ? (
            <div>
              <p><strong>Knowledge base ID</strong></p>
              <code>{knowledgeBase.id}</code>
              <p><strong>Type</strong></p>
              <p>{faq ? 'FAQ' : 'Document'}</p>
            </div>
          ) : null}
          {activeSection === 'models' ? (
            <Status>Model configuration is the next migration seam. Existing model contracts remain unchanged.</Status>
          ) : null}
          {activeSection === 'vectorStore' ? (
            <Status>Vector-store binding is available here for both document and FAQ knowledge bases.</Status>
          ) : null}
          {activeSection === 'faq' ? (
            faq ? (
              <div>
                <Status tone="success">FAQ indexing configuration</Status>
                <p>Choose whether retrieval indexes questions only or the question-and-answer pair.</p>
                <p className="wk-muted">Question index mode: combined or separate.</p>
              </div>
            ) : <Status>FAQ settings apply to FAQ knowledge bases.</Status>
          ) : null}
          {activeSection === 'parser' ? (
            faq ? <Status>Parser settings apply to document knowledge bases.</Status> : (
              <div>
                <Status tone="success">Document parser configuration</Status>
                <p>Parser engine rules will be edited here without changing the existing knowledge-base API.</p>
              </div>
            )
          ) : null}
          {activeSection === 'processing' ? (
            <div>
              <Status>Processing configuration</Status>
              <p>Ingestion and indexing progress for this knowledge base will be surfaced here.</p>
              <p className="wk-muted">Current scaffold preserves the backend list contract and does not infer processing state.</p>
            </div>
          ) : null}
        </section>
      </div>
    </Card>
  );
}

interface KnowledgeBasesPageProps {
  client: WeKnoraClient;
  scopeController: ReturnType<typeof createScopeController>;
}
export function KnowledgeBasesPage({ client, scopeController }: KnowledgeBasesPageProps) {
  const [reloadToken, setReloadToken] = useState(0);
  const [selectedKnowledgeBaseId, setSelectedKnowledgeBaseId] = useState<string | null>(null);
  const [state, setState] = useState<KnowledgeBaseListState>({ status: 'error', message: 'Loading…' });
  const scope = scopeController.current();
  const queryKey = useMemo(() => scopedKey(scope.scope, 'knowledge-bases'), [scope.scope]);

  useEffect(() => {
    let active = true;
    setState({ status: 'error', message: 'Loading…' });
    void loadKnowledgeBases(client, scope.signal).then((next) => {
      if (active && scopeController.isCurrent(scope.scope)) setState(next);
    });
    return () => { active = false; };
  }, [client, reloadToken, scopeController, scope.scope, scope.signal]);

  const selectedKnowledgeBase = state.status === 'success'
    ? state.items.find((item) => item.id === selectedKnowledgeBaseId) ?? state.items[0]
    : undefined;

  return (
    <main className="wk-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">React migration seam</p>
          <h1>Knowledge bases</h1>
          <p className="wk-muted">Live data from GET /api/v1/knowledge-bases</p>
        </div>
        <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Reload</Button>
      </header>
      <Card>
        <p className="wk-debug">scope key: {JSON.stringify(queryKey)}</p>
        {state.status === 'error' && state.message === 'Loading…' ? <Status>Loading knowledge bases…</Status> : null}
        {state.status === 'error' && state.message !== 'Loading…' ? (
          <>
            <Status tone="error">{state.message}</Status>
            <Button type="button" onClick={() => setReloadToken((value) => value + 1)}>Try again</Button>
          </>
        ) : null}
        {state.status === 'success' ? (
          state.items.length === 0 ? <Status>No knowledge bases returned by the backend.</Status> : (
            <>
              <ul className="wk-list">
                {state.items.map((item) => (
                  <li key={item.id}>
                    <Button type="button" onClick={() => setSelectedKnowledgeBaseId(item.id)}>
                      <strong>{item.name}</strong>
                    </Button>
                    <span>{item.id}</span>
                  </li>
                ))}
              </ul>
              {selectedKnowledgeBase ? <KnowledgeBaseEditor knowledgeBase={selectedKnowledgeBase} /> : null}
            </>
          )
        ) : null}
      </Card>
    </main>
  );
}

export { BillingPage } from './commercial/BillingPage.tsx';
export { CheckoutPage } from './commercial/CheckoutPage.tsx';
export { RefundPage } from './commercial/RefundPage.tsx';
export { AdminCommercialPage } from './commercial/AdminCommercialPage.tsx';
export { AppsPage } from './appconnector/AppsPage.tsx';
export { ActionApproval } from './appconnector/ActionApproval.tsx';
export { TaskDetailCommercePanel } from './appconnector/TaskDetailCommercePanel.tsx';
export { TaskBudget } from './commercial/TaskBudget.tsx';
