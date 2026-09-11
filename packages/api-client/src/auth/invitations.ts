import type { ClientRequest } from '../client.ts';
import type { AuthRequest } from './oidc.ts';

export type TenantInvitationStatus = 'pending' | 'accepted' | 'declined' | 'revoked' | 'expired';
export interface TenantInvitation {
  id: number; tenant_id: number; tenant_name?: string; invitee_user_id: string;
  invitee_email?: string; invitee_name?: string; invited_by?: string | null;
  inviter_email?: string; inviter_name?: string; role: string; status: TenantInvitationStatus;
  message?: string; expires_at: string; responded_at?: string | null; created_at: string;
  invite_url?: string; is_share_link?: boolean; accepted_count?: number;
}
export interface InvitationListResponse { success: boolean; data?: { invitations: TenantInvitation[]; total: number }; message?: string }
export interface InvitationActionResponse { success: boolean; data?: { membership?: { tenant_id: number; role: string; status: string; joined_at: string }; tenant_name?: string }; message?: string }
export interface InvitationLookupResponse { success: boolean; data?: { tenant_id: number; tenant_name?: string; role: string; expires_at: string }; message?: string }
export interface RegisterByInviteRequest { token: string; email: string; username: string; password: string }

export function createInvitationsApi(request: AuthRequest) {
  return {
    async listMine(includeTerminal = false): Promise<InvitationListResponse> {
      const path = includeTerminal ? '/api/v1/me/invitations?include_terminal=true' : '/api/v1/me/invitations';
      return await request({ method: 'GET', path }) as InvitationListResponse;
    },
    async accept(id: number): Promise<InvitationActionResponse> {
      return await request({ method: 'POST', path: `/api/v1/me/invitations/${encodeURIComponent(String(id))}/accept` }) as InvitationActionResponse;
    },
    async decline(id: number): Promise<{ success: boolean; message?: string }> {
      return await request({ method: 'POST', path: `/api/v1/me/invitations/${encodeURIComponent(String(id))}/decline` }) as { success: boolean; message?: string };
    },
    async acceptByToken(token: string): Promise<InvitationActionResponse> {
      return await request({ method: 'POST', path: '/api/v1/me/invitations/accept-by-token', body: { token } }) as InvitationActionResponse;
    },
    async lookup(token: string): Promise<InvitationLookupResponse> {
      return await request({ method: 'POST', path: '/api/v1/auth/invitations/lookup', body: { token } }) as InvitationLookupResponse;
    },
    async registerByInvite(input: RegisterByInviteRequest): Promise<Record<string, unknown>> {
      return await request({ method: 'POST', path: '/api/v1/auth/register-by-invite', body: input }) as Record<string, unknown>;
    },
  };
}
