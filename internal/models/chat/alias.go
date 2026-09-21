// Pass A compatibility alias for github.com/Tencent/WeKnora/internal/modules/airesource/models/chat — zero logic. Deleted by Pass B task B-airesource.
package chat

import "github.com/Tencent/WeKnora/internal/modules/airesource/models/chat"

// Constants forwarded to the moved package.
const CacheRetentionLong = chat.CacheRetentionLong
const CacheRetentionNone = chat.CacheRetentionNone
const CacheRetentionShort = chat.CacheRetentionShort
const ExtraConfigThinkingControl = chat.ExtraConfigThinkingControl
const MessageKindCompactionSummary = chat.MessageKindCompactionSummary

// Type aliases to the moved package.
type AnthropicChat = chat.AnthropicChat
type CacheRetention = chat.CacheRetention
type Chat = chat.Chat
type ChatConfig = chat.ChatConfig
type ChatOptions = chat.ChatOptions
type FunctionCall = chat.FunctionCall
type FunctionDef = chat.FunctionDef
type ImageURL = chat.ImageURL
type Message = chat.Message
type MessageContentPart = chat.MessageContentPart
type MessageKind = chat.MessageKind
type OllamaChat = chat.OllamaChat
type QwenChatCompletionRequest = chat.QwenChatCompletionRequest
type RemoteAPIChat = chat.RemoteAPIChat
type SSEEvent = chat.SSEEvent
type SSEReader = chat.SSEReader
type ThinkingChatCompletionRequest = chat.ThinkingChatCompletionRequest
type ThinkingConfig = chat.ThinkingConfig
type ThinkingStrategy = chat.ThinkingStrategy
type Tool = chat.Tool
type ToolCall = chat.ToolCall
type UsageFactInput = chat.UsageFactInput

// Variables forwarded to the moved package.
var LocalImageResolver = chat.LocalImageResolver

// Functions forwarded to the moved package (function values; zero logic).
var BuildPromptCacheKey = chat.BuildPromptCacheKey
var ConfigFromModel = chat.ConfigFromModel
var EffectiveThinkingControl = chat.EffectiveThinkingControl
var FingerprintPromptPrefix = chat.FingerprintPromptPrefix
var NewAnthropicChat = chat.NewAnthropicChat
var NewChat = chat.NewChat
var NewOllamaChat = chat.NewOllamaChat
var NewRemoteAPIChat = chat.NewRemoteAPIChat
var NewRemoteChat = chat.NewRemoteChat
var NewSSEReader = chat.NewSSEReader
var PromptPrefixFingerprint = chat.PromptPrefixFingerprint
var UsageFactFromTokenUsage = chat.UsageFactFromTokenUsage
