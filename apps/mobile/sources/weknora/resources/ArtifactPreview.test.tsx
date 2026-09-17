import * as React from 'react';
// @ts-expect-error react-test-renderer has no declarations in this workspace.
import { act, create } from 'react-test-renderer';
import { describe, expect, it, vi } from 'vitest';
import { ArtifactPreview } from './ArtifactPreview';
import { previewHeaders } from './preview-policy';

// The mount harness only asserts the WebView surface; keep every react-native
// dependency a host component so no native code path runs in the test process.
vi.mock('react-native', () => ({
  Linking: { openURL: vi.fn(async () => undefined) },
  Pressable: (props: any) => React.createElement('Pressable', props, props.children),
  StyleSheet: { create: (s: Record<string, unknown>) => s },
  Text: (props: any) => React.createElement('Text', props, props.children),
  View: (props: any) => React.createElement('View', props, props.children),
}));
vi.mock('react-native-webview', () => ({
  WebView: (props: any) => React.createElement('WebView', props),
}));

import { Linking } from 'react-native';

const openURL = Linking.openURL as ReturnType<typeof vi.fn>;
const PREVIEW_URL = 'https://preview.example.com/ap/TOKEN123';
const PRODUCT_API = 'https://api.example.com/api/v1/sessions';

function findall(root: any, type: string): any[] {
  const found: any[] = [];
  const walk = (node: any) => {
    if (!node) return;
    for (const child of node ?? []) {
      if (child?.type === type) found.push(child);
      if (child?.children) walk(child.children);
    }
  };
  walk(root?.children);
  return found;
}

function webviewsOf(renderer: any): any[] {
  return renderer.root.findAllByType('WebView');
}

function mountPreview(props: Partial<Parameters<typeof ArtifactPreview>[0]> = {}) {
  let renderer: any;
  act(() => {
    renderer = create(
      React.createElement(ArtifactPreview, {
        previewURL: PREVIEW_URL,
        artifactVersionID: 'v1',
        ...props,
      }),
    );
  });
  return renderer;
}

describe('W27 ArtifactPreview WebView hardening', () => {
  it('defers heavy rendering: only the placeholder card mounts until the user opts in', () => {
    const renderer = mountPreview();
    expect(findall(renderer.toJSON(), 'WebView')).toHaveLength(0);
    expect(JSON.stringify(renderer.toJSON())).toContain('加载预览');

    const button = renderer.root.findByProps({ testID: 'artifact-preview-load' });
    act(() => button.props.onPress());
    const webviews = findall(renderer.toJSON(), 'WebView');
    expect(webviews).toHaveLength(1);
  });

  it('mounts the WebView with the hardened preview config', () => {
    const renderer = mountPreview({ initiallyDeferred: false });
    const webviews = webviewsOf(renderer);
    expect(webviews).toHaveLength(1);
    const props = webviews[0].props;
    // originAllowList 仅预览 origin
    expect(props.originWhitelist).toEqual(['https://preview.example.com']);
    expect(props.source.uri).toBe(PREVIEW_URL);
    // 关闭任意文件访问
    expect(props.allowFileAccess).toBe(false);
    expect(props.allowFileAccessFromFileURLs).toBe(false);
    expect(props.allowUniversalAccessFromFileURLs).toBe(false);
    // 无原生桥
    expect(props.injectedJavaScript).toBeUndefined();
    expect(props.onMessage).toBeUndefined();
    // cookie / 存储 / 多窗口隔离
    expect(props.sharedCookiesEnabled).toBe(false);
    expect(props.thirdPartyCookiesEnabled).toBe(false);
    expect(props.setSupportMultipleWindows).toBe(false);
    expect(props.domStorageEnabled).toBe(false);
  });

  it('blocks product API, file URIs and foreign origins; hands external links to the system browser', () => {
    const renderer = mountPreview({ initiallyDeferred: false });
    const guard = webviewsOf(renderer)[0].props.onShouldStartLoadWithRequest as (r: {
      url: string;
    }) => boolean;

    expect(guard({ url: PRODUCT_API })).toBe(false);
    expect(guard({ url: 'file:///etc/passwd' })).toBe(false);
    expect(guard({ url: 'content://media/external/file/1' })).toBe(false);
    expect(guard({ url: 'intent://evil/#Intent;scheme=https;end' })).toBe(false);
    expect(guard({ url: 'javascript:alert(1)' })).toBe(false);

    expect(guard({ url: PREVIEW_URL })).toBe(true); // 预览 origin 自身放行

    expect(guard({ url: 'https://docs.example.com/guide' })).toBe(false); // 外链不进 WebView
    // 危险 scheme（file/content/intent/javascript）绝不外抛；http(s) 外链
    // （含产品 API——WebView 内已拒，凭据无从携带）交系统浏览器打开。
    const handedOut = openURL.mock.calls.map((call: unknown[]) => call[0]);
    expect(handedOut).toEqual([PRODUCT_API, 'https://docs.example.com/guide']);
  });

  it('fails closed on an unparseable ticket URL', () => {
    const renderer = mountPreview({ previewURL: '::bad url::', initiallyDeferred: false });
    expect(JSON.stringify(renderer.toJSON())).toContain('artifact-preview-invalid');
    expect(findall(renderer.toJSON(), 'WebView')).toHaveLength(0);
  });

  it('keeps the client-side policy constants in parity with the served headers', () => {
    // 客户端常量是服务端 artifact_preview.go 响应头的测试对照：两侧必须
    // 断言同一组指令（connect-src 'none' / sandbox allow-scripts / no-referrer）。
    const headers = previewHeaders();
    expect(headers['Content-Security-Policy']).toMatch(/connect-src 'none'/);
    expect(headers['Content-Security-Policy']).toMatch(/sandbox allow-scripts/);
    expect(headers['Referrer-Policy']).toBe('no-referrer');
  });
});
