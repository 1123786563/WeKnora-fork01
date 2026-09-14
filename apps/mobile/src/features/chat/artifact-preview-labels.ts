import { formatMessage, type Locale } from '@weknora/i18n';

export interface NativeArtifactPreviewLabels {
  back: string;
  share: string;
  loading: string;
  downloadOnly: string;
}

export function nativeArtifactPreviewLabels(locale: Locale): NativeArtifactPreviewLabels {
  return {
    back: formatMessage(locale, 'mobileChat.back'),
    share: formatMessage(locale, 'mobileChat.share'),
    loading: formatMessage(locale, 'mobileChat.loadingPreview'),
    downloadOnly: formatMessage(locale, 'mobileChat.downloadOnly'),
  };
}
