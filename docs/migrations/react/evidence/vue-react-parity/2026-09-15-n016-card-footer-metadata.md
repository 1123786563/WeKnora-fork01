# N016 MCP card footer metadata

The MCP card footer now groups tools/sync metadata and transport type as Vue does, while keeping the enabled switch as a separate right-side control. Tool stale state retains its warning treatment and all existing action callbacks.

Verification: MCP focused tests 13/13, Web regression 911/911, Web typecheck and `git diff --check` passed. Live synced-service and Wails/native evidence remain open.
