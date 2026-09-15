import React, { type ComponentProps, type ReactNode } from 'react';
import * as DropdownPrimitive from '@radix-ui/react-dropdown-menu';
import { cn } from './lib/utils.ts';

export const Dropdown = DropdownPrimitive.Root;
export const DropdownTrigger = DropdownPrimitive.Trigger;

// Menu names mirror shadcn/Radix vocabulary while retaining the existing
// Dropdown exports used throughout the migrated pages.
export const Menu = DropdownPrimitive.Root;
export const MenuTrigger = DropdownPrimitive.Trigger;

/** 弹出菜单面板（令牌化：白底、line 描边、浅投影）。 */
export function DropdownContent({ className, children, ...props }: { children: ReactNode } & ComponentProps<typeof DropdownPrimitive.Content>) {
  return (
    <DropdownPrimitive.Portal>
      <DropdownPrimitive.Content
        sideOffset={4}
        className={cn(
          'z-[1200] min-w-[148px] max-w-[min(100vw-16px,320px)] rounded-control border border-line bg-surface p-1 shadow-[0_2px_4px_rgba(0,0,0,0.04),0_8px_24px_rgba(0,0,0,0.1)] backdrop-blur-xl',
          className,
        )}
        {...props}
      >
        {children}
      </DropdownPrimitive.Content>
    </DropdownPrimitive.Portal>
  );
}

export function MenuContent(props: { children: ReactNode } & ComponentProps<typeof DropdownPrimitive.Content>) {
  return <DropdownContent {...props} />;
}

export function DropdownItem({ className, children, ...props }: { children: ReactNode } & ComponentProps<typeof DropdownPrimitive.Item>) {
  return (
    <DropdownPrimitive.Item
      className={cn(
        'flex cursor-pointer select-none items-center gap-2 rounded-control px-3 py-2 text-sm leading-5 text-ink outline-none transition-[background-color,transform,color] duration-150',
        'data-[highlighted]:bg-hover-wash data-[state=open]:bg-hover-wash data-[disabled]:cursor-not-allowed data-[disabled]:pointer-events-none data-[disabled]:opacity-50 active:scale-[0.98]',
        className,
      )}
      {...props}
    >
      {children}
    </DropdownPrimitive.Item>
  );
}

export function MenuItem(props: { children: ReactNode } & ComponentProps<typeof DropdownPrimitive.Item>) {
  return <DropdownItem {...props} />;
}

export function DropdownSeparator({ className, ...props }: ComponentProps<typeof DropdownPrimitive.Separator>) {
  return <DropdownPrimitive.Separator className={cn('-mx-1 my-1 h-px bg-line-soft', className)} {...props} />;
}

export const MenuSeparator = DropdownSeparator;
