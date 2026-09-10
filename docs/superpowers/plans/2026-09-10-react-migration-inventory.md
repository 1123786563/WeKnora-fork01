# React 迁移静态库存

采集日期：2026-09-10。源提交：`5cf093706ebecdfe8bc4eca80886e80c01805289`。

本表由本地文件目录和 docs/swagger.json 静态生成。任务分配是路径初分，T01 须逐个确认调用关系、权限和迁移去向；不表示逐个文件已深读或接口已通过实测。Swagger operation 不是运行路由清单。

## Vue 单文件组件

| 源文件（相对仓库根） | 建议任务 | 状态 |
|---|---|---|
| `frontend/src/App.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/AgentAvatar.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/components/AgentCreateContextualGuide.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/components/AgentEmbedChannelPanel.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/components/AgentSelector.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/components/AgentShareSettings.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/components/AttachmentUpload.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ChatAttachmentPreviewDrawer.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/components/ChatCitationFloat.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/components/ChatHeader.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ChatReferencesDrawer.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/components/ChatRequestInfoButton.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ContextualGuide.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/CreateTenantDialog.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/components/EmbedChannelPreview.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/components/EmbedInputField.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/components/FAQTagTooltip.vue` | T08 | 已盘点，未迁移 |
| `frontend/src/components/GlobalCommandPalette/ResultGroup.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/GlobalCommandPalette/ResultItem.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/GlobalCommandPalette.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/GlobalInvitationBell.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/components/IMChannelPanel.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/components/Input-field.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/IntegrationsAgentFilter.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/components/KBInfoPopover.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/KBSwitcherDropdown.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/KbCreateContextualGuide.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/KnowledgeBaseSelector.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ListSpaceSidebar.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/MentionSelector.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ModelDebugDrawer.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/ModelEditorDialog.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/ModelSelector.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/MyInvitationsDialog.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/components/NewUserGuide.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/PromptTemplateSelector.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ProtectedResourcePreview.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/components/ResourceOriginBadge.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/SandboxConfigEditorDrawer.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/SandboxSkillsPanel.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/SessionGroupByDropdown.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/SessionSidebarRow.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/SessionSourceFilter.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/ShareKnowledgeBaseDialog.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/ShareToSpaceOrgSelect.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/components/SkillFilesDrawer.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/SkillFilesPanel.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/SkillInstallTimeline.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/SourceSwitcherDropdown.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/SpaceAvatar.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/SpotlightGuide.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/TenantModelsGuide.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/TenantSelector.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/UploadConfirmHost.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/UserMenu.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/VectorStoreBadge.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/chat/ChatQuestionMinimap.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/chat/FollowUpSuggestions.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/chat/MessageTimestamp.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/chat/SandboxSidePanel.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/components/credentials/CredentialResource.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/doc-content.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/document-preview.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/components/empty-knowledge.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/knowledge-processing-timeline.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/manual-knowledge-editor.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/menu.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/components/picture-preview.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/components/settings/SandboxBackendBadge.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/settings/SettingCard.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/settings/SettingDrawer.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/components/upload-mask.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/views/agent/AgentEditorModal.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/views/agent/AgentList.vue` | T15 | 已盘点，未迁移 |
| `frontend/src/views/auth/Login.vue` | T03/T04 | 已盘点，未迁移 |
| `frontend/src/views/auth/WorkspaceOnboarding.vue` | T03/T04 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/AgentStreamDisplay.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/ArtifactFileIcon.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/ChatArtifactsDrawer.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/ChatArtifactsPanel.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/ChatMemoryStep.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/McpOAuthCard.vue` | T12 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/RagPipelineProgress.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/SandboxTerminal.vue` | T14 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/ToolApprovalCard.vue` | T12 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/ToolResultRenderer.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/botmsg.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/deepThink.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/docInfo.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/sendMsg.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/ChunkDetail.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/ContentPopup.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/DatabaseQuery.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/DocumentInfo.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/GraphQueryResults.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/GrepResults.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/KnowledgeBaseList.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/KnowledgeChunksList.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/McpToolResult.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/PlanDisplay.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/ReadSkillResult.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/RelatedChunks.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/ResultRow.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/SandboxFilesResult.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/SearchResults.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/ShellExecResult.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/ThinkingDisplay.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/WebFetchResults.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/WebSearchResults.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/WikiEditResult.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/tool-results/WriteSandboxFileResult.vue` | T07/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/components/usermsg.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/chat/index.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/creatChat/creatChat.vue` | T11/T13 | 已盘点，未迁移 |
| `frontend/src/views/dev/MarkdownTestPage.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/views/embed/EmbedBotMessage.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/embed/EmbedChatCore.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/embed/EmbedChatView.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/embed/EmbedPage.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/embed/EmbedUserMessage.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/ApiIntegrationSettings.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/ChromeExtensionLanding.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/ClawSkillLanding.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/CliIntegrationLanding.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/IntegrationExternalCta.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/IntegrationLandingLayout.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/integrations/IntegrationSettingsSection.vue` | T18 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/KnowledgeBase.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/KnowledgeBaseEditorModal.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/KnowledgeBaseList.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/BatchTagDialog.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/DocumentActionMenu.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/DocumentBatchBar.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/DocumentCardView.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/DocumentListView.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/FAQBatchBar.vue` | T08 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/FAQEntryManager.vue` | T08 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/FolderPickerMenu.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/KbFolderTree.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/KbTagManageDrawer.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/KbUploadSourceDropdown.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/KbWikiBadge.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/TagEditDialog.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/components/UploadConfirmDialog.vue` | T06/T07 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/DataSourceSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/DataSourceSyncLogs.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/DataSourceTypeIcon.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/GraphSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBAdvancedSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBChunkingDebug.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBChunkingSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBIndexingStrategy.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBModelConfig.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBParserSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBShareSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBStorageSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KBVectorStoreSettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/settings/KnowledgeBaseActivitySettings.vue` | T09 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/wiki/WikiBrowser.vue` | T08 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/wiki/WikiFolderActions.vue` | T08 | 已盘点，未迁移 |
| `frontend/src/views/knowledge/wiki/WikiRevisionDrawer.vue` | T08 | 已盘点，未迁移 |
| `frontend/src/views/organization/JoinOrganization.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/organization/OrganizationEditorModal.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/organization/OrganizationList.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/organization/OrganizationSettingsModal.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/platform/RoutePlaceholder.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/views/platform/index.vue` | T04/T05（T01 核对具体业务） | 已盘点，未迁移 |
| `frontend/src/views/settings/ChatHistorySettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/EnvVarSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/GeneralSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/McpSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/MemorySettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/MemoryWorkspaceSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/ModelSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/OllamaSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/ParserEngineSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/RetrievalSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/SandboxSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/Settings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/SkillSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/StorageBackendSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/StorageEngineSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/SystemInfo.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/TenantInfo.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/TenantMembers.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/settings/UserProfile.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/VectorStoreSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/WeKnoraCloudSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/WebSearchSettings.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/components/McpMetadataPanel.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/components/McpServiceDialog.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/components/McpTestResultBody.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/settings/components/McpToolsList.vue` | T15/T17 | 已盘点，未迁移 |
| `frontend/src/views/system/CreateUserDialog.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/system/PlatformAPIKeys.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/system/ResetPasswordDialog.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/system/RuntimeQueues.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/system/SystemAuditLog.vue` | T16 | 已盘点，未迁移 |
| `frontend/src/views/system/SystemSettings.vue` | T16 | 已盘点，未迁移 |

## API TypeScript 文件

| 源文件 | 状态 |
|---|---|
| `frontend/src/api/agent/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/auth/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/chat/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/chat/steer.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/chat/streame.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/chat/temporary-attachments.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/chat-history.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/chunker/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/datasource/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/embed/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/env-vars.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/initialization/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/knowledge-base/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/mcp-service.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/memory.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/message-suggestion.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/model/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/model/modelUsage.test.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/model/modelUsage.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/organization/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/retrieval.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/skill/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/storage-backend.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/system/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/tenant/audit-log.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/tenant/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/tenant/invitations.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/tenant/members.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/user-favorites.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/vector-store.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/web-search-provider.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/web-search.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |
| `frontend/src/api/wiki/index.ts` | 已盘点；T01 对照实际 API，T02 起按业务域迁移 |

## Swagger 2.0 操作清单

basePath：`/api/v1`。以下 path 为文档内路径；不得一律再加 basePath 而不验证实际路由。

| Method | 文档 path | 标签 | 运行契约证据 |
|---|---|---|---|
| POST | `/agent-chat/{session_id}` | 问答 | T01 待对照 handler/权限/响应 |
| POST | `/agent/mcp-oauth-resolutions/{pending_id}` | MCP服务 | T01 待对照 handler/权限/响应 |
| POST | `/agent/mcp-oauth-resolutions/{pending_id}/cancel` | MCP服务 | T01 待对照 handler/权限/响应 |
| POST | `/agent/tool-approvals/{pending_id}` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/agents` | 智能体 | T01 待对照 handler/权限/响应 |
| POST | `/agents` | 智能体 | T01 待对照 handler/权限/响应 |
| GET | `/agents/placeholders` | 智能体 | T01 待对照 handler/权限/响应 |
| GET | `/agents/type-presets` | 智能体 | T01 待对照 handler/权限/响应 |
| DELETE | `/agents/{id}` | 智能体 | T01 待对照 handler/权限/响应 |
| GET | `/agents/{id}` | 智能体 | T01 待对照 handler/权限/响应 |
| PUT | `/agents/{id}` | 智能体 | T01 待对照 handler/权限/响应 |
| POST | `/agents/{id}/copy` | 智能体 | T01 待对照 handler/权限/响应 |
| DELETE | `/agents/{id}/shares/{share_id}` | 组织 | T01 待对照 handler/权限/响应 |
| GET | `/agents/{id}/suggested-questions` | 智能体 | T01 待对照 handler/权限/响应 |
| GET | `/api/v1/knowledge/{id}/spans` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/auth/auto-setup` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/change-password` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/config` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/invitations/lookup` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/login` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/logout` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/me` | 认证 | T01 待对照 handler/权限/响应 |
| PUT | `/auth/me/preferences` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/oidc/callback` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/oidc/config` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/oidc/start` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/oidc/url` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/refresh` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/register` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/register-by-invite` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/auth/switch-tenant` | 认证 | T01 待对照 handler/权限/响应 |
| GET | `/auth/validate` | 认证 | T01 待对照 handler/权限/响应 |
| POST | `/chunker/preview` | 分块 | T01 待对照 handler/权限/响应 |
| GET | `/chunks/by-id/{id}` | 分块管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/chunks/by-id/{id}/questions` | 分块管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/chunks/{knowledge_id}` | 分块管理 | T01 待对照 handler/权限/响应 |
| GET | `/chunks/{knowledge_id}` | 分块管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/chunks/{knowledge_id}/{id}` | 分块管理 | T01 待对照 handler/权限/响应 |
| PUT | `/chunks/{knowledge_id}/{id}` | 分块管理 | T01 待对照 handler/权限/响应 |
| GET | `/datasource` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource` | DataSource | T01 待对照 handler/权限/响应 |
| GET | `/datasource/logs/{log_id}` | DataSource | T01 待对照 handler/权限/响应 |
| GET | `/datasource/types` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource/validate-credentials` | DataSource | T01 待对照 handler/权限/响应 |
| DELETE | `/datasource/{id}` | DataSource | T01 待对照 handler/权限/响应 |
| GET | `/datasource/{id}` | DataSource | T01 待对照 handler/权限/响应 |
| PUT | `/datasource/{id}` | DataSource | T01 待对照 handler/权限/响应 |
| GET | `/datasource/{id}/logs` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource/{id}/pause` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource/{id}/resource-ancestors` | DataSource | T01 待对照 handler/权限/响应 |
| GET | `/datasource/{id}/resources` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource/{id}/resume` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource/{id}/sync` | DataSource | T01 待对照 handler/权限/响应 |
| POST | `/datasource/{id}/validate` | DataSource | T01 待对照 handler/权限/响应 |
| GET | `/evaluation/` | 评估 | T01 待对照 handler/权限/响应 |
| POST | `/evaluation/` | 评估 | T01 待对照 handler/权限/响应 |
| GET | `/faq/import/progress/{task_id}` | FAQ管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/im-channels/{id}` | IM 渠道 | T01 待对照 handler/权限/响应 |
| PUT | `/im-channels/{id}` | IM 渠道 | T01 待对照 handler/权限/响应 |
| POST | `/im-channels/{id}/toggle` | IM 渠道 | T01 待对照 handler/权限/响应 |
| GET | `/im/callback/{channel_id}` | IM 回调 | T01 待对照 handler/权限/响应 |
| POST | `/im/callback/{channel_id}` | IM 回调 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/asr/check` | 初始化 | T01 待对照 handler/权限/响应 |
| GET | `/initialization/config/{kbId}` | 初始化 | T01 待对照 handler/权限/响应 |
| PUT | `/initialization/config/{kbId}` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/embedding/test` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/extract/fabri-tag` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/extract/fabri-text` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/extract/text-relation` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/initialize/{kbId}` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/multimodal/test` | 初始化 | T01 待对照 handler/权限/响应 |
| GET | `/initialization/ollama/download/progress/{taskId}` | 初始化 | T01 待对照 handler/权限/响应 |
| GET | `/initialization/ollama/download/tasks` | 初始化 | T01 待对照 handler/权限/响应 |
| GET | `/initialization/ollama/models` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/ollama/models/check` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/ollama/models/download` | 初始化 | T01 待对照 handler/权限/响应 |
| GET | `/initialization/ollama/status` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/remote/check` | 初始化 | T01 待对照 handler/权限/响应 |
| POST | `/initialization/rerank/check` | 初始化 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases` | 知识库 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases` | 知识库 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/copy` | 知识库 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/copy/progress/{task_id}` | 知识库 | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledge-bases/{id}` | 知识库 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}` | 知识库 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}` | 知识库 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/activity` | 知识库 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/duplicate` | 知识库 | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledge-bases/{id}/faq/entries` | FAQ管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/faq/entries` | FAQ管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/faq/entries` | FAQ管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/faq/entries/export` | FAQ管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/faq/entries/fields` | FAQ管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/faq/entries/tags` | FAQ管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/faq/entries/{entry_id}` | FAQ管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/faq/entries/{entry_id}` | FAQ管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/faq/entries/{entry_id}/similar-questions` | FAQ管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/faq/entry` | FAQ管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/faq/import/last-result/display` | FAQ管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/faq/search` | FAQ管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/hybrid-search` | 知识库 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/hybrid-search` | 知识库 | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledge-bases/{id}/knowledge` | 知识管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/knowledge` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/knowledge/file` | 知识管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/knowledge/folders` | 知识管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/knowledge/folders` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/knowledge/manual` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/knowledge/url` | 知识管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/move-targets` | 知识库 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/pin` | 知识库 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/shares` | 知识库共享 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/shares` | 知识库共享 | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledge-bases/{id}/shares/{share_id}` | 知识库共享 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/shares/{share_id}` | 知识库共享 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge-bases/{id}/tags` | 标签管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-bases/{id}/tags` | 标签管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledge-bases/{id}/tags/{tag_id}` | 标签管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge-bases/{id}/tags/{tag_id}` | 标签管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-chat/{session_id}` | 问答 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge-search` | 问答 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge/batch` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge/batch-delete` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge/batch-reparse` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge/folder` | 知识管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge/image/{id}/{chunk_id}` | 知识管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge/manual/{id}` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge/move` | 知识 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge/move/progress/{task_id}` | 知识 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge/search` | Knowledge | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge/tags` | 知识管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledge/{id}` | 知识管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge/{id}` | 知识管理 | T01 待对照 handler/权限/响应 |
| PUT | `/knowledge/{id}` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge/{id}/cancel-parse` | 知识管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge/{id}/download` | 知识管理 | T01 待对照 handler/权限/响应 |
| GET | `/knowledge/{id}/preview` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledge/{id}/reparse` | 知识管理 | T01 待对照 handler/权限/响应 |
| POST | `/knowledgebase/{kb_id}/wiki/auto-fix` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/folders` | Wiki | T01 待对照 handler/权限/响应 |
| POST | `/knowledgebase/{kb_id}/wiki/folders` | Wiki | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledgebase/{kb_id}/wiki/folders/{folder_id}` | Wiki | T01 待对照 handler/权限/响应 |
| PUT | `/knowledgebase/{kb_id}/wiki/folders/{folder_id}` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/graph` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/index` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/issues` | Wiki | T01 待对照 handler/权限/响应 |
| PUT | `/knowledgebase/{kb_id}/wiki/issues/{issue_id}/status` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/lint` | Wiki | T01 待对照 handler/权限/响应 |
| PUT | `/knowledgebase/{kb_id}/wiki/move-page` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/pages` | Wiki | T01 待对照 handler/权限/响应 |
| POST | `/knowledgebase/{kb_id}/wiki/pages` | Wiki | T01 待对照 handler/权限/响应 |
| DELETE | `/knowledgebase/{kb_id}/wiki/pages/{slug}` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/pages/{slug}` | Wiki | T01 待对照 handler/权限/响应 |
| PUT | `/knowledgebase/{kb_id}/wiki/pages/{slug}` | Wiki | T01 待对照 handler/权限/响应 |
| POST | `/knowledgebase/{kb_id}/wiki/rebuild-links` | Wiki | T01 待对照 handler/权限/响应 |
| POST | `/knowledgebase/{kb_id}/wiki/revert` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/revisions/{slug}` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/search` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/knowledgebase/{kb_id}/wiki/stats` | Wiki | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services` | MCP服务 | T01 待对照 handler/权限/响应 |
| POST | `/mcp-services` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services/oauth/callback` | MCP服务 | T01 待对照 handler/权限/响应 |
| DELETE | `/mcp-services/{id}` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services/{id}` | MCP服务 | T01 待对照 handler/权限/响应 |
| PUT | `/mcp-services/{id}` | MCP服务 | T01 待对照 handler/权限/响应 |
| PUT | `/mcp-services/{id}/credentials` | MCP服务 | T01 待对照 handler/权限/响应 |
| DELETE | `/mcp-services/{id}/credentials/{field}` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services/{id}/metadata` | MCP服务 | T01 待对照 handler/权限/响应 |
| POST | `/mcp-services/{id}/metadata/refresh` | MCP服务 | T01 待对照 handler/权限/响应 |
| POST | `/mcp-services/{id}/oauth/authorize-url` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services/{id}/oauth/status` | MCP服务 | T01 待对照 handler/权限/响应 |
| DELETE | `/mcp-services/{id}/oauth/token` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services/{id}/resources` | MCP服务 | T01 待对照 handler/权限/响应 |
| POST | `/mcp-services/{id}/test` | MCP服务 | T01 待对照 handler/权限/响应 |
| PUT | `/mcp-services/{id}/tool-approvals/{tool_name}` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/mcp-services/{id}/tools` | MCP服务 | T01 待对照 handler/权限/响应 |
| GET | `/me/env-vars` | Me | T01 待对照 handler/权限/响应 |
| DELETE | `/me/env-vars/sandbox` | Me | T01 待对照 handler/权限/响应 |
| PUT | `/me/env-vars/sandbox` | Me | T01 待对照 handler/权限/响应 |
| DELETE | `/me/env-vars/skill` | Me | T01 待对照 handler/权限/响应 |
| PUT | `/me/env-vars/skill` | Me | T01 待对照 handler/权限/响应 |
| GET | `/me/invitations` | 我的邀请 | T01 待对照 handler/权限/响应 |
| POST | `/me/invitations/accept-by-token` | 我的邀请 | T01 待对照 handler/权限/响应 |
| GET | `/me/invitations/pending-count` | 我的邀请 | T01 待对照 handler/权限/响应 |
| POST | `/me/invitations/{inv_id}/accept` | 我的邀请 | T01 待对照 handler/权限/响应 |
| POST | `/me/invitations/{inv_id}/decline` | 我的邀请 | T01 待对照 handler/权限/响应 |
| POST | `/memory/consolidate` | 长期记忆 | T01 待对照 handler/权限/响应 |
| GET | `/memory/documents` | 长期记忆 | T01 待对照 handler/权限/响应 |
| DELETE | `/memory/documents/{id}` | 长期记忆 | T01 待对照 handler/权限/响应 |
| GET | `/memory/export` | 长期记忆 | T01 待对照 handler/权限/响应 |
| DELETE | `/memory/items` | 长期记忆 | T01 待对照 handler/权限/响应 |
| GET | `/memory/items` | 长期记忆 | T01 待对照 handler/权限/响应 |
| POST | `/memory/items` | 长期记忆 | T01 待对照 handler/权限/响应 |
| DELETE | `/memory/items/{id}` | 长期记忆 | T01 待对照 handler/权限/响应 |
| PUT | `/memory/items/{id}` | 长期记忆 | T01 待对照 handler/权限/响应 |
| POST | `/memory/items/{id}/confirm` | 长期记忆 | T01 待对照 handler/权限/响应 |
| POST | `/memory/items/{id}/reject` | 长期记忆 | T01 待对照 handler/权限/响应 |
| GET | `/memory/settings` | 长期记忆 | T01 待对照 handler/权限/响应 |
| PUT | `/memory/settings` | 长期记忆 | T01 待对照 handler/权限/响应 |
| GET | `/memory/topics` | 长期记忆 | T01 待对照 handler/权限/响应 |
| DELETE | `/memory/topics/{id}` | 长期记忆 | T01 待对照 handler/权限/响应 |
| POST | `/memory/topics/{id}/promote` | 长期记忆 | T01 待对照 handler/权限/响应 |
| GET | `/messages/chat-history-stats` | 消息 | T01 待对照 handler/权限/响应 |
| POST | `/messages/search` | 消息 | T01 待对照 handler/权限/响应 |
| GET | `/messages/{session_id}/load` | 消息 | T01 待对照 handler/权限/响应 |
| DELETE | `/messages/{session_id}/{id}` | 消息 | T01 待对照 handler/权限/响应 |
| GET | `/models` | 模型管理 | T01 待对照 handler/权限/响应 |
| POST | `/models` | 模型管理 | T01 待对照 handler/权限/响应 |
| GET | `/models/providers` | 模型管理 | T01 待对照 handler/权限/响应 |
| GET | `/models/weknoracloud/status` | WeKnoraCloud | T01 待对照 handler/权限/响应 |
| DELETE | `/models/{id}` | 模型管理 | T01 待对照 handler/权限/响应 |
| GET | `/models/{id}` | 模型管理 | T01 待对照 handler/权限/响应 |
| PUT | `/models/{id}` | 模型管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/join` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/join-by-id` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/join-request` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/preview/{code}` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/search` | 组织管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/organizations/{id}` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}` | 组织管理 | T01 待对照 handler/权限/响应 |
| PUT | `/organizations/{id}` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/agent-shares` | 组织 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/{id}/invite` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/{id}/invite-code` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/join-requests` | 组织管理 | T01 待对照 handler/权限/响应 |
| PUT | `/organizations/{id}/join-requests/{request_id}/review` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/{id}/leave` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/members` | 组织管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/organizations/{id}/members/{tenant_id}` | 组织管理 | T01 待对照 handler/权限/响应 |
| PUT | `/organizations/{id}/members/{tenant_id}` | 组织管理 | T01 待对照 handler/权限/响应 |
| POST | `/organizations/{id}/request-upgrade` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/search-tenants` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/search-users` |  | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/shared-agents` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/shared-knowledge-bases` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/organizations/{id}/shares` | 组织管理 | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs` | SandboxConfig | T01 待对照 handler/权限/响应 |
| POST | `/sandbox-configs` | SandboxConfig | T01 待对照 handler/权限/响应 |
| DELETE | `/sandbox-configs/{id}` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}` | SandboxConfig | T01 待对照 handler/权限/响应 |
| PUT | `/sandbox-configs/{id}` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/sandboxes` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/skills` | SandboxConfig | T01 待对照 handler/权限/响应 |
| POST | `/sandbox-configs/{id}/skills` | SandboxConfig | T01 待对照 handler/权限/响应 |
| DELETE | `/sandbox-configs/{id}/skills/{skillId}` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/skills/{skillId}` | SandboxConfig | T01 待对照 handler/权限/响应 |
| PATCH | `/sandbox-configs/{id}/skills/{skillId}` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/skills/{skillId}/files` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/skills/{skillId}/files/content` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/skills/{skillId}/install-events` | SandboxConfig | T01 待对照 handler/权限/响应 |
| POST | `/sandbox-configs/{id}/skills/{skillId}/reinstall` | SandboxConfig | T01 待对照 handler/权限/响应 |
| POST | `/sandbox-configs/{id}/skills/{skillId}/stop` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sandbox-configs/{id}/skills/{skillId}/transcript` | SandboxConfig | T01 待对照 handler/权限/响应 |
| GET | `/sessions` | 会话 | T01 待对照 handler/权限/响应 |
| POST | `/sessions` | 会话 | T01 待对照 handler/权限/响应 |
| DELETE | `/sessions/batch` | 会话 | T01 待对照 handler/权限/响应 |
| GET | `/sessions/continue-stream/{session_id}` | 问答 | T01 待对照 handler/权限/响应 |
| DELETE | `/sessions/{id}` | 会话 | T01 待对照 handler/权限/响应 |
| GET | `/sessions/{id}` | 会话 | T01 待对照 handler/权限/响应 |
| PUT | `/sessions/{id}` | 会话 | T01 待对照 handler/权限/响应 |
| DELETE | `/sessions/{id}/messages` | 会话 | T01 待对照 handler/权限/响应 |
| DELETE | `/sessions/{id}/pin` | 会话 | T01 待对照 handler/权限/响应 |
| GET | `/sessions/{session_id}/artifacts` | 会话 | T01 待对照 handler/权限/响应 |
| GET | `/sessions/{session_id}/messages/{message_id}/artifacts` |  | T01 待对照 handler/权限/响应 |
| GET | `/sessions/{session_id}/messages/{message_id}/artifacts/{index}/download` |  | T01 待对照 handler/权限/响应 |
| GET | `/sessions/{session_id}/messages/{message_id}/suggestions` | 会话 | T01 待对照 handler/权限/响应 |
| POST | `/sessions/{session_id}/messages/{message_id}/suggestions` | 会话 | T01 待对照 handler/权限/响应 |
| POST | `/sessions/{session_id}/pin` | 会话 | T01 待对照 handler/权限/响应 |
| POST | `/sessions/{session_id}/stop` | 问答 | T01 待对照 handler/权限/响应 |
| POST | `/sessions/{session_id}/suggestion-events` | 会话 | T01 待对照 handler/权限/响应 |
| POST | `/sessions/{session_id}/title` | 会话 | T01 待对照 handler/权限/响应 |
| GET | `/shared-agents` | 组织 | T01 待对照 handler/权限/响应 |
| GET | `/shared-knowledge-bases` | 知识库共享 | T01 待对照 handler/权限/响应 |
| GET | `/skills` | Skills | T01 待对照 handler/权限/响应 |
| GET | `/skills/catalog` | Skills | T01 待对照 handler/权限/响应 |
| POST | `/skills/catalog` | Skills | T01 待对照 handler/权限/响应 |
| DELETE | `/skills/catalog/{id}` | Skills | T01 待对照 handler/权限/响应 |
| GET | `/skills/catalog/{id}/files` | Skills | T01 待对照 handler/权限/响应 |
| GET | `/skills/catalog/{id}/files/content` | Skills | T01 待对照 handler/权限/响应 |
| POST | `/skills/catalog/{id}/install` | Skills | T01 待对照 handler/权限/响应 |
| GET | `/storage-backends` | StorageBackend | T01 待对照 handler/权限/响应 |
| POST | `/storage-backends` | StorageBackend | T01 待对照 handler/权限/响应 |
| POST | `/storage-backends/test` | StorageBackend | T01 待对照 handler/权限/响应 |
| GET | `/storage-backends/types` | StorageBackend | T01 待对照 handler/权限/响应 |
| DELETE | `/storage-backends/{id}` | StorageBackend | T01 待对照 handler/权限/响应 |
| GET | `/storage-backends/{id}` | StorageBackend | T01 待对照 handler/权限/响应 |
| PUT | `/storage-backends/{id}` | StorageBackend | T01 待对照 handler/权限/响应 |
| PUT | `/storage-backends/{id}/default` | StorageBackend | T01 待对照 handler/权限/响应 |
| POST | `/storage-backends/{id}/test` | StorageBackend | T01 待对照 handler/权限/响应 |
| GET | `/system/admin/api-keys` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/api-keys` | System Admin | T01 待对照 handler/权限/响应 |
| DELETE | `/system/admin/api-keys/{key_id}` | System Admin | T01 待对照 handler/权限/响应 |
| GET | `/system/admin/audit-log` | 审计日志 | T01 待对照 handler/权限/响应 |
| GET | `/system/admin/list` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/promote` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/revoke` | System Admin | T01 待对照 handler/权限/响应 |
| GET | `/system/admin/runtime/queues` | 系统管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/system/admin/runtime/queues/{queue}/archived` | System Admin | T01 待对照 handler/权限/响应 |
| GET | `/system/admin/runtime/queues/{queue}/tasks` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/runtime/queues/{queue}/tasks/{task_id}/actions/{action}` | System Admin | T01 待对照 handler/权限/响应 |
| DELETE | `/system/admin/settings/{key}` | System Admin | T01 待对照 handler/权限/响应 |
| GET | `/system/admin/settings/{key}` | System Admin | T01 待对照 handler/权限/响应 |
| PUT | `/system/admin/settings/{key}` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/tenants/apply-default-storage-quota` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/users/create` | System Admin | T01 待对照 handler/权限/响应 |
| POST | `/system/admin/users/reset-password` | System Admin | T01 待对照 handler/权限/响应 |
| GET | `/system/capabilities` | 系统 | T01 待对照 handler/权限/响应 |
| POST | `/system/docreader/reconnect` | 系统 | T01 待对照 handler/权限/响应 |
| GET | `/system/info` | 系统 | T01 待对照 handler/权限/响应 |
| GET | `/system/parser-engines` | 系统 | T01 待对照 handler/权限/响应 |
| POST | `/system/parser-engines/check` | 系统 | T01 待对照 handler/权限/响应 |
| POST | `/system/sandbox-check` | 系统 | T01 待对照 handler/权限/响应 |
| POST | `/system/storage-engine-check` | 系统 | T01 待对照 handler/权限/响应 |
| GET | `/system/storage-engine-status` | 系统 | T01 待对照 handler/权限/响应 |
| GET | `/tenants` | 空间管理 | T01 待对照 handler/权限/响应 |
| POST | `/tenants` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/all` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/kv/prompt-templates` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/kv/web-search-config` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/kv/{key}` | 空间管理 | T01 待对照 handler/权限/响应 |
| PUT | `/tenants/kv/{key}` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/search` | 空间管理 | T01 待对照 handler/权限/响应 |
| DELETE | `/tenants/{id}` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/{id}` | 空间管理 | T01 待对照 handler/权限/响应 |
| PUT | `/tenants/{id}` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/{id}/api-principal-config` | 空间管理 | T01 待对照 handler/权限/响应 |
| PUT | `/tenants/{id}/api-principal-config` | 空间管理 | T01 待对照 handler/权限/响应 |
| POST | `/tenants/{id}/api-principal-test-token` | 空间管理 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/{id}/audit-log` | 审计日志 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/{id}/invitations` | 空间邀请 | T01 待对照 handler/权限/响应 |
| POST | `/tenants/{id}/invitations` | 空间邀请 | T01 待对照 handler/权限/响应 |
| DELETE | `/tenants/{id}/invitations/{inv_id}` | 空间邀请 | T01 待对照 handler/权限/响应 |
| POST | `/tenants/{id}/invite-links` | 空间邀请 | T01 待对照 handler/权限/响应 |
| POST | `/tenants/{id}/leave` | 空间成员 | T01 待对照 handler/权限/响应 |
| GET | `/tenants/{id}/members` | 空间成员 | T01 待对照 handler/权限/响应 |
| POST | `/tenants/{id}/members` | 空间成员 | T01 待对照 handler/权限/响应 |
| DELETE | `/tenants/{id}/members/{user_id}` | 空间成员 | T01 待对照 handler/权限/响应 |
| PUT | `/tenants/{id}/members/{user_id}` | 空间成员 | T01 待对照 handler/权限/响应 |
| GET | `/user/favorites` | User | T01 待对照 handler/权限/响应 |
| POST | `/user/favorites` | User | T01 待对照 handler/权限/响应 |
| DELETE | `/user/favorites/{type}/{id}` | User | T01 待对照 handler/权限/响应 |
| GET | `/vector-stores` | VectorStore | T01 待对照 handler/权限/响应 |
| POST | `/vector-stores` | VectorStore | T01 待对照 handler/权限/响应 |
| POST | `/vector-stores/test` | VectorStore | T01 待对照 handler/权限/响应 |
| GET | `/vector-stores/types` | VectorStore | T01 待对照 handler/权限/响应 |
| DELETE | `/vector-stores/{id}` | VectorStore | T01 待对照 handler/权限/响应 |
| GET | `/vector-stores/{id}` | VectorStore | T01 待对照 handler/权限/响应 |
| PUT | `/vector-stores/{id}` | VectorStore | T01 待对照 handler/权限/响应 |
| POST | `/vector-stores/{id}/test` | VectorStore | T01 待对照 handler/权限/响应 |
| POST | `/web-search-providers/test` | 网络搜索 | T01 待对照 handler/权限/响应 |
| GET | `/web-search-providers/types` | 网络搜索 | T01 待对照 handler/权限/响应 |
| DELETE | `/web-search-providers/{id}` | 网络搜索 | T01 待对照 handler/权限/响应 |
| GET | `/web-search-providers/{id}` | 网络搜索 | T01 待对照 handler/权限/响应 |
| PUT | `/web-search-providers/{id}` | 网络搜索 | T01 待对照 handler/权限/响应 |
| POST | `/web-search-providers/{id}/test` | 网络搜索 | T01 待对照 handler/权限/响应 |
| GET | `/web-search/providers` | 网络搜索 | T01 待对照 handler/权限/响应 |
| POST | `/wechat/qrcode` | IM 渠道 | T01 待对照 handler/权限/响应 |
| POST | `/wechat/qrcode/status` | IM 渠道 | T01 待对照 handler/权限/响应 |
| POST | `/weknoracloud/credentials` | WeKnoraCloud | T01 待对照 handler/权限/响应 |

## 补充调查项

- 路由注册与文档双向对照，包括 /files、SSE、WS、Embed、旧 URL 重定向。
- Go SDK、Python client、miniprogram、packages/dsh-weknora、网站文档对现有 API 的使用面。
- Desktop 全部 bridge 调用、Go 本地数据/更新行为。
- 所有没有独立路由的设置 section、弹层、工具结果和右侧面板。
- 纯函数依赖递归检查，不仅查直接 import；单独区分 DOM parser/render 与无平台状态机。
