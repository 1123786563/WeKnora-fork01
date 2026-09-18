// RW-022（M14 任务成果）页面行为验收测试。
// 断言核心语义：不可变版本+来源任务展示、下载走签名链接真实 bytes（成功提示与本地缓存）、
// 部署未启用签名如实提示、分享交出本地缓存文件（不携带登录态）、forbidden 遮蔽。
import React from "react";
import { fireEvent, screen, waitFor, render } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import type { ArtifactWire } from "@/contracts/workbench";
import type { ArtifactDownloadOutcome } from "@/features/executions/artifactDownload";

const mockRouter = { push: jest.fn(), back: jest.fn(), replace: jest.fn() };
let mockParams: Record<string, string> = {};
jest.mock("expo-router", () => ({
  // 惰性 getter：工厂在 import 提升阶段执行，直接展开 mockRouter 会得到 undefined
  get router() {
    return mockRouter;
  },
  useRouter: () => mockRouter,
  useLocalSearchParams: () => mockParams,
}));

jest.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
}));

const mockShareAsync = jest.fn(async () => undefined);
jest.mock("expo-sharing", () => ({
  get shareAsync() {
    return mockShareAsync;
  },
}));

// 页面下载成功后读文本预览 require expo-file-system：mock 为内存实现
jest.mock("expo-file-system", () => {
  class FakeFile {
    uri: string;
    constructor(uri: string) {
      this.uri = uri;
    }
    text() {
      return Promise.resolve("# 预览内容");
    }
    write() {}
  }
  return { File: FakeFile, Paths: { cache: "/cache" } };
});

let mockApp: Partial<AppHost>;
jest.mock("@/host/AppProvider", () => ({
  useApp: () => mockApp,
}));

import ArtifactDetailScreen from "../../app/artifacts/[id]";

const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;

const artifactFixture: ArtifactWire = {
  id: "msg-1:0",
  name: "客户反馈分析.md",
  mime: "text/markdown",
  version: "1",
  size: 12 * 1024,
  source_run: "run-1",
  created_at: "2026-09-18T10:00:00Z",
  index: 0,
};

function appWith(artifacts: ArtifactWire[], download: AppHost["downloadArtifact"]) {
  return {
    visualFixture: false,
    scope: { generation: { value: 1 } } as AppHost["scope"],
    api: {
      artifacts: jest.fn(async () => artifacts),
    },
    downloadArtifact: download,
  } as unknown as Partial<AppHost>;
}

describe("RW-022 M14 任务成果", () => {
  beforeEach(() => {
    mockParams = { id: "msg-1:0", runId: "run-1" };
    mockShareAsync.mockClear();
  });

  it("展示不可变版本/大小/来源任务；下载与分享按钮存在", async () => {
    mockApp = appWith([artifactFixture], jest.fn());
    await render(wrap(<ArtifactDetailScreen />));

    expect(await screen.findByText("客户反馈分析.md")).toBeDefined();
    expect(screen.getByText(/v1 · 12 KB · 来源任务 run-1/)).toBeDefined();
    expect(screen.getByText("只读版本 · 内容不可覆盖 · 当前空间内已授权成员")).toBeDefined();
    expect(screen.getByTestId("download-artifact")).toBeDefined();
    expect(screen.getByTestId("share-artifact")).toBeDefined();
  });

  it("下载成功：提示本地缓存并展示文本预览；分享交出本地文件（不携带登录态）", async () => {
    const download = jest.fn(async (): Promise<ArtifactDownloadOutcome> => ({
      kind: "downloaded",
      localUri: "file:///cache/wk-artifact-1-summary.md",
    }));
    mockApp = appWith([artifactFixture], download);
    await render(wrap(<ArtifactDetailScreen />));

    fireEvent.press(await screen.findByTestId("download-artifact"));
    expect(await screen.findByText("已下载到本地缓存，可在此页面预览或分享。")).toBeDefined();
    expect(download).toHaveBeenCalledWith("run-1", artifactFixture);
    await waitFor(() => expect(screen.getByText(/# 预览内容/)).toBeDefined());

    // 分享用已下载的本地文件 URI；不再重复下载
    fireEvent.press(screen.getByTestId("share-artifact"));
    await waitFor(() =>
      expect(mockShareAsync).toHaveBeenCalledWith(
        "file:///cache/wk-artifact-1-summary.md",
        expect.objectContaining({ mimeType: "text/markdown" }),
      ),
    );
    expect(download).toHaveBeenCalledTimes(1);
  });

  it("部署未启用签名下载 → 如实提示（不假装成功）", async () => {
    mockApp = appWith(
      [artifactFixture],
      jest.fn(async (): Promise<ArtifactDownloadOutcome> => ({ kind: "signing_disabled" })),
    );
    await render(wrap(<ArtifactDetailScreen />));

    fireEvent.press(await screen.findByTestId("download-artifact"));
    expect(await screen.findByText("此部署未启用签名下载（需配置 WEKNORA_ARTIFACT_SIGNING_KEY）。")).toBeDefined();
  });

  it("签名过期且重授权仍失败 → 提示重试（grant 语义）", async () => {
    mockApp = appWith(
      [artifactFixture],
      jest.fn(async (): Promise<ArtifactDownloadOutcome> => ({ kind: "grant_expired" })),
    );
    await render(wrap(<ArtifactDetailScreen />));

    fireEvent.press(await screen.findByTestId("download-artifact"));
    expect(await screen.findByText(/下载授权已过期（已自动重新授权一次仍失败），请稍后重试。/)).toBeDefined();
  });

  it("forbidden → 遮蔽", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        artifacts: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await render(wrap(<ArtifactDetailScreen />));
    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });

  it("列表中不存在 → 空态（不泄漏存在性）", async () => {
    mockApp = appWith([{ ...artifactFixture, id: "other:0" }], jest.fn());
    await render(wrap(<ArtifactDetailScreen />));
    expect(await screen.findByText("成果不存在")).toBeDefined();
  });
});
