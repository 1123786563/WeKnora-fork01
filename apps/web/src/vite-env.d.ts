/// <reference types="vite/client" />

interface Window {
  /** Injected by the Wails desktop shell before the React renderer starts. */
  __WEKNORA_API_BASE__?: string;
  go?: {
    main?: {
      App?: {
        GetAPIBaseURL?: () => Promise<string> | string;
      };
    };
  };
}
