import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';
import { Input } from './input.tsx';

type NumberInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'type' | 'value' | 'onChange'> & {
  value: number | '';
  min: number;
  max: number;
  step?: number;
  onValueChange: (value: number | '') => void;
};

export function NumberInput({ value, min, max, step = 1, onValueChange, className, ...props }: NumberInputProps) {
  const change = (next: number | '') => {
    if (next === '') return onValueChange('');
    onValueChange(Math.min(max, Math.max(min, next)));
  };
  return <div className={cn('flex h-8 w-full overflow-hidden rounded-[3px] border border-[#dcdcdc] bg-white focus-within:border-[#07c05f] focus-within:ring-2 focus-within:ring-[#07c05f]/20', className)}>
    <Input {...props} className="h-full min-w-0 flex-1 rounded-none border-0 px-2 focus:ring-0" type="number" min={min} max={max} step={step} value={value} onChange={(event) => change(event.target.value === '' ? '' : Number(event.target.value))} />
    <div className="flex w-6 shrink-0 flex-col border-l border-[#e7e7e7]">
      <button type="button" aria-label="Increase" className="h-1/2 text-[10px] leading-none text-black/50 hover:bg-[#07c05f]/10 disabled:cursor-not-allowed disabled:text-black/20" disabled={value !== '' && value >= max} onClick={() => change((value === '' ? min : value) + step)}>▲</button>
      <button type="button" aria-label="Decrease" className="h-1/2 text-[10px] leading-none text-black/50 hover:bg-[#07c05f]/10 disabled:cursor-not-allowed disabled:text-black/20" disabled={value !== '' && value <= min} onClick={() => change((value === '' ? min : value) - step)}>▼</button>
    </div>
  </div>;
}
