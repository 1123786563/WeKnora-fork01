import React from "react";
import { StyleSheet, Text } from "react-native";
import { render } from "@testing-library/react-native";
import { PaseoShell } from "@/paseo-shell/PaseoShell";
import { PASEO_SOURCES } from "@/paseo-shell/provenance";
import { ThemeProvider } from "@/theme/ThemeProvider";
import { getTheme } from "@/theme/theme";

describe("Paseo shell provenance", () => {
  it("records each copied Paseo module at the approved SHA and renders a Chinese route", async () => {
    expect(PASEO_SOURCES).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          upstreamPath: "packages/app/src/components/headers/screen-title.tsx",
          upstreamSha: "d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b",
          license: "Apache-2.0",
          localChanges: "Replaced react-native-unistyles theme access with the app runtime theme and React Native StyleSheet.",
        }),
      ]),
    );

    const view = await render(
      <ThemeProvider initialPreference="light">
        <PaseoShell route="tasks" onNavigate={jest.fn()}>
          <Text>九月客户回访与风险清单</Text>
        </PaseoShell>
      </ThemeProvider>,
    );

    expect(view.getByText("九月客户回访与风险清单")).toBeTruthy();
    expect(view.getByTestId("paseo-shell-title").props.children).toBe("任务");
  });

  it.each(["light", "dark"] as const)("uses the %s runtime theme for shell and title colors", async (mode) => {
    const view = await render(
      <ThemeProvider initialPreference={mode}>
        <PaseoShell route="tasks" onNavigate={jest.fn()}>
          <Text>内容</Text>
        </PaseoShell>
      </ThemeProvider>,
    );
    const theme = getTheme(mode);

    expect(StyleSheet.flatten(view.getByTestId("paseo-shell").props.style)).toMatchObject({
      backgroundColor: theme.c.canvas,
    });
    expect(StyleSheet.flatten(view.getByTestId("paseo-shell-title").props.style)).toMatchObject({
      color: theme.c.ink,
    });
    expect(StyleSheet.flatten(view.getByTestId("paseo-tab-tasks").props.style)).toMatchObject({
      backgroundColor: theme.c["brand-soft"],
    });
  });
});
