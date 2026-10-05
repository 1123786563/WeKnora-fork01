/**
 * React port of frontend/src/utils/markdownOptions.ts (shared bug fix, not a
 * product-increment divergence — Vue↔React parity convention).
 *
 * #3962 — marked's default GFM `del` tokenizer also accepts a single tilde
 * (`~text~`), but CJK technical writing uses `~` as a range separator
 * (`2020~2035`, `50~60°`, `50,000~100,000 m³/d`, `8:00~12:00`). Any line
 * holding two of those ranges had the whole span between the tildes
 * swallowed into a <del>, with the tildes themselves eaten.
 *
 * `doubleTildeDelTokenizer` is marked's default `del` tokenizer with the
 * single-tilde branch removed: only a full `~~` run can open/close a <del>,
 * and every single `~` renders literally. It reads the active rule set
 * through `this.rules` instead of copying marked's regexes, so `~~` keeps
 * exactly the delimiter-run/flanking behavior of the marked version in use.
 *
 * Consumed by the wiki face (global marked singleton, wiki/markdown.ts) and
 * the embed face (dedicated `new Marked()` instance, embed/markdown.ts).
 * The singleton is also shared with packages/views (same pnpm-resolved
 * marked module), so the main chat face inherits the fix too.
 */
import type { MarkedExtension, Tokens, TokenizerObject } from 'marked';

/**
 * Anything `applyMarkdownOptions` can configure: the global `marked`
 * singleton (a callable namespace whose `use` mutates global state) or a
 * dedicated `new Marked()` instance.
 */
export type MarkdownOptionsTarget = {
  use: (extension: MarkedExtension) => unknown;
};

export const doubleTildeDelTokenizer: TokenizerObject['del'] = function (
  src,
  maskedSrc,
  prevChar = '',
) {
  let match = this.rules.inline.delLDelim.exec(src);
  if (!match) return;

  const nextChar = match[1] || '';

  if (!nextChar || !prevChar || this.rules.inline.punctuation.exec(prevChar)) {
    // unicode Regex counts emoji as 1 char; spread into array for proper count
    const lLength = [...match[0]].length - 1;
    // #3962: single-tilde branch removed — only a full `~~` run opens a <del>.
    if (lLength !== 2) return;

    let rDelim: string | undefined;
    let rLength: number;
    let delimTotal = lLength;

    const endReg = this.rules.inline.delRDelim;
    endReg.lastIndex = 0;

    // Clip maskedSrc to same section of string as src
    maskedSrc = maskedSrc.slice(-1 * src.length + lLength);

    while ((match = endReg.exec(maskedSrc)) != null) {
      rDelim = match[1] || match[2] || match[3] || match[4] || match[5] || match[6];

      if (!rDelim) continue;

      rLength = [...rDelim].length;

      if (rLength !== lLength) continue;

      if (match[3] || match[4]) { // found another Left Delim
        delimTotal += rLength;
        continue;
      }

      delimTotal -= rLength;

      if (delimTotal > 0) continue; // Haven't found enough closing delimiters

      // Remove extra characters
      rLength = Math.min(rLength, rLength + delimTotal);
      // char length can be >1 for unicode characters
      const lastCharLength = [...match[0]][0].length;
      const raw = src.slice(0, lLength + match.index + lastCharLength + rLength);

      // Create del token — only double `~~` after removing the #3962 branch
      const text = raw.slice(lLength, -lLength);
      return {
        type: 'del',
        raw,
        text,
        tokens: this.lexer.inlineTokens(text),
      };
    }
  }
  return undefined;
};

const instancesWithSharedOptions = new WeakSet<object>();

/**
 * Apply the shared markdown options for user-facing markdown rendering (wiki
 * pages, embed chat answers): `breaks` + `gfm` on, and strikethrough
 * restricted to a full `~~` run (#3962). Idempotent per instance so the
 * singleton consumer can call it before every parse without re-wrapping the
 * override.
 */
export function applyMarkdownOptions(markedInstance: MarkdownOptionsTarget): void {
  if (instancesWithSharedOptions.has(markedInstance)) return;
  instancesWithSharedOptions.add(markedInstance);
  markedInstance.use({
    breaks: true,
    gfm: true,
    tokenizer: {
      del: doubleTildeDelTokenizer,
    },
  } satisfies MarkedExtension);
}
