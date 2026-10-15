<template>
  <section
    class="space-y-4"
    :aria-busy="loading"
    data-testid="accounting-operations"
  >
    <header class="accounting-record-heading">
      <div>
        <h2 class="text-base font-semibold">{{ title }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">{{ hint }}</p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-secondary" :disabled="loading" @click="load">
          {{ t('accounting.refresh') }}</button
        ><button
          v-if="section === 'costs' && !customer"
          class="btn btn-primary"
          @click="open('cost')"
        >
          {{ t('accounting.ops.allocateCost') }}</button
        ><button
          v-if="section === 'periods' && administrator && !customer"
          class="btn btn-primary"
          @click="open('close')"
        >
          {{ t('accounting.ops.closePeriod') }}
        </button>
      </div>
    </header>
    <div v-if="error" role="alert" class="accounting-error">
      <p>{{ error }}</p>
      <button class="btn btn-secondary mt-2" @click="load">
        {{ t('accounting.refresh') }}
      </button>
    </div>
    <template v-if="record">
      <div class="flex flex-wrap gap-2">
        <button
          v-if="
            kind === 'receivables' &&
            record.status === 'CONFIRMED' &&
            positiveMoney(record.details.outstanding_amount || record.amount)
          "
          class="btn btn-primary"
          @click="open('settle')"
        >
          {{ t('accounting.ops.settle') }}</button
        ><button
          v-if="
            kind === 'fx-exchanges' &&
            record.status === 'CONFIRMED' &&
            positiveMoney(record.details.target_available)
          "
          class="btn btn-primary"
          @click="open('fx-allocate')"
        >
          {{ t('accounting.allocate') }}</button
        ><button
          v-if="
            kind === 'vendor-invoices' &&
            record.status === 'CONFIRMED' &&
            positiveMoney(record.details.unallocated_cost || record.amount)
          "
          class="btn btn-secondary"
          @click="open('cost')"
        >
          {{ t('accounting.ops.allocateCost') }}
        </button>
      </div>
      <div v-if="related.length" class="card p-4">
        <h3 class="font-medium">{{ t('accounting.ops.related') }}</h3>
        <div class="mt-3 space-y-2">
          <button
            v-for="row in related"
            :key="`${row.kind}-${row.id}`"
            class="block w-full rounded-lg border border-gray-200 p-3 text-start text-sm dark:border-dark-600"
            @click="
              emit('open-record', String(row.kind) as AccountingKind, row.id)
            "
          >
            {{ t(`accounting.kinds.${row.kind}`) }} · {{ row.name }} ·
            {{ formatMoney(String(row.amount), String(row.currency)) }}
          </button>
        </div>
      </div>
      <div v-if="settlements.length" class="card p-4">
        <h3 class="font-medium">{{ t('accounting.ops.settlementHistory') }}</h3>
        <div
          v-for="row in settlements"
          :key="row.id"
          class="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm"
        >
          <span
            >{{ row.reference }} ·
            {{ formatMoney(String(row.amount), String(row.currency)) }} ·
            {{ statusText(row) }}</span
          ><button
            v-if="row.status === 'ACTIVE'"
            class="btn btn-secondary"
            @click="open('void-settlement', row)"
          >
            {{ t('accounting.void') }}
          </button>
        </div>
      </div>
      <div class="card p-4">
        <h3 class="font-medium">{{ t('accounting.ops.timeline') }}</h3>
        <ol class="mt-4 space-y-4">
          <li
            v-for="row in activity"
            :key="row.id"
            class="min-w-0 border-s-2 border-gray-200 ps-4 dark:border-dark-600"
          >
            <p class="text-sm font-medium">
              {{ t(`accounting.ops.actions.${row.action}`) }} ·
              {{ new Date(String(row.created_at)).toLocaleString() }}
            </p>
            <p class="mt-1 text-xs text-gray-500">
              {{ t('accounting.actor') }} {{ row.actor_id }} ·
              {{ row.request_id }}
            </p>
            <p v-if="row.summary" class="mt-2 break-words text-sm">
              {{ activityText(row) }}
            </p>
          </li>
        </ol>
        <p
          v-if="!activity.length && !loading"
          class="mt-3 text-sm text-gray-500"
        >
          {{ t('accounting.empty') }}
        </p>
      </div>
    </template>
    <template v-else-if="section === 'workbench' && workbench">
      <div class="grid gap-3 sm:grid-cols-2">
        <div
          v-for="(position, currency) in workbench.positions"
          :key="currency"
          class="card p-4"
        >
          <h3 class="font-medium">{{ currency }}</h3>
          <dl class="mt-3 space-y-2 text-sm">
            <div>
              <dt>{{ t('accounting.metrics.unallocated') }}</dt>
              <dd>{{ formatMoney(position.cash, String(currency)) }}</dd>
            </div>
            <div>
              <dt>{{ t('accounting.ops.bookBalance') }}</dt>
              <dd>{{ formatMoney(position.card, String(currency)) }}</dd>
            </div>
          </dl>
        </div>
      </div>
      <p
        v-for="pending in workbench.pending_consumption"
        :key="pending.currency"
        class="text-sm text-amber-700 dark:text-amber-300"
      >
        {{ t('accounting.ops.pendingUsage') }} ·
        {{ formatMoney(pending.amount, pending.currency) }}
      </p>
      <div class="card p-4">
        <label
          ><span class="input-label">{{ t('accounting.ops.todoType') }}</span
          ><select v-model="todoReason" class="input">
            <option value="">
              {{ t('common.all') }} · {{ workbench.total }}
            </option>
            <option
              v-for="(count, reason) in workbench.counts"
              :key="reason"
              :value="reason"
            >
              {{ t(`accounting.ops.reasons.${reason}`) }} · {{ count }}
            </option>
          </select></label
        >
        <p
          v-if="
            (workbench.matched_total ?? workbench.total) >
            workbench.items.length
          "
          class="mt-3 text-sm text-gray-500"
        >
          {{
            t('accounting.ops.todoLimit', {
              count: workbench.items.length,
              total: workbench.matched_total ?? workbench.total,
            })
          }}
        </p>
        <div class="mt-4 space-y-3">
          <button
            v-for="row in todos"
            :key="`${row.kind}-${row.id}-${row.reason}`"
            class="block w-full rounded-lg border border-gray-200 p-3 text-start dark:border-dark-600"
            @click="openTodo(row)"
          >
            <span class="block text-sm font-medium"
              >{{ row.name }} ·
              {{ formatMoney(row.amount, row.currency) }}</span
            ><span class="mt-1 block text-xs text-gray-500">{{
              t(`accounting.ops.reasons.${row.reason}`)
            }}</span>
          </button>
        </div>
        <p v-if="!todos.length" class="mt-3 text-sm text-gray-500">
          {{ t('accounting.ops.noTodo') }}
        </p>
      </div>
    </template>
    <template v-else>
      <div v-if="section === 'costs'" class="grid gap-3 sm:grid-cols-2">
        <div v-for="cost in costSummary" :key="cost.id" class="card p-4">
          <h3 class="font-medium">{{ cost.currency }}</h3>
          <dl class="mt-3 space-y-2 text-sm">
            <div
              v-for="field in [
                'book_cost',
                'invoice_total',
                'allocated_cost',
                'unallocated_cost',
              ]"
              :key="field"
            >
              <dt>{{ t(`accounting.ops.${field}`) }}</dt>
              <dd>
                {{ formatMoney(String(cost[field]), String(cost.currency)) }}
              </dd>
            </div>
          </dl>
        </div>
      </div>
      <div
        v-for="row in rows"
        :key="row.id"
        class="card flex flex-wrap items-center justify-between gap-3 p-4"
      >
        <div class="min-w-0 text-sm">
          <p class="break-words font-medium">{{ rowName(row) }}</p>
          <p
            v-if="section === 'costs' && row.source_amount"
            class="mt-2 text-sm text-gray-500"
          >
            {{ t('accounting.ops.sourceAmount') }}:
            {{
              formatMoney(
                String(row.source_amount),
                String(row.source_currency),
              )
            }}
            · {{ t('accounting.fields.exchange_rate') }}
            {{
              String(row.exchange_rate)
                .replace(/(\.\d*?)0+$/, '$1')
                .replace(/\.$/, '')
            }}
          </p>
          <p class="mt-2 text-gray-500">
            {{
              row.currency
                ? formatMoney(String(row.amount), String(row.currency))
                : ''
            }}
            ·
            {{
              section === 'periods'
                ? t(
                    row.status === 'CLOSED'
                      ? 'accounting.ops.periodClosedLabel'
                      : 'accounting.ops.periodOpenLabel',
                  )
                : statusText(row)
            }}
            <span v-if="row.revision"
              >· {{ t('accounting.ops.revision') }} {{ row.revision }}</span
            >
          </p>
          <p v-if="row.reason" class="mt-2 break-words text-gray-500">
            {{ row.reason }}
          </p>
          <p v-if="row.reference" class="mt-2 break-words text-gray-500">
            {{ row.reference }}
          </p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            v-if="section === 'external' && row.status === 'PENDING'"
            class="btn btn-primary"
            @click="open('external', row)"
          >
            {{ t('accounting.ops.confirmExternal') }}</button
          ><button
            v-if="
              section === 'external' &&
              row.status === 'PENDING' &&
              administrator &&
              !customer
            "
            class="btn btn-secondary"
            @click="open('external-exempt', row)"
          >
            {{ t('accounting.ops.exemptExternal') }}</button
          ><button
            v-if="section === 'costs' && row.status === 'ACTIVE'"
            class="btn btn-secondary"
            @click="open('void-cost', row)"
          >
            {{ t('accounting.void') }}</button
          ><template v-if="section === 'periods'"
            ><button
              v-if="row.statement_id"
              class="btn btn-secondary"
              @click="readStatement(String(row.statement_id))"
            >
              {{ t('accounting.details') }}</button
            ><button
              v-if="row.statement_id"
              class="btn btn-secondary"
              @click="downloadStatement(String(row.statement_id))"
            >
              {{ t('accounting.export') }}</button
            ><button
              v-if="row.status === 'CLOSED' && administrator && !customer"
              class="btn btn-secondary"
              @click="open('reopen', row)"
            >
              {{ t('accounting.ops.reopen') }}
            </button></template
          >
        </div>
      </div>
      <p v-if="!rows.length && !loading" class="card p-5 text-sm text-gray-500">
        {{ t('accounting.empty') }}
      </p>
    </template>
    <BaseDialog
      :show="!!action"
      :title="action ? t(`accounting.ops.dialogs.${action}`) : ''"
      :close-label="t('common.close')"
      :close-on-escape="!busy"
      :show-close-button="!busy"
      width="wide"
      @close="!busy && (action = '')"
    >
      <form :id="formID" class="space-y-4" @submit.prevent="submit">
        <p class="accounting-context">{{ organizationName }}</p>
        <p v-if="dialogError" role="alert" class="accounting-error">
          {{ dialogError }}
        </p>
        <template v-if="action === 'settle'"
          ><label class="block"
            ><span class="input-label">{{
              t('accounting.kinds.transfers')
            }}</span
            ><select v-model="form.transfer_id" class="input w-full" required>
              <option value="">{{ t('accounting.select') }}</option>
              <option v-for="r in transfers" :key="r.id" :value="r.id">
                {{ recordLabel(r.details, r.id) }} ·
                {{
                  r.details.bank_reference ||
                  r.details.invoice_number ||
                  r.details.period_start ||
                  ''
                }}
                · {{ formatMoney(r.amount, r.currency) }}
              </option>
            </select></label
          ></template
        >
        <template v-if="action === 'cost'"
          ><label class="block"
            ><span class="input-label">{{
              t('accounting.kinds.vendor-invoices')
            }}</span
            ><select v-model="form.invoice_id" class="input w-full" required>
              <option value="">{{ t('accounting.select') }}</option>
              <option v-for="r in invoices" :key="r.id" :value="r.id">
                {{ recordLabel(r.details, r.id) }} ·
                {{
                  r.details.bank_reference ||
                  r.details.invoice_number ||
                  r.details.period_start ||
                  ''
                }}
                · {{ formatMoney(r.amount, r.currency) }}
              </option>
            </select></label
          ><label class="block"
            ><span class="input-label">{{
              t('accounting.ops.sourceKind')
            }}</span
            ><select
              v-model="form.source_kind"
              class="input w-full"
              @change="form.source_id = ''"
            >
              <option value="vcc-transactions">
                {{ t('accounting.kinds.vcc-transactions') }}
              </option>
              <option value="expenses">
                {{ t('accounting.kinds.expenses') }}
              </option>
            </select></label
          ><label class="block"
            ><span class="input-label">{{
              t('accounting.referenceRecord')
            }}</span
            ><select v-model="form.source_id" class="input w-full" required>
              <option value="">{{ t('accounting.select') }}</option>
              <option v-for="r in costSources" :key="r.id" :value="r.id">
                {{ recordLabel(r.details, r.id) }} ·
                {{
                  formatMoney(
                    String(r.details.effective_amount || r.amount),
                    r.currency,
                  )
                }}
              </option>
            </select></label
          ><label v-if="crossCurrencyCost" class="block"
            ><span class="input-label"
              >{{ t('accounting.ops.sourceAmount') }} ·
              {{ selectedSource?.currency }}</span
            ><input
              v-model="form.source_amount"
              class="input w-full"
              inputmode="decimal"
              required /></label
          ><label v-if="crossCurrencyCost" class="block"
            ><span class="input-label">{{
              t('accounting.fields.exchange_rate')
            }}</span
            ><input
              v-model="form.exchange_rate"
              class="input w-full"
              inputmode="decimal"
              required
          /></label>
          <p class="text-xs text-gray-500">
            {{ t('accounting.ops.costHint') }}
          </p></template
        >
        <template v-if="action === 'fx-allocate'"
          ><label class="block"
            ><span class="input-label">{{
              t('accounting.kinds.vcc-cards')
            }}</span
            ><select v-model="form.source_id" class="input w-full" required>
              <option value="">{{ t('accounting.select') }}</option>
              <option v-for="r in cards" :key="r.id" :value="r.id">
                {{ recordLabel(r.details, r.id) }} · {{ r.currency }}
              </option>
            </select></label
          ></template
        >
        <label v-if="['settle', 'cost'].includes(action)" class="block"
          ><span class="input-label">{{ t('accounting.occurredAt') }}</span
          ><input
            v-model="businessDate"
            class="input w-full"
            type="datetime-local"
            required
        /></label>
        <label
          v-if="['settle', 'cost', 'fx-allocate'].includes(action)"
          class="block"
          ><span class="input-label">{{ t('accounting.amount') }}</span
          ><input
            v-model="form.amount"
            class="input w-full"
            inputmode="decimal"
            required
        /></label>
        <template v-if="action === 'external'"
          ><p class="text-sm text-gray-500">
            {{ t('accounting.ops.externalHint') }}
          </p>
          <label class="block"
            ><span class="input-label">{{ t('accounting.reference') }}</span
            ><input
              v-model="form.reference"
              class="input w-full"
              required /></label
          ><label class="block"
            ><span class="input-label">{{ t('accounting.files') }}</span
            ><select v-model="form.attachment_id" class="input w-full" required>
              <option value="">{{ t('accounting.select') }}</option>
              <option v-for="file in files" :key="file.id" :value="file.id">
                {{ file.filename }}
              </option>
            </select></label
          >
          <p class="text-xs text-gray-500">
            {{ t('accounting.ops.externalProof') }}
          </p></template
        >
        <template v-if="action === 'close'"
          ><label class="block"
            ><span class="input-label">{{ t('accounting.ops.month') }}</span
            ><input
              v-model="form.month"
              type="month"
              class="input w-full"
              required
          /></label>
          <p class="text-sm text-gray-500">
            {{ t('accounting.ops.closeHint') }}
          </p>
          <label class="flex items-start gap-3"
            ><input
              v-model="form.accept_partial"
              type="checkbox"
              class="mt-1"
            /><span class="text-sm">{{
              t('accounting.ops.partialHint')
            }}</span></label
          ></template
        >
        <label
          v-if="
            [
              'close',
              'reopen',
              'void-cost',
              'void-settlement',
              'external-exempt',
            ].includes(action)
          "
          class="block"
          ><span class="input-label">{{ t('accounting.reason') }}</span
          ><textarea
            v-model="form.reason"
            class="input min-h-24 w-full"
            required
          />
        </label>
      </form>
      <template #footer
        ><div class="flex justify-end gap-2">
          <button
            class="btn btn-secondary"
            :disabled="busy"
            @click="action = ''"
          >
            {{ t('accounting.cancel') }}</button
          ><button
            class="btn btn-primary"
            :form="formID"
            type="submit"
            :disabled="busy || dialogLoading"
          >
            {{ t('accounting.save') }}
          </button>
        </div></template
      >
    </BaseDialog>
    <BaseDialog
      :show="!!statement"
      :title="t('accounting.ops.statement')"
      :close-label="t('common.close')"
      width="extra-wide"
      @close="statement = null"
      ><div v-if="statement" class="space-y-4">
        <p class="font-semibold">{{ statement.month }}</p>
        <p
          v-if="!statement.opening"
          class="text-sm text-amber-700 dark:text-amber-300"
        >
          {{ t('accounting.ops.unknownOpening') }}
        </p>
        <div
          v-for="(closing, currency) in statement.closing"
          :key="currency"
          class="card p-4"
        >
          <h3 class="font-medium">{{ currency }}</h3>
          <dl class="mt-3 grid gap-4 sm:grid-cols-2">
            <div>
              <dt>{{ t('accounting.ops.openingCash') }}</dt>
              <dd>
                {{
                  statement.opening
                    ? formatMoney(
                        statement.opening[currency]?.cash || '0',
                        String(currency),
                      )
                    : t('accounting.ops.unknown')
                }}
              </dd>
            </div>
            <div>
              <dt>{{ t('accounting.ops.closingCash') }}</dt>
              <dd>{{ formatMoney(closing.cash, String(currency)) }}</dd>
            </div>
            <div>
              <dt>{{ t('accounting.ops.openingCard') }}</dt>
              <dd>
                {{
                  statement.opening
                    ? formatMoney(
                        statement.opening[currency]?.card || '0',
                        String(currency),
                      )
                    : t('accounting.ops.unknown')
                }}
              </dd>
            </div>
            <div>
              <dt>{{ t('accounting.ops.closingCard') }}</dt>
              <dd>{{ formatMoney(closing.card, String(currency)) }}</dd>
            </div>
            <div>
              <dt>{{ t('accounting.ops.cashMovement') }}</dt>
              <dd>
                {{
                  statement.opening
                    ? formatMoney(
                        statement.movement[currency]?.cash || '0',
                        String(currency),
                      )
                    : t('accounting.ops.unknown')
                }}
              </dd>
            </div>
            <div>
              <dt>{{ t('accounting.ops.cardMovement') }}</dt>
              <dd>
                {{
                  statement.opening
                    ? formatMoney(
                        statement.movement[currency]?.card || '0',
                        String(currency),
                      )
                    : t('accounting.ops.unknown')
                }}
              </dd>
            </div>
          </dl>
        </div>
        <div
          class="accounting-table-region"
          role="region"
          tabindex="0"
          :aria-label="t('accounting.ops.statement')"
        >
          <table class="w-full text-start text-sm">
            <thead>
              <tr>
                <th>{{ t('accounting.record') }}</th>
                <th>{{ t('accounting.occurredAt') }}</th>
                <th>{{ t('accounting.amount') }}</th>
                <th>{{ t('accounting.status') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in statementRecords"
                :key="`${row.kind}-${row.record.id}`"
              >
                <td>
                  {{ t(`accounting.kinds.${row.kind}`) }} ·
                  {{ recordLabel(row.record.details, row.record.id) }}
                </td>
                <td>{{ new Date(row.record.occurred_at).toLocaleString() }}</td>
                <td>
                  {{
                    formatMoney(
                      String(
                        row.record.details.effective_amount ||
                          row.record.amount,
                      ),
                      row.record.currency,
                    )
                  }}
                </td>
                <td>{{ t(`accounting.statuses.${row.record.status}`) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div></BaseDialog
    >
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import BaseDialog from '@/components/common/BaseDialog.vue'
import * as api from './api'
import { positiveMoney } from './money'
import { formatMoney } from './format'
import { recordLabel, publicFields } from './fields'
import type {
  AccountingAttachment,
  AccountingKind,
  AccountingRecord,
  Statement,
  Workbench,
  WorkflowRow,
  WorkflowTodo,
} from './types'
const props = withDefaults(
  defineProps<{
    organizationId: string
    organizationName: string
    section?: string
    record?: AccountingRecord
    kind?: AccountingKind
    customer?: boolean
    administrator?: boolean
  }>(),
  { section: 'workbench', customer: false, administrator: false },
)
const emit = defineEmits<{
  'open-record': [AccountingKind, string]
  updated: []
}>()
const { t, te } = useI18n(),
  formID = `${useId()}-workflow`
const loading = ref(false),
  error = ref(''),
  dialogError = ref(''),
  busy = ref(false),
  dialogLoading = ref(false),
  action = ref(''),
  target = ref<WorkflowRow>(),
  rows = ref<WorkflowRow[]>([]),
  related = ref<WorkflowRow[]>([]),
  activity = ref<WorkflowRow[]>([]),
  settlements = ref<WorkflowRow[]>([]),
  workbench = ref<Workbench>(),
  todoReason = ref(''),
  statement = ref<Statement | null>(null)
const costSummary = ref<WorkflowRow[]>([])
const transfers = ref<AccountingRecord[]>([]),
  invoices = ref<AccountingRecord[]>([]),
  transactions = ref<AccountingRecord[]>([]),
  expenses = ref<AccountingRecord[]>([]),
  cards = ref<AccountingRecord[]>([]),
  files = ref<AccountingAttachment[]>([])
const businessDate = ref('')
const form = reactive({
  organization_id: props.organizationId,
  transfer_id: '',
  invoice_id: '',
  source_kind: 'vcc-transactions',
  source_id: '',
  amount: '',
  source_amount: '',
  exchange_rate: '',
  reference: '',
  attachment_id: '',
  reason: '',
  month: '',
  accept_partial: false,
})
const title = computed(() =>
    t(
      props.record
        ? 'accounting.ops.recordContext'
        : `accounting.kinds.${props.section}`,
    ),
  ),
  hint = computed(() =>
    t(
      props.record
        ? 'accounting.ops.recordContextHint'
        : `accounting.ops.hints.${props.section}`,
    ),
  )
const todos = computed(
  () =>
    workbench.value?.items.filter(
      (row) => !todoReason.value || row.reason === todoReason.value,
    ) || [],
)
const costSources = computed(() =>
  form.source_kind === 'expenses' ? expenses.value : transactions.value,
)
const selectedSource = computed(() =>
  costSources.value.find((r) => r.id === form.source_id),
)
const crossCurrencyCost = computed(
  () =>
    !!selectedSource.value &&
    invoices.value.find((r) => r.id === form.invoice_id)?.currency !==
      selectedSource.value.currency,
)
const statementRecords = computed(() =>
  (statement.value?.records || [])
    .filter(
      (row) =>
        !props.customer ||
        ['transfers', 'vcc-transactions', 'contracts', 'invoices'].includes(
          row.kind,
        ),
    )
    .map((row) =>
      !props.customer
        ? row
        : {
            ...row,
            record: {
              ...row.record,
              details: Object.fromEntries(
                Object.entries(row.record.details).filter(([field]) =>
                  publicFields[row.kind]?.includes(field),
                ),
              ),
            },
          },
    ),
)
const key = api.stableRequest()
let alive = true,
  generation = 0,
  abort: AbortController | undefined
function failure(cause: unknown) {
  const err = cause as {
    reason?: string
    metadata?: { field?: string }
    message?: string
  }
  if (err.reason === 'ACCOUNTING_PERIOD_CLOSED')
    return t('accounting.ops.periodClosed')
  if (err.reason === 'ACCOUNTING_AUTHORIZATION_REQUIRED')
    return t('accounting.ops.adminRequired')
  if (err.metadata?.field) {
    const field = err.metadata.field
    if (te(`accounting.ops.validation.${field}`))
      return t(`accounting.ops.validation.${field}`)
    return `${t('accounting.validation')} ${te(`accounting.fields.${field}`) ? t(`accounting.fields.${field}`) : ''}`
  }
  return err.message || t('accounting.error')
}
async function load() {
  const g = ++generation
  abort?.abort()
  abort = new AbortController()
  loading.value = true
  error.value = ''
  try {
    if (props.record && props.kind) {
      const [links, events, receipts] = await Promise.all([
        api.workflow<WorkflowRow[]>(
          `related/${props.kind}/${props.record.id}`,
          props.organizationId,
        ),
        api.workflow<WorkflowRow[]>(
          `activity/${props.kind}/${props.record.id}`,
          props.organizationId,
        ),
        props.kind === 'receivables'
          ? api.workflow<WorkflowRow[]>(
              `receivables/${props.record.id}/settlements`,
              props.organizationId,
            )
          : Promise.resolve([]),
      ])
      if (alive && g === generation) {
        related.value = links
        activity.value = events
        settlements.value = receipts
      }
    } else if (props.section === 'workbench') {
      const data = await api.workflow<Workbench>(
        todoReason.value
          ? `workbench?reason=${encodeURIComponent(todoReason.value)}`
          : 'workbench',
        props.organizationId,
        false,
        abort.signal,
      )
      if (alive && g === generation) workbench.value = data
    } else {
      if (props.section === 'costs') {
        const summary = await api.workflow<WorkflowRow[]>(
          'cost-summary',
          props.organizationId,
          false,
          abort.signal,
        )
        if (alive && g === generation) costSummary.value = summary
      }
      const data = await api.workflow<WorkflowRow[]>(
        props.section === 'costs'
          ? 'cost-allocations'
          : props.section === 'external'
            ? 'external-actions'
            : 'periods',
        props.organizationId,
        props.customer,
        abort.signal,
      )
      if (alive && g === generation) rows.value = data
    }
  } catch (cause) {
    if (
      alive &&
      g === generation &&
      (cause as { code?: string }).code !== 'ERR_CANCELED'
    )
      error.value = failure(cause)
  } finally {
    if (alive && g === generation) loading.value = false
  }
}
async function records(kind: AccountingKind) {
  let page = 1
  const result: AccountingRecord[] = []
  for (;;) {
    const data = await api.listRecords(kind, false, {
      organization_id: props.organizationId,
      page,
      page_size: 100,
    })
    result.push(
      ...data.items.filter((r) => r.organization_id === props.organizationId),
    )
    if (page * 100 >= data.total || !data.items.length) break
    page++
  }
  return result
}
async function open(value: string, row?: WorkflowRow) {
  action.value = value
  target.value = row
  dialogError.value = ''
  dialogLoading.value = true
  Object.assign(form, {
    organization_id: props.organizationId,
    transfer_id: '',
    invoice_id: props.kind === 'vendor-invoices' ? props.record?.id || '' : '',
    source_kind: 'vcc-transactions',
    source_id: '',
    amount:
      props.kind === 'receivables'
        ? String(
            props.record?.details.outstanding_amount ||
              props.record?.amount ||
              '',
          )
        : '',
    reference: '',
    attachment_id: '',
    reason: '',
    month: '',
    accept_partial: false,
    source_amount: '',
    exchange_rate: '',
  })
  businessDate.value = new Date(
    Date.now() - new Date().getTimezoneOffset() * 60000,
  )
    .toISOString()
    .slice(0, 16)
  try {
    if (value === 'settle')
      transfers.value = (await records('transfers')).filter(
        (r) =>
          r.status === 'RECEIVED' &&
          r.currency === props.record?.currency &&
          r.details.funding_type !== 'FX',
      )
    if (value === 'cost') {
      const data = await Promise.all([
        records('vendor-invoices'),
        records('vcc-transactions'),
        records('expenses'),
      ])
      invoices.value = data[0].filter((r) => r.status === 'CONFIRMED')
      transactions.value = data[1].filter(
        (r) =>
          r.status === 'CONFIRMED' &&
          !['REFUND'].includes(String(r.details.transaction_type)) &&
          !(
            r.details.transaction_type === 'ADJUSTMENT' &&
            r.details.direction === 'CREDIT'
          ),
      )
      expenses.value = data[2].filter(
        (r) => !['DRAFT', 'VOID'].includes(r.status),
      )
    }
    if (value === 'fx-allocate')
      cards.value = (await records('vcc-cards')).filter(
        (r) =>
          r.status === 'ACTIVE' &&
          r.currency === props.record?.details.target_currency,
      )
    if (value === 'external' && row)
      files.value = (
        await api.attachments('vcc-cards', String(row.card_id), false)
      ).filter((file) => file.status === 'COMPLETE')
  } catch (cause) {
    dialogError.value = failure(cause)
  } finally {
    dialogLoading.value = false
  }
}
async function submit() {
  if (busy.value || dialogLoading.value || props.customer) return
  busy.value = true
  dialogError.value = ''
  try {
    let path = ''
    if (action.value === 'settle')
      path = `receivables/${props.record!.id}/settlements`
    if (action.value === 'cost') path = 'cost-allocations'
    if (action.value === 'fx-allocate')
      path = `fx-exchanges/${props.record!.id}/allocations`
    if (action.value === 'external-exempt')
      path = `external-actions/${target.value!.id}/void`
    if (action.value === 'external')
      path = `external-actions/${target.value!.id}/confirm`
    if (action.value === 'close') path = 'periods/close'
    if (action.value === 'reopen') path = `periods/${target.value!.id}/reopen`
    if (action.value === 'void-settlement')
      path = `settlements/${target.value!.id}/void`
    if (action.value === 'void-cost')
      path = `cost-allocations/${target.value!.id}/void`
    if (!path) return
    const payload = {
      ...form,
      ...(['settle', 'cost'].includes(action.value)
        ? { occurred_at: new Date(businessDate.value).toISOString() }
        : {}),
    }
    await api.mutate(path, payload, key(path, payload))
    key.reset()
    action.value = ''
    await load()
    emit('updated')
  } catch (cause) {
    dialogError.value = failure(cause)
  } finally {
    busy.value = false
  }
}
async function openTodo(row: WorkflowTodo) {
  if (row.kind === 'external-actions') {
    try {
      const actions = await api.workflow<WorkflowRow[]>(
        'external-actions',
        props.organizationId,
      )
      const found = actions.find((item) => item.id === row.id)
      if (found) await open('external', found)
    } catch (cause) {
      error.value = failure(cause)
    }
  } else emit('open-record', row.kind, row.id)
}
function statusText(row: WorkflowRow) {
  return props.section === 'external'
    ? t(`accounting.ops.externalStatuses.${row.status}`)
    : t(`accounting.statuses.${row.status}`)
}
function rowName(row: WorkflowRow) {
  if (props.section === 'periods') return String(row.month)
  if (props.section === 'external')
    return `${row.card_name} · ${t(`accounting.ops.externalActions.${row.action}`)}`
  return `${row.invoice_name || ''} · ${row.source_name || t(`accounting.kinds.${row.source_kind}`)}`
}
function activityText(row: WorkflowRow) {
  const summary = row.summary as Record<string, unknown>,
    before = summary.before as Record<string, unknown> | undefined,
    after = summary.after as Record<string, unknown> | undefined
  return [
    before?.status
      ? `${t('accounting.status')}: ${t(`accounting.statuses.${before.status}`)} → ${t(`accounting.statuses.${after?.status || before.status}`)}`
      : '',
    before?.amount
      ? `${t('accounting.amount')}: ${before.effective_amount || before.amount} → ${after?.effective_amount || after?.amount || before.effective_amount || before.amount} ${after?.currency || before.currency}`
      : '',
    summary.reason,
    summary.reference,
  ]
    .filter(Boolean)
    .join(' · ')
}

async function readStatement(id: string) {
  try {
    statement.value = await api.workflow<Statement>(
      `statements/${id}`,
      props.organizationId,
      props.customer,
    )
  } catch (cause) {
    error.value = failure(cause)
  }
}
async function downloadStatement(id: string) {
  try {
    saveAs(
      await api.statementCSV(id, props.customer),
      `${props.organizationName}-statement.csv`,
    )
  } catch (cause) {
    error.value = failure(cause)
  }
}
watch(todoReason, () => {
  if (props.section === 'workbench') void load()
})
watch(
  [() => form.invoice_id, () => form.source_id, () => form.source_kind],
  () => {
    form.source_amount = ''
    form.exchange_rate = ''
  },
)
watch(
  () => [props.organizationId, props.section, props.record?.id],
  () => {
    rows.value = []
    activity.value = []
    related.value = []
    settlements.value = []
    void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => {
  alive = false
  generation++
  abort?.abort()
})
</script>
