import type { ButtonHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

type SwitchProps = Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'onChange'> & {
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
};

/** 语义开关（视觉 = 既有绿色 40x20 轨道 + 16px 旋钮，focus ring accent/20）。 */
export function Switch({ checked, onCheckedChange, className, disabled, ...props }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      data-state={checked ? 'checked' : 'unchecked'}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        'relative inline-flex h-5 w-10 shrink-0 cursor-pointer items-center rounded-pill border-0 p-0 transition-colors duration-200',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent/20',
        checked ? 'bg-accent' : 'bg-track-off',
        disabled && 'cursor-not-allowed',
        checked && disabled && 'bg-track-on-disabled',
        className,
      )}
      {...props}
    >
      <span aria-hidden="true" className={cn('pointer-events-none ml-0.5 block h-4 w-4 rounded-pill bg-surface transition-transform duration-200', checked && 'translate-x-5')} />
    </button>
  );
}
