<template>
  <div
    class="mx-auto min-w-0 max-w-[1600px] space-y-5"
    data-testid="accounting-workspace"
  >
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h1 class="text-2xl font-semibold text-gray-950 dark:text-white">
          {{ t(customer ? 'accounting.customerTitle' : 'accounting.title') }}
        </h1>
        <p class="mt-2 text-sm text-gray-500 dark:text-dark-300">
          {{
            t(
              customer
                ? 'accounting.customerDescription'
                : 'accounting.description',
            )
          }}
        </p>
        <p v-if="customer" class="mt-1 text-xs text-gray-500">
          {{ t('accounting.fromAccount') }}
        </p>
      </div>
      <div class="flex flex-wrap gap-2">
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="loading"
          @click="load"
        >
          {{ t('accounting.refresh') }}</button
        ><button
          v-if="!customer"
          type="button"
          class="btn btn-primary"
          @click="openCreate('transfers')"
        >
          {{ t('accounting.kinds.transfers') }} +</button
        ><button
          v-if="!customer"
          type="button"
          class="btn btn-secondary"
          @click="openCreate('expenses')"
        >
          {{ t('accounting.kinds.expenses') }} +
        </button>
      </div>
    </header>
    <form
      class="card flex flex-wrap items-end gap-3 p-4"
      @submit.prevent="applyFilters"
    >
      <label v-if="!customer && !organizationId" class="min-w-0"
        ><span class="input-label">{{ t('accounting.organization') }}</span
        ><select
          v-model="selectedOrg"
          class="input max-w-64"
          @change="applyFilters"
        >
          <option value="">{{ t('accounting.allOrganizations') }}</option>
          <option v-for="org in orgs" :key="org.id" :value="org.id">
            {{ org.name }}
          </option>
        </select></label
      >
      <label
        ><span class="input-label">{{ t('accounting.currency') }}</span
        ><input
          v-model="query.currency"
          class="input w-28"
          maxlength="3"
          :placeholder="t('accounting.allCurrencies')"
      /></label>
      <label
        ><span class="input-label">{{ t('accounting.from') }}</span
        ><input v-model="query.from" type="date" class="input" /></label
      ><label
        ><span class="input-label">{{ t('accounting.to') }}</span
        ><input v-model="query.to" type="date" class="input"
      /></label>
      <label
        ><span class="input-label">{{ t('accounting.status') }}</span
        ><select v-model="query.status" class="input">
          <option value="">{{ t('accounting.allStatuses') }}</option>
          <option v-for="state in statusOptions" :key="state" :value="state">
            {{ t(`accounting.statuses.${state}`) }}
          </option>
        </select></label
      >
      <label v-if="!customer" class="min-w-0 flex-1"
        ><span class="input-label">{{ t('accounting.search') }}</span
        ><input v-model="query.search" class="input w-full" /></label
      ><button type="submit" class="btn btn-secondary">
        {{ t('accounting.refresh') }}
      </button>
    </form>
    <div
      v-if="error"
      role="alert"
      class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200"
    >
      {{ error }}
    </div>
    <div v-if="loading" role="status" class="text-sm text-gray-500">
      {{ t('accounting.loading') }}
    </div>
    <p class="text-xs text-gray-500">{{ t('accounting.balanceScope') }}</p>
    <section
      v-for="currency in summaries"
      :key="currency.currency"
      :aria-label="currency.currency"
      class="space-y-2"
    >
      <h2 class="text-sm font-medium">{{ currency.currency }}</h2>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div v-for="metric in metrics" :key="metric" class="card min-w-0 p-4">
          <div class="text-xs text-gray-500 dark:text-dark-300">
            {{
              t(
                `accounting.metrics.${customer && metric === 'consumed' ? 'customerConsumed' : metric}`,
              )
            }}
          </div>
          <div class="mt-2 overflow-x-auto whitespace-nowrap text-base font-semibold tabular-nums sm:text-xl">
            {{
              metric === 'unmatched'
                ? currency.unmatched
                : formatMoney(currency[metric], currency.currency)
            }}
          </div>
        </div>
      </div>
      <p
        v-if="Number(currency.balance) < 0"
        role="status"
        class="text-sm text-amber-700 dark:text-amber-300"
      >
        {{ t('accounting.overdraft') }}
      </p>
    </section>
    <nav
      role="tablist"
      :aria-label="t('accounting.title')"
      class="flex max-w-full gap-2 overflow-x-auto border-b border-gray-200 pb-2 dark:border-dark-600"
    >
      <button
        v-for="item in tabs"
        :id="`${uid}-${item}`"
        :key="item"
        type="button"
        role="tab"
        :aria-selected="tab === item"
        :aria-controls="`${uid}-panel`"
        :tabindex="tab === item ? 0 : -1"
        class="btn shrink-0"
        :class="tab === item ? 'btn-primary' : 'btn-secondary'"
        @click="selectTab(item)"
        @keydown="tabKey($event, item)"
      >
        {{
          t(
            item === 'overview'
              ? 'accounting.overview'
              : `accounting.kinds.${item}`,
          )
        }}
      </button>
    </nav>
    <section
      :id="`${uid}-panel`"
      role="tabpanel"
      :aria-labelledby="`${uid}-${tab}`"
      class="min-w-0 space-y-4"
    >
      <template v-if="tab === 'overview'">
        <div class="grid min-w-0 gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
          <div
            v-if="!customer && !organizationId"
            class="card min-w-0 overflow-x-auto"
          >
            <table class="w-full text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 dark:border-dark-600">
                  <th class="p-4">{{ t('accounting.organizations') }}</th>
                  <th class="p-4">{{ t('accounting.metrics.received') }}</th>
                  <th class="p-4">{{ t('accounting.metrics.balance') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="org in filteredOrgs"
                  :key="org.id"
                  class="border-b border-gray-100 dark:border-dark-700"
                >
                  <td class="p-4">
                    <button
                      type="button"
                      class="text-primary-600 hover:underline"
                      @click="detailOrg = org"
                    >
                      {{ org.name }}
                    </button>
                    <p class="mt-1 text-xs text-gray-500">{{ org.id }}</p>
                  </td>
                  <td class="p-4">
                    <p v-for="entry in org.summary" :key="entry.currency">
                      {{ formatMoney(entry.received, entry.currency) }}
                    </p>
                  </td>
                  <td class="p-4">
                    <p v-for="entry in org.summary" :key="entry.currency">
                      {{ formatMoney(entry.balance, entry.currency) }}
                    </p>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-else class="grid gap-3 sm:grid-cols-2">
            <div
              v-for="entry in summaries"
              :key="entry.currency"
              class="card p-5"
            >
              <p class="text-sm text-gray-500">
                {{ t('accounting.metrics.unallocated') }}
              </p>
              <p class="mt-2 text-xl font-semibold">
                {{ formatMoney(entry.unallocated, entry.currency) }}
              </p>
              <p class="mt-3 text-sm text-gray-500">
                {{ t('accounting.metrics.balance') }}
              </p>
              <p class="mt-2 font-semibold">
                {{ formatMoney(entry.balance, entry.currency) }}
              </p>
            </div>
          </div>
          <aside class="space-y-4">
            <div v-if="!customer" class="card p-4">
              <h2 class="font-semibold">{{ t('accounting.todo') }}</h2>
              <button
                v-for="entry in summaries"
                :key="entry.currency"
                type="button"
                class="mt-3 block text-sm text-primary-600"
                @click="selectTab('reconciliation')"
              >
                {{ entry.currency }} · {{ entry.unmatched }} ·
                {{ formatMoney(entry.unmatched_amount || '0', entry.currency) }}
              </button>
            </div>
            <div class="card p-4">
              <h2 class="font-semibold">{{ t('accounting.trend') }}</h2>
              <div
                v-if="trend.length"
                class="mt-4 flex h-28 items-end gap-1"
                :aria-label="t('accounting.trend')"
              >
                <div
                  v-for="point in trend"
                  :key="`${point.date}-${point.currency}`"
                  class="min-w-1 flex-1 rounded-t bg-primary-500"
                  :style="{
                    height: `${Math.max(4, (Number(point.amount) / trendMax) * 100)}%`,
                  }"
                  :title="`${point.date}: ${formatMoney(point.amount, point.currency)}`"
                />
              </div>
              <p v-else class="mt-4 text-sm text-gray-500">
                {{ t('accounting.empty') }}
              </p>
              <p v-if="trend.length" class="mt-2 text-xs text-gray-500">
                {{ trend[0]?.date }} — {{ trend[trend.length - 1]?.date }}
              </p>
            </div>
          </aside>
        </div>
      </template>
      <template v-else-if="isRecordTab(tab)">
        <div v-if="!customer" class="flex flex-wrap gap-2">
          <button
            v-if="tab !== 'expense-payments'"
            type="button"
            class="btn btn-primary"
            @click="openCreate(tab)"
          >
            {{ t('accounting.create') }}</button
          ><button
            type="button"
            class="btn btn-secondary"
            @click="exportRecords"
          >
            {{ t('accounting.export') }}
          </button>
        </div>
        <DataTable
          class="card"
          :columns="recordColumns"
          :data="records"
          :loading="loading"
          row-key="id"
        >
          <template #cell-record="{ row }"
            ><div class="max-w-xs break-words">
              {{ recordLabel(row.details, row.id) }}
              <p v-if="tab === 'vcc-cards'" class="mt-1 text-xs text-gray-500">
                •••• {{ row.details.last_four }}
              </p>
            </div></template
          >
          <template #cell-occurred_at="{ row }">{{
            date(row.occurred_at)
          }}</template>
          <template #cell-amount="{ row }"
            ><span class="tabular-nums">{{
              formatMoney(
                tab === 'vcc-cards'
                  ? String(row.details.available_balance || '0')
                  : row.amount,
                row.currency,
              )
            }}</span></template
          >
          <template #cell-status="{ row }">{{
            t(`accounting.statuses.${row.status}`)
          }}</template>
          <template #cell-actions="{ row }"
            ><button
              type="button"
              class="btn btn-secondary"
              @click="openRecordFromTab(row)"
            >
              {{ t('accounting.details') }}
            </button></template
          >
        </DataTable>
      </template>
      <div v-else-if="tab === 'reconciliation'" class="space-y-3">
        <div
          v-for="row in reconciliationRows"
          :key="`${row.object_kind}-${row.record.id}`"
          class="card flex flex-wrap items-center justify-between gap-3 p-4"
        >
          <div class="min-w-0">
            <p>
              {{ recordLabel(row.record.details, row.record.id) }} ·
              {{ formatMoney(row.record.amount, row.record.currency) }}
            </p>
            <p class="mt-1 text-sm text-gray-500">
              {{ t(`accounting.statuses.${row.status}`) }} {{ row.reason }}
            </p>
          </div>
          <div class="flex gap-2">
            <button
              class="btn btn-secondary"
              type="button"
              @click="openAction('match', row.record, row.object_kind)"
            >
              {{ t('accounting.match') }}</button
            ><button
              class="btn btn-secondary"
              type="button"
              @click="openAction('exclude', row.record, row.object_kind)"
            >
              {{ t('accounting.exclude') }}
            </button>
          </div>
        </div>
      </div>
      <div v-else class="card overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead>
            <tr>
              <th class="p-3">{{ t('accounting.createdAt') }}</th>
              <th class="p-3">{{ t('accounting.actions') }}</th>
              <th class="p-3">{{ t('accounting.actor') }}</th>
              <th class="p-3">{{ t('accounting.requestId') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in auditRows" :key="row.id">
              <td class="p-3">{{ date(row.created_at) }}</td>
              <td class="p-3">{{ row.object_kind }} · {{ row.action }}</td>
              <td class="p-3">{{ row.actor_id }}</td>
              <td class="break-all p-3">{{ row.request_id }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!loading && empty" class="card p-8 text-center">
        <p class="font-medium">{{ t('accounting.empty') }}</p>
        <p class="mt-2 text-sm text-gray-500">
          {{ t('accounting.emptyHint') }}
        </p>
      </div>
      <div
        v-if="tab !== 'overview' && total > 20"
        class="flex items-center justify-end gap-3"
      >
        <button
          type="button"
          class="btn btn-secondary"
          :disabled="page === 1"
          @click="previousPage"
        >
          {{ t('accounting.previous') }}</button
        ><span class="text-sm">{{ t('accounting.page', { page }) }}</span
        ><button
          type="button"
          class="btn btn-secondary"
          :disabled="page * 20 >= total"
          @click="nextPage"
        >
          {{ t('accounting.next') }}
        </button>
      </div>
    </section>

    <BaseDialog
      :show="!!detailOrg"
      :title="detailOrg?.name || ''"
      width="wide"
      @close="detailOrg = null"
      ><AccountingWorkspace v-if="detailOrg" :organization-id="detailOrg.id"
    /></BaseDialog>
    <BaseDialog
      :show="!!detailRecord"
      :title="t('accounting.details')"
      width="wide"
      @close="detailRecord = null"
    >
      <div v-if="detailRecord" class="space-y-5">
        <p
          v-if="error"
          role="alert"
          class="text-sm text-red-700 dark:text-red-300"
        >
          {{ error }}
        </p>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <p class="text-xs text-gray-500">{{ t('accounting.amount') }}</p>
            <p class="mt-1 font-semibold">
              {{ formatMoney(detailRecord.amount, detailRecord.currency) }}
            </p>
          </div>
          <div>
            <p class="text-xs text-gray-500">{{ t('accounting.status') }}</p>
            <p>{{ t(`accounting.statuses.${detailRecord.status}`) }}</p>
          </div>
          <div
            v-for="(value, field) in visibleDetails"
            :key="field"
            class="min-w-0"
          >
            <p class="text-xs text-gray-500">
              {{ t(`accounting.fields.${field}`) }}
            </p>
            <p class="mt-1 break-words text-sm">
              {{
                typeof value === 'boolean'
                  ? t(value ? 'common.yes' : 'common.no')
                  : value
              }}
            </p>
          </div>
        </div>
        <div v-if="!customer" class="flex flex-wrap gap-2">
          <button
            v-if="
              detailRecord.status === 'DRAFT' ||
              (detailKind === 'vcc-cards' && detailRecord.status === 'ACTIVE')
            "
            type="button"
            class="btn btn-secondary"
            @click="openEdit"
          >
            {{ t('accounting.edit') }}</button
          ><button
            v-if="
              detailRecord.status === 'DRAFT' &&
              !['expenses', 'expense-payments'].includes(detailKind)
            "
            type="button"
            class="btn btn-primary"
            @click="openAction('confirm', detailRecord, detailKind)"
          >
            {{ t('accounting.confirm') }}</button
          ><button
            v-if="detailRecord.status !== 'VOID' && detailKind !== 'vcc-cards'"
            type="button"
            class="btn btn-secondary"
            @click="openAction('void', detailRecord, detailKind)"
          >
            {{ t('accounting.void') }}</button
          ><button
            v-if="
              detailKind === 'transfers' && detailRecord.status === 'RECEIVED'
            "
            type="button"
            class="btn btn-secondary"
            @click="openAction('allocate', detailRecord, detailKind)"
          >
            {{ t('accounting.allocate') }}</button
          ><button
            v-if="
              detailKind === 'vcc-cards' && detailRecord.status === 'ACTIVE'
            "
            type="button"
            class="btn btn-secondary"
            @click="openAction('close', detailRecord, detailKind)"
          >
            {{ t('accounting.closeCard') }}</button
          ><button
            v-if="detailKind === 'expenses' && detailRecord.status !== 'VOID'"
            type="button"
            class="btn btn-secondary"
            @click="openAction('pay', detailRecord, detailKind)"
          >
            {{ t('accounting.pay') }}</button
          ><button
            v-if="
              ['transfers', 'expenses'].includes(detailKind) &&
              !['DRAFT', 'VOID'].includes(detailRecord.status)
            "
            type="button"
            class="btn btn-secondary"
            @click="openAction('adjust', detailRecord, detailKind)"
          >
            {{ t('accounting.adjust') }}</button
          ><button
            v-if="
              detailKind === 'contracts' ||
              (detailKind === 'invoices' &&
                detailRecord.details.invoice_type === 'SALES')
            "
            type="button"
            class="btn btn-secondary"
            @click="shareRecord"
          >
            {{
              t(
                detailRecord.customer_visible
                  ? 'accounting.unshare'
                  : 'accounting.share',
              )
            }}
          </button>
        </div>
        <button
          v-if="
            !customer &&
            detailKind === 'contracts' &&
            detailRecord.status === 'SIGNED'
          "
          type="button"
          class="btn btn-secondary"
          @click="openAction('expire', detailRecord, detailKind)"
        >
          {{ t('accounting.expire') }}
        </button>
        <section v-if="!customer && allocationRows.length" class="space-y-3">
          <h2 class="font-semibold">{{ t('accounting.allocationHistory') }}</h2>
          <div
            v-for="row in allocationRows"
            :key="row.id"
            class="rounded-lg border border-gray-200 p-3 text-sm dark:border-dark-600"
          >
            <p class="break-all">
              {{ detailKind === 'transfers' ? row.card_id : row.transfer_id }}
            </p>
            <p class="mt-2">
              {{ t('accounting.amount') }}:
              {{ formatMoney(row.amount, detailRecord.currency) }} ·
              {{ t('accounting.used') }}:
              {{ formatMoney(row.used_amount, detailRecord.currency) }} ·
              {{ t('accounting.released') }}:
              {{ formatMoney(row.released_amount, detailRecord.currency) }}
            </p>
          </div>
        </section>
        <section>
          <h2 class="font-semibold">{{ t('accounting.files') }}</h2>
          <div
            v-for="file in recordFiles"
            :key="file.id"
            class="mt-3 flex flex-wrap items-center justify-between gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600"
          >
            <div class="min-w-0">
              <p class="break-all text-sm">{{ file.filename }}</p>
              <p class="mt-1 text-xs text-gray-500">
                {{
                  t(
                    file.customer_visible
                      ? 'accounting.customerVisible'
                      : 'accounting.internal',
                  )
                }}
                · {{ t(`accounting.statuses.${file.status}`) }}
              </p>
            </div>
            <div class="flex flex-wrap gap-2">
              <button
                type="button"
                class="btn btn-secondary"
                @click="downloadFile(file)"
              >
                {{ t('accounting.download') }}</button
              ><template v-if="!customer"
                ><button
                  v-if="shareableFile"
                  type="button"
                  class="btn btn-secondary"
                  @click="shareFile(file)"
                >
                  {{
                    t(
                      file.customer_visible
                        ? 'accounting.unshare'
                        : 'accounting.share',
                    )
                  }}</button
                ><button
                  type="button"
                  class="btn btn-secondary"
                  @click="deleteFile(file)"
                >
                  {{ t('accounting.delete') }}
                </button></template
              >
            </div>
          </div>
          <p v-if="!recordFiles.length" class="mt-3 text-sm text-gray-500">
            {{ t('accounting.noFiles') }}
          </p>
          <label
            v-if="!customer && detailRecord.status !== 'VOID'"
            class="mt-4 block"
            ><span class="input-label">{{ t('accounting.upload') }}</span
            ><input
              type="file"
              accept=".jpg,.jpeg,.png,.webp,.pdf"
              :disabled="busy"
              @change="uploadExisting"
            /><span class="mt-2 block text-xs text-gray-500">{{
              t('accounting.fileHint')
            }}</span></label
          >
        </section>
      </div>
    </BaseDialog>
    <BaseDialog
      :show="!!action"
      :title="actionTitle"
      width="wide"
      @close="!busy && (action = '')"
    >
      <form class="space-y-4" @submit.prevent="saveAction">
        <div
          v-if="dialogError"
          role="alert"
          class="rounded-lg bg-red-50 p-3 text-sm text-red-800 dark:bg-red-950 dark:text-red-200"
        >
          {{ dialogError }}
        </div>
        <template
          v-if="action === 'create' || action === 'edit' || action === 'pay'"
        >
          <div class="grid min-w-0 grid-cols-1 gap-4 sm:grid-cols-2">
            <label v-if="action === 'create' && !organizationId"
              ><span class="input-label">{{
                t('accounting.organization')
              }}</span
              ><select
                v-model="form.organization_id"
                class="input w-full"
                required
                @change="loadReferences"
              >
                <option value="">{{ t('accounting.select') }}</option>
                <option v-for="org in orgs" :key="org.id" :value="org.id">
                  {{ org.name }}
                </option>
              </select></label
            ><label
              ><span class="input-label">{{ t('accounting.currency') }}</span
              ><input
                v-model="form.currency"
                class="input w-full"
                maxlength="3"
                required /></label
            ><label
              ><span class="input-label">{{
                formKind === 'transfers'
                  ? t('accounting.fields.received_amount')
                  : t('accounting.amount')
              }}</span
              ><input
                v-model="form.amount"
                class="input w-full"
                inputmode="decimal"
                required /></label
            ><label
              ><span class="input-label">{{ t('accounting.occurredAt') }}</span
              ><input
                v-model="formDate"
                type="datetime-local"
                class="input w-full"
                required /></label
            ><label v-if="formKind === 'vcc-transactions'"
              ><span class="input-label">{{
                t('accounting.kinds.vcc-cards')
              }}</span
              ><select v-model="form.parent_id" class="input w-full" required>
                <option value="">{{ t('accounting.select') }}</option>
                <option
                  v-for="r in references['vcc-cards'] || []"
                  :key="r.id"
                  :value="r.id"
                >
                  {{ recordLabel(r.details, r.id) }} · {{ r.currency }}
                </option>
              </select></label
            >
            <label v-for="field in formFields" :key="field.key" class="min-w-0"
              ><span class="input-label">{{
                t(`accounting.fields.${field.key}`)
              }}</span
              ><input
                v-if="field.type === 'boolean'"
                v-model="form.details[field.key]"
                type="checkbox"
                class="mt-2 h-4 w-4" /><select
                v-else-if="field.type === 'select'"
                v-model="form.details[field.key]"
                class="input w-full"
              >
                <option v-for="opt in field.options" :key="opt" :value="opt">
                  {{ t(`accounting.statuses.${opt}`) }}
                </option></select
              ><select
                v-else-if="field.type === 'attachment'"
                v-model="form.details[field.key]"
                class="input w-full"
                required
              >
                <option value="">{{ t('accounting.select') }}</option>
                <option
                  v-for="file in paymentFiles"
                  :key="file.id"
                  :value="file.id"
                >
                  {{ file.filename }}
                </option></select
              ><select
                v-else-if="field.type === 'reference'"
                v-model="form.details[field.key]"
                class="input w-full"
              >
                <option value="">{{ t('accounting.select') }}</option>
                <option
                  v-for="r in references[field.reference!] || []"
                  :key="r.id"
                  :value="r.id"
                >
                  {{ recordLabel(r.details, r.id) }} · {{ r.currency }}
                </option></select
              ><input
                v-else
                v-model="form.details[field.key]"
                :type="
                  field.type === 'date'
                    ? 'date'
                    : field.type === 'secret'
                      ? 'password'
                      : 'text'
                "
                :autocomplete="field.type === 'secret' ? 'new-password' : 'off'"
                class="input w-full"
            /></label>
          </div>
          <p v-if="formKind === 'transfers'" class="text-xs text-gray-500">
            {{ t('accounting.taxHint') }}
          </p>
          <label v-if="action !== 'pay'" class="block"
            ><span class="input-label">{{ t('accounting.upload') }}</span
            ><input
              type="file"
              accept=".jpg,.jpeg,.png,.webp,.pdf"
              @change="chooseFile"
            /><span class="mt-2 block text-xs text-gray-500">{{
              t('accounting.fileHint')
            }}</span></label
          >
        </template>
        <template v-else-if="action === 'allocate'"
          ><label class="block"
            ><span class="input-label">{{
              t('accounting.kinds.vcc-cards')
            }}</span
            ><select v-model="command.card_id" class="input w-full" required>
              <option value="">{{ t('accounting.select') }}</option>
              <option
                v-for="r in references['vcc-cards'] || []"
                :key="r.id"
                :value="r.id"
              >
                {{ recordLabel(r.details, r.id) }} · {{ r.currency }}
              </option>
            </select></label
          ><label class="block"
            ><span class="input-label">{{ t('accounting.amount') }}</span
            ><input
              v-model="command.amount"
              class="input w-full"
              inputmode="decimal"
              required
          /></label>
          <p class="text-xs text-gray-500">
            {{ t('accounting.allocationHint') }}
          </p></template
        >
        <template v-else-if="['match', 'exclude', 'adjust'].includes(action)"
          ><label class="block"
            ><span class="input-label">{{
              t(
                action === 'adjust'
                  ? 'accounting.amount'
                  : 'accounting.referenceAmount',
              )
            }}</span
            ><input
              v-model="command.amount"
              class="input w-full"
              inputmode="decimal"
              required /></label
          ><label v-if="action !== 'adjust'" class="block"
            ><span class="input-label">{{ t('accounting.reference') }}</span
            ><input v-model="command.reference" class="input w-full"
          /></label>
          <p v-if="action === 'adjust'" class="text-xs text-gray-500">
            {{ t('accounting.negativeAdjustment') }}
          </p></template
        >
        <label
          v-if="['void', 'exclude', 'match', 'adjust'].includes(action)"
          class="block"
          ><span class="input-label">{{ t('accounting.reason') }}</span
          ><textarea
            v-model="command.reason"
            class="input min-h-20 w-full"
            :required="action !== 'match'"
          />
        </label>
        <p
          v-if="action === 'confirm' || action === 'pay'"
          class="text-sm text-gray-500"
        >
          {{ t('accounting.uploadFirst') }}
        </p>
        <div class="flex justify-end gap-2">
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="busy"
            @click="action = ''"
          >
            {{ t('accounting.cancel') }}</button
          ><button type="submit" class="btn btn-primary" :disabled="busy">
            {{ t('accounting.save') }}
          </button>
        </div>
      </form>
    </BaseDialog>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { saveAs } from 'file-saver'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import { useDesktopOrganizationAccess } from '@/composables/useDesktopOrganizationAccess'
import * as api from './api'
import { formatMoney } from './format'
import {
  adminKinds,
  customerKinds,
  fields,
  publicFields,
  recordLabel,
} from './fields'
import type {
  AccountingKind,
  AccountingRecord,
  AccountingInput,
  AccountingOrganization,
  CurrencySummary,
  AccountingAttachment,
  AccountingFilter,
  AuditRow,
  ReconciliationRow,
} from './types'

defineOptions({ name: 'AccountingWorkspace' })
const props = withDefaults(
  defineProps<{ customer?: boolean; organizationId?: string }>(),
  { customer: false, organizationId: '' },
)
const { t } = useI18n()
const route = useRoute()
const uid = useId()
type Tab = AccountingKind | 'overview' | 'reconciliation' | 'audit'
const tab = ref<Tab>('overview')
const tabs = computed<Tab[]>(() => [
  'overview',
  ...(props.customer ? customerKinds : adminKinds),
  ...(!props.customer ? (['reconciliation', 'audit'] as Tab[]) : []),
])
const selectedOrg = ref(
  props.organizationId || String(route.query.organization_id || ''),
)
const query = reactive({
  currency: '',
  status: '',
  search: '',
  from: '',
  to: '',
})
const page = ref(1),
  total = ref(0),
  loading = ref(false),
  busy = ref(false),
  error = ref(''),
  dialogError = ref('')
const orgs = ref<AccountingOrganization[]>([]),
  summaries = ref<CurrencySummary[]>([]),
  records = ref<AccountingRecord[]>([]),
  auditRows = ref<AuditRow[]>([]),
  reconciliationRows = ref<ReconciliationRow[]>([])
const trend = ref<{ date: string; currency: string; amount: string }[]>([])
const trendMax = computed(() =>
  Math.max(1, ...trend.value.map((p) => Number(p.amount))),
)
const metrics = computed(() =>
  props.customer
    ? (['received', 'allocated', 'consumed', 'balance'] as const)
    : (['received', 'allocated', 'consumed', 'unmatched'] as const),
)
const recordColumns = computed(() =>
  ['record', 'occurred_at', 'amount', 'status', 'actions'].map((key) => ({
    key,
    label: t(`accounting.${key === 'occurred_at' ? 'occurredAt' : key}`),
  })),
)
const statusOptions = computed(() => {
  const reconcile = ['UNMATCHED', 'MATCHED', 'DIFFERENCE', 'EXCLUDED']
  const states: Partial<Record<Tab, string[]>> = {
    transfers: ['DRAFT', 'RECEIVED', 'VOID', ...reconcile],
    'vcc-cards': ['ACTIVE', 'CLOSED'],
    'vcc-transactions': ['DRAFT', 'CONFIRMED', 'VOID', ...reconcile],
    contracts: ['DRAFT', 'SIGNED', 'EXPIRED', 'VOID'],
    invoices: ['DRAFT', 'VERIFIED', 'VOID'],
    'vendor-invoices': ['DRAFT', 'CONFIRMED', 'VOID'],
    expenses: ['DRAFT', 'PENDING', 'PAID', 'VOID', ...reconcile],
    'expense-payments': ['PAID', 'VOID'],
    overview: reconcile,
    reconciliation: reconcile,
    audit: [],
  }
  return states[tab.value] || []
})
const filteredOrgs = computed(() =>
  selectedOrg.value
    ? orgs.value.filter((o) => o.id === selectedOrg.value)
    : orgs.value,
)
const detailOrg = ref<AccountingOrganization | null>(null),
  detailRecord = ref<AccountingRecord | null>(null),
  detailKind = ref<AccountingKind>('transfers'),
  recordFiles = ref<AccountingAttachment[]>([])
const allocationRows = ref<Awaited<ReturnType<typeof api.allocationHistory>>>(
  [],
)
const action = ref(''),
  actionRecord = ref<AccountingRecord | null>(null),
  formKind = ref<AccountingKind>('transfers')
const form = reactive<AccountingInput>({
  organization_id: '',
  parent_id: '',
  currency: 'CNY',
  amount: '',
  occurred_at: '',
  details: {},
})
const formDate = ref(''),
  selectedFile = ref<File | null>(null)
const paymentFiles = ref<AccountingAttachment[]>([])
const references = reactive<
  Partial<Record<AccountingKind, AccountingRecord[]>>
>({})
const command = reactive({
  amount: '',
  card_id: '',
  reason: '',
  reference: '',
})
const requestKey = api.stableRequest()
let abort: AbortController | null = null
let generation = 0
const access = useDesktopOrganizationAccess()
const formFields = computed(() => fields[formKind.value])
const shareableFile = computed(
  () =>
    detailKind.value === 'transfers' ||
    detailKind.value === 'contracts' ||
    (detailKind.value === 'invoices' &&
      detailRecord.value?.details.invoice_type === 'SALES') ||
    (detailKind.value === 'vcc-transactions' &&
      ['CONSUME', 'REFUND'].includes(
        String(detailRecord.value?.details.transaction_type),
      )),
)
const visibleDetails = computed(() =>
  Object.fromEntries(
    Object.entries(detailRecord.value?.details || {}).filter(
      ([k]) =>
        !['card_number', 'cvv', 'expiry', 'provider_token'].includes(k) &&
        (!props.customer || publicFields[detailKind.value]?.includes(k)),
    ),
  ),
)
const actionTitle = computed(() =>
  action.value
    ? t(
        action.value === 'create'
          ? `accounting.kinds.${formKind.value}`
          : `accounting.${action.value === 'close' ? 'closeCard' : action.value}`,
      )
    : '',
)
const empty = computed(() =>
  tab.value === 'overview'
    ? summaries.value.length === 0
    : tab.value === 'audit'
      ? auditRows.value.length === 0
      : tab.value === 'reconciliation'
        ? reconciliationRows.value.length === 0
        : records.value.length === 0,
)
function isRecordTab(value: Tab): value is AccountingKind {
  return adminKinds.includes(value as AccountingKind)
}
function date(value: string) {
  return new Date(value).toLocaleString()
}
function localDateTime(value = new Date().toISOString()) {
  const d = new Date(value)
  return new Date(d.getTime() - d.getTimezoneOffset() * 60000)
    .toISOString()
    .slice(0, 16)
}
function filters(): AccountingFilter {
  return {
    organization_id: props.customer
      ? undefined
      : selectedOrg.value || undefined,
    currency: query.currency.toUpperCase() || undefined,
    status: query.status || undefined,
    search: query.search || undefined,
    from: query.from
      ? new Date(`${query.from}T00:00:00`).toISOString()
      : undefined,
    to: query.to
      ? new Date(
          new Date(`${query.to}T00:00:00`).getTime() + 86400000,
        ).toISOString()
      : undefined,
    page: page.value,
    page_size: 20,
  }
}
function errorMessage(err: unknown) {
  const e = err as {
    reason?: string
    message?: string
    metadata?: { field?: string }
  }
  const reasons: Record<string, string> = {
    ACCOUNTING_INVALID: 'validation',
    ACCOUNTING_STATE_CONFLICT: 'stateConflict',
    ACCOUNTING_PROOF_REQUIRED: 'proofRequired',
    ACCOUNTING_STORAGE_UNAVAILABLE: 'storageUnavailable',
    ACCOUNTING_AMOUNT_CONFLICT: 'amountConflict',
    ACCOUNTING_CURRENCY_MISMATCH: 'currencyMismatch',
    IDEMPOTENCY_KEY_CONFLICT: 'keyConflict',
  }
  if (e.metadata?.field === 'fixed_encryption_key')
    return t('accounting.fixedKey')
  return e.reason && reasons[e.reason]
    ? t(`accounting.${reasons[e.reason]}`)
    : e.message || t('accounting.error')
}
async function load() {
  const current = ++generation
  abort?.abort()
  abort = new AbortController()
  loading.value = true
  error.value = ''
  try {
    if (props.customer) {
      const organization = await access.refreshDesktopOrganizationAccess(true)
      if (!organization) throw new Error(t('accounting.error'))
    }
    const f = filters()
    const [summaryResult, orgResult] = await Promise.all([
      api.summary(props.customer, f, abort.signal),
      props.customer ? Promise.resolve([]) : api.organizations(f, abort.signal),
    ])
    if (current !== generation) return
    summaries.value = summaryResult
    if (!props.customer) orgs.value = orgResult
    if (isRecordTab(tab.value)) {
      const result = await api.listRecords(
        tab.value,
        props.customer,
        f,
        abort.signal,
      )
      if (current !== generation) return
      records.value = result.items
      total.value = result.total
    } else if (tab.value === 'audit') {
      const result = await api.auditLogs(f)
      if (current !== generation) return
      auditRows.value = result.items
      total.value = result.total
    } else if (tab.value === 'reconciliation') {
      const result = await api.reconciliation(f)
      if (current !== generation) return
      reconciliationRows.value = result.items
      total.value = result.total
    } else {
      const result = await api.trend(props.customer, f, abort.signal)
      if (current !== generation) return
      trend.value = result
    }
  } catch (err) {
    if (
      current === generation &&
      !(err as { code?: string }).code?.includes('CANCELED')
    )
      error.value = errorMessage(err)
  } finally {
    if (current === generation) loading.value = false
  }
}
function applyFilters() {
  page.value = 1
  void load()
}
function previousPage() {
  page.value--
  void load()
}
function nextPage() {
  page.value++
  void load()
}
function selectTab(value: Tab) {
  tab.value = value
  query.status = ''
  page.value = 1
  void load()
}
function tabKey(event: KeyboardEvent, value: Tab) {
  const index = tabs.value.indexOf(value)
  let target = index
  if (event.key === 'ArrowRight') target = (index + 1) % tabs.value.length
  else if (event.key === 'ArrowLeft')
    target = (index - 1 + tabs.value.length) % tabs.value.length
  else if (event.key === 'Home') target = 0
  else if (event.key === 'End') target = tabs.value.length - 1
  else return
  event.preventDefault()
  const next = tabs.value[target]!
  selectTab(next)
  document.getElementById(`${uid}-${next}`)?.focus()
}
async function loadReferences() {
  if (!form.organization_id) return
  if (action.value === 'pay' && actionRecord.value)
    paymentFiles.value = (
      await api.attachments('expenses', actionRecord.value.id, false)
    ).filter((f) => f.status === 'COMPLETE')
  const kinds = new Set<AccountingKind>(
    formFields.value.flatMap((f) => (f.reference ? [f.reference] : [])),
  )
  if (formKind.value === 'vcc-transactions' || action.value === 'allocate')
    kinds.add('vcc-cards')
  for (const kind of kinds)
    references[kind] = (
      await api.listRecords(kind, false, {
        organization_id: form.organization_id,
        page_size: 100,
      })
    ).items.filter((r) => r.status !== 'VOID')
}
function setupForm(kind: AccountingKind, record?: AccountingRecord) {
  formKind.value = kind
  form.organization_id = record?.organization_id || selectedOrg.value
  form.parent_id = record?.parent_id || ''
  form.currency = record?.currency || query.currency || 'CNY'
  form.amount = record?.amount || (kind === 'vcc-cards' ? '0' : '')
  form.version = record?.version
  formDate.value = localDateTime(record?.occurred_at)
  form.details = {}
  for (const field of fields[kind]) {
    const previous = record?.details[field.key]
    form.details[field.key] =
      previous ??
      (field.type === 'boolean'
        ? false
        : field.type === 'select'
          ? field.options?.[0] || ''
          : '')
  }
  if (kind === 'transfers' && !record) {
    form.details.tax_rate = '0'
    form.details.tax_inclusive = true
  }
  selectedFile.value = null
  dialogError.value = ''
  void loadReferences().catch((err) => {
    dialogError.value = errorMessage(err)
  })
}
function openCreate(kind: AccountingKind) {
  action.value = 'create'
  actionRecord.value = null
  setupForm(kind)
}
function openEdit() {
  if (!detailRecord.value) return
  action.value = 'edit'
  actionRecord.value = detailRecord.value
  setupForm(detailKind.value, detailRecord.value)
}
function openAction(
  value: string,
  record: AccountingRecord,
  kind: AccountingKind,
) {
  action.value = value
  actionRecord.value = record
  formKind.value = kind
  dialogError.value = ''
  command.amount = record.amount
  command.card_id = ''
  command.reason = ''
  command.reference = ''
  form.organization_id = record.organization_id
  if (value === 'pay')
    setupForm('expense-payments', { ...record, amount: '', details: {} })
  else if (value === 'allocate')
    void loadReferences().catch((err) => {
      dialogError.value = errorMessage(err)
    })
}
function openRecordFromTab(record: AccountingRecord) {
  if (isRecordTab(tab.value)) void openDetail(record, tab.value)
}
async function openDetail(record: AccountingRecord, kind: AccountingKind) {
  detailRecord.value = record
  detailKind.value = kind
  recordFiles.value = []
  allocationRows.value = []
  try {
    if (!props.customer && ['transfers', 'vcc-cards'].includes(kind))
      allocationRows.value = await api.allocationHistory(kind, record.id)
    recordFiles.value =
      props.customer && kind === 'vcc-cards'
        ? []
        : await api.attachments(kind, record.id, props.customer)
  } catch (err) {
    error.value = errorMessage(err)
  }
}
function chooseFile(event: Event) {
  selectedFile.value = (event.target as HTMLInputElement).files?.[0] || null
}
async function saveAction() {
  busy.value = true
  dialogError.value = ''
  try {
    const record = actionRecord.value
    let path = '',
      payload: unknown = {},
      method: 'post' | 'patch' = 'post'
    if (['create', 'edit', 'pay'].includes(action.value)) {
      form.occurred_at = new Date(formDate.value).toISOString()
      payload = { ...form, details: { ...form.details } }
      path =
        action.value === 'pay'
          ? `expenses/${record!.id}/payments`
          : action.value === 'edit'
            ? `${formKind.value}/${record!.id}`
            : formKind.value
      if (action.value === 'edit') method = 'patch'
    } else if (action.value === 'allocate') {
      path = `transfers/${record!.id}/allocations`
      payload = { card_id: command.card_id, amount: command.amount }
    } else if (['match', 'exclude', 'adjust'].includes(action.value)) {
      path = `reconciliation/${action.value === 'adjust' ? 'adjustments' : action.value}`
      payload = {
        object_kind: formKind.value,
        object_id: record!.id,
        currency: record!.currency,
        amount: command.amount,
        reference: command.reference,
        reason: command.reason,
      }
    } else {
      path = `${formKind.value}/${record!.id}/${action.value}`
      payload = { reason: command.reason }
    }
    const saved = (await api.mutate(
      path,
      payload,
      requestKey(path, payload),
      method,
    )) as AccountingRecord
    if (
      selectedFile.value &&
      (action.value === 'create' || action.value === 'edit')
    )
      await api.upload(selectedFile.value, formKind.value, saved)
    requestKey.reset()
    action.value = ''
    detailRecord.value = null
    await load()
  } catch (err) {
    dialogError.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
async function shareRecord() {
  const r = detailRecord.value
  if (!r) return
  await perform(async () => {
    const payload = { customer_visible: !r.customer_visible }
    const path = `${detailKind.value}/${r.id}/sharing`
    detailRecord.value = (await api.mutate(
      path,
      payload,
      requestKey(path, payload),
      'patch',
    )) as AccountingRecord
    await load()
  })
}
async function perform(fn: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await fn()
  } catch (err) {
    error.value = errorMessage(err)
  } finally {
    busy.value = false
  }
}
async function uploadExisting(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file || !detailRecord.value) return
  const r = detailRecord.value
  await perform(async () => {
    await api.upload(file, detailKind.value, r)
    recordFiles.value = await api.attachments(detailKind.value, r.id, false)
  })
  ;(event.target as HTMLInputElement).value = ''
}
async function downloadFile(file: AccountingAttachment) {
  await perform(async () => {
    const url = await api.download(file.id, props.customer)
    window.open(url, '_blank', 'noopener,noreferrer')
  })
}
async function shareFile(file: AccountingAttachment) {
  await perform(async () => {
    const payload = { customer_visible: !file.customer_visible },
      path = `attachments/${file.id}/sharing`
    await api.mutate(path, payload, requestKey(path, payload), 'patch')
    file.customer_visible = !file.customer_visible
  })
}
async function deleteFile(file: AccountingAttachment) {
  if (!window.confirm(`${t('accounting.delete')}: ${file.filename}?`)) return
  await perform(async () => {
    const path = `attachments/${file.id}`
    await api.mutate(path, {}, requestKey(path, {}), 'delete')
    recordFiles.value = recordFiles.value.filter((f) => f.id !== file.id)
  })
}
async function exportRecords() {
  if (!isRecordTab(tab.value)) return
  const kind = tab.value
  await perform(async () => {
    saveAs(await api.exportCSV(kind, filters()), `accounting-${kind}.csv`)
  })
}
function onFocus() {
  if (props.customer) void load()
}
onMounted(() => {
  void load()
  window.addEventListener('focus', onFocus)
})
onUnmounted(() => {
  generation++
  abort?.abort()
  window.removeEventListener('focus', onFocus)
})
</script>
