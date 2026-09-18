import * as DocumentPicker from 'expo-document-picker';
import * as FileSystem from 'expo-file-system';
import type { PickedFile } from '../resources/upload.ts';

/**
 * 原生文件适配（MX-025）：系统选取器 + 本机读取。
 * content://（Android）/ file://（iOS）URI 只在本模块使用——真实 bytes 经 readBytes 流出。
 * 真机读取证据归 MX-034（native-e2e）；node 层经注入端口验证行为。
 */

export interface NativeFilePickResult {
  cancelled: boolean;
  file?: PickedFile;
}

/** 系统选取器（expo-document-picker；用户取消=cancelled）。 */
export async function pickNativeFile(): Promise<NativeFilePickResult> {
  const result = await DocumentPicker.getDocumentAsync({ multiple: false, copyToCacheDirectory: true });
  if (result.canceled || result.assets.length === 0) return { cancelled: true };
  const asset = result.assets[0]!;
  return {
    cancelled: false,
    file: {
      uri: asset.uri,
      name: asset.name ?? 'file',
      mimeType: asset.mimeType ?? 'application/octet-stream',
      sizeBytes: asset.size ?? 0,
    },
  };
}

/** 本机分块读取（base64 段解码为 bytes；URI 不离开本机）。 */
export async function readNativeBytes(uri: string, onChunk: (chunk: Uint8Array) => void, chunkSize = 1024 * 512): Promise<number> {
  const info = await FileSystem.getInfoAsync(uri);
  if (!info.exists) throw new Error('FILE_NOT_FOUND');
  const size = 'size' in info ? info.size : 0;
  let offset = 0;
  let total = 0;
  while (offset < size) {
    const readLength = Math.min(chunkSize, size - offset);
    const base64 = await FileSystem.readAsStringAsync(uri, { encoding: 'base64' as never, position: offset, length: readLength });
    const bytes = Uint8Array.from(atob(base64), (char) => char.charCodeAt(0));
    onChunk(bytes);
    total += bytes.length;
    offset += readLength;
  }
  return total;
}
