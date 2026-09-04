<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.binding.title')"
    width="normal"
    @close="emit('close')"
  >
    <p class="mb-5 text-sm text-gray-500 dark:text-dark-400">
      {{ t('admin.accounts.binding.description') }}
    </p>
    <AccountBindingEditor
      v-if="show && account"
      :account-id="account.id"
      @close="emit('close')"
      @updated="emit('updated', $event)"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import AccountBindingEditor from '@/components/admin/account/AccountBindingEditor.vue'
import type { Account } from '@/types'

defineProps<{
  show: boolean
  account: Account | null
}>()

const emit = defineEmits<{
  close: []
  updated: [account: Account]
}>()

const { t } = useI18n()
</script>
