import { Status } from '@weknora/ui';
import { useAppLocale } from './i18n.ts';

const copy = {
  'zh-CN': { title: '页面不存在', back: '返回知识库' },
  'en-US': { title: 'Page not found', back: 'Back to knowledge bases' },
} as const;

export function NotFoundPage({ path }: { path: string }) {
  const locale = useAppLocale();
  const t = copy[(locale in copy ? locale : 'en-US') as keyof typeof copy];
  // 挂载在 .plat-shell__outlet 内：shell.td.css 把 .wk-page 覆盖为全宽（max-width:none、
  // height:100%、overflow-y:auto），因此这里的规则取实际生效值而非 960px 版本（.wk-notfound）。
  return <main className="wk-notfound"><Status tone="error">{t.title}: {path}</Status><a href="/platform/knowledge-bases">{t.back}</a></main>;
}
