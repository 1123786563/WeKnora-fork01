import test from 'node:test';
import assert from 'node:assert/strict';
import { parseUnifiedDiff } from './diff.ts';

const SAMPLE = `--- a/src/app.ts
+++ b/src/app.ts
@@ -1,5 +1,6 @@
 import { createApp } from './app.ts';
+import { logger } from './logger.ts';

 function main() {
-  createApp();
+  createApp({ log: logger });
 }
@@ -10,2 +11,3 @@
 export function helper() {
-  return 1;
+  return 2;
+}
`;

test('a unified diff parses into hunks with add/remove/context lines', () => {
  const { hunks, malformed } = parseUnifiedDiff(SAMPLE);
  assert.equal(malformed, false);
  assert.equal(hunks.length, 2);
  assert.equal(hunks[0]!.oldStart, 1);
  assert.equal(hunks[0]!.newStart, 1);
  // 第 3 行是真空行（编辑器常剥掉 context 行的前导空格）——按 context 解析，不判畸形。
  assert.deepEqual(hunks[0]!.lines.map((line) => line.origin), ['context', 'add', 'context', 'context', 'remove', 'add', 'context']);
  assert.equal(hunks[0]!.lines[1]!.text, "import { logger } from './logger.ts';");
});

test('malformed content is reported instead of half-parsed hunks', () => {
  const broken = parseUnifiedDiff('this is not\na diff at all\n');
  assert.equal(broken.malformed, true);
  assert.deepEqual(broken.hunks, []);

  // + 行出现在任何 hunk 之前：无法归类，整体 malformed（hunk 内的 + 行是合法 add，二者靠位置区分）。
  const dangling = parseUnifiedDiff('--- a/x\n+++ b/x\n+ dangling add before any hunk\n');
  assert.equal(dangling.malformed, true);
  assert.deepEqual(dangling.hunks, [], '一旦发现畸形就整体回退原始文本，不呈现半解析结果');
});

test('an empty diff is a valid empty diff, not malformed input', () => {
  assert.deepEqual(parseUnifiedDiff(''), { hunks: [], malformed: false });
  assert.deepEqual(parseUnifiedDiff('--- a/x\n+++ b/x\n'), { hunks: [], malformed: false });
});

test('the no-newline marker is tolerated inside a hunk', () => {
  const { hunks, malformed } = parseUnifiedDiff('@@ -1 +1 @@\n-a\n\\ No newline at end of file\n+b\n');
  assert.equal(malformed, false);
  assert.deepEqual(hunks[0]!.lines.map((line) => line.origin), ['remove', 'add']);
});
