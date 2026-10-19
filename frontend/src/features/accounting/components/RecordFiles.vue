<template>
  <section class="space-y-3" data-testid="record-files">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="accounting-section-title">{{ t('accounting.files.title') }}</h2>
    </header>
    <AccountingFileUpload v-if="!customer && record.status !== 'VOID'" :model-value="selectedFile" :disabled="busy" :busy="uploading" @update:model-value="upload" />
    <div v-if="error" role="alert" class="accounting-error flex flex-wrap items-center justify-between gap-3">
      <p>{{ error }}</p>
      <button v-if="loadFailed" type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
    </div>
    <ul v-if="files.length" class="divide-y divide-gray-100 dark:divide-dark-700">
      <li v-for="file in files" :key="file.id" class="flex flex-wrap items-center justify-between gap-2 py-2.5">
        <div class="min-w-0">
          <p class="break-all text-sm font-medium">{{ file.filename }}</p>
          <p class="mt-0.5 text-xs text-gray-500">
            {{ formatSize(file.size_bytes) }}
            <template v-if="!customer"> · {{ t(file.customer_visible ? 'accounting.files.shared' : 'accounting.files.internal') }}</template>
          </p>
        </div>
        <div class="flex flex-wrap gap-1">
          <button type="button" class="btn btn-ghost btn-sm" @click="download(file)">{{ t('accounting.files.download') }}</button>
          <template v-if="!customer">
            <button v-if="shareable" type="button" class="btn btn-ghost btn-sm" @click="toggleShare(file)">
              {{ t(file.customer_visible ? 'accounting.unshare' : 'accounting.share') }}
            </button>
            <button type="button" class="btn btn-ghost btn-sm text-red-600 dark:text-red-400" @click="pendingDelete = file">
              {{ t('common.delete') }}
            </button>
          </template>
        </div>
      </li>
    </ul>
    <p v-else-if="!loading && !loadFailed" class="text-sm text-gray-500">{{ t('accounting.files.empty') }}</p>
    <ConfirmDialog
      :show="Boolean(pendingDelete)"
      :title="t('accounting.files.deleteTitle')"
      :message="t('accounting.files.deleteMessage', { name: pendingDelete?.filename || '' })"
      :confirm-text="t('common.delete')"
      :loading="busy"
      danger
      @confirm="remove"
      @cancel="pendingDelete = null"
    />
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import AccountingFileUpload from './AccountingFileUpload.vue'
import * as api from '../api'
import { errorMessage } from '../shared'
import type { AccountingAttachment, AccountingKind, AccountingRecord } from '../types'

const props = defineProps<{ kind: AccountingKind; record: AccountingRecord; customer?: boolean }>()
const emit = defineEmits<{ changed: [] }>()
const { t, te } = useI18n()
const files = ref<AccountingAttachment[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const busy = ref(false)
const uploading = ref(false)
const selectedFile = ref<File | null>(null)
const error = ref('')
const pendingDelete = ref<AccountingAttachment | null>(null)
const requestKey = api.stableRequest()
// Customers may see transfer proofs, contracts, sales invoices and card purchase receipts once shared.
const shareable =
  ['transfers', 'contracts'].includes(props.kind) ||
  (props.kind === 'invoices' && props.record.details.invoice_type === 'SALES') ||
  (props.kind === 'vcc-transactions' && ['CONSUME', 'REFUND'].includes(String(props.record.details.transaction_type)))

function formatSize(bytes: number) {
  return bytes >= 1048576 ? `${(bytes / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`
}
async function run(action: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await action()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
async function load() {
  loading.value = true
  loadFailed.value = false
  error.value = ''
  try {
    files.value = (await api.attachments(props.kind, props.record.id, props.customer)).filter((file) => file.status === 'COMPLETE')
  } catch (cause) {
    loadFailed.value = true
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}
async function upload(file: File | null) {
  if (busy.value) return
  selectedFile.value = file
  if (!file) return
  uploading.value = true
  await run(async () => {
    await api.upload(file, props.kind, props.record)
    await load()
    selectedFile.value = null
    emit('changed')
  })
  uploading.value = false
}
async function download(file: AccountingAttachment) {
  await run(async () => {
    window.open(await api.download(file.id, props.customer), '_blank', 'noopener,noreferrer')
  })
}
async function toggleShare(file: AccountingAttachment) {
  await run(async () => {
    const path = `attachments/${file.id}/sharing`
    const payload = { customer_visible: !file.customer_visible }
    await api.mutate(path, payload, requestKey(path, payload), 'patch')
    requestKey.reset()
    file.customer_visible = !file.customer_visible
  })
}
async function remove() {
  const file = pendingDelete.value
  if (!file) return
  await run(async () => {
    const path = `attachments/${file.id}`
    await api.mutate(path, {}, requestKey(path, {}), 'delete')
    requestKey.reset()
    files.value = files.value.filter((item) => item.id !== file.id)
    emit('changed')
  })
  pendingDelete.value = null
}

watch(() => props.record.id, load)
onMounted(load)
defineExpose({ reload: load })
</script>
