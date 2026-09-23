<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <section class="card space-y-4 p-6">
        <h1 class="text-xl font-semibold">{{ t('bi.title') }}</h1>
        <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('bi.account', { account: auth.user?.email || '' }) }}</p>
        <p v-if="enabled === false">{{ t('bi.disabled') }}</p>
        <form v-if="enabled" class="space-y-4" @submit.prevent="approve">
          <div>
            <label for="bi-code" class="mb-2 block text-sm font-medium">{{ t('bi.userCode') }}</label>
            <input id="bi-code" v-model="code" class="input max-w-xs font-mono uppercase" maxlength="8" pattern="[A-Za-z0-9]{8}" autocomplete="off" required :disabled="busy" aria-describedby="bi-code-hint" />
            <p id="bi-code-hint" class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('bi.userCodeHint') }}</p>
          </div>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="confirmed" type="checkbox" :disabled="busy" required />{{ t('bi.confirm') }}
          </label>
          <button type="submit" class="btn btn-primary" :disabled="busy || !confirmed || !validCode" :aria-busy="busy">{{ t('bi.approve') }}</button>
        </form>
        <p v-if="notice" role="status" class="text-sm text-green-700 dark:text-green-300">{{ notice }}</p>
        <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      </section>
      <section v-if="enabled" class="card space-y-4 p-6" :aria-busy="loading">
        <div class="flex items-center justify-between gap-3">
          <h2 class="font-semibold">{{ t('bi.listTitle') }}</h2>
          <button class="btn btn-secondary" type="button" :disabled="loading || busy" @click="load(false)">{{ t('common.refresh') }}</button>
        </div>
        <p v-if="!loading && !error && bindings.length === 0" class="text-sm text-gray-500">{{ t('bi.empty') }}</p>
        <ul class="divide-y divide-gray-200 dark:divide-dark-700">
          <li v-for="binding in bindings" :key="binding.id" class="flex flex-wrap items-center justify-between gap-4 py-4">
            <div class="space-y-1">
              <p class="font-medium">{{ binding.display_name }}</p>
              <p class="text-sm text-gray-500">{{ t('bi.createdAt') }}: {{ formatDateTime(binding.created_at) }}</p>
              <p class="text-sm text-gray-500">{{ t('bi.lastLoginAt') }}: {{ binding.last_login_at ? formatDateTime(binding.last_login_at) : t('bi.neverLoggedIn') }}</p>
            </div>
            <button type="button" class="btn btn-secondary text-red-600" :disabled="busy" @click="revokeTarget = binding">{{ t('bi.revoke') }}</button>
          </li>
        </ul>
        <button v-if="nextCursor" type="button" class="btn btn-secondary" :disabled="loading || busy" @click="load(true)">{{ t('bi.loadMore') }}</button>
      </section>
    </div>
    <ConfirmDialog :show="!!revokeTarget" :title="t('bi.revokeTitle')" :message="t('bi.revokeMessage')" :loading="busy" danger @confirm="revoke" @cancel="revokeTarget = null" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { approveBinding, isBIEnabled, listBindings, revokeBinding, type BIBinding } from '@/api/bi'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const enabled = ref<boolean | null>(null)
const code = ref('')
const confirmed = ref(false)
const validCode = computed(() => /^[A-Z0-9]{8}$/.test(code.value.trim().toUpperCase()))
const busy = ref(false)
const loading = ref(false)
const error = ref('')
const notice = ref('')
const bindings = ref<BIBinding[]>([])
const nextCursor = ref<string | null>(null)
const revokeTarget = ref<BIBinding | null>(null)
let loadGeneration = 0

function reportError(cause: unknown) {
  const code = (cause as { code?: string; reason?: string }).code || (cause as { reason?: string }).reason
  error.value = t(code === 'BINDING_EXPIRED' ? 'bi.expired' : code === 'BINDING_CONFLICT' ? 'bi.conflict' : code === 'CONTEXT_REVOKED' || code === 'CONTEXT_EXPIRED' ? 'bi.reload' : 'bi.failed')
}

async function load(append: boolean) {
  const current = ++loadGeneration
  loading.value = true
  error.value = ''
  try {
    const page = await listBindings(append ? nextCursor.value || undefined : undefined)
    if (current !== loadGeneration) return
    bindings.value = append ? [...bindings.value, ...page.items] : page.items
    nextCursor.value = page.next_cursor
  } catch (cause) { if (current === loadGeneration) reportError(cause) }
  finally { if (current === loadGeneration) loading.value = false }
}

async function approve() {
  if (busy.value || !confirmed.value || !validCode.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await approveBinding(code.value.trim().toUpperCase())
    code.value = ''
    confirmed.value = false
    notice.value = t('bi.approved')
    await load(false)
  } catch (cause) { reportError(cause) } finally { busy.value = false }
}

async function revoke() {
  if (!revokeTarget.value || busy.value) return
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await revokeBinding(revokeTarget.value.id)
    revokeTarget.value = null
    notice.value = t('bi.revoked')
    await load(false)
  } catch (cause) { reportError(cause) } finally { busy.value = false }
}

onMounted(async () => {
  try { enabled.value = await isBIEnabled(); if (enabled.value) await load(false) } catch (cause) { reportError(cause) }
})
</script>
