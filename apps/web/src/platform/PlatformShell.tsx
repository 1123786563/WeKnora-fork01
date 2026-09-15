import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import {
  activePlatformNavId,
  filterCommandItems,
  nextCommandIndex,
  platformNavItems,
  platformUserMenuItems,
  tenantDisplayName,
  tenantInitial,
  userInitial,
  type CommandItem,
  type PlatformNavItem,
  type TenantMembership,
} from './shell-model.ts';
import './platform-shell.css';

export interface PlatformShellProps {
  pathname: string;
  user: { name: string; email?: string; avatarUrl?: string };
  memberships?: readonly TenantMembership[];
  activeTenantId?: string | null;
  collapsed?: boolean;
  onCollapsedChange?: (collapsed: boolean) => void;
  onNavigate: (path: string) => void;
  onTenantChange?: (tenant: TenantMembership) => void;
  onLogout?: () => void;
  children: ReactNode;
  commands?: readonly CommandItem[];
}

export function PlatformShell(props: PlatformShellProps) {
  const { pathname, user, memberships = [], activeTenantId, onNavigate, children } = props;
  const shellRef = useRef<HTMLDivElement>(null);
  const paletteTriggerRef = useRef<HTMLButtonElement>(null);
  const [collapsed, setCollapsed] = useState(props.collapsed ?? false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteQuery, setPaletteQuery] = useState('');
  const [selectedCommand, setSelectedCommand] = useState(0);
  const [tenantOpen, setTenantOpen] = useState(false);
  const [userOpen, setUserOpen] = useState(false);
  const activeId = activePlatformNavId(pathname);
  const commands = useMemo(() => props.commands ?? platformNavItems.map((item) => ({
    id: item.id, label: item.label, hint: item.path, keywords: [item.id], run: () => onNavigate(item.path),
  })), [onNavigate, props.commands]);
  const filteredCommands = useMemo(() => filterCommandItems(commands, paletteQuery), [commands, paletteQuery]);
  const activeTenant = memberships.find((tenant) => tenant.tenantId === activeTenantId) ?? memberships[0];
  const canManageMembers = activeTenant?.role === 'owner' || activeTenant?.role === 'admin';
  const userMenuItems = useMemo(() => platformUserMenuItems({
    hasTenant: Boolean(activeTenant),
    canManageMembers,
    canLogout: Boolean(props.onLogout),
  }), [activeTenant, canManageMembers, props.onLogout]);

  useEffect(() => { props.onCollapsedChange?.(collapsed); }, [collapsed, props.onCollapsedChange]);
  useEffect(() => {
    if (props.collapsed !== undefined) setCollapsed(props.collapsed);
  }, [props.collapsed]);
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault(); setPaletteOpen(true); setPaletteQuery(''); setSelectedCommand(0);
      } else if (event.key === 'Escape') { setPaletteOpen(false); setTenantOpen(false); setUserOpen(false); }
    };
    document.addEventListener('keydown', onKeyDown);
    const onPointerDown = (event: PointerEvent) => {
      if (!shellRef.current?.contains(event.target as Node)) {
        setTenantOpen(false);
        setUserOpen(false);
      }
    };
    document.addEventListener('pointerdown', onPointerDown);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('pointerdown', onPointerDown);
    };
  }, []);

  const toggleCollapsed = () => setCollapsed((value) => !value);
  const openPalette = () => { setPaletteOpen(true); setPaletteQuery(''); setSelectedCommand(0); };
  const runSelected = () => { filteredCommands[selectedCommand]?.run(); setPaletteOpen(false); paletteTriggerRef.current?.focus(); };
  const navigateFromMenu = (path: string) => {
    setUserOpen(false);
    setTenantOpen(false);
    onNavigate(path);
  };

  return <div ref={shellRef} className={`platform-shell${collapsed ? ' is-collapsed' : ''}`}>
    <aside className="platform-sidebar" aria-label="Platform navigation">
      {!collapsed && <div className="platform-logo-row"><button className="platform-logo" onClick={() => navigateFromMenu('/platform/knowledge-bases')}>WeKnora</button><button ref={paletteTriggerRef} aria-label="Open command palette" aria-keyshortcuts="Meta+K Control+K" onClick={openPalette}>⌕</button><button aria-label="Collapse sidebar" onClick={toggleCollapsed}>‹</button></div>}
      {collapsed && <><button className="platform-expand" aria-label="Expand sidebar" onClick={toggleCollapsed}>›</button><button ref={paletteTriggerRef} className="platform-collapsed-search" aria-label="Open command palette" onClick={openPalette}>⌕</button></>}
      {memberships.length > 0 && !collapsed && <div className="platform-tenant-wrap"><button className="platform-tenant-trigger" aria-haspopup="listbox" aria-expanded={tenantOpen} onClick={() => { setTenantOpen((value) => !value); setUserOpen(false); }}><span>{activeTenant ? tenantDisplayName(activeTenant) : 'Select workspace'}</span><span aria-hidden="true">⌄</span></button>{tenantOpen && <div className="platform-tenant-menu" role="listbox" aria-label="Workspaces">{memberships.map((tenant) => <button key={tenant.tenantId} role="option" aria-selected={tenant.tenantId === activeTenantId} onClick={() => { props.onTenantChange?.(tenant); setTenantOpen(false); }}><span className="platform-tenant-avatar">{tenantInitial(tenant)}</span><span><strong>{tenantDisplayName(tenant)}</strong><small>{tenant.role ?? ''}{tenant.isHome ? ' · Home' : ''}</small></span></button>)}</div>}</div>}
      <nav className="platform-nav" aria-label="Primary"><div className="platform-nav-items">{platformNavItems.map((item) => <NavButton key={item.id} item={item} active={item.id === activeId} collapsed={collapsed} onNavigate={navigateFromMenu} />)}</div></nav>
      <div className="platform-user-wrap"><button className="platform-user-trigger" aria-haspopup="menu" aria-expanded={userOpen} onClick={() => { setUserOpen((value) => !value); setTenantOpen(false); }}><span className="platform-avatar">{user.avatarUrl ? <img src={user.avatarUrl} alt="" /> : userInitial(user.name)}</span>{!collapsed && <span className="platform-user-copy"><strong>{user.name}</strong><small>{user.email ?? ''}</small></span>}</button>{userOpen && <div className="platform-user-menu" role="menu" aria-label="Account menu">{userMenuItems.map((item) => item.path ? <button key={item.id} role="menuitem" className={item.danger ? 'is-danger' : undefined} onClick={() => navigateFromMenu(item.path!)}>{item.label}</button> : <button key={item.id} role="menuitem" className={item.danger ? 'is-danger' : undefined} onClick={() => { setUserOpen(false); props.onLogout?.(); }}>{item.label}</button>)}</div>}</div>
    </aside>
    <main className="platform-content">{children}</main>
    {paletteOpen && <div className="platform-palette-overlay" onMouseDown={() => { setPaletteOpen(false); paletteTriggerRef.current?.focus(); }}><section className="platform-palette" role="dialog" aria-modal="true" aria-label="Command palette" onMouseDown={(event) => event.stopPropagation()}><input aria-label="Search commands" autoFocus value={paletteQuery} placeholder="Search commands" onChange={(event) => { setPaletteQuery(event.target.value); setSelectedCommand(0); }} onKeyDown={(event) => { if (event.key === 'ArrowDown') { event.preventDefault(); setSelectedCommand((index) => nextCommandIndex(index, filteredCommands.length, 1)); } else if (event.key === 'ArrowUp') { event.preventDefault(); setSelectedCommand((index) => nextCommandIndex(index, filteredCommands.length, -1)); } else if (event.key === 'Enter') { event.preventDefault(); runSelected(); } }} />{filteredCommands.length > 0 ? <div id="platform-command-results" role="listbox" aria-label="Commands">{filteredCommands.map((command, index) => <button key={command.id} role="option" aria-selected={index === selectedCommand} onClick={() => { command.run(); setPaletteOpen(false); paletteTriggerRef.current?.focus(); }}><span>{command.label}</span><small>{command.hint}</small></button>)}</div> : <p role="status">No commands found</p>}</section></div>}
  </div>;
}

function NavButton({ item, active, collapsed, onNavigate }: { item: PlatformNavItem; active: boolean; collapsed: boolean; onNavigate: (path: string) => void }) {
  return <button className={`platform-nav-item${active ? ' is-active' : ''}`} aria-current={active ? 'page' : undefined} title={collapsed ? item.label : undefined} onClick={() => onNavigate(item.path)}><span aria-hidden="true">{item.icon.slice(0, 1).toUpperCase()}</span>{!collapsed && <span>{item.label}</span>}</button>;
}
