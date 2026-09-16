// Contextual-guide copy table, ported BYTE-EXACT from the Vue baseline:
//   frontend/src/i18n/locales/{zh-CN,en-US,ja-JP,ko-KR,ru-RU}.ts (contextualGuide block)
//
// Generated from those locale sources (every string is the exact locale-file
// literal) so the React client renders the same step bodies under the SAME
// vue-i18n key names (contextualGuide.<tour>.steps.<key>.title|desc).
// packages/views must not depend on @weknora/i18n — this local table follows
// the guides/steps.ts + chat/messages.ts precedent; swapping to
// formatMessage is a drop-in once packages/i18n grows the block.
//
// vue-i18n message syntax survives verbatim: named params ({current},
// {total}) are interpolated by renderContextualGuideMessage and the literal
// escapes ({'@'} in chat.steps.kb.desc / kbDetail.steps.done.desc) resolve to
// the bare character exactly like the vue-i18n message compiler does.
//
// Backfill note (2026-09-13): the shared contextualGuide.* block now lives in
// packages/i18n/src/generated/contextualGuide.ts and is byte-pinned against
// these locale sources by packages/i18n/test/contextualGuide.test.ts, which
// also byte-pins THIS table to the shared bundle (R007 precedent: two copies,
// Vue authoritative, drift is fixed on both sides). Shared-first layering of
// renderContextualGuideMessage on formatMessage (same shape as guides/steps.ts
// guideMessage) remains open follow-up work; this table stays the rendering
// source until then.
import type { NewUserGuideLocale } from './steps.ts';

/** Same five locales the Vue client ships (guides/steps.ts union). */
export type ContextualGuideLocale = NewUserGuideLocale;

export type ContextualGuideMessageKey =
  | 'contextualGuide.agentCreate.steps.agentType.desc'
  | 'contextualGuide.agentCreate.steps.agentType.title'
  | 'contextualGuide.agentCreate.steps.knowledge.desc'
  | 'contextualGuide.agentCreate.steps.knowledge.title'
  | 'contextualGuide.agentCreate.steps.mode.desc'
  | 'contextualGuide.agentCreate.steps.mode.title'
  | 'contextualGuide.agentCreate.steps.model.desc'
  | 'contextualGuide.agentCreate.steps.model.title'
  | 'contextualGuide.agentCreate.steps.multimodal.desc'
  | 'contextualGuide.agentCreate.steps.multimodal.title'
  | 'contextualGuide.agentCreate.steps.name.desc'
  | 'contextualGuide.agentCreate.steps.name.title'
  | 'contextualGuide.agentCreate.steps.navKnowledge.desc'
  | 'contextualGuide.agentCreate.steps.navKnowledge.title'
  | 'contextualGuide.agentCreate.steps.navModel.desc'
  | 'contextualGuide.agentCreate.steps.navModel.title'
  | 'contextualGuide.agentCreate.steps.navMultimodal.desc'
  | 'contextualGuide.agentCreate.steps.navMultimodal.title'
  | 'contextualGuide.agentCreate.steps.navTools.desc'
  | 'contextualGuide.agentCreate.steps.navTools.title'
  | 'contextualGuide.agentCreate.steps.navWebsearch.desc'
  | 'contextualGuide.agentCreate.steps.navWebsearch.title'
  | 'contextualGuide.agentCreate.steps.submit.desc'
  | 'contextualGuide.agentCreate.steps.submit.title'
  | 'contextualGuide.agentList.steps.create.desc'
  | 'contextualGuide.agentList.steps.create.title'
  | 'contextualGuide.chat.steps.done.desc'
  | 'contextualGuide.chat.steps.done.title'
  | 'contextualGuide.chat.steps.input.desc'
  | 'contextualGuide.chat.steps.input.title'
  | 'contextualGuide.chat.steps.kb.desc'
  | 'contextualGuide.chat.steps.kb.title'
  | 'contextualGuide.chat.steps.send.desc'
  | 'contextualGuide.chat.steps.send.title'
  | 'contextualGuide.done'
  | 'contextualGuide.interactHint'
  | 'contextualGuide.kbCreate.steps.chunking.desc'
  | 'contextualGuide.kbCreate.steps.chunking.title'
  | 'contextualGuide.kbCreate.steps.embedding.desc'
  | 'contextualGuide.kbCreate.steps.embedding.title'
  | 'contextualGuide.kbCreate.steps.faq.desc'
  | 'contextualGuide.kbCreate.steps.faq.title'
  | 'contextualGuide.kbCreate.steps.indexing.desc'
  | 'contextualGuide.kbCreate.steps.indexing.title'
  | 'contextualGuide.kbCreate.steps.llm.desc'
  | 'contextualGuide.kbCreate.steps.llm.title'
  | 'contextualGuide.kbCreate.steps.multimodalToggle.desc'
  | 'contextualGuide.kbCreate.steps.multimodalToggle.title'
  | 'contextualGuide.kbCreate.steps.multimodalVllm.desc'
  | 'contextualGuide.kbCreate.steps.multimodalVllm.title'
  | 'contextualGuide.kbCreate.steps.name.desc'
  | 'contextualGuide.kbCreate.steps.name.title'
  | 'contextualGuide.kbCreate.steps.navModels.desc'
  | 'contextualGuide.kbCreate.steps.navModels.title'
  | 'contextualGuide.kbCreate.steps.navMultimodal.desc'
  | 'contextualGuide.kbCreate.steps.navMultimodal.title'
  | 'contextualGuide.kbCreate.steps.parser.desc'
  | 'contextualGuide.kbCreate.steps.parser.title'
  | 'contextualGuide.kbCreate.steps.storage.desc'
  | 'contextualGuide.kbCreate.steps.storage.title'
  | 'contextualGuide.kbCreate.steps.submit.desc'
  | 'contextualGuide.kbCreate.steps.submit.title'
  | 'contextualGuide.kbCreate.steps.type.desc'
  | 'contextualGuide.kbCreate.steps.type.title'
  | 'contextualGuide.kbDetail.steps.done.desc'
  | 'contextualGuide.kbDetail.steps.done.title'
  | 'contextualGuide.kbDetail.steps.intro.desc'
  | 'contextualGuide.kbDetail.steps.intro.title'
  | 'contextualGuide.kbDetail.steps.upload.desc'
  | 'contextualGuide.kbDetail.steps.upload.title'
  | 'contextualGuide.kbList.steps.create.desc'
  | 'contextualGuide.kbList.steps.create.title'
  | 'contextualGuide.next'
  | 'contextualGuide.prev'
  | 'contextualGuide.skip'
  | 'contextualGuide.stepOf'
  | 'contextualGuide.tenantModels.needChatModelFirst'
  | 'contextualGuide.tenantModels.steps.addModel.desc'
  | 'contextualGuide.tenantModels.steps.addModel.title'
  | 'contextualGuide.tenantModels.steps.done.desc'
  | 'contextualGuide.tenantModels.steps.done.title'
  | 'contextualGuide.tenantModels.steps.intro.desc'
  | 'contextualGuide.tenantModels.steps.intro.title'
  | 'contextualGuide.tenantModels.stepsAgent.addModel.desc'
  | 'contextualGuide.tenantModels.stepsAgent.addModel.title'
  | 'contextualGuide.tenantModels.stepsAgent.done.desc'
  | 'contextualGuide.tenantModels.stepsAgent.done.title'
  | 'contextualGuide.tenantModels.stepsAgent.intro.desc'
  | 'contextualGuide.tenantModels.stepsAgent.intro.title';

export const CONTEXTUAL_GUIDE_MESSAGES: Record<ContextualGuideLocale, Record<ContextualGuideMessageKey, string>> = {
  'zh-CN': {
    'contextualGuide.agentCreate.steps.agentType.desc': '预设类型会自动填充系统提示词、推荐工具与知识库范围（如 Wiki 构建、数据分析等）。可按场景切换，名称与描述会随之更新。',
    'contextualGuide.agentCreate.steps.agentType.title': '选择智能体类型',
    'contextualGuide.agentCreate.steps.knowledge.desc': '「全部」适合通用助手；「指定」可限定专业领域；「不关联」则仅依赖模型自身能力或联网搜索。',
    'contextualGuide.agentCreate.steps.knowledge.title': '知识库范围',
    'contextualGuide.agentCreate.steps.mode.desc': '「普通模式」适合固定流程的快速问答；「智能推理」可调用工具、多步思考，适合复杂任务。',
    'contextualGuide.agentCreate.steps.mode.title': '选择运行模式',
    'contextualGuide.agentCreate.steps.model.desc': '从下拉列表选择已配置的对话模型；没有合适模型时请先到系统设置添加。',
    'contextualGuide.agentCreate.steps.model.title': '选择模型',
    'contextualGuide.agentCreate.steps.multimodal.desc': '可分别配置图片、音频上传及附件解析策略；启用图片上传时需在下方选择 VLM 模型。',
    'contextualGuide.agentCreate.steps.multimodal.title': '启用附件上传',
    'contextualGuide.agentCreate.steps.name.desc': '取一个易识别的名称。智能推理模式下系统可能已预填默认名称，可按需修改。',
    'contextualGuide.agentCreate.steps.name.title': '命名与描述',
    'contextualGuide.agentCreate.steps.navKnowledge.desc': '决定智能体可检索哪些知识。默认「全部知识库」，也可改为指定库或暂不关联。',
    'contextualGuide.agentCreate.steps.navKnowledge.title': '关联知识库',
    'contextualGuide.agentCreate.steps.navModel.desc': '每个智能体必须指定一个 KnowledgeQA 模型作为推理引擎。',
    'contextualGuide.agentCreate.steps.navModel.title': '绑定对话模型',
    'contextualGuide.agentCreate.steps.navMultimodal.desc': '开启后，对话中可上传图片、文档、音频等附件；图片理解需先在系统设置中配置 VLLM 模型。',
    'contextualGuide.agentCreate.steps.navMultimodal.title': '附件上传（可选）',
    'contextualGuide.agentCreate.steps.navTools.desc': '智能推理模式下可勾选内置工具、MCP 服务等，扩展搜索、计算等能力。',
    'contextualGuide.agentCreate.steps.navTools.title': '工具与 MCP（可选）',
    'contextualGuide.agentCreate.steps.navWebsearch.desc': '配置是否允许智能体调用外部搜索引擎补充实时信息。',
    'contextualGuide.agentCreate.steps.navWebsearch.title': '联网搜索（可选）',
    'contextualGuide.agentCreate.steps.submit.desc': '确认配置后点击高亮的「确定」完成创建，即可在对话中选择该智能体。',
    'contextualGuide.agentCreate.steps.submit.title': '保存智能体',
    'contextualGuide.agentList.steps.create.desc': '智能体把模型、知识库、工具与提示词组合成可复用的对话助手。点击下方高亮的「创建智能体」开始配置。',
    'contextualGuide.agentList.steps.create.title': '创建你的智能体',
    'contextualGuide.chat.steps.done.desc': '试试提一个与已上传文档相关的问题，体验带引用的精准回答。',
    'contextualGuide.chat.steps.done.title': '开始探索吧',
    'contextualGuide.chat.steps.input.desc': '直接描述你想了解的内容；也可点击上方推荐问题快速开始。',
    'contextualGuide.chat.steps.input.title': '输入你的问题',
    'contextualGuide.chat.steps.kb.desc': '点击 {\'@\'} 可指定一个或多个知识库/文件，仅基于选中内容回答；不选则按当前智能体配置检索。',
    'contextualGuide.chat.steps.kb.title': '选择知识范围',
    'contextualGuide.chat.steps.send.desc': '发送后将创建新会话，AI 会结合知识库内容作答，并标注引用片段。',
    'contextualGuide.chat.steps.send.title': '发送开始对话',
    'contextualGuide.done': '知道了',
    'contextualGuide.interactHint': '请直接点击高亮区域继续',
    'contextualGuide.kbCreate.steps.chunking.desc': '决定文档如何切分为检索片段。默认分块大小已针对 RAG 优化，一般无需修改。',
    'contextualGuide.kbCreate.steps.chunking.title': '分块策略（可选）',
    'contextualGuide.kbCreate.steps.embedding.desc': '将文本转为向量以支持语义检索。与上方索引策略中的「向量/关键词检索」配合使用。',
    'contextualGuide.kbCreate.steps.embedding.title': 'Embedding 模型',
    'contextualGuide.kbCreate.steps.faq.desc': '配置问答对的索引模式。创建后可在本页继续添加 FAQ 条目。',
    'contextualGuide.kbCreate.steps.faq.title': 'FAQ 索引方式',
    'contextualGuide.kbCreate.steps.indexing.desc': '默认已开启向量与关键词检索，可按需开启 Wiki 结构化索引或知识图谱。至少保留一种检索方式。',
    'contextualGuide.kbCreate.steps.indexing.title': '选择索引能力',
    'contextualGuide.kbCreate.steps.llm.desc': '用于文档摘要、问答生成等。若列表为空，请通过下拉菜单前往系统设置添加模型。',
    'contextualGuide.kbCreate.steps.llm.title': '对话 / 摘要模型',
    'contextualGuide.kbCreate.steps.multimodalToggle.desc': '启用后，上传的图片类文档将使用视觉语言模型提取内容，便于检索与问答。',
    'contextualGuide.kbCreate.steps.multimodalToggle.title': '开启多模态解析',
    'contextualGuide.kbCreate.steps.multimodalVllm.desc': '开启多模态后需指定 VLM（视觉语言模型）。若列表为空，请先在系统设置中添加 VLLM 类型模型。',
    'contextualGuide.kbCreate.steps.multimodalVllm.title': '选择 VLM 模型',
    'contextualGuide.kbCreate.steps.name.desc': '取一个便于识别的名称，例如「产品手册」或「客服 FAQ」。描述可选填。',
    'contextualGuide.kbCreate.steps.name.title': '填写名称',
    'contextualGuide.kbCreate.steps.navModels.desc': '知识库必须绑定对话模型；开启检索时还需 Embedding 模型。点击左侧「模型配置」进入。',
    'contextualGuide.kbCreate.steps.navModels.title': '配置模型（必填）',
    'contextualGuide.kbCreate.steps.navMultimodal.desc': '若文档含大量图表、扫描件或需理解图片内容，可在此开启多模态并选择 VLM 模型。',
    'contextualGuide.kbCreate.steps.navMultimodal.title': '多模态 / 图片理解（可选）',
    'contextualGuide.kbCreate.steps.parser.desc': '控制 PDF、Office 等文件的解析方式。默认配置适用于大多数场景，有 OCR 需求时可在此调整。',
    'contextualGuide.kbCreate.steps.parser.title': '解析引擎（可选）',
    'contextualGuide.kbCreate.steps.storage.desc': '原始文件存放位置（本地或对象存储）。默认跟随空间设置即可。',
    'contextualGuide.kbCreate.steps.storage.title': '存储引擎（可选）',
    'contextualGuide.kbCreate.steps.submit.desc': '点击「知道了」结束引导，无需现在创建知识库。准备好后，填写名称、确认类型与模型，再点击「创建」。',
    'contextualGuide.kbCreate.steps.submit.title': '创建知识库',
    'contextualGuide.kbCreate.steps.type.desc': '文档库适合上传 PDF、Word 等文件；FAQ 库适合问答对。创建后类型不可更改，请按需选择。',
    'contextualGuide.kbCreate.steps.type.title': '选择知识库类型',
    'contextualGuide.kbDetail.steps.done.desc': '文档解析入库后，可在对话中 {\'@\'} 本知识库提问，回答会附带引用来源。',
    'contextualGuide.kbDetail.steps.done.title': '解析完成后即可使用',
    'contextualGuide.kbDetail.steps.intro.desc': '添加第一份资料后，才能基于它进行检索与对话。支持拖拽上传多种文档格式。',
    'contextualGuide.kbDetail.steps.intro.title': '知识库还是空的',
    'contextualGuide.kbDetail.steps.upload.desc': '点击此处上传文件、文件夹，或导入网页与在线编辑内容。',
    'contextualGuide.kbDetail.steps.upload.title': '添加文档',
    'contextualGuide.kbList.steps.create.desc': '知识库用来存放文档与 FAQ。点击下方高亮的「新建知识库」按钮，我们会带你完成表单填写。',
    'contextualGuide.kbList.steps.create.title': '创建第一个知识库',
    'contextualGuide.next': '下一步',
    'contextualGuide.prev': '上一步',
    'contextualGuide.skip': '跳过',
    'contextualGuide.stepOf': '{current} / {total}',
    'contextualGuide.tenantModels.needChatModelFirst': '请先添加对话模型（KnowledgeQA），再创建智能体。',
    'contextualGuide.tenantModels.steps.addModel.desc': '点击「添加模型」，分别配置 KnowledgeQA（对话）与 Embedding 类型。Lite 用户可使用 Ollama 拉取本地模型。',
    'contextualGuide.tenantModels.steps.addModel.title': '添加模型',
    'contextualGuide.tenantModels.steps.done.desc': '模型保存后关闭设置页，即可点击「新建知识库」；创建向导会引导你完成类型、索引与模型绑定。',
    'contextualGuide.tenantModels.steps.done.title': '添加完成后继续',
    'contextualGuide.tenantModels.steps.intro.desc': '创建文档知识库至少需要：一个对话模型（用于摘要与问答）和一个 Embedding 模型（用于向量检索）。请先在系统设置中添加。',
    'contextualGuide.tenantModels.steps.intro.title': '需要先配置模型',
    'contextualGuide.tenantModels.stepsAgent.addModel.desc': '点击「添加模型」，选择 KnowledgeQA 类型并填写接入信息。',
    'contextualGuide.tenantModels.stepsAgent.addModel.title': '添加对话模型',
    'contextualGuide.tenantModels.stepsAgent.done.desc': '保存并关闭设置后，点击「创建智能体」，向导会带你配置模式、知识库与附件上传等选项。',
    'contextualGuide.tenantModels.stepsAgent.done.title': '然后创建智能体',
    'contextualGuide.tenantModels.stepsAgent.intro.desc': '创建智能体至少需要一种 KnowledgeQA（对话）模型。请先在系统设置中添加；Embedding 模型仅创建知识库时需要。',
    'contextualGuide.tenantModels.stepsAgent.intro.title': '需要先配置对话模型',
  },
  'en-US': {
    'contextualGuide.agentCreate.steps.agentType.desc': 'Presets fill in the system prompt, recommended tools, and knowledge scope (e.g. Wiki builder, data analysis). Switch by scenario—name and description update accordingly.',
    'contextualGuide.agentCreate.steps.agentType.title': 'Choose an agent type',
    'contextualGuide.agentCreate.steps.knowledge.desc': '"All" for general assistants; "Selected" for a domain; "None" relies on the model alone or web search.',
    'contextualGuide.agentCreate.steps.knowledge.title': 'Knowledge scope',
    'contextualGuide.agentCreate.steps.mode.desc': '"Quick answer" for straightforward Q&A; "Smart reasoning" uses tools and multi-step thinking for complex tasks.',
    'contextualGuide.agentCreate.steps.mode.title': 'Choose run mode',
    'contextualGuide.agentCreate.steps.model.desc': 'Choose from configured chat models, or add one in system settings first.',
    'contextualGuide.agentCreate.steps.model.title': 'Select model',
    'contextualGuide.agentCreate.steps.multimodal.desc': 'Configure image and audio upload plus attachment parse rules; select a VLM below when image upload is enabled.',
    'contextualGuide.agentCreate.steps.multimodal.title': 'Enable attachment upload',
    'contextualGuide.agentCreate.steps.name.desc': 'Pick a recognizable name. Smart-reasoning mode may pre-fill a default you can edit.',
    'contextualGuide.agentCreate.steps.name.title': 'Name and description',
    'contextualGuide.agentCreate.steps.navKnowledge.desc': 'Control which knowledge the agent can retrieve. Default is all knowledge bases.',
    'contextualGuide.agentCreate.steps.navKnowledge.title': 'Link knowledge bases',
    'contextualGuide.agentCreate.steps.navModel.desc': 'Every agent needs a KnowledgeQA model as its reasoning engine.',
    'contextualGuide.agentCreate.steps.navModel.title': 'Bind a chat model',
    'contextualGuide.agentCreate.steps.navMultimodal.desc': 'Lets users send images, documents, and audio in chat; image understanding requires a VLM in system settings.',
    'contextualGuide.agentCreate.steps.navMultimodal.title': 'Attachment upload (optional)',
    'contextualGuide.agentCreate.steps.navTools.desc': 'In smart-reasoning mode, enable built-in tools and MCP services for search, code, and more.',
    'contextualGuide.agentCreate.steps.navTools.title': 'Tools & MCP (optional)',
    'contextualGuide.agentCreate.steps.navWebsearch.desc': 'Allow the agent to call external search for up-to-date information.',
    'contextualGuide.agentCreate.steps.navWebsearch.title': 'Web search (optional)',
    'contextualGuide.agentCreate.steps.submit.desc': 'Click the highlighted confirm button to finish. You can then select this agent in chat.',
    'contextualGuide.agentCreate.steps.submit.title': 'Save the agent',
    'contextualGuide.agentList.steps.create.desc': 'Agents combine models, knowledge bases, tools, and prompts into reusable assistants. Click the highlighted "Create agent" button.',
    'contextualGuide.agentList.steps.create.title': 'Create your agent',
    'contextualGuide.chat.steps.done.desc': 'Try a question related to your uploaded documents and see grounded answers with references.',
    'contextualGuide.chat.steps.done.title': 'You are ready to explore',
    'contextualGuide.chat.steps.input.desc': 'Describe what you want to know, or click a suggested question above to get started quickly.',
    'contextualGuide.chat.steps.input.title': 'Type your question',
    'contextualGuide.chat.steps.kb.desc': 'Click {\'@\'} to pick one or more knowledge bases or files. Answers use only the selection; otherwise the current agent settings apply.',
    'contextualGuide.chat.steps.kb.title': 'Choose knowledge scope',
    'contextualGuide.chat.steps.send.desc': 'Sending creates a new session. The AI answers using your knowledge base and shows cited passages.',
    'contextualGuide.chat.steps.send.title': 'Send to start chatting',
    'contextualGuide.done': 'Got it',
    'contextualGuide.interactHint': 'Click the highlighted area to continue',
    'contextualGuide.kbCreate.steps.chunking.desc': 'How documents are split for retrieval. Default chunk sizes are tuned for RAG and rarely need changes.',
    'contextualGuide.kbCreate.steps.chunking.title': 'Chunking (optional)',
    'contextualGuide.kbCreate.steps.embedding.desc': 'Turns text into vectors for semantic search. Works with vector/keyword indexing above.',
    'contextualGuide.kbCreate.steps.embedding.title': 'Embedding model',
    'contextualGuide.kbCreate.steps.faq.desc': 'Choose how Q&A pairs are indexed. You can add FAQ entries after creation.',
    'contextualGuide.kbCreate.steps.faq.title': 'FAQ indexing',
    'contextualGuide.kbCreate.steps.indexing.desc': 'Vector and keyword search are on by default. You can also enable Wiki or knowledge-graph indexing. Keep at least one search mode enabled.',
    'contextualGuide.kbCreate.steps.indexing.title': 'Indexing capabilities',
    'contextualGuide.kbCreate.steps.llm.desc': 'Used for summaries and answers. If the list is empty, use the dropdown to open settings and add a model.',
    'contextualGuide.kbCreate.steps.llm.title': 'Chat / summary model',
    'contextualGuide.kbCreate.steps.multimodalToggle.desc': 'When on, image-bearing uploads are processed with a vision-language model for better retrieval.',
    'contextualGuide.kbCreate.steps.multimodalToggle.title': 'Enable multimodal parsing',
    'contextualGuide.kbCreate.steps.multimodalVllm.desc': 'Multimodal requires a VLM. Add one in system settings if the list is empty.',
    'contextualGuide.kbCreate.steps.multimodalVllm.title': 'Choose a VLM model',
    'contextualGuide.kbCreate.steps.name.desc': 'Pick a clear name such as "Product manual" or "Support FAQ". Description is optional.',
    'contextualGuide.kbCreate.steps.name.title': 'Enter a name',
    'contextualGuide.kbCreate.steps.navModels.desc': 'Every knowledge base needs a chat model; retrieval also requires an Embedding model. Open "Model configuration" on the left.',
    'contextualGuide.kbCreate.steps.navModels.title': 'Model setup (required)',
    'contextualGuide.kbCreate.steps.navMultimodal.desc': 'Enable this if documents contain charts, scans, or image-heavy content that needs vision understanding.',
    'contextualGuide.kbCreate.steps.navMultimodal.title': 'Multimodal / images (optional)',
    'contextualGuide.kbCreate.steps.parser.desc': 'How PDFs and Office files are parsed. Defaults work for most cases; adjust if you need OCR or special layouts.',
    'contextualGuide.kbCreate.steps.parser.title': 'Parser engine (optional)',
    'contextualGuide.kbCreate.steps.storage.desc': 'Where raw files are stored (local or object storage). The workspace default is usually fine.',
    'contextualGuide.kbCreate.steps.storage.title': 'Storage (optional)',
    'contextualGuide.kbCreate.steps.submit.desc': 'Click Got it to finish the guide without creating a knowledge base. When ready, enter a name, confirm the type and models, then click Create.',
    'contextualGuide.kbCreate.steps.submit.title': 'Create the knowledge base',
    'contextualGuide.kbCreate.steps.type.desc': 'Document bases are for PDFs, Word files, and similar uploads. FAQ bases are for question–answer pairs. The type cannot be changed later.',
    'contextualGuide.kbCreate.steps.type.title': 'Choose a type',
    'contextualGuide.kbDetail.steps.done.desc': 'Once documents are indexed, mention this knowledge base in chat with {\'@\'} to get answers with citations.',
    'contextualGuide.kbDetail.steps.done.title': 'Ready after parsing',
    'contextualGuide.kbDetail.steps.intro.desc': 'Add your first item so you can search and chat over it. You can also drag and drop supported file types.',
    'contextualGuide.kbDetail.steps.intro.title': 'This knowledge base is empty',
    'contextualGuide.kbDetail.steps.upload.desc': 'Use this menu to upload files or folders, import a URL, or create content online.',
    'contextualGuide.kbDetail.steps.upload.title': 'Add documents',
    'contextualGuide.kbList.steps.create.desc': 'Knowledge bases hold documents and FAQs. Click the highlighted "New knowledge base" button below and we will walk you through the form.',
    'contextualGuide.kbList.steps.create.title': 'Create your first knowledge base',
    'contextualGuide.next': 'Next',
    'contextualGuide.prev': 'Back',
    'contextualGuide.skip': 'Skip',
    'contextualGuide.stepOf': '{current} / {total}',
    'contextualGuide.tenantModels.needChatModelFirst': 'Add a chat model (KnowledgeQA) before creating an agent.',
    'contextualGuide.tenantModels.steps.addModel.desc': 'Click "Add model" and configure KnowledgeQA (chat) and Embedding types. Lite users can pull local models via Ollama.',
    'contextualGuide.tenantModels.steps.addModel.title': 'Add models',
    'contextualGuide.tenantModels.steps.done.desc': 'After saving models, close settings and click "New knowledge base". The wizard will walk you through type, indexing, and model binding.',
    'contextualGuide.tenantModels.steps.done.title': 'Then continue',
    'contextualGuide.tenantModels.steps.intro.desc': 'A document knowledge base needs at least one chat model (summaries and Q&A) and one Embedding model (vector search). Add them in system settings.',
    'contextualGuide.tenantModels.steps.intro.title': 'Configure models first',
    'contextualGuide.tenantModels.stepsAgent.addModel.desc': 'Click "Add model" and configure a KnowledgeQA type.',
    'contextualGuide.tenantModels.stepsAgent.addModel.title': 'Add a chat model',
    'contextualGuide.tenantModels.stepsAgent.done.desc': 'After saving, close settings and click "Create agent". The wizard covers mode, knowledge bases, and attachment upload options.',
    'contextualGuide.tenantModels.stepsAgent.done.title': 'Then create an agent',
    'contextualGuide.tenantModels.stepsAgent.intro.desc': 'Creating an agent requires at least one KnowledgeQA model. Add it in system settings (Embedding is only required for knowledge bases).',
    'contextualGuide.tenantModels.stepsAgent.intro.title': 'Configure a chat model first',
  },
  'ja-JP': {
    'contextualGuide.agentCreate.steps.agentType.desc': 'プリセットを選ぶと、システムプロンプト、推奨ツール、ナレッジ範囲が自動入力されます（例: Wiki作成、データ分析）。用途に応じて切り替えると、名前と説明も併せて更新されます。',
    'contextualGuide.agentCreate.steps.agentType.title': 'エージェントのタイプを選択',
    'contextualGuide.agentCreate.steps.knowledge.desc': '汎用アシスタントには「すべて」、特定領域には「指定」、「なし」の場合はモデル単体またはWeb検索に依存します。',
    'contextualGuide.agentCreate.steps.knowledge.title': 'ナレッジの範囲',
    'contextualGuide.agentCreate.steps.mode.desc': 'シンプルなQ&Aには「クイック回答」、ツールと多段階の思考が必要な複雑なタスクには「スマート推論」を使用します。',
    'contextualGuide.agentCreate.steps.mode.title': '実行モードを選択',
    'contextualGuide.agentCreate.steps.model.desc': '設定済みのチャットモデルから選択してください。無い場合は先にシステム設定で追加してください。',
    'contextualGuide.agentCreate.steps.model.title': 'モデルを選択',
    'contextualGuide.agentCreate.steps.multimodal.desc': '画像と音声のアップロードおよび添付ファイルの解析ルールを設定します。画像アップロードを有効にする場合は、下でVLMを選択してください。',
    'contextualGuide.agentCreate.steps.multimodal.title': '添付ファイルのアップロードを有効化',
    'contextualGuide.agentCreate.steps.name.desc': '分かりやすい名前を付けてください。スマート推論モードではデフォルトの名前が入力される場合があります。必要に応じて編集してください。',
    'contextualGuide.agentCreate.steps.name.title': '名前と説明',
    'contextualGuide.agentCreate.steps.navKnowledge.desc': 'エージェントが検索できるナレッジの範囲を指定します。デフォルトはすべてのナレッジベースです。',
    'contextualGuide.agentCreate.steps.navKnowledge.title': 'ナレッジベースを紐付け',
    'contextualGuide.agentCreate.steps.navModel.desc': 'すべてのエージェントには推論エンジンとしてKnowledgeQAモデルが必要です。',
    'contextualGuide.agentCreate.steps.navModel.title': 'チャットモデルを紐付け',
    'contextualGuide.agentCreate.steps.navMultimodal.desc': 'チャットで画像、ドキュメント、音声を送信できるようになります。画像理解にはシステム設定でVLMが必要です。',
    'contextualGuide.agentCreate.steps.navMultimodal.title': '添付ファイルのアップロード（任意）',
    'contextualGuide.agentCreate.steps.navTools.desc': 'スマート推論モードでは、検索やコード実行などの組み込みツールとMCPサービスを有効にできます。',
    'contextualGuide.agentCreate.steps.navTools.title': 'ツールとMCP（任意）',
    'contextualGuide.agentCreate.steps.navWebsearch.desc': 'エージェントが外部検索を呼び出して最新情報を取得できるようにします。',
    'contextualGuide.agentCreate.steps.navWebsearch.title': 'Web検索（任意）',
    'contextualGuide.agentCreate.steps.submit.desc': 'ハイライトされた確認ボタンをクリックして完了してください。その後、チャットでこのエージェントを選択できます。',
    'contextualGuide.agentCreate.steps.submit.title': 'エージェントを保存',
    'contextualGuide.agentList.steps.create.desc': 'エージェントはモデル、ナレッジベース、ツール、プロンプトを組み合わせた再利用可能なアシスタントです。ハイライトされた「エージェントを作成」ボタンをクリックしてください。',
    'contextualGuide.agentList.steps.create.title': 'エージェントを作成',
    'contextualGuide.chat.steps.done.desc': 'アップロードしたドキュメントに関する質問を試して、根拠付きの回答を確認してみましょう。',
    'contextualGuide.chat.steps.done.title': '準備完了',
    'contextualGuide.chat.steps.input.desc': '知りたいことを入力するか、上部の推奨質問をクリックするとすぐに始められます。',
    'contextualGuide.chat.steps.input.title': '質問を入力',
    'contextualGuide.chat.steps.kb.desc': '{\'@\'}をクリックして、ナレッジベースやファイルを1つ以上選択してください。選択した範囲のみが回答に使用され、未選択の場合は現在のエージェント設定が適用されます。',
    'contextualGuide.chat.steps.kb.title': 'ナレッジの範囲を選択',
    'contextualGuide.chat.steps.send.desc': '送信すると新しいセッションが作成されます。AIがナレッジベースを使って回答し、引用元の該当箇所を表示します。',
    'contextualGuide.chat.steps.send.title': '送信してチャットを開始',
    'contextualGuide.done': 'OK',
    'contextualGuide.interactHint': 'ハイライトされた箇所をクリックして続行してください',
    'contextualGuide.kbCreate.steps.chunking.desc': '検索用にドキュメントを分割する方法です。デフォルトのチャンクサイズはRAG向けに調整済みで、通常は変更不要です。',
    'contextualGuide.kbCreate.steps.chunking.title': 'チャンク分割（任意）',
    'contextualGuide.kbCreate.steps.embedding.desc': 'テキストをベクトル化して意味検索を可能にします。上記のベクトル／キーワードインデックスと連携します。',
    'contextualGuide.kbCreate.steps.embedding.title': '埋め込みモデル',
    'contextualGuide.kbCreate.steps.faq.desc': 'Q&Aペアのインデックス方法を選択してください。FAQ項目は作成後に追加できます。',
    'contextualGuide.kbCreate.steps.faq.title': 'FAQインデックス',
    'contextualGuide.kbCreate.steps.indexing.desc': 'ベクトル検索とキーワード検索はデフォルトで有効です。Wikiやナレッジグラフのインデックスも有効にできます。検索モードは少なくとも1つ有効にしてください。',
    'contextualGuide.kbCreate.steps.indexing.title': 'インデックス機能',
    'contextualGuide.kbCreate.steps.llm.desc': '要約と回答の生成に使用します。一覧が空の場合は、ドロップダウンから設定を開いてモデルを追加してください。',
    'contextualGuide.kbCreate.steps.llm.title': 'チャット／要約モデル',
    'contextualGuide.kbCreate.steps.multimodalToggle.desc': '有効にすると、画像を含むアップロードを視覚言語モデルで処理し、検索精度を高めます。',
    'contextualGuide.kbCreate.steps.multimodalToggle.title': 'マルチモーダル解析を有効化',
    'contextualGuide.kbCreate.steps.multimodalVllm.desc': 'マルチモーダルにはVLMが必要です。一覧が空の場合はシステム設定から追加してください。',
    'contextualGuide.kbCreate.steps.multimodalVllm.title': 'VLMモデルを選択',
    'contextualGuide.kbCreate.steps.name.desc': '「製品マニュアル」「サポートFAQ」のような分かりやすい名前を付けてください。説明は任意です。',
    'contextualGuide.kbCreate.steps.name.title': '名前を入力',
    'contextualGuide.kbCreate.steps.navModels.desc': 'ナレッジベースにはチャットモデルが必要で、検索にはさらに埋め込みモデルが必要です。左側の「モデル設定」を開いてください。',
    'contextualGuide.kbCreate.steps.navModels.title': 'モデル設定（必須）',
    'contextualGuide.kbCreate.steps.navMultimodal.desc': '図表やスキャン、画像が多く含まれ、視覚的な理解が必要なドキュメントの場合は有効にしてください。',
    'contextualGuide.kbCreate.steps.navMultimodal.title': 'マルチモーダル／画像（任意）',
    'contextualGuide.kbCreate.steps.parser.desc': 'PDFやOfficeファイルの解析方法です。ほとんどの場合はデフォルト値で問題ありません。OCRや特殊なレイアウトが必要な場合に調整してください。',
    'contextualGuide.kbCreate.steps.parser.title': '解析エンジン（任意）',
    'contextualGuide.kbCreate.steps.storage.desc': '元ファイルの保存先（ローカルまたはオブジェクトストレージ）です。通常はワークスペースのデフォルト値で問題ありません。',
    'contextualGuide.kbCreate.steps.storage.title': 'ストレージ（任意）',
    'contextualGuide.kbCreate.steps.submit.desc': 'ナレッジベースを作成せずにガイドを終了するには「OK」をクリックしてください。準備ができたら名前を入力し、タイプとモデルを確認して「作成」をクリックしてください。',
    'contextualGuide.kbCreate.steps.submit.title': 'ナレッジベースを作成',
    'contextualGuide.kbCreate.steps.type.desc': 'ドキュメント型はPDFやWordなどのアップロード向け、FAQ型は質問と回答のペア向けです。タイプは後から変更できません。',
    'contextualGuide.kbCreate.steps.type.title': 'タイプを選択',
    'contextualGuide.kbDetail.steps.done.desc': 'ドキュメントのインデックスが完了したら、チャットで{\'@\'}を使ってこのナレッジベースを指定すると、出典付きの回答が得られます。',
    'contextualGuide.kbDetail.steps.done.title': '解析後に利用可能',
    'contextualGuide.kbDetail.steps.intro.desc': '最初の項目を追加すると、検索やチャットで利用できます。対応形式のファイルをドラッグ＆ドロップすることもできます。',
    'contextualGuide.kbDetail.steps.intro.title': 'このナレッジベースは空です',
    'contextualGuide.kbDetail.steps.upload.desc': 'このメニューから、ファイルやフォルダのアップロード、URLのインポート、オンラインでの作成ができます。',
    'contextualGuide.kbDetail.steps.upload.title': 'ドキュメントを追加',
    'contextualGuide.kbList.steps.create.desc': 'ナレッジベースにはドキュメントやFAQを格納します。下のハイライトされた「ナレッジベースを作成」ボタンをクリックすると、入力手順をご案内します。',
    'contextualGuide.kbList.steps.create.title': '最初のナレッジベースを作成',
    'contextualGuide.next': '次へ',
    'contextualGuide.prev': '戻る',
    'contextualGuide.skip': 'スキップ',
    'contextualGuide.stepOf': '{current} / {total}',
    'contextualGuide.tenantModels.needChatModelFirst': 'エージェントを作成する前に、チャットモデル（KnowledgeQA）を追加してください。',
    'contextualGuide.tenantModels.steps.addModel.desc': '「モデルを追加」をクリックし、KnowledgeQA（チャット）とEmbeddingの両タイプを設定してください。LiteユーザはOllama経由でローカルモデルを取得できます。',
    'contextualGuide.tenantModels.steps.addModel.title': 'モデルを追加',
    'contextualGuide.tenantModels.steps.done.desc': 'モデルを保存したら設定を閉じ、「ナレッジベースを作成」をクリックしてください。ウィザードでタイプ、インデックス、モデルの紐付けを設定できます。',
    'contextualGuide.tenantModels.steps.done.title': '設定後に続行',
    'contextualGuide.tenantModels.steps.intro.desc': 'ドキュメント型ナレッジベースには、チャットモデル（要約とQ&A用）と埋め込みモデル（ベクトル検索用）が少なくとも1つずつ必要です。システム設定から追加してください。',
    'contextualGuide.tenantModels.steps.intro.title': 'まずモデルを設定',
    'contextualGuide.tenantModels.stepsAgent.addModel.desc': '「モデルを追加」をクリックし、KnowledgeQAタイプを設定してください。',
    'contextualGuide.tenantModels.stepsAgent.addModel.title': 'チャットモデルを追加',
    'contextualGuide.tenantModels.stepsAgent.done.desc': '保存したら設定を閉じ、「エージェントを作成」をクリックしてください。ウィザードでモード、ナレッジベース、添付ファイルのアップロード設定を行えます。',
    'contextualGuide.tenantModels.stepsAgent.done.title': '続いてエージェントを作成',
    'contextualGuide.tenantModels.stepsAgent.intro.desc': 'エージェントの作成にはKnowledgeQAモデルが少なくとも1つ必要です。システム設定から追加してください（Embeddingはナレッジベースにのみ必要です）。',
    'contextualGuide.tenantModels.stepsAgent.intro.title': 'まずチャットモデルを設定',
  },
  'ko-KR': {
    'contextualGuide.agentCreate.steps.agentType.desc': '프리셋은 시스템 프롬프트, 권장 도구, 지식 범위(Wiki 빌드, 데이터 분석 등)를 자동으로 채웁니다. 시나리오에 맞게 바꾸면 이름·설명도 함께 갱신됩니다.',
    'contextualGuide.agentCreate.steps.agentType.title': '에이전트 유형 선택',
    'contextualGuide.agentCreate.steps.knowledge.desc': '「전체」는 범용, 「선택」은 특정 도메인, 「없음」은 모델만 또는 웹 검색에 의존합니다.',
    'contextualGuide.agentCreate.steps.knowledge.title': '지식 범위',
    'contextualGuide.agentCreate.steps.mode.desc': '「빠른 답변」은 단순 Q&A용, 「스마트 추론」은 도구와 다단계 사고로 복잡한 작업에 적합합니다.',
    'contextualGuide.agentCreate.steps.mode.title': '실행 모드 선택',
    'contextualGuide.agentCreate.steps.model.desc': '구성된 대화 모델 중 선택하거나, 먼저 시스템 설정에서 추가하세요.',
    'contextualGuide.agentCreate.steps.model.title': '모델 선택',
    'contextualGuide.agentCreate.steps.multimodal.desc': '이미지·오디오 업로드와 첨부 파싱 규칙을 설정하세요. 이미지 업로드 시 아래에서 VLM을 선택해야 합니다.',
    'contextualGuide.agentCreate.steps.multimodal.title': '첨부 업로드 켜기',
    'contextualGuide.agentCreate.steps.name.desc': '알아보기 쉬운 이름을 입력하세요. 스마트 추론 모드는 기본값이 채워질 수 있습니다.',
    'contextualGuide.agentCreate.steps.name.title': '이름과 설명',
    'contextualGuide.agentCreate.steps.navKnowledge.desc': '에이전트가 검색할 지식 범위를 설정합니다. 기본값은 전체 지식 베이스입니다.',
    'contextualGuide.agentCreate.steps.navKnowledge.title': '지식 베이스 연결',
    'contextualGuide.agentCreate.steps.navModel.desc': '모든 에이전트에 KnowledgeQA 모델이 추론 엔진으로 필요합니다.',
    'contextualGuide.agentCreate.steps.navModel.title': '대화 모델 연결',
    'contextualGuide.agentCreate.steps.navMultimodal.desc': '채팅에서 이미지, 문서, 오디오 등 첨부를 허용합니다. 이미지 이해에는 시스템 설정의 VLM이 필요합니다.',
    'contextualGuide.agentCreate.steps.navMultimodal.title': '첨부 업로드(선택)',
    'contextualGuide.agentCreate.steps.navTools.desc': '스마트 추론 모드에서 내장 도구와 MCP로 검색·코드 등을 사용할 수 있습니다.',
    'contextualGuide.agentCreate.steps.navTools.title': '도구 및 MCP(선택)',
    'contextualGuide.agentCreate.steps.navWebsearch.desc': '최신 정보를 위해 외부 검색을 호출할 수 있게 합니다.',
    'contextualGuide.agentCreate.steps.navWebsearch.title': '웹 검색(선택)',
    'contextualGuide.agentCreate.steps.submit.desc': '강조된 확인 버튼을 눌러 완료하세요. 이후 채팅에서 이 에이전트를 선택할 수 있습니다.',
    'contextualGuide.agentCreate.steps.submit.title': '에이전트 저장',
    'contextualGuide.agentList.steps.create.desc': '에이전트는 모델·지식 베이스·도구·프롬프트를 묶은 재사용 가능한 어시스턴트입니다. 강조된 「에이전트 생성」을 클릭하세요.',
    'contextualGuide.agentList.steps.create.title': '에이전트 만들기',
    'contextualGuide.chat.steps.done.desc': '업로드한 문서와 관련된 질문을 해 보고, 인용이 포함된 답변을 확인해 보세요.',
    'contextualGuide.chat.steps.done.title': '이제 탐색해 보세요',
    'contextualGuide.chat.steps.input.desc': '알고 싶은 내용을 입력하거나, 위의 추천 질문을 눌러 빠르게 시작하세요.',
    'contextualGuide.chat.steps.input.title': '질문 입력',
    'contextualGuide.chat.steps.kb.desc': '{\'@\'}를 눌러 지식 베이스나 파일을 선택하세요. 선택한 범위만 사용해 답변합니다. 선택하지 않으면 현재 에이전트 설정이 적용됩니다.',
    'contextualGuide.chat.steps.kb.title': '지식 범위 선택',
    'contextualGuide.chat.steps.send.desc': '전송하면 새 세션이 만들어지고, AI가 지식 베이스를 바탕으로 인용과 함께 답변합니다.',
    'contextualGuide.chat.steps.send.title': '보내기로 대화 시작',
    'contextualGuide.done': '확인',
    'contextualGuide.interactHint': '강조된 영역을 클릭하여 계속하세요',
    'contextualGuide.kbCreate.steps.chunking.desc': '문서를 검색 단위로 나누는 방식입니다. RAG에 맞춘 기본값을 그대로 쓰면 됩니다.',
    'contextualGuide.kbCreate.steps.chunking.title': '청킹(선택)',
    'contextualGuide.kbCreate.steps.embedding.desc': '텍스트를 벡터로 바꿔 의미 검색을 지원합니다. 위의 벡터/키워드 색인과 함께 사용합니다.',
    'contextualGuide.kbCreate.steps.embedding.title': 'Embedding 모델',
    'contextualGuide.kbCreate.steps.faq.desc': 'Q&A 쌍의 색인 방식을 선택합니다. 생성 후 이 페이지에서 FAQ 항목을 추가할 수 있습니다.',
    'contextualGuide.kbCreate.steps.faq.title': 'FAQ 색인',
    'contextualGuide.kbCreate.steps.indexing.desc': '벡터·키워드 검색이 기본으로 켜져 있습니다. Wiki·지식 그래프도 선택할 수 있습니다. 검색 방식은 하나 이상 유지하세요.',
    'contextualGuide.kbCreate.steps.indexing.title': '색인 기능',
    'contextualGuide.kbCreate.steps.llm.desc': '요약·답변 생성에 사용됩니다. 목록이 비어 있으면 드롭다운에서 설정으로 이동해 모델을 추가하세요.',
    'contextualGuide.kbCreate.steps.llm.title': '대화/요약 모델',
    'contextualGuide.kbCreate.steps.multimodalToggle.desc': '켜면 이미지가 포함된 업로드를 VLM으로 처리해 검색 품질을 높입니다.',
    'contextualGuide.kbCreate.steps.multimodalToggle.title': '멀티모달 파싱 켜기',
    'contextualGuide.kbCreate.steps.multimodalVllm.desc': '멀티모달에는 VLM이 필요합니다. 목록이 비어 있으면 시스템 설정에서 추가하세요.',
    'contextualGuide.kbCreate.steps.multimodalVllm.title': 'VLM 모델 선택',
    'contextualGuide.kbCreate.steps.name.desc': '「제품 매뉴얼」「고객 FAQ」처럼 알아보기 쉬운 이름을 입력하세요. 설명은 선택 사항입니다.',
    'contextualGuide.kbCreate.steps.name.title': '이름 입력',
    'contextualGuide.kbCreate.steps.navModels.desc': '모든 지식 베이스에 대화 모델이 필요하며, 검색을 쓰려면 Embedding 모델도 필요합니다. 왼쪽 「모델 구성」을 여세요.',
    'contextualGuide.kbCreate.steps.navModels.title': '모델 구성(필수)',
    'contextualGuide.kbCreate.steps.navMultimodal.desc': '차트·스캔·이미지가 많은 문서를 시각 이해가 필요할 때 활성화하세요.',
    'contextualGuide.kbCreate.steps.navMultimodal.title': '멀티모달/이미지(선택)',
    'contextualGuide.kbCreate.steps.parser.desc': 'PDF·Office 파일 파싱 방식을 제어합니다. 기본값으로 대부분 충분하며 OCR이 필요할 때 조정하세요.',
    'contextualGuide.kbCreate.steps.parser.title': '파서 엔진(선택)',
    'contextualGuide.kbCreate.steps.storage.desc': '원본 파일 저장 위치(로컬 또는 객체 스토리지)입니다. 워크스페이스 기본값을 따르면 됩니다.',
    'contextualGuide.kbCreate.steps.storage.title': '스토리지(선택)',
    'contextualGuide.kbCreate.steps.submit.desc': '「확인」을 클릭하면 지식 베이스를 생성하지 않고 안내를 마칠 수 있습니다. 준비되면 이름을 입력하고 유형과 모델을 확인한 뒤 「생성」을 클릭하세요.',
    'contextualGuide.kbCreate.steps.submit.title': '지식 베이스 생성',
    'contextualGuide.kbCreate.steps.type.desc': '문서 라이브러리는 PDF·Word 등 파일용, FAQ 라이브러리는 질문·답변 쌍용입니다. 생성 후 유형은 변경할 수 없습니다.',
    'contextualGuide.kbCreate.steps.type.title': '유형 선택',
    'contextualGuide.kbDetail.steps.done.desc': '문서가 색인되면 대화에서 {\'@\'}로 이 지식 베이스를 지정해 출처가 포함된 답변을 받을 수 있습니다.',
    'contextualGuide.kbDetail.steps.done.title': '분석 후 사용 가능',
    'contextualGuide.kbDetail.steps.intro.desc': '첫 자료를 추가해야 검색과 대화에 사용할 수 있습니다. 지원 형식은 드래그 앤 드롭으로도 업로드할 수 있습니다.',
    'contextualGuide.kbDetail.steps.intro.title': '지식 베이스가 비어 있습니다',
    'contextualGuide.kbDetail.steps.upload.desc': '여기에서 파일·폴더 업로드, URL 가져오기, 온라인 편집 콘텐츠 생성을 할 수 있습니다.',
    'contextualGuide.kbDetail.steps.upload.title': '문서 추가',
    'contextualGuide.kbList.steps.create.desc': '지식 베이스에 문서와 FAQ를 보관합니다. 아래에 강조된 「새 지식 베이스」 버튼을 누르면 양식 작성을 안내합니다.',
    'contextualGuide.kbList.steps.create.title': '첫 지식 베이스 만들기',
    'contextualGuide.next': '다음',
    'contextualGuide.prev': '이전',
    'contextualGuide.skip': '건너뛰기',
    'contextualGuide.stepOf': '{current} / {total}',
    'contextualGuide.tenantModels.needChatModelFirst': '에이전트를 만들기 전에 대화 모델(KnowledgeQA)을 추가하세요.',
    'contextualGuide.tenantModels.steps.addModel.desc': '「모델 추가」를 눌러 KnowledgeQA(대화)와 Embedding 유형을 구성하세요. Lite 사용자는 Ollama로 로컬 모델을 받을 수 있습니다.',
    'contextualGuide.tenantModels.steps.addModel.title': '모델 추가',
    'contextualGuide.tenantModels.steps.done.desc': '모델을 저장하고 설정을 닫은 뒤 「새 지식 베이스」를 클릭하세요. 마법사가 유형·색인·모델 연결을 안내합니다.',
    'contextualGuide.tenantModels.steps.done.title': '추가 후 계속',
    'contextualGuide.tenantModels.steps.intro.desc': '문서 지식 베이스에는 대화 모델(요약·Q&A)과 Embedding 모델(벡터 검색)이 각각 하나 이상 필요합니다. 시스템 설정에서 추가하세요.',
    'contextualGuide.tenantModels.steps.intro.title': '먼저 모델을 구성하세요',
    'contextualGuide.tenantModels.stepsAgent.addModel.desc': '「모델 추가」를 눌러 KnowledgeQA 유형을 구성하세요.',
    'contextualGuide.tenantModels.stepsAgent.addModel.title': '대화 모델 추가',
    'contextualGuide.tenantModels.stepsAgent.done.desc': '저장 후 설정을 닫고 「에이전트 생성」을 클릭하세요. 마법사가 모드·지식 베이스·첨부 업로드 옵션을 안내합니다.',
    'contextualGuide.tenantModels.stepsAgent.done.title': '이후 에이전트 생성',
    'contextualGuide.tenantModels.stepsAgent.intro.desc': '에이전트 생성에는 KnowledgeQA 모델이 최소 하나 필요합니다. 시스템 설정에서 추가하세요(Embedding은 지식 베이스에만 필요).',
    'contextualGuide.tenantModels.stepsAgent.intro.title': '먼저 대화 모델을 구성하세요',
  },
  'ru-RU': {
    'contextualGuide.agentCreate.steps.agentType.desc': 'Пресеты подставляют системный промпт, рекомендуемые инструменты и область знаний (например, построение Wiki, анализ данных). При смене типа обновляются имя и описание.',
    'contextualGuide.agentCreate.steps.agentType.title': 'Выберите тип агента',
    'contextualGuide.agentCreate.steps.knowledge.desc': '«Все» — универсальный помощник; «Выбранные» — домен; «Нет» — только модель или веб-поиск.',
    'contextualGuide.agentCreate.steps.knowledge.title': 'Область знаний',
    'contextualGuide.agentCreate.steps.mode.desc': '«Быстрый ответ» для простого Q&A; «Умное рассуждение» — инструменты и многошаговое мышление для сложных задач.',
    'contextualGuide.agentCreate.steps.mode.title': 'Выберите режим работы',
    'contextualGuide.agentCreate.steps.model.desc': 'Выберите из настроенных моделей чата или сначала добавьте в системных настройках.',
    'contextualGuide.agentCreate.steps.model.title': 'Выбор модели',
    'contextualGuide.agentCreate.steps.multimodal.desc': 'Настройте загрузку изображений и аудио, а также правила разбора вложений; при загрузке изображений выберите VLM ниже.',
    'contextualGuide.agentCreate.steps.multimodal.title': 'Включить загрузку вложений',
    'contextualGuide.agentCreate.steps.name.desc': 'Укажите узнаваемое имя. В режиме умного рассуждения может подставиться значение по умолчанию.',
    'contextualGuide.agentCreate.steps.name.title': 'Название и описание',
    'contextualGuide.agentCreate.steps.navKnowledge.desc': 'Определите, к каким знаниям агент может обращаться. По умолчанию — все базы.',
    'contextualGuide.agentCreate.steps.navKnowledge.title': 'Связать базы знаний',
    'contextualGuide.agentCreate.steps.navModel.desc': 'Каждому агенту нужна модель KnowledgeQA как движок рассуждений.',
    'contextualGuide.agentCreate.steps.navModel.title': 'Привязать модель чата',
    'contextualGuide.agentCreate.steps.navMultimodal.desc': 'Позволяет отправлять изображения, документы и аудио в чате; для понимания изображений нужна VLM в настройках.',
    'contextualGuide.agentCreate.steps.navMultimodal.title': 'Загрузка вложений (необязательно)',
    'contextualGuide.agentCreate.steps.navTools.desc': 'В режиме умного рассуждения включите встроенные инструменты и MCP для поиска, кода и др.',
    'contextualGuide.agentCreate.steps.navTools.title': 'Инструменты и MCP (необязательно)',
    'contextualGuide.agentCreate.steps.navWebsearch.desc': 'Разрешить агенту вызывать внешний поиск для актуальной информации.',
    'contextualGuide.agentCreate.steps.navWebsearch.title': 'Веб-поиск (необязательно)',
    'contextualGuide.agentCreate.steps.submit.desc': 'Нажмите выделенную кнопку подтверждения. Затем выберите этого агента в чате.',
    'contextualGuide.agentCreate.steps.submit.title': 'Сохранить агента',
    'contextualGuide.agentList.steps.create.desc': 'Агенты объединяют модели, базы знаний, инструменты и промпты в переиспользуемых помощников. Нажмите выделенную кнопку «Создать агента».',
    'contextualGuide.agentList.steps.create.title': 'Создайте агента',
    'contextualGuide.chat.steps.done.desc': 'Задайте вопрос по загруженным документам и посмотрите ответы со ссылками на источники.',
    'contextualGuide.chat.steps.done.title': 'Можно исследовать',
    'contextualGuide.chat.steps.input.desc': 'Опишите, что хотите узнать, или нажмите на рекомендуемый вопрос выше.',
    'contextualGuide.chat.steps.input.title': 'Введите вопрос',
    'contextualGuide.chat.steps.kb.desc': 'Нажмите {\'@\'}, чтобы выбрать одну или несколько баз знаний или файлов. Иначе используются настройки текущего агента.',
    'contextualGuide.chat.steps.kb.title': 'Выберите область знаний',
    'contextualGuide.chat.steps.send.desc': 'После отправки создаётся новая сессия. ИИ отвечает на основе базы знаний и показывает цитаты.',
    'contextualGuide.chat.steps.send.title': 'Отправить, чтобы начать чат',
    'contextualGuide.done': 'Понятно',
    'contextualGuide.interactHint': 'Нажмите на выделенную область, чтобы продолжить',
    'contextualGuide.kbCreate.steps.chunking.desc': 'Как документ делится на фрагменты для поиска. Размеры по умолчанию подобраны для RAG.',
    'contextualGuide.kbCreate.steps.chunking.title': 'Разбиение (необязательно)',
    'contextualGuide.kbCreate.steps.embedding.desc': 'Преобразует текст в векторы для семантического поиска. Работает с векторным/ключевым индексом выше.',
    'contextualGuide.kbCreate.steps.embedding.title': 'Модель Embedding',
    'contextualGuide.kbCreate.steps.faq.desc': 'Выберите режим индексации пар вопрос–ответ. Записи FAQ можно добавить после создания.',
    'contextualGuide.kbCreate.steps.faq.title': 'Индексация FAQ',
    'contextualGuide.kbCreate.steps.indexing.desc': 'По умолчанию включены векторный и ключевой поиск. Можно включить Wiki или граф знаний. Оставьте хотя бы один режим поиска.',
    'contextualGuide.kbCreate.steps.indexing.title': 'Возможности индексации',
    'contextualGuide.kbCreate.steps.llm.desc': 'Для сводок и ответов. Если список пуст, через выпадающий список откройте настройки и добавьте модель.',
    'contextualGuide.kbCreate.steps.llm.title': 'Модель чата / сводки',
    'contextualGuide.kbCreate.steps.multimodalToggle.desc': 'При включении загрузки с изображениями обрабатываются VLM для лучшего поиска.',
    'contextualGuide.kbCreate.steps.multimodalToggle.title': 'Включить мультимодальный разбор',
    'contextualGuide.kbCreate.steps.multimodalVllm.desc': 'Для мультимодальности нужна VLM. Если список пуст, добавьте модель в настройках.',
    'contextualGuide.kbCreate.steps.multimodalVllm.title': 'Выберите модель VLM',
    'contextualGuide.kbCreate.steps.name.desc': 'Например «Руководство продукта» или «FAQ поддержки». Описание необязательно.',
    'contextualGuide.kbCreate.steps.name.title': 'Введите название',
    'contextualGuide.kbCreate.steps.navModels.desc': 'Каждой базе нужна модель чата; для поиска также нужен Embedding. Откройте «Конфигурация моделей» слева.',
    'contextualGuide.kbCreate.steps.navModels.title': 'Модели (обязательно)',
    'contextualGuide.kbCreate.steps.navMultimodal.desc': 'Включите, если в документах много диаграмм, сканов или изображений, требующих визуального понимания.',
    'contextualGuide.kbCreate.steps.navMultimodal.title': 'Мультимодальность / изображения (необязательно)',
    'contextualGuide.kbCreate.steps.parser.desc': 'Как разбираются PDF и Office. По умолчанию подходит в большинстве случаев; меняйте при необходимости OCR.',
    'contextualGuide.kbCreate.steps.parser.title': 'Парсер (необязательно)',
    'contextualGuide.kbCreate.steps.storage.desc': 'Где хранятся исходные файлы (локально или объектное хранилище). Обычно достаточно значения пространства.',
    'contextualGuide.kbCreate.steps.storage.title': 'Хранилище (необязательно)',
    'contextualGuide.kbCreate.steps.submit.desc': 'Нажмите «Понятно», чтобы завершить руководство без создания базы знаний. Когда будете готовы, введите название, проверьте тип и модели, затем нажмите «Создать».',
    'contextualGuide.kbCreate.steps.submit.title': 'Создать базу знаний',
    'contextualGuide.kbCreate.steps.type.desc': 'Для PDF, Word и похожих файлов — база документов. Для пар вопрос–ответ — база FAQ. Тип после создания изменить нельзя.',
    'contextualGuide.kbCreate.steps.type.title': 'Выберите тип',
    'contextualGuide.kbDetail.steps.done.desc': 'После обработки документов укажите эту базу через {\'@\'} в чате и получайте ответы со ссылками на источники.',
    'contextualGuide.kbDetail.steps.done.title': 'Готово после индексации',
    'contextualGuide.kbDetail.steps.intro.desc': 'Добавьте первый материал, чтобы искать по нему и общаться в чате. Поддерживаемые файлы можно перетащить мышью.',
    'contextualGuide.kbDetail.steps.intro.title': 'База знаний пуста',
    'contextualGuide.kbDetail.steps.upload.desc': 'Здесь можно загрузить файлы или папки, импортировать URL или создать материал онлайн.',
    'contextualGuide.kbDetail.steps.upload.title': 'Добавить документы',
    'contextualGuide.kbList.steps.create.desc': 'В базах знаний хранятся документы и FAQ. Нажмите выделенную кнопку «Новая база знаний» ниже — мы проведём вас по форме.',
    'contextualGuide.kbList.steps.create.title': 'Создайте первую базу знаний',
    'contextualGuide.next': 'Далее',
    'contextualGuide.prev': 'Назад',
    'contextualGuide.skip': 'Пропустить',
    'contextualGuide.stepOf': '{current} / {total}',
    'contextualGuide.tenantModels.needChatModelFirst': 'Перед созданием агента добавьте модель чата (KnowledgeQA).',
    'contextualGuide.tenantModels.steps.addModel.desc': 'Нажмите «Добавить модель» и настройте типы KnowledgeQA (чат) и Embedding. В Lite можно загрузить локальные модели через Ollama.',
    'contextualGuide.tenantModels.steps.addModel.title': 'Добавить модели',
    'contextualGuide.tenantModels.steps.done.desc': 'После сохранения моделей закройте настройки и нажмите «Новая база знаний». Мастер проведёт через тип, индексацию и привязку моделей.',
    'contextualGuide.tenantModels.steps.done.title': 'Затем продолжите',
    'contextualGuide.tenantModels.steps.intro.desc': 'Для документной базы нужна модель чата (сводки и ответы) и модель Embedding (векторный поиск). Добавьте их в системных настройках.',
    'contextualGuide.tenantModels.steps.intro.title': 'Сначала настройте модели',
    'contextualGuide.tenantModels.stepsAgent.addModel.desc': 'Нажмите «Добавить модель» и настройте тип KnowledgeQA.',
    'contextualGuide.tenantModels.stepsAgent.addModel.title': 'Добавить модель чата',
    'contextualGuide.tenantModels.stepsAgent.done.desc': 'После сохранения закройте настройки и нажмите «Создать агента». Мастер охватит режим, базы знаний и загрузку вложений.',
    'contextualGuide.tenantModels.stepsAgent.done.title': 'Затем создайте агента',
    'contextualGuide.tenantModels.stepsAgent.intro.desc': 'Для создания агента нужна хотя бы одна модель KnowledgeQA. Добавьте её в системных настройках (Embedding нужен только для баз знаний).',
    'contextualGuide.tenantModels.stepsAgent.intro.title': 'Сначала настройте модель чата',
  },
};
