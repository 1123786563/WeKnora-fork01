<template>
  <section v-if="run && isWaiting" class="agent-run-recovery" role="status" :aria-label="t('agentRunRecovery.title')">
    <div class="agent-run-recovery__header">
      <strong>{{ t('agentRunRecovery.title') }}</strong>
      <span class="agent-run-recovery__status">{{ statusLabel }}</span>
    </div>
    <p class="agent-run-recovery__reason">{{ run.wait_reason || t('agentRunRecovery.defaultReason') }}</p>
    <p v-if="toolName" class="agent-run-recovery__tool">{{ t('agentRunRecovery.tool', { name: toolName }) }}</p>
    <label v-if="run.status === 'waiting_user' && action === 'provide_result'" class="agent-run-recovery__result">
      {{ t('agentRunRecovery.resultLabel') }}
      <textarea v-model="resultText" :placeholder="t('agentRunRecovery.resultPlaceholder')" rows="3" />
    </label>
    <div class="agent-run-recovery__actions">
      <button type="button" :disabled="busy" @click="submit('retry')">{{ t('agentRunRecovery.retry') }}</button>
      <button type="button" :disabled="busy" @click="action === 'provide_result' ? submit('provide_result') : action = 'provide_result'">{{ t('agentRunRecovery.provideResult') }}</button>
      <button type="button" class="danger" :disabled="busy" @click="submit('terminate')">{{ t('agentRunRecovery.terminate') }}</button>
    </div>
    <p v-if="errorMessage" class="agent-run-recovery__error">{{ errorMessage }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { resolveAgentRunDecision, type RunView } from '@/api/chat/runs'

const props = defineProps<{ sessionId: string; run: RunView | null }>()
const emit = defineEmits<{ (event: 'updated', run: RunView): void; (event: 'refresh'): void }>()
const { t } = useI18n()
const busy = ref(false)
const errorMessage = ref('')
const resultText = ref('')
const action = ref<'retry' | 'provide_result'>('retry')

const isWaiting = computed(() => props.run?.status === 'waiting_user' || Boolean(props.run?.pending_id))
const statusLabel = computed(() => props.run?.status || '')
const toolName = computed(() => {
  const name = props.run?.wait_reason?.match(/tool[:=]\s*([^\s]+)/i)?.[1]
  return name || ''
})
watch(() => props.run?.run_id, () => { errorMessage.value = ''; resultText.value = ''; action.value = 'retry' })

async function submit(nextAction: 'retry' | 'provide_result' | 'terminate') {
  const run = props.run
  if (!run || busy.value || !run.pending_id) return
  busy.value = true
  errorMessage.value = ''
  try {
    const result = await resolveAgentRunDecision(props.sessionId, run.run_id, {
      pending_id: run.pending_id,
      decision_id: crypto.randomUUID(),
      expected_revision: run.revision,
      action: nextAction,
      ...(nextAction === 'provide_result' ? { result: { value: resultText.value } } : {}),
    })
    emit('updated', result)
    emit('refresh')
  } catch (error: any) {
    if (error?.$httpStatus === 409 || error?.status === 409) emit('refresh')
    errorMessage.value = error?.message || t('agentRunRecovery.failed')
  } finally { busy.value = false }
}
</script>

<style scoped>
.agent-run-recovery { margin: 12px auto; max-width: 760px; padding: 16px; border: 1px solid var(--td-warning-color-3); border-radius: 12px; background: var(--td-warning-color-1); color: var(--td-text-color-primary); }
.agent-run-recovery__header, .agent-run-recovery__actions { display: flex; align-items: center; gap: 8px; }
.agent-run-recovery__header { justify-content: space-between; }
.agent-run-recovery__status { font-size: 12px; color: var(--td-text-color-secondary); }
.agent-run-recovery__reason, .agent-run-recovery__tool { margin: 8px 0; color: var(--td-text-color-secondary); }
.agent-run-recovery__result { display: grid; gap: 6px; margin: 12px 0; }
.agent-run-recovery textarea { resize: vertical; border: 1px solid var(--td-border-level-2-color); border-radius: 6px; padding: 8px; font: inherit; background: var(--td-bg-color-container); }
.agent-run-recovery button { border: 1px solid var(--td-brand-color); border-radius: 6px; padding: 7px 12px; color: var(--td-brand-color); background: var(--td-bg-color-container); cursor: pointer; }
.agent-run-recovery button.danger { color: var(--td-error-color); border-color: var(--td-error-color); }
.agent-run-recovery button:disabled { opacity: .55; cursor: not-allowed; }
.agent-run-recovery__error { color: var(--td-error-color); margin-bottom: 0; }
</style>
