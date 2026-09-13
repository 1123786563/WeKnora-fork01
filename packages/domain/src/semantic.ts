/**
 * Semantic status view derivation (W02).
 *
 * Pure presentation logic: the view NEVER grants authorization - the
 * server re-checks every action; front-end state only shapes display.
 */

import type { SemanticDocumentStatus } from "../../contracts/src/semantic.ts";

export interface SemanticView {
  label: string;
  canRetry: boolean;
  canReadText: boolean;
}

const LABELS: Record<string, string> = {
  disabled: "语义索引未启用",
  queued: "排队中",
  indexing: "索引中",
  ready: "就绪",
  stale: "索引过期",
  failed: "语义失败",
  deleting: "删除中",
  unknown: "状态未知",
};

/**
 * Derive the display view for one document's semantic status.
 *
 * - canRetry: ONLY failed/stale (never deleting - a deleting document
 *   must not be re-indexed).
 * - canReadText: parse text stays readable in EVERY semantic state -
 *   semantic failure never blocks ordinary document reading.
 */
export function deriveSemanticView(status: SemanticDocumentStatus): SemanticView {
  const semanticStatus = status.semantic_status;
  const canRetry = semanticStatus === "failed" || semanticStatus === "stale";
  return {
    label: LABELS[semanticStatus] ?? LABELS.unknown,
    canRetry,
    canReadText: true,
  };
}
