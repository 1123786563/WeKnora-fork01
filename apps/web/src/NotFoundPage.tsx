import { Status } from '@weknora/ui';

export function NotFoundPage({ path }: { path: string }) {
  return <main className="wk-page"><Status tone="error">Page not found: {path}</Status><a href="/platform/knowledge-bases">Back to knowledge bases</a></main>;
}
