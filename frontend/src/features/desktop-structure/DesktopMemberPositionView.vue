<template>
  <StructurePage :title="t('desktopStructure.assign')" :subtitle="member?.name" :description="t('desktopStructure.assignmentHint')" :loading="loading" :error="error" :busy="busy" :dirty="guard.dirty.value" @back="guard.leave(returnTo)" @retry="reload">
    <template v-if="member">
      <p v-if="organization?.status !== 'active'" class="structure-notice" role="status">{{ t('desktopStructure.readonly') }}</p>
      <div v-if="structureError" class="structure-error" role="alert"><p class="min-w-0 flex-1">{{ structureError }}</p><button type="button" class="btn btn-secondary" :disabled="structureLoading" @click="loadStructure">{{ t('desktopStructure.retryStructureFilters') }}</button></div>
      <form class="structure-panel structure-form" :aria-busy="busy" @submit.prevent="save">
        <div class="structure-assignment space-y-2"><p class="structure-caption">{{ t('desktopStructure.current') }}</p><p class="font-semibold"><bdi>{{ member.department?.name || '—' }}</bdi> / <bdi>{{ member.position?.name || t('desktopStructure.unassigned') }}</bdi></p><p v-if="!member.position" class="structure-caption">{{ t('desktopStructure.unassignedHelp') }}</p></div>
        <p class="structure-caption">{{ t('desktopStructure.requiredHint') }}</p>
        <fieldset class="structure-section" :disabled="busy || organization?.status !== 'active'">
          <legend class="structure-section-title mb-4">{{ t('desktopStructure.assignmentInfo') }}</legend>
          <div class="structure-grid">
            <label class="structure-field"><span>{{ t('desktopStructure.department') }} <span aria-hidden="true">*</span></span><select v-model="form.department_id" name="department" class="input" required :disabled="structureLoading || !!structureError" aria-describedby="assignment-department-hint" @change="form.position_id = ''"><option value="">{{ t('desktopStructure.selectDepartment') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select><p id="assignment-department-hint" class="structure-caption">{{ departments.find(d => d.public_id === form.department_id)?.name || (structureLoading || structureError ? '' : t(departments.length ? 'desktopStructure.selectDepartmentFirst' : 'desktopStructure.noDepartments')) }}</p></label>
            <label class="structure-field"><span>{{ t('desktopStructure.position') }} <span aria-hidden="true">*</span></span><select v-model="form.position_id" name="position" class="input" required :disabled="!form.department_id || structureLoading || !!structureError" aria-describedby="assignment-position-hint"><option value="">{{ t('desktopStructure.selectPosition') }}</option><option v-for="p in availablePositions" :key="p.public_id" :value="p.public_id">{{ p.name }}</option></select><p id="assignment-position-hint" class="structure-caption">{{ positions.find(p => p.public_id === form.position_id)?.name || (structureLoading || structureError ? '' : t(!form.department_id ? 'desktopStructure.selectDepartmentFirst' : !availablePositions.length ? 'desktopStructure.noPositions' : 'desktopStructure.selectPosition')) }}</p></label>
          </div>
          <label class="structure-field"><span>{{ t('desktopStructure.reason') }} <span class="font-normal">· {{ t('desktopStructure.optional') }}</span></span><textarea v-model="form.reason" name="assignment-reason" class="input" maxlength="500" rows="3" aria-describedby="assignment-reason-hint" /><p id="assignment-reason-hint" class="structure-caption">{{ t('desktopStructure.reasonHint') }}</p></label>
        </fieldset>
        <div class="structure-notice"><Icon name="infoCircle" size="md" class="shrink-0" aria-hidden="true" /><p class="min-w-0 flex-1">{{ t('desktopStructure.historyHint') }}</p></div>
        <div class="structure-actions"><button class="btn btn-secondary" type="button" :disabled="busy" @click="guard.leave(returnTo)">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" :disabled="organization?.status !== 'active' || structureLoading || !!structureError" :aria-disabled="busy || undefined"><Icon v-if="busy" name="clock" size="sm" aria-hidden="true" />{{ t('common.save') }}</button></div>
      </form>
      <section class="structure-panel max-w-3xl space-y-5" aria-labelledby="position-history-heading" :aria-busy="historyLoading">
        <header class="space-y-1"><h2 id="position-history-heading" class="structure-section-title">{{ t('desktopStructure.history') }}</h2><p class="structure-caption">{{ t('desktopStructure.historyDescription') }}</p></header>
        <p class="structure-caption" role="status">{{ historyLoading ? t('common.loading') : '' }}</p>
        <div v-if="historyError" class="structure-error" role="alert"><p class="min-w-0 flex-1">{{ t('desktopStructure.historyLoadFailed') }}</p><button type="button" class="btn btn-secondary" @click="loadHistory">{{ t('desktopStructure.retryHistory') }}</button></div>
        <ol v-else-if="!historyLoading && history.length" class="structure-history">
          <li v-for="entry in history" :key="entry.position_revision" class="space-y-2">
            <div class="flex flex-wrap items-start gap-2"><h3 class="min-w-0 flex-1 text-sm font-semibold"><bdi>{{ entry.department.name }}</bdi> / <bdi>{{ entry.position.name }}</bdi></h3><span v-if="!entry.ended_at" class="structure-status structure-status-active">{{ t('desktopStructure.current') }}</span></div>
            <p class="structure-caption tabular-nums"><time :datetime="entry.started_at">{{ historyDate(entry.started_at) }}</time> — <time v-if="entry.ended_at" :datetime="entry.ended_at">{{ historyDate(entry.ended_at) }}</time><span v-else>{{ t('desktopStructure.untilNow') }}</span></p>
            <p v-if="entry.reason" class="whitespace-pre-wrap text-sm leading-relaxed">{{ entry.reason }}</p>
          </li>
        </ol>
        <p v-else-if="!historyLoading" class="structure-caption">{{ t('desktopStructure.historyUnknown') }}</p>
        <Pagination v-if="total > 20" v-model:page="page" :page-size="20" :total="total" :show-page-size-selector="false" @update:page="loadHistory" />
      </section>
    </template>
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { formatDateTime } from '@/utils/format'
import { useDesktopForm } from './useDesktopForm'
import { useDesktopStructureOptions } from './useDesktopStructureOptions'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, structureIdempotencyKey, type PositionHistory } from '@/api/desktopStructure'
import type { DesktopMember, DesktopOrganization } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), { t, locale } = useI18n()
const orgId = computed(() => String(route.params.organizationId || '')), id = computed(() => String(route.params.memberId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged)), returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`))
const member = ref<DesktopMember>(), organization = ref<DesktopOrganization | null>(null), history = ref<PositionHistory[]>([])
const structure = useDesktopStructureOptions()
const { departments, positions, loading: structureLoading, error: structureError } = structure
const form = reactive({ department_id: '', position_id: '', reason: '' }), availablePositions = computed(() => positions.value.filter(p => p.department?.public_id === form.department_id))
const loading = ref(true), busy = ref(false), error = ref(''), page = ref(1), total = ref(0), guard = useDesktopForm(() => form, () => busy.value, () => organization.value?.public_id || orgId.value)
const historyLoading = ref(false), historyError = ref('')
const historyDate = (value: string) => formatDateTime(value, undefined, locale.value)
let historyRequest = 0
let generation = 0, key = structureIdempotencyKey(), keyPayload = ''
function loadStructure() { return structure.load(desktopStructure(orgId.value, props.selfManaged), { status: 'active' }) }
async function load() {
  structure.reset()
  historyRequest++; history.value = []; total.value = 0; page.value = 1; historyLoading.value = false; historyError.value = ''
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged); loading.value = true; error.value = ''
  try {
    const [m, org] = await Promise.all([api.member(id.value), api.organization(), loadStructure()])
    if (g !== generation) return
    member.value = m; organization.value = org
    Object.assign(form, { department_id: m.department?.public_id || '', position_id: m.position?.public_id || '', reason: '' }); guard.clean()
    void loadHistory()
  } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
async function reload() { if (!guard.dirty.value || window.confirm(t('desktopStructure.reload'))) await load() }
async function loadHistory() {
  const g = generation, request = ++historyRequest
  historyLoading.value = true; historyError.value = ''
  try {
    const data = await desktopStructure(orgId.value, props.selfManaged).history(id.value, page.value)
    if (g === generation && request === historyRequest) { history.value = data.items; total.value = data.total }
  } catch (e) { if (g === generation && request === historyRequest) historyError.value = desktopStructureError(e, t) }
  finally { if (g === generation && request === historyRequest) historyLoading.value = false }
}
async function save() {
  if (!member.value || busy.value || structureLoading.value || structureError.value) return
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
