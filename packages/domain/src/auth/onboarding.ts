// Pure decisions for the workspace onboarding boundary, ported from
// frontend/src/views/auth/WorkspaceOnboarding.vue and CreateTenantDialog.vue.

export interface OnboardingPolicyInput {
  authenticated: boolean;
  hasTenant: boolean;
  canCreateTenant: boolean;
  pendingInvitationCount: number;
}

export type OnboardingView =
  | { kind: 'redirect' }
  | { kind: 'loading-policy' }
  | { kind: 'policy-error' }
  | { kind: 'ready'; canCreateTenant: boolean; inviteOnly: boolean; pendingInvitationCount: number };

/** Vue watch on hasValidTenant (WorkspaceOnboarding.vue:105-110): a valid
 *  tenant leaves the boundary immediately. */
export function onboardingView(policy: OnboardingPolicyInput | null): OnboardingView {
  if (policy === null) return { kind: 'loading-policy' };
  if (!policy.authenticated) return { kind: 'policy-error' };
  if (policy.hasTenant) return { kind: 'redirect' };
  return {
    kind: 'ready',
    canCreateTenant: policy.canCreateTenant,
    inviteOnly: !policy.canCreateTenant,
    pendingInvitationCount: policy.pendingInvitationCount,
  };
}

export interface CreateTenantFields { name: string; description: string }

/** Vue CreateTenantDialog formRules: name required (backend 1-128),
 *  description optional (backend max 512). Returns i18n message keys. */
export function validateCreateTenant(fields: CreateTenantFields): Record<string, string[]> {
  const errors: Record<string, string[]> = {};
  const name = fields.name.trim();
  if (!name) (errors.name ??= []).push('tenant.create.nameRequired');
  else if (name.length > 128) (errors.name ??= []).push('tenant.create.nameTooLong');
  if (fields.description.length > 512) (errors.description ??= []).push('tenant.create.descriptionTooLong');
  return errors;
}
