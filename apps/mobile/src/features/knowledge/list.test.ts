import assert from "node:assert/strict";
import test from "node:test";
import { canCreateKnowledgeBase, knowledgeBaseCountLabel, knowledgeListLabel } from "./list.ts";

test("mobile knowledge creation follows the Vue contributor/admin role gate", () => {
  assert.equal(canCreateKnowledgeBase("owner"), true);
  assert.equal(canCreateKnowledgeBase("admin"), true);
  assert.equal(canCreateKnowledgeBase("contributor"), true);
  assert.equal(canCreateKnowledgeBase("viewer"), false);
  assert.equal(canCreateKnowledgeBase(undefined), false);
});

test("mobile knowledge card count uses the available server count", () => {
  assert.equal(
    knowledgeBaseCountLabel({
      type: "faq",
      chunk_count: 4,
      knowledge_count: 99,
    }),
    "FAQ · 4 items",
  );
  assert.equal(
    knowledgeBaseCountLabel({ type: "document", knowledge_count: 7 }),
    "Documents · 7 items",
  );
  assert.equal(knowledgeBaseCountLabel({}), "Documents · 0 items");
});

test("mobile knowledge labels use the shared locale catalog", () => {
  assert.equal(knowledgeListLabel("zh-CN", "knowledgeList.create"), "新建知识库");
  assert.equal(knowledgeListLabel("en-US", "knowledgeList.create"), "Create Knowledge Base");
  assert.equal(knowledgeListLabel("ja-JP", "menu.logout"), "ログアウト");
  assert.equal(knowledgeListLabel("zh-CN", "knowledgeList.loadFailed"), "加载知识库失败");
});
