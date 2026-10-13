<template>
  <AppLayout>
    <AccountingWorkspace :customer="customer" />
  </AppLayout>
</template>
<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import AccountingWorkspace from './AccountingWorkspace.vue'
defineProps<{ customer?: boolean }>()
</script>
