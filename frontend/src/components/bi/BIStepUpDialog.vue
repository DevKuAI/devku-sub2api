<template>
  <BaseDialog :show="controller.visible.value" :title="t('stepUp.title')" width="narrow" :z-index="70" content-class="bi-access-dialog" :close-on-escape="false" :show-close-button="false">
    <TotpStepUpDialog :controller="controller" inline :digit-label="digit => t('biNative.verificationDigit', { digit })" />
  </BaseDialog>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import type { StepUpController } from '@/composables/useStepUp'
import '@/styles/biOperations.css'
defineProps<{ controller: StepUpController }>()
const { t } = useI18n()
</script>
