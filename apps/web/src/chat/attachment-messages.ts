import type { Locale } from '@weknora/i18n';

type AttachmentValidation = 'too-many' | 'too-large' | 'unsupported-type';

const COPY: Record<Locale, {
  tooMany: (maxFiles: number) => string;
  tooLarge: (maxSizeMb: number) => string;
  unsupported: (fileName: string) => string;
}> = {
  'zh-CN': {
    tooMany: (maxFiles) => `最多可添加 ${maxFiles} 个附件。`,
    tooLarge: (maxSizeMb) => `文件大小不能超过 ${maxSizeMb}MB！`,
    unsupported: (fileName) => `不支持的文件类型：${fileName}`,
  },
  'en-US': {
    tooMany: (maxFiles) => `You can attach up to ${maxFiles} files.`,
    tooLarge: (maxSizeMb) => `File size cannot exceed ${maxSizeMb}MB.`,
    unsupported: (fileName) => `Unsupported file type: ${fileName}`,
  },
  'ja-JP': {
    tooMany: (maxFiles) => `添付できるファイルは${maxFiles}個までです。`,
    tooLarge: (maxSizeMb) => `ファイルサイズは${maxSizeMb}MB以下にしてください。`,
    unsupported: (fileName) => `未対応のファイル形式です：${fileName}`,
  },
  'ko-KR': {
    tooMany: (maxFiles) => `최대 ${maxFiles}개의 파일을 첨부할 수 있습니다.`,
    tooLarge: (maxSizeMb) => `파일 크기는 ${maxSizeMb}MB를 초과할 수 없습니다.`,
    unsupported: (fileName) => `지원하지 않는 파일 형식입니다: ${fileName}`,
  },
  'ru-RU': {
    tooMany: (maxFiles) => `Можно прикрепить не более ${maxFiles} файлов.`,
    tooLarge: (maxSizeMb) => `Размер файла не может превышать ${maxSizeMb} МБ.`,
    unsupported: (fileName) => `Неподдерживаемый тип файла: ${fileName}`,
  },
};

export function resolveChatAttachmentValidationMessage(locale: Locale, validation: AttachmentValidation, fileName: string, maxSizeMb: number, maxFiles: number): string {
  const copy = COPY[locale];
  if (validation === 'too-many') return copy.tooMany(maxFiles);
  if (validation === 'too-large') return copy.tooLarge(maxSizeMb);
  return copy.unsupported(fileName);
}
