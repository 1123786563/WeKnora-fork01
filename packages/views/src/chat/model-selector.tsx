import type { ChatCopyTable } from './chat-copy.ts';
import { SpriteIcon } from './message-face.tsx';

/*
 * Composer 模型下拉（px-chat-model-selector 收敛件）。
 * Vue 事实源：frontend/src/components/Input-field.vue:2806-2838 模板 +
 * :1107-1215 updateModelDropdownPosition + :3673-3853 样式。
 * Vue 端 Teleport to body；views 包无 react-dom（agent-selector.tsx 同判例），
 * 以 in-tree position:fixed 视口锚定达到同一几何/层叠。
 */

export interface ModelSelectorOption {
  id: string;
  /** display_name || name（Vue modelDisplayName 链，宿主已解析）。 */
  name: string;
  /** 原始 name —— 仅当 display_name 存在时作为次级灰字展示。 */
  rawName?: string;
  /** formatContextWindow 产物（'200K'）；宿主解析。 */
  contextLabel?: string;
  contextIsDefault?: boolean;
}

export interface ModelSelectorProps {
  copy: ChatCopyTable;
  options: readonly ModelSelectorOption[];
  selectedModelId?: string;
  anchorRect: DOMRect;
  onSelect(modelId: string): void;
  /** 头部「+ 添加模型」——Vue handleGoToConversationModels。 */
  onAddModel(): void;
  onClose(): void;
}

/** Input-field.vue updateModelDropdownPosition（1107-1215）逐值平移：
 * 左对齐触发器左缘；下方放不下 200px+8 时以 bottom 锚定贴触发器上缘。 */
export function modelDropdownStyle(anchor: DOMRect): React.CSSProperties {
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const dropdownWidth = 280;
  const offsetY = 8;

  let left = Math.floor(anchor.left);
  const minLeft = 16;
  const maxLeft = Math.max(16, vw - dropdownWidth - 16);
  left = Math.max(minLeft, Math.min(maxLeft, left));

  const preferredDropdownHeight = 280;
  const maxDropdownHeight = 360;
  const minDropdownHeight = 200;
  const topMargin = 20;
  const spaceBelow = vh - anchor.bottom;
  const spaceAbove = anchor.top;

  if (spaceBelow >= minDropdownHeight + offsetY) {
    return {
      position: 'fixed',
      width: `${dropdownWidth}px`,
      left: `${left}px`,
      top: `${Math.floor(anchor.bottom + offsetY)}px`,
      maxHeight: `${Math.min(preferredDropdownHeight, spaceBelow - offsetY - 16)}px`,
      zIndex: 10000,
      margin: 0,
      padding: 0,
    };
  }
  const availableHeight = spaceAbove - offsetY - topMargin;
  const actualHeight = availableHeight >= preferredDropdownHeight
    ? preferredDropdownHeight
    : Math.max(minDropdownHeight, availableHeight);
  return {
    position: 'fixed',
    width: `${dropdownWidth}px`,
    left: `${left}px`,
    bottom: `${vh - anchor.top + offsetY}px`,
    maxHeight: `${actualHeight}px`,
    zIndex: 10000,
    margin: 0,
    padding: 0,
  };
}

export function ModelSelectorPanel(props: ModelSelectorProps) {
  const { copy, options, selectedModelId } = props;
  return (
    <div className="model-selector-overlay" onClick={props.onClose}>
      <div
        role="dialog"
        aria-label={copy.modelSelectorChatGroup}
        className="model-selector-dropdown"
        style={modelDropdownStyle(props.anchorRect)}
        onClick={(event) => event.stopPropagation()}
      >
        <div className="model-selector-header">
          <span>{copy.modelSelectorChatGroup}</span>
          <button type="button" className="model-selector-add" onClick={props.onAddModel}>
            <span className="add-icon" aria-hidden="true">+</span>
            <span className="add-text">{copy.modelAddModel}</span>
          </button>
        </div>
        <div className="model-selector-content">
          {options.map((model) => (
            <div
              key={model.id}
              className={'model-option' + (model.id === selectedModelId ? ' selected' : '')}
              data-model-id={model.id}
              onClick={() => props.onSelect(model.id)}
            >
              <div className="model-option-left">
                <div className="model-option-icon">
                  <SpriteIcon name="chat" size="14px" />
                </div>
                <div className="model-option-name-wrap">
                  <span className="model-option-name">{model.name}</span>
                  {model.rawName ? <span className="model-option-raw-name">{model.rawName}</span> : null}
                </div>
              </div>
              <span
                className={'model-option-ctx' + (model.contextIsDefault ? ' is-default' : '')}
              >{model.contextLabel}</span>
            </div>
          ))}
          {options.length === 0 ? (
            <div className="model-option empty">{copy.modelNoModel}</div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
