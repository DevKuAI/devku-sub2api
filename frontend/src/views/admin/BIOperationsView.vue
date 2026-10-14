<template>
  <AppLayout>
    <div class="bi-operations space-y-6">
      <header class="space-y-2"><h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.bi.title') }}</h1><p class="bi-caption">{{ t('biNative.metadataOnly') }}</p></header>
      <nav class="card bi-tabs-shell" role="tablist" aria-label="BI">
        <div class="bi-tabs"><button v-for="(item, index) in tabs" :id="`bi-tab-${item}`" :key="item" class="bi-tab" type="button" role="tab" :aria-controls="`bi-panel-${item}`" :aria-selected="tab === item" :tabindex="tab === item ? 0 : -1" @click="tab = item" @keydown="tabKey($event, index)">{{ t(`biNative.${item}`) }}</button></div>
      </nav>
      <section v-if="tab === 'overview'" id="bi-panel-overview" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-overview" class="space-y-5" :aria-busy="overviewLoading">
        <div class="card bi-panel">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t('biNative.serviceStatus') }}</h2><div class="bi-row-actions"><router-link to="/admin/settings" class="btn btn-secondary">{{ t('biNative.configure') }}</router-link><button class="btn btn-secondary" :disabled="overviewLoading" @click="loadOverview">{{ t('common.refresh') }}</button></div></header>
          <p v-if="overviewLoading" role="status" class="bi-caption">{{ t('common.loading') }}</p>
          <p v-if="overviewError" role="alert" class="bi-error">{{ overviewError }}</p>
          <dl v-if="overview" class="bi-states">
            <div class="bi-metric"><dt class="bi-caption">{{ t('biNative.biService') }}</dt><dd><span class="bi-badge" :data-tone="overview.bi_enabled ? 'success' : 'neutral'">{{ t(overview.bi_enabled ? 'biNative.enabled' : 'biNative.notEnabled') }}</span></dd></div>
            <div class="bi-metric"><dt class="bi-caption">{{ t('biNative.desktopService') }}</dt><dd><span class="bi-badge" :data-tone="overview.desktop_enabled ? 'success' : 'neutral'">{{ t(overview.desktop_enabled ? 'common.enabled' : 'biNative.notEnabled') }}</span></dd></div>
            <div class="bi-metric"><dt class="bi-caption">{{ t('biNative.privacyStatus') }}</dt><dd><span class="bi-badge" :data-tone="overview.privacy_notice_published ? 'success' : 'warning'">{{ t(overview.privacy_notice_published ? 'biNative.privacyPublished' : 'biNative.privacyMissing') }}</span></dd></div>
          </dl>
          <p v-if="overview && !overview.bi_enabled" class="bi-caption mt-4">{{ t('biNative.disabled') }}</p>
        </div>
        <div v-if="overview" class="card bi-panel"><h2 class="bi-heading mb-4">{{ t('biNative.runtimeCounts') }}</h2><dl class="bi-metrics"><div v-for="name in countKeys" :key="name" class="bi-metric"><dt class="bi-caption">{{ t(`biNative.counts.${name}`) }}</dt><dd>{{ biNumber(overview.counts?.[name] ?? 0) }}</dd></div></dl></div>
        <div v-if="overview?.config" class="card bi-panel"><h2 class="bi-heading">{{ t('biNative.configurationStatus') }}</h2><p class="bi-caption mt-2 mb-4">{{ t('biNative.configurationHint') }}</p><dl class="bi-config"><div v-for="field in configKeys" :key="field"><dt class="bi-caption">{{ t(`biNative.config.${field}`) }}</dt><dd><span v-if="typeof overview.config[field] === 'boolean'" class="bi-badge" :data-tone="overview.config[field] ? 'success' : 'warning'">{{ t(overview.config[field] ? 'admin.desktop.configured' : 'admin.desktop.notConfigured') }}</span><bdi v-else>{{ overview.config[field] || '—' }}</bdi></dd></div></dl></div>
      </section>
      <section v-else-if="tab === 'onboarding'" id="bi-panel-onboarding" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-onboarding" class="space-y-5">
        <div class="card bi-panel">
          <header class="bi-toolbar"><div><h2 class="bi-heading">{{ t('biNative.onboarding') }}</h2><p class="bi-caption mt-2">{{ t('biNative.onboardingHint') }}</p></div><button class="btn btn-secondary" :disabled="regions.organizations.loading" @click="load('organizations')">{{ t('common.refresh') }}</button></header>
          <label class="block mb-4"><span class="input-label">{{ t('biNative.searchOrganizations') }}</span><input v-model="search" class="input w-full sm:max-w-md" type="search" :placeholder="t('biNative.searchOrganizations')" /></label>
          <OperationsTable :rows="regions.organizations.rows" :columns="organizationColumns" :loading="regions.organizations.loading" :error="regions.organizations.error" @retry="load('organizations')"><template #actions="{ row }"><router-link class="btn btn-secondary" :to="organizationLink(String(row.id))">{{ t('biNative.openOrganization') }}</router-link><button class="btn btn-secondary" @click="selectedOrganization = String(row.id)">{{ t('biNative.invitations') }}</button></template></OperationsTable>
          <More path="organizations" />
        </div>
        <BIGrantManagement v-if="selectedOrganization" :key="selectedOrganization" :organization-id="selectedOrganization" />
      </section>
      <section v-else-if="tab === 'identities'" id="bi-panel-identities" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-identities" class="space-y-5">
        <div class="card bi-panel">
          <header class="bi-toolbar"><div><h2 class="bi-heading">{{ t('biNative.invitations') }}</h2><p class="bi-caption mt-2">{{ t('biNative.selectOrganizationHint') }}</p></div><button class="btn btn-secondary" :disabled="regions.organizations.loading" @click="load('organizations')">{{ t('common.refresh') }}</button></header>
          <div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('biNative.searchOrganizations') }}</span><input v-model="search" class="input" type="search" /></label><label><span class="input-label">{{ t('biNative.selectOrganization') }}</span><select v-model="selectedOrganization" class="input"><option value="">{{ t('biNative.selectOrganization') }}</option><option v-for="row in regions.organizations.rows" :key="String(row.id)" :value="String(row.id)">{{ row.name }}</option></select></label></div>
          <p v-if="regions.organizations.error" role="alert" class="bi-error mt-4">{{ regions.organizations.error }}</p><More path="organizations" />
        </div>
        <BIGrantManagement v-if="selectedOrganization" :key="selectedOrganization" :organization-id="selectedOrganization" />
        <div v-for="region in identityRegions" :key="region.path" class="card bi-panel">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t(`biNative.${region.title}`) }}</h2><button class="btn btn-secondary" :disabled="regions[region.path].loading" @click="load(region.path)">{{ t('common.refresh') }}</button></header>
          <OperationsTable :rows="regions[region.path].rows" :columns="region.columns" :loading="regions[region.path].loading" :error="regions[region.path].error" @retry="load(region.path)"><template v-if="region.path === 'identities/users'" #actions="{ row }"><button class="btn btn-secondary" :disabled="busy" @click="setStatus(String(row.id), row.status === 'active' ? 'disabled' : 'active')">{{ t(row.status === 'active' ? 'biNative.disableUser' : 'biNative.enableUser') }}</button></template></OperationsTable><More :path="region.path" />
        </div>
      </section>
      <section v-else-if="tab === 'tasks'" id="bi-panel-tasks" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-tasks" class="space-y-5">
        <div class="card bi-panel"><label class="block w-full sm:max-w-xs"><span class="input-label">{{ t('biNative.period') }}</span><select v-model="kind" class="input"><option value="">{{ t('common.all') }}</option><option v-for="k in ['day','week','month']" :key="k" :value="k">{{ t(`biNative.${k}`) }}</option></select></label></div>
        <div v-for="region in taskRegions" :key="region.path" class="card bi-panel">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t(`biNative.${region.title}`) }}</h2><button class="btn btn-secondary" :disabled="regions[region.path].loading" @click="load(region.path)">{{ t('common.refresh') }}</button></header>
          <OperationsTable :rows="regions[region.path].rows" :columns="region.columns" :loading="regions[region.path].loading" :error="regions[region.path].error" @retry="load(region.path)"><template v-if="region.path === 'report-tasks'" #actions="{ row }"><router-link class="btn btn-secondary" :to="reportLink(row)">{{ t('biNative.openOrganization') }}</router-link></template></OperationsTable><More :path="region.path" />
        </div>
      </section>
      <section v-else id="bi-panel-audit" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-audit" class="space-y-5">
        <div class="card bi-panel"><header class="bi-toolbar"><h2 class="bi-heading">{{ t('biNative.auditLog') }}</h2><button class="btn btn-secondary" :disabled="regions.audit.loading" @click="load('audit')">{{ t('common.refresh') }}</button></header><OperationsTable :rows="regions.audit.rows" :columns="['actor_type','actor_id','organization_id','action','target_id','request_id','created_at']" :loading="regions.audit.loading" :error="regions.audit.error" @retry="load('audit')" /><More path="audit" /></div>
        <form class="card bi-panel space-y-4" @submit.prevent="saveRetention">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t('biNative.retention') }}</h2><button class="btn btn-secondary" type="button" @click="loadRetention">{{ t('common.refresh') }}</button></header><p class="bi-caption">{{ t('biNative.scopeCleanup') }}</p><p v-if="retentionError" role="alert" class="bi-error">{{ retentionError }}</p>
          <template v-if="policy"><div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('biNative.auditDays') }}</span><input v-model.number="policy.audit_days" class="input" type="number" min="1" max="3650" required /></label><label><span class="input-label">{{ t('biNative.ephemeralDays') }}</span><input v-model.number="policy.ephemeral_days" class="input" type="number" min="1" max="365" required /></label></div><div class="bi-row-actions"><button class="btn btn-primary" :disabled="busy">{{ t('common.save') }}</button><button class="btn btn-secondary" type="button" :disabled="busy" @click="cleanup">{{ t('biNative.cleanup') }}</button></div></template>
        </form>
      </section>
      <TotpStepUpDialog :controller="stepUp" />
      <p v-if="actionError" role="alert" class="bi-error">{{ actionError }}</p>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { defineComponent, h, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpCancelled } from '@/composables/useStepUp'
import BIGrantManagement from '@/components/bi/BIGrantManagement.vue'
import OperationsTable from '@/components/bi/OperationsTable.vue'
import { biNumber } from '@/components/bi/display'
import '@/styles/biOperations.css'
import api, { type BIOverview, type BIOperationRow, type BIRetentionPolicy } from '@/api/admin/bi'
const { t } = useI18n(), tabs = ['overview','onboarding','identities','tasks','audit'] as const
const countKeys = ['access', 'users', 'sessions', 'invitations', 'daily_running', 'period_running'] as const
const configKeys = ['appid', 'app_secret_configured', 'jwt_secret_configured', 'identity_secret_configured', 'min_client_version'] as const
const organizationColumns = ['name','status','reporting_enabled','summary_enabled','weekly_summary_enabled','monthly_summary_enabled','analysis_model','users','last_received_at']
const identityRegions = [
  { path: 'identities/users', title: 'users', columns: ['display_name','status','authorization_version','created_at','last_seen_at'] },
  { path: 'identities/sessions', title: 'sessions', columns: ['bi_user_id','status','client_version','last_seen_at','expires_at'] },
]
const taskRegions = [
  { path: 'report-tasks', title: 'tasks', columns: ['organization_name','kind','period_start','period_end','status','revision','available_sources','missing_sources','model','total_tokens','actual_cost','reason'] },
  { path: 'report-executions', title: 'executions', columns: ['organization_id','kind','period_start','revision','model','status','round','chunk','attempt','total_tokens','actual_cost','error','started_at'] },
]
function tabKey(event: KeyboardEvent, index: number) {
  let next = index
  if (event.key === 'ArrowRight') next = (index + 1) % tabs.length
  else if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length
  else if (event.key === 'Home') next = 0
  else if (event.key === 'End') next = tabs.length - 1
  else return
  event.preventDefault()
  tab.value = tabs[next]
  void nextTick(() => { const button = document.getElementById(`bi-tab-${tab.value}`); button?.focus({ preventScroll: true }); button?.scrollIntoView({ block: 'nearest', inline: 'nearest' }) })
}
const tab = ref<typeof tabs[number]>('overview'), overview = ref<BIOverview>(), overviewLoading = ref(false), overviewError = ref(''), search = ref(''), kind = ref(''), selectedOrganization = ref(''), busy = ref(false), actionError = ref(''), policy = ref<BIRetentionPolicy>(), retentionError = ref('')
const stepUp = useStepUp()
const paths = ['organizations','identities/users','identities/sessions','report-tasks','report-executions','audit']
const regions = reactive<Record<string, { rows: BIOperationRow[]; page: number; more: boolean; loading: boolean; error: string; generation: number }>>(Object.fromEntries(paths.map(path => [path, { rows: [], page: 1, more: false, loading: false, error: '', generation: 0 }])))
let alive = true, searchTimer: ReturnType<typeof setTimeout> | undefined
const causeMessage = (cause: unknown) => t((cause as { status?: number }).status === 423 ? 'biNative.stepUp' : 'biNative.failed')
const organizationLink = (id: string, target = 'grants') => ({ path: `/admin/desktop/organizations/${encodeURIComponent(id)}`, query: { tab: target } })
const reportLink = (row: BIOperationRow) => ({path:`/admin/desktop/organizations/${encodeURIComponent(String(row.organization_id))}`,query:{tab:'reports',report_kind:String(row.kind),period_start:String(row.period_start),report_id:String(row.report_id??''),report_date:String(row.period_start)}})
async function loadOverview() { overviewLoading.value = true; overviewError.value = ''; try { const value = await api.getOverview(); if (alive) overview.value = value } catch (cause) { if (alive) overviewError.value = causeMessage(cause) } finally { if (alive) overviewLoading.value = false } }
async function load(path: string, page = 1) { const region = regions[path], g = ++region.generation; region.loading = true; region.error = ''; const params: Record<string,string> = {}; if (path === 'organizations') params.search = search.value; if (path.startsWith('report-')) params.kind = kind.value; try { const value = await api.list(path,page,params); if (!alive || g !== region.generation) return; region.rows = page === 1 ? value.items : [...region.rows,...value.items]; region.page = page; region.more = value.has_more } catch (cause) { if (alive && g === region.generation) region.error = causeMessage(cause) } finally { if (alive && g === region.generation) region.loading = false } }
const More = defineComponent({ props: { path: { type: String, required: true } }, setup(props) { return () => regions[props.path].more ? h('div', { class: 'bi-footer' }, [h('button', { class: 'btn btn-secondary', disabled: regions[props.path].loading, onClick: () => load(props.path, regions[props.path].page + 1) }, t('bi.loadMore'))]) : null } })
async function setStatus(id: string, status: string) { if (!window.confirm(t(status === 'disabled' ? 'biNative.disableUser' : 'biNative.enableUser'))) return; busy.value = true; actionError.value = ''; try { await stepUp.run(() => api.userStatus(id,status)); void load('identities/users'); void load('identities/sessions'); void loadOverview() } catch (cause) { if (!isStepUpCancelled(cause)) actionError.value = causeMessage(cause) } finally { busy.value = false } }
async function loadRetention() { retentionError.value = ''; try { const value = await api.retention(); if (alive) policy.value = value } catch (cause) { if (alive) if (!isStepUpCancelled(cause)) retentionError.value = causeMessage(cause) } }
async function saveRetention() { if (!policy.value) return; busy.value = true; try { const input = { ...policy.value }; await stepUp.run(() => api.saveRetention(input)); await loadRetention() } catch (cause) { if (!isStepUpCancelled(cause)) retentionError.value = causeMessage(cause) } finally { busy.value = false } }
async function cleanup() { busy.value = true; try { await stepUp.run(() => api.cleanup()); void load('audit'); void loadOverview() } catch (cause) { if (!isStepUpCancelled(cause)) retentionError.value = causeMessage(cause) } finally { busy.value = false } }
watch(tab, value => { if (value === 'overview') void loadOverview(); if (['onboarding','identities'].includes(value)) void load('organizations'); if (value === 'identities') { void load('identities/users'); void load('identities/sessions') } if (value === 'tasks') { void load('report-tasks'); void load('report-executions') } if (value === 'audit') { void load('audit'); void loadRetention() } }, { immediate: true })
watch(search, () => { clearTimeout(searchTimer); regions.organizations.generation++; regions.organizations.rows = []; selectedOrganization.value = ''; searchTimer = setTimeout(() => { void load('organizations') },300) })
watch(kind, () => { for (const path of ['report-tasks','report-executions']) { regions[path].generation++; regions[path].rows = []; void load(path) } })
onBeforeUnmount(() => { alive = false; stepUp.onCancel(); clearTimeout(searchTimer) })
</script>
