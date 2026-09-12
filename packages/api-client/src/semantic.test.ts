import { test } from "node:test";
import assert from "node:assert/strict";
import { createSemanticClient, SemanticApiError, SemanticAbortedError } from "./semantic.ts";

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

test("search maps HTTP errors to SemanticApiError with code", async () => {
  const client = createSemanticClient({
    fetchImpl: async () => jsonResponse(403, { error: "无权访问该知识库", code: "forbidden" }),
  });
  await assert.rejects(
    client.searchSemantic("kb1", "q"),
    (error: unknown) => error instanceof SemanticApiError && error.status === 403 && error.code === "forbidden",
  );
});

test("abort maps to SemanticAbortedError", async () => {
  const client = createSemanticClient({
    fetchImpl: async () => {
      throw Object.assign(new Error("aborted"), { name: "AbortError" });
    },
  });
  await assert.rejects(client.searchSemantic("kb1", "q"), SemanticAbortedError);
});

test("non-JSON error body keeps generic message (no internal leak)", async () => {
  const client = createSemanticClient({
    fetchImpl: async () =>
      new Response("<html>upstream 10.0.0.5:9090 connection refused</html>", { status: 502 }),
  });
  await assert.rejects(
    client.searchSemantic("kb1", "q"),
    (error: unknown) =>
      error instanceof SemanticApiError &&
      !String(error.message).includes("10.0.0.5") &&
      !String(error.message).includes("connection refused"),
  );
});

test("search success returns evidence list", async () => {
  const client = createSemanticClient({
    fetchImpl: async () =>
      jsonResponse(200, {
        mode: "graphrag",
        generation: "g1",
        truncated: false,
        evidence: [{ evidence_id: "e1", document_id: "d1", revision: "1", chunk_id: "c1" }],
      }),
  });
  const result = await client.searchSemantic("kb1", "q");
  assert.equal(result.mode, "graphrag");
  assert.equal(result.evidence.length, 1);
});

test("reason passes mode in body", async () => {
  let seen: unknown;
  const client = createSemanticClient({
    fetchImpl: async (_path: string, init?: RequestInit) => {
      seen = JSON.parse(String(init?.body));
      return jsonResponse(200, { mode: "rules", status: "supported", conclusion: "x", premise_ids: [] });
    },
  });
  await client.reasonSemantic("kb1", "q", "rules");
  assert.deepEqual(seen, { query: "q", mode: "rules" });
});
