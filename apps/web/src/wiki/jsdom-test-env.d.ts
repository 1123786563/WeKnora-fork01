// Minimal test-env typing for jsdom (devDependency without bundled types).
// Only what markdown.test.tsx needs: a window/document pair that DOMPurify
// can bind to and DOMParser for sanitized-output assertions.
declare module 'jsdom' {
  export class JSDOM {
    constructor(html?: string, options?: { url?: string });
    readonly window: Window & typeof globalThis;
  }
}
