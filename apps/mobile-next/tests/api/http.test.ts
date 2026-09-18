import { ApiError, HttpClient, canonicalOrigin, assertTrustedHost, setAllowLocalHttpForDev, type CredentialStore } from "@/api/http";
import { WeKnoraApi } from "@/api/weknora";

// ---- fetch 测试桩 ----
type Handler = (url: string, init?: RequestInit) => Response | Promise<Response>;

const mkFetch = (handler: Handler): typeof fetch => {
  return ((url: any, init?: any) => Promise.resolve(handler(String(url), init as RequestInit))) as unknown as typeof fetch;
};

const jsonRes = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const memCreds = (): CredentialStore & { value: { access: string; refresh: string } | null } => {
  return {
    value: { access: "tok-a", refresh: "tok-r" },
    async read() {
      return this.value;
    },
    async write(v) {
      this.value = v;
    },
    async clear() {
      this.value = null;
    },
  };
};

const mkClient = (handler: Handler, tenant: string | null = "t1") => {
  const creds = memCreds();
  const http = new HttpClient({
    getOrigin: () => "https://weknora.example",
    getTenantId: () => tenant,
    credentials: creds,
    fetchImpl: mkFetch(handler),
  });
  return { http, creds, api: new WeKnoraApi(http) };
};

describe("RW-006 origin 校验", () => {
  it("规范化保留协议与端口，去路径", () => {
    setAllowLocalHttpForDev(true);
    try {
      expect(canonicalOrigin("http://h.example:8082/x/y?z=1")).toBe("http://h.example:8082");
      expect(canonicalOrigin(" https://a.example ")).toBe("https://a.example");
    } finally {
      setAllowLocalHttpForDev(false);
    }
  });
  it("拒绝非 http/https 与无效地址", () => {
    expect(() => canonicalOrigin("ftp://x")).toThrow(ApiError);
    expect(() => canonicalOrigin("not a url")).toThrow(ApiError);
  });
  it("非 dev 模式拒绝 http 协议与私有/环回 host（https 私有地址同样拒绝）", () => {
    setAllowLocalHttpForDev(false);
    expect(() => canonicalOrigin("http://weknora.example")).toThrow(/仅支持 https/);
    for (const h of [
      "https://localhost:8443",
      "https://127.0.0.1",
      "https://10.0.0.5",
      "https://192.168.1.4",
      "https://172.16.0.1",
    ]) {
      expect(() => assertTrustedHost(canonicalOrigin(h))).toThrow(/不可信/);
    }
  });
  it("dev 模式放行本地 http（显式开关）", () => {
    setAllowLocalHttpForDev(true);
    try {
      expect(() => assertTrustedHost(canonicalOrigin("http://localhost:8082"))).not.toThrow();
    } finally {
      setAllowLocalHttpForDev(false);
    }
  });
});

describe("RW-006 HttpClient 请求与鉴权", () => {
  it("注入 Bearer 与 X-Tenant-ID", async () => {
    let seen: Record<string, string> = {};
    const { api } = mkClient((url, init) => {
      seen = (init?.headers ?? {}) as Record<string, string>;
      return jsonRes(200, { id: "u1", email: "a@b.c", name: "A" });
    });
    await api.me();
    expect(seen.Authorization).toBe("Bearer tok-a");
    expect(seen["X-Tenant-ID"]).toBe("t1");
  });

  it("401 → 单飞刷新一次并重放（并发共享同一刷新）", async () => {
    let refreshCalls = 0;
    let meCalls = 0;
    const { api } = mkClient((url) => {
      meCalls++;
      if (url.endsWith("/auth/me") && meCalls === 1) return jsonRes(401, { error: "Unauthorized: token expired" });
      if (url.endsWith("/auth/refresh")) {
        refreshCalls++;
        return jsonRes(200, { access_token: "tok-a2", refresh_token: "tok-r2" });
      }
      return jsonRes(200, { id: "u1", name: "A" });
    });
    const [r1] = await Promise.all([api.me(), api.me().catch(() => null), api.me().catch(() => null)]);
    expect(refreshCalls).toBe(1);
    expect(r1.id).toBe("u1");
  });

  it("401 刷新失败 → unauthorized 且触发 onAuthExpired", async () => {
    let expired = false;
    const creds = memCreds();
    const http = new HttpClient({
      getOrigin: () => "https://weknora.example",
      getTenantId: () => null,
      credentials: creds,
      fetchImpl: mkFetch((url) => {
        if (url.endsWith("/auth/refresh")) return jsonRes(401, { error: "Unauthorized" });
        return jsonRes(401, { error: "Unauthorized: expired" });
      }),
      onAuthExpired: () => {
        expired = true;
      },
    });
    await expect(new WeKnoraApi(http).me()).rejects.toMatchObject({ kind: "unauthorized" });
    expect(expired).toBe(true);
  });

  it("403 不触发刷新，直接 forbidden", async () => {
    let refreshCalls = 0;
    const { api } = mkClient((url) => {
      if (url.endsWith("/auth/refresh")) {
        refreshCalls++;
        return jsonRes(200, {});
      }
      return jsonRes(403, { error: "Forbidden: not a member" });
    });
    await expect(api.me()).rejects.toMatchObject({ kind: "forbidden" });
    expect(refreshCalls).toBe(0);
  });

  it("错误 envelope 三形态都能分类", async () => {
    const mk = (body: unknown, status: number) => {
      const creds = memCreds();
      const http = new HttpClient({
        getOrigin: () => "https://weknora.example",
        getTenantId: () => null,
        credentials: creds,
        fetchImpl: mkFetch(() => jsonRes(status, body)),
      });
      return new WeKnoraApi(http);
    };
    await expect(mk({ success: false, error: { code: "VALIDATION", message: "参数错误" } }, 422).me()).rejects.toMatchObject({
      kind: "validation",
    });
    await expect(mk({ error: "Unauthorized: bad" }, 401).me()).rejects.toMatchObject({ kind: "unauthorized" });
    await expect(mk({ success: false, error: "server busy" }, 500).me()).rejects.toMatchObject({ kind: "server" });
    await expect(mk({ error: "cursor expired", code: "cursor_expired" }, 409).me()).rejects.toMatchObject({
      kind: "cursor_expired",
    });
  });

  it("网络失败 → network；abort → aborted", async () => {
    const creds = memCreds();
    const http = new HttpClient({
      getOrigin: () => "https://weknora.example",
      getTenantId: () => null,
      credentials: creds,
      fetchImpl: ((_url: any, init?: any) => {
        if (init?.signal?.aborted) return Promise.reject(Object.assign(new Error("Aborted"), { name: "AbortError" }));
        return Promise.reject(new Error("connection refused"));
      }) as unknown as typeof fetch,
    });
    const api = new WeKnoraApi(http);
    await expect(api.me()).rejects.toMatchObject({ kind: "network" });
    const ac = new AbortController();
    ac.abort();
    await expect(api.me({ signal: ac.signal })).rejects.toMatchObject({ kind: "aborted" });
  });

  it("startExecution 走 202 + data 包装解码", async () => {
    const { api } = mkClient(() => jsonRes(202, { data: { run_id: "r1", request_id: "rq1", status: "queued" } }));
    const r = await api.startExecution({
      request_id: "rq1",
      session_id: "s1",
      agent_id: "a1",
      target_id: "platform",
      workspace_ref: "w",
      text: "整理纪要",
      budget_upper: 100,
    });
    expect(r).toEqual({ run_id: "r1", request_id: "rq1", status: "queued" });
  });
});
