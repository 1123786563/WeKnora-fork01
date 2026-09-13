<template>
  <div class="apps-view">
    <div class="apps-view__header">
      <div class="apps-view__heading">
        <h2 class="apps-view__title">{{ t('apps.catalog.title') }}</h2>
        <p class="apps-view__desc">{{ t('apps.catalog.description') }}</p>
      </div>
      <t-button
        variant="outline"
        :disabled="loading"
        :aria-label="t('apps.catalog.refresh')"
        @click="load"
      >
        <template #icon><t-icon name="refresh" /></template>
        {{ t('apps.catalog.refresh') }}
      </t-button>
    </div>

    <t-alert v-if="loadError" theme="error" :message="t('apps.catalog.loadFailed')" class="apps-view__error" />

    <!-- Reviewed, tenant-reachable action catalog: published actions with
         their provider, risk and the permission scopes an authorization
         must cover. Everything shown here is server-derived review data —
         no runtime address, secret reference or internal alias exists on
         this DTO, and none is rendered. -->
    <section class="apps-view__section" :aria-label="t('apps.catalog.title')">
      <t-table
        row-key="action_id"
        :data="catalog"
        :columns="catalogColumns"
        :loading="loading"
        :empty="t('apps.catalog.empty')"
        hover
        :max-height="520"
      >
        <template #risk="{ row }">
          <t-tag :theme="riskTheme(row.risk)" size="small">{{ riskLabel(row.risk) }}</t-tag>
        </template>
        <template #required_scopes="{ row }">
          <span>{{ row.required_scopes?.length ? row.required_scopes.join(', ') : '—' }}</span>
        </template>
        <template #schema_digest="{ row }">
          <span :title="row.schema_digest">{{ shortDigest(row.schema_digest) }}</span>
        </template>
        <template #published="{ row }">
          <t-tag v-if="row.published" theme="success" size="small">{{ t('apps.catalog.published') }}</t-tag>
          <t-tag v-else theme="default" size="small">{{ t('apps.catalog.unpublished') }}</t-tag>
        </template>
      </t-table>
    </section>

    <!-- Installed app versions in this workspace (read-only projection). -->
    <section class="apps-view__section" :aria-label="t('apps.catalog.installedTitle')">
      <h3 class="apps-view__subtitle">{{ t('apps.catalog.installedTitle') }}</h3>
      <t-table
        row-key="id"
        :data="installations"
        :columns="installationColumns"
        :loading="loading"
        :empty="t('apps.catalog.installedEmpty')"
        hover
        :max-height="360"
      >
        <template #state="{ row }">
          <t-tag :theme="row.state === 'active' ? 'success' : 'default'" size="small">
            {{ installationStateLabel(row.state) }}
          </t-tag>
        </template>
        <template #scopes="{ row }">
          <span>{{ row.scopes?.length ? row.scopes.join(', ') : '—' }}</span>
        </template>
      </t-table>
    </section>
  </div>
</template>

<script setup lang="ts">
// AppsView — the tenant app directory (/platform/apps). It shows the T13
// reviewed action catalog (published actions + their permission scopes) and
// the installed app versions, all through tenant-scoped read endpoints.
//
// Space-epoch discipline: switching spaces aborts the in-flight requests,
// clears both lists immediately, and any late response is dropped by epoch
// — a stale space's data can never bleed into the freshly selected one.
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import {
  listInstallations,
  listOCCatalog,
  type AppInstallationView,
  type OCCatalogEntry,
} from '@/api/appConnectors'

const { t } = useI18n()
const authStore = useAuthStore()
const activeTenantId = computed(() => String(authStore.currentTenantId ?? ''))

const catalog = ref<OCCatalogEntry[]>([])
const installations = ref<AppInstallationView[]>([])
const loading = ref(false)
const loadError = ref(false)

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
  const signal = inFlight.signal
  loading.value = true
  loadError.value = false
  try {
    const [catalogRows, installationRows] = await Promise.all([
      listOCCatalog({ signal }),
      listInstallations({ signal }),
    ])
    if (run !== epoch) return // late response from a previous space: drop
    catalog.value = catalogRows
    installations.value = installationRows
  } catch (e) {
    if (run !== epoch || isAbortError(e)) return
    loadError.value = true
    catalog.value = []
    installations.value = []
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
    // Space switched (or hydrated): cancel old requests, clear the lists,
    // then reload for the new space only.
    cancelInFlight()
    epoch += 1
    catalog.value = []
    installations.value = []
    loading.value = false
    loadError.value = false
    if (id) load()
  },
  { immediate: true },
)

onUnmounted(cancelInFlight)
// -------------------------------------------------------------------------

const shortDigest = (digest: string): string =>
  digest && digest.length > 12 ? digest.slice(0, 12) + '…' : digest || '—'

const riskTheme = (risk: string): 'success' | 'warning' | 'danger' | 'default' => {
  if (risk === 'read') return 'success'
  if (risk === 'write') return 'warning'
  if (risk === 'send' || risk === 'delete') return 'danger'
  return 'default'
}

const riskLabel = (risk: string): string => {
  const key = 'apps.risk.' + risk
  const label = t(key)
  return label === key ? risk : label
}

const installationStateLabel = (state: string): string => {
  if (state === 'active') return t('apps.common.stateActive')
  if (state === 'disabled') return t('apps.common.stateDisabled')
  return t('apps.common.stateOther', { state })
}

const catalogColumns = computed(() => [
  { colKey: 'action_id', title: t('apps.catalog.colAction'), ellipsis: true },
  { colKey: 'app_id', title: t('apps.catalog.colApp'), ellipsis: true },
  { colKey: 'app_version', title: t('apps.catalog.colVersion'), width: 110 },
  { colKey: 'provider', title: t('apps.catalog.colProvider'), width: 120, ellipsis: true },
  { colKey: 'risk', title: t('apps.catalog.colRisk'), width: 100 },
  { colKey: 'required_scopes', title: t('apps.catalog.colPermissions'), ellipsis: true },
  { colKey: 'schema_digest', title: t('apps.catalog.colSchemaDigest'), width: 150 },
  { colKey: 'published', title: t('apps.catalog.colPublished'), width: 110 },
])

const installationColumns = computed(() => [
  { colKey: 'app_key', title: t('apps.catalog.colApp'), ellipsis: true },
  { colKey: 'version', title: t('apps.catalog.colVersion'), width: 110 },
  { colKey: 'state', title: t('apps.catalog.colState'), width: 110 },
  { colKey: 'scopes', title: t('apps.catalog.colScopes'), ellipsis: true },
])
</script>

<style lang="less" scoped>
.apps-view {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 20px;
  overflow-y: auto;
  height: 100%;
  box-sizing: border-box;
}

.apps-view__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.apps-view__title {
  margin: 0 0 4px;
  font-size: 20px;
  font-weight: 600;
}

.apps-view__desc {
  margin: 0;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  max-width: 640px;
}

.apps-view__section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.apps-view__subtitle {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
}

.apps-view__error {
  margin-top: 8px;
}
</style>
