<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import {
  listDataSources,
  deleteDataSource,
  getDataSourceDocumentsCount,
  cancelSyncLog,
  triggerSync,
  pauseDataSource,
  resumeDataSource,
  type DataSource,
} from '@/api/datasource'
import { humanizeCron, relativeTime } from '@/utils/cronHumanize'
import DataSourceEditorDialog from './DataSourceEditorDialog.vue'
import DataSourceSyncLogs from './DataSourceSyncLogs.vue'
import DataSourceTypeIcon from './DataSourceTypeIcon.vue'
import { useAuthStore } from '@/stores/auth'

const props = defineProps<{ kbId: string }>()
const emit = defineEmits<{ (e: 'count', value: number): void }>()
const { t } = useI18n()
const authStore = useAuthStore()

// 后端 /datasource 的 list/logs 是 Viewer+，但所有写操作（POST/PUT/DELETE
// 以及 sync/pause/resume/validate）都是 Admin+。低权限用户保留只读视图，
// 增删改和触发同步全部隐藏，而不是按下去再撞 403。
const canManageDataSource = computed(() => authStore.hasRole('admin'))

const dataSources = ref<DataSource[]>([])
const loading = ref(false)
const editorVisible = ref(false)
const editingDs = ref<DataSource | null>(null)
const logsVisible = ref(false)
const logsDsId = ref('')
const logsDsName = ref('')
const pollTimer = ref<number | null>(null)

// 双选删除面板（SP2-a §4.3，与 React Task 10 语义等价）：进面板拉取该源
// 已同步文档数，默认不勾选 = 保留文档（既有承诺）；勾选后正文切换为红色
// 不可恢复警示，确认按钮变为"删除数据源及文档"，DELETE 携带
// purge_documents=true。串号守卫防止慢返回的 count 落进另一个源的面板。
const deleteVisible = ref(false)
const deleteTarget = ref<DataSource | null>(null)
const deletePurge = ref(false)
const deleteCount = ref<number | null>(null)
const deleteCountLoading = ref(false)
const deleteSubmitting = ref(false)
let deleteCountRequest = 0

// 运行中同步的协作取消（SP2-a §3.3，Task 6 cancel API 的 Vue 入口）：
// 仅 running 态显示入口；请求成功不翻转行，靠既有 3s 轮询收敛为已取消。
const cancelBusyId = ref('')

const deletePurgeLabelText = computed(() =>
  deleteCount.value !== null
    ? t('datasource.deletePanelPurgeLabel', { count: deleteCount.value })
    : t('datasource.deletePanelPurgeLabelUnknown'),
)
const deletePurgeWarningText = computed(() =>
  deleteCount.value !== null
    ? t('datasource.deletePanelPurgeWarning', { count: deleteCount.value })
    : t('datasource.deletePanelPurgeWarningUnknown'),
)

function stopPolling() {
  if (pollTimer.value !== null) {
    window.clearTimeout(pollTimer.value)
    pollTimer.value = null
  }
}

function schedulePolling() {
  stopPolling()
  pollTimer.value = window.setTimeout(() => {
    loadList(true)
  }, 3000)
}

async function loadList(silent = false) {
  if (!silent) loading.value = true
  try {
    const res = await listDataSources(props.kbId)
    dataSources.value = res?.data || res || []
    emit('count', dataSources.value.length)

    const hasRunningSync = dataSources.value.some(ds => ds.latest_sync_log?.status === 'running')
    if (hasRunningSync) {
      schedulePolling()
    } else {
      stopPolling()
    }
  } catch (e: any) {
    console.error(e)
  } finally {
    if (!silent) loading.value = false
  }
}

function openCreate() {
  editingDs.value = null
  editorVisible.value = true
}

function openEdit(ds: DataSource) {
  editingDs.value = ds
  editorVisible.value = true
}

function openLogs(ds: DataSource) {
  logsDsId.value = ds.id
  logsDsName.value = ds.name
  logsVisible.value = true
}

// 打开双选面板：复选框复位为不勾（保留文档承诺），文档数在面板背后
// 拉取；拉取失败时面板退化为无数字文案，绝不显示误导性的 0。
async function openDeletePanel(ds: DataSource) {
  deleteTarget.value = ds
  deletePurge.value = false
  deleteCount.value = null
  deleteCountLoading.value = true
  deleteVisible.value = true
  const request = ++deleteCountRequest
  try {
    const count = await getDataSourceDocumentsCount(ds.id)
    if (deleteCountRequest === request) deleteCount.value = count
  } catch {
    // count 不可用 —— 面板继续使用无数字的兜底文案
  } finally {
    if (deleteCountRequest === request) deleteCountLoading.value = false
  }
}

// 确认执行双选删除：勾选状态决定 DELETE 是否携带 purge_documents=true；
// 成功提示区分"已删除"与"连文档一起已删除"。
async function confirmDelete() {
  const ds = deleteTarget.value
  if (!ds || deleteSubmitting.value) return
  const purge = deletePurge.value
  deleteSubmitting.value = true
  try {
    await deleteDataSource(ds.id, purge)
    deleteVisible.value = false
    MessagePlugin.success(purge ? t('datasource.deleteSuccessPurged') : t('datasource.deleteSuccess'))
    await loadList()
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t('datasource.deleteFailed'))
  } finally {
    deleteSubmitting.value = false
  }
}

// 协作取消运行中的同步：POST 返回 202 cancel_requested，同步循环在下一个
// 检查点退出（cursor 保留，下次同步断点续传）。此处不翻转行状态。
async function handleCancelSync(ds: DataSource) {
  const log = ds.latest_sync_log
  if (!log || log.status !== 'running' || cancelBusyId.value) return
  cancelBusyId.value = ds.id
  try {
    await cancelSyncLog(ds.id, log.id)
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || String(e))
  } finally {
    cancelBusyId.value = ''
  }
}

async function handleSync(ds: DataSource) {
  try {
    await triggerSync(ds.id)
    MessagePlugin.success(t('datasource.syncTriggered'))
    await loadList(true)
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t('datasource.syncFailed'))
  }
}

async function handlePause(ds: DataSource) {
  try {
    await pauseDataSource(ds.id)
    MessagePlugin.success(t('datasource.paused'))
    loadList()
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t('datasource.pauseFailed'))
  }
}

async function handleResume(ds: DataSource) {
  try {
    await resumeDataSource(ds.id)
    MessagePlugin.success(t('datasource.resumed'))
    loadList()
  } catch (e: any) {
    MessagePlugin.error(e?.message || e?.error || t('datasource.resumeFailed'))
  }
}

function statusLabel(status: string) {
  return t(`datasource.status.${status}`)
}

function syncModeLabel(mode: string) {
  return t(`datasource.syncMode.${mode}`)
}

function connectorLabel(type: string) {
  return t(`datasource.connector.${type}`) || type
}

function scheduleLabel(cron: string) {
  return humanizeCron(cron, t)
}

function lastSyncTime(ds: DataSource) {
  return relativeTime(ds.last_sync_at, t)
}

function lastSyncFullTime(ds: DataSource) {
  if (!ds.last_sync_at) return ''
  return new Date(ds.last_sync_at).toLocaleString()
}

function syncResultPills(ds: DataSource) {
  const log = ds.latest_sync_log
  if (!log) return []
  const pills: { text: string; cls: string }[] = []
  if (log.items_created > 0) pills.push({ text: `+${log.items_created}`, cls: 'created' })
  if (log.items_updated > 0) pills.push({ text: `~${log.items_updated}`, cls: 'updated' })
  if (log.items_deleted > 0) pills.push({ text: `-${log.items_deleted}`, cls: 'deleted' })
  if (log.items_failed > 0) pills.push({ text: `${log.items_failed} ${t('datasource.logMetric.failed')}`, cls: 'failed' })
  if (log.items_skipped > 0) pills.push({ text: `${log.items_skipped} ${t('datasource.logMetric.skipped')}`, cls: 'skipped' })
  return pills
}

function lastSyncStatusLabel(ds: DataSource) {
  const log = ds.latest_sync_log
  if (!log) return '--'
  return t(`datasource.logStatus.${log.status}`)
}

function isSyncRunning(ds: DataSource) {
  return ds.latest_sync_log?.status === 'running'
}

function onEditorSaved() {
  editorVisible.value = false
  loadList()
}

onMounted(loadList)
onBeforeUnmount(stopPolling)
</script>

<template>
  <div class="ds-settings">
    <div class="section-header">
      <h2>{{ t('datasource.title') }}</h2>
      <p class="section-description">{{ t('datasource.description') }}</p>
    </div>

    <t-loading :loading="loading" size="small" class="ds-list-loading">
      <div
        v-if="!loading && dataSources.length === 0 && !canManageDataSource"
        class="empty-state"
      >
        <t-empty :description="t('datasource.empty')" />
      </div>

      <div v-else-if="!loading" class="ds-grid">
        <component
          :is="canManageDataSource ? 'button' : 'div'"
          v-for="ds in dataSources"
          :key="ds.id"
          :type="canManageDataSource ? 'button' : undefined"
          :class="['ds-card', `ds-card--${ds.type}`, { 'ds-card--clickable': canManageDataSource }]"
          @click="canManageDataSource ? openEdit(ds) : undefined"
        >
          <div class="ds-card__badge">
            <DataSourceTypeIcon :type="ds.type" variant="badge" />
          </div>
          <div class="ds-card__body">
            <div class="ds-card__header">
              <h3 class="ds-card__title" :title="ds.name">{{ ds.name }}</h3>
              <div class="ds-card__actions" @click.stop>
                <t-dropdown trigger="click" :min-column-width="140" attach="body">
                  <t-button
                    variant="text"
                    shape="square"
                    size="small"
                    class="ds-card__action-btn"
                    @click.stop
                  >
                    <template #icon><t-icon name="ellipsis" /></template>
                  </t-button>
                  <template #dropdown>
                    <t-dropdown-menu>
                      <t-dropdown-item v-if="canManageDataSource" @click="openEdit(ds)">
                        <t-icon name="edit" /> {{ t('datasource.edit') }}
                      </t-dropdown-item>
                      <t-dropdown-item
                        v-if="canManageDataSource"
                        :disabled="isSyncRunning(ds)"
                        @click="handleSync(ds)"
                      >
                        <t-icon name="refresh" :class="{ 'ds-icon-spin': isSyncRunning(ds) }" />
                        {{ isSyncRunning(ds) ? t('datasource.logStatus.running') : t('datasource.syncNow') }}
                      </t-dropdown-item>
                      <t-dropdown-item @click="openLogs(ds)">
                        <t-icon name="root-list" /> {{ t('datasource.logs') }}
                      </t-dropdown-item>
                      <t-dropdown-item
                        v-if="canManageDataSource && ds.status === 'active'"
                        @click="handlePause(ds)"
                      >
                        <t-icon name="pause-circle" /> {{ t('datasource.pause') }}
                      </t-dropdown-item>
                      <t-dropdown-item
                        v-else-if="canManageDataSource && ds.status === 'paused'"
                        @click="handleResume(ds)"
                      >
                        <t-icon name="play-circle" /> {{ t('datasource.resume') }}
                      </t-dropdown-item>
                      <t-dropdown-item
                        v-if="canManageDataSource && isSyncRunning(ds)"
                        :disabled="cancelBusyId === ds.id"
                        @click="handleCancelSync(ds)"
                      >
                        <t-icon name="stop-circle" :class="{ 'ds-icon-spin': cancelBusyId === ds.id }" />
                        {{ t('datasource.cancelSync') }}
                      </t-dropdown-item>
                      <t-dropdown-item
                        v-if="canManageDataSource"
                        theme="error"
                        class="ds-dropdown-delete-item"
                      >
                        <span class="ds-dropdown-delete-trigger" @click.stop="openDeletePanel(ds)">
                          <t-icon name="delete" />
                          <span>{{ t('datasource.delete') }}</span>
                        </span>
                      </t-dropdown-item>
                    </t-dropdown-menu>
                  </template>
                </t-dropdown>
              </div>
            </div>
            <p class="ds-card__subtitle">
              {{ connectorLabel(ds.type) }} · {{ syncModeLabel(ds.sync_mode) }}
              <span class="ds-card__sep">·</span>
              <span class="ds-card__status" :class="`ds-card__status--${ds.status}`">
                <span class="ds-status-dot" aria-hidden="true" />
                {{ statusLabel(ds.status) }}
              </span>
            </p>
            <p class="ds-card__detail">
              {{ scheduleLabel(ds.sync_schedule) }}
              <span class="ds-card__sep">·</span>
              <t-tooltip :content="lastSyncFullTime(ds)" :disabled="!lastSyncFullTime(ds)">
                <span>{{ lastSyncTime(ds) || '--' }}</span>
              </t-tooltip>
              <template v-if="ds.latest_sync_log">
                <span class="ds-card__sep">·</span>
                <span
                  class="ds-card__sync-result"
                  :class="`ds-card__sync-result--${ds.latest_sync_log.status}`"
                >
                  {{ lastSyncStatusLabel(ds) }}
                </span>
                <span
                  v-for="pill in syncResultPills(ds)"
                  :key="pill.cls"
                  class="ds-card__metric"
                >{{ pill.text }}</span>
              </template>
            </p>
            <div v-if="ds.error_message" class="ds-card__error">
              <t-icon name="error-circle-filled" size="14px" />
              <span>{{ ds.error_message }}</span>
            </div>
          </div>
        </component>

        <button
          v-if="canManageDataSource"
          type="button"
          class="ds-card ds-card--add"
          @click="openCreate"
        >
          <span class="ds-card--add__icon" aria-hidden="true">
            <t-icon name="add" />
          </span>
          <span class="ds-card--add__label">{{ t('datasource.add') }}</span>
        </button>
      </div>
    </t-loading>

    <DataSourceEditorDialog
      v-model:visible="editorVisible"
      :kb-id="kbId"
      :data-source="editingDs"
      @saved="onEditorSaved"
    />

    <DataSourceSyncLogs
      v-model:visible="logsVisible"
      :data-source-id="logsDsId"
      :data-source-name="logsDsName"
    />

    <!-- 双选删除面板：默认不勾选=保留已同步文档；勾选后正文切换为红色
         不可恢复警示，确认携带 purge_documents=true（SP2-a §4.3）。 -->
    <t-dialog
      v-model:visible="deleteVisible"
      :header="t('datasource.deletePanelTitle', { name: deleteTarget?.name ?? '' })"
      :confirm-btn="{
        content: deletePurge ? t('datasource.deleteAndPurge') : t('datasource.delete'),
        theme: 'danger',
        loading: deleteSubmitting,
      }"
      :cancel-btn="{ content: t('common.cancel'), disabled: deleteSubmitting }"
      :close-on-overlay-click="false"
      :width="440"
      @confirm="confirmDelete"
    >
      <div class="ds-delete-panel">
        <p
          class="ds-delete-panel__body"
          :class="{ 'ds-delete-panel__body--danger': deletePurge }"
        >
          {{ deletePurge ? deletePurgeWarningText : t('datasource.deletePanelKeep') }}
        </p>
        <p v-if="deleteCountLoading" class="ds-delete-panel__count">
          {{ t('common.loading') }}
        </p>
        <p v-else-if="deleteCount !== null" class="ds-delete-panel__count">
          {{ t('datasource.deletePanelCount', { count: deleteCount }) }}
        </p>
        <t-checkbox v-model="deletePurge" :disabled="deleteSubmitting">
          {{ deletePurgeLabelText }}
        </t-checkbox>
      </div>
    </t-dialog>
  </div>
</template>

<style scoped lang="less">
@import './datasource-surface.less';
.ds-settings {
  width: 100%;
}

.section-header {
  margin-bottom: 28px;

  h2 {
    font-size: 20px;
    font-weight: 600;
    color: var(--td-text-color-primary);
    margin: 0 0 8px 0;
  }

  .section-description {
    font-size: 14px;
    color: var(--td-text-color-secondary);
    margin: 0;
    line-height: 1.6;
  }
}

.ds-list-loading {
  min-height: 120px;
}

.empty-state {
  padding: 32px 0;
}

.ds-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;

  .ds-card--add {
    width: 100%;
    height: 100%;
  }
}

.ds-card {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 14px 16px;
  .ds-surface-card();
  text-align: left;
  font: inherit;
  color: inherit;
  min-width: 0;

  &--clickable {
    cursor: pointer;
    width: 100%;
    .ds-surface-card--interactive();
  }

  &--add {
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    min-height: 68px;
    border-style: dashed;
    background: transparent;
    color: var(--td-text-color-placeholder);
    cursor: pointer;
    width: 100%;

    &:hover,
    &:focus-visible {
      color: var(--td-brand-color);
      border-color: var(--td-brand-color);
      background: color-mix(in srgb, var(--td-brand-color) 6%, transparent);
      box-shadow: none;
    }

    &:focus-visible {
      outline: 2px solid var(--td-brand-color);
      outline-offset: 2px;
    }

    &__icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 32px;
      height: 32px;
      border-radius: 8px;
      background: color-mix(in srgb, var(--td-brand-color) 10%, transparent);
      color: var(--td-brand-color);
      font-size: 18px;
    }

    &__label {
      font-size: 13px;
      font-weight: 500;
      line-height: 1.4;
    }
  }

  &__badge {
    flex-shrink: 0;
    width: 36px;
    height: 36px;
    border-radius: 9px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin-top: 1px;
    font-size: 15px;
    font-weight: 600;
    letter-spacing: 0.02em;
    background: rgba(7, 192, 95, 0.12);
    color: #07c05f;
    overflow: hidden;
  }

  &--feishu .ds-card__badge,
  &--notion .ds-card__badge,
  &--yuque .ds-card__badge,
  &--ima .ds-card__badge,
  &--rss .ds-card__badge {
    background: var(--td-bg-color-container, #fff);
    box-shadow: inset 0 0 0 1px var(--td-component-stroke);
  }

  &__body {
    flex: 1;
    min-width: 0;
  }

  &__header {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }

  &__title {
    flex: 1;
    min-width: 0;
    margin: 0;
    font-size: 14px;
    font-weight: 600;
    line-height: 1.4;
    color: var(--td-text-color-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  &__subtitle {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px;
    margin: 2px 0 0;
    font-size: 12px;
    line-height: 1.5;
    color: var(--td-text-color-secondary);
    min-width: 0;
  }

  &__status {
    display: inline-flex;
    align-items: center;
    gap: 4px;

    &--active {
      color: var(--td-success-color);
    }

    &--paused {
      color: var(--td-warning-color);
    }

    &--error {
      color: var(--td-error-color);
    }
  }

  &__detail {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 4px;
    margin: 4px 0 0;
    font-size: 12px;
    line-height: 1.45;
    color: var(--td-text-color-placeholder);
    min-width: 0;
  }

  &__sync-result {
    font-weight: 500;
    color: var(--td-text-color-secondary);

    &--success {
      color: var(--td-success-color);
    }

    &--failed {
      color: var(--td-error-color);
    }

    &--running {
      color: var(--td-brand-color);
    }

    &--partial {
      color: var(--td-warning-color);
    }
  }

  &__metric {
    font-size: 11px;
    font-variant-numeric: tabular-nums;
    color: var(--td-text-color-disabled);
  }

  &__sep {
    color: var(--td-text-color-disabled);
    user-select: none;
  }

  &__error {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    margin-top: 8px;
    padding: 8px 10px;
    border-radius: 6px;
    background: var(--td-error-color-1);
    color: var(--td-error-color);
    font-size: 12px;
    line-height: 1.45;
    text-align: left;
  }

  &__actions {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    gap: 2px;
    margin-left: auto;
  }

  &__action-btn {
    flex-shrink: 0;
    padding: 2px;
    opacity: 0;
    color: var(--td-text-color-placeholder);
    transition: opacity 0.15s ease, color 0.15s ease;

    &:hover,
    &:focus-visible {
      background: var(--td-bg-color-secondarycontainer);
      color: var(--td-text-color-primary);
    }
  }

  &:hover .ds-card__action-btn,
  &:focus-within .ds-card__action-btn,
  &__actions:focus-within .ds-card__action-btn {
    opacity: 1;
  }
}

.ds-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}

.ds-icon-spin {
  animation: ds-spin 1s linear infinite;
}

@keyframes ds-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

:deep(.t-dropdown__item.ds-dropdown-delete-item) {
  border-top: 1px solid var(--td-component-stroke);
  margin-top: 4px;
  padding-top: 4px;

  .t-popup__reference {
    display: block;
    width: 100%;
  }
}

.ds-dropdown-delete-trigger {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  cursor: pointer;
  line-height: 22px;
}

.ds-delete-panel {
  display: grid;
  gap: 12px;
  font-size: 13px;

  &__body {
    margin: 0;
    line-height: 1.6;
    color: var(--td-text-color-primary);

    &--danger {
      font-weight: 500;
      color: var(--td-error-color);
    }
  }

  &__count {
    margin: 0;
    color: var(--td-text-color-secondary);
  }
}
</style>
