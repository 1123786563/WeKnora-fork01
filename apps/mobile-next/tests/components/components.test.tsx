import React from "react";
import { render, screen, fireEvent } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { ActionButton } from "@/components/ActionButton";
import { FormField } from "@/components/FormField";
import { StatusBadge } from "@/components/StatusBadge";
import { OfflineNotice } from "@/components/OfflineNotice";
import { TaskCard } from "@/components/TaskCard";
import { PendingCard } from "@/components/PendingCard";
import { ArtifactCard } from "@/components/ArtifactCard";
import { DecisionSheet } from "@/components/DecisionSheet";
import { RunStatusStack } from "@/components/RunStatusStack";

const wrap = (ui: React.ReactElement, mode: "light" | "dark" = "light") => (
  <ThemeProvider initialPreference={mode}>{ui}</ThemeProvider>
);

describe("RW-003 ActionButton", () => {
  it("渲染 label 并响应点击", async () => {
    const onPress = jest.fn();
    await render(wrap(<ActionButton label="登录" onPress={onPress} testID="btn" />));
    fireEvent.press(screen.getByTestId("btn"));
    expect(onPress).toHaveBeenCalledTimes(1);
    expect(screen.getByText("登录")).toBeDefined();
  });

  it("disabled / busy 时不响应点击", async () => {
    const onPress = jest.fn();
    const { rerender } = await render(
      wrap(<ActionButton label="提交" onPress={onPress} disabled testID="btn" />),
    );
    fireEvent.press(screen.getByTestId("btn"));
    expect(onPress).not.toHaveBeenCalled();
    rerender(wrap(<ActionButton label="提交" onPress={onPress} busy testID="btn" />));
    fireEvent.press(screen.getByTestId("btn"));
    expect(onPress).not.toHaveBeenCalled();
  });

  it("busy 状态读屏可感知", async () => {
    await render(wrap(<ActionButton label="提交" onPress={() => {}} busy testID="btn" />));
    expect(screen.getByTestId("btn").props.accessibilityState).toMatchObject({ busy: true, disabled: true });
  });
});

describe("RW-003 FormField", () => {
  it("显示 label 与错误文字（错误不用颜色单独承担）", async () => {
    await render(
      wrap(<FormField label="邮箱" value="a@b.c" onChangeText={() => {}} error="邮箱格式不正确" testID="f" />),
    );
    expect(screen.getByText("邮箱")).toBeDefined();
    expect(screen.getByText("邮箱格式不正确")).toBeDefined();
  });

  it("required 标记与 hint", async () => {
    await render(
      wrap(<FormField label="任务描述" value="" onChangeText={() => {}} required hint="不少于 10 个字" />),
    );
    expect(screen.getByLabelText("必填")).toBeDefined();
    expect(screen.getByText("不少于 10 个字")).toBeDefined();
  });
});

describe("RW-003 StatusBadge / RunStatusStack", () => {
  it("unknown 徽章显示等待同步", async () => {
    await render(wrap(<StatusBadge domain="run" label="等待同步" tone="success" unknown />));
    expect(screen.getByTestId("badge-run")).toBeDefined();
    expect(screen.getByText("等待同步")).toBeDefined();
  });

  it("三状态独立展示 + 最近同步时间", async () => {
    await render(
      wrap(
        <RunStatusStack
          items={[
            { key: "run", title: "任务状态", rawValue: "cancelled", label: "已申请取消", tone: "attention", unknown: false },
            { key: "execution", title: "执行观察", rawValue: "unknown", label: "等待同步", tone: "unknown", unknown: true },
            { key: "settlement", title: "结算状态", rawValue: "pending", label: "待对账", tone: "neutral", unknown: false },
          ]}
          lastSyncAt="12:03"
        />,
      ),
    );
    expect(screen.getByText("已申请取消")).toBeDefined();
    expect(screen.getByText("等待同步")).toBeDefined();
    expect(screen.getByText("待对账")).toBeDefined();
    expect(screen.getByText(/最近同步 12:03/)).toBeDefined();
  });
});

describe("RW-003 卡片组件", () => {
  it("TaskCard 展示标题/Agent/状态并响应点击", async () => {
    const onPress = jest.fn();
    await render(
      wrap(
        <TaskCard
          title="整理本周会议纪要"
          agentName="研究助理"
          runBadge={{ domain: "run", label: "进行中", tone: "progress" }}
          meta="更新于 5 分钟前"
          onPress={onPress}
          testID="task"
        />,
      ),
    );
    fireEvent.press(screen.getByTestId("task"));
    expect(onPress).toHaveBeenCalledTimes(1);
    expect(screen.getByText("研究助理")).toBeDefined();
  });

  it("TaskCard stale 态显示上次同步提示", async () => {
    await render(
      wrap(
        <TaskCard
          title="t"
          runBadge={{ domain: "run", label: "进行中", tone: "progress" }}
          meta="更新于 1 小时前"
          stale
          onPress={() => {}}
        />,
      ),
    );
    expect(screen.getByText(/当前显示上次同步结果/)).toBeDefined();
  });

  it("PendingCard 摘要展示待处理类型", async () => {
    await render(wrap(<PendingCard kindLabel="工具批准" title="向飞书发送消息" onPress={() => {}} />));
    expect(screen.getByText(/需要你处理 · 工具批准/)).toBeDefined();
  });

  it("ArtifactCard 展示不可变版本与来源", async () => {
    await render(
      wrap(
        <ArtifactCard name="纪要.md" mime="text/markdown" version="3" sizeLabel="12 KB" sourceRun="run_9" onPress={() => {}} />,
      ),
    );
    expect(screen.getByText(/v3 · 12 KB · 来源任务 run_9/)).toBeDefined();
  });
});

describe("RW-003 DecisionSheet", () => {
  const props = {
    visible: true,
    title: "批准这次操作",
    impact: "允许 Agent 向飞书群发送一条消息",
    frozenSummary: ["目标：飞书 · 产品群", "内容摘要：约 120 字", "版本：rev 4", "有效期：本次任务"],
    confirm: { label: "批准这一次", variant: "primary" as const, onPress: jest.fn() },
    dismiss: { label: "拒绝", variant: "secondary" as const, onPress: jest.fn() },
  };
  it("展示冻结对象与两侧操作", async () => {
    await render(wrap(<DecisionSheet {...props} />));
    expect(screen.getByText("批准这次操作")).toBeDefined();
    expect(screen.getByText(/目标：飞书 · 产品群/)).toBeDefined();
    expect(screen.getByText("批准这一次")).toBeDefined();
    expect(screen.getByText("拒绝")).toBeDefined();
  });

  it("stale（revision 变化）时提示刷新", async () => {
    await render(wrap(<DecisionSheet {...props} stale />));
    expect(screen.getByText(/内容已在另一端改变，请刷新后再决定/)).toBeDefined();
  });
});

describe("RW-003 OfflineNotice", () => {
  it("固定文案", async () => {
    await render(wrap(<OfflineNotice />));
    expect(screen.getByText("当前离线，草稿已保留")).toBeDefined();
  });
});
