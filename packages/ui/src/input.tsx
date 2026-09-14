import type { InputHTMLAttributes } from 'react';
import { cn } from './lib/utils.ts';

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn('h-8 w-full rounded-[3px] border border-[#dcdcdc] bg-white px-2 text-[13px] text-black/90 outline-none transition-colors placeholder:text-[#8a8a8a] hover:border-[#07c05f] focus:border-[#07c05f] focus:ring-2 focus:ring-[#07c05f]/20 disabled:cursor-not-allowed disabled:border-[#dcdcdc] disabled:bg-[#eee] disabled:text-black/25', className)} {...props} />;
}
