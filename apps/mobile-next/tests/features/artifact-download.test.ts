// RW-022 成果下载状态机（artifactDownload）单测：
// 成功链路、签名过期自动重授权一次（闭环）、重签仍失败、501 如实、403/404、网络错误。
import {
  downloadArtifact,
  SigningDisabledError,
  type ArtifactDownloadIO,
} from "@/features/executions/artifactDownload";

function makeIO(overrides: Partial<ArtifactDownloadIO> = {}): {
  io: ArtifactDownloadIO;
  signedCalls: () => number;
  fetches: string[];
  writes: Array<{ name: string; bytes: Uint8Array }>;
} {
  let signedCalls = 0;
  const fetches: string[] = [];
  const writes: Array<{ name: string; bytes: Uint8Array }> = [];
  const io: ArtifactDownloadIO = {
    requestSignedUrl: async () => {
      signedCalls += 1;
      return `https://weknora.example.com/download?attempt=${signedCalls}`;
    },
    fetchBytes: async (url) => {
      fetches.push(url);
      return { status: 200, bytes: new Uint8Array([1, 2, 3]) };
    },
    writeFile: async (name, bytes) => {
      writes.push({ name, bytes });
      return `file:///cache/${name}`;
    },
    ...overrides,
  };
  return { io, signedCalls: () => signedCalls, fetches, writes };
}

describe("RW-022 downloadArtifact 状态机", () => {
  it("成功：签名 → 200 bytes → 落盘，返回本地 URI", async () => {
    const { io, signedCalls, writes } = makeIO();
    const outcome = await downloadArtifact(io, "summary.md");
    expect(outcome).toEqual({ kind: "downloaded", localUri: "file:///cache/summary.md" });
    expect(signedCalls()).toBe(1);
    expect(writes[0]!.bytes).toEqual(new Uint8Array([1, 2, 3]));
  });

  it("文件名不安全字符被替换（不构造路径穿越）", async () => {
    const { io, writes } = makeIO();
    await downloadArtifact(io, "../../etc/passwd");
    expect(writes[0]!.name).not.toContain("/");
  });

  it("签名过期（401）：自动重授权一次并重试成功", async () => {
    let first = true;
    const { io, signedCalls } = makeIO({
      fetchBytes: async (url) => {
        if (url.endsWith("attempt=1") && first) {
          first = false;
          return { status: 401 };
        }
        return { status: 200, bytes: new Uint8Array([9]) };
      },
    });
    const outcome = await downloadArtifact(io, "data.csv");
    expect(outcome).toEqual({ kind: "downloaded", localUri: "file:///cache/data.csv" });
    expect(signedCalls()).toBe(2); // 重授权闭环：重新签名，不换目标
  });

  it("重签后仍 401 → grant_expired 如实返回（不无限重试）", async () => {
    const { io, signedCalls, writes } = makeIO({
      fetchBytes: async () => ({ status: 401 }),
    });
    const outcome = await downloadArtifact(io, "a.md");
    expect(outcome).toEqual({ kind: "grant_expired" });
    expect(signedCalls()).toBe(2);
    expect(writes).toHaveLength(0);
  });

  it("部署未启用签名（SigningDisabledError / 501 code）→ signing_disabled", async () => {
    const { io: io1 } = makeIO({
      requestSignedUrl: async () => {
        throw new SigningDisabledError("disabled");
      },
    });
    expect(await downloadArtifact(io1, "a.md")).toEqual({ kind: "signing_disabled" });

    // ApiError 形态：kind=server + code=artifact_signing_disabled
    const { io: io2 } = makeIO({
      requestSignedUrl: async () => {
        throw Object.assign(new Error("not configured"), {
          kind: "server",
          code: "artifact_signing_disabled",
        });
      },
    });
    expect(await downloadArtifact(io2, "a.md")).toEqual({ kind: "signing_disabled" });
  });

  it("下载期 403/404 分别映射 forbidden/not_found；网络错误保留信息", async () => {
    const { io: io403 } = makeIO({ fetchBytes: async () => ({ status: 403 }) });
    expect(await downloadArtifact(io403, "a.md")).toEqual({ kind: "forbidden" });

    const { io: io404 } = makeIO({ fetchBytes: async () => ({ status: 404 }) });
    expect(await downloadArtifact(io404, "a.md")).toEqual({ kind: "not_found" });

    const { io: ioNet } = makeIO({
      fetchBytes: async () => {
        throw new Error("connection reset");
      },
    });
    expect(await downloadArtifact(ioNet, "a.md")).toEqual({ kind: "network", message: "connection reset" });
  });

  it("写文件失败 → network 错误（不冒充成功）", async () => {
    const { io } = makeIO({
      writeFile: async () => {
        throw new Error("disk full");
      },
    });
    const outcome = await downloadArtifact(io, "a.md");
    expect(outcome.kind).toBe("network");
    if (outcome.kind === "network") expect(outcome.message).toContain("disk full");
  });
});
