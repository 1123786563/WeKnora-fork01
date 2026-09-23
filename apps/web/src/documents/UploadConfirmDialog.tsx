import { useEffect, useMemo, useRef, useState } from 'react';
// S6 换装（T15 前置）：packages/ui 旧栈 离栈，Button/Dialog 换 tdesign（playbook §1：
// open/title/className → visible/header/dialogClassName；variant text/primary 直译/换 theme）；
// Status 无 TDesign 对应，走 shared/wk-legacy（.wk-status 族）。
import { Button as TButton, Dialog as TDialog } from 'tdesign-react';
import { WkStatus as Status } from '../shared/wk-legacy.tsx';
import { getUploadConfirmDefaultSection, getUploadConfirmSections, isUploadConfirmDismissible, requestUploadConfirmClose, validateUploadConfirm, type UploadConfirmMode, type UploadConfirmSection, type UploadConfirmSource } from './upload-confirm.ts';
import './documents-u.css';

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

const sectionLabels: Record<UploadConfirmSection, string> = {
  tags: 'Tags', parser: 'Parser', chunking: 'Chunking', multimodal: 'Multimodal', asr: 'ASR', question: 'Questions',
};

export function UploadConfirmDialog({ open, mode = 'file', files = [], urls = [], manualContent, multimodalEnabled = false, multimodalModelId = '', asrEnabled = false, asrModelId = '', loading = false, onCancel, onConfirm }: UploadConfirmDialogProps) {
  const input = useMemo(() => ({ mode, files, urls, manualContent, multimodalEnabled, multimodalModelId, asrEnabled, asrModelId }), [asrEnabled, asrModelId, files, manualContent, mode, multimodalEnabled, multimodalModelId, urls]);
  const validation = validateUploadConfirm(input);
  const sections = getUploadConfirmSections(mode).map((key) => ({ key, label: sectionLabels[key] }));
  const [activeSection, setActiveSection] = useState<UploadConfirmSection>(() => getUploadConfirmDefaultSection(input));
  const sectionRefs = useRef<Partial<Record<UploadConfirmSection, HTMLButtonElement | null>>>({});

  useEffect(() => {
    if (open) setActiveSection(getUploadConfirmDefaultSection(input));
  }, [input, open]);

  useEffect(() => {
    if (open) sectionRefs.current[activeSection]?.focus();
  }, [activeSection, open]);

  const close = () => requestUploadConfirmClose(loading, onCancel);
  const confirm = () => {
    if (!isUploadConfirmDismissible(loading)) return;
    if (!validation.valid) {
      if (validation.firstIssueSection) setActiveSection(validation.firstIssueSection);
      return;
    }
    onConfirm();
  };

  return <TDialog visible={open} header={mode === 'reparse' ? 'Reparse document' : 'Confirm upload'} footer={false} onClose={close} dialogClassName="wk-upload-confirm-dialog">
    <div className="wk-upload-confirm-layout">
      <nav aria-label="Upload configuration" role="tablist">
        {sections.map((section) => <button key={section.key} ref={(node) => { sectionRefs.current[section.key] = node; }} type="button" role="tab" tabIndex={activeSection === section.key ? 0 : -1} aria-selected={activeSection === section.key} aria-controls={`upload-confirm-${section.key}`} onClick={() => setActiveSection(section.key)}>{section.label}{validation.issues.includes(section.key) ? ' — needs setup' : ''}</button>)}
      </nav>
      {sections.map((section) => <section key={section.key} id={`upload-confirm-${section.key}`} role="tabpanel" aria-label={`${section.key} settings`} hidden={activeSection !== section.key}>
        {validation.issues.includes(section.key) ? <Status tone="error">This section needs setup before continuing.</Status> : null}
        {section.key === 'tags' ? <p>{files.length + urls.length} source{files.length + urls.length === 1 ? '' : 's'} selected.</p> : <p>Configure {section.key} settings for this upload.</p>}
      </section>)}
    </div>
    {!validation.valid && !validation.firstIssueSection ? <Status tone="error">Select a file, URL, or manual content before continuing.</Status> : null}
    <div className="wk-ucd-1">
      <TButton type="button" variant="text" onClick={close} disabled={loading}>Cancel</TButton>
      <TButton type="button" theme="primary" loading={loading} disabled={!validation.valid} onClick={confirm}>Confirm</TButton>
    </div>
  </TDialog>;
}

export type { UploadConfirmMode, UploadConfirmSection, UploadConfirmSource } from './upload-confirm.ts';
