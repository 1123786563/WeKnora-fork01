import { useEffect, useId, useRef, useState } from 'react';

export type ModelOption = { value: string; label: string; description?: string };

export function ModelOptionSelect({ value, options, disabled = false, onChange }: {
  value: string;
  options: readonly ModelOption[];
  disabled?: boolean;
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
    const option = options[index];
    if (!option) return;
    onChange(option.value);
    setActiveIndex(index);
    setOpen(false);
  }

  return <div className="wk-model-option-select" ref={rootRef}>
    <button type="button" className="wk-model-option-select__trigger" role="combobox" value={value} data-value={value} aria-expanded={open} aria-controls={listboxId} disabled={disabled} onClick={() => setOpen((current) => !current)} onKeyDown={(event) => {
      if (disabled) return;
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        event.preventDefault(); setOpen(true);
        setActiveIndex((current) => (current + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length);
      } else if (event.key === 'Enter' && open) { event.preventDefault(); choose(activeIndex); }
      else if (event.key === 'Escape') setOpen(false);
    }}>
      <span className="wk-model-option-select__value"><span>{selected?.label ?? value}</span>{selected?.description ? <span className="wk-model-option-select__description">{selected.description}</span> : null}</span>
      <span className="wk-model-option-select__chevron" aria-hidden="true">⌄</span>
    </button>
    {open ? <div id={listboxId} className="wk-model-option-select__popup" role="listbox">{options.map((option, index) => <button type="button" role="option" aria-selected={option.value === value} data-value={option.value} className={`wk-model-option-select__option${index === activeIndex ? ' is-active' : ''}${option.value === value ? ' is-selected' : ''}`} key={option.value} onMouseEnter={() => setActiveIndex(index)} onClick={() => choose(index)}><span className="wk-model-option-select__option-title">{option.label}</span>{option.description ? <span className="wk-model-option-select__option-description">{option.description}</span> : null}</button>)}</div> : null}
  </div>;
}
