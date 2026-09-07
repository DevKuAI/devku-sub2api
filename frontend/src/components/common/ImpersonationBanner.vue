<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { clearImpersonation, getImpersonationEmail } from '@/utils/authStorage'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const email = getImpersonationEmail()
const returning = ref(false)

async function returnToAdmin() {
  if (returning.value) return
  returning.value = true
  try {
    await authStore.logout()
  } finally {
    clearImpersonation()
    window.location.assign('/admin/users')
  }
}
</script>

<template>
  <div
    v-if="email !== null"
    role="status"
    class="relative flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-amber-200 bg-amber-50 px-4 py-2 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100 md:px-6"
  >
    <span class="min-w-0 [overflow-wrap:anywhere]">{{ t('admin.users.impersonation.active', { email }) }}</span>
    <button
      type="button"
      :disabled="returning"
      class="inline-flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 font-medium hover:bg-amber-100 disabled:cursor-wait disabled:opacity-50 dark:hover:bg-amber-900"
      @click="returnToAdmin"
    >
      <Icon name="arrowLeft" size="sm" />
      {{ t('admin.users.impersonation.returnToAdmin') }}
    </button>
  </div>
</template>
