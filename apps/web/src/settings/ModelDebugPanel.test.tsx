import assert from "node:assert/strict";
import test from "node:test";
import * as React from "react";
import { renderToStaticMarkup } from "react-dom/server";
;(globalThis as typeof globalThis & { React: typeof React }).React = React;

import * as nodeModule from "node:module";
type ResolveHook = (
  specifier: string,
  context: unknown,
  nextResolve: (specifier: string, context: unknown) => unknown,
) => unknown;
const hooks = nodeModule as typeof nodeModule & {
  registerHooks?: (hooks: { resolve: ResolveHook }) => void;
};
const resolveCSS: ResolveHook = (specifier, context, nextResolve) =>
  specifier.endsWith(".css")
    ? { shortCircuit: true, url: "data:text/javascript,export default {}" }
    : nextResolve(specifier, context);
if (hooks.registerHooks) hooks.registerHooks({ resolve: resolveCSS });
else
  nodeModule.register(
    `data:text/javascript,${encodeURIComponent(`
  export async function resolve(specifier, context, nextResolve) {
    if (specifier.endsWith('.css')) return { shortCircuit: true, url: 'data:text/javascript,export default {}' };
    return nextResolve(specifier, context);
  }
`)}`,
    import.meta.url,
  );

const { ModelDebugPanel } = await import("./ModelDebugPanel.tsx");

test("model debug panel exposes model selection and type-specific input controls", () => {
  const html = renderToStaticMarkup(
    React.createElement(ModelDebugPanel, {
      client: {} as never,
      models: [
        { id: "chat-1", name: "chat", type: "KnowledgeQA", source: "remote" },
        { id: "rank-1", name: "rank", type: "Rerank", source: "remote" },
      ],
      onClose: () => undefined,
    }),
  );
  assert.match(html, /Debug model/);
  assert.match(html, /chat · chat/);
  assert.match(html, /System prompt/);
});
