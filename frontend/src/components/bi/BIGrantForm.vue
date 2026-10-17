<template>
  <section class="card bi-panel bi-access space-y-5">
    <header class="space-y-2"><h2 class="bi-heading">{{ t(editing ? 'biNative.editAccess' : 'biNative.createInvite') }}</h2><p v-if="selected" class="bi-caption">{{ selected.organization_alias || selected.display_name }} · {{ selected.bi_user_id }}</p></header>
    <p v-if="loading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
    <div v-else-if="loadError" role="alert" class="bi-error"><p>{{ loadError }}</p><button class="btn btn-secondary mt-3" @click="load">{{ t('biNative.retry') }}</button></div>
    <p v-else-if="!enabled" role="status" class="bi-caption">{{ t('biNative.disabled') }}</p>
    <form v-else id="bi-access-form" class="space-y-5" @submit.prevent="save">
      <fieldset :disabled="busy || completed" class="space-y-5">
        <label v-if="!editing" class="block"><span class="input-label">{{ t('biNative.note') }}</span><input v-model="form.note" class="input" maxlength="100" /></label>
        <label class="block"><span class="input-label">{{ t('biNative.organizationAlias') }}</span><input v-model="form.alias" class="input" maxlength="80" /><span class="bi-caption">{{ t('biNative.organizationAliasHint') }}</span></label>
        <p class="bi-caption">{{ t('biNative.wholeOrganization') }}</p>
        <div class="flex flex-wrap gap-2"><button class="btn btn-secondary" type="button" @click="form.caps = ['analytics:read']">{{ t('biNative.presetSummary') }}</button><button class="btn btn-secondary" type="button" @click="form.caps = ['members:read', 'reports:read']">{{ t('biNative.presetReports') }}</button><button class="btn btn-secondary" type="button" @click="form.caps = [...biCapabilities]">{{ t('biNative.presetAll') }}</button></div>
        <fieldset class="space-y-3"><legend class="mb-2 font-medium">{{ t('biNative.contentAccess') }}</legend><label v-for="cap in biCapabilities" :key="cap" class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700"><input class="mt-1 shrink-0" type="checkbox" :checked="form.caps.includes(cap)" @change="toggle(cap, ($event.target as HTMLInputElement).checked)" />{{ t(`biNative.capabilities.${cap}`) }}</label></fieldset>
        <p class="bi-caption">{{ t('biNative.dependencyHint') }}</p>
        <label v-if="editing" class="block"><span class="input-label">{{ t('biNative.accessStatus') }}</span><select v-model="form.status" class="input"><option value="active">{{ t('common.enabled') }}</option><option value="revoked">{{ t('biNative.revoked') }}</option></select></label>
      </fieldset>
      <p v-if="error" role="alert" class="bi-error">{{ error }}</p>
      <button v-if="conflict" class="btn btn-secondary" type="button" :disabled="busy" @click="load">{{ t('biNative.reloadAccess') }}</button>
      <footer class="flex flex-wrap items-center gap-3 border-t border-gray-200 pt-5 dark:border-dark-700">
        <button class="btn btn-secondary" type="button" :disabled="busy" @click="cancel">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" type="submit" :disabled="busy || completed || !form.caps.length">{{ t(busy ? 'common.saving' : editing ? 'common.save' : 'biNative.createInvite') }}</button>
        <span v-if="dirty" class="bi-caption" role="status">{{ t('biNative.unsavedChanges') }}</span>
      </footer>
    </form>
    <BIStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { biCapabilities, biManagement, capabilityDependencies, isBIEnabled, type BICapability, type BIIssuedInvitation, type BIUserAccess } from '@/api/bi'
import { isStepUpCancelled, StepUpCancelledError, useStepUp } from '@/composables/useStepUp'
import BIStepUpDialog from './BIStepUpDialog.vue'
import { biAccessPath, biAccessQuery } from './navigation'

const props = defineProps<{ organizationId: string; selfManaged?: boolean }>()
const emit = defineEmits<{ created: [invitation: BIIssuedInvitation] }>()
const route = useRoute(), router = useRouter(), { t } = useI18n(), stepUp = useStepUp()
const userId = computed(() => String(route.params.biUserId || ''))
const editing = computed(() => Boolean(userId.value))
const selected = ref<BIUserAccess>(), loading = ref(true), enabled = ref(false), busy = ref(false), completed = ref(false)
const loadError = ref(''), error = ref(''), conflict = ref(false), baseline = ref('')
const form = reactive({ note: '', alias: '', caps: ['analytics:read'] as BICapability[], status: 'active' as BIUserAccess['status'] })
const snapshot = () => JSON.stringify(form)
const dirty = computed(() => !completed.value && baseline.value !== '' && snapshot() !== baseline.value)
const listLocation = () => ({ path: biAccessPath(props.organizationId, props.selfManaged), query: biAccessQuery(route.query.from) })
let generation = 0

function canLeave() { return !busy.value && (!dirty.value || window.confirm(t('biNative.discardChanges'))) }
onBeforeRouteLeave(canLeave)
onBeforeRouteUpdate((to, from) => to.path === from.path || canLeave())
function beforeUnload(event: BeforeUnloadEvent) { if (dirty.value || busy.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
function cancel() { if (!busy.value) void router.push(listLocation()) }

async function load() {
  if (busy.value || (dirty.value && !window.confirm(t('biNative.discardChanges')))) return
  const g = ++generation, id = userId.value, client = biManagement(props.organizationId, props.selfManaged)
  stepUp.onCancel(); loading.value = true; loadError.value = ''; error.value = ''; conflict.value = false; baseline.value = ''; selected.value = undefined
  try {
    const available = await isBIEnabled()
    if (g !== generation) return
    enabled.value = available
    if (!available) return
    if (id) {
      // The existing API lists access grants. Resolve direct edit links across all pages.
      for (let page = 1; ; page++) {
        const result = await client.users(page)
        if (g !== generation) return
        const user = result.items.find(item => item.bi_user_id === id)
        if (user) { selected.value = user; break }
        if (!result.has_more) { loadError.value = t('biNative.accessNotFound'); return }
      }
    }
    Object.assign(form, { note: '', alias: selected.value?.organization_alias || '', caps: [...(selected.value?.capabilities || ['analytics:read'])], status: selected.value?.status || 'active' })
    completed.value = false; baseline.value = snapshot()
  } catch { if (g === generation) loadError.value = t('biNative.failed') }
  finally { if (g === generation) loading.value = false }
}
function toggle(cap: BICapability, checked: boolean) {
  if (checked) form.caps = capabilityDependencies([...form.caps, cap])
  else {
    form.caps = form.caps.filter(c => c !== cap)
    if (cap === 'members:read') form.caps = form.caps.filter(c => c === 'analytics:read')
    if (cap === 'reports:read') form.caps = form.caps.filter(c => c !== 'reports:copy')
  }
}
async function save() {
  if (busy.value || completed.value || loading.value || loadError.value || !enabled.value || !form.caps.length || (editing.value && !selected.value)) return
  busy.value = true; error.value = ''; conflict.value = false
  const g = generation, client = biManagement(props.organizationId, props.selfManaged), user = selected.value
  const permissions = [...form.caps], note = form.note, alias = form.alias, status = form.status
  try {
    const result = await stepUp.run(async () => {
      if (g !== generation) throw new StepUpCancelledError()
      try { return user ? await client.access(user, permissions, status, alias) : await client.create(permissions, note, alias) }
      catch (cause) { if (g !== generation) throw new StepUpCancelledError(); throw cause }
    })
    if (g !== generation) return
    completed.value = true; baseline.value = snapshot(); busy.value = false
    if (user) await router.push(listLocation())
    else emit('created', result as BIIssuedInvitation)
  } catch (cause) {
    if (g !== generation) return
    if (!isStepUpCancelled(cause)) {
      const status = (cause as { status?: number } | null)?.status
      conflict.value = status === 409
      error.value = t(status === 409 ? 'biNative.conflict' : status === 403 || status === 423 ? 'biNative.stepUp' : 'biNative.failed')
    }
  } finally {
    if (g === generation) {
      busy.value = false
      await nextTick()
      if (!completed.value && g === generation) document.querySelector<HTMLElement>('#bi-access-form input:not(:disabled)')?.focus()
    }
  }
}
watch([() => props.organizationId, () => props.selfManaged, userId], load, { immediate: true })
onBeforeUnmount(() => { generation++; stepUp.onCancel(); window.removeEventListener('beforeunload', beforeUnload) })
</script>
