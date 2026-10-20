<template>
  <header class="accounting-page-header">
    <router-link v-if="back?.to" class="accounting-back" :to="back.to" data-testid="page-back">
      <Icon name="arrowLeft" size="sm" aria-hidden="true" />{{ back.label }}
    </router-link>
    <button v-else-if="back" type="button" class="accounting-back" data-testid="page-back" @click="emit('back')">
      <Icon name="arrowLeft" size="sm" aria-hidden="true" />{{ back.label }}
    </button>
    <div class="accounting-page-heading">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <h1>
            <slot name="title">{{ title }}</slot>
          </h1>
          <slot name="badge" />
        </div>
        <p v-if="subtitle || $slots.subtitle" class="accounting-page-subtitle">
          <slot name="subtitle">{{ subtitle }}</slot>
        </p>
      </div>
      <div v-if="$slots.actions" class="accounting-page-actions">
        <slot name="actions" />
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import { Icon } from '@/components/icons'

// One header for every accounting page: where to go back, what this is, and what can be done here.
// Without back.to the link becomes a button, so forms can run their leave guard first.
defineProps<{ title?: string; subtitle?: string; back?: { to?: RouteLocationRaw; label: string } }>()
const emit = defineEmits<{ back: [] }>()
</script>
