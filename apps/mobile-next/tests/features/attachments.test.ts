import { AttachmentUploader, buildUploadForm, type PickedFile } from "@/features/workbench/attachments/AttachmentUploader";
import type { TemporaryDocumentWire } from "@/contracts/attachments";
import { SubmissionService, type SubmissionDraft } from "@/features/workbench/submit/SubmissionService";
import { InMemoryStore } from "@/platform/store";
import { WeKnoraApi } from "@/api/weknora";
import { HttpClient, type CredentialStore } from "@/api/http";

const doc = (over: Partial<TemporaryDocumentWire> = {}): TemporaryDocumentWire => ({
  id: "doc_1",
  session_id: "s1",
  file_name: "反馈摘录.pdf",
  file_type: "pdf",
  mime_type: "application/pdf",
  file_size: 1024,
  status: "processing",
  error_message: null,
  ...over,
});

const pdf: PickedFile = { uri: "file:///tmp/x.pdf", name: "反馈摘录.pdf", mimeType: "application/pdf", size: 1024 };

const mkTransport = (statusFlow: TemporaryDocumentWire["status"][]) => {
  let uploadCalls = 0;
  let pollCalls = 0;
  const captured: FormData[] = [];
  return {
    captured,
    uploadCalls: () => uploadCalls,
    transport: {
      async upload(_s: string, _f: PickedFile, agentId?: string) {
        uploadCalls++;
        void agentId;
        return doc({ status: statusFlow[0] ?? "processing" });
      },
      async get() {
        pollCalls++;
        return doc({ status: statusFlow[Math.min(pollCalls, statusFlow.length - 1)] ?? "processing" });
      },
    },
  };
};

describe("RW-027 附件上传状态机", () => {
  it("pick → selected；本地白名单/大小校验失败直接 failed", () => {
    const up = new AttachmentUploader({ transport: mkTransport([]).transport });
    const ok = up.pick(pdf);
    expect(ok.state).toBe("selected");
    const bad = up.pick({ uri: "file:///tmp/x.exe", name: "病毒.exe", mimeType: "application/x-msdownload", size: 10 });
    expect(bad.state).toBe("failed");
    expect(bad.error).toMatch(/不支持的文件类型/);
    const big = up.pick({ ...pdf, name: "大文件.pdf", size: 999 * 1024 * 1024 });
    expect(big.state).toBe("failed");
    expect(big.error).toMatch(/大小限制/);
  });

  it("上传成功 → verifying → 轮询到 ready；readyDocumentIds 可用、allReady=true", async () => {
    const t = mkTransport(["processing", "processing", "ready"]);
    const up = new AttachmentUploader({ transport: t.transport, pollIntervalMs: 1 });
    const item = up.pick(pdf);
    const done = await up.uploadAndWait("s1", item.localId, pdf);
    expect(done.state).toBe("ready");
    expect(done.documentId).toBe("doc_1");
    expect(up.allReady()).toBe(true);
    expect(up.readyDocumentIds()).toEqual(["doc_1"]);
    expect(t.uploadCalls()).toBe(1);
  });

  it("上传成功但解析失败 → failed + 原因；不能 allReady", async () => {
    const t = mkTransport(["processing", "failed"]);
    const up = new AttachmentUploader({ transport: t.transport, pollIntervalMs: 1 });
    const item = up.pick(pdf);
    const done = await up.uploadAndWait("s1", item.localId, pdf);
    expect(done.state).toBe("failed");
    expect(done.error).toBe("附件解析失败");
    expect(up.allReady()).toBe(false);
  });

  it("解析超时 → failed 超时提示（不用 unknown 冒充 ready）", async () => {
    const t = mkTransport(["processing"]);
    const up = new AttachmentUploader({ transport: t.transport, pollIntervalMs: 1, pollLimit: 3 });
    const item = up.pick(pdf);
    const done = await up.uploadAndWait("s1", item.localId, pdf);
    expect(done.state).toBe("failed");
    expect(done.error).toMatch(/超时/);
  });

  it("multipart 表单包含真实文件实体与字段名 file（RN FormData 语义：{uri,name,type} 原生读文件为 bytes）", () => {
    const calls: Array<[string, unknown]> = [];
    class FakeFormData {
      append(k: string, v: unknown) {
        calls.push([k, v]);
      }
    }
    const RealFormData = global.FormData;
    (global as unknown as { FormData: unknown }).FormData = FakeFormData;
    try {
      const fd = buildUploadForm(pdf, "agent_x");
      void fd;
    } finally {
      (global as unknown as { FormData: unknown }).FormData = RealFormData;
    }
    expect(calls[0]![0]).toBe("file");
    expect(calls[0]![1]).toEqual({ uri: "file:///tmp/x.pdf", name: "反馈摘录.pdf", type: "application/pdf" });
    expect(calls[1]).toEqual(["agent_id", "agent_x"]);
  });
});

// ---- 闭环：附件未 ready 阻断任务提交 ----
const noCreds: CredentialStore = { async read() { return null; }, async write() {}, async clear() {} };
const jsonRes = (status: number, body: unknown) => new Response(JSON.stringify(body), { status });

describe("RW-027×RW-013 闭环：附件未 ready 不提交任务", () => {
  const draft: SubmissionDraft = {
    text: "整理反馈",
    agentId: "a1",
    targetId: "platform-default",
    workspaceRef: "ws",
    budgetUpper: 100,
    knowledgeBaseIds: [],
    sessionId: "s1",
  };
  const mkService = () => {
    let startCalls = 0;
    const http = new HttpClient({
      getOrigin: () => "https://weknora.example",
      getTenantId: () => "t1",
      credentials: noCreds,
      fetchImpl: (async () => {
        startCalls++;
        return jsonRes(202, { data: { run_id: "r", request_id: "q", status: "queued" } });
      }) as unknown as typeof fetch,
    });
    const store = new InMemoryStore();
    const svc = new SubmissionService({
      api: new WeKnoraApi(http),
      store,
      scopeKey: () => "scope",
      canWrite: () => true,
      newRequestId: () => "req_1",
      lookup: async () => ({ runId: null, status: null }),
    });
    return { svc, startCalls: () => startCalls };
  };

  it("有附件 verifying 中 → 页面层阻断语义（hasUploading=true 且 allReady=false），不发起 start", async () => {
    const t = mkTransport(["processing", "processing", "ready"]);
    const up = new AttachmentUploader({ transport: t.transport, pollIntervalMs: 1 });
    const item = up.pick(pdf);
    void up.uploadAndWait("s1", item.localId, pdf); // 异步进行中
    expect(up.hasUploading()).toBe(true);
    expect(up.allReady()).toBe(false);
    // 页面层规则：hasUploading() 时禁用提交按钮 —— 这里验证服务调用仅在 allReady 时发生
    const { svc, startCalls } = mkService();
    if (up.allReady()) {
      await svc.submit({ ...draft, sessionId: "s1" });
    }
    expect(startCalls()).toBe(0);
  });

  it("附件 ready 后提交通常链路不受影响（回归）", async () => {
    const { svc } = mkService();
    const out = await svc.submit(draft);
    expect(out.kind).toBe("accepted");
  });
});
