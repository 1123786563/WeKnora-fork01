// RW-011（M03 工作台聚合）useOverview 行为验收测试。
// 单独成文件：probe 渲染与页面渲染混排会互相污染 act 环境（React 19 + test-renderer 组合，见 progress 注记）。
import React from "react";
import { waitFor, render } from "@testing-library/react-native";
import { Text } from "react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import type { InteractionWire } from "@/contracts/workbench";
import type { SessionWire } from "@/contracts/auth";
import { useOverview, type OverviewState } from "@/features/workbench/useOverview";

let mockApp: Partial<AppHost>;
jest.mock("@/host/AppProvider", () => ({ useApp: () => mockApp }));

const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;

function Probe({ onState }: { onState: (s: OverviewState & { refresh: () => void }) => void }) {
  const o = useOverview();
  onState(o);
  return <Text testID="probe">{o.kind}</Text>;
}

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

// 聚合数据：2 个绑定 agent 会话 + 1 个无 agent；s1 有 pending 交互
function aggregateAppMocks() {
  return {
    visualFixture: false,
    scope: { generation: { value: 1 } } as AppHost["scope"],
    api: {
      sessions: jest.fn(async () => ({
        sessions: [session("s1", "a1"), session("s2", "a2"), session("s3", null)],
        total: 3,
      })),
      interactions: jest.fn(async (sid: string) => (sid === "s1" ? [interaction("it1", "run_001")] : [])),
      agents: jest.fn(async () => []),
    },
  } as unknown as Partial<AppHost>;
}

describe("RW-011 M03 工作台聚合（useOverview，D-04 客户端聚合）", () => {
  it("counts 聚合：running=绑定 agent 会话数、waitingUser=pending 交互数、as_of 存在", async () => {
    mockApp = aggregateAppMocks();
    let latest: (OverviewState & { refresh: () => void }) | null = null;
    await render(wrap(<Probe onState={(s) => (latest = s)} />));
    await waitFor(() => expect(latest?.kind).toBe("ready"));
    // 闭包赋值不被控制流分析追踪，经 unknown 取局部引用再收窄
    const ready = latest as unknown as Extract<OverviewState, { kind: "ready" }>;
    const data = ready.data;
    expect(data.counts.running).toBe(2);
    expect(data.counts.waitingUser).toBe(1);
    expect(data.pendingInteractions.map((p) => p.id)).toEqual(["it1"]);
    expect(typeof data.asOf).toBe("string");
  });

  it("forbidden → 遮蔽（不降级为普通错误）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        sessions: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    let latest: OverviewState | null = null;
    await render(wrap(<Probe onState={(s) => (latest = s)} />));
    await waitFor(() => expect(latest?.kind).toBe("forbidden"));
  });

  it("网络错误 → offline 状态", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        sessions: jest.fn(async () => {
          throw { kind: "network" };
        }),
      },
    } as unknown as Partial<AppHost>;
    let latest: OverviewState | null = null;
    await render(wrap(<Probe onState={(s) => (latest = s)} />));
    await waitFor(() => expect(latest?.kind).toBe("offline"));
  });
});
