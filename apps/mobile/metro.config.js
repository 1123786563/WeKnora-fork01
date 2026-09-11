const { getDefaultConfig } = require('expo/metro-config');
const path = require('node:path');

const config = getDefaultConfig(__dirname, { isCSSEnabled: true });
config.resolver.assetExts.push('wasm');
const workspaceRoot = path.resolve(__dirname, '../..');
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
