import { test } from "node:test";
import assert from "node:assert/strict";
import { parseSemanticStatus, SemanticContractError } from "../src/semantic.ts";

test("semantic revision preserves uint64 precision", () => {
  const value = parseSemanticStatus({
    document_id: "d1", revision: "18446744073709551615",
    semantic_status: "stale", active_generation: "g1"
  });
  assert.equal(value.revision, "18446744073709551615");
});

test("float revision rejected", () => {
  assert.throws(
    () => parseSemanticStatus({ document_id: "d1", revision: "1.5", semantic_status: "ready" }),
    SemanticContractError,
  );
});

test("negative revision rejected", () => {
  assert.throws(
    () => parseSemanticStatus({ document_id: "d1", revision: "-1", semantic_status: "ready" }),
    SemanticContractError,
  );
});

test("numeric revision rejected (must be string)", () => {
  assert.throws(
    () => parseSemanticStatus({ document_id: "d1", revision: 42, semantic_status: "ready" }),
    SemanticContractError,
  );
});

test("unknown status maps to compat unknown, never ready", () => {
  const value = parseSemanticStatus({
    document_id: "d1", revision: "7", semantic_status: "future-state-v2",
  });
  assert.equal(value.semantic_status, "unknown");
});

test("known statuses parse verbatim", () => {
  for (const status of ["disabled", "queued", "indexing", "ready", "stale", "failed", "deleting"]) {
    const value = parseSemanticStatus({ document_id: "d", revision: "1", semantic_status: status });
    assert.equal(value.semantic_status, status);
  }
});
