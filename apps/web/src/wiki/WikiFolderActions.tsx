// WikiFolderActions——Vue frontend/src/views/knowledge/wiki/WikiFolderActions.vue
// 同构平移。目录行尾的「…」锚定弹层：菜单（新建子目录/重命名/删除）→ 内嵌
// create（名称表单）/ delete（确认表单）三态都在同一个 t-popup 里完成。
import { useRef, useState } from 'react';
import { Button, Input, Popup } from 'tdesign-react';
import { Icon as TIcon } from 'tdesign-icons-react';
import { createTranslator } from '../i18n.ts';

type Translate = ReturnType<typeof createTranslator>;

export interface WikiFolderActionsProps {
  t: Translate;
  name?: string;
  pageCount?: number;
  hasChildren?: boolean;
  onCreate: (name: string) => void;
  onRename: () => void;
  onDelete: () => void;
}

export function WikiFolderActions(props: WikiFolderActionsProps) {
  const { t, name = '', pageCount = 0, hasChildren = false, onCreate, onRename, onDelete } = props;
  const [open, setOpen] = useState(false);
  const [mode, setMode] = useState<'menu' | 'create' | 'delete'>('menu');
  const [nameInput, setNameInput] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  const deletable = pageCount === 0 && !hasChildren;
  // 菜单态走 card-more-popup 骨架（dropdown-menu.less），表单态走 anchored-form。
  const overlayClassName = mode === 'menu'
    ? 'card-more-popup wiki-folder-action-overlay'
    : 'anchored-form-popup-overlay';

  function handleVisibleChange(visible: boolean) {
    setOpen(visible);
    if (!visible) {
      setMode('menu');
      setNameInput('');
    }
  }

  function enterCreate() {
    setMode('create');
    setNameInput('');
    requestAnimationFrame(() => inputRef.current?.focus());
  }

  function submitName() {
    const value = nameInput.trim();
    if (!value) return;
    onCreate(value);
    setOpen(false);
  }

  return (
    <Popup
      visible={open}
      trigger="click"
      /* 台账 #15：React PopupPlacement 无 -start/-end 粒度，bottom-left 近似
         Vue bottom-start（边缘对齐语义）。 */
      placement="bottom-left"
      destroyOnClose
      overlayClassName={overlayClassName}
      onVisibleChange={handleVisibleChange}
      content={(
        <div className="wiki-folder-menu" onClick={(event) => event.stopPropagation()}>
          {mode === 'menu' ? (
            <div className="popup-menu">
              <div className="popup-menu-item" onClick={enterCreate}>
                <TIcon name="folder-add" className="menu-icon" />
                <span>{t('wikiBrowser.newSubfolder')}</span>
              </div>
              <div className="popup-menu-item" onClick={() => { onRename(); setOpen(false); }}>
                <TIcon name="edit" className="menu-icon" />
                <span>{t('wikiBrowser.renameFolder')}</span>
              </div>
              <div className="popup-menu-item delete" onClick={() => setMode('delete')}>
                <TIcon name="delete" className="menu-icon" />
                <span>{t('wikiBrowser.deleteFolder')}</span>
              </div>
            </div>
          ) : mode === 'create' ? (
            <div className="anchored-form-popup-inner">
              <div className="anchored-form-popup-title">{t('wikiBrowser.newSubfolder')}</div>
              <Input
                ref={inputRef as never}
                value={nameInput}
                onChange={(value) => setNameInput(String(value ?? ''))}
                placeholder={t('wikiBrowser.folderNamePlaceholder')}
                onEnter={() => submitName()}
              />
              <div className="anchored-form-popup-footer">
                <Button variant="outline" onClick={() => setOpen(false)}>{t('common.cancel')}</Button>
                <Button theme="primary" disabled={!nameInput.trim()} onClick={() => submitName()}>{t('common.confirm')}</Button>
              </div>
            </div>
          ) : (
            <div className="anchored-form-popup-inner">
              <div className="anchored-form-popup-title">{t('wikiBrowser.deleteFolder')}</div>
              <div className="anchored-form-popup-body">
                {deletable
                  ? t('wikiBrowser.deleteFolderConfirm', { name })
                  : t('wikiBrowser.deleteFolderNotEmpty')}
              </div>
              <div className="anchored-form-popup-footer">
                <Button variant="outline" onClick={() => setOpen(false)}>
                  {deletable ? t('common.cancel') : t('common.confirm')}
                </Button>
                {deletable ? (
                  <Button theme="danger" onClick={() => { onDelete(); setOpen(false); }}>{t('common.confirm')}</Button>
                ) : null}
              </div>
            </div>
          )}
        </div>
      )}
    >
      <span
        className={`wiki-directory-action wiki-directory-action--reveal${open ? ' is-open' : ''}`}
        title={t('wikiBrowser.folderActions')}
        onClick={(event) => event.stopPropagation()}
        onDragStart={(event) => event.stopPropagation()}
      >
        <TIcon name="more" />
      </span>
    </Popup>
  );
}
