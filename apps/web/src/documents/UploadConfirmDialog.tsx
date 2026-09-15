import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Dialog, Status } from '@weknora/ui';
import { getUploadConfirmDefaultSection, getUploadConfirmSections, getUploadConfirmSourceItems, isUploadConfirmDismissible, validateUploadConfirm, type UploadConfirmMode, type UploadConfirmSection, type UploadConfirmSource } from './upload-confirm.ts';

export interface UploadConfirmDialogProps {
  open: boolean;
  mode?: UploadConfirmMode;
  files?: UploadConfirmSource[];
  urls?: string[];
  manualContent?: string;
  multimodalEnabled?: boolean;
  multimodalModelId?: string;
  asrEnabled?: boolean;
  asrModelId?: string;
  reparseFileName?: string;
  loading?: boolean;
  error?: string | null;
  onCancel: () => void;
  onConfirm: () => void;
  onRemoveFile?: (index: number) => void;
  onRemoveUrl?: (index: number) => void;
}

const sectionLabels: Record<UploadConfirmSection, string> = {
  tags: 'Tags', parser: 'Parser', chunking: 'Chunking', multimodal: 'Multimodal', asr: 'ASR', question: 'Questions',
};

export function UploadConfirmDialog({ open, mode = 'file', files = [], urls = [], manualContent, multimodalEnabled = false, multimodalModelId = '', asrEnabled = false, asrModelId = '', reparseFileName, loading = false, error = null, onCancel, onConfirm, onRemoveFile, onRemoveUrl }: UploadConfirmDialogProps) {
  const input = useMemo(() => ({ mode, files, urls, manualContent, multimodalEnabled, multimodalModelId, asrEnabled, asrModelId }), [asrEnabled, asrModelId, files, manualContent, mode, multimodalEnabled, multimodalModelId, urls]);
  const validation = validateUploadConfirm(input);
  const sourceItems = getUploadConfirmSourceItems(input);
  const sections = getUploadConfirmSections(mode).map((key) => ({ key, label: sectionLabels[key] }));
  const [activeSection, setActiveSection] = useState<UploadConfirmSection>(() => getUploadConfirmDefaultSection(input));
  const sectionRefs = useRef<Partial<Record<UploadConfirmSection, HTMLButtonElement | null>>>({});

  useEffect(() => {
    if (open) setActiveSection(getUploadConfirmDefaultSection(input));
  }, [input, open]);

  useEffect(() => {
    if (open) sectionRefs.current[activeSection]?.focus();
  }, [activeSection, open]);

  const chooseSection = (section: UploadConfirmSection) => setActiveSection(section);
  const confirm = () => {
    if (!isUploadConfirmDismissible(loading)) return;
    if (!validation.valid) {
      if (validation.firstIssueSection) setActiveSection(validation.firstIssueSection);
      return;
    }
    onConfirm();
  };

  const title = mode === 'manual' ? 'Confirm manual upload' : mode === 'reparse' ? 'Reparse document' : 'Confirm upload';
  const confirmLabel = mode === 'manual' ? 'Upload manual content' : mode === 'reparse' ? 'Reparse document' : 'Upload';

  return <Dialog open={open} title={title} onOpenChange={(next) => { if (!next && isUploadConfirmDismissible(loading)) onCancel(); }} className="wk-upload-confirm-dialog">
    <div className="wk-upload-confirm-layout">
      <aside aria-label="Upload sources">
        {mode === 'manual' ? <><strong>{manualContent ? 'Manual content' : 'No manual content'}</strong><p>{manualContent?.length ?? 0} characters</p></> : mode === 'reparse' ? <><strong>{reparseFileName || 'Source document'}</strong><p>Reparse existing document</p></> : sourceItems.length > 0 ? <ul>{sourceItems.map((item) => <li key={`${item.kind}-${item.index}`}><span title={item.label}>{item.label}</span><small> {item.meta}</small>{item.kind === 'file' && onRemoveFile ? <button type="button" aria-label={`Remove ${item.label}`} onClick={() => onRemoveFile(item.index)}>Remove</button> : null}{item.kind === 'url' && onRemoveUrl ? <button type="button" aria-label={`Remove ${item.label}`} onClick={() => onRemoveUrl(item.index)}>Remove</button> : null}</li>)}</ul> : <p>No items selected.</p>}
      </aside>
      <nav aria-label="Upload configuration" role="tablist">
        {sections.map((section) => <button key={section.key} ref={(node) => { sectionRefs.current[section.key] = node; }} type="button" role="tab" tabIndex={activeSection === section.key ? 0 : -1} aria-selected={activeSection === section.key} aria-controls={`upload-confirm-${section.key}`} onClick={() => chooseSection(section.key)}>{section.label}{validation.issues.includes(section.key) ? ' — needs setup' : ''}</button>)}
      </nav>
      {sections.map((section) => <section key={section.key} id={`upload-confirm-${section.key}`} role="tabpanel" aria-label={`${section.key} settings`} hidden={activeSection !== section.key}>
        {validation.issues.includes(section.key) ? <Status tone="error">This section needs setup before continuing.</Status> : null}
        {section.key === 'tags' ? <p>{files.length + urls.length} source{files.length + urls.length === 1 ? '' : 's'} selected.</p> : <p>Configure {section.key} settings for this upload.</p>}
      </section>)}
    </div>
    {error ? <Status tone="error">Upload failed: {error}</Status> : null}
    {!validation.valid && !validation.firstIssueSection ? <Status tone="error">Select a file, URL, or manual content before continuing.</Status> : null}
    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
      <Button type="button" variant="text" onClick={() => { if (isUploadConfirmDismissible(loading)) onCancel(); }} disabled={loading}>Cancel</Button>
      <Button type="button" variant="primary" loading={loading} disabled={!validation.valid || loading} onClick={confirm}>{loading ? 'Submitting…' : confirmLabel}</Button>
    </div>
  </Dialog>;
}

export type { UploadConfirmMode, UploadConfirmSection, UploadConfirmSource } from './upload-confirm.ts';
