<template>
  <AppLayout>
    <main class="accounting-page min-w-0 space-y-6" data-testid="accounting-order-page">
      <PageHeader :back="{ to: ordersRoute(), label: t('accounting.home.title') }">
        <template #title>{{ order?.name || t('accounting.home.order') }}</template>
        <template v-if="order" #badge>
          <span class="badge" :class="stateTone" data-testid="order-state">{{ t(`accounting.orders.states.${order.state}`) }}</span>
        </template>
        <template v-if="order" #subtitle>
          <router-link class="accounting-link" :to="ordersRoute({ organization: order.organization_id })">{{ order.organization_name }}</router-link>
          · {{ t('accounting.orders.createdOn', { date: formatDate(order.created_at) }) }}
          <template v-if="order.remark"> · {{ order.remark }}</template>
        </template>
        <template v-if="order && !panel" #actions>
          <template v-if="open">
            <button type="button" class="btn btn-secondary" data-testid="order-edit" @click="startPanel('edit')">{{ t('common.edit') }}</button>
            <button v-if="order.state === 'OPEN'" type="button" class="btn btn-secondary" data-testid="order-close" @click="startPanel('close')">{{ t('accounting.home.closeOrder') }}</button>
          </template>
          <button v-else type="button" class="btn btn-secondary" data-testid="order-reopen" @click="reopening = true">{{ t('accounting.home.reopenOrder') }}</button>
        </template>
      </PageHeader>

      <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
      <p v-if="notice" role="status" class="accounting-warning">{{ notice }}</p>
      <p v-if="loading && !order" role="status" class="card p-6 text-sm text-gray-500">{{ t('accounting.loading') }}</p>

      <template v-if="order">
        <form v-if="panel === 'edit'" class="card accounting-entry-form p-5" data-testid="order-edit-form" @submit.prevent="saveOrder">
          <label class="block min-w-0 sm:col-span-2 lg:col-span-3">
            <span class="input-label">{{ t('accounting.orders.name') }}</span>
            <input v-model="orderForm.name" class="input w-full" maxlength="200" required />
          </label>
          <label class="block min-w-0 sm:col-span-2 lg:col-span-3">
            <span class="input-label">{{ t('accounting.entries.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
            <textarea v-model="orderForm.remark" class="input min-h-16 w-full resize-y" maxlength="4000" />
          </label>
          <div class="accounting-entry-actions sm:col-span-2 lg:col-span-3">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="panel = ''">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">{{ t('common.save') }}</button>
          </div>
        </form>
        <form v-if="panel === 'close'" class="card accounting-entry-form p-5" data-testid="order-close-form" @submit.prevent="closeOrder">
          <p class="text-sm text-gray-600 dark:text-dark-200 sm:col-span-2 lg:col-span-3">{{ t('accounting.home.closeHint') }}</p>
          <label class="block min-w-0 sm:col-span-2 lg:col-span-3">
            <span class="input-label">{{ t('accounting.home.closeReason') }}</span>
            <textarea v-model="orderForm.reason" class="input min-h-16 w-full resize-y" maxlength="4000" required />
          </label>
          <div class="accounting-entry-actions sm:col-span-2 lg:col-span-3">
            <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="panel = ''">{{ t('common.cancel') }}</button>
            <button type="submit" class="btn btn-danger btn-sm" :disabled="busy">{{ t('accounting.home.closeOrder') }}</button>
          </div>
        </form>
        <p v-if="order.status === 'CLOSED' && order.close_reason" class="accounting-warning">{{ t('accounting.orders.closedReason', { reason: order.close_reason }) }}</p>

        <section class="card accounting-order-summary" data-testid="order-summary">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <p class="text-xs text-gray-500 dark:text-dark-300">{{ t('accounting.orders.nextStep') }}</p>
              <p class="mt-0.5 font-semibold text-gray-950 dark:text-white" data-testid="order-next-step">
                {{ nextStep ? t(`accounting.entries.next.${nextStep.key}`) : t(order.status === 'CLOSED' ? 'accounting.entries.next.closed' : 'accounting.orders.allDone') }}
              </p>
            </div>
            <button v-if="nextStep && open && !editor" type="button" class="btn btn-primary" data-testid="order-next-action" @click="nextStep.run()">
              {{ t(`accounting.entries.nextAction.${nextStep.key}`) }}
            </button>
          </div>
          <dl v-if="totals.length" class="accounting-order-totals" data-testid="order-totals">
            <div v-for="total in totals" :key="total.metric">
              <dt>{{ t(`accounting.orders.totals.${total.metric}`) }}</dt>
              <dd v-for="value in total.values" :key="value">{{ value }}</dd>
            </div>
          </dl>
        </section>

        <section
          v-for="(block, index) in blocks"
          :key="block.step"
          class="card accounting-order-step"
          :data-testid="`order-section-${block.step}`"
          :data-done="order.progress[block.step]"
        >
          <header class="accounting-order-step-header">
            <span class="accounting-step-marker" :data-state="stepState(block.step)" aria-hidden="true">
              <Icon v-if="order.progress[block.step]" name="check" size="sm" />
              <template v-else>{{ index + 1 }}</template>
            </span>
            <div class="min-w-0">
              <h2 class="accounting-section-title">{{ t(`accounting.orders.steps.${block.step}`) }}</h2>
              <p class="text-xs text-gray-500 dark:text-dark-300">{{ t(`accounting.orders.stepHints.${block.step}`) }}</p>
            </div>
          </header>

          <template v-for="kind in block.kinds" :key="kind">
            <p v-if="kind === 'vcc-cards' && negativeCards" class="accounting-warning">{{ t('accounting.orders.negativeCardHint') }}</p>
            <ul v-if="rows[kind].length" class="accounting-entries" :data-kind="kind">
              <template v-for="record in rows[kind]" :key="record.id">
                <li v-if="editor?.recordId === record.id" class="accounting-entry">
                  <EntryForm :kind="kind" :context="context" :record="record" @saved="saved" @cancel="closeEditor" @dirty="dirty = $event" />
                </li>
                <EntryRow
                  v-else
                  :kind="kind"
                  :record="record"
                  :cards="items['vcc-cards']"
                  :editable="open && !editor"
                  @edit="startEdit(kind, record)"
                  @remove="pendingRemoval = { kind, record }"
                  @confirm="confirmDraft(kind, record)"
                  @refund="startRefund(record)"
                  @changed="load"
                />
              </template>
            </ul>
            <div v-if="editor && !editor.recordId && editor.kind === kind" class="accounting-entry-new">
              <h3 class="text-sm font-semibold">{{ t(editor.preset?.transaction_type === 'REFUND' ? 'accounting.entries.addRefund' : `accounting.entries.add.${kind}`) }}</h3>
              <EntryForm :kind="kind" :context="context" :preset="editor.preset" @saved="saved" @cancel="closeEditor" @dirty="dirty = $event" />
            </div>
          </template>
          <p v-if="!block.kinds.some((kind) => rows[kind].length) && !(editor && block.kinds.includes(editor.kind))" class="text-sm text-gray-500">
            {{ t(`accounting.entries.empty.${block.step}`) }}
          </p>

          <div v-if="open && !editor" class="accounting-step-actions">
            <button
              v-for="kind in block.kinds"
              :key="kind"
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="kind === 'vcc-transactions' && !cardsReady"
              :title="kind === 'vcc-transactions' && !cardsReady ? t('accounting.entries.cardFirst') : undefined"
              :data-testid="`add-${kind}`"
              @click="startAdd(kind)"
            >
              <Icon name="plus" size="sm" aria-hidden="true" />{{ t(`accounting.entries.add.${kind}`) }}
            </button>
            <template v-if="block.link">
              <button v-if="linking !== block.link" type="button" class="btn btn-ghost btn-sm" :data-testid="`link-${block.link}`" @click="startLink(block.link)">
                {{ t('accounting.orders.linkExisting') }}
              </button>
              <form v-else class="flex flex-wrap items-center gap-2" @submit.prevent="link">
                <select v-model="linkTarget" class="input w-auto max-w-full" required :aria-label="t('accounting.orders.linkExisting')">
                  <option value="">{{ t(linkOptions.length ? 'accounting.form.select' : 'accounting.entries.nothingToLink') }}</option>
                  <option v-for="record in linkOptions" :key="record.id" :value="record.id">
                    {{ recordLabel(record.details, record.id) }} · {{ formatMoney(record.details.effective_amount ? String(record.details.effective_amount) : record.amount, record.currency) }}
                  </option>
                </select>
                <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !linkTarget">{{ t('accounting.entries.link') }}</button>
                <button type="button" class="btn btn-ghost btn-sm" @click="linking = ''">{{ t('common.cancel') }}</button>
              </form>
            </template>
          </div>
        </section>
      </template>

      <ConfirmDialog
        :show="reopening"
        :title="t('accounting.home.reopenOrder')"
        :message="t('accounting.confirmations.orderReopen')"
        :confirm-text="t('accounting.home.reopenOrder')"
        :loading="busy"
        @confirm="reopen"
        @cancel="reopening = false"
      />
      <ConfirmDialog
        :show="Boolean(pendingRemoval)"
        :title="t(pendingRemoval?.kind === 'vcc-cards' ? 'accounting.entries.closeCardTitle' : 'accounting.entries.removeTitle')"
        :message="t(pendingRemoval?.kind === 'vcc-cards' ? 'accounting.entries.closeCardMessage' : 'accounting.entries.removeMessage')"
        :confirm-text="t(pendingRemoval?.kind === 'vcc-cards' ? 'accounting.entries.closeCard' : 'common.delete')"
        :loading="busy"
        danger
        @confirm="remove"
        @cancel="pendingRemoval = null"
      />
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { Icon } from '@/components/icons'
import EntryForm from './components/order/EntryForm.vue'
import EntryRow from './components/order/EntryRow.vue'
import PageHeader from './components/PageHeader.vue'
import * as api from './api'
import { formatMoney } from './format'
import { activeCards, type EntryContext, type EntryValues } from './orderEntries'
import { recordLabel } from './schema'
import { errorMessage, formatDate, ordersRoute } from './shared'
import type { AccountingOrder, AccountingRecord, OrderItemKind, OrderStep, OrderTotal, ReceivingAccount, Vendor } from './types'
import '@/styles/accounting.css'

type LinkKind = 'transfers' | 'invoices' | 'vendor-invoices'
interface Block {
  step: OrderStep
  kinds: OrderItemKind[]
  link?: LinkKind
}
interface Editor {
  kind: OrderItemKind
  recordId?: string
  preset?: EntryValues
}

const { t, te } = useI18n()
const route = useRoute()
const orderId = computed(() => String(route.params.orderId))
const order = ref<AccountingOrder | null>(null)
const receivingAccounts = ref<ReceivingAccount[]>([])
const vendors = ref<Vendor[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const panel = ref<'' | 'edit' | 'close'>('')
const orderForm = reactive({ name: '', remark: '', reason: '' })
const reopening = ref(false)
const editor = ref<Editor | null>(null)
const dirty = ref(false)
const pendingRemoval = ref<{ kind: OrderItemKind; record: AccountingRecord } | null>(null)
const linking = ref<LinkKind | ''>('')
const linkTarget = ref('')
const linkOptions = ref<AccountingRecord[]>([])
const requestKey = api.stableRequest()

const steps: OrderStep[] = ['receipt', 'invoice', 'purchase', 'ai_invoice']
const blocks: Block[] = [
  { step: 'receipt', kinds: ['transfers'], link: 'transfers' },
  { step: 'invoice', kinds: ['invoices'], link: 'invoices' },
  { step: 'purchase', kinds: ['vcc-cards', 'vcc-transactions'] },
  { step: 'ai_invoice', kinds: ['vendor-invoices'], link: 'vendor-invoices' },
]
const itemKinds: OrderItemKind[] = ['transfers', 'invoices', 'vcc-cards', 'vcc-transactions', 'vendor-invoices']
const totalMetrics: (keyof Omit<OrderTotal, 'currency'>)[] = ['received', 'invoiced', 'spent', 'ai_cost']

const open = computed(() => order.value?.status === 'OPEN')
const items = computed(
  () => Object.fromEntries(itemKinds.map((kind) => [kind, order.value?.items?.[kind] || []])) as Record<OrderItemKind, AccountingRecord[]>,
)
// Removed entries stay in the books as void records but leave the order page.
const rows = computed(
  () => Object.fromEntries(itemKinds.map((kind) => [kind, items.value[kind].filter((record) => record.status !== 'VOID')])) as Record<OrderItemKind, AccountingRecord[]>,
)
const context = computed<EntryContext>(() => ({ order: order.value!, items: rows.value, receivingAccounts: receivingAccounts.value, vendors: vendors.value }))
const cardsReady = computed(() => Boolean(order.value) && activeCards(context.value).length > 0)
const negativeCards = computed(() => rows.value['vcc-cards'].some((card) => Number(card.details.available_balance) < 0))
const stateTone = computed(() => ({ OPEN: 'badge-warning', COMPLETED: 'badge-success', CLOSED: 'badge-gray' })[order.value?.state || 'OPEN'])
const totals = computed(() =>
  totalMetrics
    .map((metric) => ({
      metric,
      values: (order.value?.totals || []).filter((total) => Number(total[metric]) !== 0).map((total) => formatMoney(total[metric], total.currency)),
    }))
    .filter((entry) => entry.values.length),
)
// The first step still missing, and the one action that finishes it.
const nextStep = computed(() => {
  const current = order.value
  if (!current || current.status === 'CLOSED') return null
  const draft = (kind: OrderItemKind) => rows.value[kind].find((record) => record.status === 'DRAFT')
  const pending = (kind: OrderItemKind, key: string) => {
    const record = draft(kind)
    return record ? { key, run: () => void confirmDraft(kind, record) } : null
  }
  const add = (key: string, kind: OrderItemKind) => ({ key, run: () => startAdd(kind) })
  if (!current.progress.receipt) return pending('transfers', 'confirmReceipt') || add('receipt', 'transfers')
  if (!current.progress.invoice) return pending('invoices', 'confirmInvoice') || add('invoice', 'invoices')
  if (!current.progress.purchase) {
    if (!cardsReady.value) return add('card', 'vcc-cards')
    return pending('vcc-transactions', 'confirmSpend') || add('spend', 'vcc-transactions')
  }
  if (!current.progress.ai_invoice) {
    const some = rows.value['vendor-invoices'].some((record) => record.status === 'CONFIRMED')
    return pending('vendor-invoices', 'confirmAiInvoice') || add(some ? 'aiInvoiceMore' : 'aiInvoice', 'vendor-invoices')
  }
  return null
})
function stepState(step: OrderStep) {
  if (order.value?.progress[step]) return 'done'
  return steps.find((item) => !order.value?.progress[item]) === step ? 'current' : 'pending'
}

function discardEditor() {
  return !editor.value || !dirty.value || window.confirm(t('accounting.form.discardChanges'))
}
function openEditor(next: Editor) {
  if (!discardEditor()) return
  notice.value = ''
  dirty.value = false
  linking.value = ''
  editor.value = next
}
function startAdd(kind: OrderItemKind) {
  openEditor({ kind })
}
function startEdit(kind: OrderItemKind, record: AccountingRecord) {
  openEditor({ kind, recordId: record.id })
}
// A refund goes back to the card the purchase was paid with.
function startRefund(record: AccountingRecord) {
  openEditor({
    kind: 'vcc-transactions',
    preset: { parent_id: record.parent_id || '', vendor_id: String(record.details.vendor_id || ''), transaction_type: 'REFUND', original_transaction_id: record.id, amount: '' },
  })
}
function closeEditor() {
  editor.value = null
  dirty.value = false
}
async function saved(_record: AccountingRecord, uploadFailed: boolean) {
  closeEditor()
  await load()
  if (uploadFailed) notice.value = t('accounting.entries.uploadFailed')
}
function startPanel(next: 'edit' | 'close') {
  if (!order.value) return
  Object.assign(orderForm, { name: order.value.name, remark: order.value.remark, reason: '' })
  panel.value = next
}

async function run(path: string, payload: unknown, method: 'post' | 'patch' | 'delete' = 'post') {
  busy.value = true
  error.value = ''
  try {
    await api.mutate(path, payload, requestKey(`${method}:${path}`, payload), method)
    requestKey.reset()
    await load()
    return true
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
    return false
  } finally {
    busy.value = false
  }
}
async function saveOrder() {
  if (await run(`orders/${orderId.value}`, { name: orderForm.name.trim(), remark: orderForm.remark.trim(), version: order.value?.version }, 'patch')) panel.value = ''
}
async function closeOrder() {
  if (await run(`orders/${orderId.value}/close`, { reason: orderForm.reason.trim() })) panel.value = ''
}
async function reopen() {
  if (await run(`orders/${orderId.value}/reopen`, {})) reopening.value = false
}
async function confirmDraft(kind: OrderItemKind, record: AccountingRecord) {
  await run(`${kind}/${record.id}/confirm`, {})
}
async function remove() {
  const target = pendingRemoval.value
  if (!target) return
  await run(`orders/${orderId.value}/entries/${target.kind}/${target.record.id}`, {}, 'delete')
  pendingRemoval.value = null
}

// Records entered before orders existed can be brought into this one.
async function startLink(kind: LinkKind) {
  if (!discardEditor()) return
  closeEditor()
  linking.value = kind
  linkTarget.value = ''
  linkOptions.value = []
  try {
    const result = await api.listRecords(kind, false, { organization_id: order.value?.organization_id, page_size: 100 })
    linkOptions.value = result.items.filter(
      (record) => record.status !== 'VOID' && !record.details.order_id && !record.details.funding_type && (kind !== 'invoices' || record.details.invoice_type === 'SALES'),
    )
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}
async function link() {
  if (await run(`orders/${orderId.value}/links`, { object_kind: linking.value, object_id: linkTarget.value })) linking.value = ''
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [current, accounts, vendorList] = await Promise.all([
      api.order(orderId.value),
      receivingAccounts.value.length ? receivingAccounts.value : api.receivingAccounts(),
      vendors.value.length ? vendors.value : api.vendors(),
    ])
    vendors.value = vendorList
    order.value = current
    receivingAccounts.value = accounts
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    loading.value = false
  }
}

onBeforeRouteLeave(discardEditor)
onMounted(load)
</script>
