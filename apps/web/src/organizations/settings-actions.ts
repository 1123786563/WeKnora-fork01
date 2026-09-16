// Organization settings helpers: invite-link generation + copy and
// shared-knowledge-base unshare, ported from the Vue baseline
// frontend/src/views/organization/OrganizationSettingsModal.vue and the
// ListOrgShares response (internal/handler/organization.go).

export interface SharedResourceRow {
  shareId: string;
  knowledgeBaseId: string;
  name: string;
  permission: string;
  canUnshare: boolean;
}

function textOf(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function sharedResourceRow(resource: Record<string, unknown>): SharedResourceRow {
  const shareId = textOf(resource.share_id) || textOf(resource.id);
  const knowledgeBaseId = textOf(resource.knowledge_base_id);
  const name = textOf(resource.knowledge_base_name) || textOf(resource.name) || textOf(resource.id);
  const permission = textOf(resource.permission) || textOf(resource.my_permission);
  return { shareId, knowledgeBaseId, name, permission, canUnshare: shareId !== '' && knowledgeBaseId !== '' };
}

export function buildInviteLink(inviteCode: string, location: { origin: string; pathname: string; search?: string }): string {
  const url = new URL(location.origin + location.pathname);
  const incoming = new URLSearchParams(location.search ?? '');
  for (const [key, value] of incoming.entries()) {
    if (key !== 'invite_code') url.searchParams.set(key, value);
  }
  url.searchParams.set('invite_code', inviteCode);
  return url.toString();
}

export async function copyText(value: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}
