<template>
  <AppLayout>
    <div class="mx-auto max-w-3xl space-y-6">
      <div class="flex items-start gap-3">
        <button
          type="button"
          class="btn btn-secondary px-3"
          :title="t('common.back')"
          :aria-label="t('common.back')"
          @click="router.push('/admin/accounts')"
        >
          <Icon name="arrowLeft" size="md" />
        </button>
        <div class="min-w-0 flex-1">
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.accounts.binding.title') }}
          </h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ t('admin.accounts.binding.description') }}
          </p>
        </div>
      </div>
      <AccountBindingEditor :account-id="accountID" @close="goBack" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import AccountBindingEditor from '@/components/admin/account/AccountBindingEditor.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const accountID = computed(() => Number(route.params.id))

function goBack(): void {
  void router.push('/admin/accounts')
}
</script>
