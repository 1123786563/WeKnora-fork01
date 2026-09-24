import type { EvidenceCitation } from './types.ts';

/** 已知来源键（取第一个命中的字符串值）；不命中则不编造 source。 */
const SOURCE_KEYS = ['knowledge_base_id', 'document_id', 'file_name', 'url', 'source', 'tool'] as const;

/**
 * 把 run 事件投影为 Evidence 引用：tool.* / artifact.* 事件是可追溯的来源
 * 事实（module-seams §7.1「Evidence Citation 和来源」）。detail 携带原始载荷
 * 摘要——与 Task 时间线「原始证据按需展开」同一纪律，不丢事实也不猜结构。
 */
export function projectCitations(
  events: ReadonlyArray<{ seq: number; type: string; occurredAt: string; payload: Record<string, unknown> }>,
): EvidenceCitation[] {
  const citations: EvidenceCitation[] = [];
  for (const event of events) {
    if (!event.type.startsWith('tool.') && !event.type.startsWith('artifact.')) continue;
    let source: string | undefined;
    for (const key of SOURCE_KEYS) {
      const value = event.payload?.[key];
      if (typeof value === 'string' && value.trim() !== '') {
        source = value;
        break;
      }
    }
    let detail: string;
    try {
      detail = JSON.stringify(event.payload ?? {});
    } catch {
      detail = '[unserializable]';
    }
    citations.push({ seq: event.seq, occurredAt: event.occurredAt, type: event.type, ...(source === undefined ? {} : { source }), detail });
  }
  return citations.sort((a, b) => a.seq - b.seq);
}
