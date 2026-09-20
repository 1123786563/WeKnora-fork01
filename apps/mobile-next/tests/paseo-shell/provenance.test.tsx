import React from "react";
import { Text } from "react-native";
import { render } from "@testing-library/react-native";
import { PaseoShell } from "@/paseo-shell/PaseoShell";
import { PASEO_SOURCES } from "@/paseo-shell/provenance";

describe("Paseo shell provenance", () => {
  it("records each copied Paseo module at the approved SHA and renders a Chinese route", async () => {
    expect(PASEO_SOURCES).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          upstreamSha: "d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b",
          license: "Apache-2.0",
        }),
      ]),
    );

    const view = await render(
      <PaseoShell route="tasks" onNavigate={jest.fn()}>
        <Text>九月客户回访与风险清单</Text>
      </PaseoShell>,
    );

    expect(view.getByText("九月客户回访与风险清单")).toBeTruthy();
    expect(view.getByTestId("paseo-shell-title").props.children).toBe("任务");
  });
});
