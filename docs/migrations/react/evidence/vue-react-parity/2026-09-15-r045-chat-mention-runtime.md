# R045 Chat mention protected runtime

- Runtime: authenticated React Web at `/platform/creatChat` using the shared parity tenant.
- Observed: composer exposes the localized `知识库` mention button; opening it renders the `wk-chat-mention-listbox`, a searchable field, and the available `Parity KB Demo` option with active selection semantics.
- This confirms the chat composer mention interaction survives the current lazy-entry and direct-import changes.
- Limitation: the React host currently loads KB options only. Vue also exposes file/tag/MCP/skill groups; those resource loaders remain an implementation gap and are not claimed by this evidence.
