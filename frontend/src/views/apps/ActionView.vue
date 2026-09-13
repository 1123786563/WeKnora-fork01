<template>
  <div class="action-view">
    <div class="action-view__header">
      <div class="action-view__heading">
        <h2 class="action-view__title">{{ t('apps.actions.title') }}</h2>
        <p class="action-view__desc">{{ t('apps.actions.description') }}</p>
      </div>
      <t-button
        variant="outline"
        :disabled="loading"
        :aria-label="t('apps.actions.refresh')"
        @click="reload"
      >
        <template #icon><t-icon name="refresh" /></template>
        {{ t('apps.actions.refresh') }}
      </t-button>
    </div>

    <t-alert v-if="loadError" theme="error" :message="notFound ? t('apps.actions.notFound') : t('apps.actions.loadFailed')" class="action-view__alert" />

    <template v-if="action">
      <!-- The FROZEN approval snapshot, exactly as the server persisted it:
           account (display connection name), target (server-derived per
           T06-F-08/T13), args content and the digest/fence an approval must
           echo. No runtime address, secret reference or internal alias exists
           on this DTO, and none is rendered. -->
      <div class="action-view__card">
        <t-descriptions :column="1" bordered size="medium">
          <t-descriptions-item :label="t('apps.actions.accountLabel')">
            <span class="action-view__mono">{{ action.connection_name || '—' }}</span>
          </t-descriptions-item>
          <t-descriptions-item :label="t('apps.actions.targetLabel')">
            <span class="action-view__mono">{{ action.target || '—' }}</span>
          </t-descriptions-item>
          <t-descriptions-item :label="t('apps.actions.riskLabel')">
            <t-tag v-if="riskLabel" :theme="riskTheme" size="small">{{ riskLabel }}</t-tag>
            <t-tooltip v-else :content="t('apps.actions.riskUnknownHint')">
              <span class="action-view__risk-missing">—</span>
            </t-tooltip>
          </t-descriptions-item>
          <t-descriptions-item :label="t('apps.actions.stateLabel')">
            <t-tag :theme="stateTheme" size="small">{{ stateLabel }}</t-tag>
          </t-descriptions-item>
          <t-descriptions-item :label="t('apps.actions.digestLabel')">
            <span class="action-view__mono" :title="action.digest">{{ shortText(action.digest) }}</span>
          </t-descriptions-item>
          <t-descriptions-item :label="t('apps.actions.fenceLabel')">
            <span class="action-view__mono">{{ expectedVersion }}</span>
          </t-descriptions-item>
          <t-descriptions-item :label="t('apps.actions.argsLabel')">
            <pre class="action-view__args" tabindex="0">{{ prettyArgs }}</pre>
          </t-descriptions-item>
        </t-descriptions>
      </div>

      <!-- Approval controls: EXACTLY per the plan sketch (approve gated by
           lifecycle+permission, execute gated the same way with the
           submitting spinner, unknown shows the pending-check note). The
           retry control is never offered — retry is always false in phase
           one, so there is NO resend button under any state. -->
      <div class="action-view__controls">
        <t-button v-if="controls.approve" :loading="approving" @click="approve">{{ t('apps.actions.approve') }}</t-button>
        <t-button v-if="controls.execute" :loading="submitting" @click="execute">{{ t('apps.actions.execute') }}</t-button>
        <p v-if="action.state === 'unknown'">{{ t('apps.actions.unknown') }}</p>
      </div>
      <p v-if="action.state === 'unknown'" class="action-view__hint">{{ t('apps.actions.noResendHint') }}</p>
      <t-alert
        v-if="!canDrive"
        theme="info"
        :message="t('apps.actions.memberCannotApprove')"
        class="action-view__alert"
      />
    </template>
  </div>
</template>

<script setup lang="ts">
// ActionView — the frozen external-action approval surface (/platform/apps/
// actions/:id). It renders the server-persisted snapshot (account, target,
// args, digest, fence) and drives approve/execute through the action
// pipeline. Both operations RE-READ the action from the server after the
// response — no optimistic success is ever rendered from a local guess.
//
// State mapping is actionControls() (plan sketch, verbatim): approve needs
// awaiting_approval AND the caller's permission; execute needs authorized
// AND permission; retry is ALWAYS false in phase one — unknown shows
// "结果待核对" and no resend is offered.
//
// Space-epoch discipline: switching spaces aborts in-flight requests, drops
// the loaded action, and late responses are dropped by epoch.
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useAuthStore } from '@/stores/auth'
import {
  approveAction,
  executeAction,
  getAction,
  type AppActionDetailView,
} from '@/api/appConnectors'
import { actionControls, type ActionViewModel } from './actionState'

const { t } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const activeTenantId = computed(() => String(authStore.currentTenantId ?? ''))

const actionId = computed(() => String(route.params.id || ''))

const detail = ref<AppActionDetailView | null>(null)
const loading = ref(false)
const loadError = ref(false)
const notFound = ref(false)

const action = computed(() => detail.value?.action ?? null)
const expectedVersion = computed(() => detail.value?.expected_version ?? 0)

// Presentational permission mirroring the server's action write gate
// (CanDriveActionWrites: owner/admin). The server gate is authoritative.
const currentRole = computed(() => String(authStore.currentTenantRole || ''))
const canDrive = computed(() => currentRole.value === 'owner' || currentRole.value === 'admin')

// The view model feeds actionControls: lifecycle state from the frozen
// snapshot plus the caller's permission flags.
const viewModel = computed<ActionViewModel | null>(() =>
  action.value
    ? {
        id: action.value.id,
        state: action.value.state,
        digest: action.value.digest,
        canApprove: canDrive.value,
        canExecute: canDrive.value,
      }
    : null,
)

const controls = computed(() =>
  viewModel.value ? actionControls(viewModel.value) : { approve: false, execute: false, retry: false },
)

const approving = ref(false)
const submitting = ref(false)

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

const applyDetail = (value: AppActionDetailView | null) => {
  detail.value = value
  notFound.value = false
}

const reload = async () => {
  const tenantId = activeTenantId.value
  const id = actionId.value
  if (!tenantId || !id) return
  cancelInFlight()
  const run = ++epoch
  inFlight = new AbortController()
  loading.value = true
  loadError.value = false
  notFound.value = false
  try {
    const value = await getAction(id, { signal: inFlight.signal })
    if (run !== epoch) return // late response from a previous space: drop
    applyDetail(value)
  } catch (e) {
    if (run !== epoch || isAbortError(e)) return
    const err = e as { status?: number }
    notFound.value = err?.status === 404
    loadError.value = true
    detail.value = null
  } finally {
    if (run === epoch) {
      loading.value = false
      inFlight = null
    }
  }
}

watch(
  [activeTenantId, actionId],
  ([tenantId, id]) => {
    cancelInFlight()
    epoch += 1
    detail.value = null
    loading.value = false
    loadError.value = false
    notFound.value = false
    approving.value = false
    submitting.value = false
    if (tenantId && id) reload()
  },
  { immediate: true },
)

onUnmounted(cancelInFlight)
// -------------------------------------------------------------------------

// Approve binds the CURRENT snapshot: digest + fence echo the server state.
// After the server answers, the action is RE-READ (twice the truth: the POST
// response already carries the new snapshot, and the explicit re-read rules
// out any drift) — success is never assumed from a 200 alone.
const approve = async () => {
  if (!action.value || !controls.value.approve) return
  const run = epoch
  approving.value = true
  try {
    const answered = await approveAction(action.value.id, action.value.digest, expectedVersion.value)
    if (run !== epoch) return // late response from a previous space: drop
    if (answered) applyDetail(answered)
    await reload()
  } catch (e) {
    if (run !== epoch) return
    MessagePlugin.error(errorMessage(e) || t('apps.actions.approveFailed'))
    await reload()
  } finally {
    approving.value = false
  }
}

// Execute consumes the approval and dispatches. An unobservable provider
// outcome parks the action in unknown — displayed as "结果待核对" with no
// resend (phase one offers no retry at all).
const execute = async () => {
  if (!action.value || !controls.value.execute) return
  const run = epoch
  submitting.value = true
  try {
    const answered = await executeAction(action.value.id)
    if (run !== epoch) return // late response from a previous space: drop
    if (answered) applyDetail(answered)
    await reload()
  } catch (e) {
    if (run !== epoch) return
    MessagePlugin.error(errorMessage(e) || t('apps.actions.executeFailed'))
    await reload()
  } finally {
    submitting.value = false
  }
}

const errorMessage = (e: unknown): string => {
  const err = e as { message?: string }
  return typeof err?.message === 'string' ? err.message : ''
}

// --- display helpers ------------------------------------------------------
// The frozen risk from the persisted snapshot (R18 / T15-C-2): displayed
// verbatim with the same category coloring the catalog uses. An empty
// payload (a backend predating the field) degrades to an honest em-dash
// with a hint — never a guess from any other source.
const riskValue = computed(() => String(action.value?.risk || ''))
const riskTheme = computed<'success' | 'warning' | 'danger' | 'default'>(() => {
  if (riskValue.value === 'read') return 'success'
  if (riskValue.value === 'write') return 'warning'
  if (riskValue.value === 'send' || riskValue.value === 'delete') return 'danger'
  return 'default'
})
const riskLabel = computed(() => {
  if (!riskValue.value) return ''
  const key = 'apps.risk.' + riskValue.value
  const label = t(key)
  return label === key ? riskValue.value : label
})

const stateTheme = computed<'success' | 'warning' | 'danger' | 'default'>(() => {
  const state = action.value?.state || ''
  if (state === 'succeeded') return 'success'
  if (state === 'failed') return 'danger'
  if (state === 'awaiting_approval') return 'warning'
  if (state === 'unknown') return 'warning'
  return 'default'
})

const stateLabel = computed(() => {
  const state = action.value?.state || ''
  if (state === 'unknown') return t('apps.actions.unknown')
  const key = 'apps.actions.state.' + state
  const label = t(key)
  return label === key ? t('apps.actions.state.other', { state }) : label
})

const prettyArgs = computed(() => {
  const raw = action.value?.content || ''
  if (!raw) return '—'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
})

const shortText = (value: string): string =>
  value && value.length > 16 ? value.slice(0, 16) + '…' : value || '—'
</script>

<style lang="less" scoped>
.action-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  height: 100%;
  box-sizing: border-box;
  align-items: flex-start;
}

.action-view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  width: 100%;
}

.action-view__title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
}

.action-view__desc {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  max-width: 640px;
}

.action-view__card {
  width: 100%;
  max-width: 720px;
}

.action-view__mono {
  font-family: var(--td-font-family-code, monospace);
}

.action-view__risk-missing {
  color: var(--td-text-color-placeholder);
}

.action-view__args {
  margin: 0;
  max-height: 280px;
  overflow: auto;
  background: var(--td-bg-color-page);
  border-radius: var(--td-radius-medium, 6px);
  padding: 10px 12px;
  font-family: var(--td-font-family-code, monospace);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
}

.action-view__controls {
  display: flex;
  align-items: center;
  gap: 12px;

  p {
    margin: 0;
    color: var(--td-warning-color);
    font-size: 13px;
  }
}

.action-view__hint {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 12px;
}

.action-view__alert {
  width: 100%;
  max-width: 720px;
}
</style>
