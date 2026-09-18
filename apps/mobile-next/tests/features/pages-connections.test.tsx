// RW-021（M13 连接详情）页面行为验收测试（不依赖真实 OAuth 服务器的可执行部分）。
// 断言核心语义：Provider/归属/范围/状态渲染、重新授权走系统浏览器（URL 由后端 origin 下发路径）、
// 撤销二次确认且不假装成功、移动端不展示 token/secret、写入类 scope 标注需审批。
// 真机浏览器回跳实测仍为 blocked-env（记录于 progress.md）。
import React from "react";
import { fireEvent, screen, waitFor, render } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import type { AppHost } from "@/host/AppProvider";
import type { ConnectionWire } from "@/contracts/workbench";

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

const mockOpenBrowserAsync = jest.fn(async () => ({ type: "dismiss" } as const));
jest.mock("expo-web-browser", () => ({
  // 惰性 getter：工厂在 import 提升阶段执行，直接展开 mock 变量会得到 undefined
  get openBrowserAsync() {
    return mockOpenBrowserAsync;
  },
}));

let mockApp: Partial<AppHost>;
jest.mock("@/host/AppProvider", () => ({
  useApp: () => mockApp,
}));

import ConnectionDetailScreen from "../../app/connections/[id]";

const wrap = (ui: React.ReactElement) => <ThemeProvider initialPreference="light">{ui}</ThemeProvider>;

const connList: ConnectionWire[] = [
  {
    id: "c1",
    provider: "飞书连接",
    owner_scope: "tenant",
    status: "active",
    scopes: ["read:wiki", "write:wiki"],
    account_label: "project-bot",
  },
];

function appWithConnection(overrides: Record<string, unknown> = {}) {
  return {
    visualFixture: false,
    origin: "https://weknora.example.com",
    scope: { generation: { value: 1 } } as AppHost["scope"],
    api: {
      connections: jest.fn(async () => connList),
      ...overrides,
    },
  } as unknown as Partial<AppHost>;
}

describe("RW-021 M13 连接详情", () => {
  beforeEach(() => {
    mockParams = { id: "c1" };
    mockOpenBrowserAsync.mockClear();
  });

  it("渲染 Provider/授权账号/归属/状态/授权范围；动作按范围推导（写入需审批）", async () => {
    mockApp = appWithConnection();
    await render(wrap(<ConnectionDetailScreen />));

    expect(await screen.findByText("飞书连接")).toBeDefined();
    expect(screen.getByText("授权账号 project-bot")).toBeDefined();
    expect(screen.getByText("空间连接")).toBeDefined(); // owner_scope=tenant
    expect(screen.getByText("可用")).toBeDefined(); // status=active
    expect(screen.getByText("授权范围")).toBeDefined();
    expect(screen.getAllByText("read:wiki").length).toBeGreaterThan(0); // 授权范围 + 动作推导两处
    // 写入类 scope → 需审批；读取类 → 已授权
    expect(screen.getByText("需审批")).toBeDefined();
    expect(screen.getByText("已授权")).toBeDefined();
  });

  it("页面不展示 token/secret（凭据服务端受控保管）", async () => {
    mockApp = appWithConnection();
    await render(wrap(<ConnectionDetailScreen />));

    expect(await screen.findByText("飞书连接")).toBeDefined();
    expect(screen.getByText("凭据管理")).toBeDefined();
    expect(screen.getByText("服务端受控保管")).toBeDefined();
    expect(screen.queryByText(/access_token|api[_-]?key|secret/i)).toBeNull();
  });

  it("重新授权走系统浏览器，打开部署 origin 的授权路径", async () => {
    mockApp = appWithConnection();
    await render(wrap(<ConnectionDetailScreen />));

    fireEvent.press(await screen.findByLabelText("重新授权"));
    // 不假装授权完成：给出如实提示（证明 reauthorize 执行）
    expect(await screen.findByText("需要连接部署的授权服务", undefined, { timeout: 3000 })).toBeDefined();
    await waitFor(() =>
      expect(mockOpenBrowserAsync).toHaveBeenCalledWith("https://weknora.example.com/api/v1/apps/connections/c1/authorize"),
    );
  });

  it("撤销须二次确认：Sheet 说明不回滚外部操作；服务端无端点时如实提示不假装成功", async () => {
    mockApp = appWithConnection();
    await render(wrap(<ConnectionDetailScreen />));

    fireEvent.press(await screen.findByLabelText("撤销"));
    expect(await screen.findByTestId("decision-sheet")).toBeDefined();
    expect(screen.getAllByText(/撤销不会回滚外部已执行的操作/).length).toBeGreaterThan(0); // 页面警示 + Sheet impact
    expect(screen.getByText("连接：飞书连接")).toBeDefined(); // 冻结摘要绑定目标
    fireEvent.press(await screen.findByLabelText("确认撤销"));
    expect(await screen.findByText("撤销请求需要部署的连接管理服务（能力待接入）")).toBeDefined();
  });

  it("forbidden → 遮蔽", async () => {
    mockParams = { id: "c1" };
    mockApp = appWithConnection({
      connections: jest.fn(async () => {
        throw { kind: "forbidden" };
      }),
    });
    await render(wrap(<ConnectionDetailScreen />));
    expect(await screen.findByTestId("state-forbidden")).toBeDefined();
  });

  it("列表中不存在该连接 → 空态（不泄漏存在性）", async () => {
    mockParams = { id: "c1" };
    mockApp = appWithConnection({
      connections: jest.fn(async () => [{ ...connList[0]!, id: "other" }]),
    });
    await render(wrap(<ConnectionDetailScreen />));
    expect(await screen.findByText("连接不存在")).toBeDefined();
  });
});
