import type { KnowledgeBaseActivityEntry } from '@weknora/api-client';

/** A response may update the panel only while it belongs to the latest filter/KB generation. */
export function isCurrentActivityGeneration(requestGeneration: number, currentGeneration: number): boolean {
  return requestGeneration === currentGeneration;
}

export type ActivityTone = 'success' | 'warning' | 'error' | 'primary' | 'default';

export function activityDateTime(value: string, locale?: string): { date: string; time: string } {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return { date: value, time: '' };
  // R490 A9 (Vue KnowledgeBaseActivitySettings.vue formatDatePart/formatTimePart
  // :515-540): the date renders with 2-digit month/day (2026/09/19, not
  // 2026/9/19) and the clock keeps seconds on a 24h face (00:22:01) — the
  // default toLocaleDateString/toLocaleTimeString pair produced unpadded
  // month/day and dropped the seconds.
  try {
    return {
      date: new Intl.DateTimeFormat(locale || 'zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' }).format(date),
      time: new Intl.DateTimeFormat(locale || 'zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(date),
    };
  } catch {
    return { date: value, time: '' };
  }
}

export function activityActionTone(action: string): ActivityTone {
  if (action.includes('deleted') || action.includes('removed') || action.includes('canceled')) return 'warning';
  if (action.includes('failed')) return 'error';
  if (action.includes('created') || action.includes('completed') || action.includes('added')) return 'success';
  if (action.includes('started') || action.includes('updated') || action.includes('changed')) return 'primary';
  return 'default';
}

export function activityOutcomeTone(outcome: string): ActivityTone {
  if (outcome === 'failed' || outcome === 'denied') return 'error';
  if (outcome === 'partial' || outcome === 'canceled') return 'warning';
  if (outcome === 'accepted') return 'primary';
  if (outcome === 'success' || outcome === 'completed') return 'success';
  return 'default';
}

export function activityTargetSummary(entry: KnowledgeBaseActivityEntry): { subject: string; change: string } {
  const details = entry.details && typeof entry.details === 'object' && !Array.isArray(entry.details)
    ? entry.details as Record<string, unknown>
    : {};
  const subject = String(details.title ?? details.name ?? entry.target_name ?? entry.knowledge_base_name ?? '').trim();
  if (details.permission !== undefined) return { subject, change: String(details.permission) };
  if (details.count !== undefined) {
    const failed = Number(details.failed ?? 0);
    const skipped = Number(details.skipped ?? 0);
    const count = Number(details.count ?? 0);
    const suffix = failed || skipped ? ` (${failed} failed, ${skipped} skipped)` : '';
    return { subject, change: `${count}${suffix}` };
  }
  return { subject, change: '' };
}
