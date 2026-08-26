<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="min-w-0 flex-1 sm:max-w-72">
            <input v-model="search" class="input" type="search" :placeholder="t('admin.desktop.searchOrganizations')" @input="scheduleLoad" />
          </div>
          <Select v-model="status" class="w-40" :options="statusOptions" @change="resetAndLoad" />
          <div class="ml-auto flex items-center gap-2">
            <button class="btn btn-secondary" type="button" :title="t('common.refresh')" :aria-label="t('common.refresh')" :disabled="loading" @click="loadOrganizations">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" type="button" @click="openCreate">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('admin.desktop.createOrganization') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="organizations" :loading="loading" row-key="public_id" @row-click="openOrganization">
          <template #cell-name="{ row }">
            <div class="min-w-0 max-w-72">
              <div class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</div>
              <div class="mt-1 font-mono text-xs text-gray-500 dark:text-dark-400">{{ row.code }}</div>
            </div>
          </template>
          <template #cell-status="{ row }"><StatusBadge :status="row.status" :label="statusLabel(row.status)" /></template>
          <template #cell-gateway_user="{ row }">
            <div class="min-w-0 max-w-64">
              <div class="truncate text-sm font-medium">{{ row.gateway_user.username || row.gateway_user.email }}</div>
              <div class="truncate text-xs text-gray-500 dark:text-dark-400">{{ row.gateway_user.email }}</div>
            </div>
          </template>
          <template #cell-group="{ row }"><span class="break-words">{{ row.group.name }}</span></template>
          <template #cell-target_config_assigned="{ row }">
            <span :class="['badge', row.target_config_assigned ? 'badge-success' : 'badge-warning']">
              {{ row.target_config_assigned ? t('admin.desktop.configured') : t('admin.desktop.notConfigured') }}
            </span>
          </template>
          <template #cell-updated_at="{ value }"><span class="whitespace-nowrap text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span></template>
          <template #cell-actions="{ row }">
            <button class="rounded-md p-2 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" type="button" :title="t('common.view')" :aria-label="t('common.view')" @click.stop="openOrganization(row)">
              <Icon name="arrowRight" size="sm" />
            </button>
          </template>
          <template #empty>
            <EmptyState :title="t('admin.desktop.noOrganizations')" :description="t('admin.desktop.noOrganizationsDescription')" :action-text="t('admin.desktop.createOrganization')" @action="openCreate" />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination v-if="pagination.total > 0" :page="pagination.page" :page-size="pagination.page_size" :total="pagination.total" @update:page="changePage" @update:page-size="changePageSize" />
      </template>
    </TablePageLayout>

    <BaseDialog :show="showCreate" :title="t('admin.desktop.createOrganization')" width="wide" @close="closeCreate">
      <form id="desktop-organization-create" class="space-y-4" @submit.prevent="createOrganization">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <Input v-model="form.name" :label="t('admin.desktop.organizationName')" required />
          <Input v-model="form.code" :label="t('admin.desktop.organizationCode')" required :hint="t('admin.desktop.organizationCodeHint')" />
        </div>
        <div>
          <label class="input-label mb-1.5 block">{{ t('admin.desktop.gatewayUser') }} <span class="text-red-500">*</span></label>
          <Select v-model="form.gateway_user_id" :options="userOptions" searchable remote :loading="usersLoading" :placeholder="t('admin.desktop.selectGatewayUser')" :search-placeholder="t('admin.desktop.searchGatewayUsers')" @search="loadUsers" />
        </div>
        <div>
          <label class="input-label mb-1.5 block">{{ t('admin.desktop.group') }} <span class="text-red-500">*</span></label>
          <Select v-model="form.group_id" :options="groupOptions" searchable :loading="groupsLoading" :placeholder="t('admin.desktop.selectGroup')" />
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="closeCreate">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" type="submit" form="desktop-organization-create" :disabled="creating">{{ creating ? t('common.saving') : t('common.create') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { DesktopOrganization, DesktopStatus } from '@/api/admin/desktop'
import type { AdminGroup, AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const organizations = ref<DesktopOrganization[]>([])
const loading = ref(false)
const search = ref('')
const status = ref<DesktopStatus | ''>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const showCreate = ref(false)
const creating = ref(false)
const usersLoading = ref(false)
const groupsLoading = ref(false)
const users = ref<AdminUser[]>([])
const groups = ref<AdminGroup[]>([])
const form = reactive({ name: '', code: '', gateway_user_id: null as number | null, group_id: null as number | null })
let searchTimer: ReturnType<typeof setTimeout> | undefined
let listController: AbortController | undefined
let userController: AbortController | undefined

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.desktop.organization') },
  { key: 'status', label: t('common.status') },
  { key: 'gateway_user', label: t('admin.desktop.gatewayUser') },
  { key: 'group', label: t('admin.desktop.group') },
  { key: 'member_count', label: t('admin.desktop.memberCount') },
  { key: 'target_config_assigned', label: t('admin.desktop.configuration') },
  { key: 'updated_at', label: t('admin.desktop.updatedAt') },
  { key: 'actions', label: t('common.actions') },
])
const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'active', label: t('common.active') },
  { value: 'disabled', label: t('common.disabled') },
])
const userOptions = computed(() => users.value.map((user) => ({ value: user.id, label: `${user.username || user.email} · ${user.email}` })))
const groupOptions = computed(() => groups.value.map((group) => ({ value: group.id, label: group.name })))

function errorMessage(error: unknown): string {
  const reason = (error as { reason?: string })?.reason
  return reason ? t(`admin.desktop.errors.${reason}`) : (error as { message?: string })?.message || t('admin.desktop.errors.UNKNOWN')
}
function statusLabel(value: DesktopStatus): string { return value === 'active' ? t('common.active') : t('common.disabled') }
async function loadOrganizations() {
  listController?.abort()
  listController = new AbortController()
  loading.value = true
  try {
    const result = await adminAPI.desktop.listOrganizations(pagination.page, pagination.page_size, { search: search.value.trim(), status: status.value }, listController.signal)
    organizations.value = result.items
    pagination.total = result.total
  } catch (error) {
    if ((error as { code?: string })?.code !== 'ERR_CANCELED') appStore.showError(errorMessage(error))
  } finally { loading.value = false }
}
function scheduleLoad() { clearTimeout(searchTimer); searchTimer = setTimeout(resetAndLoad, 300) }
function resetAndLoad() { pagination.page = 1; void loadOrganizations() }
function changePage(page: number) { pagination.page = page; void loadOrganizations() }
function changePageSize(pageSize: number) { pagination.page_size = pageSize; pagination.page = 1; void loadOrganizations() }
function openOrganization(row: DesktopOrganization) { void router.push(`/admin/desktop/organizations/${encodeURIComponent(row.public_id)}`) }
async function loadUsers(query = '') {
  userController?.abort(); userController = new AbortController(); usersLoading.value = true
  try { users.value = (await adminAPI.desktop.listAvailableGatewayUsers(query, userController.signal)).items }
  catch (error) { if ((error as { code?: string })?.code !== 'ERR_CANCELED') appStore.showError(errorMessage(error)) }
  finally { usersLoading.value = false }
}
async function openCreate() {
  Object.assign(form, { name: '', code: '', gateway_user_id: null, group_id: null })
  showCreate.value = true; groupsLoading.value = true
  void loadUsers()
  try { groups.value = await adminAPI.desktop.listActiveGroups() }
  catch (error) { appStore.showError(errorMessage(error)) }
  finally { groupsLoading.value = false }
}
function closeCreate() { if (!creating.value) showCreate.value = false }
async function createOrganization() {
  if (!form.name.trim() || !/^[a-z0-9]{2,16}$/.test(form.code.trim().toLowerCase()) || !form.gateway_user_id || !form.group_id) {
    appStore.showError(t('admin.desktop.errors.VALIDATION_FAILED')); return
  }
  creating.value = true
  try {
    const created = await adminAPI.desktop.createOrganization({ name: form.name.trim(), code: form.code.trim().toLowerCase(), gateway_user_id: form.gateway_user_id, group_id: form.group_id })
    appStore.showSuccess(t('admin.desktop.organizationCreated')); showCreate.value = false
    await router.push(`/admin/desktop/organizations/${encodeURIComponent(created.public_id)}`)
  } catch (error) { appStore.showError(errorMessage(error)) }
  finally { creating.value = false }
}
onMounted(loadOrganizations)
onBeforeUnmount(() => { clearTimeout(searchTimer); listController?.abort(); userController?.abort() })
</script>
