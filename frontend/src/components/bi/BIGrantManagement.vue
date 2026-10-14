<template>
  <section class="card detail-panel space-y-5">
    <header class="flex flex-wrap items-center justify-between gap-3"><h2 class="detail-section-title">{{ t('biNative.users') }}</h2><button class="btn btn-primary" :disabled="busy || !enabled" @click="openInvite">{{ t('biNative.createInvite') }}</button></header>
    <p class="detail-caption">{{ t('biNative.identityHint') }}</p>
    <p v-if="!enabled" role="status" class="detail-caption">{{ t('biNative.disabled') }}</p>
    <div v-if="errors.users" role="alert"><p>{{ errors.users }}</p><button class="btn btn-secondary" @click="loadUsers()">{{ t('biNative.retry') }}</button></div>
    <div :aria-busy="loading.users">
      <p v-if="loading.users" role="status">{{ t('common.loading') }}</p>
      <p v-if="!loading.users && !users.length && !errors.users">{{ t('biNative.noUsers') }}</p>
      <div v-for="user in users" :key="user.bi_user_id" class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 py-3 dark:border-dark-700">
        <div class="min-w-0 flex-1"><p class="break-words font-medium">{{ user.organization_alias || user.display_name }} · {{ user.status === 'active' ? t('common.enabled') : t('biNative.revoked') }}<span v-if="user.user_status === 'disabled'"> · {{ t('biNative.identityDisabled') }}</span></p><p class="detail-caption break-words">{{ user.capabilities.map(cap => t(`biNative.capabilities.${cap}`)).join('、') }}</p><p class="detail-caption break-all">{{ t('biNative.identityID') }}: {{ user.bi_user_id }}</p><p class="detail-caption">{{ t('biNative.wholeOrganization') }} · v{{ user.revision }}</p></div>
        <button class="btn btn-secondary" :disabled="busy" @click="openUser(user)">{{ t('common.edit') }}</button>
      </div>
      <button class="btn btn-secondary mt-3" :disabled="loading.users" @click="loadUsers()">{{ t('common.refresh') }}</button><button v-if="usersMore" class="btn btn-secondary mt-3" :disabled="loading.users" @click="loadUsers(userPage + 1)">{{ t('bi.loadMore') }}</button>
    </div>
    <h3 class="detail-section-title">{{ t('biNative.invitations') }}</h3>
    <p class="detail-caption">{{ t('biNative.inviteHint') }}</p>
    <div v-if="errors.invites" role="alert"><p>{{ errors.invites }}</p><button class="btn btn-secondary" @click="loadInvites()">{{ t('biNative.retry') }}</button></div>
    <div :aria-busy="loading.invites"><p v-if="loading.invites" role="status">{{ t('common.loading') }}</p><p v-if="!loading.invites && !invites.length && !errors.invites">{{ t('biNative.noInvites') }}</p>
      <div v-for="invite in invites" :key="invite.id" class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 py-3 dark:border-dark-700"><div class="min-w-0 flex-1"><p class="break-words">{{ invite.recipient_name || invite.note || invite.id }} · {{ t(`biNative.inviteStatus.${invite.status}`) }}</p><p class="detail-caption">{{ t('biNative.expires') }} {{ invite.expires_at }}<span v-if="invite.claimed_by"> · {{ t('biNative.claimedBy') }} {{ invite.claimed_by }}</span></p><p class="detail-caption">{{ invite.capabilities.map(cap => t(`biNative.capabilities.${cap}`)).join('、') }}</p></div><button v-if="invite.status === 'pending'" class="btn btn-secondary" :disabled="busy" @click="revoke(invite.id)">{{ t('biNative.revokeInvite') }}</button></div>
      <button class="btn btn-secondary mt-3" :disabled="loading.invites" @click="loadInvites()">{{ t('common.refresh') }}</button><button v-if="invitesMore" class="btn btn-secondary mt-3" :disabled="loading.invites" @click="loadInvites(invitePage + 1)">{{ t('bi.loadMore') }}</button>
    </div>
    <div v-if="issued" class="space-y-3 rounded border border-primary-300 p-4" role="status"><h3 class="font-medium">{{ t('biNative.created') }}</h3><p class="detail-caption">{{ t('biNative.sendYourself') }}</p><label class="block">{{ t('biNative.inviteCode') }}<input class="input mt-2" :value="issued.token" readonly /></label><label class="block">{{ t('biNative.invitePath') }}<input class="input mt-2" :value="issued.path" readonly /></label><button class="btn btn-secondary" @click="copyInvitation">{{ t('common.copy') }}</button><p>{{ copyStatus }}</p></div>
    <p v-if="issued" class="detail-caption">{{ t('biNative.scanInviteHint') }}</p><img v-if="issuedQR" :src="issuedQR" :alt="t('biNative.inviteQRCode')" class="h-48 w-48 rounded border bg-white p-2" />
    <TotpStepUpDialog :controller="stepUp" />
    <BaseDialog :show="editing" :title="t(selected ? 'biNative.editAccess' : 'biNative.createInvite')" width="wide" @close="close">
      <form id="bi-access-form" class="space-y-4" @submit.prevent="save"><label v-if="!selected" class="block">{{ t('biNative.note') }}<input v-model="note" class="input mt-2" maxlength="100" :disabled="busy" /></label><label class="block">{{ t('biNative.organizationAlias') }}<input v-model="organizationAlias" class="input mt-2" maxlength="80" :disabled="busy" /><span class="detail-caption">{{ t('biNative.organizationAliasHint') }}</span></label><p>{{ t('biNative.wholeOrganization') }}</p><div class="flex flex-wrap gap-2"><button class="btn btn-secondary" type="button" :disabled="busy" @click="caps = ['analytics:read']">{{ t('biNative.presetSummary') }}</button><button class="btn btn-secondary" type="button" :disabled="busy" @click="caps = ['members:read','reports:read']">{{ t('biNative.presetReports') }}</button><button class="btn btn-secondary" type="button" :disabled="busy" @click="caps = [...biCapabilities]">{{ t('biNative.presetAll') }}</button></div><fieldset :disabled="busy" class="space-y-3"><legend class="mb-2 font-medium">{{ t('biNative.contentAccess') }}</legend><label v-for="cap in biCapabilities" :key="cap" class="flex gap-2"><input type="checkbox" :checked="caps.includes(cap)" @change="toggle(cap, ($event.target as HTMLInputElement).checked)" />{{ t(`biNative.capabilities.${cap}`) }}</label></fieldset><p class="detail-caption">{{ t('biNative.dependencyHint') }}</p><label v-if="selected" class="block">{{ t('biNative.accessStatus') }}<select v-model="status" class="input mt-2" :disabled="busy"><option value="active">{{ t('common.enabled') }}</option><option value="revoked">{{ t('biNative.revoked') }}</option></select></label><p v-if="editError" role="alert" class="text-red-600">{{ editError }}</p></form>
      <template #footer><button class="btn btn-secondary" :disabled="busy" @click="close">{{ t('common.cancel') }}</button><button form="bi-access-form" class="btn btn-primary" type="submit" :disabled="busy || !caps.length">{{ t('common.save') }}</button></template>
    </BaseDialog>
  </section>
</template>
<script setup lang="ts">
import QRCode from 'qrcode'
import { onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpCancelled, StepUpCancelledError } from '@/composables/useStepUp'
import { biCapabilities, biManagement, capabilityDependencies, isBIEnabled, type BICapability, type BIInvitation, type BIIssuedInvitation, type BIUserAccess } from '@/api/bi'
const props = defineProps<{ organizationId: string; selfManaged?: boolean }>()
const { t } = useI18n()
const stepUp = useStepUp()
const enabled = ref(false), busy = ref(false), editing = ref(false)
const loading = reactive({ users: false, invites: false }), errors = reactive({ users: '', invites: '' })
const users = ref<BIUserAccess[]>([]), invites = ref<BIInvitation[]>([]), issued = ref<BIIssuedInvitation>()
const userPage = ref(1), invitePage = ref(1), usersMore = ref(false), invitesMore = ref(false)
const selected = ref<BIUserAccess>(), caps = ref<BICapability[]>([]), note = ref(''), status = ref<BIUserAccess['status']>('active'), editError = ref(''), copyStatus = ref('')
const organizationAlias = ref(''), issuedQR = ref('')
watch(issued, async value => { const token = value?.token; issuedQR.value = ''; if (token) { try { const svg = await QRCode.toString(value!.path, { type: 'svg', width: 192, margin: 2 }); if (issued.value?.token === token) issuedQR.value = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg) } catch { if (issued.value?.token === token) copyStatus.value = t('biNative.copyFailed') } } })
let generation = 0, userLoad = 0, inviteLoad = 0
const api = () => biManagement(props.organizationId, props.selfManaged)
function errorMessage(cause: unknown) { const code = (cause as { status?: number }).status; return t(code === 409 ? 'biNative.conflict' : code === 423 ? 'biNative.stepUp' : 'biNative.failed') }
async function loadUsers(page = 1) { const g = generation, n = ++userLoad; loading.users = true; errors.users = ''; try { const result = await api().users(page); if (g !== generation || n !== userLoad) return; users.value = page === 1 ? result.items : [...users.value, ...result.items]; userPage.value = page; usersMore.value = result.has_more } catch (cause) { if (g === generation) errors.users = errorMessage(cause) } finally { if (g === generation && n === userLoad) loading.users = false } }
async function loadInvites(page = 1) { const g = generation, n = ++inviteLoad; loading.invites = true; errors.invites = ''; try { const result = await api().invitations(page); if (g !== generation || n !== inviteLoad) return; invites.value = page === 1 ? result.items : [...invites.value, ...result.items]; invitePage.value = page; invitesMore.value = result.has_more } catch (cause) { if (g === generation) errors.invites = errorMessage(cause) } finally { if (g === generation && n === inviteLoad) loading.invites = false } }
function openInvite() { selected.value = undefined; caps.value = ['analytics:read']; note.value = ''; organizationAlias.value = ''; status.value = 'active'; editError.value = ''; editing.value = true }
function openUser(user: BIUserAccess) { organizationAlias.value = user.organization_alias || ''; selected.value = user; caps.value = [...user.capabilities]; status.value = user.status; editError.value = ''; editing.value = true }
function close() { if (!busy.value) editing.value = false }
function toggle(cap: BICapability, checked: boolean) { if (checked) caps.value = capabilityDependencies([...caps.value, cap]); else { caps.value = caps.value.filter(c => c !== cap); if (cap === 'members:read') caps.value = caps.value.filter(c => c === 'analytics:read'); if (cap === 'reports:read') caps.value = caps.value.filter(c => c !== 'reports:copy') } }
async function secured<T>(g: number, action: () => Promise<T>) { return stepUp.run(() => { if (g !== generation) return Promise.reject(new StepUpCancelledError()); return action() }) }
async function save() { if (busy.value) return; busy.value = true; const g = generation, client = api(), user = selected.value, permissions = [...caps.value], accessStatus = status.value, inviteNote = note.value, alias = organizationAlias.value; try { if (user) await secured(g, () => client.access(user, permissions, accessStatus, alias)); else { const result = await secured(g, () => client.create(permissions, inviteNote, alias)); if (g === generation) issued.value = result } if (g === generation) { editing.value = false; void loadUsers(); void loadInvites() } } catch (cause) { if (g === generation && !isStepUpCancelled(cause)) editError.value = errorMessage(cause) } finally { if (g === generation) busy.value = false } }
async function revoke(id: string) { if (busy.value) return; busy.value = true; const g = generation, client = api(); try { await secured(g, () => client.revoke(id)); if (g === generation) { if (issued.value?.id === id) issued.value = undefined; await loadInvites() } } catch (cause) { if (g === generation && !isStepUpCancelled(cause)) errors.invites = errorMessage(cause) } finally { if (g === generation) busy.value = false } }
async function copyInvitation() { try { await navigator.clipboard.writeText(`${issued.value?.token}\n${issued.value?.path}`); copyStatus.value = t('biNative.copied') } catch { copyStatus.value = t('biNative.copyFailed') } }
watch(() => [props.organizationId, props.selfManaged], async () => { stepUp.onCancel(); const g = ++generation; users.value = []; invites.value = []; issued.value = undefined; editing.value = false; busy.value = false; errors.users = ''; errors.invites = ''; enabled.value = false; try { const value = await isBIEnabled(); if (g !== generation) return; enabled.value = value; if (value) { void loadUsers(); void loadInvites() } } catch (cause) { if (g === generation) errors.users = errorMessage(cause) } }, { immediate: true })
onBeforeUnmount(() => { generation++; stepUp.onCancel() })
</script>
