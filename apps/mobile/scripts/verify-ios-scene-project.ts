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
  if (!plist.includes('UIApplicationSceneManifest') || !plist.includes('EXExpoAppSceneDelegate')) {
    issues.push('WeKnora/Info.plist must configure EXExpoAppSceneDelegate in UIApplicationSceneManifest');
  }
  if (!appDelegate.includes('ExpoReactNativeFactoryProvider')) {
    issues.push('AppDelegate must conform to ExpoReactNativeFactoryProvider');
  }
  if (!appDelegate.includes('RCTLinkingManager') || !/application\s*\([^)]*open\s+url/s.test(appDelegate)) {
    issues.push('AppDelegate must forward app URL callbacks through RCTLinkingManager');
  }
  if (!/IPHONEOS_DEPLOYMENT_TARGET\s*=\s*16\.4\s*;/.test(project)) {
    issues.push('Xcode project target deployment setting must be 16.4');
  }
  try {
    const properties = JSON.parse(podProperties) as Record<string, unknown>;
    if (properties['ios.deploymentTarget'] !== '16.4') issues.push('Podfile properties deployment target must be 16.4');
  } catch { issues.push('Podfile.properties.json must be valid JSON with iOS deployment target 16.4'); }
  return issues;
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(fileURLToPath(import.meta.url))) {
  const directory = process.argv[2] ?? join(process.cwd(), 'ios');
  const issues = verifyIosSceneProject(directory);
  if (issues.length) {
    console.error(`Generated iOS scene contract failed for ${directory}:\n- ${issues.join('\n- ')}`);
    process.exitCode = 1;
  } else console.log(`Generated iOS scene contract passed: ${directory}`);
}
