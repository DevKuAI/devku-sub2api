<template>
  <AppLayout>
    <AccountingWorkspace
      :key="organizationId"
      :customer="customer"
      :organization-id="organizationId"
    />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import AccountingWorkspace from './AccountingWorkspace.vue'
defineProps<{ customer?: boolean }>()
const route = useRoute()
const organizationId = computed(() =>
  typeof route.params.organizationId === 'string'
    ? route.params.organizationId
    : '',
)
</script>
