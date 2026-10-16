<template>
  <section class="space-y-4" data-testid="accounting-audit">
    <p v-if="error" role="alert" class="accounting-error">{{ error }}</p>
    <div class="card overflow-x-auto">
      <table class="accounting-table">
        <thead>
          <tr>
            <th>{{ t('accounting.audit.time') }}</th>
            <th>{{ t('accounting.audit.action') }}</th>
            <th>{{ t('accounting.audit.actor') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in rows" :key="row.id" :title="row.request_id">
            <td class="whitespace-nowrap">{{ formatDateTime(row.created_at) }}</td>
            <td>
              <router-link v-if="isRecordKind(row.object_kind)" class="accounting-link" :to="recordRoute(organizationId, row.object_kind, row.object_id)">
                {{ t(`accounting.kinds.${row.object_kind}`) }}
              </router-link>
              <span v-else>{{ te(`accounting.kinds.${row.object_kind}`) ? t(`accounting.kinds.${row.object_kind}`) : row.object_kind }}</span>
              · {{ te(`accounting.activity.${row.action}`) ? t(`accounting.activity.${row.action}`) : row.action }}
            </td>
            <td>#{{ row.actor_id }}</td>
          </tr>
        </tbody>
      </table>
      <p v-if="!rows.length && !loading" class="p-6 text-center text-sm text-gray-500">{{ t('accounting.audit.empty') }}</p>
    </div>
    <Pagination v-if="total > pageSize" v-model:page="page" :total="total" :page-size="pageSize" :show-page-size-selector="false" />
  </section>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import * as api from '../api'
import { isRecordKind } from '../schema'
import { errorMessage, formatDateTime, recordRoute } from '../shared'
import type { AuditRow } from '../types'

const props = defineProps<{ organizationId: string }>()
const { t, te } = useI18n()
const pageSize = 30
const page = ref(1)
const total = ref(0)
const rows = ref<AuditRow[]>([])
const loading = ref(true)
const error = ref('')

watch(
  page,
  async () => {
    loading.value = true
    try {
      const result = await api.auditLogs({ organization_id: props.organizationId, page: page.value, page_size: pageSize })
      rows.value = result.items
      total.value = result.total
    } catch (cause) {
      error.value = errorMessage(cause, t, te)
    } finally {
      loading.value = false
    }
  },
  { immediate: true },
)
</script>
