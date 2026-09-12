import { test } from "node:test";
import assert from "node:assert/strict";
import {
  initialSemanticQueryState, beginSemanticQuery, acceptSemanticResponse,
  failSemanticQuery, cancelSemanticQuery, conclusionLabel,
} from "./view-model.ts";

test("late response for superseded query is discarded", () => {
  let state = beginSemanticQuery(initialSemanticQueryState(), "q1");
  state = beginSemanticQuery(state, "q2"); // user re-queried
  state = acceptSemanticResponse(state, "q1", { mode: "graphrag", truncated: false, evidence: [] }, false);
  assert.equal(state.result, null, "q1's late response must not render");
});

test("aborted response is discarded", () => {
  let state = beginSemanticQuery(initialSemanticQueryState(), "q1");
  state = acceptSemanticResponse(state, "q1", { mode: "graphrag", truncated: false, evidence: [] }, true);
  assert.equal(state.result, null);
});

test("current query response lands", () => {
  let state = beginSemanticQuery(initialSemanticQueryState(), "q1");
  state = acceptSemanticResponse(state, "q1", { mode: "graphrag", truncated: false, evidence: [] }, false);
  assert.notEqual(state.result, null);
});

test("permission error clears results", () => {
  let state = beginSemanticQuery(initialSemanticQueryState(), "q1");
  state = acceptSemanticResponse(state, "q1", { mode: "graphrag", truncated: false, evidence: [] }, false);
  state = failSemanticQuery(state, "q1", "无权访问", 403);
  assert.equal(state.result, null, "revoked access must clear knowledge from view");
  assert.equal(state.error, "无权访问");
});

test("cancel keeps discarding late responses", () => {
  let state = beginSemanticQuery(initialSemanticQueryState(), "q1");
  state = cancelSemanticQuery(state);
  assert.equal(state.loading, false);
  state = acceptSemanticResponse(state, "q1", { mode: "graphrag", truncated: false, evidence: [] }, false);
  assert.equal(state.result, null, "cancelled query's response must not render");
});

test("model inference never labeled proven", () => {
  assert.match(conclusionLabel("model"), /非证明/);
  assert.equal(conclusionLabel("rule"), "规则推导");
});
