import assert from 'node:assert/strict';
import test from 'node:test';
import * as XLSX from 'xlsx';
import { parseExcelFile } from './import-export.ts';

/** Build a minimal .xlsx File from rows; extraSheets are appended after the primary sheet. */
function xlsxFile(rows: string[][], extraSheets: string[][][] = []): File {
  const workbook = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet(rows), 'FAQ');
  extraSheets.forEach((sheetRows, index) => {
    XLSX.utils.book_append_sheet(workbook, XLSX.utils.aoa_to_sheet(sheetRows), 'ignored' + index);
  });
  const bytes = XLSX.write(workbook, { bookType: 'xlsx', type: 'array' }) as Uint8Array;
  // Copy so the buffer is a plain ArrayBuffer (BlobPart requirement under TS 6 DOM libs).
  return new File([new Uint8Array(bytes)], 'faq.xlsx', { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
}

test('parseExcelFile maps the Vue Chinese columns with ## delimiters from the first sheet only', async () => {
  const file = xlsxFile([
    // Exact Vue export-template headers (FAQEntryManager.vue:2499) — ASCII parens.
    ['标签(必填)', '问题(必填)', '相似问题(选填-多个用##分隔)', '反例问题(选填-多个用##分隔)', '机器人回答(必填-多个用##分隔)', '是否全部回复(选填-默认FALSE)', '是否停用(选填-默认FALSE)', '是否禁止被推荐(选填-默认False 可被推荐)', 'tag_id'],
    ['重要', '  WeKnora 是什么？ ', '它是什么？##What is WeKnora?', ' 这不是WeKnora ## 无关问题 ', ' 一个开源知识库 ## 支持多源检索 ', 'FALSE', 'FALSE', 'FALSE', '3'],
  ], [[
    ['问题'],
    ['second-sheet-row'],
  ]]);
  assert.deepEqual(await parseExcelFile(file), [
    {
      standard_question: 'WeKnora 是什么？',
      answers: ['一个开源知识库', '支持多源检索'],
      similar_questions: ['它是什么？', 'What is WeKnora?'],
      negative_questions: ['这不是WeKnora', '无关问题'],
      tag_id: 3,
      tag_name: '重要',
      is_enabled: true,
    },
  ]);
});

test('parseExcelFile falls back to lowercased English headers and omits disabled-by-default is_enabled', async () => {
  const file = xlsxFile([
    ['QUESTION', 'ANSWERS', 'Similar_Questions', 'Negative_Questions', '是否停用'],
    ['How?', 'A1 ## A2', 'S1', '', ''],
  ]);
  const parsed = await parseExcelFile(file);
  assert.deepEqual(parsed, [
    { standard_question: 'How?', answers: ['A1', 'A2'], similar_questions: ['S1'], negative_questions: [], tag_name: '' },
  ]);
  assert.equal('is_enabled' in parsed[0], false);
});

test('parseExcelFile mirrors Vue parseBooleanField on 是否停用 (TRUE/是 disable, unknown enables)', async () => {
  const file = xlsxFile([
    ['问题', '机器人回答', '是否停用'],
    ['q-empty', 'a', ''],
    ['q-true', 'a', 'TRUE'],
    ['q-yes-cn', 'a', '是'],
    ['q-unknown', 'a', 'maybe'],
  ]);
  const parsed = await parseExcelFile(file);
  assert.deepEqual(parsed.map((entry) => entry.is_enabled), [undefined, false, false, true]);
  // Vue normalizePayload always emits tag_name ('' when the column is absent).
  assert.ok(parsed.every((entry) => 'tag_name' in entry));
});
