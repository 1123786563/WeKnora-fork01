// Port of Vue Login.vue share-link/invite flow (Login.vue:425-434, 640-643,
// 653-670, 763-810): token resolution, sessionStorage persistence across the
// OIDC redirect, and landing-mode decision.
export const PENDING_INVITE_KEY = 'weknora_pending_invite_token';

export interface InviteStorage {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

export function storePendingInviteToken(storage: InviteStorage, token: string): void {
  if (token) storage.setItem(PENDING_INVITE_KEY, token);
}

export function readPendingInviteToken(storage: InviteStorage): string {
  return storage.getItem(PENDING_INVITE_KEY)?.trim() ?? '';
}

export function clearPendingInviteToken(storage: InviteStorage): void {
  storage.removeItem(PENDING_INVITE_KEY);
}

/** Vue Login.vue:803-808 — invite_only stays on the login card so the user
 *  signs in first and the token is redeemed afterwards; open deployments go
 *  straight into register-by-invite. */
export function landingModeForInvite(registrationMode: string): 'login' | 'register' {
  return registrationMode === 'invite_only' ? 'login' : 'register';
}
