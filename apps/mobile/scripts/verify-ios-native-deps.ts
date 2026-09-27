// Thin CLI shell: resolution/lock reading and gap logic live in src/ (unit-testable; Mimosa
// path-traversal rule blocks argv-readFileSync in scripts/ — layout ruling per
// docs/plans/issue30-sweep/mimosa-adjudications.md, same as emit-acceptance-record.ts).
import { main } from '../src/ios-native-deps-cli.ts';

main();
