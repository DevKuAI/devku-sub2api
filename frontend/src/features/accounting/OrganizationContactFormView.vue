<template>
  <FormPage
    :title="t(isEditing ? 'accounting.contactForm.editTitle' : 'accounting.contactForm.createTitle')"
    :subtitle="organization?.name"
    :back-label="t('common.back')"
    :loading="loading"
    :error="error"
    :busy="busy"
    @submit="save"
    @cancel="leave(returnLocation())"
  >
    <FormSection :title="t('accounting.contactForm.basic')">
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
    </FormSection>
  </FormPage>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import * as api from './api'
import { errorMessage, safeReturnPath } from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import { useOrganization } from './useOrganization'
import type { OrganizationContactInput, OrganizationContactRole } from './types'

const { t, te } = useI18n()
const route = useRoute()
const roles: OrganizationContactRole[] = ['BUSINESS', 'FINANCE', 'LEGAL', 'TECHNICAL', 'OTHER']
const organizationId = computed(() => String(route.params.organizationId || ''))
const contactId = computed(() => String(route.params.contactId || ''))
const isEditing = computed(() => Boolean(contactId.value))
const { organization, loadOrganization } = useOrganization(() => organizationId.value)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const form = reactive<OrganizationContactInput>({
  organization_id: '',
  name: '',
  role: 'BUSINESS',
  phone: '',
  email: '',
  wechat: '',
  remark: '',
  is_primary: false,
})
const { leave, markClean } = useLeaveGuard(() => form)

// Contacts are organization master data; callers such as the receipt form pass their own page back.
function returnLocation() {
  return (
    safeReturnPath(route.query.return_to) || {
      name: 'AdminDesktopOrganizationDetail',
      params: { organizationId: organizationId.value },
      query: { tab: 'business' },
    }
  )
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const [, contacts] = await Promise.all([loadOrganization(), isEditing.value ? api.organizationContacts(organizationId.value) : []])
    form.organization_id = organizationId.value
    if (isEditing.value) {
      const contact = contacts.find((item) => item.id === contactId.value)
      if (!contact) throw new Error(t('accounting.contactForm.notFound'))
      const { name, role, phone, email, wechat, remark, is_primary } = contact
      Object.assign(form, { name, role, phone, email, wechat, remark, is_primary })
    }
    await nextTick()
    markClean()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}
async function save() {
  busy.value = true
  error.value = ''
  try {
    const payload: OrganizationContactInput = {
      ...form,
      organization_id: organizationId.value,
      name: form.name.trim(),
      phone: form.phone.trim(),
      email: form.email.trim(),
      wechat: form.wechat.trim(),
      remark: form.remark.trim(),
    }
    if (isEditing.value) await api.updateOrganizationContact(contactId.value, payload)
    else await api.createOrganizationContact(payload)
    await leave(returnLocation(), true)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
