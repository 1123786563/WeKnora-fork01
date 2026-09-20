import React, { createContext, useContext, useEffect, useState } from "react";
import type { WeKnoraApi } from "@/api/weknora";
import { CloudWorkspaceClient, type CloudWorkspaceScope } from "@/cloud-workspace/CloudWorkspaceClient";

type CloudWorkspaceSubscriber = (client: CloudWorkspaceClient | null) => void;

const CloudWorkspaceContext = createContext<CloudWorkspaceClient | null>(null);
const subscribers = new Set<CloudWorkspaceSubscriber>();
let currentClient: CloudWorkspaceClient | null = null;

function publishCloudWorkspace(client: CloudWorkspaceClient | null): void {
  currentClient = client;
  for (const subscriber of subscribers) subscriber(client);
}

/**
 * Builds the callback that the auth composition supplies as AuthDeps.onTrustedScope.
 * It deliberately receives only a scope that has already passed auth lifecycle checks.
 */
export function createTrustedScopePublisher(api: WeKnoraApi): (scope: CloudWorkspaceScope | null) => void {
  return (scope) => publishCloudWorkspace(scope ? new CloudWorkspaceClient(scope, api) : null);
}

/** Subscription seam for provider consumers and the authentication-lifecycle fixture. */
export function subscribeCloudWorkspace(subscriber: CloudWorkspaceSubscriber): () => void {
  subscribers.add(subscriber);
  subscriber(currentClient);
  return () => subscribers.delete(subscriber);
}

export function CloudWorkspaceProvider({ children }: { children: React.ReactNode }): React.ReactElement {
  const [client, setClient] = useState<CloudWorkspaceClient | null>(currentClient);

  useEffect(() => subscribeCloudWorkspace(setClient), []);

  return <CloudWorkspaceContext.Provider value={client}>{children}</CloudWorkspaceContext.Provider>;
}

export function useCloudWorkspaceClient(): CloudWorkspaceClient | null {
  return useContext(CloudWorkspaceContext);
}
