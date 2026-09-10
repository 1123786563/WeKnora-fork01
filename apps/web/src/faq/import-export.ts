import type { FAQEntry, FAQEntryPayload } from '@weknora/api-client';

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

function parseCsvLine(line: string): string[] {
  const result: string[] = [];
  let field = '';
  let quoted = false;
  for (let i = 0; i < line.length; i += 1) {
    const char = line[i];
    if (char === '"' && quoted && line[i + 1] === '"') { field += '"'; i += 1; continue; }
    if (char === '"') { quoted = !quoted; continue; }
    if (char === ',' && !quoted) { result.push(field.trim()); field = ''; continue; }
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
  const header = parseCsvLine(lines.shift() ?? '').map((value) => value.toLowerCase());
  const column = (name: string, fallback: number) => { const index = header.indexOf(name); return index >= 0 ? index : fallback; };
  const readList = (value: string): string[] => value ? value.split(/\s*\|\s*/).map((item) => item.trim()).filter(Boolean) : [];
  return lines.map((line) => {
    const fields = parseCsvLine(line);
    return normalizeFAQPayload({
      standard_question: fields[column('standard_question', 0)],
      similar_questions: readList(fields[column('similar_questions', 1)] ?? ''),
      negative_questions: readList(fields[column('negative_questions', 2)] ?? ''),
      answers: readList(fields[column('answers', 3)] ?? ''),
    });
  });
}

export function serializeFAQEntries(entries: FAQEntry[], format: FAQImportFormat): string {
  if (format === 'json') return JSON.stringify(entries.map(({ id: _id, ...entry }) => entry), null, 2);
  const columns = ['standard_question', 'similar_questions', 'negative_questions', 'answers'];
  const rows = entries.map((entry) => [entry.standard_question, entry.similar_questions.join(' | '), entry.negative_questions.join(' | '), entry.answers.join(' | ')].map(csvValue).join(','));
  return [columns.join(','), ...rows].join('\n') + (rows.length ? '\n' : '');
}
