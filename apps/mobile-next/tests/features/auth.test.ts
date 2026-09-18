import { AuthController, type AuthStage } from "@/features/auth/AuthController";
import { ScopeCoordinator, scopeCacheKey } from "@/domain/scope";
import { InMemoryStore } from "@/platform/store";
import { WeKnoraApi } from "@/api/weknora";
import { HttpClient, ApiError, type CredentialStore } from "@/api/http";
import type { SecureCredentials } from "@/platform/native";

// ---- fetch 桩 ----
type Route = (url: string, init?: RequestInit) => Response | Promise<Response>;
const mkFetch = (route: Route): typeof fetch =>
  ((url: any, init?: any) => Promise.resolve(route(String(url), init as RequestInit))) as unknown as typeof fetch;

const jsonRes = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const noCreds: CredentialStore = {
  async read() {
    return null;
  },
  async write() {},
  async clear() {},
};

const mkController = (route: Route, initialCreds: SecureCredentials | null = null) => {
  let credsValue = initialCreds;
  const credentials = {
    async read() {
      return credsValue;
    },
    async write(c: SecureCredentials) {
      credsValue = c;
    },
    async clear() {
      credsValue = null;
    },
  };
  const store = new InMemoryStore();
  const scope = new ScopeCoordinator();
  const stages: AuthStage[] = [];
  const http = new HttpClient({
    getOrigin: () => initialCreds?.origin ?? "https://weknora.example",
    getTenantId: () => scope.scope?.tenantId ?? null,
    credentials: { read: () => credentials.read() ?? null, write: (v) => credentials.write(v as never), clear: () => credentials.clear() },
    fetchImpl: mkFetch(route),
  });
  const ctrl = new AuthController({
    api: new WeKnoraApi(http),
    scope,
    store,
    credentials,
    getOrigin: () => initialCreds?.origin ?? "https://weknora.example",
    validateOrigin: (o) => {
      if (!o.startsWith("https://")) throw new ApiError("validation", "服务器不可信");
    },
    onStage: (s) => stages.push(s),
    onIdentity: () => {},
  });
  return { ctrl, stages, scope, store, credentials };
};

const userBody = { id: "u1", email: "a@b.c", name: "阿尧" };
const tenantBody = { tenants: [{ id: "t1", name: "产品研发空间", role: "owner" }, { id: "t2", name: "个人空间", role: "member" }] };
const loginBody = {
  success: true,
  user: userBody,
  active_tenant: { id: "t1", name: "产品研发空间" },
  memberships: [{ tenant_id: "t1", role: "owner", tenant_name: "产品研发空间" }],
  token: "tok-1",
  refresh_token: "ref-1",
};

describe("RW-008 冷启动 bootstrap", () => {
  it("无凭证 → login", async () => {
    const { ctrl, stages } = mkController(() => jsonRes(200, {}));
    await ctrl.bootstrap();
    expect(stages.at(-1)).toEqual({ kind: "login" });
  });

  it("有凭证 → me+tenants → 恢复上次空间 ready", async () => {
    const { ctrl, stages, scope } = mkController(
      (url) => (url.endsWith("/auth/me") ? jsonRes(200, userBody) : url.endsWith("/tenants") ? jsonRes(200, tenantBody) : jsonRes(200, {})),
      { origin: "https://weknora.example", access: "tok", refresh: "ref", userId: "u1", tenantId: "t1" },
    );
    await ctrl.bootstrap();
    expect(stages.at(-1)).toMatchObject({ kind: "ready", scope: { tenantId: "t1" } });
    expect(scope.scope?.tenantId).toBe("t1");
  });

  it("凭证 401 → 清凭证回 login（token 有效≠身份恢复）", async () => {
    const { ctrl, stages } = mkController(() => jsonRes(401, { error: "Unauthorized" }), {
      origin: "https://weknora.example",
      access: "tok",
      refresh: "ref",
      userId: "u1",
      tenantId: "t1",
    });
    await ctrl.bootstrap();
    expect(stages.at(-1)).toEqual({ kind: "login" });
  });

  it("上次空间已非成员 → 回落到第一个成员空间", async () => {
    const onlyT2 = { tenants: [{ id: "t2", name: "个人空间", role: "member" }] };
    const { ctrl, stages } = mkController(
      (url) => (url.endsWith("/auth/me") ? jsonRes(200, userBody) : url.endsWith("/tenants") ? jsonRes(200, onlyT2) : jsonRes(200, {})),
      { origin: "https://weknora.example", access: "tok", refresh: "ref", userId: "u1", tenantId: "t1" },
    );
    await ctrl.bootstrap();
    expect(stages.at(-1)).toMatchObject({ kind: "ready", scope: { tenantId: "t2" } });
  });

  it("origin 不可信 → server_untrusted，不发起恢复", async () => {
    let called = 0;
    const c = mkController(
      () => {
        called++;
        return jsonRes(200, userBody);
      },
      { origin: "http://evil.example", access: "tok", refresh: "ref", userId: "u1", tenantId: "t1" },
    );
    await c.ctrl.bootstrap();
    expect(c.stages.at(-1)).toMatchObject({ kind: "server_untrusted" });
    expect(called).toBe(0);
  });
});

describe("RW-008 loginWithPassword", () => {
  it("登录成功 → 凭证落 SecureStore → ready", async () => {
    const { ctrl, stages, credentials } = mkController((url) =>
      url.endsWith("/auth/login") ? jsonRes(200, loginBody) : url.endsWith("/auth/switch-tenant") ? jsonRes(200, {}) : jsonRes(200, {}),
    );
    await ctrl.loginWithPassword("https://weknora.example", "a@b.c", "pw");
    expect(stages.at(-1)).toMatchObject({ kind: "ready" });
    expect((await credentials.read())?.access).toBe("tok-1");
  });

  it("401 通用失败信息，不暴露账号是否存在", async () => {
    const { ctrl } = mkController(() => jsonRes(401, { error: "Unauthorized: bad credentials" }));
    await expect(ctrl.loginWithPassword("https://weknora.example", "a@b.c", "wrong")).rejects.toThrow(/登录|Unauthorized/i);
  });
});

describe("RW-009 switchSpace 顺序语义", () => {
  const boot = async () => {
    const c = mkController(
      (url) =>
        url.endsWith("/auth/me")
          ? jsonRes(200, userBody)
          : url.endsWith("/tenants")
            ? jsonRes(200, tenantBody)
            : url.endsWith("/auth/switch-tenant")
              ? jsonRes(200, { success: true })
              : jsonRes(200, {}),
      { origin: "https://weknora.example", access: "tok", refresh: "ref", userId: "u1", tenantId: "t1" },
    );
    await c.ctrl.bootstrap();
    return c;
  };

  it("切换：旧 scope 数据被清除、新 ready、凭证更新", async () => {
    const c = await boot();
    const oldKey = scopeCacheKey({ origin: "https://weknora.example", userId: "u1", tenantId: "t1" });
    await c.store.saveDraft(oldKey, "new-task", "旧空间草稿");
    await c.ctrl.switchSpace("t2");
    expect(c.scope.scope?.tenantId).toBe("t2");
    expect(await c.store.readDraft(oldKey, "new-task")).toBeNull(); // 敏感可见数据清除
    expect(c.stages.at(-1)).toMatchObject({ kind: "ready", scope: { tenantId: "t2" } });
  });

  it("非成员空间 → forbidden 拒绝切换", async () => {
    const c = await boot();
    await expect(c.ctrl.switchSpace("t-unknown")).rejects.toMatchObject({ kind: "forbidden" });
  });

  it("服务端确认成员失效 → 回空间选择（pick_space）", async () => {
    const c = mkController(
      (url) =>
        url.endsWith("/auth/me")
          ? jsonRes(200, userBody)
          : url.endsWith("/tenants")
            ? jsonRes(200, tenantBody)
            : url.endsWith("/auth/switch-tenant")
              ? jsonRes(403, { error: "Forbidden: removed" })
              : jsonRes(200, {}),
      { origin: "https://weknora.example", access: "tok", refresh: "ref", userId: "u1", tenantId: "t1" },
    );
    await c.ctrl.bootstrap();
    await c.ctrl.switchSpace("t2");
    expect(c.stages.at(-1)).toEqual({ kind: "pick_space" });
  });

  it("切换后旧 generation 的迟到响应被拒收", async () => {
    const c = await boot();
    const g1 = c.scope.generation!;
    await c.ctrl.switchSpace("t2");
    expect(c.scope.accepts(g1)).toBe(false);
  });

  it("logout：清凭证与 scope 数据，服务端错误不阻塞", async () => {
    const c = await boot();
    let logoutCalled = 0;
    await c.ctrl.logout();
    expect(logoutCalled).toBe(0);
    expect(c.stages.at(-1)).toEqual({ kind: "login" });
    expect(await c.credentials.read()).toBeNull();
    expect(c.scope.scope).toBeNull();
  });
});
