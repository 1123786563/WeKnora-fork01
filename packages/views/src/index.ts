export { ChatComposer, createChatSubmission } from './chat/composer.tsx';
export type { ChatComposerProps, ChatMentionView, ChatSteerQueueChip, ChatSubmission } from './chat/composer.tsx';
export { AgentSelectorPanel, agentNotReadyLabels } from './chat/agent-selector.tsx';
export type { AgentSelectorAgent, AgentSelectorModel, AgentSelectorProps } from './chat/agent-selector.tsx';
// R484 D15 — Vue agentWebSearch.ts gates backing the composer globe toggle.
export { isAgentWebSearchEnabled, isAgentWebSearchReady, isTenantWebSearchReady, resolveAgentWebSearchProviderId } from './chat/web-search.ts';
export type { AgentWebSearchConfigLike, WebSearchProviderLike } from './chat/web-search.ts';
export { resolveForkAffordance, stashForkLanding, takeForkLanding } from './chat/fork-point.ts';
export { MessageList, renderMessageHtml } from './chat/message-list.tsx';
export type { MessageListProps, PendingChatMessage } from './chat/message-list.tsx';
export {
  CHAT_COPY,
  CHAT_COPY_LOCALES,
  chatCopy,
  formatChatCopy,
  resolveChatCopy,
  resolveChatLocale,
  sessionGroupLabel,
} from './chat/chat-copy.ts';
export type { ChatCopyKey, ChatCopyLocale, ChatCopyTable } from './chat/chat-copy.ts';
export { renderChatMarkdown } from './chat/markdown.ts';
export { ArtifactPreview, artifactPreviewModel } from './chat/artifact-preview.tsx';
export type { ArtifactPreviewKind, ArtifactPreviewModel, ArtifactPreviewPayload, ArtifactPreviewProps } from './chat/artifact-preview.tsx';
export { hydrateMermaidBlocks, hydrateMermaidBlocksWithBrowserDefaults, MERMAID_RENDER_CONFIG } from './chat/mermaid.ts';
export type { MermaidEngine } from './chat/mermaid.ts';
export { SessionSidebar, SessionSidebarList, SessionSidebarShellContext } from './chat/session-sidebar.tsx';
export type { SessionGroupView, SessionSidebarListProps, SessionSidebarProps, SessionSourceOption } from './chat/session-sidebar.tsx';
export { ChatPage } from './chat/page.tsx';
export type { ChatAgentOption, ChatOAuthApprovalPrompt, ChatPageProps, ChatStreamPresentation, ChatTerminalView, ChatToolApprovalPrompt, ChatToolCallView } from './chat/page.tsx';
export { ReferenceList, referenceSections } from './chat/reference-list.tsx';
export type { ReferenceListProps, ReferenceSection } from './chat/reference-list.tsx';
export { ToolResultView, toolResultPresentation } from './chat/tool-result.tsx';
export type { ToolResultPresentation, ToolResultViewInput } from './chat/tool-result.tsx';
export { SETTINGS_SECTIONS, roleAtLeast, settingsSection, settingsSectionsForRole } from './settings/registry.ts';
export type { SettingsSection, SettingsScope, SettingsRole, SettingsOperation } from './settings/registry.ts';
export { createEmbedBridgeGuard, EMBED_HOST_SOURCE, EMBED_MESSAGE_SOURCE } from './embed/bridge.ts';
export type { EmbedMessageEvent } from './embed/bridge.ts';
export { INTEGRATION_SECTIONS, integrationKeyFromQuery, integrationSection } from './integrations/registry.ts';
export type { IntegrationKey, IntegrationOperation, IntegrationSection } from './integrations/registry.ts';
export { IntegrationsPage } from './integrations/page.tsx';
export type { APIPrincipalConfig, IntegrationActions, IntegrationPrincipalToken, IntegrationResource, IntegrationsPageProps } from './integrations/page.tsx';
export type { IntegrationAgentOption, IntegrationKnowledgeBaseOption, IntegrationWeChatQrPorts } from './integrations/page.tsx';
export {
  IM_WIZARD_STEPS,
  applyImPlatformChange,
  buildImCreatePayload,
  buildImUpdatePayload,
  createImWizardForm,
  imCredentialFields,
  imPlatformSupportsThread,
  imWizardFormFromChannel,
  validateImWizardSave,
  validateImWizardStep,
} from './integrations/imWizard.ts';
export type { ImWizardForm } from './integrations/imWizard.ts';
export {
  EMBED_WIZARD_STEPS,
  WEKNORA_BRAND_COLOR,
  buildEmbedCreatePayload,
  buildEmbedUpdatePayload,
  createEmbedWizardForm,
  embedWizardFormFromChannel,
  embedWizardSteps,
  mapEmbedOriginsApiError,
  parseEmbedAllowedOrigins,
  validateEmbedAllowedOrigins,
  validateEmbedWizardStep,
} from './integrations/embedWizard.ts';
export type { EmbedWizardForm, EmbedWizardWarning } from './integrations/embedWizard.ts';
export { apiKeyAccessMode, apiKeyValueDisplay, isFreshKeyVisible } from './integrations/apiKeys.ts';
export type { ApiKeyRow } from './integrations/apiKeys.ts';
export type { EmbedResourceLike } from './integrations/form.ts';
export { integrationTabForSection, normalizeIntegrationSettingsSection, integrationSettingsQuery, selectSettingsQuery } from './integrations/settings-route.ts';
export { NewUserGuide, computeCardStyle, computeBackdropPieces, computeHighlightHole, GUIDE_CARD_WIDTH } from './guides/NewUserGuide.tsx';
export type { NewUserGuideActions, NewUserGuideProps } from './guides/NewUserGuide.tsx';
export { GLOBAL_USER_GUIDE_KEY, OPEN_NEW_USER_GUIDE_EVENT, isNewUserGuideDone, markNewUserGuideDone, openNewUserGuide, shouldAutoOpenNewUserGuide } from './guides/new-user-guide.ts';
export type { KeyValueStorage } from './guides/new-user-guide.ts';
export { guideMessage, NEW_USER_GUIDE_MESSAGES, NEW_USER_GUIDE_STEPS } from './guides/steps.ts';
export type { GuidePlacement, NewUserGuideAction, NewUserGuideLocale, NewUserGuideStep } from './guides/steps.ts';
export { ContextualGuide, ContextualGuideHost } from './guides/ContextualGuide.tsx';
export type { ContextualGuideActions, ContextualGuideHostProps, ContextualGuideProps } from './guides/ContextualGuide.tsx';
export {
  AGENT_EDITOR_FOCUS_SECTION_EVENT,
  CONTEXTUAL_GUIDE_PENDING_KEY,
  CONTEXTUAL_GUIDE_STEPS,
  CONTEXTUAL_GUIDE_TOUR_IDS,
  CONTEXTUAL_GUIDE_TOUR_STORAGE_KEYS,
  CONTEXTUAL_GUIDE_OPEN_DELAY_MS,
  CONTEXTUAL_GUIDE_ALSO_COMPLETE_TOURS,
  KB_EDITOR_FOCUS_SECTION_EVENT,
  OPEN_CONTEXTUAL_GUIDE_EVENT,
  agentCreateGuideSteps,
  contextualGuideMessage,
  contextualGuideStepPrefix,
  consumePendingContextualGuide,
  focusAgentEditorSection,
  focusKbEditorSection,
  isContextualGuideDone,
  isGlobalUserGuideDone,
  kbCreateGuideSteps,
  markContextualGuideDone,
  openContextualGuide,
  resolveContextualGuideSteps,
  shouldOpenContextualGuide,
} from './guides/contextual-guides.ts';
export type {
  ContextualGuideMessageKey,
  ContextualGuideLocale,
} from './guides/contextual-guide-messages.ts';
export { CONTEXTUAL_GUIDE_MESSAGES } from './guides/contextual-guide-messages.ts';
export type {
  ContextualGuideOpenDetail,
  ContextualGuidePlacement,
  ContextualGuideStep,
  ContextualGuideStepAction,
  ContextualGuideTourId,
  ContextualGuideTriggerOptions,
} from './guides/contextual-guides.ts';
