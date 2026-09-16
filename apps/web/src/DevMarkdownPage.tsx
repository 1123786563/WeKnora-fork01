import { useEffect, useRef, useState } from 'react';
import { hydrateMermaidBlocksWithBrowserDefaults } from '@weknora/views/chat/mermaid';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import { Button } from '@weknora/ui/button';
import { Textarea } from '@weknora/ui/textarea';

export const MARKDOWN_FIXTURE_SECTIONS = [
  'basic', 'latex', 'code', 'table', 'lists', 'mixed', 'mermaid', 'stream', 'custom',
] as const;

const DEFAULT_MARKDOWN = `# Markdown rendering fixture

This development-only page exercises the safe text boundary used by the React renderer.

- **bold** and *italic* text
- [a link](https://example.com)

<img src="x" onerror="alert('unsafe')">

\`\`\`mermaid
graph TD
  A[Markdown] --> B[Sanitized SVG]
\`\`\`

\`\`\`text
<script>alert('unsafe')</script>
\`\`\`
`;

const STREAM_MARKDOWN = `# Streaming Markdown

This fixture renders one character at a time so incomplete emphasis, lists, and code fences can be inspected.

\`\`\`mermaid
graph TD
  A[stream] --> B[done]
\`\`\`
`;

const BASIC_MARKDOWN = `这是一段普通文本，包含 **加粗**、*斜体*、~~删除线~~、行内 \`code\`。

> 一级引用
>
> > 嵌套引用

- [ ] 未完成任务
- [x] 已完成任务`;

const CODE_MARKDOWN = `\`\`\`python
def fibonacci(n: int) -> int:
    return n if n <= 1 else fibonacci(n - 1) + fibonacci(n - 2)
\`\`\``;

const TABLE_MARKDOWN = `| Element | Symbol | Atomic Number |
|---------|--------|---------------:|
| Hydrogen | H | 1 |
| Helium | He | 2 |`;

const LIST_MARKDOWN = `### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B

### Blockquote
> A Vue-compatible Markdown fixture.`;

export function renderMarkdownFixture(markdown: string): string {
  return renderChatMarkdown(markdown);
}

export function shouldRenderCustomMarkdown(markdown: string): boolean {
  return markdown.trim().length > 0;
}

export function DevMarkdownPage() {
  const [markdown, setMarkdown] = useState(DEFAULT_MARKDOWN);
  const [streamBuffer, setStreamBuffer] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const outputRef = useRef<HTMLElement>(null);
  const streamTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const resetStream = () => {
    if (streamTimerRef.current) clearInterval(streamTimerRef.current);
    streamTimerRef.current = null;
    setStreamBuffer('');
    setIsStreaming(false);
  };

  const startStream = () => {
    resetStream();
    setIsStreaming(true);
    let index = 0;
    streamTimerRef.current = setInterval(() => {
      if (index >= STREAM_MARKDOWN.length) {
        if (streamTimerRef.current) clearInterval(streamTimerRef.current);
        streamTimerRef.current = null;
        setIsStreaming(false);
        return;
      }
      const next = STREAM_MARKDOWN[index++];
      if (next) setStreamBuffer((current) => current + next);
    }, 20);
  };

  useEffect(() => () => {
    if (streamTimerRef.current) clearInterval(streamTimerRef.current);
  }, []);

  useEffect(() => {
    const root = outputRef.current;
    if (!root || typeof window === 'undefined' || !root.querySelector('[data-markdown-diagram="mermaid"]')) return;
    let disposed = false;
    void (async () => {
      if (disposed) return;
      await hydrateMermaidBlocksWithBrowserDefaults(root, 'wk-dev-mermaid');
    })().catch(() => {
      // Keep the escaped code block visible when the optional renderer fails.
    });
    return () => { disposed = true; };
  }, [markdown]);

  return (
    <main className="wk-page wk-markdown-test-page mx-auto box-border max-w-[960px] px-[1.25rem] py-12">
      <header className="wk-header mb-6 flex items-start justify-between gap-4">
        <div>
          <p className="wk-eyebrow m-0 text-[0.78rem] font-bold uppercase tracking-[0.08em] text-primary">Development fixture</p>
          <h1 className="text-[clamp(1.8rem,5vw,2.5rem)] my-[0.35rem]">Markdown rendering test</h1>
          <p className="wk-muted text-muted">Paste Markdown to inspect the escaped React rendering boundary.</p>
        </div>
        <Button type="button" onClick={() => setMarkdown(DEFAULT_MARKDOWN)}>Reset</Button>
      </header>
      <section className="mb-6 grid gap-4" aria-label="Markdown fixtures">
        <section className="grid gap-2">
          <h2 className="m-0 text-lg font-semibold">Basic Text Styles</h2>
          <div className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(BASIC_MARKDOWN) }} />
        </section>
        <section className="grid gap-2">
          <h2 className="m-0 text-lg font-semibold">Code Blocks</h2>
          <div className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(CODE_MARKDOWN) }} />
        </section>
        <section className="grid gap-2">
          <h2 className="m-0 text-lg font-semibold">Tables</h2>
          <div className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(TABLE_MARKDOWN) }} />
        </section>
        <section className="grid gap-2">
          <h2 className="m-0 text-lg font-semibold">Lists &amp; Blockquotes</h2>
          <div className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(LIST_MARKDOWN) }} />
        </section>
      </section>
      <section className="mb-6 grid gap-3" aria-label="Streaming Markdown">
        <div className="flex items-center justify-between gap-3">
          <h2 className="m-0 text-lg font-semibold">Streaming Markdown</h2>
          <div className="flex items-center gap-2">
            <Button type="button" onClick={startStream} disabled={isStreaming}>Start</Button>
            <Button type="button" onClick={resetStream}>Reset</Button>
          </div>
        </div>
        <div className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(streamBuffer) }} />
      </section>
      <label className="wk-markdown-test-editor">
        Paste any Markdown here to test rendering.
        <Textarea value={markdown} onChange={(event) => setMarkdown(event.target.value)} rows={12} />
      </label>
      {shouldRenderCustomMarkdown(markdown) && (
        <section ref={outputRef} aria-label="Rendered Markdown" className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(markdown) }} />
      )}
    </main>
  );
}
