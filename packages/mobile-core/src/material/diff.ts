export interface DiffLine {
  origin: 'context' | 'add' | 'remove';
  text: string;
}

export interface DiffHunk {
  header: string;
  oldStart: number;
  newStart: number;
  lines: DiffLine[];
}

const HUNK_HEADER = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/;

/**
 * git 原生 diff 的段扩展头（出现在 diff --git 之后、---/+++ 之前）。自包含单行，
 * 不携带后续载荷——"GIT binary patch" 及其 base85 载荷故意不在列：文本渲染器对
 * 二进制补丁就该整体回退原始文本。
 */
const GIT_HEADER_PREFIXES = [
  'index ', 'old mode ', 'new mode ', 'deleted file mode ', 'new file mode ',
  'similarity index ', 'dissimilarity index ',
  'rename from ', 'rename to ', 'copy from ', 'copy to ', 'Binary files ',
] as const;

function isGitExtendedHeader(line: string): boolean {
  return GIT_HEADER_PREFIXES.some((prefix) => line.startsWith(prefix));
}

/**
 * 解析 unified diff。纪律：接受 git 原生 diff（含 index/new file mode/rename 等
 * 段扩展头；diff --git 行是段边界，多文件依次解析）；hunk 内的真空行（编辑器常
 * 剥掉 context 空行的前导空格）按 context 解析；"\ No newline at end of file"
 * 标记行忽略；一旦出现无法归类的行即整体 malformed——呈现层回退到原始文本，
 * 绝不展示半解析结果。空文本与只有文件头的输入是合法的空 diff。
 * 注意：hunk 头声明的行数不参与校验（真实工具产出的头计数时有出入，宽松读入）；
 * 畸形判定只依赖行首前缀与位置；hunk 内以 ---/+++ 开头的行与文件头有歧义，
 * 判畸形（宁可回退原文也不产出错位的半解析结果）。
 */
export function parseUnifiedDiff(text: string): { hunks: DiffHunk[]; malformed: boolean } {
  if (typeof text !== 'string' || text.trim() === '') return { hunks: [], malformed: false };
  const lines = text.split('\n');
  // 末尾空行是换行符产物，不是内容。
  if (lines.length > 0 && lines[lines.length - 1] === '') lines.pop();
  const hunks: DiffHunk[] = [];
  let current: DiffHunk | undefined;
  for (const line of lines) {
    if (line.startsWith('diff --git ')) {
      current = undefined; // 段边界：下一个文件的扩展头/文件头/hunk 从这里重新开始
      continue;
    }
    if (line.startsWith('--- ') || line.startsWith('+++ ')) {
      if (current !== undefined) return { hunks: [], malformed: true }; // 文件头只出现在 hunk 之外；hunk 内出现即歧义，判畸形
      continue;
    }
    if (current === undefined && isGitExtendedHeader(line)) continue;
    const header = HUNK_HEADER.exec(line);
    if (header !== null) {
      current = { header: line, oldStart: Number(header[1]), newStart: Number(header[2]), lines: [] };
      hunks.push(current);
      continue;
    }
    if (current === undefined) {
      if (line.trim() === '') continue;
      return { hunks: [], malformed: true };
    }
    if (line === '') current.lines.push({ origin: 'context', text: '' }); // 被剥掉前导空格的空 context 行
    else if (line.startsWith(' ')) current.lines.push({ origin: 'context', text: line.slice(1) });
    else if (line.startsWith('+')) current.lines.push({ origin: 'add', text: line.slice(1) });
    else if (line.startsWith('-')) current.lines.push({ origin: 'remove', text: line.slice(1) });
    else if (line.startsWith('\\')) continue; // "\ No newline at end of file"
    else return { hunks: [], malformed: true };
  }
  return { hunks, malformed: false };
}
