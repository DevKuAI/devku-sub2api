<template>
  <section v-if="enabled" class="card p-6">
    <h2 class="text-lg font-semibold">{{ t('biNative.users') }}</h2>
    <p class="mt-2 text-sm text-gray-500 dark:text-dark-400">{{ t('biNative.identityHint') }}</p>
    <RouterLink to="/bi/access" class="btn btn-secondary mt-4">{{ t('biNative.invitations') }}</RouterLink>
  </section>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { isBIEnabled } from '@/api/bi'

const { t } = useI18n()
const enabled = ref(false)
onMounted(async () => {
  try { enabled.value = await isBIEnabled() } catch { enabled.value = false }
})
</script>
