// Join-flow semantics ported from the Vue baseline
// frontend/src/views/organization/OrganizationList.vue (invite preview,
// lines ~405-465): an approval-gated organization branches to a request
// flow with a role select and an optional application note, while an open
// organization joins directly. Already-members never see a join action.

export type InviteJoinMode = 'member' | 'request' | 'join';

export interface InvitePreviewRow {
  require_approval?: unknown;
  is_already_member?: unknown;
  [key: string]: unknown;
}

export function inviteJoinMode(preview: InvitePreviewRow | null | undefined): InviteJoinMode {
  if (!preview || preview.is_already_member === true) return 'member';
  // A preview without any membership/approval flags is not actionable.
  if (preview.is_already_member === undefined && preview.require_approval === undefined) return 'member';
  return preview.require_approval === true ? 'request' : 'join';
}

export function requestedRoleOf(preview: { requested_role?: unknown; [key: string]: unknown }): 'admin' | 'editor' | 'viewer' {
  return preview.requested_role === 'admin' || preview.requested_role === 'editor' ? preview.requested_role : 'viewer';
}

// Vue caps the application note at 500 characters (:maxlength="500").
export function clampApplicationNote(value: string): string {
  return value.trim().slice(0, 500);
}
