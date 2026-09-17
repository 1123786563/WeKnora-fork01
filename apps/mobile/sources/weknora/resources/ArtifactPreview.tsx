/**
 * W27 — 隔离预览 WebView（isolated artifact preview surface）。
 *
 * 渲染生成的产物预览（HTML/图表/代码）。预览内容来自独立 preview origin
 * 的票据 URL（服务端 `internal/handler/artifact_preview.go` 签发），该
 * origin 与产品 API 不同源、不携带产品 cookie/token，响应自带 CSP
 * `sandbox allow-scripts` + `connect-src 'none'` —— 预览页脚本既读不到
 * 任何存储，也连不了任何端点。
 *
 * 本组件的加固全部来自 `preview-policy.ts` 的纯函数配置：
 *   - originWhitelist 仅预览 origin；其他 origin 的链接交给系统浏览器；
 *   - 关闭任意文件访问（file:/content: 一类 URI 与 file 域互访）；
 *   - 无原生桥：不注入脚本、不注册 onMessage（postMessage 到原生无接收方）；
 *   - cookie 隔离（不共享 iOS 全局 cookie、不写第三方 cookie）；
 *   - 大型图表/代码默认延迟渲染：先渲染轻量占位卡，用户点按后才挂
 *     WebView，避免大产物在列表滚动时抢占渲染资源。
 *
 * 真机 WebView 行为属 blocked-env 分层；本文件的可判定面是配置本身
 * （`buildPreviewWebViewConfig` 的纯数据断言 + 组件挂载测试）。
 */
import * as React from 'react';
import { Linking, Pressable, StyleSheet, Text, View } from 'react-native';
import { WebView } from 'react-native-webview';
import {
  buildPreviewWebViewConfig,
  decidePreviewNavigation,
  previewOriginOf,
} from './preview-policy';

export interface ArtifactPreviewProps {
  /** 服务端签发的隔离预览票据 URL（https://preview-origin/ap/<token>）。 */
  previewURL: string;
  /** 该预览绑定的不可变产物版本 ID（W26 ArtifactVersion）。 */
  artifactVersionID: string;
  /** 大型图表/代码延迟渲染：默认开启，用户点按「加载预览」后才挂 WebView。 */
  initiallyDeferred?: boolean;
}

export const ArtifactPreview: React.FC<ArtifactPreviewProps> = ({
  previewURL,
  artifactVersionID,
  initiallyDeferred = true,
}) => {
  const allowedOrigin = React.useMemo(() => previewOriginOf(previewURL), [previewURL]);
  const config = React.useMemo(() => buildPreviewWebViewConfig(previewURL), [previewURL]);
  const [activated, setActivated] = React.useState(!initiallyDeferred);

  // 新票据（重新签发/刷新）重新进入延迟渲染状态。
  React.useEffect(() => {
    setActivated(!initiallyDeferred);
  }, [previewURL, artifactVersionID, initiallyDeferred]);

  // 外链交系统浏览器；预览 origin 之外的导航一律拒绝。
  const onShouldStartLoadWithRequest = React.useCallback(
    (request: { url: string; isTopLevel?: boolean }) => {
      const decision = decidePreviewNavigation(request, allowedOrigin);
      if (decision.external) {
        void Linking.openURL(decision.external).catch(() => undefined);
      }
      return decision.allow;
    },
    [allowedOrigin],
  );

  // 票据 URL 非法（解析不出 origin）：fail-closed，不加载任何内容。
  if (!allowedOrigin) {
    return (
      <View style={styles.error} testID="artifact-preview-invalid">
        <Text style={styles.errorText}>预览票据无效，请回到会话重新打开预览。</Text>
      </View>
    );
  }

  // 延迟渲染：占位卡 + 显式点按，WebView 只在激活后挂载。
  if (!activated) {
    return (
      <View style={styles.placeholder} testID="artifact-preview-deferred">
        <Text style={styles.hint}>产物预览 · 隔离渲染（不携带产品凭据）</Text>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="加载预览"
          onPress={() => setActivated(true)}
          style={styles.loadButton}
          testID="artifact-preview-load"
        >
          <Text style={styles.loadText}>加载预览</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={styles.container} testID="artifact-preview-mounted">
      <WebView
        source={{ uri: previewURL }}
        originWhitelist={config.originWhitelist}
        onShouldStartLoadWithRequest={onShouldStartLoadWithRequest}
        allowFileAccess={config.allowFileAccess}
        allowFileAccessFromFileURLs={config.allowFileAccessFromFileURLs}
        allowUniversalAccessFromFileURLs={config.allowUniversalAccessFromFileURLs}
        sharedCookiesEnabled={config.sharedCookiesEnabled}
        thirdPartyCookiesEnabled={config.thirdPartyCookiesEnabled}
        setSupportMultipleWindows={config.setSupportMultipleWindows}
        domStorageEnabled={false}
        // 图表（mermaid/katex 单文件构建）需要脚本渲染；隔离由独立 origin +
        // 服务端 CSP sandbox（opaque origin，无存储/无网络）提供，而非关 JS。
        javaScriptEnabled
        // 无原生桥：不注入 injectedJavaScript，不注册 onMessage —— 预览页
        // 的 postMessage 没有任何原生接收方。
      />
    </View>
  );
};

const styles = StyleSheet.create({
  container: { height: 360, borderRadius: 8, overflow: 'hidden' },
  placeholder: {
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#d0d4dc',
    padding: 16,
    gap: 12,
    alignItems: 'flex-start',
  },
  hint: { fontSize: 13, color: '#5b6472' },
  loadButton: {
    borderRadius: 6,
    backgroundColor: '#2f6fed',
    paddingHorizontal: 14,
    paddingVertical: 8,
  },
  loadText: { color: '#ffffff', fontSize: 13, fontWeight: '600' },
  error: {
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#d83232',
    padding: 12,
  },
  errorText: { fontSize: 13, color: '#b42323' },
});
