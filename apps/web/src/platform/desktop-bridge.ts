export interface DesktopBridge {
  GetAPIBaseURL?: () => Promise<string> | string;
}

export interface ApiBaseUrlInputs {
  injected?: string;
  bridge?: DesktopBridge;
  configured: string;
  origin: string;
}

function normalize(value: string | undefined): string {
  return value?.trim().replace(/\/+$/, '') ?? '';
}

/** Resolve the API root shared by the Wails desktop shell and browser Web app. */
export async function resolveApiBaseUrl(inputs: ApiBaseUrlInputs): Promise<string> {
  const injected = normalize(inputs.injected);
  if (injected) return injected;

  if (inputs.bridge?.GetAPIBaseURL) {
    try {
      const bridged = normalize(await inputs.bridge.GetAPIBaseURL());
      if (bridged) return bridged;
    } catch {
      // The browser renderer can load before Wails bindings are available.
    }
  }

  return normalize(inputs.configured) || normalize(inputs.origin);
}

export function wailsBridgeFromWindow(value: {
  go?: { main?: { App?: DesktopBridge } };
}): DesktopBridge | undefined {
  return value.go?.main?.App;
}
