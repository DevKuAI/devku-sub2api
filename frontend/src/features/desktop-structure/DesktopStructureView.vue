<template>
  <StructurePage :title="t('desktopStructure.title')" :subtitle="organization?.name" :description="t('desktopStructure.description')" :loading="loading" :error="error" :focus-error="false" @back="router.push(returnTo)" @retry="reload">
    <template #actions>
      <router-link v-if="organization?.status === 'active'" class="btn btn-primary" :to="{ path: `${base}/${kind}/new`, query: { return_to: listReturnTo } }">
        <Icon name="plus" size="sm" aria-hidden="true" />{{ t(kind === 'departments' ? 'desktopStructure.addDepartment' : 'desktopStructure.addPosition') }}
      </router-link>
    </template>
    <template v-if="organization">
      <div v-if="organization.status !== 'active'" class="structure-notice" role="status"><Icon name="infoCircle" size="md" aria-hidden="true" /><p class="min-w-0 flex-1">{{ t('desktopStructure.readonly') }}</p></div>
      <div v-if="status" class="structure-notice" :class="{ 'structure-notice-warning': status.unassigned_members || status.inconsistent_members }">
        <Icon :name="status.unassigned_members || status.inconsistent_members ? 'exclamationCircle' : 'checkCircle'" size="md" class="mt-0.5 shrink-0" aria-hidden="true" />
        <div class="min-w-0 flex-1 space-y-1">
          <p class="font-medium">{{ status.inconsistent_members ? t('desktopStructure.inconsistent', { count: status.inconsistent_members }) : status.unassigned_members ? t('desktopStructure.progress', { count: status.unassigned_members }) : t('desktopStructure.ready') }}</p>
          <p v-if="status.inconsistent_members && status.unassigned_members">{{ t('desktopStructure.progress', { count: status.unassigned_members }) }}</p>
          <p>{{ t(status.inconsistent_members ? 'desktopStructure.inconsistentHelp' : status.unassigned_members ? 'desktopStructure.assignmentHint' : 'desktopStructure.readyHint') }}</p>
        </div>
        <router-link v-if="status.unassigned_members" class="structure-link" :to="`${base}?tab=members&unassigned=true`">{{ t('desktopStructure.reviewAssignments') }}<Icon name="arrowRight" size="sm" aria-hidden="true" /></router-link>
      </div>
      <section class="structure-panel space-y-5" :aria-label="t('desktopStructure.title')">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="structure-tabs" role="group" :aria-label="t('desktopStructure.kind')">
            <button v-for="value in unitKinds" :key="value" type="button" class="structure-tab" :aria-pressed="kind === value" @click="changeKind(value)"><Icon v-if="kind === value" name="check" size="sm" aria-hidden="true" />{{ t(`desktopStructure.${value}`) }}</button>
          </div>
          <button v-if="hasFilters" type="button" class="structure-link" @click="clearFilters">{{ t('desktopStructure.clearFilters') }}</button>
        </div>
        <div v-if="kind === 'positions' && structureError" class="structure-error" role="alert"><p class="min-w-0 flex-1">{{ structureError }}</p><button type="button" class="btn btn-secondary" :disabled="structureLoading" @click="loadStructure">{{ t('desktopStructure.retryStructureFilters') }}</button></div>
        <div class="structure-filters">
          <label class="structure-field"><span>{{ t('common.search') }}</span><input v-model="search" name="structure-search" class="input" type="search" :placeholder="t(kind === 'departments' ? 'desktopStructure.searchDepartments' : 'desktopStructure.searchPositions')" @input="schedule" /></label>
          <label v-if="kind === 'positions'" class="structure-field"><span>{{ t('desktopStructure.department') }}</span><select v-model="department" name="department-filter" class="input" :disabled="structureLoading || !!structureError" @change="reset"><option value="">{{ t('common.all') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select></label>
          <label class="structure-field"><span>{{ t('common.status') }}</span><select v-model="filterStatus" name="status-filter" class="input" @change="reset"><option value="">{{ t('common.all') }}</option><option value="active">{{ t('common.active') }}</option><option value="disabled">{{ t('common.disabled') }}</option></select></label>
        </div>
        <p class="structure-caption tabular-nums" role="status">{{ refreshing ? t('common.loading') : error ? '' : t('desktopStructure.resultCount', { count: total }) }}</p>
        <template v-if="!refreshing && !error">
          <div v-if="!units.length" class="structure-empty">
            <span class="structure-empty-icon"><Icon :name="hasFilters ? 'search' : 'users'" size="lg" aria-hidden="true" /></span>
            <h2 class="structure-section-title">{{ t(hasFilters ? 'desktopStructure.noResults' : kind === 'departments' ? 'desktopStructure.emptyDepartments' : 'desktopStructure.emptyPositions') }}</h2>
            <p class="structure-caption max-w-prose">{{ t(hasFilters ? 'desktopStructure.noResultsHint' : kind === 'departments' ? 'desktopStructure.emptyDepartmentsHint' : 'desktopStructure.emptyPositionsHint') }}</p>
            <button v-if="hasFilters" type="button" class="btn btn-secondary" @click="clearFilters">{{ t('desktopStructure.clearFilters') }}</button>
          </div>
          <template v-else>
            <div class="structure-desktop-list">
              <table class="structure-table">
                <caption class="sr-only">{{ t(`desktopStructure.${kind}`) }}</caption>
                <thead><tr><th scope="col">{{ t('common.name') }}</th><th v-if="kind === 'positions'" scope="col">{{ t('desktopStructure.department') }}</th><th scope="col">{{ t('common.status') }}</th><th scope="col">{{ t('desktopStructure.members') }}</th><th v-if="organization.status === 'active'" scope="col">{{ t('common.actions') }}</th></tr></thead>
                <tbody><tr v-for="unit in units" :key="unit.public_id">
                  <td class="font-medium"><bdi>{{ unit.name }}</bdi></td>
                  <td v-if="kind === 'positions'"><bdi>{{ unit.department?.name || '—' }}</bdi></td>
                  <td><span class="structure-status" :class="{ 'structure-status-active': unit.status === 'active' }">{{ t(unit.status === 'active' ? 'common.active' : 'common.disabled') }}</span></td>
                  <td class="tabular-nums">{{ unit.member_count }}</td>
                  <td v-if="organization.status === 'active'"><router-link :to="{ path: `${base}/${kind}/${unit.public_id}/edit`, query: { return_to: listReturnTo } }" class="structure-link" :aria-label="t('desktopStructure.editNamed', { name: unit.name })">{{ t('common.edit') }}</router-link></td>
                </tr></tbody>
              </table>
            </div>
            <ul class="structure-mobile-list">
              <li v-for="unit in units" :key="unit.public_id">
                <div class="flex flex-wrap items-start justify-between gap-3"><h2 class="min-w-0 flex-1 basis-24 text-sm font-semibold"><bdi>{{ unit.name }}</bdi></h2><span class="structure-status" :class="{ 'structure-status-active': unit.status === 'active' }">{{ t(unit.status === 'active' ? 'common.active' : 'common.disabled') }}</span></div>
                <p v-if="kind === 'positions'" class="structure-caption">{{ t('desktopStructure.department') }} · <bdi>{{ unit.department?.name || '—' }}</bdi></p>
                <div class="flex flex-wrap items-center justify-between gap-3"><p class="structure-caption tabular-nums">{{ t('desktopStructure.members') }} · {{ unit.member_count }}</p><router-link v-if="organization.status === 'active'" :to="{ path: `${base}/${kind}/${unit.public_id}/edit`, query: { return_to: listReturnTo } }" class="structure-link" :aria-label="t('desktopStructure.editNamed', { name: unit.name })">{{ t('common.edit') }}</router-link></div>
              </li>
            </ul>
            <Pagination v-if="total > 20" v-model:page="page" :page-size="20" :total="total" :show-page-size-selector="false" @update:page="load" />
          </template>
        </template>
      </section>
      <p class="structure-caption flex items-start gap-2"><Icon name="infoCircle" size="sm" class="mt-1 shrink-0" aria-hidden="true" /><span>{{ t('desktopStructure.scopeHint') }}</span></p>
    </template>
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, type OrganizationUnit, type StructureStatus, type UnitKind } from '@/api/desktopStructure'
import { useDesktopStructureOptions } from './useDesktopStructureOptions'
import type { DesktopOrganization } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), router = useRouter(), { t } = useI18n()
const orgId = computed(() => String(route.params.organizationId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged))
const returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`))
const organization = ref<DesktopOrganization | null>(null), status = ref<StructureStatus>(), units = ref<OrganizationUnit[]>([])
const structure = useDesktopStructureOptions()
const { departments, loading: structureLoading, error: structureError } = structure
const unitKinds: UnitKind[] = ['departments', 'positions']
const kind = ref<UnitKind>(route.query.kind === 'positions' ? 'positions' : 'departments'), search = ref(String(route.query.search || '')), department = ref(String(route.query.department_id || '')), filterStatus = ref(String(route.query.status || '')), page = ref(initialPage()), total = ref(0), loading = ref(true), refreshing = ref(false), contextError = ref(''), listError = ref('')
const error = computed(() => contextError.value || listError.value)
const hasFilters = computed(() => Boolean(search.value || department.value || filterStatus.value))
const listReturnTo = computed(() => router.resolve({ path: route.path, query: { ...route.query, kind: kind.value, search: search.value || undefined, department_id: department.value || undefined, status: filterStatus.value || undefined, page: String(page.value) } }).fullPath)
function initialPage() {
  const value = Number(route.query.page)
  return Number.isSafeInteger(value) && value > 0 ? value : 1
}
let generation = 0, contextGeneration = 0, departmentsLoaded = false
let timer: ReturnType<typeof setTimeout> | undefined, listController: AbortController | undefined
async function loadStructure() {
  await structure.load(desktopStructure(orgId.value, props.selfManaged), { positions: false })
  departmentsLoaded = !structureError.value
}
async function loadContext() {
  const g = ++contextGeneration, api = desktopStructure(orgId.value, props.selfManaged)
  loading.value = !organization.value; contextError.value = ''
  try {
    const [org, state] = await Promise.all([api.organization(), api.status()])
    if (g === contextGeneration) { organization.value = org; status.value = state }
  } catch (cause) { if (g === contextGeneration) contextError.value = desktopStructureError(cause, t) }
  finally { if (g === contextGeneration) loading.value = false }
}
async function load() {
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged)
  listController?.abort(); listController = new AbortController()
  const { signal } = listController
  refreshing.value = true; listError.value = ''
  const params = { page: page.value, page_size: 20, search: search.value, status: filterStatus.value, department_id: department.value }
  try {
    let data = await api.list(kind.value, params, signal)
    if (g !== generation) return
    const lastPage = Math.max(1, Math.ceil(data.total / 20))
    if (page.value > lastPage) {
      page.value = lastPage
      data = await api.list(kind.value, { ...params, page: lastPage }, signal)
    }
    if (g === generation) { units.value = data.items; total.value = data.total }
  } catch (cause) { if (g === generation && !signal.aborted) listError.value = desktopStructureError(cause, t) }
  finally { if (g === generation) refreshing.value = false }
}
function reset() { clearTimeout(timer); page.value = 1; void load() }
function changeKind(value: UnitKind) {
  if (kind.value === value) return
  kind.value = value; department.value = ''
  if (value === 'positions' && !departmentsLoaded) void loadStructure()
  reset()
}
function clearFilters() { search.value = ''; department.value = ''; filterStatus.value = ''; reset() }
function schedule() { clearTimeout(timer); timer = setTimeout(reset, 250) }
function reload() {
  void loadContext(); void load()
  if (kind.value === 'positions') void loadStructure()
}
watch(() => [orgId.value, props.selfManaged], () => {
  clearTimeout(timer); structure.reset(); departmentsLoaded = false
  organization.value = null; status.value = undefined; units.value = []; total.value = 0
  page.value = initialPage()
  reload()
}, { immediate: true })
onBeforeUnmount(() => { generation++; contextGeneration++; listController?.abort(); clearTimeout(timer) })
</script>
