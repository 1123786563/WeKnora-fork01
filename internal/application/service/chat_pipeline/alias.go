// Pass A compatibility alias for internal/modules/conversation/chat_pipeline — zero logic. Deleted by Pass B task B-conversation.
package chatpipeline

import chat_pipeline "github.com/Tencent/WeKnora/internal/modules/conversation/chat_pipeline"

// Variables and functions forwarded to the moved package.
var NewEventManager = chat_pipeline.NewEventManager
var NewPluginChatCompletion = chat_pipeline.NewPluginChatCompletion
var NewPluginChatCompletionStream = chat_pipeline.NewPluginChatCompletionStream
var NewPluginDataAnalysis = chat_pipeline.NewPluginDataAnalysis
var NewPluginExtractEntity = chat_pipeline.NewPluginExtractEntity
var NewPluginFilterTopK = chat_pipeline.NewPluginFilterTopK
var NewPluginIntoChatMessage = chat_pipeline.NewPluginIntoChatMessage
var NewPluginLoadHistory = chat_pipeline.NewPluginLoadHistory
var NewPluginMemoryAffinity = chat_pipeline.NewPluginMemoryAffinity
var NewPluginMemoryRecall = chat_pipeline.NewPluginMemoryRecall
var NewPluginMerge = chat_pipeline.NewPluginMerge
var NewPluginQueryUnderstand = chat_pipeline.NewPluginQueryUnderstand
var NewPluginRerank = chat_pipeline.NewPluginRerank
var NewPluginSearch = chat_pipeline.NewPluginSearch
var NewPluginSearchEntity = chat_pipeline.NewPluginSearchEntity
var NewPluginSearchParallel = chat_pipeline.NewPluginSearchParallel
var NewPluginWebFetch = chat_pipeline.NewPluginWebFetch
var NewPluginWikiBoost = chat_pipeline.NewPluginWikiBoost
