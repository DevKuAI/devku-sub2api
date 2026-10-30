<template>
  <nav class="accounting-tabs" :aria-label="t('accounting.title')">
    <router-link
      v-for="item in items"
      :key="item.name"
      :to="{ name: item.name }"
      :aria-current="current === item.name ? 'location' : undefined"
      :data-testid="`accounting-nav-${item.key}`"
    >
      {{ t(`accounting.home.nav.${item.key}`) }}
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

// Where finance works: orders, the cards those orders use, the vendor lists records choose from and our receiving accounts.
defineProps<{ current: string }>()
const { t } = useI18n()
const items = [
  { key: 'orders', name: 'AdminAccounting' },
  { key: 'cards', name: 'AdminAccountingCards' },
  { key: 'vendors', name: 'AdminAccountingVendors' },
  { key: 'accounts', name: 'AdminAccountingReceivingAccounts' },
]
</script>
