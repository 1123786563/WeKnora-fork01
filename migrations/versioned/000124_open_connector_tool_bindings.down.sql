-- T14 down (PG): drop the agent tool-call binding table. The bound app
-- actions and their approvals survive in app_actions; dropping the bindings
-- only disconnects agent tool calls from them (a later re-prepare under the
-- same tool call id would create a NEW action, never resurrect an old one).
DROP TABLE connector_tool_bindings;
