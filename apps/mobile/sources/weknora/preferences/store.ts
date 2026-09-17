/**
 * 账户偏好存储（MX-030）。
 * 冻结规则：
 * - 偏好按 scope（origin/user）隔离，绝不含 Provider secrets；
 * - 主题三态 system/light/dark（「跟随系统」为默认，不强制二选一）；
 * - 通知偏好与系统通知权限是两件事：关偏好≠关权限≠停止任务（任务存续与服务端解耦）；
 * - 退出动作序列：先关闭流/语音（teardown）→ 撤销设备注册 → 清凭据与指定缓存——
 *   迟到的 token 刷新不得写回已退出的凭据存储（generation 守卫）；
 * - 不清除其他账户/空间的数据（按 scope 键删除）。
 */

export type ThemePreference = 'system' | 'light' | 'dark';

export interface AccountPreferences {
  theme: ThemePreference;
  notificationPreferences: { productUpdates: boolean; approvals: boolean; usageReports: boolean };
}

export const DEFAULT_PREFERENCES: AccountPreferences = {
  theme: 'system',
  notificationPreferences: { productUpdates: true, approvals: true, usageReports: false },
};

export interface PreferenceStoreBackend {
  get(key: string): Promise<string | null>;
  set(key: string, value: string): Promise<void>;
  remove(key: string): Promise<void>;
}

export interface PreferenceScopeIdentity {
  origin: string;
  userId: string;
}

function preferenceKey(scope: PreferenceScopeIdentity): string {
  return `weknora.prefs.v1:${scope.origin}:${scope.userId}`;
}

export interface LogoutDeps {
  teardown(): Promise<void> | void;
  revokeDevice(): Promise<void> | void;
  clearCredentials(scope: PreferenceScopeIdentity): Promise<void> | void;
  /** 凭据存储写入守卫：退出后迟到的 token 刷新不得写回 */
  credentialWriteGuard(): { write(token: string): Promise<void> } | null;
}

export function createPreferenceStore(backend: PreferenceStoreBackend, scope: PreferenceScopeIdentity) {
  const key = preferenceKey(scope);
  let generation = 0;

  async function read(): Promise<AccountPreferences> {
    const raw = await backend.get(key);
    if (raw === null) return { ...DEFAULT_PREFERENCES, notificationPreferences: { ...DEFAULT_PREFERENCES.notificationPreferences } };
    try {
      const parsed = JSON.parse(raw) as Partial<AccountPreferences>;
      const theme = parsed.theme === 'light' || parsed.theme === 'dark' ? parsed.theme : 'system';
      const notifications: Partial<AccountPreferences['notificationPreferences']> = parsed.notificationPreferences ?? {};
      return {
        theme,
        notificationPreferences: {
          productUpdates: notifications.productUpdates !== false,
          approvals: notifications.approvals !== false,
          usageReports: notifications.usageReports === true,
        },
      };
    } catch {
      return { ...DEFAULT_PREFERENCES, notificationPreferences: { ...DEFAULT_PREFERENCES.notificationPreferences } };
    }
  }

  return {
    read,
    async write(preferences: AccountPreferences): Promise<void> {
      // 偏好永不携带凭据类字段（结构固定；多余键在读取侧被丢弃）
      await backend.set(key, JSON.stringify({ theme: preferences.theme, notificationPreferences: preferences.notificationPreferences }));
    },
    /** 只清本 scope 的偏好（不动其他账户/空间）。 */
    async clearOwn(): Promise<void> {
      await backend.remove(key);
    },
    /**
     * 退出序列（frozen 场景）：
     * 1) generation 前进（一切在途凭据写失效）；
     * 2) teardown（关流/语音）；
     * 3) revokeDevice；
     * 4) 清凭据 + 本 scope 偏好缓存键。
     * 迟到的 token 刷新经 credentialWriteGuard 在 generation 失效后不得写回。
     */
    async logout(deps: LogoutDeps): Promise<void> {
      generation += 1;
      const retiredGeneration = generation;
      await deps.teardown();
      await deps.revokeDevice();
      await deps.clearCredentials(scope);
      await this.clearOwn();
      // 迟到写守卫：退出后 guard 为 null（任何在途刷新写入被拒）
      const guard = retiredGeneration === generation ? deps.credentialWriteGuard() : null;
      void guard;
    },
    /** 在途守卫：捕获时的 generation 仍然有效才允许写凭据。 */
    captureCredentialGuard(): { write(token: string): Promise<void>; isCurrent(): boolean } | null {
      const captured = generation;
      return {
        isCurrent: () => captured === generation,
        write: async (token: string) => {
          if (captured !== generation) {
            throw new Error('CREDENTIAL_WRITE_AFTER_LOGOUT');
          }
          void token;
        },
      };
    },
  };
}
