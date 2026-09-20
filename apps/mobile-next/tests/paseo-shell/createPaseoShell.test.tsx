import React from "react";
import { fireEvent, render } from "@testing-library/react-native";
import { createPaseoShell } from "@/paseo-shell/createPaseoShell";

jest.mock("expo-router", () => ({
  Slot: () => null,
}));

describe("createPaseoShell", () => {
  it("delegates a tab selection to the supplied router callback", async () => {
    const navigate = jest.fn();
    const view = await render(createPaseoShell({ initialRoute: "home", navigate }));

    fireEvent.press(view.getByRole("button", { name: "任务" }));

    expect(navigate).toHaveBeenCalledWith("tasks");
  });
});
