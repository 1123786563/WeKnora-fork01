import React, { type ButtonHTMLAttributes, type HTMLAttributes, type ReactNode } from 'react';
import { cn } from './lib/utils.ts';
import './theme.css';
export { Dialog } from './dialog.tsx';
export type { DialogProps } from './dialog.tsx';
export { Button, type ButtonProps } from './button.tsx';
export { Input } from './input.tsx';
export { NumberInput } from './number-input.tsx';
export { Switch } from './switch.tsx';
export { Textarea } from './textarea.tsx';
export { Select } from './select.tsx';
export { Checkbox } from './checkbox.tsx';
export { Radio } from './radio.tsx';
export { Range } from './range.tsx';
export { Label } from './label.tsx';
export { Badge } from './badge.tsx';
export { Alert } from './alert.tsx';
export { Separator } from './separator.tsx';
export { Table, TableHead, TableBody, TableRow, TableHeader, TableCell } from './table.tsx';
export { Tabs, TabsList, TabsTrigger, TabsContent } from './tabs.tsx';
export { Sheet, type SheetProps } from './sheet.tsx';
export { Dropdown, DropdownTrigger, DropdownContent, DropdownItem, DropdownSeparator } from './dropdown-menu.tsx';
export { Menu, MenuTrigger, MenuContent, MenuItem, MenuSeparator } from './dropdown-menu.tsx';
export { TooltipProvider, Tooltip, TooltipTrigger, TooltipContent } from './tooltip.tsx';
export { cn } from './lib/utils.ts';
import './styles.css';

/** 语义卡片：白底、line 描边、card 圆角（视觉 = 既有 .wk-card）。 */
export function Card({ children, className, ...props }: HTMLAttributes<HTMLDivElement> & { children?: ReactNode }) {
  return <section className={cn('rounded-card border border-line bg-surface p-4', className)} {...props}>{children}</section>;
}

/** 内联状态文本（视觉 = 实际生效的 .wk-status：13px，muted-strong）。 */
export function Status({ tone = 'neutral', children }: { tone?: 'neutral' | 'error' | 'success' | 'warning'; children: ReactNode }) {
  return <p className={cn('my-[0.25rem] text-[13px] text-muted-strong', tone === 'error' && 'text-danger', tone === 'success' && 'text-success-text', tone === 'warning' && 'text-warning-text')} role={tone === 'error' ? 'alert' : 'status'}>{children}</p>;
}
