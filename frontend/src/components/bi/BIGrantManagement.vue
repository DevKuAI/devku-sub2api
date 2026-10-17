<template>
  <section class="card bi-panel bi-access space-y-5">
    <header class="bi-toolbar">
      <div class="min-w-0 flex-1 basis-80"><h2 class="bi-heading">{{ t('biNative.accessManagement') }}</h2><p class="bi-caption mt-2">{{ t('biNative.identityHint') }}</p></div>
      <div class="bi-row-actions"><slot name="actions" /><button class="btn btn-secondary" :disabled="availabilityLoading" @click="checkAvailability">{{ t('biNative.refreshAccess') }}</button><button class="btn btn-primary" :disabled="busy || enabled !== true" @click="openInvite">{{ t('biNative.createInvite') }}</button></div>
    </header>
    <p v-if="availabilityLoading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
    <p v-else-if="enabled === false" role="status" class="bi-caption">{{ t('biNative.disabled') }}</p>
    <div v-if="availabilityError" role="alert" class="bi-error"><p>{{ availabilityError }}</p><button class="btn btn-secondary mt-3" @click="checkAvailability">{{ t('biNative.retry') }}</button></div>
    <div class="bi-access-grid">
      <section class="bi-access-region" :aria-label="t('biNative.users')" :aria-busy="loading.users">
        <header class="bi-toolbar"><h3 class="bi-heading">{{ t('biNative.users') }}</h3><button class="btn btn-secondary" :disabled="loading.users || enabled !== true" @click="loadUsers()">{{ t('common.refresh') }}</button></header>
        <div v-if="errors.users" role="alert" class="bi-error"><p>{{ errors.users }}</p><button class="btn btn-secondary mt-3" @click="loadUsers()">{{ t('biNative.retry') }}</button></div>
        <p v-if="loading.users" role="status" class="bi-caption">{{ t('common.loading') }}</p>
        <p v-if="enabled === true && !loading.users && !users.length && !errors.users" class="bi-empty">{{ t('biNative.noUsers') }}</p>
        <div class="bi-access-list">
          <div v-for="user in users" :key="user.bi_user_id" class="bi-access-row">
            <div class="min-w-0 flex-1 space-y-2"><p class="break-words font-medium">{{ user.organization_alias || biText(user.display_name) }}</p><div class="flex flex-wrap gap-2"><span class="bi-badge" :data-tone="biStatusTone(user.status)">{{ user.status === 'active' ? t('common.enabled') : t('biNative.revoked') }}</span><span v-if="user.user_status === 'disabled'" class="bi-badge" data-tone="warning">{{ t('biNative.identityDisabled') }}</span></div><p class="bi-caption">{{ user.capabilities.map(cap => t(`biNative.capabilities.${cap}`)).join('、') || '-' }}</p><p class="bi-caption break-all">{{ t('biNative.identityID') }}: {{ user.bi_user_id }}</p><p class="bi-caption">{{ t('biNative.wholeOrganization') }} · v{{ user.revision }}</p></div>
            <button class="btn btn-secondary" :disabled="busy || enabled !== true" @click="openUser(user)">{{ t('common.edit') }}</button>
          </div>
        </div>
        <button v-if="usersMore" class="btn btn-secondary mt-3" :disabled="loading.users" @click="loadUsers(userPage + 1)">{{ t('bi.loadMore') }}</button>
      </section>
      <section class="bi-access-region" :aria-label="t('biNative.invitations')" :aria-busy="loading.invites">
        <header class="bi-toolbar"><h3 class="bi-heading">{{ t('biNative.invitations') }}</h3><button class="btn btn-secondary" :disabled="loading.invites || enabled !== true" @click="loadInvites()">{{ t('common.refresh') }}</button></header>
        <p class="bi-caption mb-3">{{ t('biNative.inviteHint') }}</p>
        <p v-if="invites.length" class="bi-caption mb-3">{{ t('biNative.inviteCredentialsHint') }}</p>
        <div v-if="errors.invites" role="alert" class="bi-error"><p>{{ errors.invites }}</p><button class="btn btn-secondary mt-3" @click="loadInvites()">{{ t('biNative.retry') }}</button></div>
        <p v-if="loading.invites" role="status" class="bi-caption">{{ t('common.loading') }}</p>
        <p v-if="enabled === true && !loading.invites && !invites.length && !errors.invites" class="bi-empty">{{ t('biNative.noInvites') }}</p>
        <div class="bi-access-list">
          <div v-for="invite in invites" :key="invite.id" class="bi-access-row">
            <div class="min-w-0 flex-1 space-y-2"><p class="break-words font-medium">{{ invite.recipient_name || invite.note || invite.id }}</p><span class="bi-badge" :data-tone="biStatusTone(invite.status)">{{ t(`biNative.inviteStatus.${invite.status}`) }}</span><p class="bi-caption">{{ t('biNative.expires') }} <time :datetime="invite.expires_at" :title="invite.expires_at">{{ biDate(invite.expires_at) }}</time><span v-if="invite.claimed_by"> · {{ t('biNative.claimedBy') }} {{ invite.claimed_by }}</span></p><p class="bi-caption">{{ invite.capabilities.map(cap => t(`biNative.capabilities.${cap}`)).join('、') || '-' }}</p></div>
            <button v-if="invite.status === 'pending'" class="btn btn-secondary" :disabled="busy || enabled !== true" @click="revoking = invite">{{ t('biNative.revokeInvite') }}</button>
          </div>
        </div>
        <button v-if="invitesMore" class="btn btn-secondary mt-3" :disabled="loading.invites" @click="loadInvites(invitePage + 1)">{{ t('bi.loadMore') }}</button>
      </section>
    </div>
    <BIStepUpDialog :controller="stepUp" />
    <ConfirmDialog :show="!!revoking" :title="t('biNative.revokeInvite')" :message="t('biNative.confirmRevoke')" :confirm-text="t('biNative.revokeInvite')" :loading="busy" danger content-class="bi-access-dialog" @confirm="confirmRevoke" @cancel="revoking = undefined" />
  </section>
</template>
<script setup lang="ts">
import { biDate, biStatusTone, biText } from './display'
import '@/styles/biOperations.css'
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { biAccessPath, biAccessQuery } from './navigation'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BIStepUpDialog from '@/components/bi/BIStepUpDialog.vue'
import { useStepUp, isStepUpCancelled, StepUpCancelledError } from '@/composables/useStepUp'
import { biManagement, isBIEnabled, type BIInvitation, type BIUserAccess } from '@/api/bi'
const props = defineProps<{ organizationId: string; selfManaged?: boolean }>()
const route = useRoute(), router = useRouter()
const { t } = useI18n()
const stepUp = useStepUp()
const enabled = ref<boolean | null>(null), availabilityLoading = ref(false), availabilityError = ref(''), busy = ref(false)
const loading = reactive({ users: false, invites: false }), errors = reactive({ users: '', invites: '' })
const users = ref<BIUserAccess[]>([]), invites = ref<BIInvitation[]>([])
const userPage = ref(1), invitePage = ref(1), usersMore = ref(false), invitesMore = ref(false)
const revoking = ref<BIInvitation>()
let generation = 0, userLoad = 0, inviteLoad = 0, availabilityLoad = 0
const api = () => biManagement(props.organizationId, props.selfManaged)
function errorMessage(cause: unknown) { const code = (cause as { status?: number }).status; return t(code === 409 ? 'biNative.conflict' : code === 423 ? 'biNative.stepUp' : 'biNative.failed') }
async function loadUsers(page = 1) { const g = generation, n = ++userLoad; loading.users = true; errors.users = ''; try { const result = await api().users(page); if (g !== generation || n !== userLoad) return; users.value = page === 1 ? result.items : [...users.value, ...result.items]; userPage.value = page; usersMore.value = result.has_more } catch (cause) { if (g === generation && n === userLoad) errors.users = errorMessage(cause) } finally { if (g === generation && n === userLoad) loading.users = false } }
async function loadInvites(page = 1) { const g = generation, n = ++inviteLoad; loading.invites = true; errors.invites = ''; try { const result = await api().invitations(page); if (g !== generation || n !== inviteLoad) return; invites.value = page === 1 ? result.items : [...invites.value, ...result.items]; invitePage.value = page; invitesMore.value = result.has_more } catch (cause) { if (g === generation && n === inviteLoad) errors.invites = errorMessage(cause) } finally { if (g === generation && n === inviteLoad) loading.invites = false } }
function openInvite() { void router.push({ path: `${biAccessPath(props.organizationId, props.selfManaged)}/invitations/new`, query: biAccessQuery(route.query.from) }) }
function openUser(user: BIUserAccess) { void router.push({ path: `${biAccessPath(props.organizationId, props.selfManaged)}/users/${encodeURIComponent(user.bi_user_id)}/edit`, query: biAccessQuery(route.query.from) }) }
async function secured<T>(g: number, action: () => Promise<T>) { return stepUp.run(() => { if (g !== generation) return Promise.reject(new StepUpCancelledError()); return action() }) }
async function confirmRevoke() { const invite = revoking.value; if (invite) { revoking.value = undefined; await revoke(invite.id) } }
async function revoke(id: string) { if (busy.value) return; busy.value = true; const g = generation, client = api(); try { await secured(g, () => client.revoke(id)); if (g === generation) { await loadInvites() } } catch (cause) { if (g === generation && !isStepUpCancelled(cause)) errors.invites = errorMessage(cause) } finally { if (g === generation) busy.value = false } }
async function checkAvailability() {
  const g = generation, n = ++availabilityLoad
  enabled.value = null; availabilityLoading.value = true; availabilityError.value = ''
  try {
    const value = await isBIEnabled()
    if (g !== generation || n !== availabilityLoad) return
    enabled.value = value
    if (value) { void loadUsers(); void loadInvites() }
  } catch { if (g === generation && n === availabilityLoad) availabilityError.value = t('biNative.availabilityFailed') }
  finally { if (g === generation && n === availabilityLoad) availabilityLoading.value = false }
}
watch(() => [props.organizationId, props.selfManaged], () => { stepUp.onCancel(); generation++; users.value = []; invites.value = []; revoking.value = undefined; busy.value = false; errors.users = ''; errors.invites = ''; void checkAvailability() }, { immediate: true })
onBeforeUnmount(() => { generation++; stepUp.onCancel() })
</script>
