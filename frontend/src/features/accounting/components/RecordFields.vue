<template>
  <dl class="accounting-fields">
    <div v-for="field in fields" :key="field.key" :class="{ 'sm:col-span-2 lg:col-span-3': field.wide }">
      <dt>{{ field.label }}</dt>
      <dd>{{ field.value }}</dd>
    </div>
  </dl>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { detailFields } from '../shared'
import type { AccountingKind, AccountingRecord } from '../types'

const props = defineProps<{ kind: AccountingKind; record: AccountingRecord; customer?: boolean }>()
const { t, te } = useI18n()
const fields = computed(() => detailFields(props.kind, props.record, props.customer, t, te))
</script>
