import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { createPaseoShell } from "@/paseo-shell/createPaseoShell";
import { ThemeProvider } from "@/theme/ThemeProvider";

jest.mock("expo-router", () => ({
  Slot: () => null,
}));

describe("createPaseoShell", () => {
  it("delegates a tab selection to the supplied router callback", async () => {
    const navigate = jest.fn();
    const view = await render(
      <ThemeProvider initialPreference="light">{createPaseoShell({ initialRoute: "home", navigate })}</ThemeProvider>,
    );

    fireEvent.press(view.getByRole("button", { name: "任务" }));

    expect(navigate).toHaveBeenCalledWith("tasks");
  });
});
