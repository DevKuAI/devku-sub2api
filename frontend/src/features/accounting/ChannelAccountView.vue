<template>
  <AppLayout>
    <div class="accounting-page min-w-0 space-y-6" data-testid="channel-account">
      <PageHeader
        :title="detail ? detail.account.name || detail.account.account_ref : t('accounting.channels.title')"
        :subtitle="detail ? `${detail.account.platform} · ${detail.account.account_ref}` : ''"
        :back="{ to: { name: 'AdminAccountingChannels' }, label: t('accounting.channels.detail.back') }"
      >
        <template #actions>
          <router-link class="btn btn-primary" :to="{ name: 'AdminAccountingChannelImport' }">
            <Icon name="upload" size="md" class="mr-1" aria-hidden="true" />{{ t('accounting.channels.importAction') }}
          </router-link>
        </template>
      </PageHeader>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="!detail && !error" role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>

      <template v-if="detail">
        <section class="card space-y-4 p-5">
          <h2 class="accounting-section-title">{{ t('accounting.channels.detail.flow') }}</h2>
          <WalletFlow :wallets="detail.wallets" />
          <dl class="channel-figures">
            <div>
              <dt>{{ t('accounting.channels.positions') }}</dt>
              <dd v-for="position in detail.positions" :key="position.currency" class="tabular-nums">{{ formatMoney(position.amount, position.currency) }}</dd>
            </div>
            <div v-if="detail.fee_costs.length">
              <dt>{{ t('accounting.channels.feeCosts') }}</dt>
              <dd v-for="fee in detail.fee_costs" :key="fee.currency" class="tabular-nums">{{ formatMoney(fee.amount, fee.currency) }}</dd>
            </div>
            <div>
              <dt>{{ t('accounting.channels.detail.cardSpend') }}</dt>
              <dd>{{ t('accounting.channels.cards', { count: detail.cards }) }}</dd>
            </div>
          </dl>
        </section>

        <section v-if="alerts.length || detail.missing_proofs" class="card border-amber-200 p-5 dark:border-amber-900/60" data-testid="channel-alerts">
          <h2 class="accounting-section-title">{{ t('accounting.channels.detail.alerts') }}</h2>
          <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-gray-700 dark:text-dark-200">
            <li v-for="alert in alerts" :key="alert">{{ alert }}</li>
            <li v-if="detail.missing_proofs">
              {{ t('accounting.channels.proofs.missingCount', { count: detail.missing_proofs }) }}
              <button type="button" class="accounting-link ml-2" data-testid="channel-missing-proofs" @click="filters.type = 'MISSING_PROOF'">{{ t('accounting.channels.proofs.showMissing') }}</button>
            </li>
          </ul>
        </section>

        <section v-if="detail.months.length" class="card overflow-x-auto">
          <h2 class="accounting-section-title px-5 pt-5">{{ t('accounting.channels.detail.months') }}</h2>
          <table class="accounting-table" data-testid="channel-months">
            <thead>
              <tr>
                <th>{{ t('accounting.channels.detail.month') }}</th>
                <th>{{ t('accounting.currency') }}</th>
                <th v-for="type in monthColumns" :key="type" class="text-right">{{ lineTypeLabel(type, t, te) }}</th>
                <th class="text-right">{{ t('accounting.channels.detail.feeCost') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="month in detail.months" :key="`${month.month}-${month.currency}`">
                <td class="tabular-nums">{{ month.month }}</td>
                <td><span class="badge badge-gray">{{ month.currency }}</span></td>
                <td v-for="type in monthColumns" :key="type" class="text-right tabular-nums">
                  {{ month.totals[type] ? formatMoney(month.totals[type], month.currency) : '—' }}
                </td>
                <td class="text-right tabular-nums">{{ formatMoney(month.fee_cost, month.currency) }}</td>
              </tr>
            </tbody>
          </table>
        </section>

        <div class="grid gap-6 xl:grid-cols-2">
          <section class="card overflow-x-auto">
            <h2 class="accounting-section-title px-5 pt-5">{{ t('accounting.channels.detail.conversions') }}</h2>
            <table v-if="detail.conversions.length" class="accounting-table">
              <thead>
                <tr>
                  <th>{{ t('accounting.channels.detail.occurredAt') }}</th>
                  <th class="text-right">{{ t('accounting.channels.detail.sold') }}</th>
                  <th class="text-right">{{ t('accounting.channels.detail.bought') }}</th>
                  <th class="text-right">{{ t('accounting.channels.detail.rate') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="conversion in detail.conversions" :key="conversion.reference">
                  <td>
                    <p>{{ formatDateTime(conversion.at) }}</p>
                    <p class="font-mono text-xs text-gray-500">{{ conversion.reference }}</p>
                  </td>
                  <td class="text-right tabular-nums">{{ formatMoney(conversion.sold, conversion.sold_currency) }}</td>
                  <td class="text-right tabular-nums">{{ formatMoney(conversion.bought, conversion.bought_currency) }}</td>
                  <td class="text-right tabular-nums">{{ conversion.rate }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="px-5 pb-5 pt-2 text-sm text-gray-500">{{ t('accounting.channels.detail.none') }}</p>
          </section>

          <section class="card overflow-x-auto">
            <h2 class="accounting-section-title px-5 pt-5">{{ t('accounting.channels.detail.cardSpend') }}</h2>
            <table v-if="detail.card_spend.length" class="accounting-table">
              <thead>
                <tr>
                  <th>{{ t('accounting.channels.detail.card') }}</th>
                  <th class="text-right">{{ t('accounting.channels.detail.spends') }}</th>
                  <th class="text-right">{{ t('accounting.channels.detail.spend') }}</th>
                  <th>{{ t('accounting.channels.detail.lastSpend') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="card in detail.card_spend" :key="`${card.id}-${card.currency}`" class="accounting-clickable-row" @click="filterCard(card.id)">
                  <td>
                    <p class="font-medium text-gray-900 dark:text-white">{{ card.card_name || card.card_ref }}</p>
                    <p class="text-xs text-gray-500">{{ card.card_last4 ? `**** ${card.card_last4}` : card.card_ref }}</p>
                  </td>
                  <td class="text-right tabular-nums">{{ card.spends }}</td>
                  <td class="text-right tabular-nums">
                    {{ formatMoney(card.spend, card.currency) }}
                    <span v-if="card.refund !== '0.00'" class="block text-xs text-gray-500">{{ t('accounting.channels.detail.refund') }} {{ formatMoney(card.refund, card.currency) }}</span>
                  </td>
                  <td>{{ card.last_at ? formatDate(card.last_at) : '—' }}</td>
                </tr>
              </tbody>
            </table>
            <p v-else class="px-5 pb-5 pt-2 text-sm text-gray-500">{{ t('accounting.channels.detail.none') }}</p>
          </section>
        </div>

        <section class="card space-y-4 p-5" data-testid="channel-lines">
          <h2 class="accounting-section-title">{{ t('accounting.channels.detail.lines') }}</h2>
          <div class="accounting-filters flex flex-wrap gap-3">
            <select v-model="filters.balance" class="input w-auto" :aria-label="t('accounting.channels.detail.allWallets')">
              <option value="">{{ t('accounting.channels.detail.allWallets') }}</option>
              <option v-for="wallet in detail.wallets" :key="`${wallet.wallet}/${wallet.currency}`" :value="`${wallet.wallet}/${wallet.currency}`">
                {{ walletLabel(wallet.wallet, t, te) }} · {{ wallet.currency }}
              </option>
            </select>
            <select v-model="filters.type" class="input w-auto" :aria-label="t('accounting.channels.detail.allTypes')">
              <option value="">{{ t('accounting.channels.detail.allTypes') }}</option>
              <option v-for="type in lineTypes" :key="type" :value="type">{{ lineTypeLabel(type, t, te) }}</option>
            </select>
            <input v-model="filters.from" type="date" class="input w-auto" :aria-label="t('accounting.list.from')" />
            <input v-model="filters.to" type="date" class="input w-auto" :aria-label="t('accounting.list.to')" />
            <input v-model.trim="filters.search" type="search" class="input min-w-48 flex-1" :placeholder="t('accounting.channels.detail.search')" />
            <button v-if="filters.card" type="button" class="accounting-chip" aria-pressed="true" @click="filters.card = ''">
              {{ cardName(filters.card) }} ×
            </button>
          </div>
          <div class="overflow-x-auto">
            <table v-if="lines.length" class="accounting-table">
              <thead>
                <tr>
                  <th>{{ t('accounting.channels.detail.occurredAt') }}</th>
                  <th>{{ t('accounting.channels.detail.detail') }}</th>
                  <th class="text-right">{{ t('accounting.amount') }}</th>
                  <th class="text-right">{{ t('accounting.channels.detail.balanceAfter') }}</th>
                  <th>{{ t('accounting.channels.detail.externalId') }}</th>
                  <th>{{ t('accounting.channels.proofs.title') }}</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="line in lines" :key="line.id">
                <tr>
                  <td class="whitespace-nowrap">
                    <p>{{ formatDateTime(line.occurred_at) }}</p>
                    <p class="text-xs text-gray-500">{{ walletLabel(line.wallet, t, te) }} · {{ line.currency }}</p>
                  </td>
                  <td>
                    <p class="font-medium text-gray-900 dark:text-white">{{ lineTypeLabel(line.type, t, te) }}<template v-if="line.card_name || line.card_last4"> · {{ line.card_name || `**** ${line.card_last4}` }}</template></p>
                    <p v-if="line.merchant" class="text-xs text-gray-500">{{ line.merchant }}</p>
                    <p v-if="line.remark" class="text-xs text-gray-500">{{ line.remark }}</p>
                    <p v-if="line.fee_amount" class="text-xs text-gray-500">{{ t('accounting.channels.detail.fee', { amount: formatMoney(line.fee_amount, line.currency) }) }}</p>
                    <p v-if="line.pair" class="text-xs text-gray-500">{{ t('accounting.channels.detail.pair', { wallet: `${walletLabel(line.pair.wallet, t, te)} ${line.pair.currency}`, id: line.pair.external_id }) }}</p>
                  </td>
                  <td class="text-right tabular-nums" :class="signedClass(line.amount)">{{ formatMoney(line.amount, line.currency) }}</td>
                  <td class="text-right tabular-nums">{{ line.balance_after ? formatMoney(line.balance_after, line.currency) : '—' }}</td>
                  <td class="font-mono text-xs text-gray-500">{{ line.external_id }}</td>
                  <td class="min-w-40">
                    <ul v-if="line.proofs.length" class="space-y-1">
                      <li v-for="proof in line.proofs" :key="proof.id" class="flex items-center gap-2 text-xs">
                        <button type="button" class="accounting-link max-w-40 truncate text-left" :title="proof.filename" @click="openProof(proof.id)">{{ proof.filename }}</button>
                        <button type="button" class="text-gray-400 hover:text-red-600" :aria-label="`${t('common.delete')} ${proof.filename}`" @click="removing = proof">×</button>
                      </li>
                    </ul>
                    <span v-else-if="line.type === 'FUNDING_IN'" class="badge badge-warning">{{ t('accounting.channels.proofs.missing') }}</span>
                    <button
                      type="button"
                      class="accounting-link mt-1 block text-xs"
                      :aria-expanded="uploading === line.id"
                      data-testid="channel-proof-open"
                      @click="uploading = uploading === line.id ? '' : line.id"
                    >
                      {{ t('accounting.channels.proofs.add') }}
                    </button>
                  </td>
                </tr>
                <tr v-if="uploading === line.id">
                  <td colspan="6" class="bg-gray-50/70 dark:bg-dark-800/40">
                    <ProofUploader :line-id="line.id" @uploaded="(proof) => proofAdded(line, proof)" @close="uploading = ''" />
                  </td>
                </tr>
                </template>
              </tbody>
            </table>
            <p v-else-if="!linesLoading" class="text-sm text-gray-500">{{ t('accounting.channels.detail.noLines') }}</p>
          </div>
          <Pagination v-if="linesTotal > pageSize" v-model:page="page" :total="linesTotal" :page-size="pageSize" :show-page-size-selector="false" />
        </section>

        <section class="card overflow-x-auto" data-testid="channel-imports">
          <h2 class="accounting-section-title px-5 pt-5">{{ t('accounting.channels.detail.imports') }}</h2>
          <table class="accounting-table">
            <thead>
              <tr>
                <th>{{ t('accounting.channels.detail.importedAt') }}</th>
                <th>{{ t('accounting.channels.detail.converter') }}</th>
                <th>{{ t('accounting.channels.detail.files') }}</th>
                <th>{{ t('accounting.status') }}</th>
                <th class="text-right">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="batch in detail.imports" :key="batch.id">
                <tr>
                  <td class="whitespace-nowrap">
                    <p>{{ formatDateTime(batch.created_at) }}</p>
                    <p class="text-xs text-gray-500">{{ t('accounting.channels.detail.rows', { new: batch.new_rows, duplicate: batch.duplicate_rows }) }}</p>
                  </td>
                  <td>{{ batch.converter }}</td>
                  <td>
                    <button v-for="(file, index) in batch.files" :key="file.sha256 + index" type="button" class="accounting-link block text-left text-xs" @click="download(batch.id, index)">
                      {{ file.name }}
                    </button>
                  </td>
                  <td>
                    <span class="badge" :class="batch.status === 'APPLIED' ? 'badge-success' : 'badge-gray'">{{ t(`accounting.channels.detail.status.${batch.status}`) }}</span>
                    <p v-if="batch.revoke_reason" class="mt-1 text-xs text-gray-500">{{ t('accounting.channels.detail.revokedWith', { reason: batch.revoke_reason }) }}</p>
                  </td>
                  <td class="text-right">
                    <button
                      v-if="isAdmin && batch.status === 'APPLIED'"
                      type="button"
                      class="accounting-link text-sm text-red-600 dark:text-red-400"
                      :aria-expanded="revoking === batch.id"
                      data-testid="channel-revoke-open"
                      @click="openRevoke(batch.id)"
                    >
                      {{ t('accounting.channels.detail.revoke') }}
                    </button>
                  </td>
                </tr>
                <tr v-if="revoking === batch.id">
                  <td colspan="5" class="bg-gray-50/70 dark:bg-dark-800/40">
                    <form class="space-y-3" data-testid="channel-revoke-form" @submit.prevent="revoke(batch.id)">
                      <label class="block">
                        <span class="input-label">{{ t('accounting.channels.detail.revokeReason') }}</span>
                        <textarea v-model.trim="revokeReason" class="input" rows="2" maxlength="1000" required />
                      </label>
                      <p class="text-xs text-gray-500">{{ t('accounting.channels.detail.revokeHint') }}</p>
                      <div class="flex gap-3">
                        <button type="submit" class="btn btn-danger" :disabled="!revokeReason || busy">{{ t('accounting.channels.detail.revokeConfirm') }}</button>
                        <button type="button" class="btn btn-secondary" :disabled="busy" @click="revoking = ''">{{ t('common.cancel') }}</button>
                      </div>
                    </form>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </section>
      </template>

      <ConfirmDialog
        :show="Boolean(removing)"
        :title="t('accounting.channels.proofs.deleteTitle')"
        :message="t('accounting.channels.proofs.deleteMessage', { name: removing?.filename || '' })"
        :confirm-text="t('common.delete')"
        :loading="busy"
        danger
        @confirm="removeProof"
        @cancel="removing = null"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { Icon } from '@/components/icons'
import { useAuthStore } from '@/stores/auth'
import PageHeader from './components/PageHeader.vue'
import ProofUploader from './components/channel/ProofUploader.vue'
import WalletFlow from './components/channel/WalletFlow.vue'
import * as api from './api'
import { lineTypeLabel, signedClass, walletLabel } from './channelShared'
import { formatMoney } from './format'
import { errorMessage, formatDate, formatDateTime, isCanceled } from './shared'
import type { ChannelAccountDetail, ChannelLine, ChannelLineType, ChannelProof } from './types'
import '@/styles/accounting.css'

// One channel account: its funds flow, what needs attention, monthly totals and the ledger behind them.
const { t, te } = useI18n()
const route = useRoute()
const auth = useAuthStore()
const isAdmin = computed(() => auth.isAdmin)
const typeOrder: ChannelLineType[] = ['FUNDING_IN', 'FUNDING_OUT', 'FX_OUT', 'FX_IN', 'TRANSFER_OUT', 'TRANSFER_IN', 'CONSUME', 'REFUND', 'FEE', 'ADJUSTMENT']
const lineTypes = [...typeOrder, 'MISSING_PROOF', 'UNPAIRED_TRANSFER']
const pageSize = 50
const detail = ref<ChannelAccountDetail | null>(null)
const lines = ref<ChannelLine[]>([])
const linesTotal = ref(0)
const linesLoading = ref(false)
const page = ref(1)
const filters = reactive({ balance: '', type: '', from: '', to: '', search: '', card: '' })
const error = ref('')
const busy = ref(false)
const revoking = ref('')
const revokeReason = ref('')
const uploading = ref('')
const removing = ref<ChannelProof | null>(null)
const requestKey = api.stableRequest()
let linesRequest: AbortController | null = null

const accountId = computed(() => String(route.params.accountId))
const monthColumns = computed(() => typeOrder.filter((type) => detail.value?.months.some((month) => month.totals[type])))
const alerts = computed(() => {
  const d = detail.value
  if (!d) return []
  const out: string[] = []
  for (const wallet of d.wallets) {
    const name = walletLabel(wallet.wallet, t, te)
    for (const gap of wallet.gaps) {
      out.push(t('accounting.channels.detail.gapItem', { wallet: name, currency: wallet.currency, after: gap.after_external_id, before: gap.before_external_id, difference: formatMoney(gap.difference, wallet.currency) }))
    }
    for (const span of wallet.negative_spans) {
      out.push(t('accounting.channels.detail.negativeItem', {
        wallet: name,
        currency: wallet.currency,
        from: formatDate(span.from),
        to: span.to ? formatDate(span.to) : t('accounting.channels.detail.negativeOngoing'),
        lowest: formatMoney(span.lowest, wallet.currency),
      }))
    }
    if (wallet.reported && !wallet.reported.matched) {
      out.push(`${name} ${wallet.currency}：${t('accounting.channels.difference', { amount: formatMoney(wallet.reported.difference, wallet.currency) })}`)
    }
  }
  for (const line of d.unpaired) {
    out.push(t('accounting.channels.detail.unpairedItem', { wallet: walletLabel(line.wallet, t, te), currency: line.currency, type: lineTypeLabel(line.type, t, te), amount: formatMoney(line.amount, line.currency), id: line.external_id }))
  }
  return out
})

function cardName(id: string) {
  const card = detail.value?.card_spend.find((item) => item.id === id)
  return card ? card.card_name || card.card_ref : id
}
function filterCard(id: string) {
  filters.card = id
}
async function load() {
  error.value = ''
  try {
    detail.value = await api.channelAccount(accountId.value)
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}
async function loadLines() {
  linesRequest?.abort()
  const controller = new AbortController()
  linesRequest = controller
  linesLoading.value = true
  const [wallet = '', currency = ''] = filters.balance.split('/')
  try {
    const result = await api.channelLines(
      accountId.value,
      {
        wallet: wallet || undefined,
        currency: currency || undefined,
        type: filters.type || undefined,
        card_id: filters.card || undefined,
        from: filters.from || undefined,
        to: filters.to || undefined,
        search: filters.search || undefined,
        page: page.value,
        page_size: pageSize,
      },
      controller.signal,
    )
    lines.value = result.items
    linesTotal.value = result.total
  } catch (cause) {
    if (!isCanceled(cause)) error.value = errorMessage(cause, t, te)
  } finally {
    if (linesRequest === controller) linesLoading.value = false
  }
}
function openRevoke(id: string) {
  revoking.value = revoking.value === id ? '' : id
  revokeReason.value = ''
}
async function revoke(id: string) {
  busy.value = true
  error.value = ''
  try {
    const path = `channel-imports/${id}/revoke`
    await api.revokeChannelImport(id, revokeReason.value, requestKey(path, revokeReason.value))
    requestKey.reset()
    revoking.value = ''
    await Promise.all([load(), loadLines()])
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
function proofAdded(line: ChannelLine, proof: ChannelProof) {
  line.proofs.push(proof)
  if (line.type === 'FUNDING_IN' && line.proofs.length === 1 && detail.value) detail.value.missing_proofs = Math.max(0, detail.value.missing_proofs - 1)
}
async function openProof(id: string) {
  try {
    window.open(await api.channelProofURL(id), '_blank', 'noopener')
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}
async function removeProof() {
  const proof = removing.value
  if (!proof) return
  busy.value = true
  try {
    await api.deleteChannelProof(proof.id, requestKey(`channel-proofs/${proof.id}`, {}))
    requestKey.reset()
    removing.value = null
    await Promise.all([load(), loadLines()])
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
async function download(id: string, index: number) {
  try {
    window.open(await api.channelFileURL(id, index), '_blank', 'noopener')
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}

watch(
  () => [filters.balance, filters.type, filters.from, filters.to, filters.search, filters.card],
  () => {
    if (page.value !== 1) page.value = 1
    else void loadLines()
  },
)
watch(page, loadLines)
onMounted(() => {
  void load()
  void loadLines()
})
</script>
