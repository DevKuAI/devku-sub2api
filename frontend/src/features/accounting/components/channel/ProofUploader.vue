<template>
  <div class="space-y-3" data-testid="channel-proof-uploader">
    <label
      ref="zone"
      class="channel-dropzone py-6"
      :class="{ 'is-active': dragging }"
      tabindex="0"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="dropped"
      @paste="pasted"
    >
      <input type="file" class="sr-only" multiple accept=".jpg,.jpeg,.png,.webp,.pdf" data-testid="channel-proof-input" @change="chosen" />
      <span class="font-medium text-gray-900 dark:text-white">{{ t('accounting.channels.proofs.drop') }}</span>
      <span class="text-xs text-gray-500 dark:text-dark-400">{{ t('accounting.channels.proofs.dropHint') }}</span>
    </label>
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <p v-if="busy" role="status" class="text-sm text-gray-500">{{ t('accounting.channels.proofs.uploading', { done, total }) }}</p>
    <div class="flex gap-3">
      <button type="button" class="btn btn-secondary" :disabled="busy" @click="emit('close')">{{ t('common.close') }}</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import * as api from '../../api'
import { errorMessage } from '../../shared'
import type { ChannelProof } from '../../types'

// Screenshots usually come from the clipboard, so pasting works as well as dropping or choosing files.
const props = defineProps<{ lineId: string }>()
const emit = defineEmits<{ uploaded: [proof: ChannelProof]; close: [] }>()
const { t, te } = useI18n()
const maxBytes = 20 * 1024 * 1024
const zone = ref<HTMLElement | null>(null)
const dragging = ref(false)
const busy = ref(false)
const done = ref(0)
const total = ref(0)
const error = ref('')

function acceptable(file: File) {
  if (!/\.(jpe?g|png|webp|pdf)$/i.test(file.name) && !/^image\/(jpeg|png|webp)$|^application\/pdf$/.test(file.type)) {
    error.value = t('accounting.channels.proofs.unsupported', { name: file.name })
    return false
  }
  if (file.size > maxBytes) {
    error.value = t('accounting.channels.import.tooLarge', { name: file.name })
    return false
  }
  return true
}
// Pasted screenshots arrive as image.png; give them a name that says when they were taken.
function named(file: File) {
  if (/\.(jpe?g|png|webp|pdf)$/i.test(file.name) && file.name !== 'image.png') return file
  const extension = file.type === 'application/pdf' ? 'pdf' : file.type.split('/')[1] || 'png'
  const stamp = new Date().toISOString().replace(/[-:]/g, '').slice(0, 15)
  return new File([file], `screenshot-${stamp}.${extension === 'jpeg' ? 'jpg' : extension}`, { type: file.type })
}
async function upload(list: File[]) {
  error.value = ''
  const files = list.map(named).filter(acceptable)
  if (!files.length) return
  busy.value = true
  done.value = 0
  total.value = files.length
  try {
    for (const file of files) {
      emit('uploaded', await api.uploadChannelProof(props.lineId, file, api.key()))
      done.value++
    }
  } catch (cause) {
    error.value = errorMessage(cause, t, te)
  } finally {
    busy.value = false
  }
}
function chosen(event: Event) {
  const target = event.target as HTMLInputElement
  void upload(Array.from(target.files || []))
  target.value = ''
}
function dropped(event: DragEvent) {
  dragging.value = false
  void upload(Array.from(event.dataTransfer?.files || []))
}
function pasted(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || [])
  if (!files.length) return
  event.preventDefault()
  void upload(files)
}
onMounted(() => zone.value?.focus())
</script>
