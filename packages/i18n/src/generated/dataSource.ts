import type { Locale } from '../index.ts';

export const dataSourceMessages: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'dataSource.syncHistory': '同步历史', 'dataSource.refreshLogs': '刷新日志', 'dataSource.noLogs': '暂无同步记录', 'dataSource.loadMore': '加载更多', 'dataSource.loadingMore': '加载中…',
    'dataSource.summary.total': '总次数', 'dataSource.summary.success': '成功', 'dataSource.summary.failed': '失败', 'dataSource.summary.items': '同步条目',
    'dataSource.status.running': '同步中', 'dataSource.status.success': '成功', 'dataSource.status.partial': '部分成功', 'dataSource.status.failed': '失败', 'dataSource.status.canceled': '已取消',
    'dataSource.detail.created': '新增', 'dataSource.detail.updated': '更新', 'dataSource.detail.deleted': '删除', 'dataSource.detail.skipped': '跳过', 'dataSource.detail.failed': '失败',
  },
  'en-US': {
    'dataSource.syncHistory': 'Sync History', 'dataSource.refreshLogs': 'Refresh logs', 'dataSource.noLogs': 'No sync records yet', 'dataSource.loadMore': 'Load more', 'dataSource.loadingMore': 'Loading more…',
    'dataSource.summary.total': 'Runs', 'dataSource.summary.success': 'Success', 'dataSource.summary.failed': 'Failed', 'dataSource.summary.items': 'Items',
    'dataSource.status.running': 'Syncing', 'dataSource.status.success': 'Success', 'dataSource.status.partial': 'Partial', 'dataSource.status.failed': 'Failed', 'dataSource.status.canceled': 'Canceled',
    'dataSource.detail.created': 'Created', 'dataSource.detail.updated': 'Updated', 'dataSource.detail.deleted': 'Deleted', 'dataSource.detail.skipped': 'Skipped', 'dataSource.detail.failed': 'Failed',
  },
  'ja-JP': {
    'dataSource.syncHistory': '同期履歴', 'dataSource.refreshLogs': 'ログを更新', 'dataSource.noLogs': '同期記録はまだありません', 'dataSource.loadMore': 'さらに読み込む', 'dataSource.loadingMore': '読み込み中…',
    'dataSource.summary.total': '実行回数', 'dataSource.summary.success': '成功', 'dataSource.summary.failed': '失敗', 'dataSource.summary.items': '項目',
    'dataSource.status.running': '同期中', 'dataSource.status.success': '成功', 'dataSource.status.partial': '一部成功', 'dataSource.status.failed': '失敗', 'dataSource.status.canceled': 'キャンセル済み',
    'dataSource.detail.created': '作成', 'dataSource.detail.updated': '更新', 'dataSource.detail.deleted': '削除', 'dataSource.detail.skipped': 'スキップ', 'dataSource.detail.failed': '失敗',
  },
  'ko-KR': {
    'dataSource.syncHistory': '동기화 기록', 'dataSource.refreshLogs': '로그 새로고침', 'dataSource.noLogs': '동기화 기록이 없습니다', 'dataSource.loadMore': '더 불러오기', 'dataSource.loadingMore': '로드 중…',
    'dataSource.summary.total': '실행', 'dataSource.summary.success': '성공', 'dataSource.summary.failed': '실패', 'dataSource.summary.items': '항목',
    'dataSource.status.running': '동기화 중', 'dataSource.status.success': '성공', 'dataSource.status.partial': '부분 성공', 'dataSource.status.failed': '실패', 'dataSource.status.canceled': '취소됨',
    'dataSource.detail.created': '생성', 'dataSource.detail.updated': '업데이트', 'dataSource.detail.deleted': '삭제', 'dataSource.detail.skipped': '건너뜀', 'dataSource.detail.failed': '실패',
  },
  'ru-RU': {
    'dataSource.syncHistory': 'История синхронизации', 'dataSource.refreshLogs': 'Обновить журнал', 'dataSource.noLogs': 'Нет записей синхронизации', 'dataSource.loadMore': 'Загрузить ещё', 'dataSource.loadingMore': 'Загрузка…',
    'dataSource.summary.total': 'Запуски', 'dataSource.summary.success': 'Успешно', 'dataSource.summary.failed': 'Ошибки', 'dataSource.summary.items': 'Элементы',
    'dataSource.status.running': 'Синхронизация', 'dataSource.status.success': 'Успешно', 'dataSource.status.partial': 'Частично', 'dataSource.status.failed': 'Ошибка', 'dataSource.status.canceled': 'Отменено',
    'dataSource.detail.created': 'Создано', 'dataSource.detail.updated': 'Обновлено', 'dataSource.detail.deleted': 'Удалено', 'dataSource.detail.skipped': 'Пропущено', 'dataSource.detail.failed': 'Ошибки',
  },
};
