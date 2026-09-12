/** Parity with Vue KnowledgeBase.vue multi-file upload (UploadConfirmDialog). */

export interface UploadEntry {
  file: File;
  name: string;
  size: number;
}

export type UploadEntryStatus = 'pending' | 'uploading' | 'done' | 'error';

export interface UploadEntryState {
  entry: UploadEntry;
  status: UploadEntryStatus;
  message?: string;
}

/** Collect a FileList/drop into stable entries before the confirm dialog. */
export function toUploadEntries(files: Iterable<File>): UploadEntry[] {
  return Array.from(files).map((file) => ({ file, name: file.name, size: file.size }));
}

export function formatBytes(size: number): string {
  if (!Number.isFinite(size) || size < 0) return '';
  if (size < 1024) return size + ' B';
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = size;
  let unit = 'B';
  for (const next of units) {
    if (value < 1024) break;
    value /= 1024;
    unit = next;
  }
  return (unit === 'B' ? String(value) : value.toFixed(1)) + ' ' + unit;
}

/** View-model for the confirm dialog: file names + sizes (+ tag_ids applied to all). */
export function uploadSummary(entries: readonly UploadEntry[]): { count: number; totalLabel: string } {
  const total = entries.reduce((sum, entry) => sum + entry.size, 0);
  return { count: entries.length, totalLabel: formatBytes(total) };
}

export interface RunUploadPipelineInput {
  entries: readonly UploadEntry[];
  tagIds?: string[];
  signal?: AbortSignal;
  /** One call per file; resolves with the created document. */
  upload(entry: UploadEntry, tagIds: string[] | undefined, signal: AbortSignal | undefined): Promise<unknown>;
  onStateChange?(states: readonly UploadEntryState[]): void;
}

/**
 * Sequential multi-file upload (Vue parity: one upload per file with a
 * progress panel). A per-file failure does not stop the remaining files;
 * an aborted signal stops the batch at the next file boundary.
 */
export async function runUploadPipeline(input: RunUploadPipelineInput): Promise<readonly UploadEntryState[]> {
  const states: UploadEntryState[] = input.entries.map((entry) => ({ entry, status: 'pending' }));
  const emit = () => input.onStateChange?.([...states]);
  emit();
  for (let index = 0; index < states.length; index += 1) {
    if (input.signal?.aborted) break;
    const state = states[index];
    state.status = 'uploading';
    emit();
    try {
      await input.upload(state.entry, input.tagIds, input.signal);
      state.status = 'done';
    } catch (cause) {
      state.status = 'error';
      state.message = cause instanceof Error ? cause.message : 'Upload failed';
    }
    emit();
  }
  return states;
}
