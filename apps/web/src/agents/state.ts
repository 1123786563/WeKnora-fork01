export interface Agent { id: string; name: string; description?: string; avatar?: string; is_builtin: boolean; created_by?: string; permission?: string; source_tenant_id?: string | number; config?: Record<string, unknown> }
export interface AgentPermissions { userId: string | null; roles: readonly string[] }
export type AgentSection = 'builtin' | 'mine' | 'tenant-others' | 'shared-editable' | 'shared-readonly';
export function isAdmin(permissions: AgentPermissions): boolean { return permissions.roles.includes('admin'); }
export function canManageAgent(agent: Agent, permissions: AgentPermissions): boolean { return isAdmin(permissions) || (!agent.is_builtin && !!agent.created_by && agent.created_by === permissions.userId); }
export function canEditAgent(agent: Agent): boolean { return !agent.is_builtin && (agent.permission === 'admin' || agent.permission === 'editor'); }
export function classifyAgent(agent: Agent, userId: string | null): AgentSection { if (agent.is_builtin) return 'builtin'; if (agent.permission) return canEditAgent(agent) ? 'shared-editable' : 'shared-readonly'; return agent.created_by === userId ? 'mine' : 'tenant-others'; }
export interface EditorState { mode: 'create' | 'edit'; section: AgentSection | string; saving: boolean; error: string | null }
export function initialEditorState(agent: Agent | null, section: string): EditorState { return { mode: agent ? 'edit' : 'create', section, saving: false, error: null }; }
