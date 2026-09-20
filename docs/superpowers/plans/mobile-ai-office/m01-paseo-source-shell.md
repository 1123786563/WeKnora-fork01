# M01 Paseo Source-Controlled Shell Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans task-by-task. Steps use checkbox syntax for tracking.

**Goal:** bring a traceable mobile-only Paseo visual shell into `apps/mobile-next` without adopting its daemon, cloud protocol, or authority.

**Architecture:** Copy only identified Apache-2.0 Paseo mobile presentation modules into an app-owned `paseo-shell` boundary, retaining upstream path, SHA, license and local changes. `CloudWorkspaceClient` remains the only data boundary; copied presentation code never calls Paseo services. The controller owns root-layout mounting, package/export/lockfile changes, and any migration.

**Tech Stack:** Expo 55, React Native 0.83, TypeScript, Jest, Paseo at `d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b`.

**Spec:** `../../specs/2026-09-20-mobile-ai-office-spec.md`; `../../specs/2026-09-20-mobile-ai-office-design.md`.

## Global Constraints

- Paseo is a mobile UI/adaptation base only; do not install or invoke its daemon or protocol.
- Do not relabel the existing Expo prototype as Paseo; record each imported upstream source and SHA.
- Keep Paseo Web buildable; WeKnora Web remains the complete Web client.
- React Native must not depend on DOM UI or Web Tailwind packages.
- Root layout, router DI, package exports and lockfiles are controller-owned integration files.

## Review Focus

- An upstream module importing daemon state must be excluded rather than stubbed into a hidden second authority.
- A copied component must display its retained upstream attribution and work with a long Chinese title.
- Offline/loading/error shell states must not imply a Run has started or completed.

---

### Task 1: Establish the attributable Paseo presentation boundary

**Files:**
- Create: `apps/mobile-next/src/paseo-shell/NOTICE.md`
- Create: `apps/mobile-next/src/paseo-shell/provenance.ts`
- Create: `apps/mobile-next/src/paseo-shell/PaseoScreenTitle.tsx`
- Create: `apps/mobile-next/src/paseo-shell/PaseoShell.tsx`
- Test: `apps/mobile-next/tests/paseo-shell/provenance.test.ts`

**Interfaces:**
- Consumes: Paseo `packages/app/src/components/headers/screen-title.tsx` at approved SHA; `ShellRoute = "home" | "tasks" | "agents" | "me"`.
- Produces: `PaseoShell(props: { route: ShellRoute; children: React.ReactNode; onNavigate(route: ShellRoute): void }): React.ReactElement` and `PASEO_SOURCES: readonly PaseoSource[]`.
- New types: `PaseoSource = { upstreamPath: string; upstreamSha: string; license: "Apache-2.0"; copiedAt: string; localChanges: string }`.

- [ ] **Step 1: Write the failing provenance and visual-shell test.**

```ts
it("records each copied Paseo module at the approved SHA and renders a Chinese route", () => {
  expect(PASEO_SOURCES).toEqual(expect.arrayContaining([
    expect.objectContaining({ upstreamSha: "d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b", license: "Apache-2.0" }),
  ]));
  const view = render(<PaseoShell route="tasks" onNavigate={jest.fn()}><Text>九月客户回访与风险清单</Text></PaseoShell>);
  expect(view.getByText("九月客户回访与风险清单")).toBeTruthy();
});
```

- [ ] **Step 2: Run the RED test.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-shell/provenance.test.ts`

Expected: FAIL because the shell and provenance exports do not yet exist.

- [ ] **Step 3: Inspect the source before copying it.**

Run: `git -C /Users/wuyongjun/trea/paseo show d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b:packages/app/src/components/headers/screen-title.tsx`; inspect its one-hop imports before copying.

- [ ] **Step 4: Copy the smallest presentation-only modules and write attribution.**

```ts
export const PASEO_SOURCES = [{
  upstreamPath: "packages/app/src/components/headers/screen-title.tsx",
  upstreamSha: "d636abd7a4ce302e7ccb9eb6074f637c6dd4d83b",
  license: "Apache-2.0",
  copiedAt: "2026-09-20",
  localChanges: "Replaced Paseo navigation state with supplied route callback.",
}] as const;
```

Copy that source to `PaseoScreenTitle.tsx`, preserving its `children`, `numberOfLines`, `testID` and `style` interface. Replace only `react-native-unistyles` theme access with `apps/mobile-next/src/theme/tokens.generated.ts` and React Native `StyleSheet`; do not add Unistyles or Paseo packages. `NOTICE.md` names this exact path and adaptation.

- [ ] **Step 5: Implement the inert shell boundary.**

```tsx
export function PaseoShell({ route, children, onNavigate }: Props) {
  return <SafeAreaView><PaseoScreenTitle testID="paseo-shell-title">{routeLabel(route)}</PaseoScreenTitle>{children}</SafeAreaView>;
}
```

Implement the four tab buttons locally as `PaseoTabBar` only after the copied title is mounted; it accepts `{ active: ShellRoute; onSelect(route: ShellRoute): void }`, has no fetch/persistence/session behavior, and uses existing tokens.

- [ ] **Step 6: Run GREEN and static checks.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-shell/provenance.test.ts && npm --prefix apps/mobile-next run typecheck && npm --prefix apps/mobile-next run check:isolation`

Expected: the provenance test, typecheck and isolation check pass.

- [ ] **Step 7: Review and evidence gate.**

Inspect package imports and `NOTICE.md`; capture an Expo iOS and Android narrow-screen shell proof after controller mounting. Record fixture/unit results separately from devices; do not claim device acceptance from Jest.

### Task 2: Hand off a controller-mountable constructor

**Files:**
- Create: `apps/mobile-next/src/paseo-shell/createPaseoShell.tsx`
- Test: `apps/mobile-next/tests/paseo-shell/createPaseoShell.test.tsx`
- Modify (controller integration only): `apps/mobile-next/app/_layout.tsx`

**Interfaces:**
- Consumes: `PaseoShellProps` from Task 1.
- Produces: `createPaseoShell(deps: { initialRoute: ShellRoute; navigate(route: ShellRoute): void }): React.ReactElement`.
- Integration note: controller mounts this constructor in RootLayout after M02 route and scope DI are stable.

- [ ] **Step 1: Write the failing constructor test.**

```tsx
it("delegates a tab selection to the supplied router callback", () => {
  const navigate = jest.fn();
  const view = render(createPaseoShell({ initialRoute: "home", navigate }));
  fireEvent.press(view.getByRole("button", { name: "任务" }));
  expect(navigate).toHaveBeenCalledWith("tasks");
});
```

- [ ] **Step 2: Run RED.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-shell/createPaseoShell.test.tsx`

Expected: FAIL because `createPaseoShell` is absent.

- [ ] **Step 3: Implement the constructor, without editing controller-owned integration.**

```tsx
export function createPaseoShell({ initialRoute, navigate }: ShellFactoryDeps) {
  return <PaseoShell route={initialRoute} onNavigate={navigate}><Slot /></PaseoShell>;
}
```

- [ ] **Step 4: Run GREEN.**

Run: `npm --prefix apps/mobile-next test -- --runInBand tests/paseo-shell/createPaseoShell.test.tsx`

Expected: PASS with the router callback receiving `"tasks"`.

- [ ] **Step 5: Controller integration gate.**

Provide the constructor and route contract to the controller. The controller alone resolves `_layout.tsx`, package/export/lockfile conflicts and executes `npm --prefix apps/mobile-next run export:web`.
