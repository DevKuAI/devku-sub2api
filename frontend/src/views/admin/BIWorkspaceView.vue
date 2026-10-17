<template>
  <AppLayout>
    <div class="bi-operations mx-auto max-w-6xl space-y-6">
      <header class="space-y-3">
        <router-link class="btn btn-secondary" :to="backLocation">{{ t(isList ? 'common.back' : 'biNative.backToAccess') }}</router-link>
        <h1 class="break-words text-xl font-semibold text-gray-900 dark:text-white">{{ organization?.name || t('biNative.accessManagement') }}</h1>
        <p class="bi-caption">{{ t('biNative.identityHint') }}</p>
      </header>
      <p v-if="loading" class="card bi-panel" role="status">{{ t('common.loading') }}</p>
      <div v-else-if="error" class="card bi-panel bi-error" role="alert"><p>{{ error }}</p><button class="btn btn-secondary mt-3" @click="load">{{ t('biNative.retry') }}</button></div>
      <RouterView v-else-if="organization" v-slot="{ Component }">
        <component :is="Component" :key="route.path" v-bind="childProps" @created="created" />
      </RouterView>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getOrganization, type DesktopOrganization } from '@/api/admin/desktop'
import desktopOrganizationAPI from '@/api/desktopOrganization'
import type { BIIssuedInvitation } from '@/api/bi'
import { biAccessPath, biAccessQuery } from '@/components/bi/navigation'
import '@/styles/biOperations.css'

const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), router = useRouter(), { t } = useI18n()
const organization = ref<DesktopOrganization | null>(null), loading = ref(true), error = ref('')
// Credentials live only in this mounted workspace, never in a URL or persistent storage.
const issued = shallowRef<BIIssuedInvitation>()
const isList = computed(() => route.name === 'AdminBIAccess' || route.name === 'OrganizationBIAccess')
const isResult = computed(() => Boolean(route.params.invitationId))
const base = computed(() => biAccessPath(organization.value?.public_id || String(route.params.organizationId || ''), props.selfManaged))
const query = computed(() => biAccessQuery(route.query.from))
const backLocation = computed(() => {
  if (!isList.value) return { path: base.value, query: query.value }
  if (!props.selfManaged && query.value.from !== 'organization') return { path: '/admin/desktop/operations', query: { tab: query.value.from } }
  return { path: props.selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(String(route.params.organizationId))}`, query: { tab: 'grants' } }
})
const childProps = computed(() => ({
  organizationId: organization.value!.public_id,
  selfManaged: props.selfManaged,
  ...(isResult.value ? { invitation: issued.value } : {}),
}))
let generation = 0
async function load() {
  const g = ++generation
  loading.value = true; error.value = ''; organization.value = null; issued.value = undefined
  try {
    const value = props.selfManaged ? await desktopOrganizationAPI.getOrganization() : await getOrganization(String(route.params.organizationId))
    if (g !== generation) return
    if (!value || value.status !== 'active') { error.value = t('biNative.organizationUnavailable'); return }
    organization.value = value
  } catch { if (g === generation) error.value = t('biNative.organizationUnavailable') }
  finally { if (g === generation) loading.value = false }
}
async function created(value: BIIssuedInvitation) {
  if (value.organization_id !== organization.value?.public_id) return
  issued.value = value
  await router.replace({ path: `${base.value}/invitations/${encodeURIComponent(value.id)}`, query: query.value })
}
watch([() => route.params.organizationId, () => props.selfManaged], load, { immediate: true })
watch(() => route.path, () => { if (!isResult.value) issued.value = undefined })
onBeforeUnmount(() => { generation++; issued.value = undefined })
</script>
