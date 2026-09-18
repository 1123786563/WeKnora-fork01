// 一次性真实后端联调（RW-008 证据）：node --experimental-strip-types 直接运行，不入 jest。
// 随机账号仅本地验证用；不写入任何仓库文件。
import { AuthController, type AuthStage } from "../../src/features/auth/AuthController.ts";
import { ScopeCoordinator } from "../../src/domain/scope.ts";
import { InMemoryStore } from "../../src/platform/store.ts";
import { WeKnoraApi } from "../../src/api/weknora.ts";
import { HttpClient } from "../../src/api/http.ts";

const ORIGIN = process.env.WEKNORA_ORIGIN ?? "http://localhost:8082";
const email = `mn_${Date.now().toString(36)}@test.local`;
const password = `Mn-${Math.random().toString(36).slice(2, 10)}!`;

const scope = new ScopeCoordinator();
const store = new InMemoryStore();
let credValue: any = null;
const credentials = {
  async read() {
    return credValue;
  },
  async write(c: any) {
    credValue = c;
  },
  async clear() {
    credValue = null;
  },
};
const http = new HttpClient({
  getOrigin: () => ORIGIN,
  getTenantId: () => scope.scope?.tenantId ?? null,
  credentials,
});
const api = new WeKnoraApi(http);
const stages: AuthStage[] = [];
const identities: any[] = [];
const ctrl = new AuthController({
  api,
  scope,
  store,
  credentials,
  getOrigin: () => ORIGIN,
  validateOrigin: () => {},
  onStage: (s) => stages.push(s),
  onIdentity: (i) => identities.push(i),
});

// dev 环境允许本地 http
const { setAllowLocalHttpForDev } = await import("../../src/api/http.ts");
setAllowLocalHttpForDev(true);

const assert = (cond: unknown, msg: string) => {
  if (!cond) {
    console.error(`✗ ${msg}`);
    process.exit(1);
  }
  console.log(`✓ ${msg}`);
};

// 1) 注册（self_serve 开放；真实字段 username/email/password）
const reg = await fetch(`${ORIGIN}/api/v1/auth/register`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email, password, username: email.split("@")[0]!, name: "联调验证" }),
});
assert(reg.status === 200 || reg.status === 201, `注册测试账号（HTTP ${reg.status}）`);

// 2) 新客户端登录链路（wire 解码 + 凭证写入 + memberships）
await ctrl.loginWithPassword(ORIGIN, email, password);
assert(credValue?.access?.length > 20, "loginWithPassword 拿到 token 并写入凭证");
const id1 = identities.at(-1);
assert(Array.isArray(id1?.memberships), `memberships 解码成功（${id1?.memberships?.length ?? 0} 个空间）`);
assert(stages.at(-1)?.kind === "ready" || stages.at(-1)?.kind === "pick_space", `终态 stage=${stages.at(-1)?.kind}`);

// 3) 携带真实 token 调 /auth/me 与 /tenants（wire 解码对真实 JSON）
const me = await api.me();
assert(me.email === email, `me() 解码真实用户（${me.email}）`);
const tenants = await api.tenants();
assert(Array.isArray(tenants), `tenants() 解码真实空间列表（${tenants.length} 个）`);

// 4) 会话列表 + Agent 列表（业务端点 wire 解码）
const sessions = await api.sessions({ page: 1, page_size: 5 });
assert(Array.isArray(sessions.sessions), `sessions() 解码（total=${sessions.total}）`);
const agents = await api.agents();
assert(Array.isArray(agents), `agents() 解码（${agents.length} 个 Agent）`);

// 5) 401 错误分类（错 token）
const badHttp = new HttpClient({ getOrigin: () => ORIGIN, getTenantId: () => null, credentials: { read: async () => ({ access: "bad", refresh: "bad" }), write: async () => {}, clear: async () => {} } });
try {
  await new WeKnoraApi(badHttp).me();
  assert(false, "坏 token 应当失败");
} catch (e: any) {
  assert(e.kind === "unauthorized", `坏 token → ${e.kind}（401 分类正确）`);
}

// 6) 未登录 bootstrap → login
const stages2: AuthStage[] = [];
const ctrl2 = new AuthController({
  api,
  scope: new ScopeCoordinator(),
  store: new InMemoryStore(),
  credentials: { read: async () => null, write: async () => {}, clear: async () => {} },
  getOrigin: () => ORIGIN,
  validateOrigin: () => {},
  onStage: (s) => stages2.push(s),
  onIdentity: () => {},
});
await ctrl2.bootstrap();
assert(stages2.at(-1)?.kind === "login", "无凭证 bootstrap → login");

console.log("\n=== 真实后端联调全部通过 ===");
