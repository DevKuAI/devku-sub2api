<template>
  <form class="flex flex-wrap items-end gap-3" data-testid="channel-bind-form" @submit.prevent="submit">
    <label class="block min-w-48">
      <span class="input-label">{{ t('accounting.home.organization') }}</span>
      <select v-model="organization" class="input w-full" required data-testid="channel-bind-organization" @change="loadCards">
        <option value="">{{ t('accounting.form.select') }}</option>
        <option v-for="item in organizations" :key="item.id" :value="item.id">{{ item.name }}</option>
      </select>
    </label>
    <label class="block min-w-56">
      <span class="input-label">{{ t('accounting.channels.binding.card') }}</span>
      <select v-model="card" class="input w-full" required :disabled="!organization" data-testid="channel-bind-card">
        <option value="">{{ t(organization && !cards.length ? 'accounting.channels.binding.noCard' : 'accounting.form.select') }}</option>
        <option v-for="item in cards" :key="item.id" :value="item.id">
          {{ cardLabel(item) }} · {{ item.currency }}{{ item.details.provider_card_id ? '' : ` · ${t('accounting.channels.binding.noCardId')}` }}
        </option>
      </select>
    </label>
    <div class="flex gap-2">
      <button type="submit" class="btn btn-primary btn-sm" :disabled="busy || !card">{{ t('accounting.channels.binding.bind') }}</button>
      <button v-if="bound" type="button" class="btn btn-secondary btn-sm" :disabled="busy" data-testid="channel-unbind" @click="save('')">{{ t('accounting.channels.binding.unbind') }}</button>
      <button type="button" class="btn btn-ghost btn-sm" :disabled="busy" @click="emit('close')">{{ t('common.cancel') }}</button>
    </div>
    <p class="w-full text-xs text-gray-500">{{ t('accounting.channels.binding.hint', { ref: channelCard.card_ref }) }}</p>
    <p v-if="error" role="alert" class="accounting-error w-full">{{ error }}</p>
  </form>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as api from '../../api'
import { cardLabel } from '../../orderEntries'
import { errorMessage } from '../../shared'
import type { AccountingOrganization, AccountingRecord, ChannelCardSpend } from '../../types'

// Binds a statement card to the organization card it was issued as. Its lines are booked (or moved) right away.
const props = defineProps<{ channelCard: ChannelCardSpend }>()
const emit = defineEmits<{ saved: []; close: [] }>()
const { t, te } = useI18n()
const organizations = ref<AccountingOrganization[]>([])
const organization = ref(props.channelCard.organization_id || '')
const cards = ref<AccountingRecord[]>([])
const card = ref('')
const bound = Boolean(props.channelCard.vcc_card_id)
const busy = ref(false)
const error = ref('')
const requestKey = api.stableRequest()

// Cards of the chosen organization in the statement's currency; one already holding another card ID cannot take it.
async function loadCards() {
  card.value = ''
  cards.value = []
  if (!organization.value) return
  try {
    const result = await api.listRecords('vcc-cards', false, { organization_id: organization.value, page_size: 100 })
    cards.value = result.items.filter(
      (item) =>
        item.status !== 'VOID' &&
        item.currency === props.channelCard.currency &&
        item.id !== props.channelCard.vcc_card_id &&
        (!item.details.provider_card_id || item.details.provider_card_id === props.channelCard.card_ref),
    )
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
}
async function save(target: string) {
  busy.value = true
  error.value = ''
  const path = `channel-cards/${props.channelCard.id}`
  try {
    await api.bindChannelCard(props.channelCard.id, target, requestKey(path, target))
    requestKey.reset()
    emit('saved')
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
function submit() {
  void save(card.value)
}

onMounted(async () => {
  try {
    organizations.value = await api.organizations({})
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  }
  if (organization.value) await loadCards()
})
</script>
