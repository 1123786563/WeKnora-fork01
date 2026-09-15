import type { KnowledgeBaseActivityEntry } from '@weknora/api-client';

export type ActivityTone = 'success' | 'warning' | 'error' | 'primary' | 'default';

export function activityDateTime(value: string, locale?: string): { date: string; time: string } {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return { date: value, time: '' };
  return {
    date: date.toLocaleDateString(locale),
    time: date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' }),
  };
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
