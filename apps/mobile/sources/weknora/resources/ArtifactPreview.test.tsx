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

  it('blocks product API, file URIs and foreign origins; never auto-opens the system browser', () => {
    const renderer = mountPreview({ initiallyDeferred: false });
    const guard = webviewsOf(renderer)[0].props.onShouldStartLoadWithRequest as (r: {
      url: string;
      isTopLevel?: boolean;
    }) => boolean;

    expect(guard({ url: PRODUCT_API })).toBe(false);
    expect(guard({ url: 'file:///etc/passwd' })).toBe(false);
    expect(guard({ url: 'content://media/external/file/1' })).toBe(false);
    expect(guard({ url: 'intent://evil/#Intent;scheme=https;end' })).toBe(false);
    expect(guard({ url: 'javascript:alert(1)' })).toBe(false);

    expect(guard({ url: PREVIEW_URL })).toBe(true); // 预览 origin 自身放行

    expect(guard({ url: 'https://docs.example.com/guide' })).toBe(false); // 外链不进 WebView

    // F2: 被拦截导航绝不自动外抛——预览内容不能让设备自动打开任意 URL
    // （含产品 origin：系统浏览器持有产品 cookie，自动外抛即 GET-CSRF/钓鱼面）。
    // 系统浏览器只能经由用户显式手势（「在浏览器打开」按钮）触达。
    expect(openURL).not.toHaveBeenCalled();
  });

  it('surfaces a blocked external link as an explicit user-gated affordance, not an auto-open', () => {
    const renderer = mountPreview({ initiallyDeferred: false });
    const guard = webviewsOf(renderer)[0].props.onShouldStartLoadWithRequest as (r: {
      url: string;
      isTopLevel?: boolean;
    }) => boolean;

    // 恶意预览内容尝试把 WebView 顶层导航到产品 API：被拦截、被记录为候选。
    let allowed: boolean | undefined;
    act(() => {
      allowed = guard({ url: PRODUCT_API, isTopLevel: true });
    });
    expect(allowed).toBe(false);
    // 脚本连发多个外链：仅记录最新候选，仍然零外抛。
    act(() => {
      allowed = guard({ url: 'https://docs.example.com/guide' });
    });
    expect(allowed).toBe(false);
    expect(openURL).not.toHaveBeenCalled();

    // 显式入口出现，展示外链 host（用户可辨识目标），未点按前不打开。
    const row = renderer.root.findByProps({ testID: 'artifact-preview-external-row' });
    const rowText = row
      .findAllByType('Text')
      .map((node: any) => (Array.isArray(node.props.children) ? node.props.children.join('') : node.props.children))
      .join(' ');
    expect(rowText).toContain('docs.example.com');

    // 用户点按「在浏览器打开」——唯一的 Linking.openURL 路径，且仅一次、仅该 URL。
    act(() => {
      renderer.root.findByProps({ testID: 'artifact-preview-external-open' }).props.onPress();
    });
    expect(openURL).toHaveBeenCalledTimes(1);
    expect(openURL).toHaveBeenCalledWith('https://docs.example.com/guide');

    // 打开后候选清空（入口消失）。
    expect(() => renderer.root.findByProps({ testID: 'artifact-preview-external-row' })).toThrow();

    // 再次被拦截导航重新出现候选；用户可以只关闭、不打开。
    act(() => {
      allowed = guard({ url: 'https://other.example.net/x' });
    });
    expect(allowed).toBe(false);
    act(() => {
      renderer.root.findByProps({ testID: 'artifact-preview-external-dismiss' }).props.onPress();
    });
    expect(openURL).toHaveBeenCalledTimes(1); // 关闭不打开
    expect(() => renderer.root.findByProps({ testID: 'artifact-preview-external-row' })).toThrow();
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
