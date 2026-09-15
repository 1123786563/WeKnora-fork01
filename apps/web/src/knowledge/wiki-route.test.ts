import test from "node:test";
import assert from "node:assert/strict";
import { shouldOpenWiki, wikiEntryPath } from "./wiki-route.ts";

test("disabled Wiki capability falls back to the Vue documents entry", () => {
  assert.equal(shouldOpenWiki({ indexing_strategy: { wiki_enabled: false } }), false);
  assert.equal(wikiEntryPath("kb/a"), "/knowledgeBase/kb%2Fa");
});

test("Wiki entry remains available only for an explicitly enabled capability", () => {
  assert.equal(shouldOpenWiki({ indexing_strategy: { wiki_enabled: true } }), true);
  assert.equal(shouldOpenWiki({}), false);
  assert.equal(shouldOpenWiki(null), false);
});
