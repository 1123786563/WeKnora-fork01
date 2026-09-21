import { useEffect, useRef, useState, type ReactNode } from 'react';
import { hydrateMermaidBlocksWithBrowserDefaults } from '@weknora/views/chat/mermaid';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';

export const MARKDOWN_FIXTURE_SECTIONS = [
  'basic', 'latex', 'code', 'table', 'lists', 'mixed', 'mermaid', 'stream', 'custom',
] as const;

// Dev-only page for visual regression testing of chat answer markdown — the
// React mirror of frontend/src/views/dev/MarkdownTestPage.vue. Keep the test
// cases, section order, and typography aligned with the Vue page so the
// parity pixel scan stays meaningful.

const BASIC_MARKDOWN = `这是一段普通文本，包含 **加粗**、*斜体*、***加粗斜体***、~~删除线~~、行内 \`code\`。

也可以包含快捷键样式：<kbd>⌘</kbd> + <kbd>K</kbd>。

这里有一个链接：[GitHub](https://github.com)。

> 一级引用
>
> > 嵌套引用，用于对比 GPT 的引用层级与字重。

- [ ] 未完成任务
- [x] 已完成任务
`;

const LATEX_CASES = [
  'Inline math: $E = mc^2$ in the middle of text.',
  'Block math:\n$$\\int_0^\\infty e^{-x}\\,dx = 1$$',
  'Chemical formula: $\\mathrm{Mg}^{2+} + 2\\mathrm{OH}^{-} = \\mathrm{Mg(OH)}_{2}\\downarrow$',
  'Chemical block:\n$$\\mathrm{Cu}^{2+} + 2\\mathrm{OH}^{-} \\rightarrow \\mathrm{Cu(OH)_2}\\downarrow$$',
  'Summation: $\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}$',
  'Matrix:\n$$\\begin{pmatrix} a & b \\\\ c & d \\end{pmatrix}$$',
  'Escaped delimiters: \\(\\alpha + \\beta = \\gamma\\) and \\[\\int_a^b f(x)\\,dx\\]',
];

const CODE_MARKDOWN = `Here is some Python:

\`\`\`python
def fibonacci(n: int) -> int:
    """Calculate the nth Fibonacci number."""
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

print(fibonacci(10))  # 55
\`\`\`

And inline code: \`const x = 42;\`
`;

const TABLE_MARKDOWN = `| Element | Symbol | Atomic Number |
|---------|--------|:-------------:|
| Hydrogen | H | 1 |
| Helium | He | 2 |
| Lithium | Li | 3 |
| Carbon | C | 6 |
`;

const LIST_MARKDOWN = `### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B
3. Third item

### Unordered List
- Alpha
- Beta
  - Sub-item
  - Another sub-item
- Gamma

### Blockquote
> This is a blockquote with **bold** and *italic* text.
>
> It can span multiple paragraphs.
`;

const MIXED_MARKDOWN = `## Quadratic Formula

The solutions to $ax^2 + bx + c = 0$ are given by:

$$x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}$$

### Example in Python

\`\`\`python
import math

def solve_quadratic(a, b, c):
    discriminant = b**2 - 4*a*c
    if discriminant < 0:
        return None
    x1 = (-b + math.sqrt(discriminant)) / (2*a)
    x2 = (-b - math.sqrt(discriminant)) / (2*a)
    return x1, x2
\`\`\`

| a | b | c | Solutions |
|---|---|---|-----------|
| 1 | -3 | 2 | $x = 1, 2$ |
| 1 | 0 | -4 | $x = \\pm 2$ |
| 1 | 2 | 5 | No real solutions |

> **Note:** The discriminant $\\Delta = b^2 - 4ac$ determines the nature of the roots.
`;

const MERMAID_MARKDOWN = `\`\`\`mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E
\`\`\`
`;

const STREAM_MARKDOWN = `
好的，以下是根据知识库中**《xxx》学程手册**整理的有关XXX的介绍：

**XBRL（eXtensible Business Reporting Language，可扩展商业报告语言）**是一种基于XML的标准化标记语言，专门用于电子化商业和财务报告的编制、交换和分析

该数据集包含90个文本和方程对，挑战模型提取、解释和推理相互关联的财务术语和公式的能力，例如：
\`\`\`
APR = ((Fees + Interest) / Principal) × (365 / Days in Loan Term)
\`\`\`
<kb doc="2502.08127v1.pdf" chunk_id="1ecdce8a-f922-4d0c-b124-257ab4634da2" />

### 重要性


1. **AAAAA**
   - **BBBBB**
   - **CCCCC**

1. **AAA**
2. **BBB**
3. **CCC**
4. **DDD**
5. **EEE**
6. **FFF**
7. **GGG**
8. **HHH**
9. **III**

**标题：JJJ**

**标题：KKK**


The energy-mass equivalence is $E = mc^2$.

In chemistry, the neutralization reaction:

$$\\mathrm{Mg}^{2+} + 2\\mathrm{OH}^{-} = \\mathrm{Mg(OH)}_2\\downarrow$$

Here is a code example:

\`\`\`python
def greet(name):
    print(f"Hello, {name}!")
\`\`\`

And the derivative rule: $\\frac{d}{dx}\\sin x = \\cos x$.

\`\`\`mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E
\`\`\`

### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B
3. Third item

### Unordered List
- Alpha
- Beta
  - Sub-item
  - Another sub-item
- Gamma

### Blockquote
> This is a blockquote with **bold** and *italic* text.
>
> It can span multiple paragraphs.

\`\`\`mermaid
sequenceDiagram
    participant Driver as 驾驶员 (Driver)
    participant HMI as 人机交互界面 (HMI/Cluster)
    participant ADAS as 组合驾驶辅助系统 (ADAS ECU)
    participant Sensor as 传感器/定位模块
    participant Cloud as 云端服务平台 (可选)

    Note over ADAS, Sensor: 阶段1：正常运行与监控
    Driver->>Sensor: 车辆正常行驶中
    Sensor->>ADAS: 状态数据 (环境、定位、车辆状态)
    ADAS-->>Driver: 维持组合驾驶辅助状态 (显示图标正常)

    Note over ADAS, Sensor: 阶段2：触发接管条件
    alt 系统检测到需接管场景
        Sensor->>ADAS: 检测条件满足 (如: 地图数据缺失/限速变化/系统故障/驾驶员分心)
        ADAS->>HMI: 发送接管请求信号 (HOR Signal)

        Note right of HMI: 阶段3：分级提醒策略 (符合国标要求)
        HMI-->>Driver: 视觉提示 (仪表盘图标闪烁/颜色变化)
        HMI-->>Driver: 听觉提示 (轻柔蜂鸣声)

        ADAS->>HMI: 增强提醒 (若驾驶员无响应)
        HMI-->>Driver: 强视觉警告 (红色边框/文字)
        HMI-->>Driver: 强听觉警告 (连续急促蜂鸣)
        HMI-->>Driver: 触觉提示 (方向盘震动/座椅振动)
    end

    Note over Driver, ADAS: 阶段4：驾驶员响应处理
    alt 驾驶员及时接管
        Driver->>HMI: 手握方向盘动作 (Torque/Grip Detection)
        HMI->>ADAS: 确认驾驶员介入信号
        ADAS-->>Driver: 退出自动驾驶，切换至人工驾驶模式
        ADAS->>HMI: 清除警告提示
    else 驾驶员未响应
        alt 达到最后接管时限 (e.g., T+5s)
            ADAS->>ADAS: 启动最小风险策略 (MRM/MLR)
            ADAS->>HMI: 触发紧急减速/停车提示
            HMI-->>Driver: 紧急警告 (最高级别)
            ADAS->>Sensor: 执行安全停车动作 (靠边、刹车、双闪)
        end
    end

    Note over Cloud, Driver: 阶段5：数据记录与上报
    ADAS->>Cloud: 上传接管事件数据 (时间、原因、驾驶员响应)
    Note right of Cloud: 用于事故定责与算法优化
\`\`\`

Done.`;

export function renderMarkdownFixture(markdown: string): string {
  return renderChatMarkdown(markdown);
}

export function shouldRenderCustomMarkdown(markdown: string): boolean {
  return markdown.trim().length > 0;
}

// Scoped mirror of the Vue page's styles (MarkdownTestPage.vue <style> block).
// Kept inline because the node test runner imports this module and cannot
// parse CSS files; the chat typography itself comes from the globally loaded
// chat.css via the `wk-chat-message-content` class.
const PAGE_CSS = `
.markdown-test-page { max-width: 860px; margin: 0 auto; padding: 32px 24px; box-sizing: border-box; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif; font-size: 14px; color: rgba(0,0,0,0.9); }
.markdown-test-page .page-title { font-size: 24px; font-weight: 700; margin: 16px 0 4px; }
.markdown-test-page .page-desc { color: #666; font-size: 14px; margin: 14px 0 32px; }
.markdown-test-page .test-section { margin-bottom: 36px; border-bottom: 1px solid #e5e5e5; padding-bottom: 24px; }
.markdown-test-page .test-section h2 { font-size: 18px; font-weight: 600; margin: 15px 0 12px; }
.markdown-test-page .test-hint { font-size: 13px; color: #999; margin: 13px 0 8px; }
.markdown-test-page .test-case { margin: 12px 0; }
.markdown-test-page .test-raw { background: #f5f5f5; padding: 6px 10px; border-radius: 4px; margin-bottom: 6px; font-size: 13px; overflow-x: auto; }
.markdown-test-page .test-raw code { white-space: pre-wrap; word-break: break-all; }
.markdown-test-page .test-rendered { padding: 8px 12px; border: 1px solid #e5e5e5; border-radius: 6px; background: #fff; }
.markdown-test-page .stream-controls { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.markdown-test-page .btn { padding: 4px 16px; border: 1px solid #ccc; border-radius: 4px; background: #fff; cursor: pointer; font-size: 13px; }
.markdown-test-page .btn:hover { background: #f0f0f0; }
.markdown-test-page .btn:disabled { opacity: 0.5; cursor: not-allowed; }
.markdown-test-page .speed-label { font-size: 13px; display: flex; align-items: center; gap: 6px; }
.markdown-test-page .speed-label input[type="range"] { width: 120px; }
.markdown-test-page .custom-textarea { width: 100%; padding: 10px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 13px; border: 1px solid #ccc; border-radius: 6px; resize: vertical; box-sizing: border-box; margin-bottom: 12px; }
.markdown-test-page .shimmer-demo { display: flex; flex-direction: column; gap: 14px; }
.markdown-test-page .shimmer-demo .action-name { font-size: 14px; line-height: 1.55; color: rgba(0,0,0,0.6); }
.markdown-test-page .wk-chat-message-content > :first-child { margin-top: 0; }
.markdown-test-page .wk-chat-message-content > :last-child { margin-bottom: 0; }
.markdown-test-page .wk-chat-message-content strong { font-weight: 600; }
.markdown-test-page .wk-chat-message-content code { background: rgba(0,0,0,0.05); border-radius: 3px; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 0.85em; padding: 2px 5px; }
.markdown-test-page .wk-chat-message-content pre code { background: transparent; padding: 0; font-size: 1em; }
.markdown-test-page .wk-chat-message-content blockquote { border-left: 3px solid rgba(0,0,0,0.12); color: rgba(0,0,0,0.66); margin: 8px 0; padding: 2px 0 2px 12px; }
.markdown-test-page .wk-chat-message-content blockquote blockquote { border-left-color: rgba(0,0,0,0.08); }
.markdown-test-page .wk-chat-message-content kbd { background: #f5f5f5; border: 1px solid #d9d9d9; border-bottom-width: 2px; border-radius: 4px; font-family: inherit; font-size: 0.85em; padding: 1px 6px; }
.markdown-test-page .wk-chat-message-content input[type="checkbox"] { display: none; }
.markdown-test-page .wk-chat-message-content ul { padding-left: 22px; }
.markdown-test-page .wk-chat-message-content ol { padding-left: 22px; }
.markdown-test-page .wk-chat-message-content pre { overflow-x: auto; padding: .7rem; background: #f6f8fb; border-radius: 6px; }
.markdown-test-page .wk-chat-message-content table { border-collapse: collapse; display: block; max-width: 100%; overflow-x: auto; }
.markdown-test-page .wk-chat-message-content th, .markdown-test-page .wk-chat-message-content td { border: 1px solid #dce3ed; padding: .35rem .55rem; text-align: left; }
.markdown-test-page .wk-chat-message-content .wk-chat-citation { background: #eef5ff; border: 1px solid #b9d1f2; border-radius: 999px; color: #245a9b; cursor: pointer; margin: 0 .15rem; padding: .1rem .45rem; }
.markdown-test-page .wk-chat-message-content .wk-chat-mermaid { border: 1px solid #dce3ed; border-radius: 6px; margin: .6rem 0; max-width: 100%; overflow: auto; padding: .5rem; }
.markdown-test-page .wk-chat-message-content .wk-chat-mermaid svg { height: auto; max-width: 100%; }
.markdown-test-page .wk-chat-message-content .math-inline, .markdown-test-page .wk-chat-message-content .math-block { font-family: Georgia, serif; }
.markdown-test-page .wk-chat-message-content .math-block { overflow-x: auto; padding: .4rem 0; }
`;

function Rendered(props: { html: string }): ReactNode {
  return (
    <div
      className="test-rendered wk-chat-message-content"
      dangerouslySetInnerHTML={{ __html: props.html }}
    />
  );
}

export function DevMarkdownPage() {
  const [customInput, setCustomInput] = useState('');
  const [streamBuffer, setStreamBuffer] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);
  const [streamSpeed, setStreamSpeed] = useState(30);
  const rootRef = useRef<HTMLDivElement>(null);
  const streamTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const streamSpeedRef = useRef(streamSpeed);
  streamSpeedRef.current = streamSpeed;

  const latexHtml = LATEX_CASES.map((raw) => ({ raw, html: renderMarkdownFixture(raw) }));
  const basicTextHtml = renderMarkdownFixture(BASIC_MARKDOWN);
  const codeBlockHtml = renderMarkdownFixture(CODE_MARKDOWN);
  const tableHtml = renderMarkdownFixture(TABLE_MARKDOWN);
  const listsHtml = renderMarkdownFixture(LIST_MARKDOWN);
  const mixedHtml = renderMarkdownFixture(MIXED_MARKDOWN);
  const mermaidHtml = renderMarkdownFixture(MERMAID_MARKDOWN);
  const streamHtml = streamBuffer ? renderMarkdownFixture(streamBuffer) : '';
  const customHtml = renderMarkdownFixture(customInput);

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
    }, streamSpeedRef.current);
  };

  useEffect(() => () => {
    if (streamTimerRef.current) clearInterval(streamTimerRef.current);
  }, []);

  // Vue imports katex/dist/katex.min.css for the formula face; load it at
  // runtime so the node test runner (which imports this module) is unaffected.
  useEffect(() => {
    let disposed = false;
    void import('katex/dist/katex.min.css').catch(() => {
      // Formulas degrade to plain spans when the stylesheet is unavailable.
    });
    return () => { disposed = true; };
  }, []);

  // Hydrate mermaid diagrams (static sections + streaming + custom editor).
  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof window === 'undefined' || !root.querySelector('[data-markdown-diagram="mermaid"]')) return;
    let disposed = false;
    void (async () => {
      if (disposed) return;
      await hydrateMermaidBlocksWithBrowserDefaults(root, 'wk-dev-mermaid');
    })().catch(() => {
      // Keep the escaped code block visible when the optional renderer fails.
    });
    return () => { disposed = true; };
  }, [streamBuffer, customInput]);

  return (
    <div ref={rootRef} className="markdown-test-page">
      <style>{PAGE_CSS}</style>
      <h1 className="page-title">Markdown Rendering Test</h1>
      <p className="page-desc">
        Dev-only page for visual regression testing of chat answer markdown
        (same typography as botmsg / AgentStreamDisplay / embed).
        Add new test cases or paste arbitrary markdown in the editor below.
      </p>

      <section className="test-section">
        <h2>Basic Text Styles</h2>
        <div className="test-case">
          <Rendered html={basicTextHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>LaTeX Formulas</h2>
        {latexHtml.map((tc, i) => (
          <div key={`latex-${i}`} className="test-case">
            <div className="test-raw"><code>{tc.raw}</code></div>
            <Rendered html={tc.html} />
          </div>
        ))}
      </section>

      <section className="test-section">
        <h2>Code Blocks</h2>
        <div className="test-case">
          <Rendered html={codeBlockHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>Tables</h2>
        <div className="test-case">
          <Rendered html={tableHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>Lists &amp; Blockquotes</h2>
        <div className="test-case">
          <Rendered html={listsHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>Mixed Content</h2>
        <div className="test-case">
          <Rendered html={mixedHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>Mermaid Diagram</h2>
        <div className="test-case">
          <Rendered html={mermaidHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>Streaming Simulation</h2>
        <p className="test-hint">Simulates character-by-character streaming, like during a chat response.</p>
        <div className="stream-controls">
          <button type="button" onClick={startStream} disabled={isStreaming} className="btn">Start</button>
          <button type="button" onClick={resetStream} className="btn">Reset</button>
          <label className="speed-label">
            Speed:
            <input
              type="range"
              min={10}
              max={200}
              value={streamSpeed}
              onChange={(event) => setStreamSpeed(Number(event.target.value))}
            />
            {streamSpeed}ms
          </label>
        </div>
        <div className="test-case">
          <Rendered html={streamHtml} />
        </div>
      </section>

      <section className="test-section">
        <h2>Streaming Shimmer</h2>
        <p className="test-hint">
          The &quot;light sweep&quot; applied to in-progress step titles in
          AgentStreamDisplay / RagPipelineProgress. Running steps shimmer; finished ones are static.
        </p>
        <div className="test-case shimmer-demo">
          <div className="action-card action-pending">
            <div className="action-title"><span className="action-name">正在检索知识库…</span></div>
          </div>
          <div className="action-card action-pending">
            <div className="action-title"><span className="action-name">正在生成回答…</span></div>
          </div>
          <div className="action-card">
            <div className="action-title"><span className="action-name is-done">检索完成（静态对照）</span></div>
          </div>
        </div>
      </section>

      <section className="test-section">
        <h2>Custom Input</h2>
        <p className="test-hint">Paste any markdown here to test rendering.</p>
        <textarea
          value={customInput}
          onChange={(event) => setCustomInput(event.target.value)}
          className="custom-textarea"
          rows={8}
          placeholder="Type or paste markdown here..."
        />
        {shouldRenderCustomMarkdown(customInput) && (
          <div className="test-case">
            <Rendered html={customHtml} />
          </div>
        )}
      </section>
    </div>
  );
}
