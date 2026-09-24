<template>
  <div class="min-h-full space-y-6 p-4 sm:p-6">
    <header class="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
      <div>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">BI 运维</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">查看 BI 服务状态、身份、同步、报告和审计记录。</p>
      </div>
      <button class="btn btn-secondary" type="button" :disabled="loading" @click="loadActive">刷新</button>
    </header>

    <p v-if="error" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300" role="alert">{{ error }}</p>
    <div v-if="loading && !overview" class="card p-8 text-center text-sm text-gray-500">加载中…</div>

    <template v-else>
      <section v-if="overview" class="grid grid-cols-2 gap-3 md:grid-cols-4 xl:grid-cols-7">
        <div v-for="item in overviewCards" :key="item.key" class="card p-4">
          <div class="text-xs text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ item.value }}</div>
        </div>
      </section>

      <nav class="overflow-x-auto border-b border-gray-200 dark:border-dark-700" aria-label="BI 运维模块">
        <div class="flex min-w-max gap-5">
          <button v-for="tab in tabs" :key="tab.key" type="button" class="border-b-2 px-1 py-3 text-sm font-medium" :class="activeTab === tab.key ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'" @click="activeTab = tab.key">{{ tab.label }}</button>
        </div>
      </nav>

      <section v-if="activeTab === 'overview'" class="grid gap-6 xl:grid-cols-2">
        <div class="card p-5">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">服务配置状态</h2>
          <dl v-if="overview" class="mt-4 grid gap-3 text-sm sm:grid-cols-2">
            <div><dt class="text-gray-500">启用状态</dt><dd class="mt-1 font-medium">{{ overview.config.enabled ? '已启用' : '未启用' }}</dd></div>
            <div><dt class="text-gray-500">AppID</dt><dd class="mt-1 font-medium">{{ overview.config.appid || '未配置' }}</dd></div>
            <div><dt class="text-gray-500">客户端最低版本</dt><dd class="mt-1 font-medium">{{ overview.config.min_client_version || '未配置' }}</dd></div>
            <div><dt class="text-gray-500">密钥</dt><dd class="mt-1 font-medium">{{ configuredSecrets }}</dd></div>
            <div><dt class="text-gray-500">报告保留</dt><dd class="mt-1 font-medium">{{ overview.retention.report_months }} 个月</dd></div>
          </dl>
        </div>
        <div class="card p-5">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">数据质量入口</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">来源健康度沿用用户端 freshness 状态，按来源展示覆盖范围、延迟和 partial/unknown 原因。</p>
          <button class="btn btn-secondary mt-4" type="button" @click="activeTab = 'quality'">查看来源质量</button>
        </div>
      </section>

      <section v-else-if="activeTab === 'identity'" class="space-y-5">
        <div class="flex flex-wrap gap-2">
          <button v-for="filter in ['all', 'active', 'revoked']" :key="filter" type="button" class="btn" :class="identityFilter === filter ? 'btn-primary' : 'btn-secondary'" @click="identityFilter = filter; loadIdentity()">{{ filterLabel(filter) }}</button>
          <button class="btn btn-secondary" type="button" @click="loadChallenges">待确认绑定</button>
          <button class="btn btn-secondary" type="button" @click="loadSessions">设备/会话</button>
        </div>
        <div class="card overflow-hidden">
          <table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">微信身份</th><th class="px-4 py-3">账号</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">最近登录</th><th class="px-4 py-3">会话</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in bindings" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.id }}</td><td class="px-4 py-3">{{ row.display_name }} (#{{ row.user_id }})</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ formatDate(row.last_login_at) }}</td><td class="px-4 py-3">{{ row.session_count }}</td><td class="px-4 py-3"><button v-if="row.status === 'active'" class="btn btn-danger btn-sm" type="button" @click="revokeBinding(row)">强制撤销</button></td></tr><tr v-if="!bindings.length"><td colspan="6" class="px-4 py-8 text-center text-gray-500">暂无绑定记录</td></tr></tbody></table>
        </div>
        <div v-if="challenges.length" class="card p-4"><h2 class="font-semibold">待确认/过期挑战</h2><div class="mt-3 grid gap-2 text-sm"><div v-for="row in challenges" :key="row.id" class="flex flex-wrap justify-between gap-2 border-b border-gray-100 py-2 last:border-0 dark:border-dark-700"><span class="font-mono text-xs">{{ row.id }}</span><span :class="statusClass(row.status)">{{ row.status }}</span><span>{{ formatDate(row.expires_at) }}</span></div></div></div>
        <div v-if="sessions.length" class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">会话</th><th class="px-4 py-3">设备</th><th class="px-4 py-3">客户端</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">过期时间</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in sessions" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.id }}</td><td class="px-4 py-3">{{ row.device_id || '-' }} / {{ row.platform || '-' }}</td><td class="px-4 py-3">{{ row.client_version || '-' }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ formatDate(row.expires_at) }}</td></tr></tbody></table></div>
      </section>

      <section v-else-if="activeTab === 'sources'" class="space-y-5">
        <form class="card grid gap-3 p-5 md:grid-cols-5" @submit.prevent="issueCredential"><input v-model="credentialForm.organization_id" class="input" placeholder="organization_id" required /><input v-model="credentialForm.source_id" class="input" placeholder="source_id" required /><input v-model="credentialForm.namespace" class="input" placeholder="namespace" required /><input v-model="credentialForm.allowed_kinds" class="input" placeholder="allowed kinds, comma separated" required /><input v-model="credentialForm.expires_at" class="input" type="datetime-local" required /><button class="btn btn-primary md:col-span-5 md:justify-self-start" type="submit">签发 Connector 凭证</button></form>
        <div v-if="issuedToken" class="card border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/20 dark:text-amber-200"><strong>凭证只显示一次：</strong><code class="ml-2 break-all">{{ issuedToken }}</code></div>
        <div class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">企业</th><th class="px-4 py-3">source</th><th class="px-4 py-3">namespace</th><th class="px-4 py-3">允许类型</th><th class="px-4 py-3">checkpoint</th><th class="px-4 py-3">凭证</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in sources" :key="`${row.organization_id}:${row.source_id}`"><td class="px-4 py-3">{{ row.organization_id }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.source_id }}</td><td class="px-4 py-3">{{ row.namespace }}</td><td class="px-4 py-3">{{ row.allowed_kinds.join(', ') }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.checkpoint || '-' }}</td><td class="px-4 py-3">{{ row.credential_count }}</td><td class="px-4 py-3"><button class="btn btn-secondary btn-sm" type="button" @click="loadCredentials(row)">查看凭证</button></td></tr><tr v-if="!sources.length"><td colspan="7" class="px-4 py-8 text-center text-gray-500">暂无数据源</td></tr></tbody></table></div>
        <div v-if="selectedSource" class="card overflow-hidden"><div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700"><h2 class="font-semibold">{{ selectedSource.source_id }} 的 Connector 凭证</h2></div><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">token 前缀</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">过期时间</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in credentials" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.token_prefix }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ formatDate(row.expires_at) }}</td><td class="flex gap-2 px-4 py-3"><button v-if="row.status === 'active'" class="btn btn-danger btn-sm" type="button" @click="revokeCredential(row.id)">撤销</button><button v-if="row.status === 'active'" class="btn btn-secondary btn-sm" type="button" @click="rotateCredential(row.id)">轮换</button></td></tr><tr v-if="!credentials.length"><td colspan="4" class="px-4 py-8 text-center text-gray-500">暂无凭证</td></tr></tbody></table></div>
      </section>

      <section v-else-if="activeTab === 'imports'" class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">批次</th><th class="px-4 py-3">企业/source</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">checkpoint</th><th class="px-4 py-3">attempt</th><th class="px-4 py-3">错误</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in imports" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.id }}</td><td class="px-4 py-3">{{ row.organization_id }} / {{ row.source_id }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3 font-mono text-xs">{{ row.checkpoint || '-' }}</td><td class="px-4 py-3">{{ row.attempt_count }}</td><td class="max-w-xs px-4 py-3 text-xs text-red-600">{{ row.errors?.[0]?.message || '-' }}</td><td class="px-4 py-3"><button v-if="row.status === 'failed' || row.status === 'rejected'" class="btn btn-secondary btn-sm" type="button" @click="retryImport(row)">重试</button></td></tr><tr v-if="!imports.length"><td colspan="7" class="px-4 py-8 text-center text-gray-500">暂无导入批次</td></tr></tbody></table></section>

      <section v-else-if="activeTab === 'quality'" class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">企业/source</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">覆盖范围</th><th class="px-4 py-3">延迟</th><th class="px-4 py-3">原因</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in quality" :key="`${row.organization_id}:${row.source_id}`"><td class="px-4 py-3">{{ row.organization_id }} / {{ row.source_id }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ row.coverage_start || '-' }} ~ {{ row.coverage_end || '-' }}</td><td class="px-4 py-3">{{ row.latency_seconds == null ? '-' : `${Math.round(row.latency_seconds)} 秒` }}</td><td class="px-4 py-3 text-xs text-gray-500">{{ row.reason || '-' }}</td></tr><tr v-if="!quality.length"><td colspan="5" class="px-4 py-8 text-center text-gray-500">暂无质量数据</td></tr></tbody></table></section>

      <section v-else-if="activeTab === 'reports'" class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">报告</th><th class="px-4 py-3">企业</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">尝试</th><th class="px-4 py-3">失败详情</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in reports" :key="row.id"><td class="px-4 py-3"><div>{{ row.title }}</div><code class="text-xs text-gray-500">{{ row.id }}</code></td><td class="px-4 py-3">{{ row.organization_id }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ row.attempt_count }}</td><td class="px-4 py-3 text-xs text-red-600">{{ row.failure_code || '-' }}</td><td class="flex gap-2 px-4 py-3"><button v-if="row.status === 'failed' || row.status === 'lease_timeout'" class="btn btn-secondary btn-sm" type="button" @click="retryReport(row)">重试</button><button v-if="row.status !== 'archived'" class="btn btn-danger btn-sm" type="button" @click="archiveReport(row)">归档</button></td></tr><tr v-if="!reports.length"><td colspan="6" class="px-4 py-8 text-center text-gray-500">暂无报告任务</td></tr></tbody></table></section>

      <section v-else-if="activeTab === 'retention'" class="card max-w-2xl p-5"><h2 class="text-base font-semibold">清理与保留策略</h2><form class="mt-5 grid gap-4 sm:grid-cols-2" @submit.prevent="saveRetention"><label class="text-sm">报告保留（月）<input v-model.number="retention.report_months" class="input mt-1" type="number" min="1" max="120" /></label><label class="text-sm">业务事实保留（月）<input v-model.number="retention.fact_months" class="input mt-1" type="number" min="1" max="120" /></label><label class="text-sm">审计保留（天）<input v-model.number="retention.audit_days" class="input mt-1" type="number" min="1" max="3650" /></label><label class="text-sm">临时状态保留（天）<input v-model.number="retention.ephemeral_days" class="input mt-1" type="number" min="1" max="365" /></label><div class="flex flex-wrap gap-2 sm:col-span-2"><button class="btn btn-primary" type="submit">保存策略</button><button class="btn btn-danger" type="button" @click="runCleanup">执行清理</button></div></form></section>

      <section v-else-if="activeTab === 'audit'" class="card overflow-hidden"><div class="border-b border-gray-100 p-4 dark:border-dark-700"><input v-model="auditRequestID" class="input max-w-md" placeholder="按 request_id 查询" @keyup.enter="loadAudit" /><button class="btn btn-secondary ml-2" type="button" @click="loadAudit">查询</button></div><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">时间</th><th class="px-4 py-3">动作</th><th class="px-4 py-3">资源</th><th class="px-4 py-3">request_id</th><th class="px-4 py-3">管理员</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in audit" :key="row.id"><td class="px-4 py-3">{{ formatDate(row.created_at) }}</td><td class="px-4 py-3">{{ row.action }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.target_id }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.request_id }}</td><td class="px-4 py-3">{{ row.actor_user_id || '-' }}</td></tr><tr v-if="!audit.length"><td colspan="5" class="px-4 py-8 text-center text-gray-500">暂无审计事件</td></tr></tbody></table></section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import type { BIAuditEvent, BIBindingOperation, BIChallengeOperation, BICredentialOperation, BIImportOperation, BIQualityOperation, BIReportOperation, BISessionOperation, BISourceOperation, BIOperationsOverview, BIRetentionPolicy } from '@/api/admin/bi'
import { useAppStore } from '@/stores'

type Tab = 'overview' | 'identity' | 'sources' | 'imports' | 'quality' | 'reports' | 'retention' | 'audit'
const appStore = useAppStore()
const activeTab = ref<Tab>('overview')
const loading = ref(false)
const error = ref('')
const overview = ref<BIOperationsOverview | null>(null)
const bindings = ref<BIBindingOperation[]>([])
const challenges = ref<BIChallengeOperation[]>([])
const sessions = ref<BISessionOperation[]>([])
const sources = ref<BISourceOperation[]>([])
const credentials = ref<BICredentialOperation[]>([])
const selectedSource = ref<BISourceOperation | null>(null)
const imports = ref<BIImportOperation[]>([])
const quality = ref<BIQualityOperation[]>([])
const reports = ref<BIReportOperation[]>([])
const audit = ref<BIAuditEvent[]>([])
const retention = reactive<BIRetentionPolicy>({ report_months: 24, fact_months: 24, audit_days: 180, ephemeral_days: 7 })
const identityFilter = ref('all')
const auditRequestID = ref('')
const issuedToken = ref('')
const credentialForm = reactive({ organization_id: '', source_id: '', namespace: '', allowed_kinds: '', expires_at: '' })
const tabs = [{ key: 'overview', label: '总览' }, { key: 'identity', label: '微信身份' }, { key: 'sources', label: '数据源' }, { key: 'imports', label: '数据同步' }, { key: 'quality', label: '数据质量' }, { key: 'reports', label: '报告任务' }, { key: 'retention', label: '清理与保留' }, { key: 'audit', label: '审计' }] as const

const overviewCards = computed(() => overview.value ? [
  { key: 'bindings', label: '绑定', value: overview.value.counts.bindings || 0 }, { key: 'pending', label: '待确认绑定', value: overview.value.counts.pending_challenges || 0 }, { key: 'sessions', label: '活跃会话', value: overview.value.counts.active_sessions || 0 }, { key: 'sources', label: '数据源', value: overview.value.counts.sources || 0 }, { key: 'imports', label: '处理中批次', value: overview.value.counts.imports_processing || 0 }, { key: 'reports', label: '生成中报告', value: overview.value.counts.reports_processing || 0 }, { key: 'enabled', label: 'BI 服务', value: overview.value.config.enabled ? 'ON' : 'OFF' },
] : [])
const configuredSecrets = computed(() => overview.value ? `${overview.value.config.app_secret_configured ? 'AppSecret' : ''} ${overview.value.config.jwt_secret_configured ? 'JWT' : ''} ${overview.value.config.identity_secret_configured ? 'Identity' : ''}`.trim() || '未配置' : '未加载')

function formatDate(value: string | null | undefined) { return value ? new Date(value).toLocaleString() : '-' }
function statusClass(status: string) { return status === 'ready' || status === 'active' || status === 'applied' ? 'text-green-600 dark:text-green-400' : status === 'failed' || status === 'rejected' || status === 'revoked' || status === 'expired' || status === 'unknown' ? 'text-red-600 dark:text-red-400' : 'text-amber-600 dark:text-amber-400' }
function filterLabel(value: string) { return value === 'all' ? '全部' : value === 'active' ? '有效' : '已撤销' }
function confirmAction(message: string) { return window.confirm(message) }
function showError(reason: unknown) { error.value = reason instanceof Error ? reason.message : '操作失败，请稍后重试' }
async function loadOverview() { overview.value = await adminAPI.bi.getOverview(); Object.assign(retention, overview.value.retention) }
async function loadIdentity() { const result = await adminAPI.bi.listBindings(identityFilter.value === 'all' ? undefined : { status: identityFilter.value }); bindings.value = result.items }
async function loadChallenges() { challenges.value = (await adminAPI.bi.listChallenges({ page_size: 100 })).items }
async function loadSessions() { sessions.value = (await adminAPI.bi.listSessions({ page_size: 100 })).items }
async function loadSources() { sources.value = (await adminAPI.bi.listSources({ page_size: 100 })).items }
async function loadCredentials(row: BISourceOperation) { try { selectedSource.value = row; credentials.value = (await adminAPI.bi.listCredentials(row.source_id, row.organization_id)).items } catch (reason) { showError(reason) } }
async function loadImports() { imports.value = (await adminAPI.bi.listImports({ page_size: 100 })).items }
async function loadQuality() { quality.value = (await adminAPI.bi.listQuality({ organization_id: credentialForm.organization_id || undefined })).items }
async function loadReports() { reports.value = (await adminAPI.bi.listReports({ page_size: 100 })).items }
async function loadAudit() { audit.value = (await adminAPI.bi.listAudit({ page_size: 100, ...(auditRequestID.value ? { request_id: auditRequestID.value } : {}) })).items }
async function loadActive() { loading.value = true; error.value = ''; try { if (activeTab.value === 'overview') await loadOverview(); else if (activeTab.value === 'identity') await Promise.all([loadIdentity(), loadChallenges(), loadSessions()]); else if (activeTab.value === 'sources') await loadSources(); else if (activeTab.value === 'imports') await loadImports(); else if (activeTab.value === 'quality') await loadQuality(); else if (activeTab.value === 'reports') await loadReports(); else if (activeTab.value === 'retention') Object.assign(retention, await adminAPI.bi.getRetention()); else await loadAudit() } catch (reason) { showError(reason) } finally { loading.value = false } }
async function revokeBinding(row: BIBindingOperation) { if (!confirmAction(`确认撤销 ${row.display_name} 的全部 BI 会话？`)) return; try { await adminAPI.bi.revokeBinding(row.id); await loadIdentity(); await loadOverview() } catch (reason) { showError(reason) } }
async function issueCredential() { try { issuedToken.value = (await adminAPI.bi.issueCredential({ ...credentialForm, allowed_kinds: credentialForm.allowed_kinds.split(',').map(item => item.trim()).filter(Boolean), expires_at: new Date(credentialForm.expires_at).toISOString() })).token } catch (reason) { showError(reason) } }
async function revokeCredential(id: string) { if (!confirmAction('确认立即撤销该 Connector 凭证？')) return; try { await adminAPI.bi.revokeCredential(id); if (selectedSource.value) await loadCredentials(selectedSource.value) } catch (reason) { showError(reason) } }
async function rotateCredential(id: string) { const expiresAt = window.prompt('输入新的过期时间（ISO 8601）', new Date(Date.now() + 30 * 86400000).toISOString()); if (!expiresAt) return; try { const result = await adminAPI.bi.rotateCredential(id, expiresAt); issuedToken.value = result.token; if (selectedSource.value) await loadCredentials(selectedSource.value) } catch (reason) { showError(reason) } }
async function retryImport(row: BIImportOperation) { if (!confirmAction(`确认重试批次 ${row.id}？原错误记录会保留。`)) return; try { await adminAPI.bi.retryImport(row.id); await loadImports() } catch (reason) { showError(reason) } }
async function retryReport(row: BIReportOperation) { if (!confirmAction(`确认重试报告“${row.title}”？`)) return; try { await adminAPI.bi.retryReport(row.id); await loadReports() } catch (reason) { showError(reason) } }
async function archiveReport(row: BIReportOperation) { if (!confirmAction(`确认归档报告“${row.title}”？`)) return; try { await adminAPI.bi.archiveReport(row.id); await loadReports() } catch (reason) { showError(reason) } }
async function saveRetention() { try { Object.assign(retention, await adminAPI.bi.updateRetention({ ...retention })); appStore.showSuccess('BI 保留策略已保存') } catch (reason) { showError(reason) } }
async function runCleanup() { if (!confirmAction('确认执行 BI 清理？该操作会按当前策略删除过期临时状态、报告和审计数据。')) return; try { const result = await adminAPI.bi.runCleanup(); appStore.showSuccess(`已清理 ${result.deleted} 条记录`) } catch (reason) { showError(reason) } }
watch(activeTab, () => { void loadActive() })
onMounted(() => { void loadActive() })
</script>
