// 认证与空间控制器（RW-008/009）。
// 冷启动顺序（详细设计 §3.1）：读凭证 → /auth/me → memberships → 空间恢复或 M02 → generation → 数据。
// 有 Token ≠ 身份恢复完成；切空间按禁写→generation→abort→清数据→校验→加载执行。
import { ApiError } from "@/api/http";
import type { WeKnoraApi } from "@/api/weknora";
import { ScopeCoordinator, scopeCacheKey, toCloudWorkspaceScope, type ScopeKey } from "@/domain/scope";
import type { CloudWorkspaceScope } from "@/cloud-workspace/CloudWorkspaceClient";
import type { MobileStore } from "@/platform/store";
import { secureCreds, type SecureCredentials } from "@/platform/native";

export type AuthStage =
  | { kind: "booting" }
  | { kind: "login" }
  | { kind: "restoring" }
  | { kind: "pick_space" }
  | { kind: "ready"; scope: ScopeKey }
  | { kind: "login_failed"; reason: string }
  | { kind: "server_untrusted"; reason: string };

export interface AppIdentity {
  userId: string;
  displayName: string;
  email: string;
  memberships: Array<{ tenantId: string; tenantName: string; role: string; billingRole: string | null }>;
  selectedTenantId: string | null;
}

export interface AuthDeps {
  api: WeKnoraApi;
  scope: ScopeCoordinator;
  store: MobileStore;
  credentials: typeof secureCreds;
  getOrigin(): string;
  /** 校验 origin 可信（http client 层同规则） */
  validateOrigin(origin: string): void;
  onStage(stage: AuthStage): void;
  onIdentity(identity: AppIdentity | null): void;
  onTrustedScope(scope: CloudWorkspaceScope | null): void;
}

export class AuthController {
  private identity: AppIdentity | null = null;
  private previousScopeKey: string | null = null;
  private localExpiry: Promise<void> | null = null;

  constructor(private d: AuthDeps) {}

  get currentIdentity(): AppIdentity | null {
    return this.identity;
  }

  /** Refresh exhaustion must invalidate every local auth-dependent capability before login is visible. */
  expireLocalAuth(): Promise<void> {
    if (this.localExpiry) return this.localExpiry;

    const expiry = this.clearLocalAuth(true);
    this.localExpiry = expiry;
    void expiry.then(
      () => {
        if (this.localExpiry === expiry) this.localExpiry = null;
      },
      () => {
        if (this.localExpiry === expiry) this.localExpiry = null;
      },
    );
    return expiry;
  }

  private async clearLocalAuth(publishTrustedScope: boolean): Promise<void> {
    if (publishTrustedScope) this.d.onTrustedScope(null);
    if (this.previousScopeKey) {
      await this.d.store.clearScopeData(this.previousScopeKey);
    }
    await this.d.credentials.clear();
    this.identity = null;
    this.d.onIdentity(null);
    this.previousScopeKey = null;
    this.d.scope.reset();
    this.d.onStage({ kind: "login" });
  }

  /** 冷启动：SecureStore → me → memberships → 恢复空间或选空间 */
  async bootstrap(): Promise<void> {
    this.d.onStage({ kind: "booting" });
    let creds: SecureCredentials | null = null;
    try {
      creds = await this.d.credentials.read();
    } catch {
      creds = null;
    }
    if (!creds?.access) {
      this.d.onStage({ kind: "login" });
      return;
    }
    this.d.onStage({ kind: "restoring" });
    try {
      await this.d.validateOrigin(creds.origin);
      const user = await this.d.api.me({ signal: this.d.scope.signal ?? undefined });
      const tenants = await this.d.api.tenants({ signal: this.d.scope.signal ?? undefined });
      const memberships = tenants.map((t) => ({
        tenantId: t.id,
        tenantName: t.name,
        role: t.role,
        billingRole: t.billing_role,
      }));
      this.identity = {
        userId: user.id,
        displayName: user.display_name,
        email: user.email,
        memberships,
        selectedTenantId: null,
      };
      if (!memberships.length) {
        // Token 有效但无任何空间 → 空间选择（创建/申请入口在 M02）
        this.d.onStage({ kind: "pick_space" });
        this.d.onIdentity(this.identity);
        return;
      }
      const preferred = creds.tenantId && memberships.some((m) => m.tenantId === creds!.tenantId) ? creds.tenantId : memberships[0]!.tenantId;
      // 成员被移除：preferred 无效时回落第一个，不保留旧空间缓存
      await this.enterSpace(preferred, creds);
    } catch (e) {
      if (e instanceof ApiError && e.kind === "unauthorized") {
        await this.d.credentials.clear();
        this.d.onStage({ kind: "login" });
        return;
      }
      if (e instanceof ApiError && e.kind === "validation") {
        this.d.onStage({ kind: "server_untrusted", reason: e.message });
        return;
      }
      // 网络失败：保留凭证，可重试登录页
      this.d.onStage({ kind: "login" });
    }
  }

  /** 邮箱密码登录 */
  async loginWithPassword(origin: string, email: string, password: string): Promise<void> {
    this.d.validateOrigin(origin);
    const res = await this.d.api.login(email.trim(), password);
    await this.d.credentials.write({
      origin,
      access: res.token,
      refresh: res.refresh_token,
      userId: res.user.id,
      tenantId: res.active_tenant?.id ?? null,
    });
    const memberships = res.memberships.map((m) => ({
      tenantId: m.tenant_id,
      tenantName: m.tenant_name ?? m.tenant_id,
      role: m.role,
      billingRole: m.billing_role,
    }));
    this.identity = {
      userId: res.user.id,
      displayName: res.user.display_name,
      email: res.user.email,
      memberships,
      selectedTenantId: null,
    };
    this.d.onIdentity(this.identity);
    if (!memberships.length) {
      this.d.onStage({ kind: "pick_space" });
      return;
    }
    const preferred =
      res.active_tenant?.id && memberships.some((m) => m.tenantId === res.active_tenant!.id)
        ? res.active_tenant.id
        : memberships[0]!.tenantId;
    await this.enterSpace(preferred, null);
  }

  /** 进入空间（登录后或冷启动恢复）：enter scope → 恢复凭证 tenantId → ready */
  private async enterSpace(tenantId: string, creds: SecureCredentials | null): Promise<void> {
    const origin = creds?.origin ?? this.d.getOrigin();
    const userId = creds?.userId ?? this.identity?.userId ?? "";
    const scope: ScopeKey = { origin, userId, tenantId };
    const g = this.d.scope.enter(scope);
    this.previousScopeKey = scopeCacheKey(scope);
    if (this.identity) {
      this.identity = { ...this.identity, selectedTenantId: tenantId };
      this.d.onIdentity(this.identity);
    }
    this.d.onTrustedScope(toCloudWorkspaceScope(g.scope, g.value));
    this.d.onStage({ kind: "ready", scope: g.scope });
  }

  /** 切换空间（RW-009 顺序语义） */
  async switchSpace(tenantId: string): Promise<void> {
    const current = this.d.scope.scope;
    if (!current || !this.identity) throw new Error("尚未登录");
    if (!this.identity.memberships.some((m) => m.tenantId === tenantId)) {
      throw new ApiError("forbidden", "你不是该空间成员");
    }
    const previousScope = current;
    const previousScopeKey = this.previousScopeKey ?? scopeCacheKey(previousScope);
    // 1) 禁写 2) generation+1 3) abort 旧订阅 —— ScopeCoordinator.switchTenant 内执行
    const g = this.d.scope.switchTenant(tenantId);
    // Requested tenants are never a trusted scope before server confirmation.
    this.d.onTrustedScope(null);
    // 4) 清旧空间敏感可见数据（按旧 scope 缓存键）
    await this.d.store.clearScopeData(previousScopeKey);
    const newKey = scopeCacheKey(g.scope);
    this.previousScopeKey = newKey;
    // 5) 校验新空间：服务端确认成员关系仍有效
    try {
      await this.d.api.switchTenant(tenantId, { signal: this.d.scope.signal ?? undefined });
    } catch (e) {
      if (e instanceof ApiError && e.kind === "forbidden") {
        // 成员关系失效 → 回空间选择
        await this.refreshMemberships();
        this.d.onStage({ kind: "pick_space" });
        return;
      }
      const restored = this.d.scope.enter(previousScope);
      this.previousScopeKey = previousScopeKey;
      this.d.onTrustedScope(toCloudWorkspaceScope(restored.scope, restored.value));
      throw e;
    }
    // 6) 恢复写入并更新持久凭证
    this.d.scope.reEnableWrites();
    const creds = await this.d.credentials.read();
    if (creds) await this.d.credentials.write({ ...creds, tenantId });
    this.identity = { ...this.identity, selectedTenantId: tenantId };
    this.d.onIdentity(this.identity);
    this.d.onTrustedScope(toCloudWorkspaceScope(g.scope, g.value));
    this.d.onStage({ kind: "ready", scope: g.scope });
  }

  async refreshMemberships(): Promise<void> {
    const tenants = await this.d.api.tenants();
    if (this.identity) {
      this.identity = {
        ...this.identity,
        memberships: tenants.map((t) => ({
          tenantId: t.id,
          tenantName: t.name,
          role: t.role,
          billingRole: t.billing_role,
        })),
      };
      this.d.onIdentity(this.identity);
    }
  }

  /** 退出：清凭证、清可见数据、撤销本地绑定；服务端任务不取消 */
  async logout(): Promise<void> {
    this.d.onTrustedScope(null);
    try {
      await this.d.api.logout();
    } catch {
      // 服务端登出失败不阻塞本地清理；token 24h 过期兜底
    }
    await this.clearLocalAuth(false);
  }
}
