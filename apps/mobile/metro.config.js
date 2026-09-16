const { getDefaultConfig } = require('expo/metro-config');
const path = require('node:path');

const projectRoot = __dirname;
const workspaceRoot = path.resolve(projectRoot, '../..');
const config = getDefaultConfig(projectRoot, { isCSSEnabled: true });
config.watchFolders = [workspaceRoot];
config.resolver.nodeModulesPaths = [
  path.resolve(projectRoot, 'node_modules'),
  path.resolve(workspaceRoot, 'node_modules'),
];
config.resolver.assetExts.push('wasm');
const happyWireSource = path.join(workspaceRoot, 'packages/happy-wire/src/index.ts');
const preactCjsPath = require.resolve('preact');
const preactHooksCjsPath = require.resolve('preact/hooks');
const baseResolveRequest = config.resolver.resolveRequest;
config.resolver.resolveRequest = (context, moduleName, platform) => {
  if (moduleName === '@slopus/happy-wire') return { filePath: happyWireSource, type: 'sourceFile' };
  if (moduleName === 'preact') return { filePath: preactCjsPath, type: 'sourceFile' };
  if (moduleName === 'preact/hooks') return { filePath: preactHooksCjsPath, type: 'sourceFile' };
  return baseResolveRequest?.(context, moduleName, platform) ?? context.resolveRequest(context, moduleName, platform);
};
config.transformer.getTransformOptions = async () => ({ transform: { experimentalImportSupport: false, inlineRequires: true } });
module.exports = config;
