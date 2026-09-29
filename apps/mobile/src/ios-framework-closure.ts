export interface FrameworkBinary {
  framework: string;
  loadCommands: string[];
}

/** Return missing non-system @rpath framework dependencies for embedded binaries. */
export function findMissingFrameworkDependencies(
  binaries: FrameworkBinary[],
  availableFrameworks: Set<string>,
): string[] {
  const missing = new Set<string>();
  for (const binary of binaries) {
    for (const command of binary.loadCommands) {
      const match = command.match(/^\s*@rpath\/([^/]+\.framework)\//);
      if (match && !availableFrameworks.has(match[1])) missing.add(match[1]);
    }
  }
  return [...missing].sort();
}
