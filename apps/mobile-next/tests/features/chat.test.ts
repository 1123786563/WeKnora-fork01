import { ChatProjection, ChatService, consumeChatStream, type ChatStreamHandlers } from "@/features/conversations/chat/ChatService";

const sseRes = (frames: string[]) =>
  new Response(
    new ReadableStream<Uint8Array>({
      start(controller) {
        for (const f of frames) controller.enqueue(new TextEncoder().encode(f));
        controller.close();
      },
    }),
    { status: 200, headers: { "Content-Type": "text/event-stream" } },
  );

const chunk = (type: string, content: string, done = false) =>
  `event: message\ndata: ${JSON.stringify({ response_type: type, content, done })}\n\n`;

const mkHandlers = () => {
  const log: string[] = [];
  return {
    log,
    handlers: {
      onUpdate: () => log.push("update"),
      onDone: () => log.push("done"),
      onError: (e: Error) => log.push(`error:${e.message}`),
      onDisconnected: () => log.push("disconnected"),
    } satisfies ChatStreamHandlers,
  };
};

describe("RW-015 ChatProjection 消息投影", () => {
  it("answer 增量拼接同一条 assistant 消息", () => {
    const p = new ChatProjection();
    p.appendUser("整理反馈");
    const apply = (json: object) =>
      p.applyChunk({
        id: null,
        response_type: "answer",
        content: "",
        done: false,
        session_id: null,
        assistant_message_id: null,
        tool_calls: null,
        data: null,
        finish_reason: null,
        ...json,
      } as Parameters<ChatProjection["applyChunk"]>[0]);
    apply({ response_type: "answer", content: "已找到 " });
    apply({ response_type: "answer", content: "12 条反馈。" });
    apply({ response_type: "complete", content: "", done: true });
    expect(p.list).toHaveLength(2);
    const a = p.list[1]!;
    expect(a.role).toBe("assistant");
    expect(a.streaming).toBe(false);
    expect((a.parts[0] as { text: string }).text).toBe("已找到 12 条反馈。");
  });

  it("tool_call/tool_result/thinking 分开成 part；error 持久展示并终止", () => {
    const p = new ChatProjection();
    const c = (type: string, content: string, done = false) =>
      p.applyChunk({
        id: null, response_type: type as never, content, done, session_id: null,
        assistant_message_id: null, tool_calls: null, data: null, finish_reason: null,
      });
    c("thinking", "先检索知识库");
    c("tool_call", "knowledge_search");
    c("tool_result", "命中 3 条");
    c("error", "模型服务暂不可用");
    const a = p.list[0]!;
    expect(a.parts.map((x) => x.kind)).toEqual(["thinking", "tool_call", "tool_result", "error"]);
    expect(a.streaming).toBe(false);
  });

  it("未知 response_type 安全展示不崩溃", () => {
    const p = new ChatProjection();
    p.applyChunk({
      id: null, response_type: "unknown", content: "新版本事件", done: true,
      session_id: null, assistant_message_id: null, tool_calls: null, data: null, finish_reason: null,
    });
    expect((p.list[0]!.parts[0] as { text: string }).text).toBe("新版本事件");
  });
});

describe("RW-015 consumeChatStream（真实 SSE 帧 event: message）", () => {
  it("完整流：增量 → done 帧结束", async () => {
    const p = new ChatProjection();
    const h = mkHandlers();
    const result = await consumeChatStream(
      sseRes([chunk("answer", "第一段"), chunk("answer", "第二段"), chunk("complete", "", true)]),
      p,
      h.handlers,
    );
    expect(result).toBe("done");
    expect(h.log).toContain("done");
    expect(p.list[0]!.streaming).toBe(false);
  });

  it("EOF 无终态帧 → disconnected（不是失败），携带最后消息 id 供续流", async () => {
    const p = new ChatProjection();
    const seen: Array<string | null> = [];
    const h: ChatStreamHandlers = {
      onUpdate: () => {},
      onDone: () => {},
      onError: () => {},
      onDisconnected: (_p, lastId) => seen.push(lastId),
    };
    const result = await consumeChatStream(sseRes([chunk("answer", "部分回答")]), p, h);
    expect(result).toBe("disconnected");
    expect(seen[0]).toBe(p.list[0]!.stableId);
  });

  it("残缺 JSON 帧跳过，不终止流", async () => {
    const p = new ChatProjection();
    const h = mkHandlers();
    const result = await consumeChatStream(
      sseRes(["event: message\ndata: {broken json\n\n", chunk("answer", "有效内容"), chunk("complete", "", true)]),
      p,
      h.handlers,
    );
    expect(result).toBe("done");
    expect((p.list[0]!.parts[0] as { text: string }).text).toBe("有效内容");
  });
});

describe("RW-015 ChatService.send（POST + 流消费闭环）", () => {
  const deps = (resOrFn: Response | ((url: string, init?: RequestInit) => Response)) => ({
    buildUrl: (id: string) => `https://w.example/api/v1/agent-chat/${id}`,
    headers: () => ({ Authorization: "Bearer t" }),
    fetchImpl: (async (url: any, init?: any) =>
      typeof resOrFn === "function" ? resOrFn(String(url), init as RequestInit) : resOrFn) as unknown as typeof fetch,
  });

  it("发送 → 用户消息入列 → 流式 assistant 回复完成", async () => {
    const bodies: object[] = [];
    const svc = new ChatService(
      deps((url, init) => {
        expect(url).toContain("/agent-chat/s1");
        bodies.push(JSON.parse(String(init?.body)) as object);
        return sseRes([chunk("answer", "收到，"), chunk("answer", "开始整理。"), chunk("complete", "", true)]);
      }),
    );
    const p = new ChatProjection();
    const h = mkHandlers();
    await svc.send("s1", { query: "整理反馈", agent_id: "a1", attachment_ids: ["doc_1"] }, p, h.handlers);
    expect(p.list).toHaveLength(2);
    expect(p.list[0]!.role).toBe("user");
    expect((p.list[1]!.parts[0] as { text: string }).text).toBe("收到，开始整理。");
    expect(h.log).toContain("done");
    expect(bodies[0]).toMatchObject({ query: "整理反馈", agent_id: "a1", attachment_ids: ["doc_1"], channel: "mobile" });
  });

  it("HTTP 失败 → onError（持久错误信息，不伪装成功）", async () => {
    const svc = new ChatService(deps(new Response(JSON.stringify({ message: "会话不存在" }), { status: 404 })));
    const p = new ChatProjection();
    const h = mkHandlers();
    await svc.send("s_bad", { query: "x" }, p, h.handlers);
    expect(h.log.some((l) => l.startsWith("error:"))).toBe(true);
    // 用户消息保留（发送失败保留草稿语义：投影不回滚，页面层提示）
    expect(p.list[0]!.role).toBe("user");
  });

  it("网络中断 → onDisconnected（续流信号）", async () => {
    const svc = new ChatService(
      deps(
        () =>
          new Response(
            new ReadableStream<Uint8Array>({
              start(c) {
                c.enqueue(new TextEncoder().encode(chunk("answer", "部分")));
                c.error(new TypeError("connection reset"));
              },
            }),
            { status: 200 },
          ),
      ),
    );
    const p = new ChatProjection();
    const h = mkHandlers();
    await svc.send("s1", { query: "x" }, p, h.handlers);
    expect(h.log).toContain("disconnected");
  });
});
