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
      <OrganizationContactFields v-model="form" />
    </FormSection>
  </FormPage>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import FormPage from './components/FormPage.vue'
import FormSection from './components/FormSection.vue'
import OrganizationContactFields from './components/OrganizationContactFields.vue'
import * as api from './api'
import { errorMessage, safeReturnPath } from './shared'
import { useLeaveGuard } from './useLeaveGuard'
import { useOrganization } from './useOrganization'
import type { OrganizationContactInput } from './types'

const { t, te } = useI18n()
const route = useRoute()
const organizationId = computed(() => String(route.params.organizationId || ''))
const contactId = computed(() => String(route.params.contactId || ''))
const isEditing = computed(() => Boolean(contactId.value))
const { organization, loadOrganization } = useOrganization(() => organizationId.value)
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const form = ref<OrganizationContactInput>({
  organization_id: '',
  name: '',
  role: 'BUSINESS',
  phone: '',
  email: '',
  wechat: '',
  remark: '',
  is_primary: false,
})
const { leave, markClean } = useLeaveGuard(() => form.value)

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
    form.value.organization_id = organizationId.value
    if (isEditing.value) {
      const contact = contacts.find((item) => item.id === contactId.value)
      if (!contact) throw new Error(t('accounting.contactForm.notFound'))
      const { name, role, phone, email, wechat, remark, is_primary } = contact
      Object.assign(form.value, { name, role, phone, email, wechat, remark, is_primary })
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
      ...form.value,
      organization_id: organizationId.value,
      name: form.value.name.trim(),
      phone: form.value.phone.trim(),
      email: form.value.email.trim(),
      wechat: form.value.wechat.trim(),
      remark: form.value.remark.trim(),
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
