import React, { forwardRef, type HTMLAttributes, type ReactNode } from 'react';
import * as TabsPrimitive from '@radix-ui/react-tabs';
import { cn } from './lib/utils.ts';

/** shadcn/ui Tabs（Radix）：方向键漫游、aria 接线开箱即用。 */
export const Tabs = TabsPrimitive.Root;

export const TabsList = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <TabsPrimitive.List ref={ref} className={cn('flex items-center gap-1 border-b border-line-soft', className)} {...props} />
  ),
);
TabsList.displayName = 'TabsList';

export const TabsTrigger = forwardRef<HTMLButtonElement, { value: string; children: ReactNode } & HTMLAttributes<HTMLButtonElement>>(
  ({ className, children, ...props }, ref) => (
    <TabsPrimitive.Trigger
      ref={ref}
      className={cn(
        'min-h-8 cursor-pointer rounded-t-control border border-b-2 border-transparent px-3 py-1.5 text-[13px] text-muted-strong transition-colors hover:text-ink',
        'focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-accent/35 disabled:cursor-not-allowed disabled:opacity-60',
        'data-[state=active]:border-accent data-[state=active]:bg-transparent data-[state=active]:font-medium data-[state=active]:text-ink',
        className,
      )}
      {...props}
    >
      {children}
    </TabsPrimitive.Trigger>
  ),
);
TabsTrigger.displayName = 'TabsTrigger';

export const TabsContent = forwardRef<HTMLDivElement, { value: string; children: ReactNode } & HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => (
    <TabsPrimitive.Content ref={ref} className={cn('pt-3 focus-visible:outline-none', className)} {...props}>
      {children}
    </TabsPrimitive.Content>
  ),
);
TabsContent.displayName = 'TabsContent';
