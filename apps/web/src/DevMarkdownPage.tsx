import { useEffect, useRef, useState, type ReactNode } from 'react';
import { hydrateMermaidBlocksWithBrowserDefaults } from '@weknora/views/chat/mermaid';
import { renderChatMarkdown } from '@weknora/views/chat/markdown';
import './chat/views-chat-u.css';

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
  // Vue 事实源（MarkdownTestPage.vue → chatMarkdownRenderer.renderChatMarkdown）
  // 在 marked 之前跑 repairFlankingEmphasis：`*`/`**` 之类的标点符尾强调
  // 修复（frontend/src/utils/chatMarkdownRenderer.ts:283-331）。其中
  // FLANKING_ITALIC 会把 `***加粗斜体***` 的首个 `**` 吞成 `<em>*</em>`，余下
  // `**` 以字面量泄漏——这是 Vue 端的既有渲染结果（dev 页首段的
  // `<em><em></em>加粗斜体</em>**`，probe 取证）。
  // @weknora/views/chat/markdown（React 共享渲染器）没有该前置 pass 且会转义
  // 注入的原始 HTML（renderer.html 只放行 <kbd>），无法在 markdown 源上预注入；
  // 该文件属 settings/chat 域所有权（并行流），本页按「终态 HTML 后置改写」
  // 等价复刻 Vue 对三重强调的输出（<em><strong>X</strong></em> →
  // <em><em></em>X</em>**，即 Vue repairFlanking+marked 链的可观察结果）。
  return renderChatMarkdown(markdown).replace(
    /<em><strong>([^<]*)<\/strong><\/em>/g,
    '<em><em></em>$1</em>**',
  );
}

export function shouldRenderCustomMarkdown(markdown: string): boolean {
  return markdown.trim().length > 0;
}

// Scoped mirror of the Vue page's styles (MarkdownTestPage.vue <style> block,
// which mixes in chat-markdown.less). Values below are the resolved TDesign
// token values measured off the Vue page (--td-brand-color #07c05f,
// --td-component-stroke #e7e7e7, --td-bg-color-secondarycontainer #f3f3f3,
// --td-text-color-primary rgba(0,0,0,.9), secondary rgba(0,0,0,.6)).
// Kept inline because the node test runner imports this module and cannot
// parse CSS files.
const PAGE_CSS = `
.markdown-test-page { max-width: 860px; margin: 0 auto; padding: 32px 24px; box-sizing: content-box; font-family: -apple-system, system-ui, 'Segoe UI', Roboto, 'Helvetica Neue', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif; font-size: 14px; color: rgba(0,0,0,0.9); -webkit-font-smoothing: antialiased; }
.markdown-test-page .page-title { font-size: 24px; font-weight: 700; margin: 0.67em 0 4px; }
.markdown-test-page .page-desc { color: rgba(0,0,0,0.6); font-size: 14px; margin: 14px 0 32px; }
.markdown-test-page .test-section { margin-bottom: 36px; border-bottom: 1px solid #e7e7e7; padding-bottom: 24px; }
.markdown-test-page .test-section h2 { font-size: 18px; font-weight: 600; margin: 0.83em 0 12px; }
.markdown-test-page .test-hint { font-size: 13px; color: rgba(0,0,0,0.6); margin: 13px 0 8px; }
.markdown-test-page .test-case { margin: 12px 0; }
.markdown-test-page .test-raw { background: #f3f3f3; padding: 6px 10px; border-radius: 4px; margin-bottom: 6px; font-size: 13px; overflow-x: auto; }
.markdown-test-page .test-raw code { white-space: pre-wrap; word-break: break-all; }
.markdown-test-page .test-rendered { padding: 8px 12px; border: 1px solid #e7e7e7; border-radius: 6px; background: #fff; font-size: 16px; line-height: 1.625; letter-spacing: normal; color: rgba(0,0,0,0.9); }
.markdown-test-page .stream-controls { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.markdown-test-page .btn { padding: 4px 16px; border: 1px solid #e7e7e7; border-radius: 4px; background: #fff; cursor: pointer; font-size: 13px; }
.markdown-test-page .btn:hover { background: #f0f0f0; }
.markdown-test-page .btn:disabled { opacity: 0.5; cursor: not-allowed; }
.markdown-test-page .speed-label { font-size: 13px; display: flex; align-items: center; gap: 6px; }
.markdown-test-page .speed-label input[type="range"] { width: 120px; }
.markdown-test-page .custom-textarea { width: 100%; padding: 10px; font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace; font-size: 13px; border: 1px solid #e7e7e7; border-radius: 6px; resize: vertical; box-sizing: border-box; margin-bottom: 12px; }
.markdown-test-page .shimmer-demo { display: flex; flex-direction: column; gap: 14px; }
.markdown-test-page .shimmer-demo .action-name { font-size: 14px; line-height: 1.55; color: rgba(0,0,0,0.6); }
/* ---- chat answer typography (chat-markdown-typography mixin) ---- */
/* No blanket > :first-child / > :last-child resets: the Vue mixin keeps
   list/table/blockquote outer margins on the container box. */
.markdown-test-page .wk-chat-message-content p { margin: 0 0 0.25em; line-height: 1.625; }
.markdown-test-page .wk-chat-message-content p + p { margin-top: 0.75em; }
.markdown-test-page .wk-chat-message-content p:last-child { margin-bottom: 0; }
.markdown-test-page .wk-chat-message-content p + ul,
.markdown-test-page .wk-chat-message-content p + ol,
.markdown-test-page .wk-chat-message-content ul + p,
.markdown-test-page .wk-chat-message-content ol + p,
.markdown-test-page .wk-chat-message-content blockquote + p,
.markdown-test-page .wk-chat-message-content p + blockquote { margin-top: 0.75em; }
.markdown-test-page .wk-chat-message-content strong { font-weight: 600; }
.markdown-test-page .wk-chat-message-content em { font-style: italic; }
.markdown-test-page .wk-chat-message-content del { text-decoration: line-through; opacity: 0.72; }
.markdown-test-page .wk-chat-message-content kbd { display: inline-block; padding: 0.15em 0.45em; font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace; font-size: 0.8125em; font-weight: 500; line-height: 1.4; color: rgba(0,0,0,0.9); background: #f3f3f3; border: 1px solid #e7e7e7; border-radius: 5px; box-shadow: 0 1px 0 rgba(0,0,0,0.08); }
.markdown-test-page .wk-chat-message-content code:not(pre code) { background: rgba(0,0,0,0.072); padding: 0.15em 0.3em; border-radius: 4px; font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace; font-size: 0.875em; font-weight: 500; }
.markdown-test-page .wk-chat-message-content pre { margin: 0.75em 0; padding: 14px 16px; border: 1px solid #e7e7e7; border-radius: 8px; background: color-mix(in srgb, #f3f3f3 88%, #fff); overflow-x: auto; line-height: 1.55; font-size: 13px; }
.markdown-test-page .wk-chat-message-content pre code { background: none; padding: 0; border-radius: 0; font-size: inherit; font-weight: 400; white-space: pre; display: block; }
.markdown-test-page .wk-chat-message-content .chat-code-block { margin: 0.75em 0; border: 1px solid #e7e7e7; border-radius: 8px; overflow: hidden; background: #fff; box-shadow: 0 1px 2px rgba(0,0,0,0.06); }
.markdown-test-page .wk-chat-message-content .chat-code-block__header { display: flex; align-items: center; justify-content: space-between; gap: 12px; min-height: 36px; padding: 6px 10px 6px 12px; background: #f3f3f3; border-bottom: 1px solid #e7e7e7; }
.markdown-test-page .wk-chat-message-content .chat-code-block__lang { font-size: 12px; font-weight: 600; line-height: 1.2; color: rgba(0,0,0,0.6); letter-spacing: 0.02em; }
.markdown-test-page .wk-chat-message-content .chat-code-block__copy { display: inline-flex; align-items: center; gap: 6px; margin: 0; padding: 4px 8px; border: 1px solid transparent; border-radius: 6px; background: transparent; color: rgba(0,0,0,0.6); font-size: 12px; font-weight: 500; line-height: 1; cursor: pointer; }
.markdown-test-page .wk-chat-message-content .chat-code-block__pre { margin: 0; padding: 14px 16px; border: 0; border-radius: 0; background: color-mix(in srgb, #f3f3f3 88%, #fff); }
.markdown-test-page .wk-chat-message-content h1 { font-size: 1.5em; font-weight: 600; margin: 0 0 0.375em; line-height: 1.333; }
.markdown-test-page .wk-chat-message-content h2 { font-size: 1.25em; font-weight: 600; margin: 1em 0 0.25em; line-height: 1.4; }
.markdown-test-page .wk-chat-message-content h3 { font-size: 1.125em; font-weight: 600; margin: 0.875em 0 0.25em; line-height: 1.556; }
.markdown-test-page .wk-chat-message-content h4 { font-size: 1em; font-weight: 600; margin: 1em 0 0; line-height: 1.5; }
.markdown-test-page .wk-chat-message-content h1:first-child,
.markdown-test-page .wk-chat-message-content h2:first-child,
.markdown-test-page .wk-chat-message-content h3:first-child,
.markdown-test-page .wk-chat-message-content h4:first-child { margin-top: 0; }
.markdown-test-page .wk-chat-message-content a { color: #07c05f; text-decoration: underline; text-decoration-color: rgba(7,192,95,0.45); text-underline-offset: 0.18em; }
.markdown-test-page .wk-chat-message-content ul,
.markdown-test-page .wk-chat-message-content ol { margin: 0.35em 0 0.75em !important; padding-left: 1.625em; }
.markdown-test-page .wk-chat-message-content ul { list-style-type: disc; list-style-position: outside; }
.markdown-test-page .wk-chat-message-content ol { list-style-type: decimal; list-style-position: outside; }
.markdown-test-page .wk-chat-message-content ul ul,
.markdown-test-page .wk-chat-message-content ol ol,
.markdown-test-page .wk-chat-message-content ul ol,
.markdown-test-page .wk-chat-message-content ol ul { margin: 0.25em 0 0 !important; padding-left: 1.25em; }
.markdown-test-page .wk-chat-message-content li { margin: 0; line-height: 1.625; padding-left: 0.375em; }
.markdown-test-page .wk-chat-message-content li + li { margin-top: 0.375em; }
.markdown-test-page .wk-chat-message-content li::marker { font-weight: 700; color: rgba(0,0,0,0.9); }
.markdown-test-page .wk-chat-message-content li input[type="checkbox"] { display: none; }
.markdown-test-page .wk-chat-message-content blockquote { border-left: 2px solid #e7e7e7; padding: 0.5em 0 0.5em 1.25em; margin: 0.5em 0 0.75em; color: rgba(0,0,0,0.6); font-weight: 400; font-style: normal; line-height: 1.5; }
.markdown-test-page .wk-chat-message-content blockquote blockquote { margin-top: 0.35em; margin-bottom: 0; border-left-color: #e7e7e7; }
.markdown-test-page .wk-chat-message-content blockquote p { line-height: 1.5; }
.markdown-test-page .wk-chat-message-content .chat-markdown-table { width: fit-content; max-width: 100%; margin: 0.875em 0 1em; overflow-x: auto; border: 1px solid #e7e7e7; border-radius: 8px; background: #fff; -webkit-overflow-scrolling: touch; }
.markdown-test-page .wk-chat-message-content table { word-break: initial; border-collapse: separate; border-spacing: 0; display: table; font-size: 14px; line-height: 1.55; width: max-content; min-width: 0; }
.markdown-test-page .wk-chat-message-content table thead { background-color: color-mix(in srgb, #f3f3f3 75%, #fff); }
.markdown-test-page .wk-chat-message-content table tbody tr:nth-child(2n) { background-color: color-mix(in srgb, #f3f3f3 45%, transparent); }
.markdown-test-page .wk-chat-message-content table tr th { font-weight: 600; border: 0; border-right: 1px solid #e7e7e7; border-bottom: 1px solid #e7e7e7; background-color: color-mix(in srgb, #f3f3f3 75%, #fff); text-align: left; padding: 8px 12px; white-space: nowrap; }
.markdown-test-page .wk-chat-message-content table tr td { border: 0; border-right: 1px solid #e7e7e7; border-bottom: 1px solid #e7e7e7; text-align: left; padding: 8px 12px; vertical-align: top; }
.markdown-test-page .wk-chat-message-content table tr th:last-child,
.markdown-test-page .wk-chat-message-content table tr td:last-child { border-right: 0; }
.markdown-test-page .wk-chat-message-content table tbody tr:last-child td { border-bottom: 0; }
.markdown-test-page .wk-chat-message-content table th[align='center'],
.markdown-test-page .wk-chat-message-content table td[align='center'] { text-align: center; }
.markdown-test-page .wk-chat-message-content .wk-chat-citation { background: #eef5ff; border: 1px solid #b9d1f2; border-radius: 999px; color: #245a9b; cursor: pointer; margin: 0 .15rem; padding: .1rem .45rem; }
.markdown-test-page .wk-chat-message-content .math-inline, .markdown-test-page .wk-chat-message-content .math-block { font-family: Georgia, serif; }
.markdown-test-page .wk-chat-message-content .math-block { overflow-x: auto; padding: .4rem 0; }
/* ---- mermaid block chrome (chat-mermaid-block) ---- */
.markdown-test-page .wk-chat-message-content .chat-mermaid-block { margin: 0.75em 0; border: 1px solid #e7e7e7; border-radius: 10px; overflow: hidden; background: #fff; box-shadow: 0 1px 3px rgba(0,0,0,0.05); }
.markdown-test-page .wk-chat-message-content .chat-mermaid-block__header { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 36px; padding: 6px 10px 6px 12px; background: #f3f3f3; border-bottom: 1px solid #e7e7e7; }
.markdown-test-page .wk-chat-message-content .chat-mermaid-block__badge { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: rgba(0,0,0,0.6); }
.markdown-test-page .wk-chat-message-content .chat-mermaid-block__expand { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; margin: 0; padding: 0; border: 1px solid transparent; border-radius: 6px; background: transparent; color: rgba(0,0,0,0.6); cursor: pointer; }
.markdown-test-page .wk-chat-message-content .chat-mermaid-block .wk-chat-mermaid { border: 0; border-radius: 0; margin: 0; padding: 20px 16px 18px; text-align: center; }
.markdown-test-page .wk-chat-message-content .chat-mermaid-block svg { max-width: 100%; height: auto; }
`;

const COPY_ICON = '<svg class="chat-code-block__copy-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>';
const EXPAND_ICON = '<svg class="chat-mermaid-block__expand-icon" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>';

// markdownEnhancements.formatCodeLang label table (Vue side).
const LANG_LABELS: Record<string, string> = {
  ts: 'TypeScript', typescript: 'TypeScript', py: 'Python', python: 'Python',
  go: 'Go', rust: 'Rust', java: 'Java', kotlin: 'Kotlin', swift: 'Swift',
  rb: 'Ruby', ruby: 'Ruby', php: 'PHP', cs: 'C#', cpp: 'C++', c: 'C',
  sql: 'SQL', bash: 'Bash', sh: 'Shell', shell: 'Shell', json: 'JSON',
  yaml: 'YAML', yml: 'YAML', xml: 'XML', html: 'HTML', css: 'CSS',
  markdown: 'Markdown', md: 'Markdown',
};

function formatCodeLang(lang: string): string {
  const normalized = (lang || 'Code').trim();
  if (!normalized) return 'Code';
  const key = normalized.toLowerCase();
  return LANG_LABELS[key] || normalized.charAt(0).toUpperCase() + normalized.slice(1);
}

// The Vue dev page renders fenced code through markdownEnhancements as a
// .chat-code-block card (language header + copy button). Mirror that chrome
// here with plain DOM wrapping so the parity scan compares like with like.
function wrapCodeBlocksWithChrome(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>('pre > code[class*="language-"]').forEach((code) => {
    const pre = code.parentElement;
    if (!pre || pre.closest('.chat-code-block') || pre.hasAttribute('data-markdown-diagram')) return;
    const langMatch = [...code.classList].find((cls) => cls.startsWith('language-'));
    const block = document.createElement('div');
    block.className = 'chat-code-block';
    block.innerHTML = `<div class="chat-code-block__header"><span class="chat-code-block__lang">${formatCodeLang(langMatch?.slice('language-'.length) || '')}</span><div class="chat-code-block__actions"><button type="button" class="chat-code-block__copy" aria-label="复制代码" title="复制代码">${COPY_ICON}<span class="chat-code-block__copy-text">复制代码</span></button></div></div>`;
    pre.parentNode?.insertBefore(block, pre);
    block.appendChild(pre);
    pre.classList.add('chat-code-block__pre');
  });
}

// Vue enhanceMarkdownContainer upgrades hydrated mermaid figures with the
// .chat-mermaid-block header chrome (图表 badge + 全屏查看 trigger).
function wrapMermaidFiguresWithChrome(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>('.wk-chat-mermaid').forEach((figure) => {
    if (figure.closest('.chat-mermaid-block')) return;
    const block = document.createElement('div');
    block.className = 'chat-mermaid-block';
    block.innerHTML = `<div class="chat-mermaid-block__header"><span class="chat-mermaid-block__badge">图表</span><div class="chat-mermaid-block__actions"><button type="button" class="chat-mermaid-block__expand" aria-label="全屏查看" title="全屏查看">${EXPAND_ICON}</button></div></div>`;
    figure.parentNode?.insertBefore(block, figure);
    block.appendChild(figure);
  });
}

// Vue renderer wraps markdown tables in a .chat-markdown-table card that
// carries the border/radius; the <table> itself is borderless.
function wrapTablesWithChrome(root: HTMLElement): void {
  root.querySelectorAll<HTMLElement>('.test-rendered table').forEach((table) => {
    if (table.closest('.chat-markdown-table')) return;
    const wrapper = document.createElement('div');
    wrapper.className = 'chat-markdown-table';
    table.parentNode?.insertBefore(wrapper, table);
    wrapper.appendChild(table);
  });
}

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
  // runtime via a <link> so the node test runner (which imports this module)
  // is unaffected and Vite resolves the asset URL.
  useEffect(() => {
    let disposed = false;
    void (async () => {
      try {
        const mod = await import('katex/dist/katex.min.css?url');
        if (disposed || document.querySelector('link[data-wk-katex-css]')) return;
        const link = document.createElement('link');
        link.rel = 'stylesheet';
        link.href = mod.default;
        link.setAttribute('data-wk-katex-css', '');
        document.head.appendChild(link);
      } catch {
        // Formulas degrade to plain spans when the stylesheet is unavailable.
      }
    })();
    return () => { disposed = true; };
  }, []);

  // Hydrate mermaid diagrams (static sections + streaming + custom editor) and
  // apply the Vue-side code-block / mermaid-block chrome.
  useEffect(() => {
    const root = rootRef.current;
    if (!root || typeof window === 'undefined') return;
    wrapCodeBlocksWithChrome(root);
    wrapTablesWithChrome(root);
    if (!root.querySelector('[data-markdown-diagram="mermaid"]')) return;
    let disposed = false;
    void (async () => {
      if (disposed) return;
      await hydrateMermaidBlocksWithBrowserDefaults(root, 'wk-dev-mermaid');
      if (!disposed) wrapMermaidFiguresWithChrome(root);
    })().catch(() => {
      // Keep the escaped code block visible when the optional renderer fails.
    });
    return () => { disposed = true; };
  }, [streamBuffer, customInput]);

  // Copy button behavior for the code-block chrome (chatMarkdownRenderer parity).
  const handleRootClick = (event: React.MouseEvent<HTMLDivElement>) => {
    const btn = (event.target as HTMLElement).closest?.('.chat-code-block__copy');
    if (!btn) return;
    const code = btn.closest('.chat-code-block')?.querySelector('code')?.textContent ?? '';
    const label = btn.querySelector('.chat-code-block__copy-text');
    void navigator.clipboard?.writeText(code).then(() => {
      if (!label) return;
      label.textContent = '已复制';
      window.setTimeout(() => { label.textContent = '复制代码'; }, 2000);
    }).catch(() => {});
  };

  return (
    <div ref={rootRef} className="markdown-test-page" onClick={handleRootClick}>
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
