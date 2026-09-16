/**
 * Testable contract snapshot for the KnowledgeBaseEditorModal shell.
 *
 * The runtime stylesheet is owned by the web entry point. This module keeps
 * the Vue-derived geometry and responsive hierarchy available to focused
 * contract tests without importing the browser entry point.
 */
export const KNOWLEDGE_EDITOR_LAYOUT_CSS = `
.wk-kb-editor-dialog {
  width: min(90vw, 1000px);
  height: min(85vh, 750px);
  max-height: 750px;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.12);
}
.wk-kb-editor-dialog > .wk-dialog-header {
  position: absolute;
}
.wk-kb-editor-dialog > .wk-dialog-header h2 {
  left: 14px;
}
.wk-kb-editor-dialog form > div:first-child > div:last-child {
  overflow-y: auto;
}
.wk-kb-editor-dialog form > div:last-child {
  border-top: 1px solid var(--color-line);
}
.wk-kb-editor-dialog form > div:last-child > button:first-child { order: 2; }
.wk-kb-editor-dialog form > div:last-child > button:last-child { order: 1; }
@media (max-width: 680px) {
  .wk-kb-editor-dialog form > div:first-child { grid-template-columns: 1fr; }
  .wk-kb-editor-dialog form > div:first-child > div:last-child { overflow-y: visible; padding: 16px; }
}
`;
