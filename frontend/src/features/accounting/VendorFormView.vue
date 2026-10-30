<template>
  <FormPage
    :title="t(isEditing ? 'accounting.vendors.editTitle' : `accounting.vendors.add.${form.kind}`)"
    :subtitle="t(`accounting.vendors.hints.${form.kind}`)"
    :back-label="t('common.back')"
    :loading="loading"
    :error="error"
    :busy="busy"
    @submit="save"
    @cancel="leave(returnLocation())"
  >
    <FormSection :title="t(`accounting.vendors.kinds.${form.kind}`)">
      <label class="block">
        <span class="input-label">{{ t('accounting.vendors.name') }}</span>
        <input v-model="form.name" class="input w-full" maxlength="100" required autocomplete="off" data-testid="vendor-name" />
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.status') }}</span>
        <select v-model="form.status" class="input w-full">
          <option value="ACTIVE">{{ t('accounting.vendors.active') }}</option>
          <option value="DISABLED">{{ t('accounting.vendors.disabled') }}</option>
        </select>
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('accounting.vendors.aliases') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model="aliases" class="input w-full" maxlength="1200" autocomplete="off" data-testid="vendor-aliases" />
        <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">{{ t('accounting.vendors.aliasesHint') }}</span>
      </label>
      <label class="block">
        <span class="input-label">{{ t('accounting.vendors.sort') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
        <input v-model.number="form.sort" type="number" class="input w-full" min="0" max="9999" step="1" />
      </label>
      <label class="block sm:col-span-2">
        <span class="input-label">{{ t('admin.desktop.business.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
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
import type { VendorInput, VendorKind } from './types'

// One vendor of a maintained list. Aliases are fragments of how statements and older records spell the vendor;
// saving reclassifies older records that mention one of them.
const { t, te } = useI18n()
const route = useRoute()
const vendorId = computed(() => String(route.params.vendorId || ''))
const isEditing = computed(() => Boolean(vendorId.value))
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const initialKind: VendorKind = route.query.kind === 'AI_VENDOR' ? 'AI_VENDOR' : 'CARD_PROVIDER'
const form = reactive<Required<VendorInput>>({ kind: initialKind, name: '', aliases: [], status: 'ACTIVE', sort: 0, remark: '' })
const aliases = ref('')
const requestKey = api.stableRequest()
const { leave, markClean } = useLeaveGuard(() => ({ ...form, aliases: aliases.value }))

function returnLocation() {
  return safeReturnPath(route.query.return_to) || { name: 'AdminAccountingVendors' }
}
function aliasList() {
  return aliases.value.split(/[,，、\n]/).map((alias) => alias.trim()).filter(Boolean)
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    if (isEditing.value) {
      const vendor = await api.vendor(vendorId.value)
      Object.assign(form, { kind: vendor.kind, name: vendor.name, status: vendor.status, sort: vendor.sort, remark: vendor.remark })
      // The name is always an alias; list only the extra ones.
      aliases.value = vendor.aliases.filter((alias) => alias !== vendor.name.toLowerCase()).join('、')
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
    const payload: VendorInput = { kind: form.kind, name: form.name.trim(), aliases: aliasList(), status: form.status, sort: Number(form.sort) || 0, remark: form.remark.trim() }
    const path = isEditing.value ? `vendors/${vendorId.value}` : 'vendors'
    if (isEditing.value) await api.updateVendor(vendorId.value, payload, requestKey(path, payload))
    else await api.createVendor(payload, requestKey(path, payload))
    requestKey.reset()
    await leave(returnLocation(), true)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
