import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { join, resolve } from 'node:path';

export function verifyIosSceneProject(iosDirectory: string): string[] {
  const root = resolve(iosDirectory);
  const issues: string[] = [];
  const read = (path: string): string | undefined => {
    try { return readFileSync(join(root, path), 'utf8'); }
    catch { issues.push(`missing generated file: ${path}`); return undefined; }
  };
  const plist = read('WeKnora/Info.plist');
  const appDelegate = read('WeKnora/AppDelegate.swift');
  const project = read('WeKnora.xcodeproj/project.pbxproj');
  const podProperties = read('Podfile.properties.json');
  if (plist !== undefined) {
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
  }
  if (appDelegate !== undefined) {
    const swiftCode = maskSwiftNonCode(appDelegate);
    if (!swiftCode?.includes('ExpoReactNativeFactoryProvider')) {
      issues.push('AppDelegate must conform to ExpoReactNativeFactoryProvider');
    }
    const openUrlBody = swiftCode && swiftMethodBody(swiftCode, /\boverride\s+func\s+application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*open\s+\w+\s*:\s*URL\b/);
    if (!openUrlBody || !/\bRCTLinkingManager\s*\.\s*application\s*\(/.test(openUrlBody)) {
      issues.push('AppDelegate open-URL callback must forward to RCTLinkingManager.application');
    }
    const universalLinkBody = swiftCode && swiftMethodBody(swiftCode, /\boverride\s+func\s+application\s*\(\s*_?\s*\w+\s*:\s*UIApplication\s*,\s*continue\s+\w+\s*:\s*NSUserActivity\b/);
    if (!universalLinkBody || !/\bRCTLinkingManager\s*\.\s*application\s*\(/.test(universalLinkBody)) {
      issues.push('AppDelegate universal-link callback must forward to RCTLinkingManager.application');
    }
  }
  if (project !== undefined) {
  const nativeTarget = project.match(/\/\* Begin PBXNativeTarget section \*\/([\s\S]*?)\/\* End PBXNativeTarget section \*\//)?.[1] ?? '';
  const targetBlock = nativeTarget.match(/([A-F0-9]+)\s*\/\* WeKnora \*\/\s*=\s*\{([\s\S]*?)\n\s*\};/)?.[2] ?? '';
  const configListId = targetBlock.match(/buildConfigurationList\s*=\s*([A-F0-9]+)/)?.[1];
  const configLists = project.match(/\/\* Begin XCConfigurationList section \*\/([\s\S]*?)\/\* End XCConfigurationList section \*\//)?.[1] ?? '';
  const configListBlock = configListId ? pbxBlock(configLists, configListId) : undefined;
  const configIds = [...(configListBlock ?? '').matchAll(/([A-F0-9]+)(?=\s*(?:\/\*[^*]*\*\/\s*)?[,])/g)].map((match) => match[1]);
  const configSections = project.match(/\/\* Begin XCBuildConfiguration section \*\/([\s\S]*?)\/\* End XCBuildConfiguration section \*\//)?.[1] ?? '';
  const deploymentValues = configIds.map((id) => {
    const block = pbxBlock(stripPbxBlockComments(configSections), id);
    const buildSettings = block ? pbxDictionary(block, 'buildSettings') : undefined;
    return buildSettings?.match(/(?:^|\n)\s*IPHONEOS_DEPLOYMENT_TARGET\s*=\s*([^;]+);/)?.[1].trim();
  });
  if (configIds.length === 0) {
    issues.push('Unable to locate WeKnora build configuration list in project.pbxproj');
  } else if (deploymentValues.some((value) => value !== '16.4')) {
    issues.push('All app target deployment settings must be 16.4');
  }
  }
  try {
    if (podProperties === undefined) return issues;
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

function stripPbxBlockComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, '');
}

function pbxDictionary(source: string, key: string): string | undefined {
  const declaration = new RegExp(`(?:^|[,{\\n])\\s*${key}\\s*=\\s*\\{`).exec(source);
  if (!declaration) return undefined;
  const open = source.indexOf('{', declaration.index + declaration[0].length - 1);
  let depth = 1;
  for (let index = open + 1; index < source.length; index++) {
    if (source[index] === '{') depth++;
    if (source[index] === '}' && --depth === 0) return source.slice(open + 1, index);
  }
  return undefined;
}

function objectValue(value: unknown): Record<string, unknown> | undefined {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : undefined;
}

// Parse the plist value forms emitted by the generated Xcode project without a runtime dependency.
function parsePlist(xml: string): unknown {
  const tokens = (xml.match(/<\/?(?:dict|array|string|integer|real|date|data|true|false|key)\s*\/?\s*>|[^<]+/g) ?? [])
    .filter((token) => token.startsWith('<') || token.trim().length > 0);
  let index = 0;
  const parseValue = (): unknown => {
    const token = tokens[index++]?.trim();
    if (token === '<dict/>' || token === '<dict />') return {};
    if (token === '<array/>' || token === '<array />') return [];
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
    if (token === '<string>' || token === '<integer>' || token === '<real>' || token === '<date>' || token === '<data>') {
      const closing = `</${token.slice(1, -1)}>`;
      let value = '';
      if (tokens[index]?.trim() !== closing) value = tokens[index++]?.trim() ?? '';
      if (tokens[index++]?.trim() !== closing) throw new Error('Malformed plist scalar closing element');
      return value;
    }
    if (token === '<string/>' || token === '<string />' || token === '<data/>' || token === '<data />') return '';
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

// Preserve positions and braces in executable code while hiding Swift comments and literals.
// This only recognizes the lexical forms needed to inspect generated AppDelegate callbacks.
function maskSwiftNonCode(source: string): string | undefined {
  const result = source.split('');
  const mask = (index: number) => { if (source[index] !== '\n' && source[index] !== '\r') result[index] = ' '; };
  let state: 'code' | 'line-comment' | 'block-comment' | 'string' = 'code';
  let blockDepth = 0;
  let delimiter = '';
  let multiline = false;
  let rawHashes = 0;
  for (let index = 0; index < source.length;) {
    const current = source[index];
    const next = source[index + 1];
    if (state === 'code') {
      if (current === '/' && (next === '/' || next === '*')) {
        state = next === '/' ? 'line-comment' : 'block-comment';
        blockDepth = next === '*' ? 1 : 0;
        mask(index++); mask(index++);
        continue;
      }
      let quote = index;
      if (current === '#') { while (source[quote] === '#') quote++; }
      if ((source[quote] === '"' && (current === '"' || current === '#')) || current === "'") {
        rawHashes = quote - index;
        multiline = source.slice(quote, quote + 3) === '"""';
        let quoteDelimiter: string;
        if (current === "'") quoteDelimiter = "'";
        else if (multiline) quoteDelimiter = '"""';
        else quoteDelimiter = '"';
        delimiter = quoteDelimiter + '#'.repeat(rawHashes);
        state = 'string';
        const end = quote + (multiline ? 3 : 1);
        while (index < end) mask(index++);
        continue;
      }
      index++;
      continue;
    }
    if (state === 'line-comment') {
      if (current === '\n' || current === '\r') state = 'code';
      else mask(index);
      index++;
      continue;
    }
    if (state === 'block-comment') {
      if (current === '/' && next === '*') { blockDepth++; mask(index++); mask(index++); }
      else if (current === '*' && next === '/') {
        blockDepth--; mask(index++); mask(index++);
        if (blockDepth === 0) state = 'code';
      } else mask(index++);
      continue;
    }
    if (source.startsWith(delimiter, index)) {
      for (let count = 0; count < delimiter.length; count++) mask(index++);
      state = 'code';
      continue;
    }
    if (!multiline && (current === '\n' || current === '\r')) return undefined;
    if (current === '\\' && rawHashes > 0 && source.startsWith('#'.repeat(rawHashes) + '"', index + 1)) {
      // A raw string's escaped quote is content, not its closing delimiter.
      for (let count = 0; count < rawHashes + 2; count++) mask(index++);
      continue;
    }
    if (current === '\\' && rawHashes === 0) {
      mask(index++);
      if (index < source.length) mask(index++);
    } else mask(index++);
  }
  return state === 'code' || state === 'line-comment' ? result.join('') : undefined;
}

function swiftMethodBody(source: string, signature: RegExp): string | undefined {
  const match = signature.exec(source);
  if (!match) return undefined;
  const openParameters = source.indexOf('(', match.index);
  let parentheses = 0;
  let closeParameters = -1;
  for (let index = openParameters; index < source.length; index++) {
    if (source[index] === '(') parentheses++;
    else if (source[index] === ')' && --parentheses === 0) { closeParameters = index; break; }
    else if (source[index] === '{' || source[index] === '}') return undefined;
  }
  if (closeParameters < 0) return undefined;
  const opening = /^\s*->\s*Bool\s*\{/.exec(source.slice(closeParameters + 1));
  if (!opening) return undefined;
  const open = closeParameters + 1 + opening[0].length - 1;
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
