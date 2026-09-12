import { useEffect, useMemo, useRef, useState } from 'react';
import { SemanticPanel } from './SemanticPanel.tsx';
import { EvidencePanel } from './EvidencePanel.tsx';
import {
  acceptSemanticResponse, beginSemanticQuery, cancelSemanticQuery,
  failSemanticQuery, initialSemanticQueryState,
} from './view-model.ts';
import { createSemanticClient, SemanticApiError } from '@weknora/api-client/semantic';
import type { SemanticDocumentStatus } from '@weknora/contracts/semantic';

export interface SemanticPageProps {
  baseUrl?: string;
  knowledgeBaseId: string;
  documentId: string;
}

/**
 * Semantic indexing status + evidence flow (W02). Requests carry an
 * AbortSignal; late responses are discarded by the view model; permission
 * errors clear knowledge from the view.
 */
export function SemanticPage({ baseUrl, knowledgeBaseId, documentId }: SemanticPageProps) {
  const client = useMemo(() => createSemanticClient({ baseUrl }), [baseUrl]);
  const [status, setStatus] = useState<SemanticDocumentStatus | null>(null);
  const [loadingStatus, setLoadingStatus] = useState(true);
  const [query, setQuery] = useState(initialSemanticQueryState());
  const [input, setInput] = useState('');
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    abortRef.current = controller;
    setLoadingStatus(true);
    client
      .getSemanticStatus(knowledgeBaseId, documentId, controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) {
          setStatus({ ...value, semantic_status: value.semantic_status as SemanticDocumentStatus['semantic_status'] });
          setLoadingStatus(false);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) setLoadingStatus(false);
      });
    return () => controller.abort();
  }, [client, knowledgeBaseId, documentId]);

  const runSearch = async () => {
    const queryId = String(Date.now());
    const controller = new AbortController();
    abortRef.current = controller;
    setQuery(beginSemanticQuery(query, queryId));
    try {
      const result = await client.searchSemantic(knowledgeBaseId, input, controller.signal);
      setQuery((state) => acceptSemanticResponse(state, queryId, result, controller.signal.aborted));
    } catch (error) {
      if (error instanceof SemanticApiError) {
        setQuery((state) => failSemanticQuery(state, queryId, error.message, error.status));
      } else {
        setQuery((state) => failSemanticQuery(state, queryId, '请求已取消'));
      }
    }
  };

  const cancel = () => {
    abortRef.current?.abort?.();
  };

  return (
    <main className="wk-page">
      <SemanticPanel
        status={status}
        loading={loadingStatus}
        onRetry={() => {
          // Server-side idempotent retry submission; the server re-checks
          // authorization - this button never grants anything.
          void client.retrySemanticIndex(documentId).catch(() => undefined);
        }}
      />
      <section className="semantic-search">
        <input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          placeholder="语义检索问题…"
          aria-label="语义检索"
        />
        <ButtonLike onClick={() => void runSearch()} disabled={query.loading}>
          检索
        </ButtonLike>
        <ButtonLike onClick={() => { abortRef.current?.abort(); setQuery((s) => cancelSemanticQuery(s)); }}>
          取消
        </ButtonLike>
      </section>
      {query.error ? <p className="semantic-error" role="alert">{query.error}</p> : null}
      <EvidencePanel result={query.result} />
    </main>
  );
}

function ButtonLike({ onClick, disabled, children }: { onClick: () => void; disabled?: boolean; children: React.ReactNode }) {
  return (
    <button type="button" className="semantic-action" onClick={onClick} disabled={disabled}>
      {children}
    </button>
  );
}
