<template>
  <AppLayout>
    <OrganizationDirectory v-if="!customer && !organizationId" />
    <AccountingWorkspace v-else :key="organizationId" :customer="customer" :administrator="auth.isAdmin" :organization-id="organizationId" />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import AppLayout from '@/components/layout/AppLayout.vue'
import AccountingWorkspace from './AccountingWorkspace.vue'
import OrganizationDirectory from './components/OrganizationDirectory.vue'
defineProps<{ customer?: boolean }>()
const auth = useAuthStore()
const route = useRoute()
const organizationId = computed(() => (typeof route.params.organizationId === 'string' ? route.params.organizationId : ''))
</script>
