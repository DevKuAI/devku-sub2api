<template>
  <section class="accounting-form-section">
    <div>
      <h2>{{ title }}</h2>
      <p v-if="hint">{{ hint }}</p>
    </div>
    <div class="grid min-w-0 content-start gap-4 sm:grid-cols-2">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{ title: string; hint?: string }>()
</script>
