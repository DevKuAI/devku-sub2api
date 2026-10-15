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
        <div v-if="overview" class="card bi-panel"><h2 class="bi-heading mb-4">{{ t('biNative.runtimeCounts') }}</h2><dl class="bi-metrics"><div v-for="name in countKeys" :key="name" class="bi-metric"><dt class="bi-caption">{{ t(`biNative.counts.${name}`) }}</dt><dd>{{ overview.counts?.[name] == null ? '-' : biNumber(overview.counts[name]) }}</dd></div></dl></div>
        <div v-if="overview?.config" class="card bi-panel"><h2 class="bi-heading">{{ t('biNative.configurationStatus') }}</h2><p class="bi-caption mt-2 mb-4">{{ t('biNative.configurationHint') }}</p><dl class="bi-config"><div v-for="field in configKeys" :key="field"><dt class="bi-caption">{{ t(`biNative.config.${field}`) }}</dt><dd><span v-if="typeof overview.config[field] === 'boolean'" class="bi-badge" :data-tone="overview.config[field] ? 'success' : 'warning'">{{ t(overview.config[field] ? 'admin.desktop.configured' : 'admin.desktop.notConfigured') }}</span><bdi v-else>{{ biText(overview.config[field]) }}</bdi></dd></div></dl></div>
      </section>
      <section v-else-if="tab === 'onboarding'" id="bi-panel-onboarding" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-onboarding" class="space-y-5">
        <div class="card bi-panel">
          <header class="bi-toolbar"><div><h2 class="bi-heading">{{ t('biNative.onboarding') }}</h2><p class="bi-caption mt-2">{{ t('biNative.onboardingHint') }}</p></div><button class="btn btn-secondary" :disabled="regions.organizations.loading" @click="load('organizations')">{{ t('common.refresh') }}</button></header>
          <div class="bi-filter-bar"><label class="min-w-0 flex-1 sm:max-w-md"><span class="input-label">{{ t('biNative.searchOrganizations') }}</span><input v-model="search" class="input" type="search" /></label><button v-if="search" class="btn btn-secondary" @click="search = ''">{{ t('biNative.clearSearch') }}</button></div>
          <OperationsTable :rows="regions.organizations.rows" :columns="organizationColumns" :loading="regions.organizations.loading" :error="regions.organizations.error" :empty-text="search ? t('biNative.noOrganizationsMatch') : undefined" @retry="load('organizations')">
            <template #cell-features="{ row }"><div class="bi-feature-grid"><span v-for="field in organizationFeatures" :key="field" class="bi-feature" :data-enabled="row[field] === true"><span>{{ t(`biNative.featureLabels.${field}`) }}</span><span>{{ row[field] == null ? '-' : t(row[field] ? 'common.enabled' : 'biNative.notEnabled') }}</span></span></div></template>
            <template #actions="{ row }"><router-link class="btn btn-secondary" :to="organizationLink(String(row.id))">{{ t('biNative.openOrganization') }}</router-link><button class="btn btn-secondary" aria-haspopup="dialog" @click="openAccess(row)">{{ t('biNative.invitations') }}</button></template>
          </OperationsTable>
          <More path="organizations" />
        </div>
      </section>
      <section v-else-if="tab === 'identities'" id="bi-panel-identities" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-identities" class="space-y-5">
        <div class="card bi-panel">
          <header class="bi-toolbar"><div><h2 class="bi-heading">{{ t('biNative.invitations') }}</h2><p class="bi-caption mt-2">{{ t('biNative.selectOrganizationHint') }}</p></div><button class="btn btn-secondary" :disabled="regions.organizations.loading" @click="load('organizations')">{{ t('common.refresh') }}</button></header>
          <div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('biNative.searchOrganizations') }}</span><input v-model="search" class="input" type="search" /></label><label><span class="input-label">{{ t('biNative.selectOrganization') }}</span><select v-model="selectedOrganization" class="input"><option value="">{{ t('biNative.selectOrganization') }}</option><option v-for="row in regions.organizations.rows" :key="String(row.id)" :value="String(row.id)">{{ row.name }}</option></select></label></div>
          <p v-if="regions.organizations.loading" role="status" class="bi-caption mt-4">{{ t('common.loading') }}</p>
          <div v-if="regions.organizations.error" role="alert" class="bi-error mt-4"><p>{{ regions.organizations.error }}</p><button class="btn btn-secondary mt-3" @click="load('organizations')">{{ t('biNative.retry') }}</button></div>
          <p v-if="!regions.organizations.loading && !regions.organizations.error && !regions.organizations.rows.length" class="bi-empty mt-4">{{ t(search ? 'biNative.noOrganizationsMatch' : 'biNative.noData') }}</p>
          <More path="organizations" />
        </div>
        <div v-if="selectedOrganization" class="card bi-panel bi-toolbar"><div class="min-w-0"><h2 class="bi-heading break-words">{{ selectedOrganizationName }}</h2><p class="bi-caption mt-2">{{ t('biNative.selectOrganizationHint') }}</p></div><button class="btn btn-primary" aria-haspopup="dialog" @click="accessDialogOpen = true">{{ t('biNative.accessManagement') }}</button></div>
        <div v-for="region in identityRegions" :key="region.path" class="card bi-panel">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t(`biNative.${region.title}`) }}</h2><button class="btn btn-secondary" :disabled="regions[region.path].loading" @click="load(region.path)">{{ t('common.refresh') }}</button></header>
          <OperationsTable :rows="regions[region.path].rows" :columns="region.columns" :loading="regions[region.path].loading" :error="regions[region.path].error" @retry="load(region.path)"><template v-if="region.path === 'identities/users'" #actions="{ row }"><button class="btn btn-secondary" :disabled="busy" @click="setStatus(String(row.id), row.status === 'active' ? 'disabled' : 'active')">{{ t(row.status === 'active' ? 'biNative.disableUser' : 'biNative.enableUser') }}</button></template></OperationsTable><More :path="region.path" />
        </div>
      </section>
      <section v-else-if="tab === 'tasks'" id="bi-panel-tasks" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-tasks" class="space-y-5">
        <ReportTaskStatistics :statistics="statistics" :loading="statisticsLoading" :error="statisticsError" @refresh="refreshReports"><template #filters><label><span class="input-label">{{ t('biNative.period') }}</span><select v-model="kind" class="input"><option value="">{{ t('common.all') }}</option><option v-for="k in ['day','week','month']" :key="k" :value="k">{{ t(`biNative.${k}`) }}</option></select></label></template></ReportTaskStatistics>
        <div v-for="region in taskRegions" :key="region.path" class="card bi-panel" :data-testid="`bi-${region.path}`">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t(`biNative.${region.title}`) }}</h2><button class="btn btn-secondary" :disabled="regions[region.path].loading" @click="refreshReportRegion(region.path)">{{ t('common.refresh') }}</button></header>
          <OperationsTable :rows="regions[region.path].rows" :columns="region.columns" :details="region.details" :loading="regions[region.path].loading" :error="regions[region.path].error" @retry="load(region.path)"><template v-if="region.path === 'report-tasks'" #actions="{ row }"><router-link class="btn btn-secondary" :to="reportLink(row)">{{ t('biNative.openOrganization') }}</router-link></template></OperationsTable><More :path="region.path" />
        </div>
      </section>
      <section v-else id="bi-panel-audit" role="tabpanel" tabindex="0" aria-labelledby="bi-tab-audit" class="space-y-5">
        <div class="card bi-panel"><header class="bi-toolbar"><h2 class="bi-heading">{{ t('biNative.auditLog') }}</h2><button class="btn btn-secondary" :disabled="regions.audit.loading" @click="load('audit')">{{ t('common.refresh') }}</button></header><OperationsTable :rows="regions.audit.rows" :columns="['actor_type','actor_id','organization_id','action','target_id','request_id','created_at']" :loading="regions.audit.loading" :error="regions.audit.error" @retry="load('audit')" /><More path="audit" /></div>
        <form class="card bi-panel space-y-4" @submit.prevent="saveRetention">
          <header class="bi-toolbar"><h2 class="bi-heading">{{ t('biNative.retention') }}</h2><button class="btn btn-secondary" type="button" :disabled="busy || retentionLoading" @click="loadRetention">{{ t('common.refresh') }}</button></header><p class="bi-caption">{{ t('biNative.scopeCleanup') }}</p><p v-if="retentionLoading" role="status" class="bi-caption">{{ t('common.loading') }}</p><p v-if="retentionError" role="alert" class="bi-error">{{ retentionError }}</p><p role="status" class="bi-caption">{{ retentionMessage }}</p>
          <template v-if="policy"><div class="grid gap-4 sm:grid-cols-2"><label><span class="input-label">{{ t('biNative.auditDays') }}</span><input v-model.number="policy.audit_days" class="input" type="number" min="1" max="3650" required /></label><label><span class="input-label">{{ t('biNative.ephemeralDays') }}</span><input v-model.number="policy.ephemeral_days" class="input" type="number" min="1" max="365" required /></label></div><div class="bi-row-actions"><button class="btn btn-primary" :disabled="busy || retentionLoading">{{ t('common.save') }}</button><button class="btn btn-danger" type="button" :disabled="busy || retentionLoading" @click="cleanupConfirm = true">{{ t('biNative.cleanup') }}</button></div></template>
        </form>
      </section>
      <BaseDialog :show="accessDialogOpen" :title="t('biNative.organizationAccessTitle', { name: selectedOrganizationName })" :close-label="t('common.close')" width="extra-wide" content-class="bi-access-dialog" :close-on-escape="!accessBusy" :show-close-button="!accessBusy" @close="closeAccess">
        <BIGrantManagement v-if="accessDialogOpen && selectedOrganization" :key="selectedOrganization" :organization-id="selectedOrganization" @busy-change="accessBusy = $event" />
        <template #footer><button class="btn btn-secondary" :disabled="accessBusy" @click="closeAccess">{{ t('common.close') }}</button></template>
      </BaseDialog>
      <ConfirmDialog :show="cleanupConfirm" :title="t('biNative.cleanup')" :message="t('biNative.confirmCleanup')" :confirm-text="t('biNative.cleanup')" :loading="busy" danger content-class="bi-access-dialog" @confirm="cleanup" @cancel="cleanupConfirm = false" />
      <BIStepUpDialog :controller="stepUp" />
      <p v-if="actionError" role="alert" class="bi-error">{{ actionError }}</p>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, defineComponent, h, nextTick, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BIStepUpDialog from '@/components/bi/BIStepUpDialog.vue'
import { useStepUp, isStepUpCancelled } from '@/composables/useStepUp'
import BIGrantManagement from '@/components/bi/BIGrantManagement.vue'
import OperationsTable from '@/components/bi/OperationsTable.vue'
import ReportTaskStatistics from '@/components/bi/ReportTaskStatistics.vue'
import { biNumber, biText } from '@/components/bi/display'
import '@/styles/biOperations.css'
import api, { type BIOverview, type BIOperationRow, type BIRetentionPolicy, type BIReportStatistics } from '@/api/admin/bi'
const { t } = useI18n(), tabs = ['overview','onboarding','identities','tasks','audit'] as const
const countKeys = ['access', 'users', 'sessions', 'invitations', 'daily_running', 'period_running'] as const
const configKeys = ['appid', 'app_secret_configured', 'jwt_secret_configured', 'identity_secret_configured', 'min_client_version'] as const
const organizationColumns = ['name','status','features','analysis_model','users','last_received_at']
const organizationFeatures = ['reporting_enabled','summary_enabled','weekly_summary_enabled','monthly_summary_enabled']
const identityRegions = [
  { path: 'identities/users', title: 'users', columns: ['display_name','status','authorization_version','created_at','last_seen_at'] },
  { path: 'identities/sessions', title: 'sessions', columns: ['bi_user_id','status','client_version','last_seen_at','expires_at'] },
]
const taskRegions = [
  { path: 'report-tasks', title: 'tasks', columns: ['organization_name','kind','period_start','period_end','status','model','total_tokens','actual_cost'], details: ['organization_id','revision','available_sources','missing_sources','reason'] },
  { path: 'report-executions', title: 'executions', columns: ['organization_name','kind','period_start','model','status','total_tokens','actual_cost','started_at'], details: ['organization_id','task_id','period_end','revision','round','chunk','attempt','request_id','finished_at','error'] },
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
const accessDialogOpen = ref(false), accessBusy = ref(false), cleanupConfirm = ref(false)
const retentionLoading = ref(false), retentionMessage = ref('')
const selectedOrganizationName = computed(() => biText(regions.organizations.rows.find(row => String(row.id) === selectedOrganization.value)?.name))
function openAccess(row: BIOperationRow) { selectedOrganization.value = String(row.id); accessDialogOpen.value = true }
function closeAccess() { if (!accessBusy.value) accessDialogOpen.value = false }
const paths = ['organizations','identities/users','identities/sessions','report-tasks','report-executions','audit']
const regions = reactive<Record<string, { rows: BIOperationRow[]; page: number; more: boolean; loading: boolean; error: string; generation: number }>>(Object.fromEntries(paths.map(path => [path, { rows: [], page: 1, more: false, loading: false, error: '', generation: 0 }])))
const statistics = ref<BIReportStatistics>(), statisticsLoading = ref(false), statisticsError = ref('')
let statisticsGeneration = 0
let alive = true, searchTimer: ReturnType<typeof setTimeout> | undefined
const causeMessage = (cause: unknown) => t((cause as { status?: number }).status === 423 ? 'biNative.stepUp' : 'biNative.failed')
const organizationLink = (id: string, target = 'grants') => ({ path: `/admin/desktop/organizations/${encodeURIComponent(id)}`, query: { tab: target } })
const reportLink = (row: BIOperationRow) => ({path:`/admin/desktop/organizations/${encodeURIComponent(String(row.organization_id))}`,query:{tab:'reports',report_kind:String(row.kind),period_start:String(row.period_start),report_id:String(row.report_id??''),report_date:String(row.period_start)}})
let overviewGeneration = 0, retentionGeneration = 0
async function loadOverview() { const g = ++overviewGeneration; overviewLoading.value = true; overviewError.value = ''; try { const value = await api.getOverview(); if (alive && g === overviewGeneration) overview.value = value } catch (cause) { if (alive && g === overviewGeneration) overviewError.value = causeMessage(cause) } finally { if (alive && g === overviewGeneration) overviewLoading.value = false } }
async function loadStatistics() {
  const g = ++statisticsGeneration
  statistics.value = undefined; statisticsLoading.value = true; statisticsError.value = ''
  try { const value = await api.reportStatistics({ kind: kind.value }); if (alive && g === statisticsGeneration) statistics.value = value }
  catch (cause) { if (alive && g === statisticsGeneration) statisticsError.value = causeMessage(cause) }
  finally { if (alive && g === statisticsGeneration) statisticsLoading.value = false }
}
function refreshReports() { void loadStatistics(); void load('report-tasks'); void load('report-executions') }
function refreshReportRegion(path: string) { void load(path); if (path.startsWith('report-')) void loadStatistics() }
async function load(path: string, page = 1) { const region = regions[path], g = ++region.generation; region.loading = true; region.error = ''; const params: Record<string,string> = {}; if (path === 'organizations') params.search = search.value; if (path.startsWith('report-')) params.kind = kind.value; try { const value = await api.list(path,page,params); if (!alive || g !== region.generation) return; region.rows = page === 1 ? value.items : [...region.rows,...value.items]; region.page = page; region.more = value.has_more } catch (cause) { if (alive && g === region.generation) region.error = causeMessage(cause) } finally { if (alive && g === region.generation) region.loading = false } }
const More = defineComponent({ props: { path: { type: String, required: true } }, setup(props) { return () => regions[props.path].more ? h('div', { class: 'bi-footer' }, [h('button', { class: 'btn btn-secondary', disabled: regions[props.path].loading, onClick: () => load(props.path, regions[props.path].page + 1) }, t('bi.loadMore'))]) : null } })
async function setStatus(id: string, status: string) { if (busy.value) return; if (!window.confirm(t(status === 'disabled' ? 'biNative.disableUser' : 'biNative.enableUser'))) return; busy.value = true; actionError.value = ''; try { await stepUp.run(() => api.userStatus(id,status)); void load('identities/users'); void load('identities/sessions'); void loadOverview() } catch (cause) { if (!isStepUpCancelled(cause)) actionError.value = causeMessage(cause) } finally { busy.value = false } }
async function loadRetention() {
  const g = ++retentionGeneration
  retentionLoading.value = true
  retentionError.value = ''
  try { const value = await api.retention(); if (alive && g === retentionGeneration) policy.value = value }
  catch (cause) { if (alive && g === retentionGeneration && !isStepUpCancelled(cause)) retentionError.value = causeMessage(cause) }
  finally { if (alive && g === retentionGeneration) retentionLoading.value = false }
}
async function saveRetention() {
  if (!policy.value || busy.value || retentionLoading.value) return
  busy.value = true; retentionError.value = ''; retentionMessage.value = ''
  try { const input = { ...policy.value }; await stepUp.run(() => api.saveRetention(input)); await loadRetention(); retentionMessage.value = t('biNative.retentionSaved') }
  catch (cause) { if (!isStepUpCancelled(cause)) retentionError.value = causeMessage(cause) }
  finally { busy.value = false }
}
async function cleanup() {
  if (busy.value) return
  cleanupConfirm.value = false; busy.value = true; retentionError.value = ''; retentionMessage.value = ''
  try { await stepUp.run(() => api.cleanup()); retentionMessage.value = t('biNative.cleanupCompleted'); void load('audit'); void loadOverview() }
  catch (cause) { if (!isStepUpCancelled(cause)) retentionError.value = causeMessage(cause) }
  finally { busy.value = false }
}
watch(tab, value => { actionError.value = ''; retentionMessage.value = '';  if (value === 'overview') void loadOverview(); if (['onboarding','identities'].includes(value)) void load('organizations'); if (value === 'identities') { void load('identities/users'); void load('identities/sessions') } if (value === 'tasks') refreshReports(); if (value === 'audit') { void load('audit'); void loadRetention() } }, { immediate: true })
watch(search, () => { clearTimeout(searchTimer); regions.organizations.generation++; regions.organizations.rows = []; regions.organizations.more = false; regions.organizations.loading = true; selectedOrganization.value = ''; accessDialogOpen.value = false; searchTimer = setTimeout(() => { void load('organizations') },300) })
watch(kind, () => { void loadStatistics(); for (const path of ['report-tasks','report-executions']) { regions[path].generation++; regions[path].rows = []; regions[path].more = false; void load(path) } })
onBeforeUnmount(() => { alive = false; stepUp.onCancel(); clearTimeout(searchTimer) })
</script>
