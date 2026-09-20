import { createTrustedScopePublisher, subscribeCloudWorkspace } from "@/cloud-workspace/CloudWorkspaceProvider";
import type { WeKnoraApi } from "@/api/weknora";

describe("CloudWorkspaceProvider authentication lifetime", () => {
  it("publishes t1 → null → t2 client lifetimes with their generations", () => {
    const scopes: Array<{ tenantId: string; generation: number } | null> = [];
    const api = {} as WeKnoraApi;
    const onTrustedScope = createTrustedScopePublisher(api);
    onTrustedScope({ backend: "https://api", accountId: "u1", tenantId: "t1", generation: 4 });
    const unsubscribe = subscribeCloudWorkspace((client) => {
      scopes.push(client ? { tenantId: client.scope.tenantId, generation: client.scope.generation } : null);
    });
    scopes.length = 0;

    onTrustedScope(null);
    onTrustedScope({ backend: "https://api", accountId: "u1", tenantId: "t2", generation: 5 });

    expect(scopes).toEqual([null, { tenantId: "t2", generation: 5 }]);
    unsubscribe();
    onTrustedScope(null);
  });
});
