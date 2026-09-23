import type { ReactNode } from 'react';
import type { Locale } from '@weknora/i18n';
import { useAppLocale } from '../i18n.ts';

/**
 * Shared settings-domain empty state mirroring the TDesign `<t-empty>` (medium)
 * anatomy both Vue baselines render (SkillSettings.vue:20, TenantMembers.vue:412):
 * a 48px "no result" illustration, a default title line and a description line,
 * all centered. Typography and colors follow tdesign.css .t-empty__{image,title,
 * description} (14px/22px; title = --td-text-color-secondary rgba(0,0,0,.6),
 * description = --td-text-color-placeholder rgba(0,0,0,.4)) with the 8px/4px
 * stack gaps from --td-comp-margin-s / -xs.
 *
 * Lives in apps/web/src/settings (frontend.md: web-specific content stays in the
 * business directory) — both consumers are settings panels.
 */

// TDesign locale `empty.titleText` (tdesign-vue-next esm/locale/*.js) — the
// title line the Vue empty state renders when no explicit title is passed.
const EMPTY_TITLES: Record<Locale, string> = {
  'zh-CN': '暂无数据',
  'en-US': 'Empty Data',
  'ja-JP': 'データなし',
  'ko-KR': '데이터 없음',
  'ru-RU': 'Нет данных',
};

/** TDesign EmptySvg (tdesign-vue-next esm/empty/components/EmptySvg.js), 48px box;
 * the Vue default type="empty" illustration (open box with rays), monochrome. */
function EmptyIllustration() {
  return (
    <svg width="48" height="48" viewBox="0 0 48 48" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
      <path d="M22 0H26V8H22V0Z" fill="currentColor" />
      <path d="M10.002 1.17157L7.17353 4L13.002 9.82843L15.8304 7L10.002 1.17157Z" fill="currentColor" />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M2 27.4689L10.8394 12H37.1606L46 27.4689V44H2V27.4689ZM13.1606 16L7.44636 26H17.8025L18.1889 27.5015C18.8551 30.0898 21.207 32 24 32C26.793 32 29.1449 30.0898 29.8111 27.5015L30.1975 26H40.5536L34.8394 16H13.1606Z"
        fill="currentColor"
      />
      <path d="M37.998 1.17157L32.1696 7L34.998 9.82843L40.8265 4L37.998 1.17157Z" fill="currentColor" />
    </svg>
  );
}

export interface EmptyStateProps {
  /** Description line under the title (the `<t-empty description>` prop). */
  description: ReactNode;
  /** Overrides the TDesign locale title (zh-CN "暂无数据"). */
  title?: ReactNode;
  /** Optional fine-print hint (Vue `.empty-hint`, SkillSettings.vue:21-23). */
  hint?: ReactNode;
  /** Centered actions row (Vue `.empty-actions`, SkillSettings.vue:24-31). */
  children?: ReactNode;
}

export function EmptyState({ description, title, hint, children }: EmptyStateProps) {
  const locale = useAppLocale();
  // SkillSettings.vue:1292-1294 gives the t-empty description a 16px bottom
  // margin whenever the hint/actions follow it; .empty-hint adds its own 16px.
  const gapBelowDescription = hint || children ? ' wk-empty-description--gap-below' : '';
  return (
    <div className="wk-empty">
      <span className="wk-empty-image" aria-hidden="true">
        <EmptyIllustration />
      </span>
      <p className="wk-empty-title">{title ?? EMPTY_TITLES[locale]}</p>
      <p className={`wk-empty-description${gapBelowDescription}`}>{description}</p>
      {hint ? <p className="wk-empty-hint">{hint}</p> : null}
      {children ? <div className="wk-empty-actions">{children}</div> : null}
    </div>
  );
}
