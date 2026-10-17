<template>
  <section class="bi-access-region space-y-4" :aria-label="t('biNative.created')">
    <header class="space-y-2">
      <h3 class="bi-heading">{{ t('biNative.created') }}</h3>
      <p v-if="invitation.organization_name" class="font-medium break-words">{{ invitation.organization_name }}</p>
      <p class="bi-caption">{{ t('biNative.sendYourself') }}</p>
      <p class="bi-caption">{{ t('biNative.expires') }} <time :datetime="invitation.expires_at">{{ biDate(invitation.expires_at) }}</time></p>
    </header>
    <p v-if="!active" role="status" class="bi-empty">{{ t('biNative.invitationUnavailable') }}</p>
    <div v-else class="bi-invite-result">
      <div class="min-w-0 space-y-4">
        <p class="bi-caption">{{ t('biNative.scanInviteHint') }}</p>
        <label class="block">
          <span class="input-label">{{ t('biNative.inviteCode') }}</span>
          <input class="input font-mono" :value="invitation.token" readonly />
        </label>
        <p class="bi-caption">{{ t('biNative.manualInviteHint') }}</p>
        <button class="btn btn-secondary" type="button" @click="copyInvitation">{{ t('biNative.copyInviteCode') }}</button>
        <p role="status" class="bi-caption">{{ copyStatus }}</p>
      </div>
      <div class="min-w-0 space-y-3" :aria-busy="loading">
        <div v-if="loading" role="status" class="bi-empty flex min-h-48 items-center justify-center">{{ t('biNative.generatingQRCode') }}</div>
        <template v-else-if="qr">
          <img :src="qr" :alt="t('biNative.inviteQRCode')" class="h-auto w-60 max-w-full rounded-lg border bg-white p-2" @load="imageLoaded = true" @error="imageFailed" />
          <a v-if="imageLoaded" class="btn btn-secondary" :href="qr" :download="`wechat-invitation.${imageExtension}`">{{ t('biNative.saveQRCode') }}</a>
        </template>
        <div v-else class="space-y-3">
          <p role="alert" class="bi-error">{{ qrError }}</p>
          <p v-if="retrySeconds > 0" role="status" class="bi-caption">{{ t('biNative.qrRetryAfter', { seconds: retrySeconds }) }}</p>
          <button v-if="canGenerate" class="btn btn-secondary" type="button" :disabled="retrySeconds > 0" @click="generate">{{ t('biNative.retryQRCode') }}</button>
        </div>
      </div>
    </div>
    <BIStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { biManagement, type BIIssuedInvitation } from '@/api/bi'
import { isStepUpCancelled, StepUpCancelledError, useStepUp } from '@/composables/useStepUp'
import BIStepUpDialog from './BIStepUpDialog.vue'
import { biDate } from './display'

const props = defineProps<{ invitation: BIIssuedInvitation; organizationId: string; selfManaged?: boolean }>()
const emit = defineEmits<{ busyChange: [value: boolean] }>()
const { t } = useI18n()
const stepUp = useStepUp()
const qr = ref(''), qrError = ref(''), copyStatus = ref(''), loading = ref(false), imageLoaded = ref(false)
const imageExtension = ref('png'), now = ref(Date.now()), retryAt = ref(0), unavailable = ref(false)
const active = computed(() => !unavailable.value && props.invitation.status === 'pending' && Date.parse(props.invitation.expires_at) > now.value)
const canGenerate = computed(() => /^[A-Za-z0-9_-]{32}$/.test(props.invitation.scene || ''))
const retrySeconds = computed(() => Math.max(0, Math.ceil((retryAt.value - now.value) / 1000)))
let generation = 0
const timer = setInterval(() => { now.value = Date.now() }, 1000)

watch(loading, value => emit('busyChange', value), { flush: 'sync' })
watch(active, value => {
  if (value) return
  generation++
  stepUp.onCancel()
  loading.value = false
  qr.value = ''
  copyStatus.value = ''
})

async function generate() {
  if (loading.value || !active.value || retrySeconds.value > 0) return
  qr.value = ''; qrError.value = ''; imageLoaded.value = false
  if (!canGenerate.value) { qrError.value = t('biNative.qrCredentialMissing'); return }
  loading.value = true
  const g = generation, invitation = props.invitation, client = biManagement(props.organizationId, props.selfManaged)
  try {
    const result = await stepUp.run(async () => {
      if (g !== generation || !active.value) throw new StepUpCancelledError()
      try { return await client.qrcode(invitation.id, invitation.scene) }
      catch (cause) {
        if (g !== generation || !active.value) throw new StepUpCancelledError()
        throw cause
      }
    })
    if (g !== generation || !active.value) return
    if (!['image/png', 'image/jpeg'].includes(result.mime_type) || !result.image_base64 || result.image_base64.length > 2_800_000 || !/^[A-Za-z0-9+/]+={0,2}$/.test(result.image_base64)) {
      throw new Error('Invalid QR image')
    }
    imageExtension.value = result.mime_type === 'image/jpeg' ? 'jpg' : 'png'
    qr.value = `data:${result.mime_type};base64,${result.image_base64}`
  } catch (cause) {
    if (g !== generation) return
    const error = (cause ?? {}) as { status?: number; retryAfter?: string }
    if (error.status === 404) { unavailable.value = true; return }
    if (error.status === 429) {
      const seconds = Number(error.retryAfter)
      const date = error.retryAfter ? Date.parse(error.retryAfter) : NaN
      now.value = Date.now()
      retryAt.value = Number.isFinite(seconds) && seconds > 0 ? now.value + seconds * 1000 : Number.isFinite(date) && date > now.value ? date : now.value + 60_000
    }
    qrError.value = t(isStepUpCancelled(cause) ? 'biNative.qrVerificationCancelled' : error.status === 403 || error.status === 423 ? 'biNative.stepUp' : 'biNative.qrGenerationFailed')
  } finally {
    if (g === generation) loading.value = false
  }
}

function imageFailed() { qr.value = ''; imageLoaded.value = false; qrError.value = t('biNative.qrGenerationFailed') }
async function copyInvitation() {
  if (!active.value) return
  const g = generation
  try { await navigator.clipboard.writeText(props.invitation.token); if (g === generation) copyStatus.value = t('biNative.copied') }
  catch { if (g === generation) copyStatus.value = t('biNative.copyFailed') }
}

watch([() => props.organizationId, () => props.selfManaged, () => props.invitation.id], () => {
  generation++; stepUp.onCancel(); loading.value = false
  unavailable.value = false; retryAt.value = 0; qr.value = ''; copyStatus.value = ''; now.value = Date.now()
  void generate()
}, { immediate: true })
onBeforeUnmount(() => { generation++; stepUp.onCancel(); clearInterval(timer); emit('busyChange', false) })
</script>
