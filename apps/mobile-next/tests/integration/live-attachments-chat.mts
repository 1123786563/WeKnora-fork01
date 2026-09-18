// 一次性真实后端联调扩展（RW-027/RW-015 证据）：附件上传真实 bytes + agent-chat SSE 流。
// node 环境用 Blob 构造 multipart（node FormData 原生支持）；随机账号，不入库。
import { AttachmentUploader, type PickedFile } from "../../src/features/workbench/attachments/AttachmentUploader.ts";
import { ChatProjection, ChatService } from "../../src/features/conversations/chat/ChatService.ts";
import { decodeTemporaryDocument } from "../../src/contracts/attachments.ts";

const ORIGIN = process.env.WEKNORA_ORIGIN ?? "http://localhost:8082";
const email = `mn2_${Date.now().toString(36)}@test.local`;
const password = `Mn-${Math.random().toString(36).slice(2, 10)}!`;

const assert = (cond: unknown, msg: string) => {
  if (!cond) {
    console.error(`✗ ${msg}`);
    process.exit(1);
  }
  console.log(`✓ ${msg}`);
};

// 1) 注册 + 登录
await fetch(`${ORIGIN}/api/v1/auth/register`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email, password, username: email.split("@")[0] }),
});
const loginBody = await (await fetch(`${ORIGIN}/api/v1/auth/login`, {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ email, password }),
})).json();
const token = String(loginBody.token ?? "");
assert(token.length > 20, "登录取得 token");
const auth = { Authorization: `Bearer ${token}`, "X-Tenant-ID": String(loginBody.active_tenant?.id ?? "") };

// 2) 创建会话
const sessionRes = await fetch(`${ORIGIN}/api/v1/sessions`, {
  method: "POST",
  headers: { "Content-Type": "application/json", ...auth },
  body: JSON.stringify({ title: "附件联调" }),
});
const sessionBody = await sessionRes.json();
const sessionId = String((sessionBody.data ?? sessionBody).id ?? "");
assert(sessionId.length > 0, `创建会话（${sessionId}）`);

// 3) 附件：multipart 真实 bytes 上传（node Blob，与 RN {uri,name,type} 等价的原生传输）
const fileContent = "# 客户反馈摘录\n\n1. 登录偶尔失败\n2. 进度提示不清楚\n3. 导出太慢\n";
const file = new File([fileContent], "反馈摘录.md", { type: "text/markdown" });
const fd = new FormData();
fd.append("file", file);
const upRes = await fetch(`${ORIGIN}/api/v1/sessions/${sessionId}/attachments`, { method: "POST", headers: auth, body: fd });
assert(upRes.status === 202, `附件上传被受理（HTTP ${upRes.status}）`);
const upBody = await upRes.json();
const doc = decodeTemporaryDocument(upBody.data ?? upBody);
assert(doc.id.length > 0, `wire 解码 document（id=${doc.id.slice(0, 8)}… status=${doc.status}）`);

// 4) AttachmentUploader 状态机对真实后端轮询到 ready
const uploader = new AttachmentUploader({
  transport: {
    upload: async (_s, f: PickedFile) => {
      const r = await fetch(`${ORIGIN}/api/v1/sessions/${sessionId}/attachments`, {
        method: "POST",
        headers: auth,
        body: (() => {
          const d = new FormData();
          d.append("file", new File(["重复文件内容"], f.name, { type: f.mimeType }));
          return d;
        })(),
      });
      const b = await r.json();
      return decodeTemporaryDocument(b.data ?? b);
    },
    get: async (_s, id) => {
      const r = await fetch(`${ORIGIN}/api/v1/sessions/${sessionId}/attachments/${id}`, { headers: auth });
      const b = await r.json();
      return decodeTemporaryDocument(b.data ?? b);
    },
  },
  pollIntervalMs: 800,
  pollLimit: 30,
});
const item = uploader.pick({ uri: "file:///tmp/x.md", name: "反馈摘录2.md", mimeType: "text/markdown", size: fileContent.length });
const settled = await uploader.uploadAndWait(sessionId, item.localId, { uri: "file:///tmp/x.md", name: "反馈摘录2.md", mimeType: "text/markdown", size: fileContent.length });
assert(settled.state === "ready", `附件解析 ready（documentId=${settled.documentId?.slice(0, 8)}…，轮询=${uploader.allReady()}）`);
assert(uploader.readyDocumentIds().length === 1, "readyDocumentIds 提供提交用 id");

// 5) agent-chat：POST → SSE 流消费（builtin Quick Answer；provider 未配置时如实记录）
const agentId = "builtin-quick-answer";
const projection = new ChatProjection();
let outcome: string | null = null;
let errText: string | null = null;
const chat = new ChatService({
  buildUrl: (sid) => `${ORIGIN}/api/v1/agent-chat/${sid}`,
  headers: () => auth,
});
await chat.send(
  sessionId,
  { query: "用一句话说明这份附件的要点", agent_id: agentId, attachment_ids: uploader.readyDocumentIds() },
  projection,
  {
    onUpdate: () => {},
    onDone: () => {
      outcome = "done";
    },
    onError: (e) => {
      outcome = "error";
      errText = e.message;
    },
    onDisconnected: () => {
      outcome ??= "disconnected";
    },
  },
);
if (outcome === "done") {
  const text = projection.list.filter((m) => m.role === "assistant").map((m) => m.parts.map((p) => (p as { text?: string }).text ?? "").join("")).join("");
  assert(text.length > 0, `agent-chat 流式回复完成（${text.length} 字）：${text.slice(0, 60)}…`);
} else if (outcome === "error") {
  console.log(`⚠ agent-chat 返回错误（可能未配置模型 provider）：${errText}`);
} else {
  console.log(`⚠ agent-chat 未得到终态帧（outcome=${outcome}）；投影消息数=${projection.list.length}（断流路径已由单测覆盖）`);
}

console.log("\n=== 附件 + 聊天真实联调完成 ===");
