<template>
  <AppLayout>
    <div class="space-y-6">
      <RouterLink to="/admin/settings?tab=security" class="btn btn-secondary">{{ t('common.back') }}</RouterLink>
      <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('admin.settings.registration.emailBlacklist.editorTitle') }}</h1>
      <RegistrationEmailBlacklistEditor page />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import RegistrationEmailBlacklistEditor from '@/components/admin/settings/RegistrationEmailBlacklistEditor.vue'

const { t } = useI18n()
</script>
