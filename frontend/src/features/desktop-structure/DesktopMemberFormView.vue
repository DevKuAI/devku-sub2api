<template>
  <StructurePage :title="t(id ? 'admin.desktop.editMember' : 'admin.desktop.createMember')" :subtitle="organization?.name" :description="t(id ? 'desktopStructure.memberEditHint' : 'desktopStructure.memberDetails')" :loading="loading" :error="error" :busy="busy" :dirty="guard.dirty.value" @back="guard.leave(returnTo)" @retry="reload">
    <template v-if="organization">
      <p v-if="organization.status !== 'active'" class="structure-notice" role="status">{{ t('desktopStructure.readonly') }}</p>
      <div v-if="structureError" class="structure-error" role="alert"><p class="min-w-0 flex-1">{{ structureError }}</p><button type="button" class="btn btn-secondary" :disabled="structureLoading" @click="loadStructure">{{ t('desktopStructure.retryStructureFilters') }}</button></div>
      <form class="structure-panel structure-form" :aria-busy="busy" @submit.prevent="save">
        <p class="structure-caption">{{ t('desktopStructure.requiredHint') }}</p>
        <fieldset class="structure-section" :disabled="busy || organization.status !== 'active'">
          <legend class="structure-section-title mb-4">{{ t('desktopStructure.basicInfo') }}</legend>
          <div class="structure-grid">
            <label class="structure-field"><span>{{ t('admin.desktop.memberName') }} <span aria-hidden="true">*</span></span><input v-model="form.name" name="member-name" autocomplete="section-member name" class="input" required maxlength="100" /></label>
            <label class="structure-field"><span>{{ t('admin.desktop.phone') }} <span aria-hidden="true">*</span></span><input v-model="form.phone" name="member-phone" autocomplete="section-member tel" class="input" type="tel" required /></label>
          </div>
        </fieldset>
        <fieldset class="structure-section" :disabled="busy || organization.status !== 'active'">
          <legend class="structure-section-title mb-4">{{ t('desktopStructure.assignmentInfo') }}</legend>
          <p class="structure-caption">{{ t('desktopStructure.assignmentHint') }}</p>
          <template v-if="!id">
            <div class="structure-grid">
              <label class="structure-field"><span>{{ t('desktopStructure.department') }} <span aria-hidden="true">*</span></span><select v-model="form.department_id" name="department" class="input" required :disabled="structureLoading || !!structureError" aria-describedby="member-department-hint" @change="form.position_id = ''"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select><p id="member-department-hint" class="structure-caption">{{ departments.find(d => d.public_id === form.department_id)?.name || (structureLoading || structureError ? '' : t(departments.length ? 'desktopStructure.selectDepartmentFirst' : 'desktopStructure.noDepartments')) }}</p></label>
              <label class="structure-field"><span>{{ t('desktopStructure.position') }} <span aria-hidden="true">*</span></span><select v-model="form.position_id" name="position" class="input" required :disabled="!form.department_id || structureLoading || !!structureError" aria-describedby="member-position-hint"><option value="">{{ t('desktopStructure.selectPosition') }}</option><option v-for="p in availablePositions" :key="p.public_id" :value="p.public_id">{{ p.name }}</option></select><p id="member-position-hint" class="structure-caption">{{ selectedPosition?.name || (structureLoading || structureError ? '' : t(!form.department_id ? 'desktopStructure.selectDepartmentFirst' : !availablePositions.length ? 'desktopStructure.noPositions' : 'desktopStructure.selectPosition')) }}</p></label>
            </div>
            <button type="button" class="structure-link" @click="createPosition"><Icon name="plus" size="sm" aria-hidden="true" />{{ t('desktopStructure.createPosition') }}</button>
          </template>
          <div v-else class="structure-assignment space-y-3">
            <p class="font-medium"><bdi>{{ member?.department?.name || '—' }}</bdi> / <bdi>{{ member?.position?.name || t('desktopStructure.unassigned') }}</bdi></p>
            <router-link v-if="organization.status === 'active'" class="structure-link" :to="{ path: `${base}/members/${id}/position`, query: { return_to: route.fullPath } }">{{ t('desktopStructure.assign') }}<Icon name="arrowRight" size="sm" aria-hidden="true" /></router-link>
          </div>
        </fieldset>
        <fieldset class="structure-section" :disabled="busy || organization.status !== 'active'">
          <legend class="structure-section-title mb-4">{{ t('admin.desktop.budget.title') }}</legend>
          <p id="member-budget-hint" class="structure-caption">{{ t('admin.desktop.budget.hint') }}</p>
          <div class="structure-grid">
            <label v-for="field in budgetFields" :key="field" class="structure-field"><span>{{ t(`admin.desktop.budget.${field}`) }} <span class="font-normal">(USD)</span></span><input v-model.number="form.budget[field]" :name="field" type="number" inputmode="decimal" min="0" step="0.01" class="input tabular-nums" aria-describedby="member-budget-hint" /></label>
          </div>
        </fieldset>
        <label class="structure-field"><span>{{ t('admin.desktop.memberRemark') }} <span class="font-normal">· {{ t('desktopStructure.optional') }}</span></span><textarea v-model="form.remark" name="member-remark" class="input" rows="3" maxlength="500" :disabled="busy || organization.status !== 'active'" /></label>
        <div class="structure-actions"><button type="button" class="btn btn-secondary" :disabled="busy" @click="guard.leave(returnTo)">{{ t('common.cancel') }}</button><button type="submit" class="btn btn-primary" :disabled="organization.status !== 'active' || (!id && (structureLoading || !!structureError))" :aria-disabled="busy || undefined"><Icon v-if="busy" name="clock" size="sm" aria-hidden="true" />{{ t('common.save') }}</button></div>
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
import { useDesktopStructureOptions } from './useDesktopStructureOptions'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, structureIdempotencyKey } from '@/api/desktopStructure'
import type { DesktopOrganization, DesktopMember } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), { t } = useI18n()
const id = computed(() => String(route.params.memberId || '')), orgId = computed(() => String(route.params.organizationId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged))
const returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`))
const blank = () => ({ name: '', phone: '', remark: '', department_id: '', position_id: '', budget: { quota: 0, limit_1d: 0, limit_7d: 0 } })
const form = reactive(blank()), organization = ref<DesktopOrganization | null>(null), member = ref<DesktopMember>()
const structure = useDesktopStructureOptions()
const { departments, positions, loading: structureLoading, error: structureError } = structure
const availablePositions = computed(() => positions.value.filter(p => p.department?.public_id === form.department_id))
const selectedPosition = computed(() => positions.value.find(p => p.public_id === form.position_id))
const budgetFields = ['quota', 'limit_1d', 'limit_7d'] as const
const loading = ref(true), busy = ref(false), error = ref('')
const guard = useDesktopForm(() => form, () => busy.value, () => organization.value?.public_id || orgId.value)
let generation = 0, key = structureIdempotencyKey(), keyPayload = ''
function loadStructure() { return structure.load(desktopStructure(orgId.value, props.selfManaged), { status: 'active' }) }
async function load() {
  structure.reset()
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged); loading.value = true; error.value = ''
  try {
    const [org, , m] = await Promise.all([api.organization(), id.value ? undefined : loadStructure(), id.value ? api.member(id.value) : undefined])
    if (g !== generation) return
    organization.value = org; member.value = m
    Object.assign(form, blank(), m ? { name: m.name, phone: m.phone, remark: m.remark || '', budget: { quota: m.budget?.quota || 0, limit_1d: m.budget?.limit_1d || 0, limit_7d: m.budget?.limit_7d || 0 } } : {})
    const draft = guard.restore(); guard.clean(); if (draft && !id.value) Object.assign(form, draft)
  } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
async function reload() { if (!guard.dirty.value || window.confirm(t('desktopStructure.reload'))) await load() }
async function createPosition() { await guard.child(`${base.value}/positions/new?return_to=${encodeURIComponent(route.fullPath)}&department_id=${encodeURIComponent(form.department_id)}`) }
async function save() {
  if (busy.value || (!id.value && (structureLoading.value || structureError.value))) return
  if (!id.value && organization.value && organization.value.member_count >= organization.value.member_limit) { error.value = t('admin.desktop.errors.MEMBER_LIMIT_REACHED'); return }
  busy.value = true; error.value = ''
  try {
    const api = desktopStructure(orgId.value, props.selfManaged), input = { name: form.name.trim(), phone: form.phone.trim(), remark: form.remark.trim(), budget: { ...form.budget } }
    if (id.value && member.value) {
      const original = member.value
      const originalBudget = { quota: original.budget?.quota || 0, limit_1d: original.budget?.limit_1d || 0, limit_7d: original.budget?.limit_7d || 0 }
      const patch = { ...(input.name !== original.name ? { name: input.name } : {}), ...(input.phone !== original.phone ? { phone: input.phone } : {}), ...(input.remark !== (original.remark || '') ? { remark: input.remark } : {}), ...(JSON.stringify(input.budget) !== JSON.stringify(originalBudget) ? { budget: input.budget } : {}) }
      if (Object.keys(patch).length) await api.members.updateMember(orgId.value, id.value, patch)
    }
    else {
      const payload = { ...input, position_id: form.position_id }, serialized = JSON.stringify(payload)
      if (keyPayload && keyPayload !== serialized) key = structureIdempotencyKey(); keyPayload = serialized
      await api.members.createMember(orgId.value, payload, key)
    }
    await guard.leave(returnTo.value, true)
  } catch (e) { error.value = desktopStructureError(e, t) }
  finally { busy.value = false }
}
watch(() => [orgId.value, id.value, props.selfManaged], load, { immediate: true })
onBeforeUnmount(() => generation++)
</script>
