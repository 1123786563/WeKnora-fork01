// T20 (#139) assembly of the T02 (#121) deferred item: the workbench's
// archive-expansion entrance. The server endpoint (POST /craft/inputs/expand,
// already mounted and guarded: unknown ref → 404, cross-scope → 403) performs
// the bounded ATOMIC extraction — every member becomes an immutable input or
// nothing does. This panel only PROJECTS that authority: it offers the
// entrance for associated archive inputs, shows honest in-flight progress,
// and lists the members the server actually published. It never parses,
// decompresses or fabricates anything client-side.
import React from 'react';
import type { CraftInputView } from '@weknora/contracts';
import { Button } from 'tdesign-react';
import type { CraftLocale } from './presentation.ts';

/** The server's own archive-extension vocabulary (craft/archive.go). */
const CRAFT_ARCHIVE_EXTENSIONS = ['.zip', '.tar', '.tgz', '.gz'] as const;

/** True when the input's NAME offers expansion (the server re-validates). */
export function isExpandableArchiveInput(input: CraftInputView): boolean {
  const name = input.name.toLowerCase();
  return CRAFT_ARCHIVE_EXTENSIONS.some((extension) => name.endsWith(extension));
}

export interface CraftInputExpandPanelProps {
  locale: CraftLocale;
  /** Inputs associated with THIS session (the assembly tracks them). */
  inputs: readonly CraftInputView[];
  /**
   * Expands one associated archive input. Resolves only after the server
   * published every member atomically; rejects with the server's refusal.
   */
  onExpand(ref: string): Promise<readonly CraftInputView[]>;
}

interface CraftInputExpandStrings {
  heading: string;
  hint: string;
  expand: string;
  expanding: string;
  expandedCount: (count: number) => string;
  failed: string;
  alreadyExpanded: string;
}

function inputExpandStrings(locale: CraftLocale): CraftInputExpandStrings {
  return locale === 'zh'
    ? {
        heading: '归档材料',
        hint: '有界原子展开：所有成员一次性发布为不可变输入，失败不落任何新材料。',
        expand: '展开归档',
        expanding: '展开中…',
        expandedCount: (count) => `已展开 ${count} 个成员（全部作为不可变输入入库）`,
        failed: '归档展开失败，未发布任何成员。请重试或更换材料。',
        alreadyExpanded: '已展开',
      }
    : {
        heading: 'Archive materials',
        hint: 'Bounded atomic expansion: every member publishes as an immutable input at once, or nothing does.',
        expand: 'Expand archive',
        expanding: 'Expanding…',
        expandedCount: (count) => `Expanded ${count} members (all published as immutable inputs)`,
        failed: 'Archive expansion failed; no member was published. Retry or use another file.',
        alreadyExpanded: 'Expanded',
      };
}

export function CraftInputExpandPanel(props: CraftInputExpandPanelProps) {
  const strings = inputExpandStrings(props.locale);
  const archives = props.inputs.filter(isExpandableArchiveInput);
  const [expandingRef, setExpandingRef] = React.useState<string | null>(null);
  const [expanded, setExpanded] = React.useState<Record<string, readonly CraftInputView[]>>({});
  const [failedRef, setFailedRef] = React.useState<string | null>(null);
  if (archives.length === 0) return null;
  const expand = async (ref: string) => {
    setExpandingRef(ref);
    setFailedRef(null);
    try {
      const members = await props.onExpand(ref);
      setExpanded((prior) => ({ ...prior, [ref]: members }));
    } catch {
      // All-or-nothing: a failure publishes NOTHING; the entrance stays so
      // the member can retry deliberately.
      setFailedRef(ref);
    } finally {
      setExpandingRef(null);
    }
  };
  return (
    <section aria-label={strings.heading}>
      <h3>{strings.heading}</h3>
      <p className="wk-craft-hint">{strings.hint}</p>
      {archives.map((input) => {
        const busy = expandingRef === input.ref;
        const members = expanded[input.ref];
        return (
          <div key={input.ref} data-archive-ref={input.ref} aria-busy={busy || undefined}>
            <strong>{input.name}</strong> <code>{input.ref}</code>{' '}
            {members === undefined ? (
              <Button type="button" disabled={expandingRef !== null} onClick={() => void expand(input.ref)}>
                {busy ? strings.expanding : strings.expand}
              </Button>
            ) : (
              <span>{strings.alreadyExpanded} · {strings.expandedCount(members.length)}</span>
            )}
            {members !== undefined ? (
              <ul>
                {members.map((member) => (
                  <li key={member.ref}><code>{member.name}</code></li>
                ))}
              </ul>
            ) : null}
            {failedRef === input.ref ? <p role="alert">{strings.failed}</p> : null}
          </div>
        );
      })}
    </section>
  );
}
