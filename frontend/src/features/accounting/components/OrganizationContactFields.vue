<template>
  <label class="block">
    <span class="input-label">{{ t('common.name') }}</span>
    <input v-model="form.name" class="input w-full" maxlength="100" required autocomplete="off" />
  </label>
  <label class="block">
    <span class="input-label">{{ t('admin.desktop.business.role') }}</span>
    <select v-model="form.role" class="input w-full" required>
      <option v-for="role in roles" :key="role" :value="role">{{ t(`admin.desktop.business.roles.${role}`) }}</option>
    </select>
  </label>
  <label class="block">
    <span class="input-label">{{ t('admin.desktop.business.phone') }}</span>
    <input v-model="form.phone" type="tel" class="input w-full" maxlength="40" autocomplete="tel" />
  </label>
  <label class="block">
    <span class="input-label">{{ t('common.email') }}</span>
    <input v-model="form.email" type="email" class="input w-full" maxlength="254" autocomplete="email" />
  </label>
  <label class="block">
    <span class="input-label">{{ t('admin.desktop.business.wechat') }}</span>
    <input v-model="form.wechat" class="input w-full" maxlength="100" autocomplete="off" />
  </label>
  <label class="flex items-start gap-3 sm:col-span-2">
    <input v-model="form.is_primary" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0" />
    <span>
      <span class="block text-sm font-medium">{{ t('admin.desktop.business.setPrimary') }}</span>
      <span class="mt-1 block text-xs text-gray-500">{{ t('admin.desktop.business.primaryHint') }}</span>
    </span>
  </label>
  <label class="block sm:col-span-2">
    <span class="input-label">{{ t('admin.desktop.business.remark') }}</span>
    <textarea v-model="form.remark" class="input min-h-20 w-full resize-y" maxlength="1000" />
  </label>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { OrganizationContactInput, OrganizationContactRole } from '../types'

const form = defineModel<OrganizationContactInput>({ required: true })
const { t } = useI18n()
const roles: OrganizationContactRole[] = ['BUSINESS', 'FINANCE', 'LEGAL', 'TECHNICAL', 'OTHER']
</script>
