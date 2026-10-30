<template>
  <AppLayout>
    <div class="accounting-page min-w-0 space-y-6" data-testid="accounting-vendors">
      <AccountingNav current="AdminAccountingVendors" />
      <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('accounting.vendors.description') }}</p>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="loading" role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>

      <template v-else>
        <section v-for="kind in kinds" :key="kind" class="card overflow-x-auto" :data-testid="`vendors-${kind}`">
          <header class="flex items-center justify-between gap-4 px-5 pt-5">
            <div>
              <h2 class="accounting-section-title">{{ t(`accounting.vendors.kinds.${kind}`) }}</h2>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-dark-400">{{ t(`accounting.vendors.hints.${kind}`) }}</p>
            </div>
            <router-link class="btn btn-primary btn-sm" :to="{ name: 'AdminAccountingVendorNew', query: { kind } }">
              <Icon name="plus" size="sm" class="mr-1" aria-hidden="true" />{{ t(`accounting.vendors.add.${kind}`) }}
            </router-link>
          </header>
          <table v-if="listOf(kind).length" class="accounting-table mt-3">
            <thead>
              <tr>
                <th>{{ t('accounting.vendors.name') }}</th>
                <th>{{ t('accounting.vendors.aliases') }}</th>
                <th>{{ t('accounting.status') }}</th>
                <th class="text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="vendor in listOf(kind)" :key="vendor.id" class="accounting-clickable-row" @click="edit(vendor)">
                <td class="font-medium text-gray-900 dark:text-white">{{ vendor.name }}</td>
                <td class="text-xs text-gray-500">{{ vendor.aliases.join('、') || '—' }}</td>
                <td>
                  <span class="badge" :class="vendor.status === 'ACTIVE' ? 'badge-success' : 'badge-gray'">
                    {{ t(vendor.status === 'ACTIVE' ? 'accounting.vendors.active' : 'accounting.vendors.disabled') }}
                  </span>
                </td>
                <td class="text-right">
                  <router-link class="accounting-link text-sm" :to="editRoute(vendor)" @click.stop>{{ t('common.edit') }}</router-link>
                </td>
              </tr>
            </tbody>
          </table>
          <p v-else class="px-5 pb-5 pt-3 text-sm text-gray-500">{{ t('accounting.vendors.empty') }}</p>
        </section>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { Icon } from '@/components/icons'
import AccountingNav from './components/AccountingNav.vue'
import * as api from './api'
import { errorMessage } from './shared'
import type { Vendor, VendorKind } from './types'
import '@/styles/accounting.css'

// The lists records choose from: card providers issue our cards, AI vendors are who we buy from.
const { t, te } = useI18n()
const router = useRouter()
const kinds: VendorKind[] = ['CARD_PROVIDER', 'AI_VENDOR']
const vendors = ref<Vendor[]>([])
const loading = ref(true)
const error = ref('')

function listOf(kind: VendorKind) {
  return vendors.value.filter((vendor) => vendor.kind === kind)
}
function editRoute(vendor: Vendor) {
  return { name: 'AdminAccountingVendorEdit', params: { vendorId: vendor.id } }
}
function edit(vendor: Vendor) {
  void router.push(editRoute(vendor))
}
onMounted(async () => {
  try {
    vendors.value = await api.vendors()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
})
</script>
