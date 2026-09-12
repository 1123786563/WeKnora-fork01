// W04 craft workbench state machine.
//
// One tiny pure reducer owns everything the workbench projects from run
// events. Three invariants keep refreshes and replays harmless:
//   1. generation — events from a previous scope generation are dropped, so
//      switching spaces can never leak one workspace's stream into another;
//   2. runId — only the active main run's events apply, so a replayed older
//      run never mutates the current message;
//   3. seq — a strictly monotonic per-run cursor, so duplicates from
//      reconnect replays are no-ops.
// A delegation terminal event NEVER finishes the main run: only the Run
// status projection (controller-owned) decides mainStatus. That is why
// mainStatus is not derivable from any field of CraftEvent below.
export interface CraftState { generation:number; runId:string|null; seq:number; mainStatus:string; delegationStatus:string; versionId:string|null }
export interface CraftEvent { generation:number; runId:string; seq:number; kind:string; versionId?:string }
export function applyCraftEvent(s:CraftState,e:CraftEvent):CraftState {
  if(e.generation!==s.generation || e.runId!==s.runId || e.seq<=s.seq) return s;
  return {...s,seq:e.seq,
    delegationStatus:e.kind==='delegation.finished'?'finished':s.delegationStatus,
    versionId:e.kind==='artifact.published'?(e.versionId??s.versionId):s.versionId};
}
