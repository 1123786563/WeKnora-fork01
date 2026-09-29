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

test('native git diffs parse: index lines, new-file headers and multi-file sections', () => {
  const single = [
    'diff --git a/src/app.ts b/src/app.ts',
    'index 3f7a1c2..9b4e8d1 100644',
    '--- a/src/app.ts',
    '+++ b/src/app.ts',
    '@@ -1,3 +1,4 @@',
    ' import { app } from "./app";',
    '+import { log } from "./log";',
    ' ',
    ' export default app;',
  ].join('\n') + '\n';
  const parsed = parseUnifiedDiff(single);
  assert.equal(parsed.malformed, false);
  assert.equal(parsed.hunks.length, 1);
  assert.deepEqual(parsed.hunks[0]!.lines.map((line) => line.origin), ['context', 'add', 'context', 'context']);

  const newFile = [
    'diff --git a/notes.md b/notes.md',
    'new file mode 100644',
    'index 0000000..1111111',
    '--- /dev/null',
    '+++ b/notes.md',
    '@@ -0,0 +1,2 @@',
    '+# Notes',
    '+hello',
  ].join('\n') + '\n';
  const parsedNew = parseUnifiedDiff(newFile);
  assert.equal(parsedNew.malformed, false);
  assert.equal(parsedNew.hunks.length, 1);
  assert.deepEqual(parsedNew.hunks[0]!.lines.map((line) => line.origin), ['add', 'add']);

  const multi = [
    'diff --git a/a.ts b/a.ts',
    'index 1111111..2222222 100644',
    '--- a/a.ts',
    '+++ b/a.ts',
    '@@ -1 +1 @@',
    '-old',
    '+new',
    'diff --git a/b.ts b/b.ts',
    'index 3333333..4444444 100644',
    '--- a/b.ts',
    '+++ b/b.ts',
    '@@ -2 +2 @@',
    '-two',
    '+TWO',
    'diff --git a/logo.png b/logo.png',
    'index 5555555..6666666 100644',
    'Binary files a/logo.png and b/logo.png differ',
  ].join('\n') + '\n';
  const parsedMulti = parseUnifiedDiff(multi);
  assert.equal(parsedMulti.malformed, false, '第二段起的 diff --git 复位段边界：多文件 git diff 不判畸形');
  assert.equal(parsedMulti.hunks.length, 2, '两段各一个 hunk；Binary files 段无 hunk 但不破坏整体');
  assert.deepEqual(parsedMulti.hunks[1]!.lines.map((line) => line.origin), ['remove', 'add']);
});

test('rename and mode-change git headers stay outside hunks without breaking parsing', () => {
  const renamed = [
    'diff --git a/old.ts b/new.ts',
    'old mode 100644',
    'new mode 100755',
    'similarity index 90%',
    'rename from old.ts',
    'rename to new.ts',
    'index 7777777..8888888 100644',
    '--- a/old.ts',
    '+++ b/new.ts',
    '@@ -1 +1 @@',
    '-a',
    '+b',
  ].join('\n') + '\n';
  const parsed = parseUnifiedDiff(renamed);
  assert.equal(parsed.malformed, false);
  assert.equal(parsed.hunks.length, 1);
  assert.deepEqual(parsed.hunks[0]!.lines.map((line) => line.origin), ['remove', 'add']);
});

test('a hunk line starting with --- stays malformed instead of misparsing as a section boundary', () => {
  const ambiguous = '@@ -1,2 +1 @@\n ctx\n--- looks like a header but is a remove line\n';
  const parsed = parseUnifiedDiff(ambiguous);
  assert.equal(parsed.malformed, true, 'hunk 内以 --- 开头的行与文件头有歧义：判畸形回退原文，绝不产出错位的半解析结果');
  assert.deepEqual(parsed.hunks, []);
});
