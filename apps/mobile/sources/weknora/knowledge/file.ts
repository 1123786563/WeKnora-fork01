export function bytesToBase64(bytes: ArrayBuffer): string {
  const values = new Uint8Array(bytes);
  let binary = '';
  const chunkSize = 0x8000;
  for (let offset = 0; offset < values.length; offset += chunkSize) {
    binary += String.fromCharCode(...values.subarray(offset, offset + chunkSize));
  }
  return btoa(binary);
}

export function previewText(bytes: ArrayBuffer, contentType: string): string | null {
  if (contentType.startsWith('text/') || contentType.includes('json') || contentType.includes('xml')) {
    return new TextDecoder().decode(bytes);
  }
  return null;
}
