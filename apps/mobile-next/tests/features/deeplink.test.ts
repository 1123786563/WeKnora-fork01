import { parseDeepLink, handleDeepLink, type DeepLinkContext } from "@/features/notifications/deeplink/DeepLinkController";

const readyCtx = (over: Partial<DeepLinkContext> = {}): DeepLinkContext => ({
  stage: "ready",
  currentTenantId: "t1",
  resourceTenantId: null,
  ...over,
});

describe("RW-028 parseDeepLink 白名单与安全剥离", () => {
  it("weknora:// 资源路径解析为对象式路由（pathname + routeParams）", () => {
    const r = parseDeepLink("weknora://executions/run_9");
    expect(r).toMatchObject({ kind: "ok", pathname: "/executions/[runId]", routeParams: { runId: "run_9" } });
    const s = parseDeepLink("weknora:///sessions/s_1");
    expect(s).toMatchObject({ kind: "ok", pathname: "/sessions/[sessionId]", routeParams: { sessionId: "s_1" } });
    const i = parseDeepLink("weknora://inbox");
    expect(i).toMatchObject({ kind: "ok", pathname: "/inbox", routeParams: {} });
  });

  it("安全 query 参数透传（runId 等）", () => {
    const r = parseDeepLink("weknora://interactions/it_1?runId=run_9");
    expect(r).toMatchObject({
      kind: "ok",
      pathname: "/interactions/[interactionId]",
      routeParams: { resourceId: "it_1", runId: "run_9" },
    });
  });

  it("批准/凭证类参数一律剥离并记录（URI 不得携带批准动作）", () => {
    const r = parseDeepLink("weknora://interactions/it_1?approve=1&action=approve&token=secret&runId=r1");
    expect(r).toMatchObject({ kind: "ok" });
    if (r.kind === "ok") {
      expect(r.strippedParams).toEqual(expect.arrayContaining(["approve", "action", "token"]));
      expect(r.routeParams).toEqual({ resourceId: "it_1", runId: "r1" }); // 危险参数不进入路由
      expect(Object.values(r.routeParams).join(" ")).not.toContain("secret");
    }
  });

  it("非受信 scheme 与未知路径拒绝", () => {
    expect(parseDeepLink("https://evil.example/executions/run_9")).toMatchObject({ kind: "rejected", reason: "unsupported_scheme" });
    expect(parseDeepLink("weknora://admin/shell")).toMatchObject({ kind: "rejected", reason: "unknown_path" });
    expect(parseDeepLink("")).toMatchObject({ kind: "rejected", reason: "empty" });
  });

  it("资源 id 只允许安全字符集（路径穿越拒绝）", () => {
    expect(parseDeepLink("weknora://executions/run%20../../etc")).toMatchObject({ kind: "rejected", reason: "unknown_path" });
    expect(parseDeepLink("weknora://executions/run_9/../../etc")).toMatchObject({ kind: "rejected", reason: "unknown_path" });
  });
});

describe("RW-028 handleDeepLink 认证与跨空间规则", () => {
  it("已认证 + 同空间资源 → navigate（对象式路由）", () => {
    const d = handleDeepLink("weknora://executions/run_9", readyCtx());
    expect(d).toMatchObject({ kind: "navigate", pathname: "/executions/[runId]" });
  });

  it("未认证 → defer_to_login（登录后落位，不跳过授权）", () => {
    expect(handleDeepLink("weknora://executions/run_9", readyCtx({ stage: "login" }))).toMatchObject({ kind: "defer_to_login" });
    expect(handleDeepLink("weknora://executions/run_9", readyCtx({ stage: "booting" }))).toMatchObject({ kind: "defer_to_login" });
  });

  it("跨空间资源 → pick_space_first（显式确认，不静默切换）", () => {
    expect(handleDeepLink("weknora://sessions/s_other", readyCtx({ resourceTenantId: "t2" }))).toMatchObject({ kind: "pick_space_first" });
  });

  it("资源空间未知 → 正常导航（页面内重新授权）", () => {
    expect(handleDeepLink("weknora://sessions/s_x", readyCtx({ resourceTenantId: null }))).toMatchObject({ kind: "navigate" });
  });

  it("解析失败 → dropped；危险参数剥离后导航仍可", () => {
    expect(handleDeepLink("weknora://nope/nada", readyCtx())).toEqual({ kind: "dropped" });
    const d = handleDeepLink("weknora://interactions/i1?action=approve", readyCtx());
    expect(d.kind).toBe("navigate");
    if (d.kind === "navigate") {
      expect(d.stripped).toContain("action");
      expect(d.routeParams).toEqual({ resourceId: "i1" });
    }
  });
});
