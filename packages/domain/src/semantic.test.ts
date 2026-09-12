import { test } from "node:test";
import assert from "node:assert/strict";
import { deriveSemanticView } from "./semantic.ts";

test("semantic failure keeps parsed text readable", () => {
  const view = deriveSemanticView({document_id:"d1",revision:"1",semantic_status:"failed"});
  assert.equal(view.canReadText, true);
  assert.equal(view.canRetry, true);
});

test("deleting document cannot retry semantic indexing", () => {
  assert.equal(deriveSemanticView({document_id:"d1",revision:"2",semantic_status:"deleting"}).canRetry, false);
});

test("ready status is not retryable", () => {
  assert.equal(deriveSemanticView({document_id:"d1",revision:"1",semantic_status:"ready"}).canRetry, false);
});

test("stale status is retryable", () => {
  assert.equal(deriveSemanticView({document_id:"d1",revision:"1",semantic_status:"stale"}).canRetry, true);
});

test("unknown status falls to unknown label, not retryable", () => {
  const view = deriveSemanticView({document_id:"d1",revision:"1",semantic_status:"unknown"});
  assert.equal(view.label, "状态未知");
  assert.equal(view.canRetry, false);
});

test("every semantic state keeps text readable", () => {
  for (const s of ["disabled","queued","indexing","ready","stale","failed","deleting","unknown"]) {
    assert.equal(deriveSemanticView({document_id:"d",revision:"1",semantic_status:s}).canReadText, true, s);
  }
});
