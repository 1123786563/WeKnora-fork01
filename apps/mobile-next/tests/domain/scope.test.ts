import { ScopeCoordinator, sameScope, scopeCacheKey, type ScopeKey } from "@/domain/scope";

const mk = (tenantId: string): ScopeKey => ({ origin: "https://weknora.example", userId: "u1", tenantId });

describe("RW-009 ScopeCoordinator（切空间隔离）", () => {
  it("enter 建立初始 generation", () => {
    const c = new ScopeCoordinator();
    const g = c.enter(mk("t1"));
    expect(g.value).toBe(1);
    expect(c.canWrite).toBe(true);
    expect(c.accepts(g)).toBe(true);
  });

  it("switchTenant 执行：禁写→generation+1→旧 generation 结果被拒绝", () => {
    const c = new ScopeCoordinator();
    const g1 = c.enter(mk("t1"));
    c.switchTenant("t2");
    expect(c.canWrite).toBe(false); // 禁写直到校验完成
    expect(c.accepts(g1)).toBe(false); // 迟到结果丢弃
    expect(c.scope?.tenantId).toBe("t2");
    c.reEnableWrites();
    expect(c.canWrite).toBe(true);
  });

  it("switchTenant abort 旧 signal", () => {
    const c = new ScopeCoordinator();
    c.enter(mk("t1"));
    const s1 = c.signal!;
    c.switchTenant("t2");
    expect(s1.aborted).toBe(true);
    expect(c.signal!.aborted).toBe(false);
  });

  it("reset 清空并 abort", () => {
    const c = new ScopeCoordinator();
    c.enter(mk("t1"));
    const s = c.signal!;
    c.reset();
    expect(s.aborted).toBe(true);
    expect(c.scope).toBeNull();
    expect(c.canWrite).toBe(false);
  });

  it("onGenerationChange 通知且可取消", () => {
    const c = new ScopeCoordinator();
    const seen: number[] = [];
    const off = c.onGenerationChange((g) => seen.push(g.value));
    c.enter(mk("t1"));
    c.switchTenant("t2");
    off();
    c.switchTenant("t3");
    expect(seen).toEqual([1, 2]);
  });
});

describe("RW-009 ScopeKey 序列化", () => {
  it("sameScope 逐字段比较", () => {
    expect(sameScope(mk("t1"), mk("t1"))).toBe(true);
    expect(sameScope(mk("t1"), mk("t2"))).toBe(false);
  });
  it("scopeCacheKey 分隔符转义，不碰撞", () => {
    const a = scopeCacheKey({ origin: "https://a", userId: "b\u0001c", tenantId: "d" });
    const b = scopeCacheKey({ origin: "https://a", userId: "b", tenantId: "c\u0001d" });
    expect(a).not.toBe(b);
  });
  it("缓存键按 origin/user/tenant 隔离", () => {
    const k1 = scopeCacheKey({ origin: "https://a", userId: "u1", tenantId: "t1" });
    const k2 = scopeCacheKey({ origin: "https://a", userId: "u1", tenantId: "t2" });
    const k3 = scopeCacheKey({ origin: "https://b", userId: "u1", tenantId: "t1" });
    expect(new Set([k1, k2, k3]).size).toBe(3);
  });
});
