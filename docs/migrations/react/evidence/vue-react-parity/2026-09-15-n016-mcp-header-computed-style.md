# N016 MCP page-header computed-style comparison

## Runtime baseline

At the authenticated MCP route (`http://localhost:5173/platform/settings?section=mcp`), Vue computed styles were: heading `600 20px` with `0 0 8px` margin and `rgba(0,0,0,.9)` color; description `14px/22.4px`, `rgba(0,0,0,.6)`. The paired React route at port 5181 had matching `600 20px` heading geometry and `14px/22.4px` description geometry, but its colors were `#172033` and `#66758b`.

## Change and verification

React MCP page-header utilities now use the Vue runtime colors directly while preserving the existing Tailwind/shadcn structure. MCP tests 13/13, Web typecheck, and full Web regression 911/911 passed after the change.

## Evidence boundary

The pre-change paired computed-style sample and source correction are recorded here. A reload of the React tab lost the authenticated content before a post-change browser sample could be captured; post-change authenticated computed-style evidence remains open.
