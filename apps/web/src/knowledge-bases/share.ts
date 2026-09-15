export type SharePermission = 'viewer' | 'editor' | 'admin'
export interface ShareRequest { organization_id: string; permission: SharePermission }
export interface KnowledgeBaseShare { id?: string; [key: string]: unknown }

export interface ShareOrganization {
  id: string
  name: string
  is_owner?: boolean
  my_role?: string
  [key: string]: unknown
}

export function getShareableOrganizations(
  organizations: ShareOrganization[],
  shares: Array<{ organization_id?: unknown }>,
): ShareOrganization[] {
  const sharedIds = new Set(shares.map((share) => share.organization_id).filter((id): id is string => typeof id === 'string'))
  return organizations.filter((organization) =>
    !sharedIds.has(organization.id) &&
    (organization.is_owner === true || organization.my_role === 'admin' || organization.my_role === 'editor'),
  )
}

export type ShareDialogStatus = 'idle' | 'submitting' | 'success' | 'error' | 'forbidden'
export interface ShareDialogState {
  open: boolean
  organizationId: string
  permission: Extract<SharePermission, 'viewer' | 'editor'>
  status: ShareDialogStatus
  message?: string
}

type ShareDialogEvent =
  | { type: 'open' }
  | { type: 'select-organization'; organizationId: string }
  | { type: 'set-permission'; permission: Extract<SharePermission, 'viewer' | 'editor'> }
  | { type: 'submit'; canShare: boolean }
  | { type: 'success' }
  | { type: 'failure'; message: string }
  | { type: 'cancel' }
  | { type: 'escape' }

const freshShareDialog = (): ShareDialogState => ({ open: true, organizationId: '', permission: 'viewer', status: 'idle' })
function clearDialogMessage(state: ShareDialogState): Omit<ShareDialogState, 'message'> {
  const { message: _message, ...withoutMessage } = state
  return withoutMessage
}

export function reduceShareDialog(state: ShareDialogState | undefined, event: ShareDialogEvent): ShareDialogState {
  if (event.type === 'open') return freshShareDialog()
  const current = state ?? { open: false, organizationId: '', permission: 'viewer', status: 'idle' as const }
  if (event.type === 'cancel' || event.type === 'escape') {
    return { open: false, organizationId: '', permission: 'viewer', status: current.status === 'success' ? 'success' : 'idle' }
  }
  if (!current.open) return current
  if (event.type === 'select-organization') return { ...clearDialogMessage(current), organizationId: event.organizationId, status: 'idle' }
  if (event.type === 'set-permission') return { ...clearDialogMessage(current), permission: event.permission, status: 'idle' }
  if (event.type === 'submit') {
    if (!event.canShare) return { ...current, status: 'forbidden', message: 'You do not have permission to share' }
    if (!current.organizationId) return { ...current, status: 'error', message: 'Select an organization to share with' }
    if (current.status === 'submitting') return current
    return { ...clearDialogMessage(current), status: 'submitting' }
  }
  if (event.type === 'success') return { open: false, organizationId: '', permission: 'viewer', status: 'success' }
  return { ...current, status: 'error', message: event.message }
}

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
