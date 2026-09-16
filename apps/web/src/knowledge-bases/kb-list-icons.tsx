import type { ReactNode } from 'react';

// Inline stroke icons for the kb-list page (R009 anatomy slice). The Vue page
// uses TDesign icons (KnowledgeBaseList.vue / ListSpaceSidebar.vue); these are
// lightweight 24x24 stroke equivalents drawn in the same style as the shell
// nav icons (PlatformShell.tsx Icon). Icons are decorative: visible text
// labels or aria-labels always carry the meaning.
export type KbIconName =
  | 'layers'
  | 'star'
  | 'star-filled'
  | 'history'
  | 'workspace'
  | 'folder'
  | 'folder-add'
  | 'chat'
  | 'info-circle'
  | 'dots'
  | 'chevron-down'
  | 'chevron-right'
  | 'pin'
  | 'pin-filled'
  | 'user'
  | 'usergroup'
  | 'edit'
  | 'browse'
  | 'share'
  | 'copy'
  | 'trash'
  | 'settings'
  | 'close'
  | 'image'
  | 'relation'
  | 'help-circle'
  | 'upload'
  | 'check-circle-filled'
  | 'usergroup-add'
  | 'wiki'
  | 'chevron-left';

const ICONS: Record<KbIconName, ReactNode> = {
  layers: (
    <>
      <path d="M12 2 2 7l10 5 10-5-10-5z" />
      <path d="m2 12 10 5 10-5" />
      <path d="m2 17 10 5 10-5" />
    </>
  ),
  star: <path d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8L3.5 9.7l5.9-.9L12 3.5z" />,
  "star-filled": (
    <path
      d="M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8L3.5 9.7l5.9-.9L12 3.5z"
      fill="currentColor"
    />
  ),
  history: (
    <>
      <path d="M3.05 11a9 9 0 1 0 .5-4.3" />
      <path d="M3 3v5h5" />
      <path d="M12 7v5l4 2" />
    </>
  ),
  workspace: (
    <>
      <path d="M3 3h7v7H3z" />
      <path d="M14 3h7v7h-7z" />
      <path d="M14 14h7v7h-7z" />
      <path d="M3 14h7v7H3z" />
    </>
  ),
  folder: <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />,
  "folder-add": (
    <>
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z" />
      <path d="M12 10.5v5M9.5 13h5" />
    </>
  ),
  chat: <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />,
  "info-circle": (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 8h.01M12 11.5V16" />
    </>
  ),
  dots: (
    <g fill="currentColor" stroke="none">
      <circle cx="5.5" cy="12" r="1.7" />
      <circle cx="12" cy="12" r="1.7" />
      <circle cx="18.5" cy="12" r="1.7" />
    </g>
  ),
  "chevron-down": <path d="m6 9 6 6 6-6" />,
  "chevron-right": <path d="m9 6 6 6-6 6" />,
  pin: (
    <>
      <path d="M9 3.5h6l-.8 6.2 3.3 2.8v1.5H6.5V12.5l3.3-2.8L9 3.5z" />
      <path d="M12 14v6.5" />
    </>
  ),
  "pin-filled": (
    <>
      <path d="M9 3.5h6l-.8 6.2 3.3 2.8v1.5H6.5V12.5l3.3-2.8L9 3.5z" fill="currentColor" />
      <path d="M12 14v6.5" />
    </>
  ),
  user: (
    <>
      <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
      <circle cx="12" cy="7" r="4" />
    </>
  ),
  usergroup: (
    <>
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
    </>
  ),
  edit: (
    <>
      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
      <path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
    </>
  ),
  browse: (
    <>
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
      <circle cx="12" cy="12" r="3" />
    </>
  ),
  share: (
    <>
      <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
      <path d="m16 6-4-4-4 4" />
      <path d="M12 2v13" />
    </>
  ),
  copy: (
    <>
      <rect x="9" y="9" width="12" height="12" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </>
  ),
  trash: (
    <>
      <path d="M3 6h18" />
      <path d="M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
      <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6" />
      <path d="M10 11v6M14 11v6" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </>
  ),
  close: <path d="m5 5 14 14M19 5 5 19" />,
  image: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="2" />
      <circle cx="8.5" cy="8.5" r="1.5" />
      <path d="m21 15-5-5L5 21" />
    </>
  ),
  relation: (
    <>
      <circle cx="5" cy="6" r="2.5" />
      <circle cx="19" cy="6" r="2.5" />
      <circle cx="12" cy="18" r="2.5" />
      <path d="M6.5 8 10.5 16M17.5 8l-4 8M7.5 6h9" />
    </>
  ),
  "help-circle": (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M9.1 9a3 3 0 0 1 5.8 1c0 2-3 2.4-3 4" />
      <path d="M12 17.5h.01" />
    </>
  ),
  upload: (
    <>
      <path d="M12 15V4" />
      <path d="m7 8 5-5 5 5" />
      <path d="M4 20h16" />
    </>
  ),
  "check-circle-filled": (
    <>
      <path
        d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20z"
        fill="currentColor"
        stroke="none"
      />
      <path d="m7.5 12.3 3.1 3.1 6-6.2" />
    </>
  ),
  "usergroup-add": (
    <>
      <path d="M14 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="7.5" cy="7" r="4" />
      <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
      <path d="M18 10V4M15 7h6" />
    </>
  ),
  wiki: (
    <>
      <path d="M12 7v14" />
      <path d="M16 12h2" />
      <path d="M16 8h2" />
      <path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z" />
      <path d="M6 12h2" />
      <path d="M6 8h2" />
    </>
  ),
  "chevron-left": <path d="m15 6-6 6 6 6" />,
};

export function KbIcon({ name, size = 16 }: { name: KbIconName; size?: number }): ReactNode {
  return (
    <svg data-kb-icon={name} viewBox="0 0 24 24" width={size} height={size} fill="none" stroke="currentColor"
      strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {ICONS[name]}
    </svg>
  );
}
