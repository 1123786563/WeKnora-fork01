/*
 * Browser tool result details — React port of upstream
 * frontend/src/views/chat/components/BrowserToolDetails.vue.
 */
import * as React from 'react';

import type { ChatCopyTable } from './chat-copy.ts';
import { browserToolContent, browserToolIncomplete, browserToolSummary, type BrowserToolCopy, type BrowserToolEvent } from './browser-tool-display.ts';

export function BrowserToolDetails({ event, copy }: { event: BrowserToolEvent; copy: ChatCopyTable }) {
  const content = browserToolContent(event);
  const browserCopy: BrowserToolCopy = copy;
  return (
    <section className="wk-browser-tool-details">
      <p className={browserToolIncomplete(event) ? 'wk-browser-status wk-browser-status-failed' : 'wk-browser-status'}>
        {browserToolSummary(browserCopy, event)}
      </p>
      {content.error ? <p className="wk-browser-error">{content.error}</p> : null}
      {content.recoveryHint ? <p className="wk-browser-hint">{content.recoveryHint}</p> : null}
      {content.empty ? <p>{copy.browserToolNoEntries}</p> : null}
      {content.title ? <p className="wk-browser-title">{content.title}</p> : null}
      {content.address ? <p className="wk-browser-address">{content.address}</p> : null}
      {content.prompt ? <p className="wk-browser-prompt">{content.prompt}</p> : null}
      {content.image ? <img className="wk-browser-image" src={content.image} alt={copy.browserToolPreview} /> : null}
      {content.tabs.length > 0 ? (
        <ul className="wk-browser-tabs">
          {content.tabs.map((tab, index) => (
            <li key={index}>
              <strong>{tab.title || copy.browserToolUntitledTab}</strong>
              {tab.address ? <span>{tab.address}</span> : null}
            </li>
          ))}
        </ul>
      ) : null}
      {content.text ? <pre className="wk-browser-text">{content.text}</pre> : null}
      {content.truncated ? <p className="wk-browser-note">{copy.browserToolContentTruncated}</p> : null}
    </section>
  );
}
