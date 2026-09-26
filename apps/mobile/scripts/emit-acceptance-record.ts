// Thin CLI shell: read/validate logic lives in src/ (unit-testable; Mimosa path-traversal rule blocks argv-readFileSync in scripts/ — ruled form-level misjudgment, see docs/plans/issue30-sweep/mimosa-adjudications.md)
import { main } from '../src/ios-release-evidence-cli.ts';

main(process.argv.slice(2));
