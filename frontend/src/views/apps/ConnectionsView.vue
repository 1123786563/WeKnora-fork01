<template>
  <div class="connections-view">
    <div class="connections-view__header">
      <div class="connections-view__heading">
        <h2 class="connections-view__title">{{ t('apps.connections.title') }}</h2>
        <p class="connections-view__desc">{{ t('apps.connections.description') }}</p>
      </div>
      <t-button
        variant="outline"
        :disabled="loading"
        :aria-label="t('apps.connections.refresh')"
        @click="load"
      >
        <template #icon><t-icon name="refresh" /></template>
        {{ t('apps.connections.refresh') }}
      </t-button>
    </div>

    <t-alert v-if="loadError" theme="error" :message="t('apps.connections.loadFailed')" class="connections-view__error" />

    <t-alert
      v-if="!canManage"
      theme="info"
      :message="t('apps.connections.memberCannotManage')"
      class="connections-view__error"
    />

    <t-table
      row-key="id"
      :data="connections"
      :columns="columns"
      :loading="loading"
      :empty="t('apps.connections.empty')"
      hover
    >
      <template #id="{ row }">
        <span :title="row.id" class="connections-view__mono">{{ shortId(row.id) }}</span>
      </template>
      <template #kind="{ row }">
        <t-tag :theme="row.kind === 'space' ? 'primary' : 'default'" size="small">
          {{ kindLabel(row.kind) }}
        </t-tag>
      </template>
      <template #owner="{ row }">
        <span :title="accountTitle(row)">{{ accountLabel(row) }}</span>
      </template>
      <template #state="{ row }">
        <t-tag :theme="stateTheme(row.state)" size="small">{{ stateLabel(row.state) }}</t-tag>
      </template>
      <template #ops="{ row }">
        <div class="connections-view__ops">
          <t-button
            v-if="canManage && row.state === 'active'"
            size="small"
            variant="text"
            :aria-label="t('apps.connections.startAuthorization')"
            @click="startAuthorization(row)"
          >
            {{ t('apps.connections.startAuthorization') }}
          </t-button>
          <t-popconfirm
            v-if="canManage && row.state === 'active'"
            :content="t('apps.connections.revokeConfirmContent')"
            :confirm-btn="{ content: t('apps.connections.revoke'), theme: 'danger', loading: revokingId === row.id }"
            :cancel-btn="{ content: t('apps.common.cancel'), theme: 'default' }"
            @confirm="revoke(row)"
          >
            <t-button
              size="small"
              variant="text"
              theme="danger"
              :disabled="revokingId !== ''"
              :aria-label="t('apps.connections.revoke')"
            >
              {{ t('apps.connections.revoke') }}
            </t-button>
          </t-popconfirm>
          <span v-if="row.state === 'revoked'" class="connections-view__cleanup-note">
            {{ t('apps.connections.remoteCleanupNote') }}
          </span>
        </div>
      </template>
    </t-table>
  </div>
</template>

<script setup lang="ts">
// ConnectionsView — the workspace app-connection list (/platform/apps/
// connections): personal vs space connections with their account ownership,
// the correlate-able authorization entry point, and revocation.
//
// Disconnect distinguishes LOCAL success (the server response confirms the
// connection row is revoked — that transaction is the authorization
// authority) from REMOTE cleanup still pending (the upstream token and
// external connection are removed asynchronously, which no DTO observes
// today; the note states exactly that, nothing more).
//
// Space-epoch discipline: switching spaces aborts in-flight requests, clears
// the list, and late responses are dropped by epoch.
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAuthStore } from '@/stores/auth'
import {
  beginAuthorizationAttempt,
  listConnections,
  revokeConnection,
  type AppConnectionView,
} from '@/api/appConnectors'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const activeTenantId = computed(() => String(authStore.currentTenantId ?? ''))

// Presentational role gate mirroring the server's connection-management
// write gate (owner/admin). The server gate stays authoritative — hiding
// these buttons for other roles is not authorization.
const currentRole = computed(() => String(authStore.currentTenantRole || ''))
const canManage = computed(() => currentRole.value === 'owner' || currentRole.value === 'admin')

const connections = ref<AppConnectionView[]>([])
const loading = ref(false)
const loadError = ref(false)
const revokingId = ref('')

// --- space epoch ---------------------------------------------------------
let epoch = 0
let inFlight: AbortController | null = null

const cancelInFlight = () => {
  if (inFlight) {
    inFlight.abort()
    inFlight = null
  }
}

const isAbortError = (e: unknown): boolean => {
  const err = e as { code?: string; name?: string }
  return err?.code === 'ERR_CANCELED' || err?.name === 'CanceledError' || err?.name === 'AbortError'
}

const load = async () => {
  const tenantId = activeTenantId.value
  if (!tenantId) return
  cancelInFlight()
  const run = ++epoch
  inFlight = new AbortController()
  loading.value = true
  loadError.value = false
  try {
    const rows = await listConnections({ signal: inFlight.signal })
    if (run !== epoch) return // late response from a previous space: drop
    connections.value = rows
  } catch (e) {
    if (run !== epoch || isAbortError(e)) return
    loadError.value = true
    connections.value = []
  } finally {
    if (run === epoch) {
      loading.value = false
      inFlight = null
    }
  }
}

watch(
  activeTenantId,
  (id) => {
    cancelInFlight()
    epoch += 1
    connections.value = []
    loading.value = false
    loadError.value = false
    revokingId.value = ''
    if (id) load()
  },
  { immediate: true },
)

onUnmounted(cancelInFlight)
// -------------------------------------------------------------------------

const shortId = (id: string): string => (id && id.length > 14 ? id.slice(0, 14) + '…' : id || '—')

const kindLabel = (kind: string): string => {
  if (kind === 'personal') return t('apps.connections.kindPersonal')
  if (kind === 'space') return t('apps.connections.kindSpace')
  return kind
}

// Account ownership: a space connection is workspace-shared (no personal
// account); a personal connection names its owning account. Only the
// server-provided owner id is used — never a runtime address, secret
// reference or internal alias (those do not exist on this DTO).
const accountLabel = (row: AppConnectionView): string => {
  if (row.kind === 'space') return t('apps.connections.accountSpace')
  if (row.owner_id) return shortId(row.owner_id)
  return t('apps.connections.accountUnknown')
}

const accountTitle = (row: AppConnectionView): string =>
  row.kind === 'space' ? t('apps.connections.accountSpace') : row.owner_id || ''

const stateTheme = (state: string): 'success' | 'danger' | 'default' => {
  if (state === 'active') return 'success'
  if (state === 'revoked') return 'danger'
  return 'default'
}

const stateLabel = (state: string): string => {
  if (state === 'active') return t('apps.common.stateActive')
  if (state === 'revoked') return t('apps.common.stateRevoked')
  return t('apps.common.stateOther', { state })
}

// Start the correlate-able authorization attempt (T13) and jump to the
// polling view. T13-F-3: no authorization URL exists on this path — the
// polling view carries guidance text, never a fabricated link.
const startAuthorization = async (row: AppConnectionView) => {
  try {
    const attempt = await beginAuthorizationAttempt(row.id)
    if (attempt.attempt_id) {
      router.push('/platform/apps/authorization/' + encodeURIComponent(attempt.attempt_id))
      return
    }
    MessagePlugin.error(t('apps.connections.startAuthorizationFailed'))
  } catch (e) {
    MessagePlugin.error(errorMessage(e) || t('apps.connections.startAuthorizationFailed'))
  }
}

const errorMessage = (e: unknown): string => {
  const err = e as { message?: string }
  return typeof err?.message === 'string' ? err.message : ''
}

// Revoke with the best-known authorization version. The list DTO does not
// expose the live auth_version (T15 report concern T15-C-1): connections
// start at version 1 and stay there while active, so 1 is the honest
// best-known value; a stale guess can only produce a safe 409 (the server's
// CAS refuses), which we surface and follow with a re-read — never a
// fabricated success.
const revoke = async (row: AppConnectionView) => {
  revokingId.value = row.id
  try {
    await revokeConnection(row.id, 1)
    // 200 = the LOCAL revocation committed (authority). Remote cleanup is
    // asynchronous and unobservable from this DTO — say exactly that.
    MessagePlugin.success(t('apps.connections.revokeSuccess'))
    await load()
  } catch (e) {
    const err = e as { status?: number; error?: { code?: string } }
    const code = err?.error?.code || ''
    if (err?.status === 409 || code === 'VERSION_CONFLICT') {
      MessagePlugin.warning(t('apps.connections.revokeConflict'))
      await load()
    } else {
      MessagePlugin.error(errorMessage(e) || t('apps.connections.revokeFailed'))
    }
  } finally {
    revokingId.value = ''
  }
}

const columns = computed(() => [
  { colKey: 'id', title: t('apps.connections.colId'), width: 170 },
  { colKey: 'kind', title: t('apps.connections.colKind'), width: 100 },
  { colKey: 'owner', title: t('apps.connections.colAccount'), ellipsis: true },
  { colKey: 'state', title: t('apps.connections.colState'), width: 110 },
  { colKey: 'ops', title: t('apps.connections.colActions'), width: 260 },
])
</script>

<style lang="less" scoped>
.connections-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  height: 100%;
  box-sizing: border-box;
}

.connections-view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.connections-view__title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
}

.connections-view__desc {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  max-width: 640px;
}

.connections-view__mono {
  font-family: var(--td-font-family-code, monospace);
}

.connections-view__ops {
  display: flex;
  align-items: center;
  gap: 4px;
}

.connections-view__cleanup-note {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
}

.connections-view__error {
  margin-top: 0;
}
</style>
