import { useMemo } from "react";
import { deriveSemanticView } from "@weknora/domain/semantic";
import type { SemanticDocumentStatus } from "@weknora/contracts/semantic";

export interface SemanticPanelProps {
  status: SemanticDocumentStatus | null;
  loading: boolean;
  onRetry?: () => void;
}

/**
 * Semantic indexing status + retry action (W02). The retry button only
 * APPEARS for failed/stale; the server re-checks authorization on the
 * actual call - the front-end never acts as the authority.
 */
export function SemanticPanel({ status, loading, onRetry }: SemanticPanelProps) {
  const view = useMemo(() => (status ? deriveSemanticView(status) : null), [status]);
  if (loading && !status) {
    return <section className="semantic-panel" aria-busy="true">语义状态加载中…</section>;
  }
  if (!view) {
    return <section className="semantic-panel">语义状态不可用</section>;
  }
  return (
    <section className="semantic-panel" data-status={status?.semantic_status}>
      <h4>语义索引</h4>
      <p className="semantic-status-label" role="status">{view.label}</p>
      <p className="semantic-text-hint">
        {view.canReadText ? "原文阅读不受语义状态影响" : null}
      </p>
      {view.canRetry && onRetry ? (
        <button type="button" className="semantic-retry" onClick={onRetry}>
          重试语义索引
        </button>
      ) : null}
    </section>
  );
}