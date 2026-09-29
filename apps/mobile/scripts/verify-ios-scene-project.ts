import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join, resolve } from 'node:path';

export function verifyIosSceneProject(iosDirectory: string): string[] {
  const root = resolve(iosDirectory);
  const issues: string[] = [];
  const read = (path: string) => {
    try { return readFileSync(join(root, path), 'utf8'); }
    catch { issues.push(`missing generated file: ${path}`); return ''; }
  };
  const plist = read('WeKnora/Info.plist');
  const appDelegate = read('WeKnora/AppDelegate.swift');
  const project = read('WeKnora.xcodeproj/project.pbxproj');
  const podProperties = read('Podfile.properties.json');
  const plistValue = parsePlist(plist);
  const manifest = objectValue(objectValue(plistValue)?.UIApplicationSceneManifest);
  const configurations = objectValue(manifest?.UISceneConfigurations);
  const applicationScenes = configurations?.UIWindowSceneSessionRoleApplication;
  const sceneDelegateNames = Array.isArray(applicationScenes)
    ? applicationScenes.map((scene) => objectValue(scene)?.UISceneDelegateClassName)
    : [];
  if (!sceneDelegateNames.includes('EXExpoAppSceneDelegate')) {
    issues.push('WeKnora/Info.plist application scene role must map to EXExpoAppSceneDelegate');
  }
  if (!appDelegate.includes('ExpoReactNativeFactoryProvider')) {
    issues.push('AppDelegate must conform to ExpoReactNativeFactoryProvider');
  }
  const openUrlBody = swiftMethodBody(appDelegate, /application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*open\s+\w+\s*:\s*URL/);
  if (!openUrlBody?.includes('RCTLinkingManager.application')) {
    issues.push('AppDelegate open-URL callback must forward to RCTLinkingManager.application');
  }
  const universalLinkBody = swiftMethodBody(appDelegate, /application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*continue\s+\w+\s*:\s*NSUserActivity/);
  if (!universalLinkBody?.includes('RCTLinkingManager.application')) {
    issues.push('AppDelegate universal-link callback must forward to RCTLinkingManager.application');
  }
  const nativeTarget = project.match(/\/\* Begin PBXNativeTarget section \*\/([\s\S]*?)\/\* End PBXNativeTarget section \*\//)?.[1] ?? '';
  const targetBlock = nativeTarget.match(/([A-F0-9]+)\s*\/\* WeKnora \*\/\s*=\s*\{([\s\S]*?)\n\s*\};/)?.[2] ?? '';
  const configListId = targetBlock.match(/buildConfigurationList\s*=\s*([A-F0-9]+)/)?.[1];
  const configLists = project.match(/\/\* Begin XCConfigurationList section \*\/([\s\S]*?)\/\* End XCConfigurationList section \*\//)?.[1] ?? '';
  const configListBlock = configListId ? pbxBlock(configLists, configListId) : undefined;
  const configIds = [...(configListBlock ?? '').matchAll(/([A-F0-9]+)(?=\s*(?:\/\*[^*]*\*\/\s*)?[,])/g)].map((match) => match[1]);
  const configSections = project.match(/\/\* Begin XCBuildConfiguration section \*\/([\s\S]*?)\/\* End XCBuildConfiguration section \*\//)?.[1] ?? '';
  const deploymentValues = configIds.map((id) => {
    const block = pbxBlock(configSections, id);
    return block?.match(/IPHONEOS_DEPLOYMENT_TARGET\s*=\s*([^;]+);/)?.[1].trim();
  });
  if (configIds.length === 0 || deploymentValues.some((value) => value !== '16.4')) {
    issues.push('All app target deployment settings must be 16.4');
  }
  try {
    const properties = JSON.parse(podProperties) as Record<string, unknown>;
    if (properties['ios.deploymentTarget'] !== '16.4') issues.push('Podfile properties deployment target must be 16.4');
  } catch { issues.push('Podfile.properties.json must be valid JSON with iOS deployment target 16.4'); }
  return issues;
}

function pbxBlock(section: string, id: string): string | undefined {
  const declaration = new RegExp(`(?:^|\\n)\\s*${id}\\s*(?:\\/\\*[^*]*\\*\\/\\s*)?=\\s*\\{`).exec(section);
  if (!declaration) return undefined;
  const open = section.indexOf('{', declaration.index + declaration[0].length - 1);
  let depth = 1;
  for (let index = open + 1; index < section.length; index++) {
    if (section[index] === '{') depth++;
    if (section[index] === '}' && --depth === 0) return section.slice(open + 1, index);
  }
  return undefined;
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

// Parse the plist's dict/array/string/integer/boolean subset without adding a runtime dependency.
function parsePlist(xml: string): unknown {
  const tokens = (xml.match(/<\/?(?:dict|array|string|integer|true|false|key)\s*\/?\s*>|[^<]+/g) ?? [])
    .filter((token) => token.startsWith('<') || token.trim().length > 0);
  let index = 0;
  const parseValue = (): unknown => {
    const token = tokens[index++]?.trim();
    if (token === '<dict>') {
      const result: Record<string, unknown> = {};
      while (tokens[index]?.trim() !== '</dict>') {
        if (tokens[index++]?.trim() !== '<key>') throw new Error('Malformed plist dictionary key');
        const key = tokens[index++]?.trim() ?? '';
        if (tokens[index++]?.trim() !== '</key>') throw new Error('Malformed plist dictionary key');
        result[key] = parseValue();
      }
      index++;
      return result;
    }
    if (token === '<array>') {
      const result: unknown[] = [];
      while (tokens[index]?.trim() !== '</array>') result.push(parseValue());
      index++;
      return result;
    }
    if (token === '<string>' || token === '<integer>') {
      const value = tokens[index++]?.trim() ?? '';
      index++;
      return value;
    }
    if (token === '<true/>' || token === '<true />') return true;
    if (token === '<false/>' || token === '<false />') return false;
    throw new Error('Malformed plist value');
  };
  try {
    const root = tokens.findIndex((token) => token.trim() === '<dict>');
    if (root < 0) return undefined;
    index = root;
    return parseValue();
  } catch { return undefined; }
}

function swiftMethodBody(source: string, signature: RegExp): string | undefined {
  const match = signature.exec(source);
  if (!match) return undefined;
  const open = source.indexOf('{', match.index + match[0].length);
  if (open < 0) return undefined;
  let depth = 1;
  for (let index = open + 1; index < source.length; index++) {
    if (source[index] === '{') depth++;
    if (source[index] === '}' && --depth === 0) return source.slice(open + 1, index);
  }
  return undefined;
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  const directory = process.argv[2] ?? join(process.cwd(), 'ios');
  const issues = verifyIosSceneProject(directory);
  if (issues.length) {
    console.error(`Generated iOS scene contract failed for ${directory}:\n- ${issues.join('\n- ')}`);
    process.exitCode = 1;
  } else console.log(`Generated iOS scene contract passed: ${directory}`);
}
