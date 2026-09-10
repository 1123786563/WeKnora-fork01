export type ConfigurationSectionKey = 'agents' | 'models' | 'mcp' | 'skills';

export const configurationSections: Array<{ key: ConfigurationSectionKey; title: string; description: string; writeSupport: 'read-only' | 'planned' }> = [
  { key: 'agents', title: 'Agents', description: 'Custom and built-in agent configurations available in this workspace.', writeSupport: 'planned' },
  { key: 'models', title: 'Models', description: 'Model bindings returned by the workspace configuration service.', writeSupport: 'planned' },
  { key: 'mcp', title: 'MCP services', description: 'MCP endpoints and server-reported tool metadata.', writeSupport: 'planned' },
  { key: 'skills', title: 'Skills', description: 'Skill catalog availability associated with the workspace sandbox.', writeSupport: 'read-only' },
];

export function configurationStatus(value: { source?: unknown; enabled?: unknown; id?: unknown; name?: unknown }): string {
  if (value.source === 'env') return 'environment-managed';
  if (value.enabled === false) return 'disabled';
  if (value.enabled === true) return 'enabled';
  if (typeof value.id === 'string' || typeof value.name === 'string') return 'configured';
  return 'unknown';
}
