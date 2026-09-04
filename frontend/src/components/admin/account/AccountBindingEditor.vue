<template>
  <div>
    <div v-if="loading" class="flex justify-center py-12" role="status">
      <div class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"></div>
    </div>

    <div v-else-if="loadFailed" class="rounded-lg border border-red-200 py-8 text-center dark:border-red-900/50">
      <p class="text-sm text-red-600 dark:text-red-400">{{ t('admin.accounts.binding.loadFailed') }}</p>
      <button type="button" class="btn btn-secondary mt-4" @click="loadAccount">
        {{ t('admin.accounts.binding.retry') }}
      </button>
    </div>

    <template v-else-if="account">
      <section class="grid gap-4 rounded-lg border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-900/40 sm:grid-cols-3">
        <div class="min-w-0 sm:col-span-2">
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.binding.account') }}</div>
          <div class="mt-1 break-words font-medium text-gray-900 dark:text-white">{{ account.name }}</div>
          <div class="mt-1 font-mono text-xs text-gray-400">#{{ account.id }}</div>
        </div>
        <div>
          <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.columns.platformType') }}</div>
          <div class="mt-1"><PlatformTypeBadge :platform="account.platform" :type="account.type" /></div>
        </div>
      </section>

      <form class="mt-5 space-y-5" @submit.prevent="saveBinding">
        <div>
          <label class="input-label mb-1.5 block" for="bound-user-select">
            {{ t('admin.accounts.binding.selectUser') }}
          </label>
          <Select
            id="bound-user-select"
            v-model="selectedUserID"
            :options="userOptions"
            :placeholder="t('admin.accounts.binding.unbound')"
            :search-placeholder="t('admin.accounts.binding.searchUser')"
            :empty-text="t('admin.accounts.binding.noUsers')"
            :loading="usersLoading"
            searchable
            remote
            clearable
            @search="loadUsers"
          />
          <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.binding.hint') }}
          </p>
        </div>

        <div
          v-if="account.bound_user"
          class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-gray-200 px-4 py-3 dark:border-dark-700"
        >
          <div class="min-w-0">
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.accounts.binding.currentUser') }}</div>
            <div class="mt-1 break-words text-sm font-medium text-gray-900 dark:text-white">
              {{ account.bound_user.username || account.bound_user.email }}
            </div>
            <div v-if="account.bound_user.username" class="break-all text-xs text-gray-500 dark:text-dark-400">
              {{ account.bound_user.email }}
            </div>
          </div>
          <button
            type="button"
            class="btn btn-danger"
            :disabled="saving"
            @click="showRemoveConfirm = true"
          >
            <Icon name="x" size="sm" class="mr-1.5" />
            {{ t('admin.accounts.binding.removeUser') }}
          </button>
        </div>

        <div class="flex justify-end gap-3 border-t border-gray-100 pt-4 dark:border-dark-700">
          <button type="button" class="btn btn-secondary" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button type="submit" class="btn btn-primary" :disabled="saving || !bindingChanged">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </form>
    </template>
  </div>

  <ConfirmDialog
    :show="showRemoveConfirm"
    :title="t('admin.accounts.binding.removeTitle')"
    :message="removeConfirmMessage"
    :confirm-text="t('admin.accounts.binding.removeUser')"
    :cancel-text="t('common.cancel')"
    :danger="true"
    @confirm="removeBinding"
    @cancel="showRemoveConfirm = false"
  />
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import { useSubscriptionAccountAccess } from '@/composables/useSubscriptionAccountAccess'
import { useAppStore } from '@/stores/app'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import Icon from '@/components/icons/Icon.vue'
import { extractApiErrorCode } from '@/utils/apiError'
import type { Account, AdminUser } from '@/types'

const props = defineProps<{
  accountId: number
}>()

const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const { refreshSubscriptionAccountAccess } = useSubscriptionAccountAccess()

const account = ref<Account | null>(null)
const selectedUserID = ref<number | null>(null)
const users = ref<AdminUser[]>([])
const loading = ref(true)
const loadFailed = ref(false)
const usersLoading = ref(false)
const saving = ref(false)
const showRemoveConfirm = ref(false)
let usersAbortController: AbortController | null = null

const bindingChanged = computed(() => selectedUserID.value !== (account.value?.bound_user_id ?? null))
const removeConfirmMessage = computed(() => t('admin.accounts.binding.removeConfirm', {
  account: account.value?.name ?? '',
  user: account.value?.bound_user?.username || account.value?.bound_user?.email || ''
}))

const userOptions = computed<SelectOption[]>(() => {
  const byID = new Map<number, { id: number; username: string; email: string }>()
  if (account.value?.bound_user) {
    byID.set(account.value.bound_user.id, account.value.bound_user)
  }
  for (const user of users.value) {
    byID.set(user.id, user)
  }
  return [...byID.values()].map((user) => ({
    value: user.id,
    label: user.username ? `${user.username} · ${user.email}` : user.email,
    description: user.email
  }))
})

async function loadAccount(): Promise<void> {
  if (!Number.isInteger(props.accountId) || props.accountId <= 0) {
    loading.value = false
    loadFailed.value = true
    return
  }

  usersAbortController?.abort()
  loading.value = true
  loadFailed.value = false
  showRemoveConfirm.value = false
  try {
    account.value = await adminAPI.accounts.getById(props.accountId)
    selectedUserID.value = account.value.bound_user_id
    await loadUsers('')
  } catch {
    account.value = null
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}

async function loadUsers(search: string): Promise<void> {
  usersAbortController?.abort()
  const controller = new AbortController()
  usersAbortController = controller
  usersLoading.value = true
  try {
    const result = await adminAPI.users.list(
      1,
      20,
      { status: 'active', search: search || undefined },
      { signal: controller.signal }
    )
    if (!controller.signal.aborted) {
      users.value = result.items
    }
  } catch {
    if (!controller.signal.aborted) {
      users.value = []
    }
  } finally {
    if (usersAbortController === controller) {
      usersLoading.value = false
    }
  }
}

async function applyBinding(userID: number | null, successMessage: string): Promise<void> {
  if (!account.value) return
  const expectedBoundUserID = account.value.bound_user_id
  saving.value = true
  try {
    account.value = await adminAPI.accounts.bindUser(account.value.id, userID, expectedBoundUserID)
    selectedUserID.value = account.value.bound_user_id
    showRemoveConfirm.value = false
    appStore.showSuccess(successMessage)
    await refreshSubscriptionAccountAccess(true)
    emit('updated', account.value)
  } catch (error) {
    if (extractApiErrorCode(error) === 'ACCOUNT_BINDING_CONFLICT') {
      await loadAccount()
      appStore.showError(t('admin.accounts.binding.conflict'))
    } else {
      appStore.showError(t('admin.accounts.binding.saveFailed'))
    }
  } finally {
    saving.value = false
  }
}

async function saveBinding(): Promise<void> {
  await applyBinding(selectedUserID.value, t('admin.accounts.binding.saved'))
}

async function removeBinding(): Promise<void> {
  await applyBinding(null, t('admin.accounts.binding.removed'))
}

watch(() => props.accountId, loadAccount, { immediate: true })
onBeforeUnmount(() => usersAbortController?.abort())
</script>
