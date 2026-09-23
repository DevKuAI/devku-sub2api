<template>
  <section v-if="enabled || error" class="card mt-6 space-y-4 p-6" :aria-busy="loading">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="font-semibold">{{ t('bi.grants') }}</h2>
      <button v-if="enabled" type="button" class="btn btn-secondary" :disabled="busy || loading" @click="openEditor()">{{ t('bi.addGrant') }}</button>
    </div>
    <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('bi.grantsDescription') }}</p>
    <p v-if="error" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
    <p v-if="enabled && !loading && !error && grants.length === 0" class="text-sm text-gray-500">{{ t('bi.noGrants') }}</p>
    <ul class="divide-y divide-gray-200 dark:divide-dark-700">
      <li v-for="grant in grants" :key="grant.manager_id" class="flex flex-wrap items-center justify-between gap-3 py-3">
        <div>
          <p class="font-medium">{{ grant.display_name || grant.user_id }} · {{ t(`bi.roles.${grant.role}`) }}</p>
          <p class="text-sm text-gray-500">{{ grant.revoked ? t('bi.grantRevoked') : grant.all_teams ? t('bi.allTeams') : grant.team_ids.join(', ') }}</p>
        </div>
        <div class="flex gap-2">
          <button class="btn btn-secondary" type="button" :disabled="busy" @click="openEditor(grant)">{{ t('common.edit') }}</button>
          <button v-if="!grant.revoked" class="btn btn-secondary text-red-600" type="button" :disabled="busy" @click="revokeTarget = grant">{{ t('bi.revokeGrant') }}</button>
        </div>
      </li>
    </ul>
    <div v-if="enabled" class="flex flex-wrap gap-2">
      <button type="button" class="btn btn-secondary" :disabled="busy || loading" @click="load(1)">{{ t('common.refresh') }}</button>
      <button v-if="hasMore" type="button" class="btn btn-secondary" :disabled="busy || loading" @click="load(page + 1)">{{ t('bi.loadMore') }}</button>
    </div>
    <BaseDialog :show="editing" :title="t(editingGrant ? 'bi.editGrant' : 'bi.addGrant')" width="wide" @close="closeEditor">
      <form id="bi-grant-form" class="space-y-4" @submit.prevent="save">
        <div>
          <label for="bi-grant-user" class="mb-1 block text-sm">{{ t('bi.userID') }}</label>
          <input id="bi-grant-user" v-model.number="form.user_id" type="number" min="1" step="1" required class="input" :disabled="busy || !!editingGrant" />
        </div>
        <div>
          <label for="bi-grant-role" class="mb-1 block text-sm">{{ t('bi.role') }}</label>
          <select id="bi-grant-role" v-model="form.role" class="input" :disabled="busy">
            <option v-for="role in roles" :key="role" :value="role">{{ t(`bi.roles.${role}`) }}</option>
          </select>
        </div>
        <label class="flex items-center gap-2 text-sm"><input v-model="form.all_teams" type="checkbox" :disabled="busy" />{{ t('bi.allTeams') }}</label>
        <div v-if="!form.all_teams">
          <label for="bi-grant-teams" class="mb-1 block text-sm">{{ t('bi.teams') }}</label>
          <input id="bi-grant-teams" v-model="teamText" class="input" :disabled="busy" />
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
    <ConfirmDialog :show="!!revokeTarget" :title="t('bi.revokeGrant')" :message="t('bi.revokeGrantMessage')" :loading="busy" danger @confirm="revoke" @cancel="revokeTarget = null" />
  </section>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { isBIEnabled, listGrants, revokeGrant, saveGrant, type BIGrant, type BIGrantInput } from '@/api/bi'

const props = defineProps<{ organizationId: string }>()
const { t } = useI18n()
const enabled = ref(false)
const loading = ref(false)
const busy = ref(false)
const error = ref('')
const editError = ref('')
const grants = ref<BIGrant[]>([])
const capabilities = ref<string[]>([])
const page = ref(1)
const hasMore = ref(false)
const editing = ref(false)
const editingGrant = ref<BIGrant | null>(null)
const revokeTarget = ref<BIGrant | null>(null)
const roles = ['org_admin', 'team_manager', 'viewer'] as const
const teamText = ref('')
const form = reactive<BIGrantInput>({ user_id: 0, role: 'viewer', all_teams: false, team_ids: [], capabilities: [], expected_revision: 0 })
let generation = 0

function message(cause: unknown) { return t((cause as { status?: number }).status === 409 ? 'bi.reload' : 'bi.failed') }

async function load(nextPage = 1) {
  const current = generation
  loading.value = true
  error.value = ''
  try {
    const result = await listGrants(props.organizationId, nextPage)
    if (current !== generation) return
    grants.value = nextPage === 1 ? result.items : [...grants.value, ...result.items]
    page.value = result.page
    hasMore.value = result.has_more
    capabilities.value = result.capabilities
  } catch (cause) { if (current === generation) error.value = message(cause) }
  finally { if (current === generation) loading.value = false }
}

function openEditor(grant?: BIGrant) {
  editingGrant.value = grant || null
  Object.assign(form, { user_id: grant?.user_id || 0, role: grant?.role || 'viewer', all_teams: grant?.all_teams || false,
    team_ids: [...(grant?.team_ids || [])], capabilities: [...(grant?.capabilities || [])], expected_revision: grant?.revision || 0 })
  teamText.value = form.team_ids.join(', ')
  editError.value = ''
  editing.value = true
}

function closeEditor() { if (!busy.value) editing.value = false }

async function save() {
  if (busy.value) return
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
  editing.value = false
  revokeTarget.value = null
  grants.value = []
  error.value = ''
  try {
    const available = await isBIEnabled()
    if (current !== generation) return
    enabled.value = available
    if (available) await load()
  } catch (cause) { if (current === generation) error.value = message(cause) }
}, { immediate: true })
</script>
