// RW-019（M11 资源）/ RW-020（M12 知识详情）/ RW-025（M17 我的）/ RW-026（M18 空间用量）
// 页面行为验收测试。断言核心语义：分类入口、forbidden 遮蔽、退出二次确认、只读用量、无支付按钮。
import React from "react";
import { fireEvent, screen, waitFor, render } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import type { AppIdentity } from "@/features/auth/AuthController";
import type { KnowledgeBaseWire, ConnectionWire, UsageSummaryWire } from "@/contracts/workbench";

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

import ResourcesScreen from "../../app/(tabs)/resources";
import KnowledgeScreen from "../../app/knowledge/[id]";
import ProfileScreen from "../../app/(tabs)/profile";
import UsageScreen from "../../app/usage";

const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;

const kbList: KnowledgeBaseWire[] = [
  { id: "kb1", name: "产品资料库", document_count: 12, status: "ready", updated_at: "2026-09-18T08:00:00Z" },
];
const connList: ConnectionWire[] = [
  { id: "c1", provider: "feishu", owner_scope: "tenant", status: "active", scopes: ["im:send"], account_label: "产品群机器人" },
];

const identity: AppIdentity = {
  userId: "u1",
  displayName: "测试用户",
  email: "user@example.com",
  memberships: [{ tenantId: "t1", tenantName: "测试空间", role: "owner", billingRole: "admin" }],
  selectedTenantId: "t1",
};

describe("RW-019 M11 资源（分类入口）", () => {
  it("知识/连接分类渲染；点知识入口导航到知识详情", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        knowledgeBases: jest.fn(async () => kbList),
        connections: jest.fn(async () => connList),
      },
    } as unknown as Partial<AppHost>;
    await render(wrap(<ResourcesScreen />));

    expect(await screen.findByText("产品资料库")).toBeDefined();
    expect(screen.getByText("feishu")).toBeDefined();
    fireEvent.press(screen.getByText("产品资料库"));
    await waitFor(() => expect(mockRouter.push).toHaveBeenCalledWith("/knowledge/kb1"));
  });
});

describe("RW-020 M12 知识详情", () => {
  beforeEach(() => {
    mockParams = { id: "kb1" };
  });

  it("渲染知识库摘要（文档数/状态）；索引进度如实展示", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: { knowledgeBases: jest.fn(async () => kbList) },
    } as unknown as Partial<AppHost>;
    await render(wrap(<KnowledgeScreen />));

    expect(await screen.findByText("产品资料库")).toBeDefined();
    expect(screen.getByText(/12/)).toBeDefined(); // document_count
  });

  it("forbidden → 遮蔽", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        knowledgeBases: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await render(wrap(<KnowledgeScreen />));
    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });
});

describe("RW-025 M17 我的", () => {
  it("用户与空间卡渲染（来自身份恢复，非本地缓存直显）", async () => {
    const logout = jest.fn(async () => undefined);
    mockApp = {
      visualFixture: false,
      identity,
      logout,
    } as unknown as Partial<AppHost>;
    await render(wrap(<ProfileScreen />));

    expect(await screen.findByText("user@example.com")).toBeDefined();
    expect(screen.getByText("测试空间")).toBeDefined();
  });

  it("退出登录须二次确认：确认前不调用 logout，Sheet 说明服务端任务继续", async () => {
    const logout = jest.fn(async () => undefined);
    mockApp = {
      visualFixture: false,
      identity,
      logout,
    } as unknown as Partial<AppHost>;
    await render(wrap(<ProfileScreen />));

    // 入口按钮与 Sheet 确认按钮同名"退出登录"：入口在前（列表项），确认在后（Sheet）
    const entry = (await screen.findAllByLabelText("退出登录"))[0]!;
    fireEvent.press(entry);
    expect(await screen.findByTestId("decision-sheet")).toBeDefined();
    expect(screen.getByText(/服务端任务继续运行/)).toBeDefined();
    expect(logout).not.toHaveBeenCalled(); // 确认前不执行
    const confirm = (await screen.findAllByLabelText("退出登录"))[1]!;
    fireEvent.press(confirm);
    await waitFor(() => expect(logout).toHaveBeenCalledTimes(1));
  });
});

describe("RW-026 M18 空间用量", () => {
  const usage: UsageSummaryWire = {
    as_of: "2026-09-18T10:00:00Z",
    period: "2026-09",
    available: 820,
    held: 30,
    settled: 150,
    pending: 0,
    unit: "credits",
  };

  it("只读展示额度/预占/已结算 + as_of；无支付/充值按钮", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: { usageSummary: jest.fn(async () => usage) },
    } as unknown as Partial<AppHost>;
    await render(wrap(<UsageScreen />));

    expect(await screen.findByText(/820/)).toBeDefined(); // 可用额度
    expect(screen.getByText(/150/)).toBeDefined(); // 已结算
    await waitFor(() => expect(screen.getByText(/数据截至/)).toBeDefined()); // as_of 展示
    // 只读：无支付/充值按钮（页面可有"不在此页提供"的如实说明文案，但无操作入口）
    const payButtons = screen
      .queryAllByRole("button")
      .filter((b) => /支付|充值|购买|付款/.test(String(b.props.accessibilityLabel ?? "")));
    expect(payButtons).toHaveLength(0);
  });

  it("无账单权限 → 遮蔽明细（不显示数字）", async () => {
    mockApp = {
      visualFixture: false,
      scope: { generation: { value: 1 } } as AppHost["scope"],
      api: {
        usageSummary: jest.fn(async () => {
          throw { kind: "forbidden" };
        }),
      },
    } as unknown as Partial<AppHost>;
    await render(wrap(<UsageScreen />));

    expect(await screen.findByText("账单权限不足，无法查看明细")).toBeDefined();
    expect(screen.getByTestId("state-forbidden")).toBeDefined();
  });
});
