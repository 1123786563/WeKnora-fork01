import { Status } from '@weknora/ui';
import { useAppLocale } from './i18n.ts';

const copy = {
  'zh-CN': { title: '页面不存在', back: '返回知识库' },
  'en-US': { title: 'Page not found', back: 'Back to knowledge bases' },
} as const;

export function NotFoundPage({ path }: { path: string }) {
  const locale = useAppLocale();
  const t = copy[(locale in copy ? locale : 'en-US') as keyof typeof copy];
  return <main className="mx-auto box-border max-w-[960px] px-[1.25rem] py-12"><Status tone="error">{t.title}: {path}</Status><a href="/platform/knowledge-bases">{t.back}</a></main>;
}
