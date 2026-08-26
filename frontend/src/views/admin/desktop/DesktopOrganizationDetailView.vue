<template>
  <AppLayout>
    <div class="mx-auto min-w-0 max-w-7xl space-y-6">
      <div class="flex flex-wrap items-start gap-3">
        <button class="btn btn-secondary px-3" type="button" :title="t('common.back')" :aria-label="t('common.back')" @click="router.push('/admin/desktop/organizations')">
          <Icon name="arrowLeft" size="md" />
        </button>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-3">
            <h1 class="break-words text-xl font-semibold text-gray-900 dark:text-white">{{ organization?.name || t('common.loading') }}</h1>
            <StatusBadge v-if="organization" :status="organization.status" :label="statusLabel(organization.status)" />
          </div>
          <p v-if="organization" class="mt-1 font-mono text-sm text-gray-500 dark:text-dark-400">{{ organization.code }}</p>
        </div>
        <button v-if="organization" class="btn btn-secondary" type="button" @click="openEditOrganization">
          <Icon name="edit" size="sm" class="mr-1" />{{ t('common.edit') }}
        </button>
      </div>

      <div v-if="organization" class="grid grid-cols-1 gap-x-8 gap-y-4 border-y border-gray-200 py-5 dark:border-dark-700 sm:grid-cols-2 lg:grid-cols-4">
        <div class="min-w-0"><div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.gatewayUser') }}</div><div class="mt-1 truncate text-sm font-medium">{{ organization.gateway_user.username || organization.gateway_user.email }}</div><div class="truncate text-xs text-gray-500">{{ organization.gateway_user.email }}</div></div>
        <div class="min-w-0"><div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.group') }}</div><div class="mt-1 break-words text-sm font-medium">{{ organization.group.name }}</div></div>
        <div><div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.memberCount') }}</div><div class="mt-1 text-sm font-medium tabular-nums">{{ organization.member_count }}</div></div>
        <div><div class="text-xs text-gray-500 dark:text-dark-400">{{ t('admin.desktop.configuration') }}</div><div class="mt-1 text-sm font-medium">{{ organization.target_config_assigned ? t('admin.desktop.configured') : t('admin.desktop.notConfigured') }}</div></div>
      </div>

      <div class="flex gap-1 overflow-x-auto border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('admin.desktop.detailTabs')">
        <button v-for="(tab, index) in tabs" :id="tabId(tab.value)" :key="tab.value" class="shrink-0 border-b-2 px-4 py-3 text-sm font-medium" :class="activeTab === tab.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-dark-400 dark:hover:text-gray-200'" type="button" role="tab" :aria-controls="panelId(tab.value)" :aria-selected="activeTab === tab.value" :tabindex="activeTab === tab.value ? 0 : -1" @click="setTab(tab.value)" @keydown="handleTabKeydown($event, index)">{{ tab.label }}</button>
      </div>

      <section v-if="activeTab === 'members'" :id="panelId('members')" class="min-w-0 space-y-4" role="tabpanel" :aria-labelledby="tabId('members')">
        <div class="flex flex-wrap items-center gap-3">
          <div class="min-w-0 flex-1 sm:max-w-72"><input v-model="memberSearch" class="input" type="search" :placeholder="t('admin.desktop.searchMembers')" @input="scheduleMembers" /></div>
          <Select v-model="memberStatus" class="w-40" :options="statusOptions" @change="resetMembers" />
          <div class="ml-auto flex items-center gap-2">
            <button class="btn btn-secondary" type="button" :disabled="membersLoading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadMembers"><Icon name="refresh" size="md" :class="membersLoading ? 'animate-spin' : ''" /></button>
            <button class="btn btn-primary" type="button" :disabled="organization?.status !== 'active'" @click="openCreateMember"><Icon name="plus" size="md" class="mr-1" />{{ t('admin.desktop.createMember') }}</button>
          </div>
        </div>
        <div class="min-w-0">
          <DataTable :columns="memberColumns" :data="members" :loading="membersLoading" row-key="public_id">
            <template #cell-name="{ row }"><div class="min-w-0 max-w-64"><div class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</div><div class="mt-1 font-mono text-xs text-gray-500">{{ row.public_id }}</div></div></template>
            <template #cell-status="{ row }"><StatusBadge :status="row.status" :label="statusLabel(row.status)" /></template>
            <template #cell-model_token_status="{ row }"><span :class="['badge', tokenBadge(row.model_token_status)]">{{ tokenStatusLabel(row.model_token_status) }}</span></template>
            <template #cell-updated_at="{ value }"><span class="whitespace-nowrap text-sm text-gray-500">{{ formatDateTime(value) }}</span></template>
            <template #cell-actions="{ row }">
              <div class="flex flex-wrap items-center gap-1">
                <button class="action-button" type="button" :title="t('common.edit')" :aria-label="t('common.edit')" @click="openEditMember(row)"><Icon name="edit" size="sm" /></button>
                <button class="action-button" type="button" :title="row.status === 'active' ? t('common.disable') : t('common.enable')" :aria-label="row.status === 'active' ? t('common.disable') : t('common.enable')" @click="toggleMember(row)"><Icon :name="row.status === 'active' ? 'ban' : 'checkCircle'" size="sm" /></button>
                <button class="action-button" type="button" :title="t('admin.desktop.rotateModelToken')" :aria-label="t('admin.desktop.rotateModelToken')" @click="confirmRotate(row)"><Icon name="refresh" size="sm" /></button>
                <button class="action-button action-button-danger" type="button" :title="t('common.delete')" :aria-label="t('common.delete')" @click="confirmDeleteMember(row)"><Icon name="trash" size="sm" /></button>
              </div>
            </template>
          </DataTable>
          <Pagination v-if="memberPagination.total > 0" :page="memberPagination.page" :page-size="memberPagination.page_size" :total="memberPagination.total" @update:page="changeMemberPage" @update:page-size="changeMemberPageSize" />
        </div>
      </section>

      <section v-else :id="panelId('configuration')" class="min-w-0" role="tabpanel" :aria-labelledby="tabId('configuration')">
        <form class="space-y-6" @submit.prevent="saveConfiguration">
          <div class="border-b border-gray-200 pb-6 dark:border-dark-700">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3"><h2 class="text-base font-semibold text-gray-900 dark:text-white">ChatGPT Codex</h2><label class="flex items-center gap-2 text-sm"><input v-model="configForm.chat.enabled" class="h-4 w-4 rounded" type="checkbox" />{{ t('common.enabled') }}</label></div>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <Input v-model="configForm.chat.provider_id" :label="t('admin.desktop.providerId')" required />
              <Input v-model="configForm.chat.display_name" :label="t('admin.desktop.displayName')" required />
              <Input v-model="configForm.chat.requested_model" :label="t('admin.desktop.requestedModel')" required />
              <Input model-value="/responses" :label="t('admin.desktop.wireApi')" readonly />
              <Input v-model="configForm.chat.minimum_app_version" :label="t('admin.desktop.minimumAppVersion')" />
              <label class="flex items-center gap-2 self-end pb-3 text-sm"><input v-model="configForm.chat.restart_required" class="h-4 w-4 rounded" type="checkbox" />{{ t('admin.desktop.restartRequired') }}</label>
            </div>
          </div>
          <div class="border-b border-gray-200 pb-6 dark:border-dark-700">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3"><h2 class="text-base font-semibold text-gray-900 dark:text-white">Workbuddy</h2><label class="flex items-center gap-2 text-sm text-gray-500"><input :checked="false" class="h-4 w-4 rounded" type="checkbox" disabled />{{ t('common.disabled') }}</label></div>
            <label class="mb-4 flex items-center gap-2 text-sm"><input v-model="configForm.includeWorkbuddy" class="h-4 w-4 rounded" type="checkbox" />{{ t('admin.desktop.configureWorkbuddy') }}</label>
            <div v-if="configForm.includeWorkbuddy" class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <Input v-model="configForm.work.provider_id" :label="t('admin.desktop.providerId')" required />
              <Input v-model="configForm.work.display_name" :label="t('admin.desktop.displayName')" required />
              <Input v-model="configForm.work.requested_model" :label="t('admin.desktop.requestedModel')" required />
              <Input model-value="/chat/completions" :label="t('admin.desktop.wireApi')" readonly />
              <Input v-model="configForm.work.minimum_app_version" :label="t('admin.desktop.minimumAppVersion')" />
              <label class="flex items-center gap-2 self-end pb-3 text-sm"><input v-model="configForm.work.restart_required" class="h-4 w-4 rounded" type="checkbox" />{{ t('admin.desktop.restartRequired') }}</label>
            </div>
          </div>
          <div class="flex justify-end"><button class="btn btn-primary" type="submit" :disabled="configSaving">{{ configSaving ? t('common.saving') : t('common.save') }}</button></div>
        </form>
      </section>
    </div>

    <BaseDialog :show="showOrganizationEdit" :title="t('admin.desktop.editOrganization')" width="wide" @close="closeOrganizationEdit">
      <form id="desktop-organization-edit" class="space-y-4" @submit.prevent="submitOrganizationEdit">
        <Input v-model="organizationForm.name" :label="t('admin.desktop.organizationName')" required />
        <div><label class="input-label mb-1.5 block">{{ t('common.status') }}</label><Select v-model="organizationForm.status" :options="editableStatusOptions" /></div>
        <div><label class="input-label mb-1.5 block">{{ t('admin.desktop.gatewayUser') }}</label><Select v-model="organizationForm.gateway_user_id" :options="gatewayUserOptions" searchable remote :loading="gatewayUsersLoading" :disabled="provisioningLocked" @search="loadGatewayUsers" /></div>
        <div><label class="input-label mb-1.5 block">{{ t('admin.desktop.group') }}</label><Select v-model="organizationForm.group_id" :options="groupOptions" searchable :loading="groupsLoading" :disabled="provisioningLocked" /></div>
        <p v-if="provisioningLocked" class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.provisioningLockedHint') }}</p>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" @click="closeOrganizationEdit">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" form="desktop-organization-edit" :disabled="organizationSaving">{{ organizationSaving ? t('common.saving') : t('common.save') }}</button></div></template>
    </BaseDialog>

    <BaseDialog :show="showMemberDialog" :title="editingMember ? t('admin.desktop.editMember') : t('admin.desktop.createMember')" width="narrow" @close="closeMemberDialog">
      <form id="desktop-member-form" class="space-y-4" @submit.prevent="saveMember">
        <Input v-model="memberForm.name" :label="t('admin.desktop.memberName')" required />
		<Input v-model="memberForm.phone" type="tel" :label="t('admin.desktop.phone')" required placeholder="13800138000" :hint="t('admin.desktop.phoneHint')" />
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" @click="closeMemberDialog">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" form="desktop-member-form" :disabled="memberSaving">{{ memberSaving ? t('common.saving') : t('common.save') }}</button></div></template>
    </BaseDialog>

    <ConfirmDialog :show="confirmState.show" :title="confirmState.title" :message="confirmState.message" :confirm-text="confirmState.confirmText" :loading="confirmPending" danger @confirm="runConfirmedAction" @cancel="closeConfirm" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { DesktopMember, DesktopModelTokenStatus, DesktopOrganization, DesktopStatus, DesktopTargetConfig, DesktopWireAPI } from '@/api/admin/desktop'
import type { AdminGroup, AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Icon from '@/components/icons/Icon.vue'

type DetailTab = 'members' | 'configuration'
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const organizationID = computed(() => String(route.params.organizationId || ''))
const organization = ref<DesktopOrganization | null>(null)
const activeTab = computed<DetailTab>(() => route.query.tab === 'configuration' ? 'configuration' : 'members')
const tabs = computed(() => [{ value: 'members' as const, label: t('admin.desktop.members') }, { value: 'configuration' as const, label: t('admin.desktop.configuration') }])
const members = ref<DesktopMember[]>([])
const membersLoading = ref(false)
const memberSearch = ref('')
const memberStatus = ref<DesktopStatus | ''>('')
const memberPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const showOrganizationEdit = ref(false)
const organizationSaving = ref(false)
const currentGatewayUser = ref<AdminUser | null>(null)
const gatewayUsers = ref<AdminUser[]>([])
const gatewayUsersLoading = ref(false)
const groups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)
const organizationForm = reactive({ name: '', status: 'active' as DesktopStatus, gateway_user_id: null as number | null, group_id: null as number | null })
const showMemberDialog = ref(false)
const editingMember = ref<DesktopMember | null>(null)
const memberSaving = ref(false)
const memberForm = reactive({ name: '', phone: '' })
const configSaving = ref(false)
const blankTarget = () => ({ enabled: false, provider_id: '', display_name: '', requested_model: '', minimum_app_version: '', restart_required: false })
const configForm = reactive({ chat: { ...blankTarget(), enabled: true }, work: blankTarget(), includeWorkbuddy: false })
const confirmState = reactive({ show: false, title: '', message: '', confirmText: '', action: null as null | (() => Promise<void>) })
const confirmPending = ref(false)
let memberTimer: ReturnType<typeof setTimeout> | undefined
let memberController: AbortController | undefined
let gatewayController: AbortController | undefined

const provisioningLocked = computed(() => (organization.value?.member_count ?? 0) > 0)
const statusOptions = computed(() => [{ value: '', label: t('common.all') }, { value: 'active', label: t('common.active') }, { value: 'disabled', label: t('common.disabled') }])
const editableStatusOptions = computed(() => statusOptions.value.slice(1))
const memberColumns = computed<Column[]>(() => [
	{ key: 'name', label: t('admin.desktop.member') }, { key: 'phone', label: t('admin.desktop.phone') },
  { key: 'status', label: t('common.status') }, { key: 'model_token_status', label: t('admin.desktop.modelToken') },
  { key: 'updated_at', label: t('admin.desktop.updatedAt') }, { key: 'actions', label: t('common.actions') },
])
const gatewayUserOptions = computed(() => {
  const all = [...gatewayUsers.value]
  if (currentGatewayUser.value && !all.some((user) => user.id === currentGatewayUser.value?.id)) all.unshift(currentGatewayUser.value)
  return all.map((user) => ({ value: user.id, label: `${user.username || user.email} · ${user.email}` }))
})
const groupOptions = computed(() => groups.value.map((group) => ({ value: group.id, label: group.name })))

function errorMessage(error: unknown): string { const reason = (error as { reason?: string })?.reason; return reason ? t(`admin.desktop.errors.${reason}`) : (error as { message?: string })?.message || t('admin.desktop.errors.UNKNOWN') }
function statusLabel(value: DesktopStatus): string { return value === 'active' ? t('common.active') : t('common.disabled') }
function tokenStatusLabel(value: DesktopModelTokenStatus): string { return t(`admin.desktop.tokenStatus.${value}`) }
function tokenBadge(value: DesktopModelTokenStatus): string { return value === 'active' ? 'badge-success' : value === 'disabled' ? 'badge-warning' : 'badge-gray' }
function setTab(tab: DetailTab) { void router.replace({ query: { ...route.query, tab } }) }
function tabId(tab: DetailTab) { return `desktop-organization-tab-${tab}` }
function panelId(tab: DetailTab) { return `desktop-organization-panel-${tab}` }
function handleTabKeydown(event: KeyboardEvent, index: number) {
  let nextIndex: number | undefined
  if (event.key === 'ArrowRight') nextIndex = (index + 1) % tabs.value.length
  else if (event.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.value.length) % tabs.value.length
  else if (event.key === 'Home') nextIndex = 0
  else if (event.key === 'End') nextIndex = tabs.value.length - 1
  if (nextIndex === undefined) return

  event.preventDefault()
  const nextTab = tabs.value[nextIndex].value
  setTab(nextTab)
  void nextTick(() => document.getElementById(tabId(nextTab))?.focus())
}

async function loadOrganization() {
  try {
    const result = await adminAPI.desktop.getOrganization(organizationID.value)
    organization.value = result
    currentGatewayUser.value = await adminAPI.desktop.getGatewayUser(result.gateway_user.id)
    applyConfiguration(result.target_config)
  } catch (error) { appStore.showError(errorMessage(error)) }
}
async function loadMembers() {
  memberController?.abort(); memberController = new AbortController(); membersLoading.value = true
  try {
    const result = await adminAPI.desktop.listMembers(organizationID.value, memberPagination.page, memberPagination.page_size, { search: memberSearch.value.trim(), status: memberStatus.value }, memberController.signal)
    members.value = result.items; memberPagination.total = result.total
  } catch (error) { if ((error as { code?: string })?.code !== 'ERR_CANCELED') appStore.showError(errorMessage(error)) }
  finally { membersLoading.value = false }
}
function scheduleMembers() { clearTimeout(memberTimer); memberTimer = setTimeout(resetMembers, 300) }
function resetMembers() { memberPagination.page = 1; void loadMembers() }
function changeMemberPage(page: number) { memberPagination.page = page; void loadMembers() }
function changeMemberPageSize(size: number) { memberPagination.page_size = size; memberPagination.page = 1; void loadMembers() }

async function loadGatewayUsers(query = '') {
  gatewayController?.abort(); gatewayController = new AbortController(); gatewayUsersLoading.value = true
  try { gatewayUsers.value = (await adminAPI.desktop.listAvailableGatewayUsers(query, gatewayController.signal)).items }
  catch (error) { if ((error as { code?: string })?.code !== 'ERR_CANCELED') appStore.showError(errorMessage(error)) }
  finally { gatewayUsersLoading.value = false }
}
async function openEditOrganization() {
  if (!organization.value) return
  Object.assign(organizationForm, { name: organization.value.name, status: organization.value.status, gateway_user_id: organization.value.gateway_user.id, group_id: organization.value.group.id })
  showOrganizationEdit.value = true; groupsLoading.value = true
  void loadGatewayUsers()
  try { groups.value = await adminAPI.desktop.listActiveGroups() } catch (error) { appStore.showError(errorMessage(error)) } finally { groupsLoading.value = false }
}
function closeOrganizationEdit() { if (!organizationSaving.value) showOrganizationEdit.value = false }
function submitOrganizationEdit() {
  if (!organization.value || !organizationForm.name.trim()) { appStore.showError(t('admin.desktop.errors.VALIDATION_FAILED')); return }
  if (organization.value.status === 'active' && organizationForm.status === 'disabled') {
    openConfirm(t('admin.desktop.disableOrganization'), t('admin.desktop.disableOrganizationImpact', { name: organization.value.name }), t('common.disable'), saveOrganization)
    return
  }
  void saveOrganization()
}
async function saveOrganization() {
  if (!organization.value) return
  organizationSaving.value = true
  try {
    const input = { name: organizationForm.name.trim(), status: organizationForm.status } as { name: string; status: DesktopStatus; gateway_user_id?: number; group_id?: number }
    if (!provisioningLocked.value && organizationForm.gateway_user_id && organizationForm.group_id) { input.gateway_user_id = organizationForm.gateway_user_id; input.group_id = organizationForm.group_id }
    organization.value = await adminAPI.desktop.updateOrganization(organizationID.value, input)
    appStore.showSuccess(t('admin.desktop.organizationUpdated')); showOrganizationEdit.value = false; closeConfirm(); await loadMembers()
  } catch (error) { appStore.showError(errorMessage(error)) } finally { organizationSaving.value = false }
}

function openCreateMember() { editingMember.value = null; Object.assign(memberForm, { name: '', phone: '' }); showMemberDialog.value = true }
function openEditMember(member: DesktopMember) { editingMember.value = member; Object.assign(memberForm, { name: member.name, phone: member.phone }); showMemberDialog.value = true }
function closeMemberDialog() { if (!memberSaving.value) showMemberDialog.value = false }
async function saveMember() {
	if (!memberForm.name.trim() || !memberForm.phone.trim()) { appStore.showError(t('admin.desktop.errors.VALIDATION_FAILED')); return }
	memberSaving.value = true
	try {
		if (editingMember.value) await adminAPI.desktop.updateMember(organizationID.value, editingMember.value.public_id, { name: memberForm.name.trim(), ...(memberForm.phone.trim() !== editingMember.value.phone ? { phone: memberForm.phone.trim() } : {}) })
    else await adminAPI.desktop.createMember(organizationID.value, { name: memberForm.name.trim(), phone: memberForm.phone.trim() })
    appStore.showSuccess(t(editingMember.value ? 'admin.desktop.memberUpdated' : 'admin.desktop.memberCreated')); showMemberDialog.value = false; await Promise.all([loadMembers(), loadOrganization()])
  } catch (error) { appStore.showError(errorMessage(error)) } finally { memberSaving.value = false }
}
function toggleMember(member: DesktopMember) {
  const action = async () => { try { await adminAPI.desktop.updateMember(organizationID.value, member.public_id, { status: member.status === 'active' ? 'disabled' : 'active' }); appStore.showSuccess(t('admin.desktop.memberUpdated')); closeConfirm(); await loadMembers() } catch (error) { appStore.showError(errorMessage(error)) } }
  if (member.status === 'active') openConfirm(t('admin.desktop.disableMember'), t('admin.desktop.disableMemberImpact', { name: member.name }), t('common.disable'), action)
  else void action()
}
function confirmRotate(member: DesktopMember) { openConfirm(t('admin.desktop.rotateModelToken'), t('admin.desktop.rotateModelTokenImpact', { name: member.name }), t('admin.desktop.rotate'), async () => { try { await adminAPI.desktop.rotateModelToken(organizationID.value, member.public_id); appStore.showSuccess(t('admin.desktop.modelTokenRotated')); closeConfirm(); await loadMembers() } catch (error) { appStore.showError(errorMessage(error)) } }) }
function confirmDeleteMember(member: DesktopMember) { openConfirm(t('admin.desktop.deleteMember'), t('admin.desktop.deleteMemberImpact', { name: member.name }), t('common.delete'), async () => { try { await adminAPI.desktop.deleteMember(organizationID.value, member.public_id); appStore.showSuccess(t('admin.desktop.memberDeleted')); closeConfirm(); await Promise.all([loadMembers(), loadOrganization()]) } catch (error) { appStore.showError(errorMessage(error)) } }) }
function openConfirm(title: string, message: string, confirmText: string, action: () => Promise<void>) { Object.assign(confirmState, { show: true, title, message, confirmText, action }) }
function closeConfirm() { confirmState.show = false; confirmState.action = null }
async function runConfirmedAction() {
  if (confirmPending.value || !confirmState.action) return
  confirmPending.value = true
  try { await confirmState.action() } finally { confirmPending.value = false }
}

function applyConfiguration(value?: DesktopTargetConfig | null) {
  const chat = value?.targets.chatgpt_codex; const work = value?.targets.workbuddy
  Object.assign(configForm.chat, chat ? { ...chat, minimum_app_version: chat.minimum_app_version || '' } : { ...blankTarget(), enabled: true })
  Object.assign(configForm.work, work ? { ...work, minimum_app_version: work.minimum_app_version || '' } : blankTarget())
  configForm.includeWorkbuddy = Boolean(work)
}
function buildTarget(target: typeof configForm.chat, wireAPI: DesktopWireAPI, enabled = target.enabled) { return { enabled, provider_id: target.provider_id.trim(), display_name: target.display_name.trim(), requested_model: target.requested_model.trim(), wire_api: wireAPI, ...(target.minimum_app_version.trim() ? { minimum_app_version: target.minimum_app_version.trim() } : {}), restart_required: target.restart_required } }
async function saveConfiguration() {
  const chat = buildTarget(configForm.chat, 'responses')
  const work = configForm.includeWorkbuddy ? buildTarget(configForm.work, 'chat_completions', false) : undefined
  if (![chat, work].filter(Boolean).every((target) => target?.provider_id && target.display_name && target.requested_model)) { appStore.showError(t('admin.desktop.errors.VALIDATION_FAILED')); return }
  configSaving.value = true
  try { organization.value = await adminAPI.desktop.updateModelConfiguration(organizationID.value, { schema_version: 1, targets: { chatgpt_codex: chat, ...(work ? { workbuddy: work } : {}) } }); appStore.showSuccess(t('admin.desktop.configurationSaved')) }
  catch (error) { appStore.showError(errorMessage(error)) } finally { configSaving.value = false }
}

watch(organizationID, async () => { await Promise.all([loadOrganization(), loadMembers()]) })
onMounted(() => { void Promise.all([loadOrganization(), loadMembers()]) })
onBeforeUnmount(() => { clearTimeout(memberTimer); memberController?.abort(); gatewayController?.abort() })
</script>

<style scoped>
.action-button { @apply rounded-md p-2 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400; }
.action-button-danger { @apply hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400; }
</style>
