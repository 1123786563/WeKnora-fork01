/**
 * Semantic panel interaction state (W02).
 *
 * Cancels abort the in-flight request and LATE responses are discarded
 * (query_id/generation bind the view); permission errors clear results.
 * Knowledge content renders ONLY after a successful response arrives.
 */

import type { SemanticSearchResponse } from "../../../../packages/contracts/src/semantic.ts";

export interface SemanticQueryState {
  activeQueryId: string | null;
  loading: boolean;
  result: SemanticSearchResponse | null;
  error: string | null;
}

export function initialSemanticQueryState(): SemanticQueryState {
  return { activeQueryId: null, loading: false, result: null, error: null };
}

export function beginSemanticQuery(state: SemanticQueryState, queryId: string): SemanticQueryState {
  return { activeQueryId: queryId, loading: true, result: null, error: null };
}

/**
 * A response only lands when its query_id still matches the active one
 * AND the signal has not aborted: late responses from superseded or
 * cancelled queries are discarded.
 */
export function acceptSemanticResponse(
  state: SemanticQueryState,
  queryId: string,
  result: SemanticSearchResponse,
  aborted: boolean,
): SemanticQueryState {
  if (queryId !== state.activeQueryId || aborted) {
    return state; // stale or cancelled - discard
  }
  return { ...state, loading: false, result, error: null };
}

export function failSemanticQuery(
  state: SemanticQueryState,
  queryId: string,
  message: string,
  status?: number,
): SemanticQueryState {
  if (queryId !== state.activeQueryId) {
    return state;
  }
  // Permission errors CLEAR any previous results (never keep stale
  // knowledge on screen after authorization was revoked).
  if (status === 403 || status === 401) {
    return { ...state, loading: false, result: null, error: message };
  }
  return { ...state, loading: false, error: message };
}

export function cancelSemanticQuery(state: SemanticQueryState): SemanticQueryState {
  // Closing the query: no active id remains, so any late response for the
  // cancelled query id is discarded by acceptSemanticResponse's match.
  return { ...state, loading: false, activeQueryId: null };
}

/**
 * Conclusion-kind display: a MODEL inference must NEVER be labeled as
 * proven (rule-derived only).
 */
export function conclusionLabel(kind: string | undefined): string {
  if (kind === "rule") return "规则推导";
  if (kind === "model") return "模型推断（非证明）";
  return "未知类别";
}
