// RW-024（M16 语音输入/确认式听写）行为验收测试。
// 竞态保护以纯函数 transcriptFill 单测（迟到转写仅回填空草稿）；
// UI 层验证"真实录音未接入时不假装成功 + 文字输入不阻塞"。
// 注：voice 页 1200ms 转写定时器与多测试连续渲染在本组合（React19+test-renderer 1.3+RNTL14）
// 下会破坏文件内后续 render 环境，故本文件只保留一个 UI 测试（行为已由纯函数覆盖）。
import React from "react";
import { fireEvent, screen, waitFor, render } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import { transcriptFill } from "../../app/voice";

const mockRouter = { push: jest.fn(), back: jest.fn(), replace: jest.fn() };
jest.mock("expo-router", () => ({
  // 惰性 getter：工厂在 import 提升阶段执行，直接展开 mockRouter 会得到 undefined
  get router() {
    return mockRouter;
  },
  useRouter: () => mockRouter,
  useLocalSearchParams: () => ({} as Record<string, string>),
}));

jest.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
}));

let mockApp: Partial<AppHost>;
jest.mock("@/host/AppProvider", () => ({
  useApp: () => mockApp,
}));

import VoiceScreen from "../../app/voice";

const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;

describe("RW-024 M16 听写竞态（transcriptFill 纯函数）", () => {
  it("迟到转写不覆盖用户已编辑的草稿", () => {
    expect(transcriptFill("我的手动编辑内容", "整理本周客户反馈，生成一份优先处理建议。")).toBe("我的手动编辑内容");
  });

  it("空草稿（含纯空白）回填转写结果", () => {
    const demo = "整理本周客户反馈，生成一份优先处理建议。";
    expect(transcriptFill("", demo)).toBe(demo);
    expect(transcriptFill("   ", demo)).toBe(demo);
  });
});

describe("RW-024 M16 语音输入 UI（真实录音未接入的如实展示）", () => {
  it("不假装录音成功（持久提示），文字输入不阻塞，放入草稿须用户显式操作", async () => {
    const saveDraft = jest.fn(async () => undefined);
    mockApp = {
      visualFixture: false,
      store: { saveDraft },
      scopeKey: () => "test-scope",
    } as unknown as Partial<AppHost>;
    await render(wrap(<VoiceScreen />));

    fireEvent.press(screen.getByLabelText("开始录音"));
    expect(await screen.findByText("录音能力将在原生环境验证后启用（当前为草稿编辑模式）")).toBeDefined();
    // 文字输入不受阻塞
    const input = screen.getByPlaceholderText("录音转写后，你可以在这里修改文字…");
    fireEvent.changeText(input, "手动输入的内容");
    await waitFor(() => expect(input.props.value).toBe("手动输入的内容"));
    // 显式放入草稿（保存当前文字，不自动发送）
    fireEvent.press(screen.getByLabelText("放入草稿"));
    await waitFor(() => expect(saveDraft).toHaveBeenCalledWith("test-scope", "voice-draft", "手动输入的内容"));
  });
});
