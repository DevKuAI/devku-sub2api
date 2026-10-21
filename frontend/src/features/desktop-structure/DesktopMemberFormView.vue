<template>
  <StructurePage :title="t(id ? 'admin.desktop.editMember' : 'admin.desktop.createMember')" :subtitle="organization?.name" :loading="loading" :error="error" :busy="busy" @back="guard.leave(returnTo)" @retry="reload">
    <form v-if="organization" class="card max-w-2xl space-y-5 p-6" :aria-busy="busy" @submit.prevent="save">
      <label class="block space-y-1"><span class="block">{{ t('admin.desktop.memberName') }}</span><input v-model="form.name" class="input w-full" required maxlength="100" :disabled="busy" /></label>
      <label class="block space-y-1"><span class="block">{{ t('admin.desktop.phone') }}</span><input v-model="form.phone" class="input w-full" type="tel" required :disabled="busy" /></label>
      <template v-if="!id">
        <label class="block space-y-1"><span class="block">{{ t('desktopStructure.department') }}</span><select v-model="form.department_id" class="input w-full" required :disabled="busy" @change="form.position_id = ''"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select></label>
        <label class="block space-y-1"><span class="block">{{ t('desktopStructure.position') }}</span><select v-model="form.position_id" class="input w-full" required :disabled="busy || !form.department_id"><option value="">{{ t('desktopStructure.selectPosition') }}</option><option v-for="p in availablePositions" :key="p.public_id" :value="p.public_id">{{ p.name }}</option></select></label>
        <button type="button" class="text-primary-600 hover:underline" :disabled="busy" @click="createPosition">{{ t('desktopStructure.createPosition') }}</button>
      </template>
      <p v-else>{{ t('desktopStructure.current') }}: {{ member?.department?.name || '—' }} / {{ member?.position?.name || t('desktopStructure.unassigned') }} <router-link class="ml-3 text-primary-600 hover:underline" :to="{ path: `${base}/members/${id}/position`, query: { return_to: route.fullPath } }">{{ t('desktopStructure.assign') }}</router-link></p>
      <fieldset class="space-y-3"><legend>{{ t('admin.desktop.budget.title') }}</legend><p class="text-sm text-gray-500">{{ t('admin.desktop.budget.hint') }}</p><label v-for="field in budgetFields" :key="field" class="block space-y-1"><span class="block">{{ t(`admin.desktop.budget.${field}`) }}</span><input v-model.number="form.budget[field]" type="number" min="0" step="0.01" class="input w-full" :disabled="busy" /></label></fieldset>
      <label class="block space-y-1"><span class="block">{{ t('admin.desktop.memberRemark') }}</span><textarea v-model="form.remark" class="input w-full" rows="3" maxlength="500" :disabled="busy" /></label>
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
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, structureIdempotencyKey, type OrganizationUnit } from '@/api/desktopStructure'
import type { DesktopOrganization, DesktopMember } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), { t } = useI18n()
const id = computed(() => String(route.params.memberId || '')), orgId = computed(() => String(route.params.organizationId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged))
const returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`))
const blank = () => ({ name: '', phone: '', remark: '', department_id: '', position_id: '', budget: { quota: 0, limit_1d: 0, limit_7d: 0 } })
const form = reactive(blank()), organization = ref<DesktopOrganization | null>(null), member = ref<DesktopMember>(), departments = ref<OrganizationUnit[]>([]), positions = ref<OrganizationUnit[]>([])
const availablePositions = computed(() => positions.value.filter(p => p.department?.public_id === form.department_id))
const budgetFields = ['quota', 'limit_1d', 'limit_7d'] as const
const loading = ref(true), busy = ref(false), error = ref('')
const guard = useDesktopForm(() => form, () => busy.value, () => organization.value?.public_id || orgId.value)
let generation = 0, key = structureIdempotencyKey(), keyPayload = ''
async function load() {
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged); loading.value = true; error.value = ''
  try {
    const [org, deps, ps, m] = await Promise.all([api.organization(), api.all('departments', { status: 'active' }), api.all('positions', { status: 'active' }), id.value ? api.member(id.value) : undefined])
    if (g !== generation) return
    organization.value = org; departments.value = deps; positions.value = ps; member.value = m
    Object.assign(form, blank(), m ? { name: m.name, phone: m.phone, remark: m.remark || '', budget: { quota: m.budget?.quota || 0, limit_1d: m.budget?.limit_1d || 0, limit_7d: m.budget?.limit_7d || 0 } } : {})
    const draft = guard.restore(); guard.clean(); if (draft && !id.value) Object.assign(form, draft)
  } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
async function reload() { if (window.confirm(t('desktopStructure.reload'))) await load() }
async function createPosition() { await guard.child(`${base.value}/positions/new?return_to=${encodeURIComponent(route.fullPath)}&department_id=${encodeURIComponent(form.department_id)}`) }
async function save() {
  if (busy.value) return
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
