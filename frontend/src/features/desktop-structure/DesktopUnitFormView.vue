<template>
  <StructurePage :title="t(kind === 'departments' ? id ? 'desktopStructure.editDepartment' : 'desktopStructure.addDepartment' : id ? 'desktopStructure.editPosition' : 'desktopStructure.addPosition')" :subtitle="organization?.name" :description="t(id ? 'desktopStructure.unitEditHint' : 'desktopStructure.setupHint')" :loading="loading" :error="error" :busy="busy" :dirty="guard.dirty.value" @back="guard.leave(returnTo)" @retry="reload">
    <template v-if="organization">
      <p v-if="organization.status !== 'active'" class="structure-notice" role="status">{{ t('desktopStructure.readonly') }}</p>
      <form class="structure-panel structure-form" :aria-busy="busy" @submit.prevent="save">
        <p class="structure-caption">{{ t('desktopStructure.requiredHint') }}</p>
        <fieldset class="structure-section" :disabled="busy || organization.status !== 'active'">
          <legend class="structure-section-title mb-4">{{ t('desktopStructure.basicInfo') }}</legend>
          <label class="structure-field"><span>{{ t(kind === 'departments' ? 'desktopStructure.departmentName' : 'desktopStructure.positionName') }} <span aria-hidden="true">*</span></span><input v-model="form.name" name="unit-name" class="input" required maxlength="100" aria-describedby="unit-name-hint" /><p id="unit-name-hint" class="structure-caption">{{ t(kind === 'departments' ? 'desktopStructure.departmentNameHint' : 'desktopStructure.positionNameHint') }}</p></label>
          <label v-if="kind === 'positions'" class="structure-field"><span>{{ t('desktopStructure.department') }} <span aria-hidden="true">*</span></span><select v-model="form.department_id" name="department" class="input" required :disabled="!!id" :aria-describedby="id ? 'department-locked-hint' : 'unit-department-hint'"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select><p v-if="id" id="department-locked-hint" class="structure-caption"><bdi>{{ selectedDepartment?.name }}</bdi> · {{ t('desktopStructure.departmentLocked') }}</p><p v-else id="unit-department-hint" class="structure-caption">{{ selectedDepartment?.name || t(departments.length ? 'desktopStructure.assignmentHint' : 'desktopStructure.noDepartments') }}</p></label>
          <button v-if="kind === 'positions' && !id" type="button" class="structure-link" @click="createDepartment"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('desktopStructure.createDepartment') }}</button>
          <label v-if="id" class="structure-field"><span>{{ t('common.status') }}</span><select v-model="form.status" name="unit-status" class="input" aria-describedby="unit-status-hint"><option value="active">{{ t('common.active') }}</option><option value="disabled">{{ t('common.disabled') }}</option></select><p id="unit-status-hint" class="structure-caption">{{ t('desktopStructure.unitStatusHint') }}</p></label>
        </fieldset>
        <div v-if="id" class="structure-notice"><Icon name="infoCircle" size="md" class="shrink-0" aria-hidden="true" /><p class="min-w-0 flex-1">{{ t('desktopStructure.disableHint') }}</p></div>
        <div class="structure-actions"><button type="button" class="btn btn-secondary" :disabled="busy" @click="guard.leave(returnTo)">{{ t('common.cancel') }}</button><button type="submit" class="btn btn-primary" :disabled="organization.status !== 'active'" :aria-disabled="busy || undefined"><Icon v-if="busy" name="clock" size="sm" aria-hidden="true" />{{ t('common.save') }}</button></div>
      </form>
    </template>
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import Icon from '@/components/icons/Icon.vue'
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
const selectedDepartment = computed(() => departments.value.find(d => d.public_id === form.department_id))
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
async function reload() { if (!guard.dirty.value || window.confirm(t('desktopStructure.reload'))) await load() }
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
