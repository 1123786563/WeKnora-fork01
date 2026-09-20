export function organizationRoleLabel(role: string): string { return ({ admin: 'Admin', editor: 'Editor', viewer: 'Viewer' } as Record<string, string>)[role] ?? role; }

export interface OrganizationSettingsNavGroup {
  key: 'basic' | 'management' | 'resources';
  /** i18n key of the group title (organization.navGroups.*). */
  titleKey: string;
  items: string[];
}

/**
 * Vue OrganizationSettingsModal only exposes join-request review to admins.
 * R487 K1: the standalone invite section is gone — the invite affordances live
 * inside the basic 邀请成员 card like the Vue modal (never a nav item).
 */
export function organizationSettingsSections(mode: 'create' | 'edit', canManage: boolean): string[] {
  return organizationSettingsNavGroups(mode, canManage).flatMap((group) => group.items);
}

/**
 * Vue navGroups (OrganizationSettingsModal.vue:1028-1058): the edit-mode
 * navigation renders THREE titled groups — 基础[基本信息] / 成员与协作[成员管理,
 * 加入申请] / 共享资源[共享知识库, 共享智能体] — while create mode keeps the single
 * 基础 group. Join requests stay admin-gated (Vue isAdmin).
 */
export function organizationSettingsNavGroups(mode: 'create' | 'edit', canManage: boolean): OrganizationSettingsNavGroup[] {
  if (mode === 'create') return [{ key: 'basic', titleKey: 'organization.navGroups.basic', items: ['basic', 'permissions'] }];
  return [
    { key: 'basic', titleKey: 'organization.navGroups.basic', items: ['basic'] },
    { key: 'management', titleKey: 'organization.navGroups.management', items: ['members', ...(canManage ? ['requests'] : [])] },
    { key: 'resources', titleKey: 'organization.navGroups.resources', items: ['shares', 'agents'] },
  ];
}
