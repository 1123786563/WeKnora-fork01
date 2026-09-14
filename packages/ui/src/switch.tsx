import type { ButtonHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

type SwitchProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> & {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
};

export function Switch({ checked, onCheckedChange, className, disabled, ...props }: SwitchProps) {
  return <button type="button" role="switch" aria-checked={checked} disabled={disabled} className={cn('relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full bg-[rgba(120,135,155,.35)] transition-colors focus-visible:outline focus-visible:outline-1 focus-visible:outline-[#07c05f] focus-visible:ring-2 focus-visible:ring-[#07c05f]/20 disabled:cursor-not-allowed disabled:opacity-50 data-[state=checked]:bg-[#07c05f]', className)} data-state={checked ? 'checked' : 'unchecked'} onClick={() => onCheckedChange(!checked)} {...props}>
    <span aria-hidden="true" className={cn('pointer-events-none block h-4 w-4 translate-x-0.5 rounded-full bg-white transition-transform', checked && 'translate-x-[18px]')} />
  </button>;
}
