<template>
  <div class="authorization-view">
    <div class="authorization-view__header">
      <div class="authorization-view__heading">
        <h2 class="authorization-view__title">{{ t('apps.authorization.title') }}</h2>
        <p class="authorization-view__desc">{{ t('apps.authorization.description') }}</p>
      </div>
      <t-button
        variant="outline"
        :disabled="loading"
        :aria-label="t('apps.authorization.refresh')"
        @click="pollOnce"
      >
        <template #icon><t-icon name="refresh" /></template>
        {{ t('apps.authorization.refresh') }}
      </t-button>
    </div>

    <t-alert v-if="loadError" theme="error" :message="t('apps.authorization.loadFailed')" class="authorization-view__alert" />

    <!-- T13-F-3 (binding): this deployment's authorization-attempt endpoints
         return NO authorization URL — only a status to poll. The UI must not
         fabricate or link one, so this card states the control-plane
         guidance instead of any redirect. -->
    <t-alert theme="info" :message="t('apps.authorization.noUrlGuidance')" class="authorization-view__alert" />

    <div class="authorization-view__card" role="status" :aria-live="polled ? 'polite' : 'off'">
      <t-descriptions :column="1" bordered size="medium">
        <t-descriptions-item :label="t('apps.authorization.attemptLabel')">
          <span class="authorization-view__mono">{{ attemptId }}</span>
        </t-descriptions-item>
        <t-descriptions-item :label="t('apps.authorization.connectionLabel')">
          <span v-if="attempt?.connection_id" :title="attempt.connection_id" class="authorization-view__mono">
            {{ shortId(attempt.connection_id) }}
          </span>
          <span v-else>—</span>
        </t-descriptions-item>
        <t-descriptions-item :label="t('apps.authorization.statusLabel')">
          <t-tag :theme="statusTheme" size="small">{{ statusLabel }}</t-tag>
        </t-descriptions-item>
        <t-descriptions-item :label="t('apps.authorization.expiresLabel')">
          {{ formatTime(attempt?.expires_at) }}
        </t-descriptions-item>
      </t-descriptions>

      <p v-if="polling" class="authorization-view__hint">{{ t('apps.authorization.pollingHint') }}</p>
      <p v-else-if="succeeded" class="authorization-view__hint authorization-view__hint--ok">
        {{ t('apps.authorization.completedHint') }}
      </p>
    </div>

    <t-button variant="text" :aria-label="t('apps.authorization.back')" @click="goBack">
      <template #icon><t-icon name="arrow-left" /></template>
      {{ t('apps.authorization.back') }}
    </t-button>
  </div>
</template>

<script setup lang="ts">
// AuthorizationView — completion polling for one correlate-able open-
// connector authorization attempt (/platform/apps/authorization/:id).
//
// "授权完成轮询本地 attempt": the page polls ONLY the local attempt status
// endpoint (tenant+actor scoped). Per T13-F-3 the backend never returns an
// authorization URL on this path, so the UI shows guidance text and never a
// fabricated or linked URL.
//
// Space-epoch discipline: switching spaces (or leaving the page) cancels the
// in-flight poll and the polling timer; late responses are dropped by epoch.
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { getAuthorizationAttempt, type OCAuthorizationAttemptView } from '@/api/appConnectors'

const POLL_INTERVAL_MS = 3000
const POLL_MAX_INTERVAL_MS = 30000
// Consecutive poll failures double the next delay (reset on success), so a
// degraded backend backs off instead of being hammered every 3s (T15 QF-3).
let pollFailures = 0

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const activeTenantId = computed(() => String(authStore.currentTenantId ?? ''))

const attemptId = computed(() => String(route.params.id || ''))
const attempt = ref<OCAuthorizationAttemptView | null>(null)
const loading = ref(false)
const loadError = ref(false)
const polled = ref(false)

// Statuses still moving towards completion keep polling; terminal statuses
// stop (active = authorized, failed/expired/revoked = ended, anything else
// is displayed verbatim — never guessed into a friendly label).
const pollingStatuses = new Set(['pending', 'authorizing', 'verifying'])
const terminalOkStatuses = new Set(['active'])

const polling = computed(() => pollingStatuses.has(attempt.value?.status || ''))
const succeeded = computed(() => terminalOkStatuses.has(attempt.value?.status || ''))

const statusTheme = computed<'success' | 'danger' | 'warning' | 'default'>(() => {
  const status = attempt.value?.status || ''
  if (status === 'active') return 'success'
  if (status === 'failed' || status === 'expired' || status === 'revoked') return 'danger'
  if (pollingStatuses.has(status)) return 'warning'
  return 'default'
})

const statusLabel = computed(() => {
  const status = attempt.value?.status || ''
  const key = 'apps.authorization.status.' + status
  const label = t(key)
  return label === key ? t('apps.authorization.status.other', { state: status }) : label
})

// --- space epoch + polling ------------------------------------------------
let epoch = 0
let inFlight: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | null = null

const cancelAll = () => {
  if (timer) {
    clearTimeout(timer)
    timer = null
  }
  if (inFlight) {
    inFlight.abort()
    inFlight = null
  }
  pollFailures = 0
}

const isAbortError = (e: unknown): boolean => {
  const err = e as { code?: string; name?: string }
  return err?.code === 'ERR_CANCELED' || err?.name === 'CanceledError' || err?.name === 'AbortError'
}

const pollOnce = async () => {
  const tenantId = activeTenantId.value
  const id = attemptId.value
  if (!tenantId || !id) return
  if (inFlight) inFlight.abort()
  const run = ++epoch
  inFlight = new AbortController()
  loading.value = true
  loadError.value = false
  try {
    const view = await getAuthorizationAttempt(id, { signal: inFlight.signal })
    if (run !== epoch) return // late response from a previous space: drop
    attempt.value = view
    polled.value = true
    pollFailures = 0
    scheduleNext()
  } catch (e) {
    if (run !== epoch || isAbortError(e)) return
    loadError.value = true
    polled.value = true
    // A transient error no longer kills polling permanently: back off and
    // keep trying while the attempt is still in a polling state (T15 QF-3).
    scheduleNext()
  } finally {
    if (run === epoch) {
      loading.value = false
      inFlight = null
    }
  }
}

const scheduleNext = () => {
  if (timer) clearTimeout(timer)
  if (!polling.value) return // terminal: stop polling, keep the last state
  // expires_at stop: the backend GET does not lazily expire rows, so an
  // attempt stuck pending past its TTL must not poll for the page's whole
  // lifetime (T15 QF-3). The last state stays rendered.
  const expires = attempt.value?.expires_at
  const expiresMs = expires ? Date.parse(expires) : NaN
  if (!Number.isNaN(expiresMs) && expiresMs <= Date.now()) return
  const delay = Math.min(POLL_INTERVAL_MS * 2 ** pollFailures, POLL_MAX_INTERVAL_MS)
  timer = setTimeout(() => {
    timer = null
    pollOnce()
  }, delay)
}

watch(
  [activeTenantId, attemptId],
  ([tenantId, id]) => {
    cancelAll()
    epoch += 1
    attempt.value = null
    loading.value = false
    loadError.value = false
    polled.value = false
    if (tenantId && id) pollOnce()
  },
  { immediate: true },
)

onUnmounted(cancelAll)
// ---------------------------------------------------------------------------

const shortId = (id: string): string => (id && id.length > 14 ? id.slice(0, 14) + '…' : id || '—')

const formatTime = (value?: string): string => {
  if (!value) return '—'
  try {
    return new Date(value).toLocaleString()
  } catch {
    return value
  }
}

const goBack = () => {
  router.push('/platform/apps/connections')
}
</script>

<style lang="less" scoped>
.authorization-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  height: 100%;
  box-sizing: border-box;
  align-items: flex-start;
}

.authorization-view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}

.authorization-view__title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
}

.authorization-view__desc {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  max-width: 640px;
}

.authorization-view__card {
  width: 100%;
  max-width: 640px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.authorization-view__mono {
  font-family: var(--td-font-family-code, monospace);
}

.authorization-view__hint {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
}

.authorization-view__hint--ok {
  color: var(--td-success-color);
}

.authorization-view__alert {
  width: 100%;
  max-width: 640px;
}
</style>
