import { MobileTasks } from '../composition.ts';

/** /tasks 一级入口；Surface 仍由 Runtime 快照裁决。 */
export default function Tasks() {
  return <MobileTasks />;
}
