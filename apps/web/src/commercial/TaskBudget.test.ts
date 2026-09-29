import test from 'node:test';
import assert from 'node:assert/strict';
import * as nodeModule from 'node:module';

// @weknora/ui pulls in theme.css; node:test needs the same short-circuit as
// the sibling commercial tests (BillingPage.test.ts pattern), and the module
// under test must be imported dynamically AFTER the hook is registered.
const hooks = nodeModule as typeof nodeModule & { registerHooks?: (hooks: { resolve: (specifier: string, context: unknown, nextResolve: (specifier: string, context: unknown) => unknown) => void }) => void };
if (hooks.registerHooks) hooks.registerHooks({ resolve: (specifier, context, nextResolve) => specifier.endsWith('.css') ? { shortCircuit: true, url: 'data:text/javascript,export default {}' } : nextResolve(specifier, context) });

const { remainingCreditsOf } = await import('./TaskBudget.tsx');

test('remainingCreditsOf derives the fourth number from the three facts', () => {
  assert.equal(remainingCreditsOf({ limit: 1000, used: 400, held: 100 }), 500);
  assert.equal(remainingCreditsOf({ limit: 100, used: 0, held: 0 }), 100);
  assert.equal(remainingCreditsOf({ limit: 100, used: 100, held: 0 }), 0);
});
