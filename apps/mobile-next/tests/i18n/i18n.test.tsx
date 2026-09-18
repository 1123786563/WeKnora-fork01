import { t, zh, type I18nKey } from "@/i18n";
import { I18nProvider, useI18n } from "@/i18n";
import React from "react";
import { Text } from "react-native";
import { render, screen } from "@testing-library/react-native";

describe("RW-029 t() 文案表", () => {
  it("通用页面状态文案与设计规格固定文案逐条一致（02 规格 §2）", () => {
    expect(t("state.loading")).toBe("正在同步当前空间");
    expect(t("state.stale")).toBe("当前显示上次同步结果");
    expect(t("state.offline")).toBe("当前离线，草稿已保留");
    expect(t("state.empty.task")).toBe("这里还没有任务");
    expect(t("state.error")).toBe("暂时无法连接，稍后重试");
    expect(t("state.forbidden")).toBe("无法访问这项资源");
    expect(t("state.conflict")).toBe("内容已在另一端改变，请刷新");
    expect(t("state.uncertain")).toBe("正在核实原请求，请勿重复提交");
  });

  it("核心页面关键文案在表中（M01/M02/M03/M05）", () => {
    expect(t("login.title")).toContain("让想法开始");
    expect(t("home.heroTitle")).toContain("把时间留给自己");
    expect(t("spaces.notice")).toContain("不会停止已提交的任务");
    expect(t("task.attach.verifying")).toContain("通过前不能发送");
    expect(t("run.cancel.impact")).toContain("不等于退款");
  });

  it("参数插值与缺失键回退（缺失返回 key 本身，可发现）", () => {
    // 现有键插值示例（表内暂无占位符键，用运行时行为验证）
    expect(t("state.empty.task", {})).toBe("这里还没有任务");
    expect(t("not.a.real.key" as I18nKey)).toBe("not.a.real.key");
  });

  it("文案表键唯一且非空", () => {
    const entries = Object.entries(zh);
    expect(entries.length).toBeGreaterThan(60);
    for (const [k, v] of entries) {
      expect(typeof v).toBe("string");
      expect(v.length).toBeGreaterThan(0);
      expect(k).toMatch(/^[a-z][a-zA-Z0-9.]*$/);
    }
  });
});

describe("RW-029 I18nProvider", () => {
  it("Provider 内 useI18n 取 zh 文案；未包 Provider 时回退默认", async () => {
    const Probe = () => {
      const { t } = useI18n();
      return <Text testID="probe">{t("action.retry")}</Text>;
    };
    await render(
      <I18nProvider>
        <Probe />
      </I18nProvider>,
    );
    expect(screen.getByTestId("probe").props.children).toBe("重试");

    const Bare = () => {
      const { locale, t } = useI18n();
      return (
        <Text testID="bare">
          {locale}:{t("action.retry")}
        </Text>
      );
    };
    await render(<Bare />);
    expect(screen.getByTestId("bare").props.children.join("")).toBe("zh:重试");
  });
});
