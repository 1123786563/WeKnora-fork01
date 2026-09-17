import { ExecutionPage as Feature } from '../../../features/execution/pages.tsx';
import { useSession } from '../../../components/ui.tsx';
/** Drop all local feature state when account or tenant changes. */
export default function Page() {
  const session = useSession();
  return <Feature key={`${session.userId}:${session.tenantId}`} />;
}
