<template>
  <div class="space-y-2">
    <p v-if="segment.truncated" class="text-xs text-amber-700 dark:text-amber-400">{{ t('admin.desktop.conversations.truncated') }}</p>
    <div class="conversation-markdown rounded-lg bg-white p-4 dark:bg-dark-800" v-html="html" />
    <button v-if="segment.text.length > previewLength" class="btn btn-secondary text-sm" type="button" :aria-expanded="expanded" @click="expanded = !expanded">
      {{ t(expanded ? 'admin.desktop.conversations.collapse' : 'admin.desktop.conversations.expand') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import type { DesktopConversationSegment } from '@/api/desktopConversations'

const props = defineProps<{ segment: DesktopConversationSegment }>()
const { t } = useI18n()
const expanded = ref(false)
const previewLength = 2400
const visibleText = computed(() => expanded.value ? props.segment.text : props.segment.text.slice(0, previewLength))
const escapeHtml = (text: string) => text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
const html = computed(() => {
  const renderer = new marked.Renderer()
  renderer.html = ({ text }) => escapeHtml(text)
  renderer.table = function (token) {
    const label = escapeHtml(t('admin.desktop.detail.tableScrollHint'))
    return `<div class="conversation-markdown-table" tabindex="0" role="region" aria-label="${label}">${marked.Renderer.prototype.table.call(this, token)}</div>`
  }
  return DOMPurify.sanitize(marked.parse(visibleText.value || t('admin.desktop.conversations.emptyText'), {
    async: false, gfm: true, breaks: true, renderer,
  }), { USE_PROFILES: { html: true } })
})
</script>

<style scoped>
.conversation-markdown { @apply min-w-0 text-base leading-relaxed text-gray-900 dark:text-gray-100; overflow-wrap: anywhere; }
.conversation-markdown :deep(h1), .conversation-markdown :deep(h2), .conversation-markdown :deep(h3), .conversation-markdown :deep(h4), .conversation-markdown :deep(h5), .conversation-markdown :deep(h6) { margin: 1.4em 0 0.6em; font-weight: 600; line-height: 1.4; }
.conversation-markdown :deep(h1) { font-size: 1.5rem; }
.conversation-markdown :deep(h2) { font-size: 1.25rem; }
.conversation-markdown :deep(h3) { font-size: 1.125rem; }
.conversation-markdown :deep(p), .conversation-markdown :deep(ul), .conversation-markdown :deep(ol), .conversation-markdown :deep(blockquote) { margin: 0.85em 0; }
.conversation-markdown :deep(> :first-child) { margin-top: 0; }
.conversation-markdown :deep(> :last-child) { margin-bottom: 0; }
.conversation-markdown :deep(ul) { list-style: disc; padding-inline-start: 1.5rem; }
.conversation-markdown :deep(ol) { list-style: decimal; padding-inline-start: 1.5rem; }
.conversation-markdown :deep(li) { margin-block: 0.3em; }
.conversation-markdown :deep(a) { @apply text-primary-700 underline dark:text-primary-300; text-underline-offset: 3px; }
.conversation-markdown :deep(code) { @apply rounded bg-gray-100 px-1 py-0.5 font-mono text-sm dark:bg-dark-700; }
.conversation-markdown :deep(pre) { @apply my-4 overflow-x-auto rounded-lg bg-gray-50 p-4 dark:bg-dark-900; white-space: pre; overflow-wrap: normal; }
.conversation-markdown :deep(pre code) { @apply bg-transparent p-0 dark:bg-transparent; }
.conversation-markdown :deep(.conversation-markdown-table) { max-width: 100%; overflow-x: auto; overscroll-behavior-inline: contain; margin-block: 1rem; }
.conversation-markdown :deep(.conversation-markdown-table:focus-visible) { outline: 2px solid theme('colors.primary.500'); outline-offset: 2px; }
.conversation-markdown :deep(table) { width: 100%; border-collapse: collapse; font-size: 0.875rem; }
.conversation-markdown :deep(th), .conversation-markdown :deep(td) { @apply border border-gray-200 px-3 py-2 text-left dark:border-dark-700; min-width: 6rem; }
.conversation-markdown :deep(th) { @apply bg-gray-50 font-semibold dark:bg-dark-900; }
.conversation-markdown :deep(blockquote) { @apply border-l-4 border-primary-200 pl-4 text-gray-600 dark:border-primary-700 dark:text-dark-300; }
.conversation-markdown :deep(hr) { @apply my-6 border-gray-200 dark:border-dark-700; }
.conversation-markdown :deep(img) { max-width: 100%; height: auto; }
</style>
