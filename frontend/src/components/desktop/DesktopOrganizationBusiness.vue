<template>
  <div class="min-w-0 space-y-5" data-testid="organization-business">
    <section class="card detail-panel space-y-4">
      <header class="detail-tab-heading">
        <div>
          <h2 class="detail-section-title">{{ t('admin.desktop.business.contacts') }}</h2>
          <p class="detail-caption mt-1">{{ t('admin.desktop.business.contactsHint') }}</p>
        </div>
        <button ref="addContactButton" class="btn btn-primary" type="button" @click="creatingContact = true">
          <Icon name="userPlus" size="sm" />
          <span>{{ t('admin.desktop.business.addContact') }}</span>
        </button>
      </header>

      <p v-if="error" class="detail-error" role="alert">{{ error }}</p>
      <p v-if="loading" class="detail-caption" role="status">{{ t('common.loading') }}</p>
      <div v-else-if="contacts.length" class="table-responsive">
        <table class="w-full text-left text-sm">
          <thead class="text-xs text-gray-500 dark:text-dark-400">
            <tr class="border-b border-gray-200 dark:border-dark-700">
              <th class="px-3 py-2 font-medium">{{ t('common.name') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.desktop.business.role') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.desktop.business.contactMethods') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.desktop.business.remark') }}</th>
              <th class="px-3 py-2 text-right font-medium">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="contact in contacts" :key="contact.id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
              <td class="px-3 py-3 align-top">
                <div class="flex flex-wrap items-center gap-2 font-medium text-gray-900 dark:text-white">
                  <span>{{ contact.name }}</span>
                  <span v-if="contact.is_primary" class="badge badge-success">{{ t('admin.desktop.business.primary') }}</span>
                </div>
              </td>
              <td class="px-3 py-3 align-top">{{ t(`admin.desktop.business.roles.${contact.role}`) }}</td>
              <td class="px-3 py-3 align-top">
                <div class="space-y-1">
                  <a v-if="contact.phone" class="block text-primary-600 hover:underline dark:text-primary-400" :href="`tel:${contact.phone}`">{{ contact.phone }}</a>
                  <a v-if="contact.email" class="block break-all text-primary-600 hover:underline dark:text-primary-400" :href="`mailto:${contact.email}`">{{ contact.email }}</a>
                  <span v-if="contact.wechat" class="block break-all">{{ t('admin.desktop.business.wechat') }}: {{ contact.wechat }}</span>
                  <span v-if="!contact.phone && !contact.email && !contact.wechat" class="text-gray-400">—</span>
                </div>
              </td>
              <td class="max-w-64 px-3 py-3 align-top text-gray-600 dark:text-dark-300">{{ contact.remark || '—' }}</td>
              <td class="px-3 py-3 align-top">
                <div class="flex justify-end gap-1">
                  <router-link class="detail-row-action" :to="editContactRoute(contact)" :title="t('common.edit')" :aria-label="`${t('common.edit')} ${contact.name}`">
                    <Icon name="edit" size="sm" />
                  </router-link>
                  <button class="detail-row-action text-red-600 dark:text-red-400" type="button" :title="t('common.delete')" :aria-label="`${t('common.delete')} ${contact.name}`" @click="removeContact(contact)">
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.business.noContacts') }}</p>
    </section>

    <section class="card detail-panel space-y-4">
      <header class="detail-tab-heading">
        <div>
          <h2 class="detail-section-title">{{ t('admin.desktop.business.contracts') }}</h2>
          <p class="detail-caption mt-1">{{ t('admin.desktop.business.contractsHint') }}</p>
        </div>
        <router-link class="btn btn-secondary" :to="contractsRoute">
          <Icon name="document" size="sm" />
          <span>{{ t('admin.desktop.business.manageContracts') }}</span>
        </router-link>
      </header>

      <div v-if="contracts.length" class="table-responsive">
        <table class="w-full text-left text-sm">
          <thead class="text-xs text-gray-500 dark:text-dark-400">
            <tr class="border-b border-gray-200 dark:border-dark-700">
              <th class="px-3 py-2 font-medium">{{ t('admin.desktop.business.contract') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.desktop.business.contact') }}</th>
              <th class="px-3 py-2 text-right font-medium">{{ t('accounting.amount') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('common.status') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.desktop.business.validity') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="contract in contracts" :key="contract.id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
              <td class="px-3 py-3 align-top">
                <div class="font-medium text-gray-900 dark:text-white">{{ contract.details.name || '—' }}</div>
                <div class="mt-1 font-mono text-xs text-gray-500">{{ contract.details.contract_number || '—' }}</div>
              </td>
              <td class="px-3 py-3 align-top">{{ contract.details.contact_name || '—' }}</td>
              <td class="px-3 py-3 text-right align-top tabular-nums">{{ formatMoney(contract.amount, contract.currency) }}</td>
              <td class="px-3 py-3 align-top">{{ t(`accounting.statuses.${contract.status}`) }}</td>
              <td class="px-3 py-3 align-top text-gray-600 dark:text-dark-300">{{ validity(contract) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-else-if="!loading" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.business.noContracts') }}</p>
    </section>
    <OrganizationContactCreateDialog v-if="creatingContact" :key="organizationId" :organization-id="organizationId" @close="closeContact" @saved="contactCreated" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import OrganizationContactCreateDialog from '@/features/accounting/components/OrganizationContactCreateDialog.vue'
import * as accountingAPI from '@/features/accounting/api'
import { formatMoney } from '@/features/accounting/format'
import type { AccountingRecord, OrganizationContact } from '@/features/accounting/types'

const props = defineProps<{ organizationId: string }>()
const { t } = useI18n()
const contacts = ref<OrganizationContact[]>([])
const contracts = ref<AccountingRecord[]>([])
const loading = ref(false)
const error = ref('')
const creatingContact = ref(false)
const addContactButton = ref<HTMLButtonElement>()
const contractsRoute = computed(() => ({
  name: 'AdminOrganizationAccounting',
  params: { organizationId: props.organizationId },
  query: { tab: 'contracts' },
}))
function contactCreated() {
  closeContact()
  void loadBusiness()
}
function closeContact() {
  creatingContact.value = false
  void nextTick(() => addContactButton.value?.focus())
}
function editContactRoute(contact: OrganizationContact) {
  return {
    name: 'AdminAccountingOrganizationContactEdit',
    params: { organizationId: props.organizationId, contactId: contact.id },
  }
}

function message(value: unknown) {
  const candidate = value as { message?: string }
  return candidate.message || t('accounting.errors.generic')
}
function validity(contract: AccountingRecord) {
  const start = String(contract.details.effective_at || contract.details.signed_at || '')
  const end = String(contract.details.expires_at || '')
  return start && end ? `${start} - ${end}` : start || end || '—'
}
async function loadBusiness() {
  if (!props.organizationId) return
  loading.value = true
  error.value = ''
  try {
    const [contactResult, contractResult] = await Promise.all([
      accountingAPI.organizationContacts(props.organizationId),
      accountingAPI.listRecords('contracts', false, { organization_id: props.organizationId, page_size: 100 }),
    ])
    contacts.value = contactResult
    contracts.value = contractResult.items
  } catch (loadError) {
    error.value = message(loadError)
  } finally {
    loading.value = false
  }
}
async function removeContact(contact: OrganizationContact) {
  if (!window.confirm(t('admin.desktop.business.deleteContact', { name: contact.name }))) return
  try {
    await accountingAPI.deleteOrganizationContact(contact.id)
    await loadBusiness()
  } catch (deleteError) {
    error.value = message(deleteError)
  }
}

watch(() => props.organizationId, () => {
  creatingContact.value = false
  void loadBusiness()
})
onMounted(loadBusiness)
</script>
