<template>
  <BaseDialog
    :show="true"
    :title="t('accounting.contactForm.createTitle')"
    :close-label="t('common.close')"
    :show-close-button="!busy"
    :close-on-escape="!busy"
    @close="close"
  >
    <form :id="formId" :aria-busy="busy" @submit.prevent="save">
      <p v-if="error" class="mb-4 text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
      <fieldset class="grid min-w-0 gap-4 sm:grid-cols-2" :disabled="busy">
        <OrganizationContactFields v-model="form" />
      </fieldset>
    </form>
    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.cancel') }}</button>
        <button type="submit" :form="formId" class="btn btn-primary" :disabled="busy">{{ t('common.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import OrganizationContactFields from './OrganizationContactFields.vue'
import { createOrganizationContact } from '../api'
import { errorMessage } from '../shared'
import type { OrganizationContactInput } from '../types'

const props = defineProps<{ organizationId: string }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t, te } = useI18n()
const formId = useId()
const form = ref<OrganizationContactInput>({
  organization_id: props.organizationId,
  name: '', role: 'BUSINESS', phone: '', email: '', wechat: '', remark: '', is_primary: false,
})
const initial = JSON.stringify(form.value)
const busy = ref(false)
const error = ref('')

function close() {
  if (busy.value) return
  if (JSON.stringify(form.value) !== initial && !window.confirm(t('accounting.form.discardChanges'))) return
  emit('close')
}

async function save(event: Event) {
  if (busy.value || !(event.target as HTMLFormElement).reportValidity()) return
  const payload: OrganizationContactInput = {
    ...form.value,
    name: form.value.name.trim(), phone: form.value.phone.trim(), email: form.value.email.trim(),
    wechat: form.value.wechat.trim(), remark: form.value.remark.trim(),
  }
  if (!payload.name) { error.value = t('accounting.errors.invalidField', { field: t('common.name') }); return }
  busy.value = true
  error.value = ''
  try {
    await createOrganizationContact(payload)
    emit('saved')
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
</script>
