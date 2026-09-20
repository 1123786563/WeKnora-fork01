import { createTrustedScopePublisher, subscribeCloudWorkspace } from "@/cloud-workspace/CloudWorkspaceProvider";
import type { WeKnoraApi } from "@/api/weknora";

describe("CloudWorkspaceProvider authentication lifetime", () => {
  it("drops t1 client before publishing t2 after a successful server switch", async () => {
    const scopes: Array<string | null> = [];
    const api = {} as WeKnoraApi;
    const onTrustedScope = createTrustedScopePublisher(api);
    const unsubscribe = subscribeCloudWorkspace((client) => scopes.push(client?.scope.tenantId ?? null));
    const auth = {
      async switchSpace(tenantId: string): Promise<void> {
        onTrustedScope(null);
        onTrustedScope({ backend: "https://api", accountId: "u1", tenantId, generation: 5 });
      },
    };

    await auth.switchSpace("t2");

    expect(scopes).toContain(null);
    expect(scopes.at(-1)).toBe("t2");
    unsubscribe();
  });
});
