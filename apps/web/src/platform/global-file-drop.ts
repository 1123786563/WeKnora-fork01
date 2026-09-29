// 全局拖拽上传（Vue frontend/src/views/platform/index.vue:69-232 同构平移）。
//
// Vue 基准机制：
//   - document 级 dragenter/dragover/dragleave/drop 监听（capture），计数式
//     判定（dragenter 计数++、dragleave 计数--、drop/卸载重置），避免子元素
//     事件抖动；拖入 OS 文件时显示全屏 .upload-mask。
//   - isFileDrag：仅 dataTransfer.types 含 "Files" 的 OS 文件拖拽才接管；
//     应用内元素拖拽（wiki 目录/页面拖放，只有 text/* types）原样放行。
//   - 聊天路由下 drop → 派发 weknora:chat-file-drop（Input-field.vue:81/
//     110-115/1821 handleChatFileDrop → handleDroppedFiles 投入输入框附件）。
//   - 知识库上下文 drop → 先校验 KB 初始化（checkKnowledgeBaseInitialization），
//     再派发 weknora:knowledge-file-drop（KnowledgeBase.vue:1275/1352 上传到当前 KB）。
//   - 非聊天且无 kbId 的页面 drop → 「缺少知识库ID」错误提示（Vue 原行为）。
import { useEffect, useRef, useState } from 'react';
import { formatMessage, type Locale } from '@weknora/i18n';
import { collectDroppedFiles } from './collect-dropped-files.ts';

export const CHAT_FILE_DROP_EVENT = 'weknora:chat-file-drop';
export const KNOWLEDGE_FILE_DROP_EVENT = 'weknora:knowledge-file-drop';

// isFileDrag distinguishes an OS file drag (the only thing the global upload
// drop zone cares about) from an in-app element drag such as the wiki
// folder/page drag-and-drop. Element drags carry only "text/*" types, never
// "Files", so we bail out and let the originating component handle the drop.
export function isFileDragEvent(event: DragEvent): boolean {
  const types = event.dataTransfer?.types;
  if (!types) return false;
  return Array.from(types).includes('Files');
}

// Vue CHAT_DROP_ROUTE_NAMES = ['chat', 'globalCreatChat', 'kbCreatChat']
// （platform/index.vue:77-81）→ React 路由的 pathname 等价集合：
// /platform/chat/:id（chat）、/platform/creatChat（globalCreatChat）、
// /platform/knowledge-bases/:kbId/creatChat（kbCreatChat）。
export function isChatFileDropPath(pathname: string): boolean {
  return pathname === '/platform/creatChat'
    || /^\/platform\/chat\/[^/]+/.test(pathname)
    || /^\/platform\/knowledge-bases\/[^/]+\/creatChat$/.test(pathname);
}

// Vue getCurrentKbId()（route.params.kbId，platform/index.vue:73-75）→ React
// 两种 KB 路径形态（/knowledgeBase/:id… 与 /platform/knowledge-bases/:id…）。
// /platform/knowledge-bases/:kbId/creatChat 已被聊天分支先行接管。
export function kbIdFromPathname(pathname: string): string | null {
  const match = pathname.match(/^\/(?:knowledgeBase|platform\/knowledge-bases)\/([^/]+)/);
  if (!match) return null;
  try {
    const id = decodeURIComponent(match[1]!).trim();
    return id ? id : null;
  } catch {
    return null;
  }
}

/*
 * Vue checkKnowledgeBaseInitialization 的纯判定部分（platform/index.vue:96-105）：
 *   1. 无 summary_model_id → 未初始化；
 *   2. indexing_strategy 缺失或开启 vector/keyword → 还需要 embedding_model_id，
 *      否则未初始化。
 * 返回需提示的 i18n key（与 Vue 一律走 knowledgeBase.notInitialized），可过
 * 则返回 null。
 */
export function knowledgeBaseInitializationWarning(kb: Record<string, unknown> | null | undefined): 'knowledgeBase.notInitialized' | null {
  if (!kb || typeof kb !== 'object') return 'knowledgeBase.notInitialized';
  if (!kb.summary_model_id) return 'knowledgeBase.notInitialized';
  const strategy = kb.indexing_strategy as { vector_enabled?: unknown; keyword_enabled?: unknown } | null | undefined;
  const needsEmbedding = !strategy || Boolean(strategy.vector_enabled) || Boolean(strategy.keyword_enabled);
  if (needsEmbedding && !kb.embedding_model_id) return 'knowledgeBase.notInitialized';
  return null;
}

/*
 * 遮罩主文案 file.upload（upload-mask.vue $t('file.upload')）尚未进入
 * @weknora/i18n 生成表（apps/web 域外，本改动不触 packages/i18n），先以
 * Vue locales（frontend/src/i18n/locales/*.ts file.upload）逐字内联五语；
 * 一旦键位回填进共享包，formatMessage 优先。
 */
const FILE_UPLOAD_TITLES: Record<Locale, string> = {
  'zh-CN': '上传文件',
  'en-US': 'Upload File',
  'ja-JP': 'ファイルをアップロード',
  'ko-KR': '파일 업로드',
  'ru-RU': 'Загрузить файл',
};

export function fileUploadTitle(locale: Locale): string {
  const viaBundle = formatMessage(locale, 'file.upload');
  return viaBundle !== 'file.upload' ? viaBundle : FILE_UPLOAD_TITLES[locale];
}

// 壳层 client 的可选探测面（同 shell 对 organizations/settings 的防御式
// 探测惯例：测试替身 / embed 挂载没有该命名空间时跳过初始化校验，放行派发）。
type GlobalFileDropClient = {
  knowledgeBases?: { settings?: { get?: (id: string) => Promise<Record<string, unknown>> } };
};

/*
 * useGlobalFileDrop — PlatformShell 挂载的全局文件拖拽通路，返回遮罩可见态。
 * 监听在挂载期注册一次（options 经 ref 透传最新值），卸载时移除并重置计数。
 * 聊天分支 stopPropagation 与 KB 分支不拦截均与 Vue 逐分支对齐：前者避免
 * 页面级 drop 处理器二次消费，后者让 FAQ 导入等页面本地 dropzone 仍能收到
 * 原生 drop 事件。
 */
export function useGlobalFileDrop(options: {
  client: unknown;
  locale: Locale;
  notify: (message: string) => void;
}): boolean {
  const [maskVisible, setMaskVisible] = useState(false);
  const optionsRef = useRef(options);
  optionsRef.current = options;
  // 用于跟踪拖拽进入/离开的计数器，解决子元素触发 dragleave 的问题
  //（Vue platform/index.vue:70；额外做 0 下限钳制，drop 后的幽灵
  // dragleave 不会把计数推成负数导致下一轮遮罩失灵）。
  const dragCounterRef = useRef(0);

  useEffect(() => {
    const handleGlobalDragEnter = (event: DragEvent) => {
      if (!isFileDragEvent(event)) return;
      event.preventDefault();
      dragCounterRef.current++;
      if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = 'all';
      }
      setMaskVisible(true);
    };

    const handleGlobalDragOver = (event: DragEvent) => {
      if (!isFileDragEvent(event)) return;
      event.preventDefault();
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = 'copy';
      }
    };

    const handleGlobalDragLeave = (event: DragEvent) => {
      if (!isFileDragEvent(event)) return;
      event.preventDefault();
      dragCounterRef.current = Math.max(0, dragCounterRef.current - 1);
      if (dragCounterRef.current === 0) {
        setMaskVisible(false);
      }
    };

    const handleGlobalDrop = async (event: DragEvent) => {
      if (!isFileDragEvent(event)) return;
      event.preventDefault();
      dragCounterRef.current = 0;
      setMaskVisible(false);

      const { client, locale: currentLocale, notify } = optionsRef.current;
      const message = (key: string) => formatMessage(currentLocale, key);

      const droppedFiles = await collectDroppedFiles(event);
      if (droppedFiles.length === 0) {
        notify(message('knowledgeBase.dragFileNotText'));
        return;
      }

      const pathname = window.location.pathname;
      if (isChatFileDropPath(pathname)) {
        event.stopPropagation();
        window.dispatchEvent(new CustomEvent(CHAT_FILE_DROP_EVENT, {
          detail: { files: droppedFiles },
        }));
        return;
      }

      const kbId = kbIdFromPathname(pathname);
      if (!kbId) {
        notify(message('knowledgeBase.missingId'));
        return;
      }

      // checkKnowledgeBaseInitialization（platform/index.vue:84-111）：
      // 拉取 KB 详情，未配置摘要/向量模型则提示并终止。
      const settingsApi = (client as GlobalFileDropClient).knowledgeBases?.settings;
      const getKnowledgeBase = settingsApi?.get?.bind(settingsApi);
      if (getKnowledgeBase) {
        let knowledgeBase: Record<string, unknown> | null = null;
        try {
          knowledgeBase = await getKnowledgeBase(kbId);
        } catch {
          notify(message('knowledgeBase.getInfoFailed'));
          return;
        }
        const warning = knowledgeBaseInitializationWarning(knowledgeBase);
        if (warning) {
          notify(message(warning));
          return;
        }
      }

      window.dispatchEvent(new CustomEvent(KNOWLEDGE_FILE_DROP_EVENT, {
        detail: { kbId, files: droppedFiles },
      }));
    };

    document.addEventListener('dragenter', handleGlobalDragEnter, true);
    document.addEventListener('dragover', handleGlobalDragOver, true);
    document.addEventListener('dragleave', handleGlobalDragLeave, true);
    document.addEventListener('drop', handleGlobalDrop, true);
    return () => {
      document.removeEventListener('dragenter', handleGlobalDragEnter, true);
      document.removeEventListener('dragover', handleGlobalDragOver, true);
      document.removeEventListener('dragleave', handleGlobalDragLeave, true);
      document.removeEventListener('drop', handleGlobalDrop, true);
      dragCounterRef.current = 0;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return maskVisible;
}
