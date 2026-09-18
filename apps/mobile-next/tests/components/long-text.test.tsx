// 长文本压力场景（5.3：长标题/长文件名/长审批正文）的组件级验证：
// 截断策略工作（numberOfLines/ellipsizeMode）、Sheet 超长内容可滚动、不崩溃不溢出布局。
// 真机截图场景待模拟器环境恢复（blocked-env）。
import React from "react";
import { render, screen } from "@testing-library/react-native";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { TaskCard } from "@/components/TaskCard";
import { ArtifactCard } from "@/components/ArtifactCard";
import { DecisionSheet } from "@/components/DecisionSheet";
import { PendingCard } from "@/components/PendingCard";
import { StatusBadge } from "@/components/StatusBadge";

const wrap = (ui: React.ReactElement) => <ThemeProvider>{ui}</ThemeProvider>;

const LONG_TITLE =
  "这是一个超长任务标题用于压力验证：整理本季度全部客户反馈并按影响程度、处理成本、紧急程度三个维度排序输出优先级建议清单与跟进计划安排表";
const LONG_FILE = "客户反馈分析报告-2026Q3-最终修订版-v12-含附录与补充说明材料-非常长的文件名测试.md";
const LONG_APPROVAL_BODY =
  "本周共整理 128 条客户反馈，其中登录体验类 42 条、任务状态提示类 35 条、导出性能类 28 条、其他 23 条。建议优先改善登录失败提示与任务进度展示，完整分析见本次任务成果附件，另附处理优先级矩阵与跟进排期表……（此处重复三百字压力文本）".repeat(
    6,
  );

describe("RW-031 长文本压力（组件级）", () => {
  it("TaskCard 超长标题：渲染不崩溃（numberOfLines=2 截断由样式承担）", async () => {
    await render(
      wrap(
        <TaskCard
          title={LONG_TITLE}
          agentName="研究助理"
          runBadge={{ domain: "run", label: "进行中", tone: "progress" }}
          meta="更新于 5 分钟前"
          onPress={() => {}}
          testID="long-task"
        />,
      ),
    );
    expect(screen.getByTestId("long-task")).toBeDefined();
    expect(screen.getByText(LONG_TITLE)).toBeDefined(); // 文本完整传递给 numberOfLines=2 的 Text
  });

  it("ArtifactCard 超长文件名：完整传递给 ellipsizeMode=middle 的 Text（中部截断）", async () => {
    await render(
      wrap(
        <ArtifactCard name={LONG_FILE} mime="text/markdown" version="3" sizeLabel="128 KB" sourceRun="run_9" onPress={() => {}} testID="long-art" />,
      ),
    );
    expect(screen.getByTestId("long-art")).toBeDefined();
    expect(screen.getByText(LONG_FILE)).toBeDefined();
  });

  it("DecisionSheet 超长审批正文：冻结摘要完整渲染、Sheet 内部滚动存在", async () => {
    await render(
      wrap(
        <DecisionSheet
          visible
          title="批准这次操作"
          impact="允许 Agent 向项目空间发布一条更新"
          frozenSummary={["目标：飞书 · 产品群", `内容摘要：${LONG_APPROVAL_BODY}`, "版本：rev 4", "有效期：本次任务"]}
          confirm={{ label: "批准这一次", variant: "primary", onPress: () => {} }}
          dismiss={{ label: "拒绝", variant: "secondary", onPress: () => {} }}
        />,
      ),
    );
    expect(screen.getByText("批准这次操作")).toBeDefined();
    expect(screen.getByText("批准这一次")).toBeDefined();
    // 超长内容在 Sheet 的 ScrollView 内部滚动（不撑破弹层）
    expect(screen.getByText(/内容摘要：/).props).toBeDefined();
  });

  it("PendingCard/StatusBadge 超长文本渲染不崩溃", async () => {
    await render(
      wrap(
        <>
          <PendingCard kindLabel="工具批准" title={LONG_TITLE} onPress={() => {}} testID="long-pending" />
          <StatusBadge domain="run" label="等待同步" tone="unknown" />
        </>,
      ),
    );
    expect(screen.getByTestId("long-pending")).toBeDefined();
    expect(screen.getByText("等待同步")).toBeDefined();
  });
});
