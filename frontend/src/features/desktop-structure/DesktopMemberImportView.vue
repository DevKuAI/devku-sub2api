<template>
  <StructurePage :title="t('admin.desktop.memberImport.button')" :subtitle="organization?.name" :loading="loading" :error="error" :busy="busy" @back="leave" @retry="load">
    <DesktopMemberImportDialog v-if="organization" :show="true" :organization-id="organization.public_id" :self-managed="!!selfManaged" :available-slots="Math.max(0, organization.member_limit - organization.member_count)" :organization-active="organization.status === 'active'" @close="leave" @imported="load" @busy="busy = $event" @dirty="dirty = $event" />
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import DesktopMemberImportDialog from '@/components/desktop/DesktopMemberImportDialog.vue'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn } from '@/api/desktopStructure'
import type { DesktopOrganization } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), router = useRouter(), { t } = useI18n()
const organization = ref<DesktopOrganization | null>(null), loading = ref(true), error = ref(''), busy = ref(false), dirty = ref(false)
const orgId = computed(() => String(route.params.organizationId || '')), base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged))
let generation = 0
const mayLeave = () => !busy.value && (!dirty.value || window.confirm(t('desktopStructure.discard')))
onBeforeRouteLeave(mayLeave); onBeforeRouteUpdate(mayLeave)
async function leave() { if (!busy.value) await router.push(safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`)) }
async function load() {
  const g = ++generation; loading.value = !organization.value; error.value = ''
  try { const org = await desktopStructure(orgId.value, props.selfManaged).organization(); if (g === generation) organization.value = org }
  catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
watch(() => [orgId.value, props.selfManaged], () => { organization.value = null; void load() }, { immediate: true })
const beforeUnload = (event: BeforeUnloadEvent) => { if (busy.value || dirty.value) { event.preventDefault(); event.returnValue = '' } }
window.addEventListener('beforeunload', beforeUnload)
onBeforeUnmount(() => { generation++; window.removeEventListener('beforeunload', beforeUnload) })
</script>
