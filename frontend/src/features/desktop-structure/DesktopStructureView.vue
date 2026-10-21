<template>
  <StructurePage :title="t('desktopStructure.title')" :subtitle="organization?.name" :loading="loading" :error="error" @back="router.push(returnTo)" @retry="load">
    <p v-if="status" class="rounded-lg border p-4" role="status">{{ t('desktopStructure.progress', { count: status.unassigned_members }) }} <span v-if="status.inconsistent_members">{{ t('desktopStructure.inconsistent', { count: status.inconsistent_members }) }}</span></p>
    <div class="flex flex-wrap items-end gap-3">
      <label class="space-y-1"><span class="block">{{ t('desktopStructure.kind') }}</span><select v-model="kind" class="input" @change="reset"><option value="departments">{{ t('desktopStructure.departments') }}</option><option value="positions">{{ t('desktopStructure.positions') }}</option></select></label>
      <label class="space-y-1"><span class="block">{{ t('common.search') }}</span><input v-model="search" class="input" type="search" @input="schedule" /></label>
      <label class="space-y-1"><span class="block">{{ t('common.status') }}</span><select v-model="filterStatus" class="input" @change="reset"><option value="">{{ t('common.all') }}</option><option value="active">{{ t('common.active') }}</option><option value="disabled">{{ t('common.disabled') }}</option></select></label>
      <label v-if="kind === 'positions'" class="space-y-1"><span class="block">{{ t('desktopStructure.department') }}</span><select v-model="department" class="input" @change="reset"><option value="">{{ t('common.all') }}</option><option v-for="d in departments" :key="d.public_id" :value="d.public_id">{{ d.name }}</option></select></label>
      <router-link v-if="organization?.status === 'active'" class="btn btn-primary" :to="{ path: `${base}/${kind}/new`, query: { return_to: route.fullPath } }">{{ t('desktopStructure.create') }}</router-link>
    </div>
    <div class="overflow-x-auto rounded-lg border">
      <table class="w-full text-left text-sm"><thead><tr><th class="p-3">{{ t('common.name') }}</th><th v-if="kind === 'positions'" class="p-3">{{ t('desktopStructure.department') }}</th><th class="p-3">{{ t('common.status') }}</th><th class="p-3">{{ t('desktopStructure.members') }}</th><th class="p-3">{{ t('common.actions') }}</th></tr></thead>
        <tbody><tr v-for="unit in units" :key="unit.public_id" class="border-t"><td class="max-w-72 break-words p-3">{{ unit.name }}</td><td v-if="kind === 'positions'" class="p-3">{{ unit.department?.name }}</td><td class="p-3">{{ t(unit.status === 'active' ? 'common.active' : 'common.disabled') }}</td><td class="p-3">{{ unit.member_count }}</td><td class="p-3"><router-link :to="{ path: `${base}/${kind}/${unit.public_id}/edit`, query: { return_to: route.fullPath } }" class="text-primary-600 hover:underline">{{ t('common.edit') }} {{ unit.name }}</router-link></td></tr></tbody>
      </table><p v-if="!units.length" class="p-6 text-center">{{ t('desktopStructure.empty') }}</p>
    </div>
    <Pagination v-model:page="page" :page-size="20" :total="total" :show-page-size-selector="false" @update:page="load" />
    <p class="text-sm text-gray-500">{{ t('desktopStructure.scopeHint') }}</p>
  </StructurePage>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import StructurePage from './StructurePage.vue'
import Pagination from '@/components/common/Pagination.vue'
import { desktopStructure, desktopStructureError, desktopOrganizationPath, safeDesktopReturn, type OrganizationUnit, type StructureStatus, type UnitKind } from '@/api/desktopStructure'
import type { DesktopOrganization } from '@/api/admin/desktop'
const props = defineProps<{ selfManaged?: boolean }>()
const route = useRoute(), router = useRouter(), { t } = useI18n()
const orgId = computed(() => String(route.params.organizationId || ''))
const base = computed(() => desktopOrganizationPath(orgId.value, !!props.selfManaged))
const returnTo = computed(() => safeDesktopReturn(route.query.return_to, base.value, `${base.value}?tab=members`))
const organization = ref<DesktopOrganization | null>(null), status = ref<StructureStatus>(), departments = ref<OrganizationUnit[]>([]), units = ref<OrganizationUnit[]>([])
const kind = ref<UnitKind>('departments'), search = ref(''), department = ref(''), filterStatus = ref(''), page = ref(1), total = ref(0), loading = ref(true), error = ref('')
let generation = 0, timer: ReturnType<typeof setTimeout> | undefined
async function load() {
  const g = ++generation, api = desktopStructure(orgId.value, props.selfManaged)
  loading.value = true; error.value = ''
  try {
    const [org, state, deps, data] = await Promise.all([api.organization(), api.status(), api.all('departments'), api.list(kind.value, { page: page.value, page_size: 20, search: search.value, status: filterStatus.value, department_id: department.value })])
    if (g !== generation) return
    organization.value = org; status.value = state; departments.value = deps; units.value = data.items; total.value = data.total
  } catch (e) { if (g === generation) error.value = desktopStructureError(e, t) }
  finally { if (g === generation) loading.value = false }
}
function reset() { page.value = 1; void load() }
function schedule() { clearTimeout(timer); timer = setTimeout(reset, 250) }
watch(() => [orgId.value, props.selfManaged], reset, { immediate: true })
onBeforeUnmount(() => { generation++; clearTimeout(timer) })
</script>
