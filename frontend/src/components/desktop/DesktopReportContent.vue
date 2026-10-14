<template>
  <article class="report-content" @click="onReference" v-html="html" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

const props = withDefaults(defineProps<{ content: string; sourceDates?: string[] }>(), { sourceDates: () => [] })
const emit = defineEmits<{ source: [date: string] }>()
const { t } = useI18n()
const html = computed(() => {
  const linked = props.content.replace(/\[日总结:(\d{4}-\d{2}-\d{2})\]/g, (original, date: string) =>
    (props.sourceDates || []).includes(date) ? `[日总结:${date}](#frozen-source-${date})` : original)
  const renderer = new marked.Renderer()
  renderer.table = function (token) {
    const label = t('admin.desktop.reports.tableScrollHint').replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
    return `<div class="report-content-table" tabindex="0" role="region" aria-label="${label}">${marked.Renderer.prototype.table.call(this, token)}</div>`
  }
  return DOMPurify.sanitize(marked.parse(linked, { async: false, gfm: true, breaks: true, renderer }) as string)
})
function onReference(event: MouseEvent) {
  const link = event.target instanceof Element ? event.target.closest('a[href^="#frozen-source-"]') : null
  const date = link?.getAttribute('href')?.slice('#frozen-source-'.length)
  if (date && (props.sourceDates || []).includes(date)) { event.preventDefault(); emit('source', date) }
}
</script>

<style scoped>
.report-content { @apply min-w-0 text-base text-gray-900 dark:text-gray-100; max-width: 100%; line-height: 1.8; overflow-wrap: anywhere; }
.report-content :deep(h1), .report-content :deep(h2), .report-content :deep(h3), .report-content :deep(h4) { margin: 1.6em 0 0.65em; font-weight: 600; line-height: 1.4; }
.report-content :deep(h1) { font-size: 1.5rem; }
.report-content :deep(h2) { font-size: 1.25rem; }
.report-content :deep(h3) { font-size: 1.125rem; }
.report-content :deep(h4) { font-size: 1rem; }
.report-content :deep(> :first-child) { margin-top: 0; }
.report-content :deep(p), .report-content :deep(ul), .report-content :deep(ol), .report-content :deep(blockquote) { margin: 0.85em 0; max-width: 80ch; }
.report-content :deep(li) { margin-block: 0.3em; }
.report-content :deep(ul) { list-style: disc; padding-inline-start: 1.5rem; }
.report-content :deep(ol) { list-style: decimal; padding-inline-start: 1.5rem; }
.report-content :deep(a) { @apply text-primary-700 underline dark:text-primary-300; text-underline-offset: 3px; }
.report-content :deep(code) { @apply rounded bg-gray-100 px-1 py-0.5 font-mono text-sm dark:bg-dark-700; }
.report-content :deep(pre) { @apply my-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-800; max-width: 100%; white-space: pre-wrap; }
.report-content :deep(pre code) { background: none; padding: 0; }
.report-content :deep(.report-content-table) { max-width: 100%; overflow-x: auto; overscroll-behavior-inline: contain; margin-block: 1rem; }
.report-content :deep(.report-content-table:focus-visible) { outline: 2px solid theme('colors.primary.500'); outline-offset: 2px; }
.report-content :deep(table) { width: 100%; border-collapse: collapse; font-size: 0.875rem; line-height: 1.6; }
.report-content :deep(th), .report-content :deep(td) { @apply border border-gray-200 px-3 py-2 dark:border-dark-700; min-width: 6rem; text-align: start; }
.report-content :deep(th) { @apply bg-gray-50 font-semibold dark:bg-dark-800; }
.report-content :deep(blockquote) { @apply rounded-e-lg border-s-4 border-primary-200 bg-primary-50/40 px-4 py-2 text-gray-600 dark:border-primary-700 dark:bg-primary-900/10 dark:text-dark-300; }
.report-content :deep(blockquote > :first-child) { margin-top: 0; }
.report-content :deep(blockquote > :last-child) { margin-bottom: 0; }
.report-content :deep(hr) { @apply my-6 border-gray-200 dark:border-dark-700; }
.report-content :deep(img) { max-width: 100%; height: auto; }
</style>
