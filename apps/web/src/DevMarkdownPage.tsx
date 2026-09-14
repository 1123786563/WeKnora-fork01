import { useEffect, useRef, useState } from 'react';
import { hydrateMermaidBlocksWithBrowserDefaults, renderChatMarkdown } from '@weknora/views';

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

export function renderMarkdownFixture(markdown: string): string {
  return renderChatMarkdown(markdown);
}

export function DevMarkdownPage() {
  const [markdown, setMarkdown] = useState(DEFAULT_MARKDOWN);
  const outputRef = useRef<HTMLElement>(null);

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
          <h1>Markdown rendering test</h1>
          <p className="wk-muted text-muted">Paste Markdown to inspect the escaped React rendering boundary.</p>
        </div>
        <button type="button" onClick={() => setMarkdown(DEFAULT_MARKDOWN)}>Reset</button>
      </header>
      <label className="wk-markdown-test-editor">
        Markdown input
        <textarea value={markdown} onChange={(event) => setMarkdown(event.target.value)} rows={12} />
      </label>
      <section ref={outputRef} aria-label="Rendered Markdown" className="wk-markdown-test-output" dangerouslySetInnerHTML={{ __html: renderMarkdownFixture(markdown) }}>
      </section>
    </main>
  );
}
