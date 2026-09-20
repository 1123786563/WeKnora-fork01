import { Slot } from "expo-router";
import type { ReactElement } from "react";
import { PaseoShell, type ShellRoute } from "./PaseoShell";

export interface ShellFactoryDeps {
  initialRoute: ShellRoute;
  navigate(route: ShellRoute): void;
}

export function createPaseoShell({ initialRoute, navigate }: ShellFactoryDeps): ReactElement {
  return (
    <PaseoShell route={initialRoute} onNavigate={navigate}>
      <Slot />
    </PaseoShell>
  );
}
