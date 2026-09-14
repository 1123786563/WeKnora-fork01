import type { ButtonHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

type SwitchProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> & {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
};

// Styled with the package's own plain CSS (styles.css): page-level Tailwind
// utilities are not guaranteed to be generated for classes used only inside
// this package, which previously collapsed the switch to a sliver.
export function Switch({ checked, onCheckedChange, className, disabled, ...props }: SwitchProps) {
  return <button type="button" role="switch" aria-checked={checked} disabled={disabled} className={cn('wk-switch', className)} data-state={checked ? 'checked' : 'unchecked'} onClick={() => onCheckedChange(!checked)} {...props}>
    <span aria-hidden="true" className="wk-switch-knob" />
  </button>;
}
