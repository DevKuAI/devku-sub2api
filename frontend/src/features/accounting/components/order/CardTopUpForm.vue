<template>
  <form class="accounting-entry-form" novalidate :aria-busy="busy || undefined" data-testid="card-top-up" @submit.prevent="submit">
    <p v-if="error" ref="errorRef" role="alert" tabindex="-1" class="accounting-error sm:col-span-2 lg:col-span-3">{{ error }}</p>
    <label class="block min-w-0">
      <span class="input-label">{{ t('accounting.entries.fields.amount') }}</span>
      <span class="accounting-money-input">
        <input v-model="amount" type="text" class="input w-full tabular-nums" inputmode="decimal" pattern="\d+(\.\d{1,6})?" required autocomplete="off" />
        <span aria-hidden="true">{{ card.currency }}</span>
      </span>
    </label>
    <label class="block min-w-0">
      <span class="input-label">{{ t('accounting.entries.fields.occurred_at') }}</span>
      <input v-model="date" type="date" class="input w-full" required />
    </label>
    <label class="block min-w-0">
      <span class="input-label">{{ t('accounting.entries.fields.remark') }}<span class="accounting-optional">{{ t('accounting.form.optional') }}</span></span>
      <input v-model="remark" class="input w-full" maxlength="4000" autocomplete="off" />
    </label>
    <div class="accounting-entry-actions sm:col-span-2 lg:col-span-3">
      <button type="button" class="btn btn-secondary btn-sm" :disabled="busy" @click="emit('cancel')">{{ t('common.cancel') }}</button>
      <button type="submit" class="btn btn-primary btn-sm" :disabled="busy">
        <span v-if="busy" class="accounting-spinner" aria-hidden="true" />{{ t('accounting.entries.topUp') }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as api from '../../api'
import { compareMoney } from '../../money'
import { errorMessage, localDate, toISO } from '../../shared'
import type { AccountingRecord } from '../../types'

// Our own money loaded onto a card; it defaults to whatever the card is short.
const props = defineProps<{ card: AccountingRecord }>()
const emit = defineEmits<{ saved: []; cancel: [] }>()
const { t, te } = useI18n()
const balance = String(props.card.details.available_balance || '0')
const amount = ref(compareMoney(balance, '0') < 0 ? balance.replace('-', '') : '')
const date = ref(localDate())
const remark = ref('')
const busy = ref(false)
const error = ref('')
const errorRef = ref<HTMLElement>()
const requestKey = api.stableRequest()

async function submit(event: Event) {
  const form = event.target as HTMLFormElement
  if (!form.checkValidity()) {
    form.reportValidity()
    return
  }
  busy.value = true
  error.value = ''
  try {
    const path = `vcc-cards/${props.card.id}/top-ups`
    const payload = { amount: amount.value, occurred_at: toISO(date.value), remark: remark.value.trim() }
    await api.mutate(path, payload, requestKey(path, payload))
    requestKey.reset()
    emit('saved')
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
    await nextTick()
    errorRef.value?.focus()
  } finally {
    busy.value = false
  }
}
</script>
