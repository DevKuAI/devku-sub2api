<template><AppLayout><section class="card p-6 space-y-4"><h1 class="text-xl font-semibold">{{ t('biNative.users') }}</h1><p>{{ t('biNative.identityHint') }}</p><p>{{ t('biNative.inviteHint') }}</p><RouterLink class="btn btn-primary" to="/desktop/organization?tab=grants">{{ t('biNative.invitations') }}</RouterLink></section></AppLayout></template>
<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
const { t } = useI18n()
</script>
