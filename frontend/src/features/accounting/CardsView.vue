<template>
  <AppLayout>
    <div class="accounting-page min-w-0 space-y-6" data-testid="accounting-cards">
      <AccountingNav current="AdminAccountingCards" />

      <div class="accounting-toolbar">
        <div class="w-full sm:w-56">
          <select v-model="filters.organization" class="input" :aria-label="t('accounting.home.organization')">
            <option value="">{{ t('accounting.home.allOrganizations') }}</option>
            <option v-for="organization in organizations" :key="organization.id" :value="organization.id">{{ organization.name }}</option>
          </select>
        </div>
        <div class="flex flex-wrap gap-1.5" role="group" :aria-label="t('accounting.status')">
          <button v-for="option in states" :key="option" type="button" class="accounting-chip" :aria-pressed="filters.status === option" @click="filters.status = option">
            {{ t(option ? `accounting.home.cardStates.${option}` : 'accounting.orders.allStates') }}
          </button>
        </div>
      </div>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <div v-if="rows.length" class="card overflow-x-auto">
        <table class="accounting-table">
          <thead>
            <tr>
              <th>{{ t('accounting.home.card') }}</th>
              <th>{{ t('accounting.home.organization') }}</th>
              <th class="text-right">{{ t('accounting.home.balance') }}</th>
              <th>{{ t('accounting.status') }}</th>
              <th class="text-right">{{ t('common.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="card in rows" :key="card.id">
              <tr>
                <td>
                  <p class="font-medium text-gray-900 dark:text-white">{{ cardLabel(card) }}</p>
                  <p class="mt-0.5 text-xs text-gray-500">{{ [card.details.provider, card.currency].filter(Boolean).join(' · ') }}</p>
                </td>
                <td>
                  <p>{{ organizationName(card.organization_id) }}</p>
                  <router-link v-if="card.details.order_id" class="accounting-link text-xs" :to="orderRoute(String(card.details.order_id))">
                    {{ card.details.order_name || t('accounting.home.order') }}
                  </router-link>
                </td>
                <td class="text-right tabular-nums" :class="{ 'text-amber-700 dark:text-amber-300': negative(card) }">
                  {{ formatMoney(String(card.details.available_balance || '0'), card.currency) }}
                </td>
                <td><span class="badge" :class="card.status === 'ACTIVE' ? 'badge-success' : 'badge-gray'">{{ t(`accounting.home.cardStates.${card.status}`) }}</span></td>
                <td class="text-right">
                  <div v-if="card.status === 'ACTIVE'" class="flex justify-end gap-3">
                    <button type="button" class="accounting-link text-sm" :aria-expanded="toppingUp === card.id" data-testid="card-top-up-open" @click="toppingUp = toppingUp === card.id ? '' : card.id">
                      {{ t('accounting.entries.topUp') }}
                    </button>
                    <button type="button" class="accounting-link text-sm text-red-600 dark:text-red-400" @click="closing = card">{{ t('accounting.entries.closeCard') }}</button>
                  </div>
                </td>
              </tr>
              <tr v-if="toppingUp === card.id">
                <td colspan="5" class="bg-gray-50/70 dark:bg-dark-800/40">
                  <CardTopUpForm :card="card" @saved="toppedUp" @cancel="toppingUp = ''" />
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
      <div v-else-if="!loading" class="card p-10 text-center">
        <p class="font-medium text-gray-900 dark:text-white">{{ t('accounting.home.noCards') }}</p>
        <p class="mt-2 text-sm text-gray-500">{{ t('accounting.home.noCardsHint') }}</p>
      </div>
      <p v-else role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>
      <Pagination v-if="total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />

      <ConfirmDialog
        :show="Boolean(closing)"
        :title="t('accounting.entries.closeCardTitle')"
        :message="t('accounting.entries.closeCardMessage')"
        :confirm-text="t('accounting.entries.closeCard')"
        :loading="busy"
        danger
        @confirm="closeCard"
        @cancel="closing = null"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import AccountingNav from './components/AccountingNav.vue'
import CardTopUpForm from './components/order/CardTopUpForm.vue'
import * as api from './api'
import { formatMoney } from './format'
import { compareMoney } from './money'
import { cardLabel } from './orderEntries'
import { errorMessage, orderRoute } from './shared'
import type { AccountingOrganization, AccountingRecord } from './types'
import '@/styles/accounting.css'

// Every card across organizations with its balance; cards are added inside orders, topped up and closed here.
const { t, te } = useI18n()
const states = ['', 'ACTIVE', 'CLOSED']
const pageSize = 20
const filters = reactive({ organization: '', status: 'ACTIVE' })
const page = ref(1)
const total = ref(0)
const rows = ref<AccountingRecord[]>([])
const organizations = ref<AccountingOrganization[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const toppingUp = ref('')
const closing = ref<AccountingRecord | null>(null)
const requestKey = api.stableRequest()

function organizationName(id: string) {
  return organizations.value.find((organization) => organization.id === id)?.name || id
}
function negative(card: AccountingRecord) {
  return compareMoney(String(card.details.available_balance || '0'), '0') < 0
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await api.listRecords('vcc-cards', false, { organization_id: filters.organization || undefined, status: filters.status || undefined, page: page.value, page_size: pageSize })
    rows.value = result.items
    total.value = result.total
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}
function toppedUp() {
  toppingUp.value = ''
  void load()
}
async function closeCard() {
  const card = closing.value
  if (!card) return
  busy.value = true
  try {
    const path = `vcc-cards/${card.id}/close`
    await api.mutate(path, {}, requestKey(path, {}))
    requestKey.reset()
    closing.value = null
    await load()
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}

watch(
  () => [filters.organization, filters.status],
  () => {
    page.value = 1
    void load()
  },
)
watch(page, load)
onMounted(async () => {
  void load()
  try {
    organizations.value = await api.organizations({})
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
})
</script>
