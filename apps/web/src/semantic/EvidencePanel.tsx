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
 * Evidence list with version-pinned citations (W02). Each item opens its
 * exact document/revision/chunk - never a silent jump to the current
 * revision. HONEST LIMITATION: an explicit historical-text-unavailability
 * notice requires an availability field on the evidence DTO (contracts
 * change, deferred to W03); today the link target itself reports the
 * mismatch when the historical revision is no longer readable.
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
            <a
              className="evidence-link"
              href={"#/documents/" + item.document_id + "?revision=" + item.revision + "&chunk=" + item.chunk_id}
            >
              {item.document_id} @ 修订 {item.revision} / {item.chunk_id}
            </a>
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