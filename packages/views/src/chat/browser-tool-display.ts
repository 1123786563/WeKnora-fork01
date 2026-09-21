/*
 * Browser tool display helpers — React port of upstream
 * frontend/src/utils/browserToolDisplay.ts (values byte-exact, copy table
 * keys flattened into ChatCopyTable as browserTool*).
 */

import type { ChatCopyTable } from './chat-copy.ts';

export interface BrowserToolCopy extends Pick<ChatCopyTable,
  | 'browserToolOpenPage' | 'browserToolSwitchPage' | 'browserToolCaptureScreenshot' | 'browserToolReadPage'
  | 'browserToolListTabs' | 'browserToolClickPage' | 'browserToolFillPage' | 'browserToolPressKey'
  | 'browserToolWaitPage' | 'browserToolHoverPage' | 'browserToolScrollPage' | 'browserToolFocusElement'
  | 'browserToolBlurElement' | 'browserToolSelectOption' | 'browserToolCloseTab' | 'browserToolRunScript'
  | 'browserToolReadConsole' | 'browserToolReadNetwork' | 'browserToolResizeWindow' | 'browserToolEmulateDevice'
  | 'browserToolOpenTab' | 'browserToolSwitchTab' | 'browserToolAuthorizeTab' | 'browserToolReturnTab'
  | 'browserToolNeedHelp' | 'browserToolBrowserAction' | 'browserToolActionFailed' | 'browserToolActionPending'
  | 'browserToolNavigationIncomplete' | 'browserToolActionCompleted' | 'browserToolActionRecorded'
  | 'browserToolCommandBusy' | 'browserToolInvalidArguments' | 'browserToolCommandInterrupted'
  | 'browserToolReconnectHint' | 'browserToolActionFailedHint' | 'browserToolNoEntries'
  | 'browserToolUntitledTab' | 'browserToolContentTruncated' | 'browserToolPreview' | 'browserToolLocal'
  | 'browserToolStop'
> {}

export type BrowserToolEvent = {
  arguments?: unknown;
  output?: unknown;
  error?: unknown;
  pending?: boolean;
  success?: boolean;
  tool_data?: unknown;
};

const METHOD_KEYS: Readonly<Record<string, keyof BrowserToolCopy>> = Object.freeze({
  navigate: 'browserToolOpenPage', navigate_back: 'browserToolSwitchPage', navigate_forward: 'browserToolSwitchPage',
  reload: 'browserToolOpenPage', screenshot: 'browserToolCaptureScreenshot', stop: 'browserToolStop',
  observe: 'browserToolReadPage', snapshot: 'browserToolReadPage', get_html: 'browserToolReadPage',
  tab_list: 'browserToolListTabs', click: 'browserToolClickPage', fill: 'browserToolFillPage',
  press: 'browserToolPressKey', wait_ms: 'browserToolWaitPage', wait_for_navigation: 'browserToolWaitPage',
  hover: 'browserToolHoverPage', wheel: 'browserToolScrollPage', scroll_to: 'browserToolScrollPage',
  focus: 'browserToolFocusElement', blur: 'browserToolBlurElement', select: 'browserToolSelectOption',
  tab_close: 'browserToolCloseTab', evaluate: 'browserToolRunScript', console: 'browserToolReadConsole',
  network: 'browserToolReadNetwork', window_resize: 'browserToolResizeWindow', emulate: 'browserToolEmulateDevice',
  tab_create: 'browserToolOpenTab', tab_select: 'browserToolSwitchTab', tab_borrow: 'browserToolAuthorizeTab',
  tab_return: 'browserToolReturnTab', request_help: 'browserToolNeedHelp',
});

// Same shape as upstream's /^[A-Za-z0-9+/\r\n]+={0,2}$/ — built via RegExp so the
// CR/LF escapes stay literal source characters instead of control bytes.
const BASE64_SHAPE = new RegExp('^[A-Za-z0-9+/\\r\\n]+={0,2}$');

const NEWLINE = String.fromCharCode(10);

function record(value: unknown): Record<string, any> {
  if (typeof value === 'string') {
    try {
      value = JSON.parse(value);
    } catch {
      return {};
    }
  }
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, any>) : {};
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

// Show the destination without credentials or query/fragment tokens. Never make
// page-provided URLs clickable or fetch them while rendering a tool result.
export function browserPageAddress(value: unknown): string {
  try {
    const url = new URL(text(value));
    return ['https:', 'http:'].includes(url.protocol) ? `${url.origin}${url.pathname}` : '';
  } catch {
    return '';
  }
}

function sourceLocation(entry: Record<string, any>): string {
  const address = browserPageAddress(entry.url);
  return address
    + (Number.isInteger(entry.line) ? ':' + entry.line : '')
    + (Number.isInteger(entry.column) ? ':' + entry.column : '');
}

const NAVIGATION_METHODS = new Set(['navigate', 'navigate_back', 'navigate_forward', 'reload', 'wait_for_navigation']);

export function browserToolIncomplete(event: BrowserToolEvent): boolean {
  return event.success === false
    || (NAVIGATION_METHODS.has(record(event.arguments).method) && record(event.output).reached === 'timeout');
}

export function browserActionLabel(copy: BrowserToolCopy, method: string): string {
  return copy[METHOD_KEYS[method] ?? 'browserToolBrowserAction'];
}

export function browserToolTitle(copy: BrowserToolCopy, event: BrowserToolEvent): string {
  const args = record(event.arguments);
  const label = browserActionLabel(copy, args.method);
  let target = '';
  if (args.method === 'navigate' || args.method === 'tab_create') {
    try {
      target = new URL(browserPageAddress(args.url)).host;
    } catch {
      /* No valid destination yet. */
    }
  } else if (args.method === 'press') {
    target = text(args.key).slice(0, 40);
  }
  return `${copy.browserToolLocal} · ${label}${target ? ` · ${target}` : ''}`
    + (event.pending ? '…' : browserToolIncomplete(event) ? ` · ${copy.browserToolActionFailed}` : '');
}

export function browserToolSummary(copy: BrowserToolCopy, event: BrowserToolEvent): string {
  const failure = event.error ?? event.output;
  const raw = typeof failure === 'string' ? failure : text(record(failure).message);
  if (event.pending) return copy.browserToolActionPending;
  if (NAVIGATION_METHODS.has(record(event.arguments).method) && record(event.output).reached === 'timeout') {
    return copy.browserToolNavigationIncomplete;
  }
  if (event.success !== false) {
    return event.success === true ? copy.browserToolActionCompleted : copy.browserToolActionRecorded;
  }
  if (/unfinished command|preview.*busy/i.test(raw)) return copy.browserToolCommandBusy;
  if (/Parameter validation failed|Invalid browser arguments|invalid_params|duration_ms/i.test(raw)) {
    return copy.browserToolInvalidArguments;
  }
  if (/paused|interrupted|timed out|timeout/i.test(raw)) return copy.browserToolCommandInterrupted;
  if (/disconnected|offline|connect BrowserSkill|not paired/i.test(raw)) return copy.browserToolReconnectHint;
  return copy.browserToolActionFailedHint;
}

export function browserToolContent(event: BrowserToolEvent) {
  const output = { ...record(event.tool_data), ...record(event.output) };
  const args = record(event.arguments);
  let content = text(output.text) || text(output.html);
  const entries = Array.isArray(output.entries) ? output.entries : [];
  if (args.method === 'console') {
    content = entries.slice(0, 100).map(record).map((entry) => {
      const location = sourceLocation(entry);
      const stack = Array.isArray(entry.stack_trace)
        ? entry.stack_trace.slice(0, 20).map(record).map((frame) =>
          `  ${text(frame.function_name)} ${sourceLocation(frame)}`).join(NEWLINE)
        : '';
      return `[${text(entry.level) || text(entry.kind)}] ${text(entry.text)}`
        + (location ? NEWLINE + location : '')
        + (stack ? NEWLINE + stack : '');
    }).join(NEWLINE + NEWLINE);
  } else if (args.method === 'network') {
    content = entries.slice(0, 100).map(record).map((entry) =>
      [text(entry.method),
        typeof entry.status === 'number' ? String(entry.status) : '',
        browserPageAddress(entry.url),
        text(entry.status_text),
        text(entry.error_text),
      ].filter(Boolean).join(' ')).join(NEWLINE);
  } else if (args.method === 'evaluate' && Object.hasOwn(output, 'value')) {
    content = typeof output.value === 'string' && output.value !== ''
      ? output.value
      : (JSON.stringify(output.value, null, 2) ?? '');
  }
  const resultError = record(output.error);
  const error = text(output.error_text) || text(resultError.message) || text(resultError.text)
    || text(event.error) || text(record(event.error).message);
  const diagnostics = args.method === 'console' || args.method === 'network';
  const image = text(output.image_base64);
  const format = text(output.format);
  return {
    error: error.slice(0, 4000),
    recoveryHint: text(output.recovery_hint).slice(0, 4000),
    empty: diagnostics && entries.length === 0 && !error && !event.pending && !browserToolIncomplete(event),
    title: text(output.title).slice(0, 300),
    address: browserPageAddress(output.final_url || output.url || args.url),
    text: content.slice(0, 12000),
    truncated: output.truncated === true
      || content.length > 12000
      || (diagnostics
        && (entries.length > 100
          || entries.some((entry) => record(entry).truncated === true
            || (Array.isArray(record(entry).stack_trace) && record(entry).stack_trace.length > 20)))),
    image: ['png', 'jpeg'].includes(format) && image.length <= 8 * 1024 * 1024
      && BASE64_SHAPE.test(image)
      ? `data:image/${format};base64,${image}` : '',
    tabs: Array.isArray(output.tabs)
      ? output.tabs.map(record).map((tab) => ({
        title: text(tab.title).slice(0, 300), address: browserPageAddress(tab.url),
      }))
      : [],
    prompt: args.method === 'request_help' ? text(args.prompt) : '',
  };
}
