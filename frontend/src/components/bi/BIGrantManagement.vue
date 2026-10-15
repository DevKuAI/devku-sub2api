<template>
  <section class="card bi-panel bi-access space-y-5">
    <header class="bi-toolbar">
      <div class="min-w-0 flex-1 basis-80"><h2 class="bi-heading">{{ t('biNative.accessManagement') }}</h2><p class="bi-caption mt-2">{{ t('biNative.identityHint') }}</p></div>
      <div class="bi-row-actions"><slot name="actions" /><button class="btn btn-secondary" :disabled="availabilityLoading" @click="checkAvailability">{{ t('biNative.refreshAccess') }}</button><button class="btn btn-primary" :disabled="busy || enabled !== true" @click="openInvite">{{ t('biNative.createInvite') }}</button></div>
    </header>
    <p v-if="availabilityLoading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
    <p v-else-if="enabled === false" role="status" class="bi-caption">{{ t('biNative.disabled') }}</p>
    <div v-if="availabilityError" role="alert" class="bi-error"><p>{{ availabilityError }}</p><button class="btn btn-secondary mt-3" @click="checkAvailability">{{ t('biNative.retry') }}</button></div>
    <section v-if="issued" class="bi-access-region space-y-4" :aria-label="t('biNative.created')">
      <h3 class="bi-heading">{{ t('biNative.created') }}</h3>
      <p class="bi-caption">{{ t('biNative.sendYourself') }}</p>
      <div class="bi-invite-result">
        <div class="min-w-0 space-y-4">
          <label class="block"><span class="input-label">{{ t('biNative.inviteCode') }}</span><input class="input" :value="issued.token" readonly /></label>
          <label class="block"><span class="input-label">{{ t('biNative.invitePath') }}</span><input class="input" :value="issued.path" readonly /></label>
          <button class="btn btn-secondary" @click="copyInvitation">{{ t('common.copy') }}</button>
          <p role="status" class="bi-caption">{{ copyStatus }}</p>
        </div>
        <div class="space-y-3"><img v-if="issuedQR" :src="issuedQR" :alt="t('biNative.inviteQRCode')" class="h-48 w-48 max-w-full rounded-lg border bg-white p-2" /><p class="bi-caption">{{ t('biNative.scanInviteHint') }}</p></div>
      </div>
    </section>
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
    <BaseDialog :show="editing" :title="t(selected ? 'biNative.editAccess' : 'biNative.createInvite')" :close-label="t('common.close')" :close-on-escape="!busy" :show-close-button="!busy" :z-index="60" content-class="bi-access-dialog" width="wide" @close="close">
      <form id="bi-access-form" class="space-y-4" @submit.prevent="save"><label v-if="!selected" class="block">{{ t('biNative.note') }}<input v-model="note" class="input mt-2" maxlength="100" :disabled="busy" /></label><label class="block">{{ t('biNative.organizationAlias') }}<input v-model="organizationAlias" class="input mt-2" maxlength="80" :disabled="busy" /><span class="bi-caption">{{ t('biNative.organizationAliasHint') }}</span></label><p>{{ t('biNative.wholeOrganization') }}</p><div class="flex flex-wrap gap-2"><button class="btn btn-secondary" type="button" :disabled="busy" @click="caps = ['analytics:read']">{{ t('biNative.presetSummary') }}</button><button class="btn btn-secondary" type="button" :disabled="busy" @click="caps = ['members:read','reports:read']">{{ t('biNative.presetReports') }}</button><button class="btn btn-secondary" type="button" :disabled="busy" @click="caps = [...biCapabilities]">{{ t('biNative.presetAll') }}</button></div><fieldset :disabled="busy" class="space-y-3"><legend class="mb-2 font-medium">{{ t('biNative.contentAccess') }}</legend><label v-for="cap in biCapabilities" :key="cap" class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700"><input class="mt-1 shrink-0" type="checkbox" :checked="caps.includes(cap)" @change="toggle(cap, ($event.target as HTMLInputElement).checked)" />{{ t(`biNative.capabilities.${cap}`) }}</label></fieldset><p class="bi-caption">{{ t('biNative.dependencyHint') }}</p><label v-if="selected" class="block">{{ t('biNative.accessStatus') }}<select v-model="status" class="input mt-2" :disabled="busy"><option value="active">{{ t('common.enabled') }}</option><option value="revoked">{{ t('biNative.revoked') }}</option></select></label><p v-if="editError" role="alert" class="text-red-600">{{ editError }}</p></form>
      <template #footer><button class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.cancel') }}</button><button form="bi-access-form" class="btn btn-primary" type="submit" :disabled="busy || !caps.length">{{ t('common.save') }}</button></template>
    </BaseDialog>
  </section>
</template>
<script setup lang="ts">
import QRCode from 'qrcode'
import { biDate, biStatusTone, biText } from './display'
import '@/styles/biOperations.css'
import { nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import BIStepUpDialog from '@/components/bi/BIStepUpDialog.vue'
import { useStepUp, isStepUpCancelled, StepUpCancelledError } from '@/composables/useStepUp'
import { biCapabilities, biManagement, capabilityDependencies, isBIEnabled, type BICapability, type BIInvitation, type BIIssuedInvitation, type BIUserAccess } from '@/api/bi'
const props = defineProps<{ organizationId: string; selfManaged?: boolean }>()
const emit = defineEmits<{ busyChange: [value: boolean] }>()
const { t } = useI18n()
const stepUp = useStepUp()
const enabled = ref<boolean | null>(null), availabilityLoading = ref(false), availabilityError = ref(''), busy = ref(false), editing = ref(false)
const loading = reactive({ users: false, invites: false }), errors = reactive({ users: '', invites: '' })
const users = ref<BIUserAccess[]>([]), invites = ref<BIInvitation[]>([]), issued = ref<BIIssuedInvitation>()
const userPage = ref(1), invitePage = ref(1), usersMore = ref(false), invitesMore = ref(false)
const selected = ref<BIUserAccess>(), caps = ref<BICapability[]>([]), note = ref(''), status = ref<BIUserAccess['status']>('active'), editError = ref(''), copyStatus = ref('')
const organizationAlias = ref(''), issuedQR = ref(''), revoking = ref<BIInvitation>()
watch(busy, value => emit('busyChange', value), { flush: 'sync' })
watch(busy, async (value, previous) => {
  if (!previous || value || !editing.value) return
  const g = generation
  await nextTick()
  if (g === generation && editing.value) document.querySelector<HTMLElement>('#bi-access-form input:not(:disabled)')?.focus()
})
watch(issued, async value => { const token = value?.token; issuedQR.value = ''; if (token) { try { const svg = await QRCode.toString(value!.path, { type: 'svg', width: 192, margin: 2 }); if (issued.value?.token === token) issuedQR.value = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg) } catch { if (issued.value?.token === token) copyStatus.value = t('biNative.copyFailed') } } })
let generation = 0, userLoad = 0, inviteLoad = 0, availabilityLoad = 0
const api = () => biManagement(props.organizationId, props.selfManaged)
function errorMessage(cause: unknown) { const code = (cause as { status?: number }).status; return t(code === 409 ? 'biNative.conflict' : code === 423 ? 'biNative.stepUp' : 'biNative.failed') }
async function loadUsers(page = 1) { const g = generation, n = ++userLoad; loading.users = true; errors.users = ''; try { const result = await api().users(page); if (g !== generation || n !== userLoad) return; users.value = page === 1 ? result.items : [...users.value, ...result.items]; userPage.value = page; usersMore.value = result.has_more } catch (cause) { if (g === generation && n === userLoad) errors.users = errorMessage(cause) } finally { if (g === generation && n === userLoad) loading.users = false } }
async function loadInvites(page = 1) { const g = generation, n = ++inviteLoad; loading.invites = true; errors.invites = ''; try { const result = await api().invitations(page); if (g !== generation || n !== inviteLoad) return; invites.value = page === 1 ? result.items : [...invites.value, ...result.items]; invitePage.value = page; invitesMore.value = result.has_more } catch (cause) { if (g === generation && n === inviteLoad) errors.invites = errorMessage(cause) } finally { if (g === generation && n === inviteLoad) loading.invites = false } }
function openInvite() { copyStatus.value = ''; selected.value = undefined; caps.value = ['analytics:read']; note.value = ''; organizationAlias.value = ''; status.value = 'active'; editError.value = ''; editing.value = true }
function openUser(user: BIUserAccess) { organizationAlias.value = user.organization_alias || ''; selected.value = user; caps.value = [...user.capabilities]; status.value = user.status; editError.value = ''; editing.value = true }
function close() { if (!busy.value) editing.value = false }
function toggle(cap: BICapability, checked: boolean) { if (checked) caps.value = capabilityDependencies([...caps.value, cap]); else { caps.value = caps.value.filter(c => c !== cap); if (cap === 'members:read') caps.value = caps.value.filter(c => c === 'analytics:read'); if (cap === 'reports:read') caps.value = caps.value.filter(c => c !== 'reports:copy') } }
async function secured<T>(g: number, action: () => Promise<T>) { return stepUp.run(() => { if (g !== generation) return Promise.reject(new StepUpCancelledError()); return action() }) }
async function save() { if (busy.value) return; busy.value = true; editError.value = ''; const g = generation, client = api(), user = selected.value, permissions = [...caps.value], accessStatus = status.value, inviteNote = note.value, alias = organizationAlias.value; try { if (user) await secured(g, () => client.access(user, permissions, accessStatus, alias)); else { const result = await secured(g, () => client.create(permissions, inviteNote, alias)); if (g === generation) issued.value = result } if (g === generation) { editing.value = false; void loadUsers(); void loadInvites() } } catch (cause) { if (g === generation && !isStepUpCancelled(cause)) editError.value = errorMessage(cause) } finally { if (g === generation) busy.value = false } }
async function confirmRevoke() { const invite = revoking.value; if (invite) { revoking.value = undefined; await revoke(invite.id) } }
async function revoke(id: string) { if (busy.value) return; busy.value = true; const g = generation, client = api(); try { await secured(g, () => client.revoke(id)); if (g === generation) { if (issued.value?.id === id) issued.value = undefined; await loadInvites() } } catch (cause) { if (g === generation && !isStepUpCancelled(cause)) errors.invites = errorMessage(cause) } finally { if (g === generation) busy.value = false } }
async function copyInvitation() { try { await navigator.clipboard.writeText(`${issued.value?.token}\n${issued.value?.path}`); copyStatus.value = t('biNative.copied') } catch { copyStatus.value = t('biNative.copyFailed') } }
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
watch(() => [props.organizationId, props.selfManaged], () => { stepUp.onCancel(); generation++; users.value = []; invites.value = []; issued.value = undefined; revoking.value = undefined; copyStatus.value = ''; editing.value = false; busy.value = false; errors.users = ''; errors.invites = ''; void checkAvailability() }, { immediate: true })
onBeforeUnmount(() => { generation++; stepUp.onCancel() })
</script>
