import assert from "node:assert/strict";
import test from "node:test";
import { applyMarqueeSelection, toggleDocumentSelection } from "./selection.ts";

test("marquee add keeps the base selection and adds every intersecting row", () => {
  const result = applyMarqueeSelection(new Set(["a"]), ["b", "c"], "add");
  assert.deepEqual([...result], ["a", "b", "c"]);
});

test("marquee subtract removes every intersecting row without toggling", () => {
  const result = applyMarqueeSelection(new Set(["a", "b", "c"]), ["b", "c"], "subtract");
  assert.deepEqual([...result], ["a"]);
});

test("shift-click selects the inclusive Vue document range", () => {
  const result = toggleDocumentSelection({ ids: ["a", "b", "c", "d"], selected: new Set(["a"]), id: "d", checked: true, shiftKey: true, lastIndex: 0 });
  assert.deepEqual([...result.selected], ["a", "b", "c", "d"]);
  assert.equal(result.lastIndex, 3);
});

test("shift-click removes the inclusive range when unchecked", () => {
  const result = toggleDocumentSelection({ ids: ["a", "b", "c", "d"], selected: new Set(["a", "b", "c", "d"]), id: "b", checked: false, shiftKey: true, lastIndex: 3 });
  assert.deepEqual([...result.selected], ["a"]);
});

test("normal click toggles only the clicked row", () => {
  const result = toggleDocumentSelection({ ids: ["a", "b", "c"], selected: new Set(["a"]), id: "c", checked: true, lastIndex: -1 });
  assert.deepEqual([...result.selected], ["a", "c"]);
});
