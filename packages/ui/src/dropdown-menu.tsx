import type { ComponentProps, ReactNode } from 'react';
import * as DropdownPrimitive from '@radix-ui/react-dropdown-menu';
import { cn } from './lib/utils.ts';

export const Dropdown = DropdownPrimitive.Root;
export const DropdownTrigger = DropdownPrimitive.Trigger;

/** 弹出菜单面板（令牌化：白底、line 描边、浅投影）。 */
export function DropdownContent({ className, children, ...props }: { children: ReactNode } & ComponentProps<typeof DropdownPrimitive.Content>) {
  return (
    <DropdownPrimitive.Portal>
      <DropdownPrimitive.Content
        sideOffset={4}
        className={cn(
          'z-[1200] min-w-[10rem] rounded-card border border-line bg-surface p-1 shadow-[0_8px_24px_rgba(15,23,42,0.12)]',
          className,
        )}
        {...props}
      >
        {children}
      </DropdownPrimitive.Content>
    </DropdownPrimitive.Portal>
  );
}

export function DropdownItem({ className, children, ...props }: { children: ReactNode } & ComponentProps<typeof DropdownPrimitive.Item>) {
  return (
    <DropdownPrimitive.Item
      className={cn(
        'flex cursor-pointer select-none items-center gap-2 rounded-control px-2.5 py-1.5 text-[13px] text-ink outline-none',
        'data-[highlighted]:bg-hover-wash data-[disabled]:cursor-not-allowed data-[disabled]:opacity-50 data-[disabled]:pointer-events-none',
        className,
      )}
      {...props}
    >
      {children}
    </DropdownPrimitive.Item>
  );
}

export function DropdownSeparator({ className, ...props }: ComponentProps<typeof DropdownPrimitive.Separator>) {
  return <DropdownPrimitive.Separator className={cn('-mx-1 my-1 h-px bg-line-soft', className)} {...props} />;
}
