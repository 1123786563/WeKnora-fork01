import type { FAQEntry, FAQEntryPayload } from '@weknora/api-client';
import * as XLSX from 'xlsx';

export type FAQImportFormat = 'json' | 'csv';

function strings(value: unknown, field: string, required = false): string[] {
  if (value === undefined && !required) return [];
  if (!Array.isArray(value) || value.some((item) => typeof item !== 'string')) throw new Error(`${field} must be an array of strings`);
  return value.map((item) => item.trim()).filter(Boolean);
}

export function normalizeFAQPayload(value: Partial<FAQEntryPayload>): FAQEntryPayload {
  const question = value.standard_question?.trim() ?? '';
  if (!question) throw new Error('Standard question is required');
  const answers = strings(value.answers, 'answers', true);
  if (answers.length === 0) throw new Error('At least one answer is required');
  if (value.tag_id !== undefined && value.tag_id !== null && (!Number.isSafeInteger(value.tag_id) || value.tag_id < 0)) throw new Error('tag_id must be a non-negative integer');
  return {
    standard_question: question,
    similar_questions: strings(value.similar_questions, 'similar_questions'),
    negative_questions: strings(value.negative_questions, 'negative_questions'),
    answers,
    ...(value.tag_id === undefined ? {} : { tag_id: value.tag_id }),
    ...(value.is_enabled === undefined ? {} : { is_enabled: value.is_enabled }),
    ...(value.is_recommended === undefined ? {} : { is_recommended: value.is_recommended }),
  };
}

function parseCsvLine(line: string, delimiter = ','): string[] {
  const result: string[] = [];
  let field = '';
  let quoted = false;
  for (let i = 0; i < line.length; i += 1) {
    const char = line[i];
    if (char === '"' && quoted && line[i + 1] === '"') { field += '"'; i += 1; continue; }
    if (char === '"') { quoted = !quoted; continue; }
    if (char === delimiter && !quoted) { result.push(field.trim()); field = ''; continue; }
    field += char;
  }
  if (quoted) throw new Error('CSV contains an unterminated quoted field');
  result.push(field.trim());
  return result;
}

function csvValue(value: string): string { return /[",\n]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value; }

export function parseFAQImportText(text: string, format: FAQImportFormat): FAQEntryPayload[] {
  if (!text.trim()) throw new Error('Import file is empty');
  if (format === 'json') {
    const value: unknown = JSON.parse(text);
    if (!Array.isArray(value)) throw new Error('JSON import must be an array');
    return value.map((item) => normalizeFAQPayload(item as Partial<FAQEntryPayload>));
  }
  const lines = text.replace(/^\uFEFF/, '').split(/\r?\n/).filter((line) => line.trim());
  const rawHeader = lines.shift() ?? '';
  const delimiter = rawHeader.includes('\t') && !rawHeader.includes(',') ? '\t' : ',';
  const normalizeHeader = (value: string) => {
    const cleaned = value.trim().replace(/\([^)]*\)/g, '').trim();
    return /[\u4e00-\u9fa5]/.test(cleaned) ? cleaned : cleaned.toLowerCase();
  };
  const header = parseCsvLine(rawHeader, delimiter).map(normalizeHeader);
  const column = (names: string[], fallback: number) => {
    const index = names.map((name) => header.indexOf(name)).find((value) => value >= 0);
    return index === undefined ? fallback : index;
  };
  const readList = (value: string): string[] => splitByDelimiter(value);
  const tagColumn = column(['tag_name', '标签', '分类'], -1);
  return lines.map((line) => {
    const fields = parseCsvLine(line, delimiter);
    const disabled = parseBooleanField(fields[header.indexOf('是否停用')], false);
    const payload = normalizeExcelPayload({
      standard_question: fields[column(['standard_question', '问题'], 0)] ?? '',
      similar_questions: readList(fields[column(['similar_questions', '相似问题'], 1)] ?? ''),
      negative_questions: readList(fields[column(['negative_questions', '反例问题'], 2)] ?? ''),
      answers: readList(fields[column(['answers', '机器人回答'], 3)] ?? ''),
      ...(tagColumn >= 0 ? { tag_name: fields[tagColumn] ?? '' } : {}),
      is_enabled: disabled === undefined ? undefined : !disabled,
    });
    if (tagColumn >= 0) return payload;
    const { tag_name: _tagName, ...withoutTag } = payload;
    return withoutTag;
  });
}

// Vue FAQEntryManager.vue:2035-2051 — '##' is the only list delimiter so answers
// containing commas/semicolons survive; a value without '##' stays one item.
function splitByDelimiter(value?: string): string[] {
  if (!value) return [];
  const trimmedValue = value.trim();
  if (!trimmedValue) return [];
  if (trimmedValue.includes('##')) {
    return trimmedValue.split('##').map((item) => item.trim()).filter(Boolean);
  }
  return [trimmedValue];
}

// Vue FAQEntryManager.vue:2053-2064 — TRUE/1/是/YES → true, FALSE/0/否/NO → false,
// empty → undefined, anything else → the caller's default (Excel passes false).
function parseBooleanField(value: string | undefined, defaultValue: boolean): boolean | undefined {
  if (!value) return undefined;
  const normalized = value.trim().toUpperCase();
  if (normalized === 'TRUE' || normalized === '1' || normalized === '是' || normalized === 'YES') return true;
  if (normalized === 'FALSE' || normalized === '0' || normalized === '否' || normalized === 'NO') return false;
  return defaultValue;
}

// Vue FAQEntryManager.vue:2066-2074 normalizePayload — deliberately lenient
// (no "question required" throw): invalid rows are reported per-row by the
// backend import task, mirroring the Vue flow. Undefined tag_id/is_enabled are
// omitted so the request body matches the JSON import path's wire format.
function normalizeExcelPayload(payload: {
  standard_question: string;
  answers: string[];
  similar_questions: string[];
  negative_questions: string[];
  tag_id?: number;
  tag_name?: string;
  is_enabled?: boolean;
}): FAQEntryPayload {
  // Vue normalizePayload (:2072): tag_name is always present ('' when the
  // column is absent), matching the backend import contract (faq.go:336).
  return {
    standard_question: payload.standard_question || '',
    answers: payload.answers?.filter(Boolean) || [],
    similar_questions: payload.similar_questions?.filter(Boolean) || [],
    negative_questions: payload.negative_questions?.filter(Boolean) || [],
    tag_name: payload.tag_name || '',
    ...(payload.tag_id ? { tag_id: payload.tag_id } : {}),
    ...(payload.is_enabled !== undefined ? { is_enabled: payload.is_enabled } : {}),
  };
}

// Vue FAQEntryManager.vue:1999-2033 parseExcelFile — first worksheet only,
// sheet_to_json with defval '' + raw:false, headers normalized by stripping
// parenthetical notes and lowercasing non-Chinese names, then the same column
// fallbacks as the Vue CSV path (问题/standard_question/question, 机器人回答/answers,
// 相似问题/similar_questions, 反例问题/negative_questions, 是否停用 inverted into
// is_enabled, numeric tag_id). The Vue 标签/分类/tag_name column has no
// FAQEntryPayload field on the React client, so it is not emitted.
export async function parseExcelFile(file: File): Promise<FAQEntryPayload[]> {
  const data = await file.arrayBuffer();
  const workbook = XLSX.read(data, { type: 'array' });
  const sheetName = workbook.SheetNames[0];
  const worksheet = workbook.Sheets[sheetName];
  // raw:false keeps formatted strings (same rationale as the Vue comment).
  const json = XLSX.utils.sheet_to_json<Record<string, string>>(worksheet, { defval: '', raw: false });
  return json.map((row) => {
    const normalizedRow: Record<string, string> = {};
    Object.keys(row).forEach((key) => {
      const normalizedKey = key.trim()
        .replace(/\([^)]*\)/g, '') // 移除括号及内容
        .trim();
      // 对于中文字段名，不转换为小写；对于英文字段名，转换为小写
      const finalKey = /[\u4e00-\u9fa5]/.test(normalizedKey) ? normalizedKey : normalizedKey.toLowerCase();
      normalizedRow[finalKey] = String(row[key] || '').trim();
    });
    const isDisabled = parseBooleanField(normalizedRow['是否停用'], false);
    return normalizeExcelPayload({
      standard_question: normalizedRow['问题'] || normalizedRow['standard_question'] || normalizedRow['question'] || '',
      answers: splitByDelimiter(normalizedRow['机器人回答'] || normalizedRow['answers']),
      similar_questions: splitByDelimiter(normalizedRow['相似问题'] || normalizedRow['similar_questions']),
      negative_questions: splitByDelimiter(normalizedRow['反例问题'] || normalizedRow['negative_questions']),
      tag_id: normalizedRow['tag_id'] ? Number(normalizedRow['tag_id']) : undefined,
      tag_name: normalizedRow['标签'] || normalizedRow['分类'] || normalizedRow['tag_name'] || '',
      is_enabled: isDisabled !== undefined ? !isDisabled : undefined, // 是否停用取反：FALSE 启用 / TRUE 停用
    });
  });
}

export function serializeFAQEntries(entries: FAQEntry[], format: FAQImportFormat): string {
  if (format === 'json') return JSON.stringify(entries.map(({ id: _id, ...entry }) => entry), null, 2);
  const columns = ['standard_question', 'similar_questions', 'negative_questions', 'answers'];
  const rows = entries.map((entry) => [entry.standard_question, entry.similar_questions.join(' | '), entry.negative_questions.join(' | '), entry.answers.join(' | ')].map(csvValue).join(','));
  return [columns.join(','), ...rows].join('\n') + (rows.length ? '\n' : '');
}
