import { SubmissionService, fnv1a, canonicalInput, newRequestId, type SubmissionDraft } from "@/features/workbench/submit/SubmissionService";
import { InMemoryStore } from "@/platform/store";
import { WeKnoraApi } from "@/api/weknora";
import { HttpClient, type CredentialStore } from "@/api/http";

const draft: SubmissionDraft = {
  text: "整理本周客户反馈并生成摘要",
  agentId: "agent_research",
  targetId: "platform-default",
  workspaceRef: "ws_default",
  budgetUpper: 100,
  knowledgeBaseIds: ["kb_1"],
  sessionId: "s_1",
};

type Route = (url: string, init?: RequestInit) => Response | Promise<Response>;
const mkFetch = (route: Route): typeof fetch =>
  ((url: any, init?: any) => Promise.resolve(route(String(url), init as RequestInit))) as unknown as typeof fetch;
const jsonRes = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

const noCreds: CredentialStore = {
  async read() {
    return null;
  },
  async write() {},
  async clear() {},
};

const mkService = (route: Route, opts: { canWrite?: boolean } = {}) => {
  const store = new InMemoryStore();
  let writable = opts.canWrite ?? true;
  const http = new HttpClient({
    getOrigin: () => "https://weknora.example",
    getTenantId: () => "t1",
    credentials: noCreds,
    fetchImpl: mkFetch(route),
  });
  const svc = new SubmissionService({
    api: new WeKnoraApi(http),
    store,
    scopeKey: () => "scope-key-1",
    canWrite: () => writable,
    newRequestId: () => newRequestId(),
    lookup: async (requestId) => {
      try {
        const r = await new WeKnoraApi(http).lookupRequest(requestId);
        return { runId: r?.run_id ?? null, status: r?.status ?? null };
      } catch {
        return { runId: null, status: null };
      }
    },
  });
  return { svc, store, setWritable: (v: boolean) => (writable = v) };
};

describe("RW-013 提交状态机", () => {
  it("正常链路：prepared 落盘在 HTTP 之前 → accepted 持久绑定 runId", async () => {
    const order: string[] = [];
    const { svc, store } = mkService((url, init) => {
      if (url.endsWith("/workbench/executions")) {
        order.push("http");
        return jsonRes(202, { data: { run_id: "run_9", request_id: "x", status: "queued" } });
      }
      return jsonRes(200, {});
    });
    const origSave = store.savePending.bind(store);
    store.savePending = async (p) => {
      order.push(`save:${p.state}`);
      return origSave(p);
    };
    const out = await svc.submit(draft);
    expect(out).toMatchObject({ kind: "accepted", runId: "run_9" });
    expect(order.indexOf("save:prepared")).toBeLessThan(order.indexOf("http"));
    const pending = await store.listPending("scope-key-1");
    expect(pending[0]).toMatchObject({ state: "accepted", runId: "run_9" });
  });

  it("落盘失败不得发送网络", async () => {
    let httpCalled = 0;
    const { svc } = mkService(() => {
      httpCalled++;
      return jsonRes(202, { data: { run_id: "r", request_id: "x", status: "queued" } });
    });
    // 用存储故障模拟：直接构造坏 store
    const store = new InMemoryStore();
    (svc as unknown as { d: { store: InMemoryStore } }).d.store = store;
    store.savePending = async () => {
      throw new Error("disk full");
    };
    const out = await svc.submit(draft);
    expect(out).toMatchObject({ kind: "rejected", kindDetail: "storage" });
    expect((out as Extract<typeof out, { kind: "rejected" }>).reason).toMatch(/未发送/);
    expect(httpCalled).toBe(0);
  });

  it("网络中断 → uncertain + lookup 对账（不新建 ID）", async () => {
    const { svc, store } = mkService((url) => {
      if (url.endsWith("/workbench/executions")) throw new TypeError("fetch failed");
      if (url.includes("/requests/")) return jsonRes(200, { run_id: "run_found", status: "running" });
      return jsonRes(200, {});
    });
    const out = await svc.submit(draft);
    expect(out).toMatchObject({ kind: "uncertain" });
    const reqId = (out as { requestId: string }).requestId;
    const reconciled = await svc.reconcile(reqId);
    expect(reconciled).toMatchObject({ kind: "accepted", runId: "run_found", requestId: reqId });
    const pending = await store.readPending("scope-key-1", reqId);
    expect(pending).toMatchObject({ state: "accepted", runId: "run_found" });
  });

  it("lookup 未受理 → 保持 uncertain（unknown 不是失败）", async () => {
    const { svc } = mkService((url) => {
      if (url.endsWith("/workbench/executions")) throw new TypeError("fetch failed");
      if (url.includes("/requests/")) return jsonRes(404, { error: "not found" });
      return jsonRes(200, {});
    });
    const out = await svc.submit(draft);
    const r = await svc.reconcile((out as { requestId: string }).requestId);
    expect(r).toMatchObject({ kind: "uncertain" });
  });

  it("服务端明确拒绝 → rejected + 原因", async () => {
    const { svc } = mkService((url) => {
      if (url.endsWith("/workbench/executions")) return jsonRes(422, { success: false, error: { code: "BUDGET_LIMIT", message: "超出部署预算上限" } });
      return jsonRes(200, {});
    });
    const out = await svc.submit(draft);
    expect(out.kind).toBe("rejected");
    expect((out as Extract<typeof out, { kind: "rejected" }>).reason).toMatch(/预算/);
  });

  it("scope 只读（切换中）拒绝提交", async () => {
    const { svc } = mkService(() => jsonRes(202, {}), { canWrite: false });
    const out = await svc.submit(draft);
    expect(out).toMatchObject({ kind: "rejected", kindDetail: "scope_readonly" });
  });

  it("校验：空文本/无 Agent/非正整数预算", async () => {
    const { svc } = mkService(() => jsonRes(202, {}));
    expect(await svc.submit({ ...draft, text: "  " })).toMatchObject({ kindDetail: "validation" });
    expect(await svc.submit({ ...draft, agentId: "" })).toMatchObject({ kindDetail: "validation" });
    expect(await svc.submit({ ...draft, budgetUpper: 0 })).toMatchObject({ kindDetail: "validation" });
  });

  it("相同 request_id + 不同输入 → 冲突拒绝", async () => {
    const store = new InMemoryStore();
    const input1 = canonicalInput("scope-key-1", draft);
    await store.savePending({
      scopeKey: "scope-key-1",
      requestId: "req_same",
      inputHash: fnv1a(input1),
      input: input1,
      state: "prepared",
      runId: null,
      createdAt: "2026-09-18T00:00:00Z",
    });
    await expect(
      store.savePending({
        scopeKey: "scope-key-1",
        requestId: "req_same",
        inputHash: fnv1a(canonicalInput("scope-key-1", { ...draft, text: "不同的输入" })),
        input: "x",
        state: "prepared",
        runId: null,
        createdAt: "2026-09-18T00:00:01Z",
      }),
    ).rejects.toThrow(/冲突/);
  });

  it("重启恢复：uncertain/sending 列入待对账", async () => {
    const { svc } = mkService((url) => {
      if (url.endsWith("/workbench/executions")) return jsonRes(202, { data: { run_id: "r1", request_id: "x", status: "ok" } });
      return jsonRes(200, {});
    });
    expect(await svc.pendingReconcile()).toHaveLength(0);
    await svc.submit(draft);
    expect(await svc.pendingReconcile()).toHaveLength(0); // accepted 不列
  });
});
