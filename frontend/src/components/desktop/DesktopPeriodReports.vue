<template>
  <div class="desktop-analytics desktop-reports min-w-0 space-y-5">
    <div class="card report-kind-panel">
      <nav class="tabs report-tabs report-kind-tabs" :aria-label="t('biNative.period')">
        <button v-for="k in ['day','week','month'] as const" :key="k" class="tab report-tab" :class="{ 'tab-active': kind === k }" type="button" :aria-pressed="kind === k" @click="kind = k">{{ t(`biNative.${k}`) }}</button>
      </nav>
    </div>
    <DesktopDailyReports v-if="kind === 'day'" :organization-id="organizationId" :self-managed="selfManaged === true" :enabled="enabled" :analysis-model="analysisModel" :initial-date="initialDate" />
    <template v-else>
      <header class="card analytics-panel report-period-toolbar">
        <div class="report-period-settings">
          <label class="min-w-0 space-y-2 text-sm font-medium">
            <span class="block">{{ t('biNative.periodStart') }}</span>
            <input v-model="start" type="date" name="period-start" class="input analytics-input" required :aria-invalid="!validStart || undefined" :aria-describedby="!validStart ? 'period-start-error' : undefined" />
          </label>
          <dl class="report-settings-grid">
            <div><dt class="analytics-caption">{{ t(kind === 'week' ? 'biNative.weeklyAuto' : 'biNative.monthlyAuto') }}</dt><dd><span class="report-state" :data-tone="(kind === 'week' ? weeklyEnabled : monthlyEnabled) ? 'success' : 'neutral'">{{ t((kind === 'week' ? weeklyEnabled : monthlyEnabled) ? 'common.enabled' : 'common.disabled') }}</span></dd></div>
            <div><dt class="analytics-caption">{{ t('admin.desktop.reports.configuredModel') }}</dt><dd><bdi>{{ analysisModel || t('admin.desktop.notConfigured') }}</bdi></dd></div>
          </dl>
        </div>
        <p v-if="!validStart" id="period-start-error" class="text-sm text-red-700 dark:text-red-300" role="alert">{{ t(kind === 'week' ? 'biNative.invalidWeekStart' : 'biNative.invalidMonthStart') }}</p>
        <details class="report-guidance"><summary>{{ t('biNative.generationRules') }}</summary><div class="space-y-2 pt-3"><p>{{ t('biNative.periodHint') }}</p><p>{{ t('biNative.automaticHint') }}</p></div></details>
        <div class="report-actions">
          <button class="btn btn-secondary analytics-button" type="button" :disabled="loading || !validStart" @click="load(1, true)">{{ t('common.refresh') }}</button>
          <template v-if="!selfManaged"><button v-for="mode in ['generate','retry','regenerate']" :key="mode" class="btn btn-secondary analytics-button" type="button" :disabled="!canRun(mode)" @click="run(mode)">{{ t(`biNative.${mode}`) }}</button></template>
        </div>
        <p v-if="!enabled" class="report-control-note">{{ t('biNative.summaryDisabled') }}</p>
        <p v-else-if="!analysisModel.trim()" class="report-control-note">{{ t('admin.desktop.reports.reasons.model_missing') }}</p>
        <div v-if="latestReport" class="report-control-note space-y-2" role="status">
          <div class="flex flex-wrap items-center justify-between gap-3"><span>{{ t('biNative.latestTask') }} · v{{ latestReport.revision }} <span class="report-state ms-2" :data-tone="biStatusTone(latestReport.status)">{{ statusLabel(latestReport.status) }}</span></span><button v-if="taskActive" class="btn btn-secondary analytics-button" @click="toggleRefresh">{{ t(autoRefresh ? 'admin.desktop.reports.pauseRefresh' : 'admin.desktop.reports.resumeRefresh') }}</button></div>
          <p v-if="latestReport.reason">{{ reasonLabel(latestReport.reason) }}</p>
          <p v-if="taskActive">{{ t('biNative.taskProgressHint') }}</p>
          <p v-if="!selfManaged">{{ t('biNative.latestTaskActions') }}</p>
        </div>
        <p v-else class="report-control-note">{{ t('biNative.waitingHint') }}</p>
      </header>
      <div v-if="error" role="alert" class="card analytics-panel text-red-700 dark:text-red-300">{{ error }}</div>
      <div class="report-result-grid">
        <section class="card analytics-panel min-w-0 space-y-5" data-testid="period-detail" :aria-busy="detailLoading">
          <p v-if="detailLoading && !detail" role="status" class="analytics-caption">{{ t('common.loading') }}</p>
          <div v-if="detailError" role="alert"><p>{{ detailError }}</p><button class="btn btn-secondary mt-3" @click="select(selected)">{{ t('biNative.retry') }}</button></div>
          <p v-if="!selected && !error" class="analytics-caption">{{ t(loading ? 'common.loading' : reports.length ? 'biNative.selectReport' : 'biNative.noSummaryForPeriod') }}</p>
          <template v-if="detail">
            <header class="flex flex-wrap items-center justify-between gap-3"><div><h2 class="analytics-heading">{{ t(`biNative.${kind}`) }}</h2><p class="analytics-caption mt-1 tabular-nums">{{ detail.report.range.start_date }} — {{ detail.report.range.end_date }}</p></div><button v-if="detail.content" class="btn btn-primary analytics-button" @click="copy">{{ t('common.copy') }}</button></header>
            <p class="analytics-caption tabular-nums">v{{ detail.report.revision }} · {{ biDate(detail.report.generated_at) }}</p>
            <div v-if="viewingHistory" class="report-reading-notice"><p>{{ t('biNative.historyReading') }}</p><button class="btn btn-secondary analytics-button" @click="select(latestReadable!.id)">{{ t('biNative.viewLatestReadable') }}</button></div>
            <p v-if="detail.report.source_changed" class="report-reading-notice">{{ t('biNative.sourceChanged') }}</p>
            <DesktopReportContent v-if="detail.content" :content="detail.content" :source-dates="detail.summary_outline?.validated_source_dates" @source="openSource" />
            <p v-else class="report-control-note">{{ taskActive ? t('admin.desktop.reports.processingHint') : t('biNative.noReportContent') }}</p>
            <p v-if="copyStatus" role="status" class="analytics-caption">{{ copyStatus }}</p>
            <details :key="`metadata-${detail.report.id}`" class="report-disclosure" data-testid="period-metadata">
              <summary>{{ t('biNative.generationDetails') }}</summary>
              <dl class="report-detail-metadata">
                <div><dt>{{ t('biNative.status') }}</dt><dd><span class="report-state" :data-tone="biStatusTone(detail.report.status)">{{ statusLabel(detail.report.status) }}</span></dd></div>
                <div><dt>{{ t('biNative.version') }}</dt><dd>v{{ detail.report.revision }}</dd></div>
                <div><dt>{{ t('biNative.model') }}</dt><dd><bdi>{{ detail.report.model || '—' }}</bdi></dd></div>
                <div><dt>{{ t('admin.desktop.reports.generatedAt') }}</dt><dd>{{ biDate(detail.report.generated_at) }}</dd></div>
                <div v-if="detail.report.reason"><dt>{{ t('biNative.reason') }}</dt><dd>{{ reasonLabel(detail.report.reason) }}</dd></div>
              </dl>
            </details>
            <details :key="`coverage-${detail.report.id}`" class="report-disclosure" data-testid="period-coverage">
              <summary><span>{{ t('biNative.coverage') }}</span><span class="report-state">{{ t('biNative.available') }} {{ coverage.available_dates.length }}</span><span class="report-state" :data-tone="coverage.missing_dates.length ? 'warning' : 'neutral'">{{ t('biNative.missing') }} {{ coverage.missing_dates.length }}</span></summary>
              <dl class="report-coverage-grid">
                <div v-for="field in coverageFields" :key="field.key"><dt>{{ t(`biNative.${field.label}`) }} <span class="tabular-nums">{{ coverage[field.key].length }}</span></dt><dd class="report-date-list"><span v-for="date in coverage[field.key]" :key="date" class="report-date">{{ date }}</span><span v-if="!coverage[field.key].length" class="analytics-caption">—</span></dd></div>
              </dl>
              <details v-if="reasonGroups.length" class="report-guidance mt-4"><summary>{{ t('biNative.coverageReasons') }}</summary><div class="space-y-4 pt-4"><div v-for="group in reasonGroups" :key="group.reason"><h4 class="text-sm font-medium">{{ reasonLabel(group.reason) }} <span class="analytics-caption">({{ group.dates.length }})</span></h4><div class="report-date-list mt-2"><span v-for="date in group.dates" :key="date" class="report-date">{{ date }}</span></div></div></div></details>
            </details>
            <div v-if="sourceError" role="alert" class="report-control-note"><p>{{ sourceError }}</p><button class="btn btn-secondary mt-3" @click="loadSources()">{{ t('biNative.retry') }}</button></div>
            <details :key="`sources-${detail.report.id}`" class="report-disclosure" :aria-busy="sourcesLoading">
              <summary>{{ t('biNative.sources') }} <span class="report-state">{{ sourcesLoading ? t('common.loading') : sources.length }}</span></summary>
              <p v-if="!sourcesLoading && !sourceError && !sources.length" class="analytics-caption mt-3">{{ t('biNative.noData') }}</p><div class="report-source-list mt-4"><button v-for="source in sources" :key="source.date" class="report-version" @click="openSource(source.date)"><span class="font-medium tabular-nums">{{ source.date }}</span><span class="text-sm">v{{ source.revision }}</span><span class="report-state" :data-tone="biStatusTone(source.status)">{{ statusLabel(source.status) }}</span></button></div>
              <button v-if="sourceMore" class="btn btn-secondary mt-4" :disabled="sourcesLoading" @click="loadSources(sourcePage + 1)">{{ t('bi.loadMore') }}</button>
            </details>
            <section v-if="sourceDetail || sourceDetailLoading || sourceDetailError" ref="sourceDetailPanel" class="report-source-detail space-y-4" tabindex="-1" :aria-busy="sourceDetailLoading" :aria-label="t('biNative.sources')">
              <p v-if="sourceDetailLoading" class="analytics-caption" role="status">{{ t('biNative.readingSource') }}</p>
              <div v-if="sourceDetailError" role="alert"><p>{{ sourceDetailError }}</p><button class="btn btn-secondary mt-3" @click="openSource(sourceDate)">{{ t('biNative.retry') }}</button></div>
              <template v-if="sourceDetail"><header class="flex flex-wrap items-center justify-between gap-3"><h3 class="analytics-heading">{{ sourceDetail.date }} · v{{ sourceDetail.revision }}</h3><span class="report-state" :data-tone="biStatusTone(sourceDetail.status)">{{ statusLabel(sourceDetail.status) }}</span></header><p class="analytics-caption">{{ t('biNative.frozenSourceHint') }}</p><DesktopReportContent :content="sourceDetail.content || ''" /></template>
            </section>
          </template>
        </section>
        <aside class="card analytics-panel min-w-0 space-y-4" data-testid="period-history" :aria-busy="loading">
          <details :open="!selected" class="report-history">
          <summary class="analytics-heading">{{ t('biNative.allVersions') }} <span class="report-state ms-2">{{ reports.length }}{{ more ? '+' : '' }}</span></summary>
          <div class="report-history-list mt-4">
          <p v-if="loading" role="status" class="analytics-caption">{{ t('common.loading') }}</p><p v-if="!loading && !reports.length && !error" class="analytics-caption">{{ t('biNative.noData') }}</p>
          <button v-for="report in reports" :key="report.id" class="report-version w-full" :aria-pressed="selected === report.id" @click="select(report.id)"><span class="flex flex-wrap items-center gap-1 text-sm tabular-nums"><time :datetime="report.range.start_date" class="whitespace-nowrap">{{ report.range.start_date }}</time><span>—</span><time :datetime="report.range.end_date" class="whitespace-nowrap">{{ report.range.end_date }}</time></span><span class="mt-2 flex flex-wrap items-center gap-2"><span class="text-sm font-medium">v{{ report.revision }}</span><span class="report-state" :data-tone="biStatusTone(report.status)">{{ statusLabel(report.status) }}</span></span></button>
          <button v-if="more" class="btn btn-secondary" :disabled="loading" @click="load(page + 1)">{{ t('bi.loadMore') }}</button>
          </div>
          </details>
        </aside>
      </div>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DesktopDailyReports from './DesktopDailyReports.vue'
import DesktopReportContent from './DesktopReportContent.vue'
import { biDate, biStatusKey, biStatusTone } from '@/components/bi/display'
import '@/styles/desktopAnalytics.css'
import '@/styles/desktopReports.css'
import { desktopPeriods, lastClosedPeriod, type PeriodCoverage, type PeriodReport, type PeriodSource } from '@/api/desktopPeriods'
const props = defineProps<{ organizationId: string; selfManaged?: boolean; enabled: boolean; analysisModel: string; weeklyEnabled?: boolean; monthlyEnabled?: boolean; initialKind?: string; initialStart?: string; initialReportId?: string; initialDate?: string }>()
const { t, te } = useI18n(), kind = ref<'day' | 'week' | 'month'>(props.initialKind === 'week' || props.initialKind === 'month' ? props.initialKind : 'day'), start = ref(props.initialStart || (props.initialKind === 'week' || props.initialKind === 'month' ? lastClosedPeriod(props.initialKind) : '')), reports = ref<PeriodReport[]>([]), page = ref(1), more = ref(false), loading = ref(false), error = ref(''), busy = ref(false), selected = ref(''), detail = ref<{ report: PeriodReport; content: string | null; summary_outline?: {sections:{kind:string;title:string;start_line:number;end_line:number;source_dates:string[]}[];validated_source_dates:string[];unverified_source_dates:string[]} }>(), detailLoading = ref(false), detailError = ref(''), sources = ref<PeriodSource[]>([]), sourceMore = ref(false), sourcePage = ref(1), sourceError = ref(''), sourceDetail = ref<PeriodSource>(), copyStatus = ref('')
let generation = 0, listGeneration = 0, detailGeneration = 0, sourceListGeneration = 0, sourceDetailGeneration = 0
const api = () => desktopPeriods(props.organizationId, props.selfManaged)
const validStart = computed(() => { if (!/^\d{4}-\d{2}-\d{2}$/.test(start.value) || kind.value === 'day') return false; const d = new Date(start.value+'T00:00:00+08:00'); return !Number.isNaN(d.getTime()) && (kind.value === 'week' ? new Date(d.getTime()+8*3600000).getUTCDay() === 1 : start.value.endsWith('-01')) && start.value <= lastClosedPeriod(kind.value) })
const sourceDetailPanel = ref<HTMLElement>()
const sourcesLoading = ref(false), sourceDetailLoading = ref(false), sourceDetailError = ref(''), sourceDate = ref('')
const coverageFields = [
  { key: 'available_dates', label: 'available' }, { key: 'partial_dates', label: 'partial' },
  { key: 'missing_dates', label: 'missing' }, { key: 'no_record_dates', label: 'noRecords' },
] as const
const coverage = computed<PeriodCoverage>(() => ({
  available_dates: detail.value?.report.coverage?.available_dates ?? [],
  partial_dates: detail.value?.report.coverage?.partial_dates ?? [],
  missing_dates: detail.value?.report.coverage?.missing_dates ?? [],
  no_record_dates: detail.value?.report.coverage?.no_record_dates ?? [],
  reasons: detail.value?.report.coverage?.reasons ?? {},
}))
const reasonGroups = computed(() => {
  const groups = new Map<string, string[]>()
  for (const [date, reason] of Object.entries(coverage.value.reasons)) {
    const dates = groups.get(reason) || []; dates.push(date); groups.set(reason, dates)
  }
  return [...groups].map(([reason, dates]) => ({ reason, dates: dates.sort() }))
})
function statusLabel(value: string) { const key = biStatusKey(value); return key ? t(key) : value }
function reasonLabel(value: string) {
  if (value === 'not_generated') return t('admin.desktop.reports.notGenerated')
  const periodKey = `biNative.periodReasons.${value}`
  if (te(periodKey)) return t(periodKey)
  const key = `admin.desktop.reports.reasons.${value}`
  return te(key) ? t(key) : statusLabel(value)
}
function message(cause: unknown) { const error = cause as { status?: number; metadata?: { reason?: string } }; return error.metadata?.reason ? reasonLabel(error.metadata.reason) : t(error.status === 423 ? 'biNative.stepUp' : 'biNative.failed') }
const latestReport = computed(() => reports.value[0])
const latestReadable = computed(() => reports.value.find(report => ['completed', 'partial'].includes(report.status)))
const taskActive = computed(() => ['pending', 'waiting', 'running'].includes(latestReport.value?.status || ''))
const viewingHistory = computed(() => Boolean(detail.value && latestReadable.value && detail.value.report.id !== latestReadable.value.id))
const autoRefresh = ref(true)
let refreshTimer: ReturnType<typeof setTimeout> | undefined
function scheduleRefresh() {
  clearTimeout(refreshTimer)
  if (!autoRefresh.value || !taskActive.value) return
  refreshTimer = setTimeout(() => {
    if (document.visibilityState === 'visible') void load(1, true)
    else scheduleRefresh()
  }, 5000)
}
function toggleRefresh() { autoRefresh.value = !autoRefresh.value; scheduleRefresh() }
function canRun(mode: string) {
  if (props.selfManaged || busy.value || loading.value || detailLoading.value || detailError.value || error.value || !props.enabled || !props.analysisModel.trim() || !validStart.value) return false
  const current = latestReport.value
  if (!current) return mode === 'generate'
  if (['pending', 'running'].includes(current.status)) return false
  if (mode === 'retry') return ['failed', 'waiting'].includes(current.status) && !(detail.value?.report.id === current.id && detail.value.content)
  return mode === 'regenerate' && !['pending', 'waiting', 'running'].includes(current.status)
}
function mergeReports(first: PeriodReport[], second: PeriodReport[]) {
  const ids = new Set<string>()
  return [...first, ...second].filter(report => { if (ids.has(report.id)) return false; ids.add(report.id); return true })
}
async function load(next = 1, refreshSelected = false) {
  if (!validStart.value) return
  clearTimeout(refreshTimer)
  const g = generation, n = ++listGeneration, keepPages = refreshSelected && page.value > 1
  loading.value = true; error.value = ''
  try {
    const value = await api().list(kind.value, start.value, next)
    if (g !== generation || n !== listGeneration) return
    reports.value = next === 1 ? (keepPages ? mergeReports(value.items, reports.value) : value.items) : mergeReports(reports.value, value.items)
    if (!keepPages) { page.value = next; more.value = value.has_more }
    if (!selected.value && reports.value[0]) await select((latestReadable.value || reports.value[0]).id)
    else if (refreshSelected && selected.value) await select(selected.value, true)
  } catch (cause) { if (g === generation && n === listGeneration) error.value = message(cause) }
  finally { if (g === generation && n === listGeneration) { loading.value = false; scheduleRefresh() } }
}
async function select(id: string, preserve = false) {
  selected.value = id
  if (!preserve) { detail.value = undefined; sources.value = []; sourceDetail.value = undefined; sourceError.value = ''; copyStatus.value = ''; sourceListGeneration++; sourceDetailGeneration++; sourcesLoading.value = false; sourceDetailLoading.value = false; sourceDetailError.value = ''; sourceDate.value = '' }
  detailError.value = ''
  const g = generation, n = ++detailGeneration
  detailLoading.value = true
  try {
    const value = await api().detail(id)
    if (g !== generation || n !== detailGeneration || selected.value !== id) return
    const sourcesChanged = !preserve || detail.value?.report.status !== value.report.status
    detail.value = value
    if (sourcesChanged) void loadSources()
  } catch (cause) {
    if (g === generation && n === detailGeneration) {
      if ([401, 403, 404].includes((cause as { status?: number }).status || 0)) { detail.value = undefined; sources.value = []; sourceDetail.value = undefined; sourceListGeneration++; sourceDetailGeneration++; sourcesLoading.value = false; sourceDetailLoading.value = false }
      detailError.value = message(cause)
    }
  } finally { if (g === generation && n === detailGeneration) detailLoading.value = false }
}
async function loadSources(next = 1) {
  const id = selected.value, g = generation, n = ++sourceListGeneration
  sourcesLoading.value = true; sourceError.value = ''
  try {
    const value = await api().sources(id, next)
    if (g !== generation || n !== sourceListGeneration || id !== selected.value) return
    sources.value = next === 1 ? value.items : [...sources.value, ...value.items]
    sourcePage.value = next; sourceMore.value = value.has_more
  } catch (cause) { if (g === generation && n === sourceListGeneration && id === selected.value) sourceError.value = message(cause) }
  finally { if (g === generation && n === sourceListGeneration) sourcesLoading.value = false }
}
async function openSource(date: string) {
  const id = selected.value, g = generation, n = ++sourceDetailGeneration
  sourceDate.value = date; sourceDetail.value = undefined; sourceDetailError.value = ''; sourceDetailLoading.value = true
  try {
    const value = await api().source(id, date)
    if (g !== generation || n !== sourceDetailGeneration || id !== selected.value) return
    sourceDetail.value = value
  } catch (cause) { if (g === generation && n === sourceDetailGeneration) sourceDetailError.value = message(cause) }
  finally {
    if (g === generation && n === sourceDetailGeneration) {
      sourceDetailLoading.value = false; await nextTick()
      if (g === generation && n === sourceDetailGeneration) { sourceDetailPanel.value?.focus({ preventScroll: true }); sourceDetailPanel.value?.scrollIntoView?.({ block: 'nearest' }) }
    }
  }
}
async function run(mode: string) { if (!canRun(mode)) return; if (mode === 'regenerate' && !window.confirm(t('biNative.confirmRegenerate'))) return; busy.value = true; const g = generation; try { await api().run(kind.value,start.value,mode); if (g === generation) { await load(1, true) } } catch (cause) { if (g === generation) error.value = message(cause) } finally { if (g === generation) busy.value = false } }
async function copy() { const id = selected.value, g = generation; try { const value = await api().detail(id); if (g !== generation || id !== selected.value || detail.value?.report.id !== id || detailError.value || !value.content) return; await navigator.clipboard.writeText(value.content); copyStatus.value = t('biNative.copied') } catch { copyStatus.value = t('biNative.copyFailed') } }
function resetPeriod() {
  clearTimeout(refreshTimer); generation++
  reports.value = []; selected.value = ''; detail.value = undefined; sources.value = []; sourceDetail.value = undefined
  loading.value = false; detailLoading.value = false; busy.value = false; error.value = ''; detailError.value = ''; sourceError.value = ''; copyStatus.value = ''; sourcesLoading.value = false; sourceDetailLoading.value = false; sourceDetailError.value = ''; sourceDate.value = ''
  page.value = 1; more.value = false; sourcePage.value = 1; sourceMore.value = false
}
watch([kind, () => props.organizationId, () => props.selfManaged], () => {
  resetPeriod()
  if (kind.value !== 'day') { const next = lastClosedPeriod(kind.value); if (start.value === next) void load(); else start.value = next }
})
watch(start, () => { resetPeriod(); void load() })
if (kind.value !== 'day') { void load(); if (props.initialReportId) void select(props.initialReportId) }
onBeforeUnmount(() => { generation++; clearTimeout(refreshTimer) })
</script>
