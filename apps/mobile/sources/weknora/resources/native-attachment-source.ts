/**
 * Production NativeAttachmentSource (W25 I-1 fix): the document picker and
 * camera entry points backed by the Expo modules the app already ships.
 * Device-level behaviour (real content:// reads, permission prompts, camera
 * capture) is blocked-env manual evidence; this adapter only translates the
 * picked assets into AttachmentCandidate values for the upload manager.
 *
 * `consumeSharedFile` stays an inert seam: incoming OS share sheets need
 * app-manifest intent registration that lands with the device wiring round.
 */
import * as DocumentPicker from 'expo-document-picker';
import * as ImagePicker from 'expo-image-picker';
import type { AttachmentCandidate, NativeAttachmentSource } from './upload';

function safeSize(size: unknown): number {
  return typeof size === 'number' && Number.isSafeInteger(size) && size >= 0 ? size : 0;
}

function documentCandidate(asset: DocumentPicker.DocumentPickerAsset): AttachmentCandidate | null {
  if (!asset?.uri) return null;
  return {
    uri: asset.uri,
    name: asset.name?.trim() || `attachment-${Date.now()}`,
    mime: asset.mimeType ?? 'application/octet-stream',
    size: safeSize(asset.size),
  };
}

function photoCandidate(asset: ImagePicker.ImagePickerAsset): AttachmentCandidate | null {
  if (!asset?.uri) return null;
  const mime = asset.mimeType ?? 'image/jpeg';
  return {
    uri: asset.uri,
    name: asset.fileName?.trim() || `photo-${Date.now()}.jpg`,
    mime,
    size: safeSize(asset.fileSize),
    // Local preview only; the manager always uploads the original `uri`.
    previewUri: asset.uri,
    previewMime: mime,
  };
}

export const nativeAttachmentSource: NativeAttachmentSource = {
  async pickDocuments(): Promise<AttachmentCandidate[] | null> {
    // copyToCacheDirectory keeps content:// URIs readable by the platform
    // local file port on the same run; multiple selection feeds one attach
    // per candidate so a single failure cannot abandon the rest.
    const result = await DocumentPicker.getDocumentAsync({ multiple: true, copyToCacheDirectory: true });
    if (result.canceled) return null;
    const candidates = result.assets
      .map((asset) => documentCandidate(asset))
      .filter((candidate): candidate is AttachmentCandidate => candidate !== null);
    return candidates.length > 0 ? candidates : null;
  },
  async capturePhoto(): Promise<AttachmentCandidate | null> {
    const permission = await ImagePicker.requestCameraPermissionsAsync();
    // A denied permission behaves like a dismissed picker: no record, no error.
    if (!permission.granted) return null;
    const shot = await ImagePicker.launchCameraAsync({ exif: false, quality: 1 });
    if (shot.canceled) return null;
    return photoCandidate(shot.assets[0]);
  },
  async consumeSharedFile(): Promise<AttachmentCandidate | null> {
    // Seam pending the blocked-env share-intent adapter (see module comment).
    return null;
  },
};
