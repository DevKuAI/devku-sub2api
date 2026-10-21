<template>
  <StructurePage :title="t(id ? 'desktopStructure.edit' : 'desktopStructure.create') + ' · ' + t(`desktopStructure.${kind}`)" :subtitle="organization?.name" :loading="loading" :error="error" :busy="busy" @back="guard.leave(returnTo)" @retry="reload">
    <form v-if="organization" class="card max-w-2xl space-y-5 p-6" :aria-busy="busy" @submit.prevent="save">
      <label class="block space-y-1"><span class="block">{{ t('common.name') }}</span><input v-model="form.name" class="input w-full" required maxlength="100" :disabled="busy || organization.status !== 'active'" /></label>
      <label v-if="kind === 'positions'" class="block space-y-1"><span class="block">{{ t('desktopStructure.department') }}</span><select v-model="form.department_id" class="input w-full" required :disabled="!!id || busy || organization.status !== 'active'"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select></label>
      <button v-if="kind === 'positions' && !id" type="button" class="text-primary-600 hover:underline" :disabled="busy" @click="createDepartment">{{ t('desktopStructure.createDepartment') }}</button>
      <label v-if="id" class="block space-y-1"><span class="block">{{ t('common.status') }}</span><select v-model="form.status" class="input w-full" :disabled="busy || organization.status !== 'active'"><option value="active">{{ t('common.active') }}</option><option value="disabled">{{ t('common.disabled') }}</option></select></label>
      <p class="text-sm text-gray-500">{{ t('desktopStructure.disableHint') }}</p>
      <div class="flex flex-wrap gap-3"><button type="button" class="btn btn-secondary" :disabled="busy" @click="guard.leave(returnTo)">{{ t('common.cancel') }}</button><button type="submit" class="btn btn-primary" :disabled="busy || organization.status !== 'active'">{{ t(busy ? 'common.saving' : 'common.save') }}</button></div>
    </form>
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import { useDesktopForm } from './useDesktopForm'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, structureIdempotencyKey, type OrganizationUnit, type UnitKind } from '@/api/desktopStructure'
import type { DesktopOrganization } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean; kind: UnitKind }>()
const route = useRoute(), { t } = useI18n()
const id = computed(() => String(route.params.unitId || '')), orgId = computed(() => String(route.params.organizationId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged))
const returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}/structure`))
const form = reactive({ name: '', department_id: '', status: 'active' as 'active' | 'disabled' })
const loading = ref(true), busy = ref(false), error = ref(''), organization = ref<DesktopOrganization | null>(null), departments = ref<OrganizationUnit[]>([])
const guard = useDesktopForm(() => form, () => busy.value, () => organization.value?.public_id || orgId.value)
let revision = 0, generation = 0, key = structureIdempotencyKey(), keyPayload = ''
async function load() {
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged)
  loading.value = true; error.value = ''
  try {
    const [org, deps, unit] = await Promise.all([api.organization(), api.all('departments'), id.value ? api.unit(props.kind, id.value) : undefined])
    if (g !== generation) return
    organization.value = org; departments.value = deps.filter(d => d.status === 'active' || d.public_id === unit?.department?.public_id)
    Object.assign(form, { name: unit?.name || '', department_id: unit?.department?.public_id || String(route.query.department_id || ''), status: unit?.status || 'active' })
    revision = unit?.revision || 0; const draft = guard.restore(); guard.clean(); if (draft && !id.value) Object.assign(form, draft)
  } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
async function createDepartment() { await guard.child(`${base.value}/departments/new?return_to=${encodeURIComponent(route.fullPath)}`) }
async function reload() { if (window.confirm(t('desktopStructure.reload'))) await load() }
async function save() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    const payload = { name: form.name.trim(), ...(id.value ? { status: form.status, expected_revision: revision } : props.kind === 'positions' ? { department_id: form.department_id } : {}) }
    const serialized = JSON.stringify(payload); if (keyPayload && keyPayload !== serialized) key = structureIdempotencyKey(); keyPayload = serialized
    await desktopStructure(orgId.value, props.selfManaged).save(props.kind, id.value, payload, key)
    await guard.leave(returnTo.value, true)
  } catch (e) { error.value = desktopStructureError(e, t) }
  finally { busy.value = false }
}
watch(() => [orgId.value, id.value, props.kind, props.selfManaged], load, { immediate: true })
onBeforeUnmount(() => generation++)
</script>
