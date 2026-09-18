/** Proposed ADDITIVE read models only. These do not replace current platform DTOs.
 * Confirm actual API envelope and identifier types against the implementation baseline.
 */
export type RunStatus = 'queued' | 'running' | 'waiting_user' | 'reconciling' | 'succeeded' | 'failed' | 'canceled';
export type Availability =
  | { state: 'available' }
  | { state: 'forbidden' | 'unavailable'; reason_code: string; message: string };
export interface TaskCardDTO {
  run_id: string;
  session_id: string;
  title: string;
  agent: { id: string; name: string; icon_key?: string };
  run_status: RunStatus;
  created_at: string;
  updated_at: string;
  pending_action_count: number;
}
export interface TaskPage { items: TaskCardDTO[]; next_cursor: string | null; as_of: string; }
export interface TaskListQuery { status?: RunStatus; cursor?: string; limit?: number; }
export interface InboxItem {
  interaction_id: string;
  run_id: string;
  title: string;
  action_type: string;  // display classification; NOT a free-form decision payload
  target_summary: string;
  current_revision: string; // additive display token; adapt actual revision type before submit
  state: 'pending' | 'handled' | 'expired';
  created_at: string;
  expires_at?: string;
}
export interface InboxPage { items: InboxItem[]; next_cursor: string | null; as_of: string; }
export interface MiniCapabilities {
  workbench: Availability;
  knowledge: Availability;
  wechat_login: Availability;
  purchase: Availability;
  notifications: Availability;
}
export interface BudgetDisplayPolicy {
  unit_code: string;
  unit_label: string;
  min_display: string;
  max_display: string;
  step_display: string;
  backend_unit_code: string;
  // Exact rational conversion, never inferred from marketing copy.
  to_backend_numerator: string;
  to_backend_denominator: string;
}
export interface LaunchOptions {
  agent_id: string;
  options_version: string;
  as_of: string;
  targets: Array<{ target_id: string; workspace_ref: string; label: string; availability: Availability }>;
  budget: BudgetDisplayPolicy;
}
export interface ScopedRecord<T> {
  schemaVersion: 1;
  scope: { origin: string; userId: string; tenantId: string };
  savedAt: string;
  expiresAt: string;
  payload: T;
}
export interface PendingIntent {
  request_id: string;
  session_id: string;
  agent_id: string;
  created_at: string;
  phase: 'prepared' | 'submitting' | 'uncertain' | 'admitted' | 'rejected';
}
