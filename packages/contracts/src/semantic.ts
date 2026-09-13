/**
 * Semantic pipeline DTOs (W01).
 *
 * IDs and revisions are DECIMAL STRINGS end-to-end: uint64 values exceed
 * JS safe integers, so parsing must never coerce to float. Unknown
 * status enums surface as a compat "unknown" state - never defaulted to
 * "ready".
 */

export type SemanticStatus =
  | "disabled"
  | "queued"
  | "indexing"
  | "ready"
  | "stale"
  | "failed"
  | "deleting"
  | "unknown";

export type SemanticMode =
  | "graphrag"
  | "retrieval.degraded"
  | "retrieval.pending-reason"
  | "rules"
  | "model"
  | "unknown";

export interface SemanticDocumentStatus {
  document_id: string;
  revision: string;
  semantic_status: SemanticStatus;
  active_generation?: string;
}

export interface SemanticEvidenceDTO {
  evidence_id: string;
  document_id: string;
  revision: string;
  chunk_id: string;
  content_hash?: string;
  quote?: string;
}

export interface SemanticSearchResponse {
  mode: SemanticMode;
  generation?: string;
  truncated: boolean;
  evidence: SemanticEvidenceDTO[];
}

export interface SemanticReasonResponse {
  mode: SemanticMode;
  status: "supported" | "insufficient_evidence" | "conflicting_evidence" | "budget_exhausted" | "unknown";
  conclusion: string;
  premise_ids: string[];
  rule_ids?: string[];
  limitations?: string[];
}

export class SemanticContractError extends Error {}

const REVISION_PATTERN = /^(0|[1-9][0-9]*)$/;
const KNOWN_STATUSES: ReadonlySet<string> = new Set([
  "disabled", "queued", "indexing", "ready", "stale", "failed", "deleting",
]);

/**
 * Parse a semantic document status payload. Revisions are validated as
 * decimal strings (floats and negatives rejected - uint64 precision is
 * preserved verbatim); unknown status enums map to "unknown" (compat,
 * never "ready").
 */
export function parseSemanticStatus(payload: unknown): SemanticDocumentStatus {
  if (typeof payload !== "object" || payload === null) {
    throw new SemanticContractError("semantic status payload must be an object");
  }
  const raw = payload as Record<string, unknown>;
  const documentId = raw.document_id;
  const revision = raw.revision;
  const status = raw.semantic_status;
  if (typeof documentId !== "string" || documentId === "") {
    throw new SemanticContractError("document_id must be a non-empty string");
  }
  if (typeof revision !== "string" || !REVISION_PATTERN.test(revision)) {
    throw new SemanticContractError(
      "revision must be a decimal string (floats are rejected to preserve uint64 precision)",
    );
  }
  let semanticStatus: SemanticStatus;
  if (typeof status === "string" && KNOWN_STATUSES.has(status)) {
    semanticStatus = status as SemanticStatus;
  } else {
    semanticStatus = "unknown";
  }
  const activeGeneration = raw.active_generation;
  return {
    document_id: documentId,
    revision,
    semantic_status: semanticStatus,
    ...(typeof activeGeneration === "string" ? { active_generation: activeGeneration } : {}),
  };
}
