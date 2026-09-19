import type { ClientRequest } from './client.ts';

export interface MbtiAxis { pole: string; percent: number }
export interface MbtiProfile {
  code: string; name_zh: string; name_en: string; nickname_zh: string;
  summary_zh: string; summary_en: string; descriptors_zh: string; descriptors_en: string;
  dimensions: Record<'ei'|'sn'|'tf'|'jp', MbtiAxis>;
  behavior: Record<'answer_style'|'casual_chat'|'conflict'|'creativity'|'emotion'|'planning'|'answer_style_zh'|'casual_chat_zh'|'conflict_zh'|'creativity_zh'|'emotion_zh'|'planning_zh', string>;
  color: string; symbol: string;
}
export interface MbtiQuestion {
  id: number; dimension: 'EI'|'SN'|'TF'|'JP'; a_pole: string; b_pole: string;
  question_zh: string; option_a_zh: string; option_b_zh: string;
  question_en: string; option_a_en: string; option_b_en: string;
}
export interface MbtiScore {
  code: string; dimensions: Record<'ei'|'sn'|'tf'|'jp', { dominant: string; percent: number }>; profile: MbtiProfile;
}

type RecordValue = Record<string, unknown>;
function record(value: unknown, path: string): RecordValue {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) throw new Error(`${path} must be an object`);
  return value as RecordValue;
}
function required(value: unknown, path: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${path} must be a non-empty string`);
  return value;
}
function numberValue(value: unknown, path: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw new Error(`${path} must be a finite number`);
  return value;
}
function successfulData(value: unknown, path: string): unknown {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
  if (!Object.prototype.hasOwnProperty.call(envelope, 'data')) throw new Error(`${path}.data is required`);
  return envelope.data;
}
function id(value: string, name: string): string {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`${name} must not be empty`);
  return encodeURIComponent(value);
}

const dimensionKeys = ['ei', 'sn', 'tf', 'jp'] as const;
const behaviorKeys = [
  'answer_style', 'casual_chat', 'conflict', 'creativity', 'emotion', 'planning',
  'answer_style_zh', 'casual_chat_zh', 'conflict_zh', 'creativity_zh', 'emotion_zh', 'planning_zh',
] as const;
const questionDimensions = ['EI', 'SN', 'TF', 'JP'] as const;

function parseAxis(value: unknown, path: string): MbtiAxis {
  const row = record(value, path);
  return { pole: required(row.pole, `${path}.pole`), percent: numberValue(row.percent, `${path}.percent`) };
}

function parseDimensions(value: unknown, path: string): MbtiProfile['dimensions'] {
  const row = record(value, path);
  const result = {} as MbtiProfile['dimensions'];
  for (const key of dimensionKeys) result[key] = parseAxis(row[key], `${path}.${key}`);
  return result;
}

function parseBehavior(value: unknown, path: string): MbtiProfile['behavior'] {
  const row = record(value, path);
  const result = {} as MbtiProfile['behavior'];
  for (const key of behaviorKeys) result[key] = required(row[key], `${path}.${key}`);
  return result;
}

function parseProfile(value: unknown, path: string): MbtiProfile {
  const row = record(value, path);
  return {
    code: required(row.code, `${path}.code`),
    name_zh: required(row.name_zh, `${path}.name_zh`),
    name_en: required(row.name_en, `${path}.name_en`),
    nickname_zh: required(row.nickname_zh, `${path}.nickname_zh`),
    summary_zh: required(row.summary_zh, `${path}.summary_zh`),
    summary_en: required(row.summary_en, `${path}.summary_en`),
    descriptors_zh: required(row.descriptors_zh, `${path}.descriptors_zh`),
    descriptors_en: required(row.descriptors_en, `${path}.descriptors_en`),
    dimensions: parseDimensions(row.dimensions, `${path}.dimensions`),
    behavior: parseBehavior(row.behavior, `${path}.behavior`),
    color: required(row.color, `${path}.color`),
    symbol: required(row.symbol, `${path}.symbol`),
  };
}

function parseProfiles(value: unknown, path: string): MbtiProfile[] {
  const data = successfulData(value, path);
  const row = record(data, `${path}.data`);
  if (!Array.isArray(row.types)) throw new Error(`${path}.data.types must be an array`);
  return row.types.map((item, index) => parseProfile(item, `${path}.data.types[${index}]`));
}

function parseQuestion(value: unknown, path: string): MbtiQuestion {
  const row = record(value, path);
  if (!questionDimensions.includes(row.dimension as MbtiQuestion['dimension'])) throw new Error(`${path}.dimension is invalid`);
  return {
    id: numberValue(row.id, `${path}.id`),
    dimension: row.dimension as MbtiQuestion['dimension'],
    a_pole: required(row.a_pole, `${path}.a_pole`),
    b_pole: required(row.b_pole, `${path}.b_pole`),
    question_zh: required(row.question_zh, `${path}.question_zh`),
    option_a_zh: required(row.option_a_zh, `${path}.option_a_zh`),
    option_b_zh: required(row.option_b_zh, `${path}.option_b_zh`),
    question_en: required(row.question_en, `${path}.question_en`),
    option_a_en: required(row.option_a_en, `${path}.option_a_en`),
    option_b_en: required(row.option_b_en, `${path}.option_b_en`),
  };
}

function parseQuestions(value: unknown, path: string): MbtiQuestion[] {
  const data = successfulData(value, path);
  const row = record(data, `${path}.data`);
  if (!Array.isArray(row.questions)) throw new Error(`${path}.data.questions must be an array`);
  return row.questions.map((item, index) => parseQuestion(item, `${path}.data.questions[${index}]`));
}

function parseScore(value: unknown, path: string): MbtiScore {
  const data = record(successfulData(value, path), `${path}.data`);
  const row = record(data.dimensions, `${path}.data.dimensions`);
  const dimensions = {} as MbtiScore['dimensions'];
  for (const key of dimensionKeys) {
    const axis = record(row[key], `${path}.data.dimensions.${key}`);
    dimensions[key] = { dominant: required(axis.dominant, `${path}.data.dimensions.${key}.dominant`), percent: numberValue(axis.percent, `${path}.data.dimensions.${key}.percent`) };
  }
  return { code: required(data.code, `${path}.data.code`), dimensions, profile: parseProfile(data.profile, `${path}.data.profile`) };
}

function parsePreview(value: unknown, path: string): { code: string; markdown: string } {
  const data = record(successfulData(value, path), `${path}.data`);
  return { code: required(data.code, `${path}.data.code`), markdown: required(data.markdown, `${path}.data.markdown`) };
}

function parseActionEnvelope(value: unknown, path: string): void {
  const envelope = record(value, path);
  if (envelope.success !== true) throw new Error(`${path}.success must be true`);
}

/**
 * MBTI persona API (internal/handler/persona.go, routes_persona.go). Catalog,
 * preview, and test endpoints are Viewer+; apply/remove on an agent use the
 * agent-update ownership guard. Responses ride the standard success envelope
 * and are unwrapped/validated here like every other module.
 */
export function createMbtiApi(request: (input: ClientRequest) => Promise<unknown>) {
  return {
    async types(signal?: AbortSignal): Promise<MbtiProfile[]> {
      return parseProfiles(await request({ method: 'GET', path: '/api/v1/mbti/types', ...(signal === undefined ? {} : { signal }) }), '/mbti/types');
    },
    async type(code: string, signal?: AbortSignal): Promise<MbtiProfile> {
      return parseProfile(successfulData(await request({ method: 'GET', path: `/api/v1/mbti/types/${id(code, 'code')}`, ...(signal === undefined ? {} : { signal }) }), '/mbti/types/:code'), '/mbti/types/:code.data');
    },
    async preview(code: string, signal?: AbortSignal): Promise<{ code: string; markdown: string }> {
      return parsePreview(await request({ method: 'GET', path: `/api/v1/mbti/preview/${id(code, 'code')}`, ...(signal === undefined ? {} : { signal }) }), '/mbti/preview/:code');
    },
    async questions(signal?: AbortSignal): Promise<MbtiQuestion[]> {
      return parseQuestions(await request({ method: 'GET', path: '/api/v1/mbti/test/questions', ...(signal === undefined ? {} : { signal }) }), '/mbti/test/questions');
    },
    async submit(answers: Record<string, 'A' | 'B'>, signal?: AbortSignal): Promise<MbtiScore> {
      if (answers === null || typeof answers !== 'object' || Array.isArray(answers)) throw new Error('answers must be an object');
      for (const [key, choice] of Object.entries(answers)) {
        if (key.trim() === '') throw new Error('answers keys must be question ids');
        if (choice !== 'A' && choice !== 'B') throw new Error(`answers.${key} must be "A" or "B"`);
      }
      return parseScore(await request({ method: 'POST', path: '/api/v1/mbti/test/submit', body: { answers }, ...(signal === undefined ? {} : { signal }) }), '/mbti/test/submit');
    },
    async applyPersona(agentId: string, code: string, style?: string, signal?: AbortSignal): Promise<void> {
      parseActionEnvelope(await request({
        method: 'PUT', path: `/api/v1/agents/${id(agentId, 'agentId')}/persona`,
        body: { code, ...(style === undefined ? {} : { style }) },
        ...(signal === undefined ? {} : { signal }),
      }), '/agents/:id/persona PUT');
    },
    async removePersona(agentId: string, signal?: AbortSignal): Promise<void> {
      parseActionEnvelope(await request({ method: 'DELETE', path: `/api/v1/agents/${id(agentId, 'agentId')}/persona`, ...(signal === undefined ? {} : { signal }) }), '/agents/:id/persona DELETE');
    },
  };
}

export type MbtiApi = ReturnType<typeof createMbtiApi>;
