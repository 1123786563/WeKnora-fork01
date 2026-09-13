import type { SemanticSearchResponse } from "@weknora/contracts/semantic";

export interface EvidencePanelProps {
  result: SemanticSearchResponse | null;
}

function modeLabel(mode: string): string {
  switch (mode) {
    case "graphrag": return "GraphRAG 检索";
    case "retrieval.degraded": return "普通检索（已降级）";
    case "retrieval.pending-reason": return "普通检索（推理待接入）";
    case "rules": return "规则推导";
    case "model": return "模型推断（非证明）";
    default: return "未知模式";
  }
}

/**
 * Evidence list with version-pinned citations (W02). Citations render as
 * TEXT pinned to document/revision/chunk - they do NOT navigate (no
 * document route exists yet; a dead link would unmount this page, which
 * is worse than no link). HONEST LIMITATIONS: opening the pinned revision
 * and an explicit historical-text-unavailability notice both require the
 * documents route + availability DTO field (deferred to W03).
 */
export function EvidencePanel({ result }: EvidencePanelProps) {
  if (!result) {
    return null;
  }
  return (
    <section className="evidence-panel" data-mode={result.mode}>
      <h4>
        {modeLabel(result.mode)}
        {result.truncated ? <span className="evidence-truncated">（结果已截断）</span> : null}
      </h4>
      {result.generation ? (
        <p className="evidence-generation">generation: {result.generation}</p>
      ) : null}
      <ul className="evidence-list">
        {result.evidence.map((item) => (
          <li key={item.evidence_id} className="evidence-item">
            <span className="evidence-ref">
              {item.document_id} @ 修订 {item.revision} / {item.chunk_id}
            </span>
            {item.quote ? <blockquote className="evidence-quote">{item.quote}</blockquote> : null}
          </li>
        ))}
      </ul>
      {result.evidence.length === 0 ? (
        <p className="evidence-empty">无可用证据</p>
      ) : null}
    </section>
  );
}