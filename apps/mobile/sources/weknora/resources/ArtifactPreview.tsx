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
 *   - originWhitelist 仅预览 origin；被拦截的外链绝不自动交系统浏览器
 *     （预览内容不能让设备自动打开任意 URL——系统浏览器持有产品站点
 *     cookie，自动外抛即 GET-CSRF/钓鱼面）。外链只被记录为候选，并经
 *     「在浏览器打开」按钮这一显式用户手势才外抛（F2）；
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
  // 被拦截导航携带的外链候选（仅最新一条）。绝不自动外抛——系统浏览器
  // 只能由用户点按「在浏览器打开」触达（F2）。
  const [externalCandidate, setExternalCandidate] = React.useState<string | null>(null);

  // 新票据（重新签发/刷新）重新进入延迟渲染状态，外链候选一并清空。
  React.useEffect(() => {
    setActivated(!initiallyDeferred);
    setExternalCandidate(null);
  }, [previewURL, artifactVersionID, initiallyDeferred]);

  // 预览 origin 之外的导航一律拒绝；外链只记录为候选，不自动外抛。
  const onShouldStartLoadWithRequest = React.useCallback(
    (request: { url: string; isTopLevel?: boolean }) => {
      const decision = decidePreviewNavigation(request, allowedOrigin);
      if (!decision.allow && decision.external) {
        setExternalCandidate(decision.external);
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
      {externalCandidate ? (
        <View style={styles.externalRow} testID="artifact-preview-external-row">
          {/* 只显示外链 origin（不显示完整路径），用户可辨识目标且不被
              长 URL 钓鱼文案铺满。 */}
          <Text numberOfLines={1} style={styles.externalText}>
            外部链接 · {previewOriginOf(externalCandidate).replace(/^https?:\/\//, '')}
          </Text>
          <Pressable
            accessibilityLabel="在浏览器打开"
            accessibilityRole="button"
            onPress={() => {
              void Linking.openURL(externalCandidate).catch(() => undefined);
              setExternalCandidate(null);
            }}
            style={styles.externalButton}
            testID="artifact-preview-external-open"
          >
            <Text style={styles.loadText}>在浏览器打开</Text>
          </Pressable>
          <Pressable
            accessibilityLabel="关闭外部链接提示"
            accessibilityRole="button"
            onPress={() => setExternalCandidate(null)}
            style={styles.externalDismiss}
            testID="artifact-preview-external-dismiss"
          >
            <Text style={styles.externalDismissText}>✕</Text>
          </Pressable>
        </View>
      ) : null}
      <View style={styles.webviewFrame}>
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
    </View>
  );
};

const styles = StyleSheet.create({
  container: { borderRadius: 8, overflow: 'hidden' },
  webviewFrame: { height: 360 },
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
  externalRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    paddingHorizontal: 10,
    paddingVertical: 6,
    backgroundColor: '#f1f3f7',
  },
  externalText: { flex: 1, fontSize: 12, color: '#3c4450' },
  externalButton: {
    borderRadius: 6,
    backgroundColor: '#2f6fed',
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  externalDismiss: { padding: 4 },
  externalDismissText: { fontSize: 13, color: '#5b6472' },
  error: {
    borderRadius: 8,
    borderWidth: 1,
    borderColor: '#d83232',
    padding: 12,
  },
  errorText: { fontSize: 13, color: '#b42323' },
});
