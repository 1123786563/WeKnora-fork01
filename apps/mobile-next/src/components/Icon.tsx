import React from "react";
import Svg, { Path } from "react-native-svg";
import { useTheme } from "@/theme/ThemeProvider";

// 图标 path 数据提取自设计包 weknora-expo-hifi-v2 原型（ICONS 常量），stroke 风格 24x24。
export const ICON_PATHS = {
  spark: "M12 3l2.3 6.7L21 12l-6.7 2.3L12 21l-2.3-6.7L3 12l6.7-2.3L12 3 M19 3v4 M17 5h4",
  home: "M3 10l9-7 9 7 M5 9v11h5v-6h4v6h5V9",
  chat: "M20 4H4v13h4v4l5-4h7V4 M8 8h8 M8 12h5",
  grid: "M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z",
  user: "M16 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0 M4 21v-2a8 8 0 0 1 16 0v2",
  bell: "M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9 M10 21h4",
  plus: "M12 5v14 M5 12h14",
  arrow: "M4 12h16 M14 6l6 6-6 6",
  back: "M15 5l-7 7 7 7",
  chevron: "M9 5l7 7-7 7",
  down: "M6 9l6 6 6-6",
  check: "M5 12l4 4L19 6",
  shield: "M12 3l8 3v6c0 4-8 9-8 9s-8-5-8-9V6l8-3 M8 12l3 3 5-6",
  search: "M16 10a6 6 0 1 1-12 0 6 6 0 0 1 12 0 M14.5 14.5L21 21",
  book: "M12 5C8 2 4 3 3 4v15c3-2 6-1 9 1 3-2 6-3 9-1V4c-1-1-5-2-9 1v15",
  file: "M14 3H5v18h14V8l-5-5v5h5 M8 12h8 M8 16h5",
  link: "M10 14l4-4 M8 16l-1 1a4 4 0 0 1-6-6l5-5a4 4 0 0 1 6 0 M16 8l1-1a4 4 0 0 1 6 6l-5 5a4 4 0 0 1-6 0",
  monitor: "M3 4h18v13H3z M9 21h6 M12 17v4",
  mic: "M9 6a3 3 0 0 1 6 0v6a3 3 0 0 1-6 0V6 M5 10v2a7 7 0 0 0 14 0v-2 M12 19v3 M9 22h6",
  activity: "M2 12h4l3-8 6 16 3-8h4",
  layers: "M12 3L2 8l10 5 10-5-10-5 M2 12l10 5 10-5 M2 16l10 5 10-5",
  pie: "M11 3a9 9 0 1 0 10 10H11V3 M15 3v6h6a8 8 0 0 0-6-6",
  moon: "M20 14a8 8 0 0 1-10-10 9 9 0 1 0 10 10",
  sun: "M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1 1 M18 18l1 1 M5 19l1-1 M18 6l1-1",
  more: "M5 12h.1 M12 12h.1 M19 12h.1",
  paperclip: "M9 16l7-7a3 3 0 0 0-4-4L4 13a5 5 0 0 0 7 7l9-9",
  send: "M3 3l18 9-18 9 4-9-4-9 M7 12h14",
  lock: "M6 10h12v11H6z M8 10V7a4 4 0 0 1 8 0v3",
  clock: "M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M12 7v5l3 2",
  stop: "M6 6h12v12H6z",
  refresh: "M20 8a8 8 0 1 0 0 8 M20 3v5h-5",
  info: "M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M12 11v6 M12 7v.1",
  alert: "M12 3L2 21h20L12 3 M12 10v5 M12 18v.1",
  wifi: "M3 8a14 14 0 0 1 18 0 M6 12a9 9 0 0 1 12 0 M9 16a4 4 0 0 1 6 0 M12 20v.1",
  download: "M12 3v12 M7 10l5 5 5-5 M4 16v5h16v-5",
  share: "M12 15V2 M7 7l5-5 5 5 M5 10v11h14V10",
  close: "M6 6l12 12 M18 6L6 18",
  code: "M8 6l-6 6 6 6 M16 6l6 6-6 6 M14 3l-4 18",
  folder: "M3 5h7l2 3h9v12H3z",
  star: "M12 3l3 6 7 1-5 5 1 7-6-3-6 3 1-7-5-5 7-1 3-6",
  logout: "M10 3H3v18h7 M14 7l5 5-5 5 M7 12h12",
  settings: "M12 8a4 4 0 1 1 0 8 4 4 0 0 1 0-8 M12 2v3 M12 19v3 M2 12h3 M19 12h3 M5 5l2 2 M17 17l2 2 M5 19l2-2 M17 7l2-2",
  globe: "M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M3 12h18 M12 3c-5 4-5 14 0 18 5-4 5-14 0-18",
  checkcircle: "M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0 M7 12l3 3 7-7",
  copy: "M8 8h13v13H8z M16 8V3H3v13h5",
  mail: "M3 5h18v14H3z M3 5l9 8 9-8",
  filter: "M3 5h18 M7 12h10 M10 19h4",
} as const;

export type IconName = keyof typeof ICON_PATHS;

export function Icon({
  name,
  size,
  color,
  strokeWidth = 1.8,
  testID,
}: {
  name: IconName;
  size?: number;
  color?: string;
  strokeWidth?: number;
  testID?: string;
}) {
  const { theme } = useTheme();
  const s = size ?? theme.size.icon;
  return (
    <Svg width={s} height={s} viewBox="0 0 24 24" fill="none" testID={testID}>
      <Path
        d={ICON_PATHS[name]}
        stroke={color ?? theme.c.ink}
        strokeWidth={strokeWidth}
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </Svg>
  );
}
