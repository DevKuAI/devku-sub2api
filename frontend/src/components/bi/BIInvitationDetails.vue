<template>
  <div class="card bi-panel bi-access">
    <BIInvitationResult v-if="current" :invitation="current" :organization-id="organizationId" :self-managed="selfManaged" />
    <div v-else class="space-y-4" role="status"><h2 class="bi-heading">{{ t('biNative.invitationResult') }}</h2><p class="bi-caption">{{ t('biNative.inviteCredentialsHint') }}</p></div>
  </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { BIIssuedInvitation } from '@/api/bi'
import BIInvitationResult from './BIInvitationResult.vue'
const props = defineProps<{ organizationId: string; selfManaged?: boolean; invitation?: BIIssuedInvitation }>()
const route = useRoute(), { t } = useI18n()
const current = computed(() => props.invitation?.id === route.params.invitationId && props.invitation?.organization_id === props.organizationId ? props.invitation : undefined)
</script>
