<template>
  <StructurePage :title="t('desktopStructure.assign')" :subtitle="member?.name" :loading="loading" :error="error" :busy="busy" @back="guard.leave(returnTo)" @retry="reload">
    <form v-if="member" class="card max-w-2xl space-y-5 p-6" :aria-busy="busy" @submit.prevent="save">
      <p>{{ t('desktopStructure.current') }}: {{ member.department?.name || '—' }} / {{ member.position?.name || t('desktopStructure.unassigned') }}</p>
      <label class="block space-y-1"><span class="block">{{ t('desktopStructure.department') }}</span><select v-model="form.department_id" class="input w-full" required :disabled="busy" @change="form.position_id = ''"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select></label>
      <label class="block space-y-1"><span class="block">{{ t('desktopStructure.position') }}</span><select v-model="form.position_id" class="input w-full" required :disabled="busy || !form.department_id"><option value="">{{ t('desktopStructure.selectPosition') }}</option><option v-for="p in availablePositions" :key="p.public_id" :value="p.public_id">{{ p.name }}</option></select></label>
      <label class="block space-y-1"><span class="block">{{ t('desktopStructure.reason') }}</span><textarea v-model="form.reason" class="input w-full" maxlength="500" rows="3" :disabled="busy" /></label>
      <p class="text-sm text-gray-500">{{ t('desktopStructure.historyHint') }}</p>
      <div class="flex gap-3"><button class="btn btn-secondary" type="button" :disabled="busy" @click="guard.leave(returnTo)">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" :disabled="busy || organization?.status !== 'active'">{{ t(busy ? 'common.saving' : 'common.save') }}</button></div>
    </form>
    <section class="card space-y-3 p-6"><h2 class="font-semibold">{{ t('desktopStructure.history') }}</h2><ol class="space-y-3"><li v-for="entry in history" :key="entry.position_revision" class="border-b pb-3"><p>{{ entry.department.name }} / {{ entry.position.name }}</p><p class="text-sm text-gray-500">{{ formatDateTime(entry.started_at) }} — {{ entry.ended_at ? formatDateTime(entry.ended_at) : t('desktopStructure.current') }}</p><p>{{ entry.reason }}</p></li></ol><p v-if="!history.length">{{ t('desktopStructure.historyUnknown') }}</p><Pagination v-model:page="page" :page-size="20" :total="total" :show-page-size-selector="false" @update:page="loadHistory" /></section>
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useDesktopForm } from './useDesktopForm'
import { formatDateTime } from '@/utils/format'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, structureIdempotencyKey, type OrganizationUnit, type PositionHistory } from '@/api/desktopStructure'
import type { DesktopMember, DesktopOrganization } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), { t } = useI18n()
const orgId = computed(() => String(route.params.organizationId || '')), id = computed(() => String(route.params.memberId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged)), returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`))
const member = ref<DesktopMember>(), organization = ref<DesktopOrganization | null>(null), departments = ref<OrganizationUnit[]>([]), positions = ref<OrganizationUnit[]>([]), history = ref<PositionHistory[]>([])
const form = reactive({ department_id: '', position_id: '', reason: '' }), availablePositions = computed(() => positions.value.filter(p => p.department?.public_id === form.department_id))
const loading = ref(true), busy = ref(false), error = ref(''), page = ref(1), total = ref(0), guard = useDesktopForm(() => form, () => busy.value, () => organization.value?.public_id || orgId.value)
let generation = 0, key = structureIdempotencyKey(), keyPayload = ''
async function load() {
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged); loading.value = true; error.value = ''
  try {
    const [m, org, ds, ps, hs] = await Promise.all([api.member(id.value), api.organization(), api.all('departments', { status: 'active' }), api.all('positions', { status: 'active' }), api.history(id.value)])
    if (g !== generation) return
    member.value = m; organization.value = org; departments.value = ds; positions.value = ps; history.value = hs.items; total.value = hs.total; page.value = 1
    Object.assign(form, { department_id: m.department?.public_id || '', position_id: m.position?.public_id || '', reason: '' }); guard.clean()
  } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
async function reload() { if (window.confirm(t('desktopStructure.reload'))) await load() }
async function loadHistory() { const g = generation; try { const data = await desktopStructure(orgId.value, props.selfManaged).history(id.value, page.value); if (g === generation) { history.value = data.items; total.value = data.total } } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) } }
async function save() {
  if (!member.value || busy.value) return
  busy.value = true; error.value = ''
  try {
    const payload = { position_id: form.position_id, expected_revision: member.value.position_revision || 0, reason: form.reason.trim() }, serialized = JSON.stringify(payload)
    if (keyPayload && keyPayload !== serialized) key = structureIdempotencyKey(); keyPayload = serialized
    await desktopStructure(orgId.value, props.selfManaged).assign(id.value, payload, key); await guard.leave(returnTo.value, true)
  } catch (e) { error.value = desktopStructureError(e, t) }
  finally { busy.value = false }
}
watch(() => [orgId.value, id.value, props.selfManaged], load, { immediate: true })
onBeforeUnmount(() => generation++)
</script>
