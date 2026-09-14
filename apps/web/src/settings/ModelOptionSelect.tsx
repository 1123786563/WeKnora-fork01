import { useEffect, useId, useRef, useState } from 'react';

export type ModelOption = { value: string; label: string; description?: string };

export function ModelOptionSelect({ value, options, disabled = false, clearable = false, clearLabel = 'Clear selection', addModelLabel, onAddModel, onChange }: {
  value: string;
  options: readonly ModelOption[];
  disabled?: boolean;
  clearable?: boolean;
  clearLabel?: string;
  addModelLabel?: string;
  onAddModel?: () => void;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(() => Math.max(0, options.findIndex((option) => option.value === value)));
  const rootRef = useRef<HTMLDivElement>(null);
  const listboxId = `wk-model-option-list-${useId()}`;
  const selected = options.find((option) => option.value === value) ?? options[0];

  useEffect(() => {
    if (!open) return;
    const close = (event: MouseEvent) => { if (!rootRef.current?.contains(event.target as Node)) setOpen(false); };
    document.addEventListener('mousedown', close);
    return () => document.removeEventListener('mousedown', close);
  }, [open]);
  useEffect(() => {
    const index = options.findIndex((option) => option.value === value);
    setActiveIndex(index >= 0 ? index : 0);
  }, [options, value]);

  function choose(index: number) {
    if (index === -1 && clearable) {
      onChange('');
      setOpen(false);
      return;
    }
    const option = options[index];
    if (!option) return;
    onChange(option.value);
    setActiveIndex(index);
    setOpen(false);
  }

  return <div className="wk-model-option-select relative w-full" ref={rootRef}>
    <button type="button" className="wk-model-option-select__trigger flex h-8 w-full items-center justify-between rounded-[3px] border border-[#dcdcdc] bg-white px-2.5 text-left text-[13px] text-black/90 transition-colors hover:border-[#07c05f] focus:border-[#07c05f] focus:outline-none focus:ring-2 focus:ring-[#07c05f]/20 disabled:cursor-not-allowed disabled:border-[#dcdcdc] disabled:bg-[#eee] disabled:text-black/25" role="combobox" value={value} data-value={value} aria-expanded={open} aria-controls={listboxId} disabled={disabled} onClick={() => setOpen((current) => !current)} onKeyDown={(event) => {
      if (disabled) return;
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault(); setOpen(true);
        setActiveIndex((current) => (current + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length);
      } else if (event.key === 'Enter' && open) { event.preventDefault(); choose(activeIndex); }
      else if (event.key === 'Escape') setOpen(false);
    }}>
      <span className="wk-model-option-select__value flex min-w-0 flex-1 overflow-hidden"><span className="truncate">{selected?.label ?? value}</span>{selected?.description ? <span className="wk-model-option-select__description ml-2 truncate text-xs text-[#8a8a8a]">{selected.description}</span> : null}</span>
      <span className="wk-model-option-select__chevron ml-2 text-base leading-none text-black/40" aria-hidden="true">⌄</span>
    </button>
    {open ? <div id={listboxId} className="wk-model-option-select__popup absolute left-0 right-0 top-[calc(100%+4px)] z-20 grid max-h-[280px] min-w-[22rem] overflow-auto rounded-md border border-[#e7e7e7] bg-white p-1 shadow-[0_8px_24px_rgba(23,32,51,.14)] max-[720px]:min-w-0" role="listbox">{clearable ? <button type="button" role="option" aria-selected={value === ''} className="m-0.5 flex w-full rounded-md border-0 px-2.5 py-2 text-left text-[13px] text-black/50 hover:bg-[#07c05f]/[.08]" onClick={() => choose(-1)}>{clearLabel}</button> : null}{options.map((option, index) => <button type="button" role="option" aria-selected={option.value === value} data-value={option.value} className={`wk-model-option-select__option m-0.5 flex w-full flex-col gap-0.5 rounded-md border-0 px-2.5 py-2 text-left text-[13px] ${index === activeIndex ? 'bg-[#07c05f]/[.08]' : ''} ${option.value === value ? 'relative bg-[#07c05f]/[.12] before:absolute before:bottom-2 before:left-0 before:top-2 before:w-[3px] before:rounded-r-sm before:bg-[#07c05f]' : ''}`} key={option.value} onMouseEnter={() => setActiveIndex(index)} onClick={() => choose(index)}><span className="wk-model-option-select__option-title">{option.label}</span>{option.description ? <span className="wk-model-option-select__option-description text-xs text-[#8a8a8a]">{option.description}</span> : null}</button>)}{onAddModel && addModelLabel ? <button type="button" className="m-0.5 border-0 border-t border-[#e7e7e7] px-2.5 py-2 text-left text-[13px] font-medium text-[#0a8f4c]" onClick={() => { onAddModel(); setOpen(false); }}>{addModelLabel}</button> : null}</div> : null}
  </div>;
}
