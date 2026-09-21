<template>
  <div class="space-y-2">
    <p v-if="segment.truncated" class="text-xs text-amber-700 dark:text-amber-400">{{ t('admin.desktop.conversations.truncated') }}</p>
    <pre class="whitespace-pre-wrap break-words rounded-lg bg-white p-4 font-sans text-sm leading-7 text-gray-900 dark:bg-dark-800 dark:text-gray-100 [overflow-wrap:anywhere]">{{ visibleText || t('admin.desktop.conversations.emptyText') }}</pre>
    <button v-if="segment.text.length > previewLength" class="btn btn-secondary text-sm" type="button" :aria-expanded="expanded" @click="expanded = !expanded">
      {{ t(expanded ? 'admin.desktop.conversations.collapse' : 'admin.desktop.conversations.expand') }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DesktopConversationSegment } from '@/api/desktopConversations'

const props = defineProps<{ segment: DesktopConversationSegment }>()
const { t } = useI18n()
const expanded = ref(false)
const previewLength = 2400
const visibleText = computed(() => expanded.value ? props.segment.text : props.segment.text.slice(0, previewLength))
</script>
