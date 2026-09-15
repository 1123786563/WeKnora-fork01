import { useEffect, useMemo, useRef, useState } from 'react';
import { Button, Dialog, Status } from '@weknora/ui';
import { getUploadConfirmDefaultSection, validateUploadConfirm, type UploadConfirmMode, type UploadConfirmSection, type UploadConfirmSource } from './upload-confirm.ts';

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
  loading?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

const sections: Array<{ key: UploadConfirmSection; label: string }> = [
  { key: 'tags', label: 'Tags' },
  { key: 'parser', label: 'Parser' },
  { key: 'chunking', label: 'Chunking' },
  { key: 'multimodal', label: 'Multimodal' },
  { key: 'asr', label: 'ASR' },
  { key: 'question', label: 'Questions' },
];

export function UploadConfirmDialog({ open, mode = 'file', files = [], urls = [], manualContent, multimodalEnabled = false, multimodalModelId = '', asrEnabled = false, asrModelId = '', loading = false, onCancel, onConfirm }: UploadConfirmDialogProps) {
  const input = useMemo(() => ({ mode, files, urls, manualContent, multimodalEnabled, multimodalModelId, asrEnabled, asrModelId }), [asrEnabled, asrModelId, files, manualContent, mode, multimodalEnabled, multimodalModelId, urls]);
  const validation = validateUploadConfirm(input);
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
    if (!validation.valid) {
      if (validation.firstIssueSection) setActiveSection(validation.firstIssueSection);
      return;
    }
    onConfirm();
  };

  return <Dialog open={open} title={mode === 'reparse' ? 'Reparse document' : 'Confirm upload'} onOpenChange={(next) => { if (!next && !loading) onCancel(); }} className="wk-upload-confirm-dialog">
    <div className="wk-upload-confirm-layout">
      <nav aria-label="Upload configuration" role="tablist">
        {sections.map((section) => <button key={section.key} ref={(node) => { sectionRefs.current[section.key] = node; }} type="button" role="tab" aria-selected={activeSection === section.key} aria-controls={`upload-confirm-${section.key}`} onClick={() => chooseSection(section.key)}>{section.label}{validation.issues.includes(section.key) ? ' — needs setup' : ''}</button>)}
      </nav>
      <section id={`upload-confirm-${activeSection}`} role="tabpanel" aria-label={`${activeSection} settings`}>
        {validation.issues.includes(activeSection) ? <Status tone="error">This section needs setup before continuing.</Status> : null}
        {activeSection === 'tags' ? <p>{files.length + urls.length} source{files.length + urls.length === 1 ? '' : 's'} selected.</p> : <p>Configure {activeSection} settings for this upload.</p>}
      </section>
    </div>
    {!validation.valid && !validation.firstIssueSection ? <Status tone="error">Select a file, URL, or manual content before continuing.</Status> : null}
    <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
      <Button type="button" variant="text" onClick={onCancel} disabled={loading}>Cancel</Button>
      <Button type="button" variant="primary" loading={loading} disabled={!validation.valid} onClick={confirm}>Confirm</Button>
    </div>
  </Dialog>;
}

export type { UploadConfirmMode, UploadConfirmSection, UploadConfirmSource } from './upload-confirm.ts';
