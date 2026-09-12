/**
 * Semantic API client calls (W01).
 *
 * All scopes are decided server-side from the path + session identity;
 * the client never sends authorization material. AbortSignal is honored
 * on every call.
 */

import type {
  SemanticSearchResponse,
  SemanticReasonResponse,
} from "@weknora/contracts/semantic";

export interface SemanticClientDeps {
  fetchImpl?: typeof fetch;
  baseUrl?: string;
}

export class SemanticApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

export class SemanticAbortedError extends Error {}

export function createSemanticClient(deps: SemanticClientDeps = {}) {
  const doFetch = deps.fetchImpl ?? fetch;
  const base = deps.baseUrl ?? "";

  async function call<T>(path: string, init: RequestInit & { signal?: AbortSignal }, parse: (raw: unknown) => T): Promise<T> {
    let response: Response;
    try {
      response = await doFetch(base + path, {
        ...init,
        headers: { "Content-Type": "application/json", ...(init.headers ?? {}) },
      });
    } catch (error) {
      if (error instanceof Error && error.name === "AbortError") {
        throw new SemanticAbortedError("semantic request aborted");
      }
      throw error;
    }
    if (!response.ok) {
      let code = "unknown";
      let message = `semantic request failed with ${response.status}`;
      try {
        const body = (await response.json()) as { error?: string; code?: string };
        if (typeof body.code === "string") code = body.code;
        if (typeof body.error === "string") message = body.error;
      } catch {
        // Non-JSON error body: keep the generic message - never leak
        // internal service addresses or credentials.
      }
      throw new SemanticApiError(response.status, code, message);
    }
    return parse(await response.json());
  }

  return {
    getSemanticStatus(knowledgeBaseId: string, documentId: string, signal?: AbortSignal) {
      return call(
        `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/semantic/status?document_id=${encodeURIComponent(documentId)}`,
        { method: "GET", signal },
        parseStatusRaw,
      );
    },
    searchSemantic(knowledgeBaseId: string, query: string, signal?: AbortSignal) {
      return call(
        `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/semantic/search`,
        { method: "POST", body: JSON.stringify({ query }), signal },
        parseSearchRaw,
      );
    },
    reasonSemantic(
      knowledgeBaseId: string,
      query: string,
      mode: "rules" | "model",
      signal?: AbortSignal,
    ) {
      return call(
        `/api/v1/knowledge-bases/${encodeURIComponent(knowledgeBaseId)}/semantic/reason`,
        { method: "POST", body: JSON.stringify({ query, mode }), signal },
        parseReasonRaw,
      );
    },
    retrySemanticIndex(knowledgeId: string, signal?: AbortSignal) {
      return call(
        `/api/v1/knowledge/${encodeURIComponent(knowledgeId)}/semantic/retry`,
        { method: "POST", signal },
        parseRetryRaw,
      );
    },
  };
}

// Raw parsers keep the client tolerant of additive fields while the
// contracts package owns strict DTO validation.

function parseStatusRaw(raw: unknown) {
  return raw as { document_id: string; revision: string; semantic_status: string; active_generation?: string };
}

function parseSearchRaw(raw: unknown): SemanticSearchResponse {
  const body = raw as SemanticSearchResponse;
  return { ...body, evidence: body.evidence ?? [] };
}

function parseReasonRaw(raw: unknown): SemanticReasonResponse {
  const body = raw as SemanticReasonResponse;
  return { ...body, premise_ids: body.premise_ids ?? [] };
}

function parseRetryRaw(raw: unknown) {
  return raw as { accepted: boolean; operation_id?: string };
}
