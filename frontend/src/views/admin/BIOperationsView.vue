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
        <div v-if="overview && overview.counts.sources === 0" class="rounded-lg border border-amber-200 bg-amber-50 p-5 text-sm text-amber-900 dark:border-amber-800 dark:bg-amber-950/20 dark:text-amber-200 xl:col-span-2" role="status">
          <h2 class="font-semibold">尚未配置 BI 数据源</h2>
          <p class="mt-2 max-w-3xl">小程序已经可以完成微信登录，但没有数据源就无法创建分析上下文。先为企业签发一个 Connector 凭证，再在“数据源”页开始同步 sub2api 用量。</p>
          <button class="btn btn-primary mt-4" type="button" @click="activeTab = 'sources'">开始配置数据源</button>
        </div>
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
        <p v-if="identityErrors.bindings" class="text-sm text-red-600 dark:text-red-400" role="alert">绑定列表：{{ identityErrors.bindings }} <button class="ml-2 underline" type="button" @click="loadIdentity">重试绑定列表</button></p>
        <p v-if="identityErrors.challenges" class="text-sm text-red-600 dark:text-red-400" role="alert">待确认绑定：{{ identityErrors.challenges }} <button class="ml-2 underline" type="button" @click="loadChallenges">重试待确认绑定</button></p>
        <p v-if="identityErrors.sessions" class="text-sm text-red-600 dark:text-red-400" role="alert">设备/会话：{{ identityErrors.sessions }} <button class="ml-2 underline" type="button" @click="loadSessions">重试设备/会话</button></p>
        <div class="card overflow-hidden">
          <table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">微信身份</th><th class="px-4 py-3">账号</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">最近登录</th><th class="px-4 py-3">会话</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in bindings" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.id }}</td><td class="px-4 py-3">{{ row.display_name }} (#{{ row.user_id }})</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ formatDate(row.last_login_at) }}</td><td class="px-4 py-3">{{ row.session_count }}</td><td class="px-4 py-3"><button v-if="row.status === 'active'" class="btn btn-danger btn-sm" type="button" @click="revokeBinding(row)">强制撤销</button></td></tr><tr v-if="!bindings.length"><td colspan="6" class="px-4 py-8 text-center text-gray-500">暂无绑定记录</td></tr></tbody></table>
          <Pagination v-if="pageState.bindings.total > 0" :page="pageState.bindings.page" :page-size="pageState.bindings.page_size" :total="pageState.bindings.total" :page-size-options="[20, 50, 100]" @update:page="setPage('bindings', $event)" @update:page-size="setPageSize('bindings', $event)" />
        </div>
        <div v-if="challenges.length || pageState.challenges.total" class="card p-4"><h2 class="font-semibold">待确认/过期挑战</h2><div class="mt-3 grid gap-2 text-sm"><div v-for="row in challenges" :key="row.id" class="flex flex-wrap justify-between gap-2 border-b border-gray-100 py-2 last:border-0 dark:border-dark-700"><span class="font-mono text-xs">{{ row.id }}</span><span :class="statusClass(row.status)">{{ row.status }}</span><span>{{ formatDate(row.expires_at) }}</span></div></div><Pagination v-if="pageState.challenges.total > 0" :page="pageState.challenges.page" :page-size="pageState.challenges.page_size" :total="pageState.challenges.total" :page-size-options="[20, 50, 100]" @update:page="setPage('challenges', $event)" @update:page-size="setPageSize('challenges', $event)" /></div>
        <div v-if="sessions.length || pageState.sessions.total" class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">会话</th><th class="px-4 py-3">设备</th><th class="px-4 py-3">客户端</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">过期时间</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in sessions" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.id }}</td><td class="px-4 py-3">{{ row.device_id || '-' }} / {{ row.platform || '-' }}</td><td class="px-4 py-3">{{ row.client_version || '-' }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ formatDate(row.expires_at) }}</td></tr></tbody></table><Pagination v-if="pageState.sessions.total > 0" :page="pageState.sessions.page" :page-size="pageState.sessions.page_size" :total="pageState.sessions.total" :page-size-options="[20, 50, 100]" @update:page="setPage('sessions', $event)" @update:page-size="setPageSize('sessions', $event)" /></div>
      </section>

      <section v-else-if="activeTab === 'sources'" class="space-y-5">
        <p v-if="contextOrganizationID" class="text-sm text-gray-600 dark:text-gray-300">当前企业：<code>{{ contextOrganizationID }}</code></p>
        <section class="card space-y-4 p-5">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">添加 BI 数据源</h2>
              <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">数据源代表一个生产者，凭证只用于后端导入数据，不给小程序使用。sub2api 用量接入只需要允许 <code>usage</code> 类型。</p>
            </div>
            <button class="btn btn-secondary shrink-0" type="button" @click="useSub2apiDefaults">填入 sub2api 推荐值</button>
          </div>
          <form class="grid gap-4 md:grid-cols-2" @submit.prevent="issueCredential">
            <label class="text-sm">企业 ID
              <input v-model="credentialForm.organization_id" class="input mt-1" placeholder="企业 public_id，例如 org_..." autocomplete="off" :readonly="!!contextOrganizationID" required />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">使用 Desktop 企业详情页的 public_id；企业须已建立 BI 授权且处于启用状态。</span>
            </label>
            <label class="text-sm">source ID
              <input v-model="credentialForm.source_id" class="input mt-1" placeholder="稳定的生产者 ID" autocomplete="off" required />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">同一生产者轮换凭证时必须保持不变。</span>
            </label>
            <label class="text-sm">namespace
              <input v-model="credentialForm.namespace" class="input mt-1" placeholder="实体 ID 前缀，不带冒号" autocomplete="off" required />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">sub2api 推荐填写 <code>sub2api</code>。</span>
            </label>
            <label class="text-sm">允许的数据类型
              <input v-model="credentialForm.allowed_kinds" class="input mt-1" placeholder="多个类型用英文逗号分隔" autocomplete="off" required />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">sub2api 用量填写 <code>usage</code>，不要填写未接入的类型。</span>
            </label>
            <label class="text-sm md:col-span-2">凭证过期时间
              <input v-model="credentialForm.expires_at" class="input mt-1 max-w-sm" type="datetime-local" required />
              <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">凭证只在签发成功时显示一次，轮换不会重置 checkpoint。</span>
            </label>
            <button class="btn btn-primary md:col-span-2 md:justify-self-start" type="submit" :disabled="issuing" :aria-busy="issuing">{{ issuing ? '签发中…' : '签发 Connector 凭证' }}</button>
          </form>
        </section>
        <section class="rounded-lg border border-gray-200 bg-gray-50 p-5 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-800/50 dark:text-gray-300">
          <h2 class="font-semibold text-gray-900 dark:text-white">签发后还要完成首次导入</h2>
          <ol class="mt-2 list-decimal space-y-1 pl-5">
            <li>在下方数据源列表点击“开始同步 sub2api 用量”，填写该来源的有效凭证和日志起始时间。</li>
            <li>每次最多导入 500 条；批次变为 <code>applied</code> 后可导入下一批，已导入的记录会自动跳过。此入口按次执行，持续同步仍需配置后端采集任务。</li>
            <li>若显示“没有可导入日志”，需核对时间范围与企业成员 API Key；网关日志缺少的员工、应用、场景和结果信息仍标为未知。</li>
            <li>导入真实记录后，回到小程序点击“重新连接 BI”。</li>
          </ol>
        </section>
        <div v-if="issuedToken" class="card border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 dark:border-amber-700 dark:bg-amber-950/20 dark:text-amber-200"><strong>凭证只显示一次：</strong><code class="ml-2 break-all">{{ issuedToken }}</code></div>
        <section v-if="syncSource" class="card space-y-4 p-5" data-testid="source-sync">
          <h2 class="font-semibold">同步 sub2api 用量：{{ syncSource.organization_id }} / {{ syncSource.source_id }}</h2>
          <p class="text-sm text-gray-500">读取该企业成员 API Key 对应的真实网关用量日志。请选择专用于 sub2api 的来源，不要与外部采集任务共用。</p>
          <form class="grid gap-4 md:grid-cols-2" @submit.prevent="startSourceSync">
            <label class="text-sm">Connector 凭证<input v-model="syncForm.connector_token" class="input mt-1" type="password" autocomplete="off" required :disabled="syncing || syncPending" /></label>
            <label class="text-sm">日志起始时间<input v-model="syncForm.since" class="input mt-1" type="datetime-local" required :disabled="syncing || syncPending" /><span class="mt-1 block text-xs text-gray-500">按当前设备时区选择；继续下一批时保留相同起始时间。</span></label>
            <div class="flex flex-wrap gap-2 md:col-span-2">
              <button class="btn btn-primary" type="submit" :disabled="syncing || syncPending || (!!syncJob && syncJob.status !== 'applied')" :aria-busy="syncing">{{ syncing ? '提交中…' : syncPending ? '批次处理中…' : syncJob?.status === 'applied' ? '导入下一批（最多 500 条）' : '提交同步（最多 500 条）' }}</button>
              <button v-if="syncJobID" class="btn btn-secondary" type="button" :disabled="syncRefreshing" @click="refreshSourceSync">刷新结果</button>
              <button class="btn btn-secondary" type="button" :disabled="syncing" @click="closeSourceSync">关闭</button>
            </div>
          </form>
          <p v-if="syncError" role="alert" class="text-sm text-red-600">{{ syncError }}</p>
          <p v-if="syncMessage" role="status" class="text-sm">{{ syncMessage }}</p>
          <div v-if="syncJob" class="space-y-2 text-sm">
            <p>批次：<code>{{ syncJob.id }}</code> · 状态：<span :class="statusClass(syncJob.status)">{{ syncJob.status }}</span></p>
            <p v-if="syncJob.status === 'applied'" class="text-green-600">本批次已导入。可继续下一批，直到没有可导入日志。</p>
            <pre v-if="syncJob.errors.length" class="max-h-64 overflow-auto whitespace-pre-wrap text-red-600">{{ JSON.stringify(syncJob.errors, null, 2) }}</pre>
          </div>
          <button class="btn btn-secondary btn-sm" type="button" @click="activeTab = 'imports'">查看数据同步列表</button>
        </section>
        <div class="card overflow-x-auto"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">企业</th><th class="px-4 py-3">source</th><th class="px-4 py-3">namespace</th><th class="px-4 py-3">允许类型</th><th class="px-4 py-3">checkpoint</th><th class="px-4 py-3">凭证</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in sources" :key="`${row.organization_id}:${row.source_id}`"><td class="px-4 py-3">{{ row.organization_id }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.source_id }}</td><td class="px-4 py-3">{{ row.namespace }}</td><td class="px-4 py-3">{{ row.allowed_kinds.join(', ') }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.checkpoint || '-' }}</td><td class="px-4 py-3">{{ row.credential_count }}</td><td class="px-4 py-3"><div class="flex flex-wrap gap-2"><button v-if="canSyncSub2api(row)" class="btn btn-primary btn-sm" type="button" :disabled="syncing" @click="openSourceSync(row)">开始同步 sub2api 用量</button><button class="btn btn-secondary btn-sm" type="button" @click="loadCredentials(row)">查看凭证</button></div></td></tr><tr v-if="!sources.length"><td colspan="7" class="px-4 py-8 text-center text-gray-500">{{ contextOrganizationID ? '该企业尚无数据源' : '暂无数据源' }}</td></tr></tbody></table><Pagination v-if="pageState.sources.total > 0" :page="pageState.sources.page" :page-size="pageState.sources.page_size" :total="pageState.sources.total" :page-size-options="[20, 50, 100]" @update:page="setPage('sources', $event)" @update:page-size="setPageSize('sources', $event)" /></div>
        <div v-if="selectedSource" class="card overflow-hidden"><div class="border-b border-gray-100 px-4 py-3 dark:border-dark-700"><h2 class="font-semibold">{{ selectedSource.source_id }} 的 Connector 凭证</h2></div><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">token 前缀</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">过期时间</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in credentials" :key="row.id"><td class="px-4 py-3 font-mono text-xs">{{ row.token_prefix }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ formatDate(row.expires_at) }}</td><td class="flex gap-2 px-4 py-3"><button v-if="row.status === 'active'" class="btn btn-danger btn-sm" type="button" @click="revokeCredential(row.id)">撤销</button><button v-if="row.status === 'active'" class="btn btn-secondary btn-sm" type="button" @click="rotateCredential(row.id)">轮换</button></td></tr><tr v-if="!credentials.length"><td colspan="4" class="px-4 py-8 text-center text-gray-500">暂无凭证</td></tr></tbody></table><Pagination v-if="pageState.credentials.total > 0" :page="pageState.credentials.page" :page-size="pageState.credentials.page_size" :total="pageState.credentials.total" :page-size-options="[20, 50, 100]" @update:page="setPage('credentials', $event)" @update:page-size="setPageSize('credentials', $event)" /></div>
      </section>

      <section v-else-if="activeTab === 'imports'" class="space-y-4"><div class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">批次</th><th class="px-4 py-3">企业/source</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">checkpoint</th><th class="px-4 py-3">attempt</th><th class="px-4 py-3">错误</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in imports" :key="row.id"><td class="px-4 py-3 font-mono text-xs"><button class="text-primary-600 hover:underline" type="button" @click="loadImportDetails(row)">{{ row.id }}</button></td><td class="px-4 py-3">{{ row.organization_id }} / {{ row.source_id }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3 font-mono text-xs">{{ row.checkpoint || '-' }}</td><td class="px-4 py-3">{{ row.attempt_count }}</td><td class="max-w-xs px-4 py-3 text-xs text-red-600">{{ row.errors?.[0]?.message || '-' }}</td><td class="px-4 py-3"><button v-if="row.status === 'failed' || row.status === 'rejected'" class="btn btn-secondary btn-sm" type="button" @click="retryImport(row)">重试</button></td></tr><tr v-if="!imports.length"><td colspan="7" class="px-4 py-8 text-center text-gray-500">暂无导入批次</td></tr></tbody></table><Pagination v-if="pageState.imports.total > 0" :page="pageState.imports.page" :page-size="pageState.imports.page_size" :total="pageState.imports.total" :page-size-options="[20, 50, 100]" @update:page="setPage('imports', $event)" @update:page-size="setPageSize('imports', $event)" /></div><div v-if="selectedImport" class="card p-4"><h2 class="font-semibold">导入批次详情</h2><dl class="mt-3 grid gap-2 text-sm sm:grid-cols-2"><div><dt class="text-gray-500">状态</dt><dd>{{ selectedImport.status }}</dd></div><div><dt class="text-gray-500">data revision</dt><dd class="font-mono">{{ selectedImport.data_revision || '-' }}</dd></div><div class="sm:col-span-2"><dt class="text-gray-500">全部错误</dt><dd><pre class="mt-1 max-h-80 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 text-xs dark:bg-dark-800">{{ JSON.stringify(selectedImport.errors || [], null, 2) }}</pre></dd></div></dl></div></section>

      <section v-else-if="activeTab === 'quality'" class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">企业/source</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">覆盖范围</th><th class="px-4 py-3">延迟</th><th class="px-4 py-3">缺失维度</th><th class="px-4 py-3">原因</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in quality" :key="`${row.organization_id}:${row.source_id}`"><td class="px-4 py-3">{{ row.organization_id }} / {{ row.source_id }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ row.coverage_start || '-' }} ~ {{ row.coverage_end || '-' }}</td><td class="px-4 py-3">{{ row.latency_seconds == null ? '-' : `${Math.round(row.latency_seconds)} 秒` }}</td><td class="px-4 py-3 text-xs">{{ row.missing_dimensions?.length ? row.missing_dimensions.join(', ') : '-' }}</td><td class="px-4 py-3 text-xs text-gray-500">{{ row.reason || '-' }}</td></tr><tr v-if="!quality.length"><td colspan="6" class="px-4 py-8 text-center text-gray-500">暂无质量数据</td></tr></tbody></table></section>

      <section v-else-if="activeTab === 'reports'" class="space-y-4"><div class="card overflow-hidden"><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">报告</th><th class="px-4 py-3">企业</th><th class="px-4 py-3">状态</th><th class="px-4 py-3">尝试</th><th class="px-4 py-3">失败详情</th><th class="px-4 py-3">操作</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in reports" :key="row.id"><td class="px-4 py-3"><button class="text-left text-primary-600 hover:underline" type="button" @click="loadReportDetails(row)"><div>{{ row.title }}</div><code class="text-xs text-gray-500">{{ row.id }}</code></button></td><td class="px-4 py-3">{{ row.organization_id }}</td><td class="px-4 py-3"><span :class="statusClass(row.status)">{{ row.status }}</span></td><td class="px-4 py-3">{{ row.attempt_count }}</td><td class="px-4 py-3 text-xs text-red-600">{{ row.failure_code || '-' }}</td><td class="flex gap-2 px-4 py-3"><button v-if="row.status === 'failed' || row.status === 'lease_timeout'" class="btn btn-secondary btn-sm" type="button" @click="retryReport(row)">重试</button><button v-if="row.status !== 'archived'" class="btn btn-danger btn-sm" type="button" @click="archiveReport(row)">归档</button></td></tr><tr v-if="!reports.length"><td colspan="6" class="px-4 py-8 text-center text-gray-500">暂无报告任务</td></tr></tbody></table><Pagination v-if="pageState.reports.total > 0" :page="pageState.reports.page" :page-size="pageState.reports.page_size" :total="pageState.reports.total" :page-size-options="[20, 50, 100]" @update:page="setPage('reports', $event)" @update:page-size="setPageSize('reports', $event)" /></div><div v-if="selectedReport" class="card p-4"><h2 class="font-semibold">报告详情</h2><dl class="mt-3 grid gap-2 text-sm sm:grid-cols-2"><div><dt class="text-gray-500">状态</dt><dd>{{ selectedReport.status }}</dd></div><div><dt class="text-gray-500">生成时间</dt><dd>{{ formatDate(selectedReport.generated_at) }}</dd></div><div><dt class="text-gray-500">失败代码</dt><dd>{{ selectedReport.failure_code || '-' }}</dd></div><div><dt class="text-gray-500">重试来源</dt><dd class="font-mono">{{ selectedReport.retry_of || '-' }}</dd></div></dl></div></section>

      <section v-else-if="activeTab === 'retention'" class="space-y-4"><div class="card max-w-2xl p-5"><h2 class="text-base font-semibold">清理与保留策略</h2><form class="mt-5 grid gap-4 sm:grid-cols-2" @submit.prevent="saveRetention"><label class="text-sm">报告保留（月）<input v-model.number="retention.report_months" class="input mt-1" type="number" min="1" max="120" /></label><label class="text-sm">业务事实保留（月）<input v-model.number="retention.fact_months" class="input mt-1" type="number" min="1" max="120" /></label><label class="text-sm">审计保留（天）<input v-model.number="retention.audit_days" class="input mt-1" type="number" min="1" max="3650" /></label><label class="text-sm">临时状态保留（天）<input v-model.number="retention.ephemeral_days" class="input mt-1" type="number" min="1" max="365" /></label><div class="flex flex-wrap gap-2 sm:col-span-2"><button class="btn btn-primary" type="submit">保存策略</button><button class="btn btn-danger" type="button" @click="runCleanup">执行清理</button></div></form></div><div v-if="cleanupStatus" class="card p-4 text-sm"><div>上次清理：{{ formatDate(cleanupStatus.last_run_at) }}</div><div class="mt-1">删除记录：{{ cleanupStatus.last_deleted ?? 0 }}</div></div></section>

      <section v-else-if="activeTab === 'audit'" class="card overflow-hidden"><div class="border-b border-gray-100 p-4 dark:border-dark-700"><input v-model="auditRequestID" class="input max-w-md" placeholder="按 request_id 查询" @keyup.enter="loadAudit" /><button class="btn btn-secondary ml-2" type="button" @click="loadAudit">查询</button></div><table class="min-w-full text-left text-sm"><thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">时间</th><th class="px-4 py-3">动作</th><th class="px-4 py-3">资源</th><th class="px-4 py-3">request_id</th><th class="px-4 py-3">管理员</th></tr></thead><tbody class="divide-y divide-gray-100 dark:divide-dark-700"><tr v-for="row in audit" :key="row.id"><td class="px-4 py-3">{{ formatDate(row.created_at) }}</td><td class="px-4 py-3">{{ row.action }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.target_id }}</td><td class="px-4 py-3 font-mono text-xs">{{ row.request_id }}</td><td class="px-4 py-3">{{ row.actor_user_id || '-' }}</td></tr><tr v-if="!audit.length"><td colspan="5" class="px-4 py-8 text-center text-gray-500">暂无审计事件</td></tr></tbody></table><Pagination v-if="pageState.audit.total > 0" :page="pageState.audit.page" :page-size="pageState.audit.page_size" :total="pageState.audit.total" :page-size-options="[20, 50, 100]" @update:page="setPage('audit', $event)" @update:page-size="setPageSize('audit', $event)" /></section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { BIAuditEvent, BIBindingOperation, BIChallengeOperation, BICredentialOperation, BIImportOperation, BIQualityOperation, BIReportOperation, BISessionOperation, BISourceOperation, BIOperationsOverview, BIRetentionPolicy } from '@/api/admin/bi'
import { useAppStore } from '@/stores'
import { extractApiErrorCode, extractApiErrorMessage, extractApiErrorMetadata } from '@/utils/apiError'
import Pagination from '@/components/common/Pagination.vue'

type Tab = 'overview' | 'identity' | 'sources' | 'imports' | 'quality' | 'reports' | 'retention' | 'audit'
const route = useRoute()
const contextOrganizationID = computed(() => typeof route.query.organization_id === 'string' ? route.query.organization_id : '')
const appStore = useAppStore()
const activeTab = ref<Tab>(route.query.tab === 'sources' ? 'sources' : 'overview')
const loading = ref(false)
const error = ref('')
const identityErrors = reactive({ bindings: '', challenges: '', sessions: '' })
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
const selectedImport = ref<BIImportOperation | null>(null)
const selectedReport = ref<BIReportOperation | null>(null)
const cleanupStatus = ref<{ last_run_at?: string | null; last_deleted?: number; retention: BIRetentionPolicy } | null>(null)
const retention = reactive<BIRetentionPolicy>({ report_months: 24, fact_months: 24, audit_days: 180, ephemeral_days: 7 })
const identityFilter = ref('all')
const auditRequestID = ref('')
const issuedToken = ref('')
const issuing = ref(false)
const issuedTokenSource = ref<{ organization_id: string; source_id: string } | null>(null)
const syncSource = ref<BISourceOperation | null>(null)
const syncForm = reactive({ connector_token: '', since: localDateTimeAfter(-24 * 30) })
const syncing = ref(false)
const syncRefreshing = ref(false)
const syncJobID = ref('')
const syncJob = ref<BIImportOperation | null>(null)
const syncError = ref('')
const syncMessage = ref('')
const syncErrorMessages = {
  INVALID_CONNECTOR_CREDENTIAL: '凭证无效、已过期或已撤销，请签发或轮换该来源的凭证后重试。',
  FORBIDDEN: '凭证须属于所选企业和数据源，并允许 usage 类型。',
  IMPORT_BUSY: '该来源已有批次正在处理，请在“数据同步”中等待其完成后重试。',
  CHECKPOINT_CONFLICT: '来源进度已更新，请查看现有批次结果后重试。',
  CONFLICT: '该来源已经声明完整数据水位，请使用原采集任务继续同步。',
}
const syncPending = computed(() => !!syncJobID.value && (!syncJob.value || ['queued', 'validating', 'processing'].includes(syncJob.value.status)))
let syncTimer: ReturnType<typeof setTimeout> | undefined
let syncGeneration = 0
let syncPolls = 0
function canSyncSub2api(row: BISourceOperation) { return row.allowed_kinds.includes('usage') && !row.complete_through && !row.initial_backfill_complete && row.namespace.length <= 82 }
function stopSyncPolling() { clearTimeout(syncTimer); syncTimer = undefined }
function closeSourceSync() {
  stopSyncPolling()
  syncGeneration++
  syncSource.value = null
  syncForm.connector_token = ''
  syncJobID.value = ''
  syncJob.value = null
  syncRefreshing.value = false
}
function openSourceSync(row: BISourceOperation) {
  closeSourceSync()
  syncSource.value = row
  syncError.value = ''
  syncMessage.value = ''
  if (issuedTokenSource.value?.organization_id === row.organization_id && issuedTokenSource.value?.source_id === row.source_id) syncForm.connector_token = issuedToken.value
}
async function refreshSourceSync() {
  if (!syncJobID.value || syncRefreshing.value) return
  stopSyncPolling()
  const generation = syncGeneration
  syncRefreshing.value = true
  try {
    const job = await adminAPI.bi.getImport(syncJobID.value)
    if (generation !== syncGeneration) return
    syncJob.value = job
    syncError.value = ''
    if (!syncPending.value) syncMessage.value = ''
    if (syncPending.value && ++syncPolls < 20 && activeTab.value === 'sources') syncTimer = setTimeout(() => { void refreshSourceSync() }, 3000)
    else if (syncPending.value) syncMessage.value = '批次仍在处理，请点击“刷新结果”查看进度。'
  } catch (reason) { if (generation === syncGeneration) syncError.value = formatError(reason, syncErrorMessages) }
  finally { if (generation === syncGeneration) syncRefreshing.value = false }
}
async function startSourceSync() {
  if (!syncSource.value || syncing.value || syncPending.value || (syncJob.value && syncJob.value.status !== 'applied')) return
  syncing.value = true
  syncError.value = ''
  syncMessage.value = ''
  const generation = syncGeneration
  try {
    const since = new Date(syncForm.since)
    if (!Number.isFinite(since.getTime()) || since.getTime() > Date.now()) { syncError.value = '请选择有效的过去时间'; return }
    const result = await adminAPI.bi.syncSub2api(syncSource.value.source_id, { organization_id: syncSource.value.organization_id, connector_token: syncForm.connector_token.trim(), since: since.toISOString(), limit: 500 })
    if (generation !== syncGeneration) return
    syncJob.value = null
    syncJobID.value = result.job?.id || ''
    syncPolls = 0
    syncMessage.value = result.status === 'idle' ? '该时间范围没有可导入日志。请核对起始时间和企业成员 API Key；已导入的记录不会重复导入。' : `已提交 ${result.records} 条记录，正在处理。`
    if (syncJobID.value) await refreshSourceSync()
  } catch (reason) { if (generation === syncGeneration) syncError.value = formatError(reason, syncErrorMessages) }
  finally { syncing.value = false }
}
function localDateTimeAfter(hours: number) {
  const date = new Date(Date.now() + hours * 60 * 60 * 1000)
  date.setMinutes(date.getMinutes() - date.getTimezoneOffset())
  return date.toISOString().slice(0, 16)
}
const credentialForm = reactive({ organization_id: contextOrganizationID.value, source_id: '', namespace: '', allowed_kinds: '', expires_at: localDateTimeAfter(24) })
type PageKey = 'bindings' | 'challenges' | 'sessions' | 'sources' | 'credentials' | 'imports' | 'reports' | 'audit'
type PageState = { page: number; page_size: number; total: number }
const pageState = reactive<Record<PageKey, PageState>>({
  bindings: { page: 1, page_size: 20, total: 0 },
  challenges: { page: 1, page_size: 20, total: 0 },
  sessions: { page: 1, page_size: 20, total: 0 },
  sources: { page: 1, page_size: 20, total: 0 },
  credentials: { page: 1, page_size: 20, total: 0 },
  imports: { page: 1, page_size: 20, total: 0 },
  reports: { page: 1, page_size: 20, total: 0 },
  audit: { page: 1, page_size: 20, total: 0 },
})
const tabs = [{ key: 'overview', label: '总览' }, { key: 'identity', label: '微信身份' }, { key: 'sources', label: '数据源' }, { key: 'imports', label: '数据同步' }, { key: 'quality', label: '数据质量' }, { key: 'reports', label: '报告任务' }, { key: 'retention', label: '清理与保留' }, { key: 'audit', label: '审计' }] as const

const overviewCards = computed(() => overview.value ? [
  { key: 'bindings', label: '绑定', value: overview.value.counts.bindings || 0 }, { key: 'pending', label: '待确认绑定', value: overview.value.counts.pending_challenges || 0 }, { key: 'sessions', label: '活跃会话', value: overview.value.counts.active_sessions || 0 }, { key: 'sources', label: '数据源', value: overview.value.counts.sources || 0 }, { key: 'imports', label: '处理中批次', value: overview.value.counts.imports_processing || 0 }, { key: 'reports', label: '生成中报告', value: overview.value.counts.reports_processing || 0 }, { key: 'enabled', label: 'BI 服务', value: overview.value.config.enabled ? 'ON' : 'OFF' },
] : [])
const configuredSecrets = computed(() => overview.value ? `${overview.value.config.app_secret_configured ? 'AppSecret' : ''} ${overview.value.config.jwt_secret_configured ? 'JWT' : ''} ${overview.value.config.identity_secret_configured ? 'Identity' : ''}`.trim() || '未配置' : '未加载')

function formatDate(value: string | null | undefined) { return value ? new Date(value).toLocaleString() : '-' }
function statusClass(status: string) { return status === 'ready' || status === 'active' || status === 'applied' ? 'text-green-600 dark:text-green-400' : status === 'failed' || status === 'rejected' || status === 'revoked' || status === 'expired' || status === 'unknown' ? 'text-red-600 dark:text-red-400' : 'text-amber-600 dark:text-amber-400' }
function filterLabel(value: string) { return value === 'all' ? '全部' : value === 'active' ? '有效' : '已撤销' }
function confirmAction(message: string) { return window.confirm(message) }
function formatError(reason: unknown, messages?: Record<string, string>): string {
  const message = extractApiErrorCode(reason) === 'INTERNAL_ERROR'
    ? 'BI 服务异常，请稍后重试'
    : extractApiErrorMessage(reason, '操作失败，请稍后重试', messages)
  const requestID = extractApiErrorMetadata(reason)?.request_id
  return typeof requestID === 'string' && requestID ? `${message}（请求 ID：${requestID}）` : message
}
function showError(reason: unknown) { error.value = formatError(reason) }
function useSub2apiDefaults() {
  credentialForm.source_id = 'source_sub2api'
  credentialForm.namespace = 'sub2api'
  credentialForm.allowed_kinds = 'usage'
}
async function loadOverview() { overview.value = await adminAPI.bi.getOverview(); Object.assign(retention, overview.value.retention) }
function setPage(key: PageKey, page: number) { pageState[key].page = page; void loadPage(key) }
function setPageSize(key: PageKey, pageSize: number) { pageState[key].page_size = pageSize; pageState[key].page = 1; void loadPage(key) }
async function loadPage(key: PageKey) {
  try {
    if (key === 'bindings') await loadIdentity()
    else if (key === 'challenges') await loadChallenges()
    else if (key === 'sessions') await loadSessions()
    else if (key === 'sources') await loadSources()
    else if (key === 'credentials' && selectedSource.value) await loadCredentials(selectedSource.value)
    else if (key === 'imports') await loadImports()
    else if (key === 'reports') await loadReports()
    else if (key === 'audit') await loadAudit()
  } catch (reason) { showError(reason) }
}
function applyPage(key: PageKey, result: { page: number; page_size: number; total: number }) { pageState[key].page = result.page; pageState[key].page_size = result.page_size; pageState[key].total = result.total }
async function loadIdentity() {
  identityErrors.bindings = ''
  try {
    const result = await adminAPI.bi.listBindings({ ...(identityFilter.value === 'all' ? {} : { status: identityFilter.value }), page: pageState.bindings.page, page_size: pageState.bindings.page_size })
    bindings.value = result.items
    applyPage('bindings', result)
  } catch (reason) { identityErrors.bindings = formatError(reason) }
}
async function loadChallenges() {
  identityErrors.challenges = ''
  try {
    const result = await adminAPI.bi.listChallenges({ page: pageState.challenges.page, page_size: pageState.challenges.page_size })
    challenges.value = result.items
    applyPage('challenges', result)
  } catch (reason) { identityErrors.challenges = formatError(reason) }
}
async function loadSessions() {
  identityErrors.sessions = ''
  try {
    const result = await adminAPI.bi.listSessions({ page: pageState.sessions.page, page_size: pageState.sessions.page_size })
    sessions.value = result.items
    applyPage('sessions', result)
  } catch (reason) { identityErrors.sessions = formatError(reason) }
}
async function loadSources() { const result = await adminAPI.bi.listSources({ page: pageState.sources.page, page_size: pageState.sources.page_size, ...(contextOrganizationID.value ? { organization_id: contextOrganizationID.value } : {}) }); sources.value = result.items; applyPage('sources', result) }
async function loadCredentials(row: BISourceOperation) { try { selectedSource.value = row; const result = await adminAPI.bi.listCredentials(row.source_id, row.organization_id, { page: pageState.credentials.page, page_size: pageState.credentials.page_size }); credentials.value = result.items; applyPage('credentials', result) } catch (reason) { showError(reason) } }
async function loadImports() { const result = await adminAPI.bi.listImports({ page: pageState.imports.page, page_size: pageState.imports.page_size }); imports.value = result.items; applyPage('imports', result) }
async function loadQuality() { quality.value = (await adminAPI.bi.listQuality({ organization_id: credentialForm.organization_id || undefined })).items }
async function loadReports() { const result = await adminAPI.bi.listReports({ page: pageState.reports.page, page_size: pageState.reports.page_size }); reports.value = result.items; applyPage('reports', result) }
async function loadAudit() {
  error.value = ''
  try {
    const result = await adminAPI.bi.listAudit({ page: pageState.audit.page, page_size: pageState.audit.page_size, ...(auditRequestID.value ? { request_id: auditRequestID.value } : {}) })
    audit.value = result.items
    applyPage('audit', result)
  } catch (reason) { showError(reason) }
}
async function loadImportDetails(row: BIImportOperation) { try { selectedImport.value = await adminAPI.bi.getImport(row.id) } catch (reason) { showError(reason) } }
async function loadReportDetails(row: BIReportOperation) { try { selectedReport.value = await adminAPI.bi.getReport(row.id) } catch (reason) { showError(reason) } }
async function loadActive() { loading.value = true; error.value = ''; try { if (activeTab.value === 'overview') await loadOverview(); else if (activeTab.value === 'identity') await Promise.all([loadIdentity(), loadChallenges(), loadSessions()]); else if (activeTab.value === 'sources') await loadSources(); else if (activeTab.value === 'imports') await loadImports(); else if (activeTab.value === 'quality') await loadQuality(); else if (activeTab.value === 'reports') await loadReports(); else if (activeTab.value === 'retention') { Object.assign(retention, await adminAPI.bi.getRetention()); cleanupStatus.value = await adminAPI.bi.getCleanup() } else await loadAudit() } catch (reason) { showError(reason) } finally { loading.value = false } }
async function revokeBinding(row: BIBindingOperation) { if (!confirmAction(`确认撤销 ${row.display_name} 的全部 BI 会话？`)) return; try { await adminAPI.bi.revokeBinding(row.id); await loadIdentity(); await loadOverview() } catch (reason) { showError(reason) } }
async function issueCredential() {
  if (issuing.value) return
  issuing.value = true
  try {
    error.value = ''
    const input = { ...credentialForm, allowed_kinds: credentialForm.allowed_kinds.split(',').map(item => item.trim()).filter(Boolean), expires_at: new Date(credentialForm.expires_at).toISOString() }
    issuedToken.value = (await adminAPI.bi.issueCredential(input)).token
    issuedTokenSource.value = { organization_id: input.organization_id, source_id: input.source_id }
  } catch (reason) { showError(reason); return } finally { issuing.value = false }
  try { await Promise.all([loadOverview(), loadSources()]) } catch (reason) { showError(reason) }
}
async function revokeCredential(id: string) { if (!confirmAction('确认立即撤销该 Connector 凭证？')) return; try { await adminAPI.bi.revokeCredential(id); if (selectedSource.value) await loadCredentials(selectedSource.value) } catch (reason) { showError(reason) } }
async function rotateCredential(id: string) { const expiresAt = window.prompt('输入新的过期时间（ISO 8601）', new Date(Date.now() + 30 * 86400000).toISOString()); if (!expiresAt) return; try { const result = await adminAPI.bi.rotateCredential(id, expiresAt); issuedToken.value = result.token; issuedTokenSource.value = selectedSource.value ? { organization_id: selectedSource.value.organization_id, source_id: selectedSource.value.source_id } : null; if (selectedSource.value) await loadCredentials(selectedSource.value) } catch (reason) { showError(reason) } }
async function retryImport(row: BIImportOperation) { if (!confirmAction(`确认重试批次 ${row.id}？原错误记录会保留。`)) return; try { await adminAPI.bi.retryImport(row.id); await loadImports() } catch (reason) { showError(reason) } }
async function retryReport(row: BIReportOperation) { if (!confirmAction(`确认重试报告“${row.title}”？`)) return; try { await adminAPI.bi.retryReport(row.id); await loadReports() } catch (reason) { showError(reason) } }
async function archiveReport(row: BIReportOperation) { if (!confirmAction(`确认归档报告“${row.title}”？`)) return; try { await adminAPI.bi.archiveReport(row.id); await loadReports() } catch (reason) { showError(reason) } }
async function saveRetention() { try { Object.assign(retention, await adminAPI.bi.updateRetention({ ...retention })); appStore.showSuccess('BI 保留策略已保存') } catch (reason) { showError(reason) } }
async function runCleanup() { if (!confirmAction('确认执行 BI 清理？该操作会按当前策略删除过期临时状态、报告和审计数据。')) return; try { const result = await adminAPI.bi.runCleanup(); cleanupStatus.value = await adminAPI.bi.getCleanup(); appStore.showSuccess(`已清理 ${result.deleted} 条记录`) } catch (reason) { showError(reason) } }
watch(activeTab, () => { stopSyncPolling(); void loadActive(); if (activeTab.value === 'sources' && syncPending.value) { syncPolls = 0; void refreshSourceSync() } })
watch(() => [route.query.tab, route.query.organization_id], () => {
  closeSourceSync()
  credentialForm.organization_id = contextOrganizationID.value
  selectedSource.value = null
  pageState.sources.page = 1
  const nextTab: Tab = route.query.tab === 'sources' ? 'sources' : 'overview'
  if (activeTab.value === nextTab) void loadActive()
  else activeTab.value = nextTab
})
onBeforeUnmount(closeSourceSync)
onMounted(() => { void loadActive() })
</script>
