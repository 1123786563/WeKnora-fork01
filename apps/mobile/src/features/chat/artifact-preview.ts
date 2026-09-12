import type { ChatArtifact } from '@weknora/domain/chat/artifacts';

export type NativeArtifactPreviewKind = 'text' | 'markdown' | 'image' | 'download-only';

export interface NativeArtifactPreviewModel {
  kind: NativeArtifactPreviewKind;
  label: string;
}

export interface NativeMarkdownLine {
  kind: 'heading' | 'bullet' | 'code' | 'text';
  text: string;
}

function extension(fileName: string): string {
  return fileName.trim().toLowerCase().split('.').pop() ?? '';
}

export function classifyNativeArtifactPreview(artifact: Pick<ChatArtifact, 'fileName' | 'fileType'>): NativeArtifactPreviewModel {
  const type = artifact.fileType?.trim().toLowerCase() ?? '';
  const ext = extension(artifact.fileName);
  if (type === 'text/html' || type === 'application/xhtml+xml' || ext === 'html' || ext === 'htm') return { kind: 'download-only', label: 'Download to view' };
  if (type === 'application/pdf' || ext === 'pdf') return { kind: 'download-only', label: 'Download to view' };
  if (type.startsWith('image/') && !type.startsWith('image/svg')) return { kind: 'image', label: 'Image preview' };
  if (ext === 'md' || ext === 'markdown' || type === 'text/markdown') return { kind: 'markdown', label: 'Markdown preview' };
  if (type.startsWith('text/') || ['csv', 'tsv', 'txt', 'log', 'json', 'xml', 'yaml', 'yml', 'mmd', 'mermaid'].includes(ext)) return { kind: 'text', label: 'Text preview' };
  return { kind: 'download-only', label: 'Download to view' };
}

/** Project Markdown into native text styles; never interpret the source as HTML. */
export function nativeMarkdownLines(markdown: string): NativeMarkdownLine[] {
  const result: NativeMarkdownLine[] = [];
  let inCode = false;
  for (const rawLine of markdown.split(/\r?\n/)) {
    const line = rawLine.trimEnd();
    if (/^\s*```/.test(line)) { inCode = !inCode; continue; }
    if (!line.trim()) continue;
    if (inCode) { result.push({ kind: 'code', text: line }); continue; }
    const heading = line.match(/^\s{0,3}#{1,6}\s+(.+)$/);
    if (heading) { result.push({ kind: 'heading', text: heading[1]!.trim() }); continue; }
    const bullet = line.match(/^\s*[-*+]\s+(.+)$/);
    if (bullet) { result.push({ kind: 'bullet', text: bullet[1]!.trim() }); continue; }
    result.push({ kind: 'text', text: line.trim() });
  }
  return result;
}
