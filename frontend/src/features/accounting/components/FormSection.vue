<template>
  <section class="grid gap-5 py-7 md:grid-cols-[200px_minmax(0,1fr)]">
    <div>
      <h2 class="text-base font-semibold text-gray-950 dark:text-white">{{ title }}</h2>
      <p v-if="hint" class="mt-2 text-sm leading-6 text-gray-500 dark:text-dark-300">{{ hint }}</p>
    </div>
    <div class="grid min-w-0 content-start gap-4 sm:grid-cols-2">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{ title: string; hint?: string }>()
</script>
