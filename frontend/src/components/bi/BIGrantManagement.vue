<template>
  <section class="card space-y-4 p-6" :aria-busy="loading">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="font-semibold">{{ t('bi.grants') }}</h2>
      <button v-if="enabled" type="button" class="btn btn-secondary" :disabled="busy || loading" @click="openEditor()">{{ t('bi.addGrant') }}</button>
    </div>
    <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('bi.grantsDescription') }}</p>
    <p v-if="loading && !enabled" class="text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="!enabled && !error" class="text-sm text-gray-500">{{ t('bi.disabled') }}</p>
    <p v-if="error" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-if="enabled && !loading && !error && grants.length === 0" class="text-sm text-gray-500">{{ t('bi.noGrants') }}</p>
    <ul class="divide-y divide-gray-200 dark:divide-dark-700">
      <li v-for="grant in grants" :key="grant.manager_id" class="flex flex-wrap items-center justify-between gap-3 py-3">
        <div>
          <p class="font-medium">{{ grant.display_name || grant.user_id }} · {{ t(`bi.roles.${grant.role}`) }}</p>
          <p class="text-sm text-gray-500">{{ grant.revoked ? t('bi.grantRevoked') : grant.all_teams ? t('bi.allTeams') : grant.team_ids.join(', ') }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-secondary" type="button" :disabled="busy" @click="openEditor(grant)">{{ t('common.edit') }}</button>
          <button v-if="!grant.revoked" class="btn btn-secondary" type="button" :disabled="busy" @click="openBinding(grant)">{{ t('bi.bindWechat') }}</button>
          <button v-if="!grant.revoked" class="btn btn-secondary text-red-600" type="button" :disabled="busy" @click="revokeTarget = grant">{{ t('bi.revokeGrant') }}</button>
        </div>
      </li>
    </ul>
    <p v-if="bindingNotice" role="status" class="text-sm text-green-700 dark:text-green-300">{{ bindingNotice }}</p>
    <div v-if="enabled" class="flex flex-wrap gap-2">
      <button type="button" class="btn btn-secondary" :disabled="busy || loading" @click="load(1)">{{ t('common.refresh') }}</button>
      <button v-if="hasMore" type="button" class="btn btn-secondary" :disabled="busy || loading" @click="load(page + 1)">{{ t('bi.loadMore') }}</button>
    </div>
    <BaseDialog :show="editing" :title="t(editingGrant ? 'bi.editGrant' : 'bi.addGrant')" width="wide" @close="closeEditor">
      <form id="bi-grant-form" class="space-y-4" @submit.prevent="save">
        <div>
          <label for="bi-grant-user" class="mb-1 block text-sm">{{ t('bi.selectUser') }}</label>
          <Select
            id="bi-grant-user"
            :model-value="form.user_id || null"
            :options="userOptions"
            :placeholder="t('bi.selectUserPlaceholder')"
            :search-placeholder="t('bi.searchUser')"
            :empty-text="t(usersLoadFailed ? 'bi.userSearchFailed' : 'bi.noUsers')"
            :loading="usersLoading"
            :disabled="busy || !!editingGrant"
            :error="!!userSelectionError"
            :aria-label="t('bi.selectUser')"
            :aria-describedby="userSelectionError ? 'bi-grant-user-hint bi-grant-user-error' : 'bi-grant-user-hint'"
            searchable
            remote
            @update:model-value="form.user_id = Number($event) || 0"
            @change="onUserChange"
            @search="loadUsers"
          />
          <p id="bi-grant-user-hint" class="input-hint">
            {{ t('bi.userIDHint') }}
            <a href="/admin/users" target="_blank" rel="noopener noreferrer" class="font-medium text-primary-600 underline dark:text-primary-400">{{ t('bi.openUserManagement') }}</a>
          </p>
          <p v-if="userSelectionError" id="bi-grant-user-error" class="mt-1 text-sm text-red-600 dark:text-red-400" role="alert">{{ userSelectionError }}</p>
          <dl v-if="selectedUser" class="mt-3 grid gap-3 border-t border-gray-200 pt-3 text-sm dark:border-dark-700 sm:grid-cols-3">
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('bi.userName') }}</dt><dd class="mt-1 break-words font-medium">{{ selectedUser.username || '—' }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('bi.userEmail') }}</dt><dd class="mt-1 break-all">{{ selectedUser.email || t(usersLoadFailed ? 'bi.userDetailsUnavailable' : 'common.loading') }}</dd></div>
            <div><dt class="text-xs text-gray-500 dark:text-dark-400">{{ t('bi.userID') }}</dt><dd class="mt-1 font-medium tabular-nums">{{ selectedUser.id }}</dd></div>
          </dl>
        </div>
        <div>
          <label for="bi-grant-role" class="mb-1 block text-sm">{{ t('bi.role') }}</label>
          <select id="bi-grant-role" v-model="form.role" class="input" :disabled="busy">
            <option v-for="role in roles" :key="role" :value="role">{{ t(`bi.roles.${role}`) }}</option>
          </select>
        </div>
        <label class="flex items-center gap-2 text-sm"><input v-model="form.all_teams" type="checkbox" :disabled="busy" aria-describedby="bi-grant-all-teams-hint" />{{ t('bi.allTeams') }}</label>
        <p id="bi-grant-all-teams-hint" class="input-hint">{{ t('bi.allTeamsHint') }}</p>
        <div v-if="!form.all_teams">
          <label for="bi-grant-teams" class="mb-1 block text-sm">{{ t('bi.teams') }}</label>
          <input id="bi-grant-teams" v-model="teamText" class="input" :disabled="busy" aria-describedby="bi-grant-teams-hint" />
          <p id="bi-grant-teams-hint" class="input-hint">{{ t('bi.teamsHint') }}</p>
        </div>
        <fieldset class="grid grid-cols-1 gap-3 sm:grid-cols-2" :disabled="busy">
          <legend class="mb-2 text-sm font-medium">{{ t('bi.capabilities') }}</legend>
          <label v-for="capability in capabilities" :key="capability" class="flex items-center gap-2 text-sm">
            <input v-model="form.capabilities" type="checkbox" :value="capability" />{{ t(`bi.capabilityLabels.${capability}`) }}
          </label>
        </fieldset>
        <p v-if="editError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ editError }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" :disabled="busy" @click="closeEditor">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" form="bi-grant-form" type="submit" :disabled="busy || form.capabilities.length === 0" :aria-busy="busy">{{ t('common.save') }}</button>
        </div>
      </template>
    </BaseDialog>
    <BaseDialog :show="!!bindingTarget" :title="t('bi.bindWechat')" width="narrow" @close="closeBinding">
      <form id="bi-admin-binding-form" class="space-y-4" @submit.prevent="approveGrantBindingCode">
        <p class="text-sm text-gray-600 dark:text-dark-300">{{ t('bi.bindWechatTarget', { account: bindingTarget?.display_name || '', id: bindingTarget?.user_id || '' }) }}</p>
        <div>
          <label for="bi-admin-binding-code" class="mb-1 block text-sm font-medium">{{ t('bi.userCode') }}</label>
          <input id="bi-admin-binding-code" v-model="bindingCode" class="input max-w-xs font-mono uppercase" maxlength="8" pattern="[A-Za-z0-9]{8}" autocomplete="off" required :disabled="busy" aria-describedby="bi-admin-binding-hint" />
          <p id="bi-admin-binding-hint" class="input-hint">{{ t('bi.adminBindingHint') }}</p>
        </div>
        <label class="flex items-start gap-2 text-sm"><input v-model="bindingConfirmed" type="checkbox" required :disabled="busy" class="mt-1" />{{ t('bi.adminBindingConfirm') }}</label>
        <p v-if="bindingError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ bindingError }}</p>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" type="button" :disabled="busy" @click="closeBinding">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" form="bi-admin-binding-form" type="submit" :disabled="busy || !bindingConfirmed || !validBindingCode" :aria-busy="busy">{{ t('bi.approve') }}</button>
        </div>
      </template>
    </BaseDialog>
    <ConfirmDialog :show="!!revokeTarget" :title="t('bi.revokeGrant')" :message="t('bi.revokeGrantMessage')" :loading="busy" danger @confirm="revoke" @cancel="revokeTarget = null" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminUser } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import { approveGrantBinding, isBIEnabled, listGrants, revokeGrant, saveGrant, type BIGrant, type BIGrantInput } from '@/api/bi'

const props = defineProps<{ organizationId: string }>()
const { t } = useI18n()
const enabled = ref(false)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const editError = ref('')
const grants = ref<BIGrant[]>([])
const capabilities = ref<string[]>([])
const users = ref<AdminUser[]>([])
const selectedUser = ref<Pick<AdminUser, 'id' | 'username' | 'email'> | null>(null)
const usersLoading = ref(false)
const usersLoadFailed = ref(false)
const userSelectionError = ref('')
const page = ref(1)
const hasMore = ref(false)
const editing = ref(false)
const editingGrant = ref<BIGrant | null>(null)
const revokeTarget = ref<BIGrant | null>(null)
const bindingTarget = ref<BIGrant | null>(null)
const bindingCode = ref('')
const bindingConfirmed = ref(false)
const bindingError = ref('')
const bindingNotice = ref('')
const roles = ['org_admin', 'team_manager', 'viewer'] as const
const teamText = ref('')
const form = reactive<BIGrantInput>({ user_id: 0, role: 'viewer', all_teams: false, team_ids: [], capabilities: [], expected_revision: 0 })
let generation = 0
let loadGeneration = 0
let editorGeneration = 0
let usersController: AbortController | undefined

const userOptions = computed(() => {
  const byID = new Map<number, Pick<AdminUser, 'id' | 'username' | 'email'>>()
  if (selectedUser.value) byID.set(selectedUser.value.id, selectedUser.value)
  for (const user of users.value) byID.set(user.id, user)
  return [...byID.values()].map(user => ({
    value: user.id,
    label: [user.username, user.email, `#${user.id}`].filter(Boolean).join(' · '),
  }))
})
const validBindingCode = computed(() => /^[A-Z0-9]{8}$/.test(bindingCode.value.trim().toUpperCase()))

function message(cause: unknown) { return t((cause as { status?: number }).status === 409 ? 'bi.reload' : 'bi.failed') }

async function load(nextPage = 1) {
  const current = ++loadGeneration
  loading.value = true
  error.value = ''
  try {
    const result = await listGrants(props.organizationId, nextPage)
    if (current !== loadGeneration) return
    grants.value = nextPage === 1 ? result.items : [...grants.value, ...result.items]
    page.value = result.page
    hasMore.value = result.has_more
    capabilities.value = result.capabilities
  } catch (cause) { if (current === loadGeneration) error.value = message(cause) }
  finally { if (current === loadGeneration) loading.value = false }
}

function openEditor(grant?: BIGrant) {
  const current = ++editorGeneration
  usersController?.abort()
  users.value = []
  usersLoadFailed.value = false
  userSelectionError.value = ''
  selectedUser.value = grant ? { id: grant.user_id, username: grant.display_name, email: '' } : null
  editingGrant.value = grant || null
  Object.assign(form, { user_id: grant?.user_id || 0, role: grant?.role || 'viewer', all_teams: grant?.all_teams || false,
    team_ids: [...(grant?.team_ids || [])], capabilities: [...(grant?.capabilities || [])], expected_revision: grant?.revision || 0 })
  teamText.value = form.team_ids.join(', ')
  editError.value = ''
  editing.value = true
  if (grant) {
    void adminAPI.users.getById(grant.user_id).then(user => {
      if (current === editorGeneration) selectedUser.value = user
    }).catch(() => {
      if (current === editorGeneration) usersLoadFailed.value = true
    })
  } else {
    void loadUsers()
  }
}

function closeEditor() {
  if (busy.value) return
  editing.value = false
  ++editorGeneration
  usersController?.abort()
}

async function loadUsers(search = '') {
  usersController?.abort()
  const controller = new AbortController()
  usersController = controller
  usersLoading.value = true
  usersLoadFailed.value = false
  try {
    const result = await adminAPI.users.list(1, 20, { status: 'active', search: search.trim() || undefined }, { signal: controller.signal })
    if (!controller.signal.aborted) users.value = result.items
  } catch {
    if (!controller.signal.aborted) {
      users.value = []
      usersLoadFailed.value = true
    }
  } finally {
    if (usersController === controller) usersLoading.value = false
  }
}

function onUserChange(value: string | number | boolean | null) {
  const userID = Number(value)
  selectedUser.value = users.value.find(user => user.id === userID) || (selectedUser.value?.id === userID ? selectedUser.value : null)
  userSelectionError.value = ''
}

function openBinding(grant: BIGrant) {
  bindingTarget.value = grant
  bindingCode.value = ''
  bindingConfirmed.value = false
  bindingError.value = ''
  bindingNotice.value = ''
}

function closeBinding() {
  if (!busy.value) bindingTarget.value = null
}

async function approveGrantBindingCode() {
  if (!bindingTarget.value || busy.value || !bindingConfirmed.value || !validBindingCode.value) return
  busy.value = true
  bindingError.value = ''
  const current = generation
  try {
    await approveGrantBinding(props.organizationId, bindingTarget.value.manager_id, bindingCode.value.trim().toUpperCase())
    if (current !== generation) return
    bindingTarget.value = null
    bindingNotice.value = t('bi.adminBindingApproved')
  } catch (cause) {
    if (current === generation) {
      const code = (cause as { code?: string; reason?: string }).code || (cause as { reason?: string }).reason
      bindingError.value = t(code === 'BINDING_EXPIRED' ? 'bi.expired' : code === 'BINDING_CONFLICT' ? 'bi.conflict' : code === 'NOT_FOUND' ? 'bi.adminBindingGrantMissing' : 'bi.failed')
    }
  } finally { busy.value = false }
}

async function save() {
  if (busy.value) return
  if (!Number.isInteger(form.user_id) || form.user_id <= 0) {
    userSelectionError.value = t('bi.selectUserRequired')
    document.getElementById('bi-grant-user')?.focus()
    return
  }
  busy.value = true
  editError.value = ''
  const current = generation
  try {
    await saveGrant(props.organizationId, { ...form, capabilities: [...form.capabilities], team_ids: form.all_teams ? [] : teamText.value.split(',').map(id => id.trim()).filter(Boolean) })
    if (current !== generation) return
    editing.value = false
    await load()
  } catch (cause) { if (current === generation) editError.value = message(cause) }
  finally { busy.value = false }
}

async function revoke() {
  if (!revokeTarget.value || busy.value) return
  busy.value = true
  const current = generation
  try {
    await revokeGrant(props.organizationId, revokeTarget.value)
    if (current !== generation) return
    revokeTarget.value = null
    await load()
  } catch (cause) { if (current === generation) error.value = message(cause) }
  finally { busy.value = false }
}

watch(() => props.organizationId, async () => {
  const current = ++generation
  ++loadGeneration
  ++editorGeneration
  usersController?.abort()
  users.value = []
  selectedUser.value = null
  loading.value = true
  enabled.value = false
  editing.value = false
  revokeTarget.value = null
  bindingTarget.value = null
  bindingNotice.value = ''
  grants.value = []
  capabilities.value = []
  page.value = 1
  hasMore.value = false
  error.value = ''
  try {
    const available = await isBIEnabled()
    if (current !== generation) return
    enabled.value = available
    if (available) await load()
  } catch (cause) { if (current === generation) error.value = message(cause) }
  finally { if (current === generation) loading.value = false }
}, { immediate: true })

onBeforeUnmount(() => { generation++; loadGeneration++; editorGeneration++; usersController?.abort() })
</script>
