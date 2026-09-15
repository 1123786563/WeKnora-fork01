export type SharePermission = 'viewer' | 'editor' | 'admin'
export interface ShareRequest { organization_id: string; permission: SharePermission }
export interface KnowledgeBaseShare { id?: string; [key: string]: unknown }

export function buildShareRequest(input: { organizationId: string; permission: SharePermission }): ShareRequest {
  return { organization_id: input.organizationId, permission: input.permission }
}

type ShareState = { status: 'empty'; shares: KnowledgeBaseShare[] } | { status: 'ready'; shares: KnowledgeBaseShare[] } | { status: 'error'; shares: KnowledgeBaseShare[]; message: string }
function unwrap(value: unknown): KnowledgeBaseShare[] {
  if (Array.isArray(value)) return value.filter((item): item is KnowledgeBaseShare => Boolean(item) && typeof item === 'object')
  if (value && typeof value === 'object') {
    const record = value as { shares?: unknown; data?: unknown }
    if (Array.isArray(record.shares)) return unwrap(record.shares)
    return unwrap(record.data)
  }
  return []
}

export async function loadKnowledgeBaseShares(request: () => Promise<unknown>): Promise<ShareState> {
  try { const shares = unwrap(await request()); return shares.length ? { status: 'ready', shares } : { status: 'empty', shares } }
  catch (error: unknown) { return { status: 'error', shares: [], message: error instanceof Error ? error.message : 'Unable to load shares' } }
}

export type ShareMutationResult<T> = { status: 'success'; value: T } | { status: 'error'; message: string }
export function mutateKnowledgeBaseShare<Input, Output>(operation: (input: Input) => Promise<Output>) {
  let pending: Promise<ShareMutationResult<Output>> | undefined
  return (input: Input) => {
    if (pending) return pending
    const current = operation(input).then((value) => ({ status: 'success', value }) as const).catch((error: unknown) => ({ status: 'error' as const, message: error instanceof Error ? error.message : 'Unable to update share' })).finally(() => { pending = undefined })
    pending = current
    return current
  }
}
