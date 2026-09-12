import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { formatMessage, isLocale, type Locale } from '@weknora/i18n';
import type { createWeKnoraClient } from '@weknora/api-client';
import './shell.css';

type Client = ReturnType<typeof createWeKnoraClient>;

export interface PlatformShellProps {
  client: Client;
  onLogout: () => void | Promise<void>;
  children: ReactNode;
}

// Locale resolution mirrors App.tsx / SettingsPage: navigator.language with a
// package-supported fallback.
function resolveLocale(): Locale {
  const language = typeof navigator !== 'undefined' ? navigator.language : 'en-US';
  return isLocale(language) ? language : isLocale(language.split('-')[0] ?? '') ? (language.split('-')[0] as Locale) : 'en-US';
}

// Nav labels migrated to packages/i18n menu.* (auto-ported from Vue locales).

interface NavItem {
  key: string;
  href: string;
  label: string;
  icon: ReactNode;
  match: (pathname: string) => boolean;
}

const KB_ACTIVE = (pathname: string): boolean =>
  pathname === '/platform/knowledge-bases' ||
  /^\/platform\/knowledge-bases\/[^/]+/.test(pathname) ||
  /^\/knowledgeBase(\/|$)/.test(pathname);

function Icon({ path }: { path: string }): ReactNode {
  return (
    <svg className="plat-shell__icon" viewBox="0 0 24 24" width="18" height="18" fill="none"
      stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d={path} />
    </svg>
  );
}

const ICONS = {
  chat: 'M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z',
  book: 'M4 19.5A2.5 2.5 0 0 1 6.5 17H20M4 19.5A2.5 2.5 0 0 0 6.5 22H20V2H6.5A2.5 2.5 0 0 0 4 4.5v15z',
  bot: 'M12 8V4m0 0a1.5 1.5 0 1 0 0-3 1.5 1.5 0 0 0 0 3zM4 7h16a1 1 0 0 1 1 1v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a1 1 0 0 1 1-1zm5 5h.01M15 12h.01M9 17h6',
  users: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2m22-2v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0z',
};

function buildNavItems(t: (key: string) => string, labels: Record<string, string>): NavItem[] {
  return [
    { key: 'newChat', href: '/platform/creatChat', label: labels.newChat, icon: <Icon path={ICONS.chat} />, match: (p: string) => p === '/platform/creatChat' || p.startsWith('/platform/chat/') },
    { key: 'knowledgeBases', href: '/platform/knowledge-bases', label: t('common.knowledgeBases'), icon: <Icon path={ICONS.book} />, match: KB_ACTIVE },
    { key: 'agents', href: '/platform/agents', label: labels.agents, icon: <Icon path={ICONS.bot} />, match: (p: string) => p === '/platform/agents' || p.startsWith('/platform/agents/') || p === '/platform/configuration' },
    { key: 'organizations', href: '/platform/organizations', label: labels.organizations, icon: <Icon path={ICONS.users} />, match: (p: string) => p.startsWith('/platform/organizations') },
  ];
}

const COLLAPSE_STORAGE_KEY = 'weknora_sidebar_collapsed';

export function PlatformShell({ client, onLogout, children }: PlatformShellProps): ReactNode {
  const locale = useMemo(resolveLocale, []);
  const t = useCallback((key: string) => formatMessage(locale, key), [locale]);
  const labels = {
    newChat: formatMessage(locale, 'menu.newChat'),
    agents: formatMessage(locale, 'menu.agents'),
    organizations: formatMessage(locale, 'menu.organizations'),
    personalSettings: formatMessage(locale, 'general.personalSettings'),
  };

  const [pathname, setPathname] = useState(() => window.location.pathname);
  const [collapsed, setCollapsed] = useState(() => window.localStorage.getItem(COLLAPSE_STORAGE_KEY) === 'true');
  const [menuOpen, setMenuOpen] = useState(false);
  const [user, setUser] = useState<{ name: string; email: string; avatar: string }>({ name: '', email: '', avatar: '' });

  useEffect(() => {
    // Route transitions in this app mostly use full navigations and popstate
    // reloads the page. Track pushState/replaceState so in-place navigations
    // (e.g. settings section links) still update the active highlight.
    const update = () => setPathname(window.location.pathname);
    window.addEventListener('popstate', update);
    const originalPush = window.history.pushState.bind(window.history);
    const originalReplace = window.history.replaceState.bind(window.history);
    window.history.pushState = (...args: Parameters<typeof originalPush>) => { const r = originalPush(...args); update(); return r; };
    window.history.replaceState = (...args: Parameters<typeof originalReplace>) => { const r = originalReplace(...args); update(); return r; };
    return () => {
      window.removeEventListener('popstate', update);
      window.history.pushState = originalPush;
      window.history.replaceState = originalReplace;
    };
  }, []);

  useEffect(() => {
    let active = true;
    void client.auth.me().then((me) => {
      if (!active) return;
      const record = me.user as Record<string, unknown>;
      setUser({
        name: typeof record.username === 'string' && record.username ? record.username : '—',
        email: typeof record.email === 'string' ? record.email : '',
        avatar: typeof record.avatar === 'string' ? record.avatar : '',
      });
    }).catch(() => { /* menu falls back to placeholders; the page still works */ });
    return () => { active = false; };
  }, [client]);

  const navItems = useMemo(() => buildNavItems(t, labels), [t, labels]);

  // KB-list quick filters: the React KB list page reads ?scope=all|mine from
  // the URL. 收藏/最近 from the Vue rail have no backing in the React page
  // this round, so only the two supported scopes are rendered.
  const kbListContext = pathname === '/platform/knowledge-bases' || pathname === '/platform';
  const currentScope = new URLSearchParams(window.location.search).get('scope') === 'mine' ? 'mine' : 'all';

  const toggleCollapsed = () => {
    setCollapsed((current) => {
      window.localStorage.setItem(COLLAPSE_STORAGE_KEY, String(!current));
      return !current;
    });
  };

  const initial = (user.name || '?').charAt(0).toUpperCase();

  return (
    <div className="plat-shell">
      <aside className={`plat-shell__aside${collapsed ? ' plat-shell__aside--collapsed' : ''}`}>
        <div className="plat-shell__logo-row">
          <a className="plat-shell__logo" href="/platform/knowledge-bases" aria-label="WeKnora">
            <span className="plat-shell__logo-mark" aria-hidden="true">W</span>
            {!collapsed && <span className="plat-shell__logo-text">WeKnora</span>}
          </a>
          {!collapsed && (
            <button type="button" className="plat-shell__toggle" onClick={toggleCollapsed} aria-label="Collapse sidebar" title="Collapse sidebar">
              <svg viewBox="0 0 20 20" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
                <rect x="1.5" y="1.5" width="17" height="17" rx="3" />
                <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" />
                <line x1="5" y1="10" x2="3" y2="8" strokeLinecap="round" />
                <line x1="5" y1="10" x2="3" y2="12" strokeLinecap="round" />
              </svg>
            </button>
          )}
        </div>
        {collapsed && (
          <button type="button" className="plat-shell__item plat-shell__toggle-item" onClick={toggleCollapsed} aria-label="Expand sidebar">
            <svg viewBox="0 0 20 20" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.2" aria-hidden="true">
              <rect x="1.5" y="1.5" width="17" height="17" rx="3" />
              <line x1="7.5" y1="1.5" x2="7.5" y2="18.5" />
              <line x1="4" y1="7.5" x2="4" y2="12.5" strokeLinecap="round" />
            </svg>
          </button>
        )}

        <div className="plat-shell__top">
          <nav className="plat-shell__nav" aria-label="Platform">
            {navItems.map((item) => {
              const active = item.match(pathname);
              return (
                <a key={item.key} href={item.href} className={`plat-shell__item${active ? ' plat-shell__item--active' : ''}`}
                  aria-current={active ? 'page' : undefined} title={collapsed ? item.label : undefined}>
                  <span className="plat-shell__item-icon">{item.icon}</span>
                  {!collapsed && <span className="plat-shell__item-label">{item.label}</span>}
                </a>
              );
            })}
          </nav>

          {kbListContext && !collapsed && (
            <div className="plat-shell__kb-filters" role="navigation" aria-label={t('common.knowledgeBases')}>
              <a href="/platform/knowledge-bases" className={`plat-shell__kb-filter${currentScope === 'all' ? ' plat-shell__kb-filter--active' : ''}`}>{t('common.all')}</a>
              <a href="/platform/knowledge-bases?scope=mine" className={`plat-shell__kb-filter${currentScope === 'mine' ? ' plat-shell__kb-filter--active' : ''}`}>{t('knowledgeList.sections.mine')}</a>
            </div>
          )}
        </div>

        <div className="plat-shell__bottom">
          <div className={`plat-shell__user${menuOpen ? ' plat-shell__user--open' : ''}`}>
            <button type="button" className="plat-shell__user-button" aria-haspopup="menu" aria-expanded={menuOpen}
              onClick={() => setMenuOpen((open) => !open)}>
              <span className="plat-shell__avatar" aria-hidden="true">
                {user.avatar ? <img src={user.avatar} alt="" /> : <span className="plat-shell__avatar-initial">{initial}</span>}
              </span>
              {!collapsed && (
                <span className="plat-shell__user-info">
                  <span className="plat-shell__user-name">{user.name || '—'}</span>
                  <span className="plat-shell__user-email">{user.email}</span>
                </span>
              )}
            </button>
            {menuOpen && (
              <div className="plat-shell__dropdown" role="menu">
                <a role="menuitem" className="plat-shell__dropdown-item"
                  href="/platform/settings?section=userprofile"
                  onClick={() => setMenuOpen(false)}>
                  {labels.personalSettings}
                </a>
                <div className="plat-shell__dropdown-divider" aria-hidden="true" />
                <button type="button" role="menuitem" className="plat-shell__dropdown-item plat-shell__dropdown-item--danger"
                  onClick={() => { setMenuOpen(false); void onLogout(); }}>
                  {t('auth.logout')}
                </button>
              </div>
            )}
          </div>
        </div>
      </aside>
      <div className="plat-shell__outlet">{children}</div>
    </div>
  );
}