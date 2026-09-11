import { useState } from 'react';

const DEFAULT_MARKDOWN = `# Markdown rendering fixture

This development-only page exercises the safe text boundary used by the React renderer.

- **bold** and *italic* text
- [a link](https://example.com)

\`\`\`text
<script>alert('unsafe')</script>
\`\`\`
`;

export function DevMarkdownPage() {
  const [markdown, setMarkdown] = useState(DEFAULT_MARKDOWN);

  return (
    <main className="wk-page wk-markdown-test-page">
      <header className="wk-header">
        <div>
          <p className="wk-eyebrow">Development fixture</p>
          <h1>Markdown rendering test</h1>
          <p className="wk-muted">Paste Markdown to inspect the escaped React rendering boundary.</p>
        </div>
        <button type="button" onClick={() => setMarkdown(DEFAULT_MARKDOWN)}>Reset</button>
      </header>
      <label className="wk-markdown-test-editor">
        Markdown input
        <textarea value={markdown} onChange={(event) => setMarkdown(event.target.value)} rows={12} />
      </label>
      <section aria-label="Rendered Markdown" className="wk-markdown-test-output">
        <pre>{markdown}</pre>
      </section>
    </main>
  );
}
