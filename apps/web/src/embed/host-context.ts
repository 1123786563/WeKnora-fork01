// Vue parity: frontend/src/utils/embedContext.ts — the host page can inject
// key/value context over the bridge; it is prefixed onto every user query so
// the agent sees the embedding page's situation without UI changes.
export function buildQueryWithHostContext(
  query: string,
  hostContext?: Record<string, unknown>,
): string {
  if (!hostContext || !Object.keys(hostContext).length) return query;
  const lines = Object.entries(hostContext)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`);
  if (!lines.length) return query;
  return `[Host context]\n${lines.join('\n')}\n\n${query}`;
}
