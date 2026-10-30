<template>
  <AppLayout>
    <div class="accounting-page min-w-0 space-y-6" data-testid="channel-import">
      <PageHeader
        :title="t('accounting.channels.import.title')"
        :subtitle="t('accounting.channels.import.description')"
        :back="{ label: t('accounting.channels.import.back') }"
        @back="leave({ name: 'AdminAccountingChannels' })"
      />

      <div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <section class="space-y-4">
          <template v-if="!result">
            <label
              class="channel-dropzone"
              :class="{ 'is-active': dragging }"
              data-testid="channel-dropzone"
              @dragover.prevent="dragging = true"
              @dragleave.prevent="dragging = false"
              @drop.prevent="dropped"
            >
              <input ref="input" type="file" class="sr-only" multiple accept=".pdf,.xlsx,.csv" data-testid="channel-file-input" @change="chosen" />
              <Icon name="upload" size="lg" aria-hidden="true" />
              <span class="font-medium text-gray-900 dark:text-white">{{ t('accounting.channels.import.drop') }}</span>
              <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('accounting.channels.import.dropHint') }}</span>
            </label>

            <div v-if="files.length" class="card p-4">
              <div class="mb-2 flex items-center justify-between">
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('accounting.channels.import.selected', { count: files.length }) }}</p>
                <button type="button" class="accounting-link text-sm" :disabled="Boolean(busy)" @click="clearFiles">{{ t('accounting.channels.import.clear') }}</button>
              </div>
              <ul class="divide-y divide-gray-100 dark:divide-dark-700">
                <li v-for="(file, index) in files" :key="fileKey(file)" class="flex items-center justify-between gap-3 py-2 text-sm">
                  <span class="min-w-0 truncate">{{ file.name }}</span>
                  <span class="flex shrink-0 items-center gap-3 text-xs text-gray-500">
                    {{ size(file.size) }}
                    <button type="button" class="accounting-link" :disabled="Boolean(busy)" :aria-label="`${t('accounting.channels.import.remove')} ${file.name}`" @click="files.splice(index, 1)">
                      {{ t('accounting.channels.import.remove') }}
                    </button>
                  </span>
                </li>
              </ul>
            </div>

            <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
            <div v-if="problems.length" role="alert" class="card border-red-200 p-4 dark:border-red-900/60" data-testid="channel-problems">
              <p class="font-medium text-red-700 dark:text-red-300">{{ t('accounting.channels.import.problemsTitle') }}</p>
              <ul class="mt-2 space-y-1 text-sm text-gray-700 dark:text-dark-200">
                <li v-for="(problem, index) in problems" :key="index">
                  <span v-if="problem.file" class="font-mono text-xs text-gray-500">
                    {{ problem.row ? t('accounting.channels.import.problemAt', { file: problem.file, row: problem.row }) : problem.file }}
                    <template v-if="problem.column"> · {{ problem.column }}</template>
                  </span>
                  {{ problem.message }}
                </li>
              </ul>
            </div>
            <p v-if="notice" role="status" class="text-sm text-emerald-700 dark:text-emerald-400">{{ notice }}</p>

            <div class="flex flex-wrap gap-3">
              <button type="button" class="btn btn-primary" :disabled="!files.length || Boolean(busy)" data-testid="channel-import-submit" @click="submit">
                {{ busy === 'import' ? t('accounting.channels.import.importing') : t('accounting.channels.import.submit') }}
              </button>
              <button type="button" class="btn btn-secondary" :disabled="!files.length || Boolean(busy)" data-testid="channel-convert" @click="convert">
                {{ busy === 'convert' ? t('accounting.channels.import.converting') : t('accounting.channels.import.convert') }}
              </button>
            </div>
          </template>

          <div v-else class="card space-y-4 p-5" data-testid="channel-import-result">
            <div>
              <p class="text-base font-semibold text-gray-900 dark:text-white">{{ t('accounting.channels.import.resultTitle') }}</p>
              <p class="mt-1 text-sm text-gray-600 dark:text-dark-300">
                {{ t('accounting.channels.import.resultRows', { total: result.import.total_rows, new: result.import.new_rows, duplicate: result.import.duplicate_rows }) }}
              </p>
            </div>
            <div v-for="summary in result.accounts" :key="summary.account.id" class="space-y-3">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ summary.account.name || summary.account.account_ref }} · {{ summary.account.platform }} {{ summary.account.account_ref }}</p>
              <WalletFlow :wallets="summary.wallets" />
              <router-link class="accounting-link text-sm" :to="{ name: 'AdminAccountingChannel', params: { accountId: summary.account.id } }">{{ t('accounting.channels.import.viewAccount') }}</router-link>
            </div>
            <button type="button" class="btn btn-secondary" @click="reset">{{ t('accounting.channels.import.importMore') }}</button>
          </div>
        </section>

        <aside class="card h-fit space-y-4 p-5">
          <div>
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('accounting.channels.import.reportsTitle') }}</p>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">{{ t('accounting.channels.import.reportsHint') }}</p>
          </div>
          <ol class="space-y-2 text-sm">
            <li v-for="report in reports" :key="report" class="flex gap-2">
              <Icon name="document" size="sm" class="mt-0.5 shrink-0 text-gray-400" aria-hidden="true" />
              <span>
                <span class="block text-gray-900 dark:text-white">{{ t(`accounting.channels.import.reports.${report}.name`) }}</span>
                <span class="block text-xs text-gray-500 dark:text-dark-400">{{ t(`accounting.channels.import.reports.${report}.hint`) }}</span>
              </span>
            </li>
          </ol>
          <p class="border-t border-gray-100 pt-3 text-xs text-gray-500 dark:border-dark-700 dark:text-dark-400">{{ t('accounting.channels.import.otherPlatforms') }}</p>
          <button type="button" class="accounting-link text-sm" data-testid="channel-template" @click="template">
            {{ t('accounting.channels.import.template') }}
          </button>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import PageHeader from './components/PageHeader.vue'
import WalletFlow from './components/channel/WalletFlow.vue'
import * as api from './api'
import { saveFile, uploadProblems } from './channelShared'
import { errorMessage } from './shared'
import type { ChannelImportResult, ChannelProblem } from './types'
import { useLeaveGuard } from './useLeaveGuard'
import '@/styles/accounting.css'

// One upload books everything that passes the checks; rejected uploads list every row that needs fixing.
const { t, te } = useI18n()
const reports = ['statementHKD', 'statementUSD', 'cardBalances', 'authorisations', 'collection', 'conversion']
const maxFiles = 20
const maxBytes = 20 * 1024 * 1024
const files = ref<File[]>([])
const dragging = ref(false)
const busy = ref<'' | 'import' | 'convert'>('')
const error = ref('')
const notice = ref('')
const problems = ref<ChannelProblem[]>([])
const result = ref<ChannelImportResult | null>(null)
const input = ref<HTMLInputElement | null>(null)
const requestKey = api.stableRequest()
const { leave, markClean } = useLeaveGuard(() => files.value.map(fileKey))
markClean()

function fileKey(file: File) {
  return `${file.name}:${file.size}:${file.lastModified}`
}
function size(bytes: number) {
  return bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.round(bytes / 1024))} KB`
}
function add(list: FileList | null | undefined) {
  error.value = ''
  for (const file of Array.from(list || [])) {
    if (!/\.(pdf|xlsx|csv)$/i.test(file.name)) {
      error.value = t('accounting.channels.import.unsupported', { name: file.name })
      continue
    }
    if (file.size > maxBytes) {
      error.value = t('accounting.channels.import.tooLarge', { name: file.name })
      continue
    }
    if (files.value.some((existing) => fileKey(existing) === fileKey(file))) continue
    if (files.value.length >= maxFiles) {
      error.value = t('accounting.channels.import.tooMany')
      break
    }
    files.value.push(file)
  }
  problems.value = []
  notice.value = ''
}
function chosen(event: Event) {
  const target = event.target as HTMLInputElement
  add(target.files)
  target.value = ''
}
function dropped(event: DragEvent) {
  dragging.value = false
  add(event.dataTransfer?.files)
}
function clearFiles() {
  files.value = []
  problems.value = []
  error.value = ''
  notice.value = ''
}
function fail(cause: unknown) {
  problems.value = uploadProblems(cause)
  error.value = problems.value.length ? '' : errorMessage(cause, t, te)
}
async function submit() {
  busy.value = 'import'
  error.value = ''
  problems.value = []
  notice.value = ''
  try {
    const fingerprint = files.value.map(fileKey)
    result.value = await api.importChannel(files.value, requestKey('channel-imports', fingerprint))
    requestKey.reset()
    files.value = []
    markClean()
  } catch (cause) {
    fail(cause)
  } finally {
    busy.value = ''
  }
}
async function convert() {
  busy.value = 'convert'
  error.value = ''
  problems.value = []
  notice.value = ''
  try {
    const converted = await api.convertChannel(files.value)
    saveFile('channel-statement-converted.csv', converted.csv)
    notice.value = t('accounting.channels.import.converted', { lines: converted.lines })
  } catch (cause) {
    fail(cause)
  } finally {
    busy.value = ''
  }
}
async function template() {
  try {
    saveFile('channel-statement-template.csv', await api.channelTemplate())
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}
function reset() {
  result.value = null
  clearFiles()
  markClean()
  input.value?.focus()
}
</script>
