export interface WikiDiffLine {
  type: 'same' | 'add' | 'del';
  text: string;
}

const LCS_LINE_LIMIT = 1500;

export function diffWikiLines(oldText: string, newText: string): WikiDiffLine[] {
  const oldLines = oldText ? oldText.split('\n') : [];
  const newLines = newText ? newText.split('\n') : [];
  let start = 0;
  while (start < oldLines.length && start < newLines.length && oldLines[start] === newLines[start]) start++;
  let oldEnd = oldLines.length;
  let newEnd = newLines.length;
  while (oldEnd > start && newEnd > start && oldLines[oldEnd - 1] === newLines[newEnd - 1]) {
    oldEnd--;
    newEnd--;
  }
  const prefix = oldLines.slice(0, start).map((text): WikiDiffLine => ({ type: 'same', text }));
  const suffix = oldLines.slice(oldEnd).map((text): WikiDiffLine => ({ type: 'same', text }));
  const oldMid = oldLines.slice(start, oldEnd);
  const newMid = newLines.slice(start, newEnd);
  let middle: WikiDiffLine[];
  if (oldMid.length === 0) middle = newMid.map((text): WikiDiffLine => ({ type: 'add', text }));
  else if (newMid.length === 0) middle = oldMid.map((text): WikiDiffLine => ({ type: 'del', text }));
  else if (oldMid.length > LCS_LINE_LIMIT || newMid.length > LCS_LINE_LIMIT) {
    middle = [...oldMid.map((text): WikiDiffLine => ({ type: 'del', text })), ...newMid.map((text): WikiDiffLine => ({ type: 'add', text }))];
  } else middle = lcsDiff(oldMid, newMid);
  return [...prefix, ...middle, ...suffix];
}

function lcsDiff(oldLines: string[], newLines: string[]): WikiDiffLine[] {
  const dp: Uint32Array[] = Array.from({ length: oldLines.length + 1 }, () => new Uint32Array(newLines.length + 1));
  for (let i = oldLines.length - 1; i >= 0; i--) {
    for (let j = newLines.length - 1; j >= 0; j--) {
      dp[i][j] = oldLines[i] === newLines[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const output: WikiDiffLine[] = [];
  let i = 0;
  let j = 0;
  while (i < oldLines.length && j < newLines.length) {
    if (oldLines[i] === newLines[j]) output.push({ type: 'same', text: oldLines[i++] }), j++;
    else if (dp[i + 1][j] >= dp[i][j + 1]) output.push({ type: 'del', text: oldLines[i++] });
    else output.push({ type: 'add', text: newLines[j++] });
  }
  while (i < oldLines.length) output.push({ type: 'del', text: oldLines[i++] });
  while (j < newLines.length) output.push({ type: 'add', text: newLines[j++] });
  return output;
}

export interface WikiRevisionSnapshot { title: string; summary: string; content: string; }
export type WikiRevisionDiffField = keyof WikiRevisionSnapshot;
export interface WikiRevisionDiffSection { field: WikiRevisionDiffField; lines: WikiDiffLine[]; }

export function diffWikiRevision(oldSnap: WikiRevisionSnapshot, newSnap: WikiRevisionSnapshot): WikiRevisionDiffSection[] {
  const sections: WikiRevisionDiffSection[] = [];
  for (const field of ['title', 'summary', 'content'] as const) {
    const lines = diffWikiLines(oldSnap[field], newSnap[field]);
    if (lines.some((line) => line.type !== 'same')) sections.push({ field, lines });
  }
  return sections;
}
