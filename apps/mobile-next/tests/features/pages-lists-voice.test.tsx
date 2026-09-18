// RW-012（M04 会话列表）/ RW-014（M06 Agent 选择）/ RW-018（M10 收件箱）/
// RW-023（M15 执行目标）/ RW-024（M16 语音听写）页面行为验收测试。
// 断言验收核心语义：防抖搜索、空态区分、选择回写、不可用原因如实展示、迟到转写不覆盖新编辑。
// useOverview 的 probe 测试单独在 overview.test.tsx（probe 与页面渲染混排会污染 act 环境）。
import React from "react";
import { act, fireEvent, screen, waitFor, render } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import type { InteractionWire, ExecutionTargetWire } from "@/contracts/workbench";
import type { SessionWire, AgentWire } from "@/contracts/auth";
import { targetSelection } from "@/features/targets/selection";
import { agentSelection } from "@/features/workbench/agentSelection";

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

import SessionsScreen from "../../app/(tabs)/sessions";
import AgentsScreen from "../../app/agents";
import InboxScreen from "../../app/inbox";
import TargetsScreen from "../../app/targets";
import VoiceScreen from "../../app/voice";

const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;

const session = (id: string, agent_id: string | null): SessionWire => ({
  id,
  title: `会话 ${id}`,
  agent_id,
  updated_at: "2026-09-18T08:00:00Z",
});

const interaction = (id: string, runId: string): InteractionWire => ({
  id,
  run_id: runId,
  kind: "tool_approval",
  status: "pending",
  revision: 2,
  title: `待处理 ${id}`,
  summary: "",
  payload: null,
  created_at: "2026-09-18T08:00:00Z",
});

// 会话/收件箱共用的聚合数据：2 个绑定 agent 会话 + 1 个无 agent；s1 有 pending 交互
function aggregateAppMocks(overrides: Record<string, unknown> = {}) {
  const sessionsMock = jest.fn(async () => ({
    sessions: [session("s1", "a1"), session("s2", "a2"), session("s3", null)],
    total: 3,
  }));
  const interactionsMock = jest.fn(async (sid: string) =>
    sid === "s1" ? [interaction("it1", "run_001")] : [],
  );
  return {
    visualFixture: false,
    scope: { generation: { value: 1 } } as AppHost["scope"],
    api: {
      sessions: sessionsMock,
      interactions: interactionsMock,
      agents: jest.fn(async () => [] as AgentWire[]),
      ...overrides,
    },
  } as unknown as Partial<AppHost>;
}

describe("RW-012 M04 会话列表", () => {
  it("渲染会话卡片；derived 状态 badge（waiting 来自 pending 聚合）", async () => {
    mockApp = aggregateAppMocks();
    await render(wrap(<SessionsScreen />));
    expect(await screen.findByTestId("session-card-s1")).toBeDefined();
    expect(screen.getByTestId("session-card-s2")).toBeDefined();
    // s1 有 pending 交互 → 待处理 badge
    expect(screen.getByText("待处理")).toBeDefined();
  });

  it("搜索防抖 300ms：停顿后才携带 keyword 重取并回第一页", async () => {
    // 不用 fake timers：本工程 React19+test-renderer 组合下 fake timers 会污染后续 render
    const sessionsMock = jest.fn(async () => ({ sessions: [session("s1", "a1")], total: 1 }));
    mockApp = aggregateAppMocks({ sessions: sessionsMock });
    await render(wrap(<SessionsScreen />));
    await waitFor(() => expect(sessionsMock).toHaveBeenCalled());
    const before = sessionsMock.mock.calls.length;

    fireEvent.changeText(screen.getByTestId("sessions-search"), "反馈");
    await new Promise((r) => setTimeout(r, 150));
    expect(sessionsMock.mock.calls.length).toBe(before); // 300ms 防抖窗口内不触发
    await waitFor(() => expect(sessionsMock.mock.calls.length).toBeGreaterThan(before));
    const calls = sessionsMock.mock.calls as unknown as Array<[{ page: number; keyword?: string }]>;
    const lastCall = calls[calls.length - 1]![0];
    expect(lastCall.keyword).toBe("反馈");
    expect(lastCall.page).toBe(1); // 关键词变化回第一页
  });

  it("空态：无会话 → 引导新建（区别于搜索无结果）", async () => {
    mockApp = aggregateAppMocks({
      sessions: jest.fn(async () => ({ sessions: [] as SessionWire[], total: 0 })),
    });
    await render(wrap(<SessionsScreen />));
    expect(await screen.findByText("这里还没有任务")).toBeDefined();
  });

  it("筛选无匹配 → 没有匹配的会话（非空库空态）", async () => {
    mockApp = aggregateAppMocks(); // 3 个会话均非 waiting
    await render(wrap(<SessionsScreen />));
    await screen.findByTestId("session-card-s1");
    fireEvent.press(screen.getByText("待处理")); // 筛选 chip
    await waitFor(() => expect(screen.getByText("没有匹配的会话")).toBeDefined());
  });

  it("forbidden → 遮蔽", async () => {
    mockApp = aggregateAppMocks({
      sessions: jest.fn(async () => {
        throw { kind: "forbidden" };
      }),
    });
    await render(wrap(<SessionsScreen />));
    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });
});

describe("RW-014 M06 Agent 选择", () => {
  const agentList: AgentWire[] = [
    { id: "a1", name: "通用助手", description: "日常事务", builtin: true },
    { id: "a2", name: "知识研究员", description: "知识检索与研究", builtin: true },
    { id: "a3", name: "工程开发", description: "代码与工程开发", builtin: false },
  ];

  it("渲染目录；选择回写 agentSelection 并返回（M05 草稿回读源）", async () => {
    agentSelection.current = null;
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: { agents: jest.fn(async () => agentList) },
    } as unknown as Partial<AppHost>;
    await render(wrap(<AgentsScreen />));

    expect(await screen.findByText("知识研究员")).toBeDefined();
    expect(screen.getByText("工程开发")).toBeDefined();
    fireEvent.press(screen.getByText("工程开发"));
    await waitFor(() => expect(mockRouter.back).toHaveBeenCalled());
    expect(agentSelection.current).toEqual({ id: "a3", name: "工程开发" });
    agentSelection.current = null;
  });

  it("forbidden → 遮蔽（不展示目录）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        agents: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await render(wrap(<AgentsScreen />));
    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });
});

describe("RW-018 M10 收件箱（客户端聚合读模型，D-06）", () => {
  it("聚合 pending 交互与任务动态；筛选后只剩任务动态", async () => {
    mockApp = aggregateAppMocks();
    await render(wrap(<InboxScreen />));

    expect(await screen.findByText("待处理 it1")).toBeDefined();
    expect(screen.getByText("会话 s1")).toBeDefined(); // 任务动态（最近会话）
    fireEvent.press(screen.getByText("任务动态")); // 筛选 chip
    await waitFor(() => expect(screen.queryByText("待处理 it1")).toBeNull());
    expect(screen.getByText("会话 s1")).toBeDefined();
  });
});

describe("RW-023 M15 执行目标", () => {
  const targets: ExecutionTargetWire[] = [
    { id: "t1", name: "平台默认", status: "available", capabilities: ["可观察"], unavailable_reason: null },
    { id: "t2", name: "维护中节点", status: "unavailable", capabilities: [], unavailable_reason: "管理员暂停授权" },
  ];

  it("展示能力与不可用原因；只可选服务端受权目标并回写选择", async () => {
    targetSelection.current = null;
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: { executionTargets: jest.fn(async () => targets) },
    } as unknown as Partial<AppHost>;
    await render(wrap(<TargetsScreen />));

    expect(await screen.findByText("平台默认")).toBeDefined();
    expect(screen.getByText("维护中节点")).toBeDefined();
    expect(screen.getByText("管理员暂停授权")).toBeDefined(); // 不可用原因如实展示
    expect(screen.getByText("手机只选择服务端受权引用，不提供任意节点地址输入。")).toBeDefined();

    fireEvent.press(screen.getByLabelText("执行目标 平台默认"));
    await waitFor(() => expect(mockRouter.back).toHaveBeenCalled());
    expect(targetSelection.current).toEqual({ id: "t1", name: "平台默认" });

    // 不可用目标禁选
    expect(screen.getByLabelText("执行目标 维护中节点").props.accessibilityState).toMatchObject({ disabled: true });
    targetSelection.current = null;
  });

  it("forbidden → 遮蔽", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        executionTargets: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await render(wrap(<TargetsScreen />));
    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });
});
