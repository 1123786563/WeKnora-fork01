// RW-016（M08 执行详情）+ RW-017（M09 审批）页面行为验收测试。
// 断言验收条件的核心语义：三状态独立展示、unknown 不冒充完成、取消携带 revision、
// 409 冲突禁用且不换 ID 重发、DecisionSheet 冻结摘要、forbidden 遮蔽。
import React from "react";
import { fireEvent, screen, waitFor } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import type { InteractionWire, RunViewWire } from "@/contracts/workbench";

// ---- 页面依赖 mock（jest 工厂引用需 mock 前缀变量）----
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

let mockApp: Partial<AppHost>;
jest.mock("@/host/AppProvider", () => ({
  useApp: () => mockApp,
}));

const renderPage = (ui: React.ReactElement) => render(wrap(ui));
const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;
import { render } from "@testing-library/react-native";

const runFixture: RunViewWire = {
  run_id: "run_001",
  session_id: "s1",
  status: "running",
  revision: 3,
  budget_upper: 100,
  spent: 38,
  as_of: "2026-09-18T10:00:00Z",
};

const interactionFixture: InteractionWire = {
  id: "it1",
  run_id: "run_001",
  kind: "tool_approval",
  status: "pending",
  revision: 3,
  title: "发布客户反馈摘要",
  summary: "将本周反馈摘要发布到指定频道",
  payload: { target: "#product-team", risk: "外部写入" },
  created_at: "2026-09-18T09:00:00Z",
};

import ExecutionScreen from "../../app/executions/[runId]";
import ApprovalScreen from "../../app/interactions/[interactionId]";

describe("RW-016 M08 执行详情", () => {
  beforeEach(() => {
    mockParams = { runId: "run_001" };
    mockRouter.back.mockReset();
  });

  it("三状态行独立展示；执行观察/结算 unknown 不冒充完成", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        getExecution: jest.fn(async () => runFixture),
        snapshot: jest.fn(async () => {
          throw new Error("snapshot 不可用");
        }),
        sessions: jest.fn(async () => ({ sessions: [], total: 0 })),
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ExecutionScreen />);

    expect(await screen.findByText("产品任务")).toBeDefined();
    expect(screen.getAllByText("进行中").length).toBeGreaterThan(0); // run_status=running
    expect(screen.getByText("执行观察")).toBeDefined();
    expect(screen.getByText("等待同步")).toBeDefined(); // execution unknown → 等待同步（非完成）
    expect(screen.getByText("费用结算")).toBeDefined();
    expect(screen.getByText("待对账 · 非最终消耗")).toBeDefined(); // settlement 非最终
    expect(screen.getByText("最后同步")).toBeDefined(); // 最近同步/观察时间展示
  });

  it("取消申请携带当前 revision；成功后关闭 Sheet", async () => {
    const command = jest.fn(async () => ({ ...runFixture, status: "cancelling" }));
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        getExecution: jest.fn(async () => runFixture),
        snapshot: jest.fn(async () => {
          throw new Error("不可用");
        }),
        sessions: jest.fn(async () => ({ sessions: [], total: 0 })),
        command,
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ExecutionScreen />);

    fireEvent.press(await screen.findByTestId("cancel-run-button"));
    expect(await screen.findByTestId("cancel-decision-sheet")).toBeDefined();
    // 冻结摘要含执行 ID 与 revision
    expect(screen.getByText("执行 ID：run_001")).toBeDefined();
    expect(screen.getByText("revision 3")).toBeDefined();
    fireEvent.press(await screen.findByTestId("confirm-cancel"));
    await waitFor(() => expect(command).toHaveBeenCalledWith("run_001", "cancel", 3));
    await waitFor(() => expect(screen.queryByTestId("cancel-decision-sheet")).toBeNull());
  });

  it("取消 409 → 冲突提示（已在另一端处理），不自动重发", async () => {
    const command = jest.fn(async () => {
      throw { kind: "conflict", message: "cursor expired" };
    });
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        getExecution: jest.fn(async () => runFixture),
        snapshot: jest.fn(async () => {
          throw new Error("不可用");
        }),
        sessions: jest.fn(async () => ({ sessions: [], total: 0 })),
        command,
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ExecutionScreen />);

    fireEvent.press(await screen.findByTestId("cancel-run-button"));
    fireEvent.press(await screen.findByTestId("confirm-cancel"));
    expect(await screen.findByText("已在另一端处理")).toBeDefined();
    expect(command).toHaveBeenCalledTimes(1);
  });

  it("revision 缺失 → 取消禁用并提示（unknown 不提供可执行控制）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        getExecution: jest.fn(async () => ({ ...runFixture, revision: null })),
        snapshot: jest.fn(async () => {
          throw new Error("不可用");
        }),
        sessions: jest.fn(async () => ({ sessions: [], total: 0 })),
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ExecutionScreen />);

    const btn = await screen.findByTestId("cancel-run-button");
    expect(btn.props.accessibilityState).toMatchObject({ disabled: true });
    expect(screen.getByText("缺少版本信息，暂不能取消；下拉或稍后刷新重试。")).toBeDefined();
  });

  it("forbidden → 遮蔽敏感正文（state-forbidden）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        getExecution: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ExecutionScreen />);

    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });
});

describe("RW-017 M09 审批", () => {
  beforeEach(() => {
    mockParams = { interactionId: "it1", runId: "run_001" };
  });

  it("渲染待处理交互：类型/目标/revision 展示（能力与 revision 来自后端）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        interactions: jest.fn(async () => [interactionFixture, { ...interactionFixture, id: "it2", status: "resolved" }]),
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ApprovalScreen />);

    expect(await screen.findByText("发布客户反馈摘要")).toBeDefined();
    expect(screen.getAllByText("revision 3").length).toBeGreaterThan(0);
    expect(screen.getByText("#product-team")).toBeDefined(); // payload.target
    expect(screen.getByText("待决定")).toBeDefined();
  });

  it("批准：Sheet 冻结摘要（目标/内容摘要/revision）→ decide 携带 expected_revision", async () => {
    const decide = jest.fn(async () => ({ ...interactionFixture, status: "resolved" }));
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        interactions: jest.fn(async () => [interactionFixture]),
        decide,
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ApprovalScreen />);

    fireEvent.press(await screen.findByTestId("approve-button"));
    expect(await screen.findByTestId("approval-decision-sheet")).toBeDefined();
    expect(screen.getByText("目标：#product-team")).toBeDefined();
    expect(screen.getByText("内容摘要：将本周反馈摘要发布到指定频道")).toBeDefined();
    expect(screen.getAllByText("revision 3").length).toBeGreaterThan(0);
    fireEvent.press(await screen.findByTestId("confirm-decide"));
    await waitFor(() =>
      expect(decide).toHaveBeenCalledWith("it1", {
        pending_id: "it1",
        expected_revision: 3,
        action: "approve",
      }),
    );
  });

  it("409 → 版本已变化标记、确认禁用、不换 ID 重发", async () => {
    const decide = jest.fn(async () => {
      throw { kind: "conflict" };
    });
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        interactions: jest.fn(async () => [interactionFixture]),
        decide,
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ApprovalScreen />);

    fireEvent.press(await screen.findByTestId("approve-button"));
    fireEvent.press(await screen.findByTestId("confirm-decide"));
    // 冲突后页面给出"版本已变化"徽标 + 刷新提示
    expect(await screen.findByText("版本已变化")).toBeDefined();
    expect(screen.getByText("内容已在另一端改变，请刷新。")).toBeDefined();
    // 按钮禁用（冲突期间不可再提交）
    expect(screen.getByTestId("approve-button").props.accessibilityState).toMatchObject({ disabled: true });
    expect(decide).toHaveBeenCalledTimes(1);
  });

  it("forbidden → 遮蔽（不展示交互正文）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        interactions: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ApprovalScreen />);

    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });

  it("列表中不存在该交互 → notfound 空态（可能已在别处处理）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        interactions: jest.fn(async () => [{ ...interactionFixture, id: "other" }]),
      },
    } as unknown as Partial<AppHost>;
    await renderPage(<ApprovalScreen />);

    expect(await screen.findByText("没有找到这条待处理")).toBeDefined();
  });
});
