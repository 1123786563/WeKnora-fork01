import { useEffect, useMemo, useRef, useState } from 'react';
import { SemanticPanel } from './SemanticPanel.tsx';
import { EvidencePanel } from './EvidencePanel.tsx';
import {
  acceptSemanticResponse, beginSemanticQuery, cancelSemanticQuery,
  failSemanticQuery, initialSemanticQueryState,
} from './view-model.ts';
import { createSemanticClient, SemanticAbortedError, SemanticApiError } from '@weknora/api-client/semantic';
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
  const [retryError, setRetryError] = useState<string | null>(null);
  const searchAbortRef = useRef<AbortController | null>(null);
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
    const queryId = String(Date.now()) + '-' + String(Math.random()).slice(2, 8);
    const controller = new AbortController();
    searchAbortRef.current?.abort();
    searchAbortRef.current = controller;
    setRetryError(null);
    setQuery(beginSemanticQuery(query, queryId));
    try {
      const result = await client.searchSemantic(knowledgeBaseId, input, controller.signal);
      setQuery((state) => acceptSemanticResponse(state, queryId, result, controller.signal.aborted));
    } catch (error) {
      if (error instanceof SemanticApiError) {
        setQuery((state) => failSemanticQuery(state, queryId, error.message, error.status));
      } else if (error instanceof SemanticAbortedError) {
        setQuery((state) => failSemanticQuery(state, queryId, '请求已取消'));
      } else {
        setQuery((state) => failSemanticQuery(state, queryId, '语义检索失败'));
      }
    }
  };

  const cancel = () => {
    searchAbortRef.current?.abort();
    // Cancel BOTH: abort the request AND close the query state so the
    // abort error path silently discards (no user-facing banner).
    setQuery((state) => cancelSemanticQuery(state));
  };

  // Abort any in-flight search when the page unmounts.
  useEffect(() => () => searchAbortRef.current?.abort(), []);

  return (
    <main className="wk-page">
      <SemanticPanel
        status={status}
        loading={loadingStatus}
        onRetry={() => {
          // Server-side idempotent retry submission; the server re-checks
          // authorization - this button never grants anything. Errors
          // surface as an alert (never silently swallowed).
          void client
            .retrySemanticIndex(documentId)
            .then(() => {
              setRetryError(null);
              // Accepted retry: refresh the status panel so the new state
              // is reflected immediately.
              setLoadingStatus(true);
              const controller = new AbortController();
              abortRef.current = controller;
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
            })
            .catch((error: unknown) =>
              setRetryError(error instanceof Error ? error.message : '语义重试提交失败'));
          // HONEST LIMITATION: the Go retry endpoint is not mounted yet
          // (W03 deferral) - the submit surfaces a clear error until then.
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
        <ButtonLike onClick={cancel}>
          取消
        </ButtonLike>
      </section>
      {retryError ? <p className="semantic-error" role="alert">{retryError}</p> : null}
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
