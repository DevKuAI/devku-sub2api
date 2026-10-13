<template>
  <AppLayout>
    <div class="desktop-detail min-w-0 space-y-6">
      <div class="flex flex-wrap items-start gap-3">
        <button v-if="!selfManaged" class="btn btn-secondary btn-icon" type="button" :title="t('common.back')" :aria-label="t('common.back')" @click="router.push('/admin/desktop/organizations')">
          <Icon name="arrowLeft" size="md" />
        </button>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-3">
            <h1 class="break-words text-balance text-xl font-semibold text-gray-900 dark:text-white">{{ organization?.name || t(organizationError ? 'admin.desktop.detailTitle' : 'common.loading') }}</h1>
            <StatusBadge v-if="organization" :status="organization.status" :label="statusLabel(organization.status)" />
          </div>
          <p v-if="organization" class="mt-1 break-all font-mono text-sm text-gray-500 dark:text-dark-400">{{ organization.code }}</p>
        </div>
        <router-link v-if="organization" class="btn btn-secondary" :to="selfManaged ? '/organization/accounting' : { path: '/admin/accounting', query: { organization_id: organizationID } }">{{ t('accounting.customerTitle') }}</router-link>
        <button v-if="organization && !selfManaged" class="btn btn-secondary shrink-0" type="button" :title="t('common.edit')" :aria-label="t('common.edit')" @click="openEditOrganization">
          <Icon name="edit" size="sm" /><span>{{ t('common.edit') }}</span>
        </button>
      </div>
      <div v-if="organizationError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300" role="alert">
        {{ t('admin.desktop.organizationLoadFailed') }}
        <button class="ml-2 font-medium underline" type="button" @click="retryOrganization">{{ t('admin.desktop.retryOrganization') }}</button>
      </div>

      <p v-if="organizationLoading && !organization" class="card detail-panel detail-caption" role="status">{{ t('admin.desktop.detail.loadingOrganization') }}</p>
      <section v-if="organization" class="card detail-overview space-y-4" data-testid="organization-overview" :aria-label="t('admin.desktop.organizationForm.overview')">
        <dl class="detail-overview-grid">
          <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.gatewayUser') }}</dt><dd class="mt-1 break-words text-sm font-medium">{{ organization.gateway_user.username || organization.gateway_user.email }}</dd><dd class="break-all text-xs text-gray-500 dark:text-dark-400">{{ organization.gateway_user.email }}</dd></div>
          <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.group') }}</dt><dd class="mt-1 break-words text-sm font-medium">{{ organization.group.name }}</dd></div>
          <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.memberCapacity') }}</dt><dd class="mt-1 text-lg font-semibold tabular-nums" data-testid="organization-member-capacity">{{ organization.member_count }} / {{ organization.member_limit }}</dd><dd class="detail-caption mt-1" :class="{ 'text-amber-700 dark:text-amber-300': memberLimitReached }">{{ t('admin.desktop.organizationForm.remainingSlots', { count: Math.max(0, organization.member_limit - organization.member_count) }) }}</dd></div>
          <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.organizationForm.clientConfiguration') }}</dt><dd class="mt-1"><span class="badge" :class="organization.target_config_assigned ? 'badge-success' : 'badge-warning'">{{ organization.target_config_assigned ? t('admin.desktop.configured') : t('admin.desktop.notConfigured') }}</span></dd></div>
        </dl>
        <div class="space-y-4 border-t border-gray-100 pt-4 dark:border-dark-700">
          <dl class="detail-overview-grid">
            <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.conversationReporting') }}</dt><dd class="mt-1"><StatusBadge :status="organization.conversation_reporting_enabled ? 'active' : 'disabled'" :label="t(organization.conversation_reporting_enabled ? 'common.enabled' : 'common.disabled')" /></dd></div>
            <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.reports.title') }}</dt><dd class="mt-1"><StatusBadge :status="organization.conversation_summary_enabled ? 'active' : 'disabled'" :label="t(organization.conversation_summary_enabled ? 'common.enabled' : 'common.disabled')" /></dd></div>
            <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.reports.model') }}</dt><dd class="mt-1 break-all text-sm font-medium" data-testid="organization-analysis-model"><bdi>{{ organization.analysis_model || t('admin.desktop.notConfigured') }}</bdi></dd></div>
            <div class="min-w-0"><dt class="detail-caption">{{ t('admin.desktop.managementPolicy.analysisLimit') }}</dt><dd class="mt-1 text-sm font-medium" data-testid="organization-analysis-limit">{{ analysisLimitLabel }}</dd></div>
          </dl>
          <dl v-if="organization.management_policy" class="flex flex-wrap gap-x-6 gap-y-2 text-xs text-gray-500 dark:text-dark-400">
            <div v-for="field in retentionFields" :key="field" class="flex min-w-0 flex-wrap gap-x-1"><dt>{{ t(`admin.desktop.managementPolicy.${field}`) }}:</dt><dd>{{ retentionLabel(organization.management_policy[field]) }}</dd></div>
          </dl>
        </div>
      </section>

      <div v-if="organization" class="detail-tabs-shell">
        <nav ref="tabScroll" class="detail-tabs-scroll" role="tablist" :aria-label="t('admin.desktop.detailTabs')">
          <div class="detail-tabs">
            <button v-for="(tab, index) in tabs" :id="tabId(tab.value)" :key="tab.value" class="detail-tab" :class="{ 'detail-tab-active': activeTab === tab.value }" type="button" role="tab" :aria-controls="panelId(tab.value)" :aria-selected="activeTab === tab.value" :tabindex="activeTab === tab.value ? 0 : -1" @click="setTab(tab.value)" @keydown="handleTabKeydown($event, index)">
              <span class="detail-tab-icon" aria-hidden="true"><Icon :name="tabIcons[tab.value]" size="sm" /></span>
              <span class="detail-tab-label">{{ tab.label }}</span>
            </button>
          </div>
        </nav>
      </div>

      <section v-if="organization && activeTab === 'usage'" :id="panelId('usage')" class="min-w-0 space-y-6" tabindex="0" role="tabpanel" :aria-labelledby="tabId('usage')">
        <DesktopAnalyticsRangePicker v-model="analyticsRange" />
        <DesktopOrganizationUsageStatistics v-if="organization" :organization-id="organizationID" :self-managed="selfManaged" :range="analyticsRange" view="insights" />
        <div v-if="organization?.conversation_reporting_enabled" class="border-t border-gray-200 pt-6 dark:border-dark-700">
          <DesktopConversationStatistics :organization-id="organizationID" :self-managed="selfManaged" :filters="unfilteredConversationFilters" :range="analyticsRange" view="usage" />
        </div>
      </section>

      <section v-else-if="organization && activeTab === 'members'" :id="panelId('members')" class="min-w-0 space-y-4" tabindex="0" role="tabpanel" :aria-labelledby="tabId('members')">
        <DesktopOrganizationUsageStatistics v-if="organization" :organization-id="organizationID" :self-managed="selfManaged" />
        <header class="detail-tab-heading"><div><h2 class="detail-section-title">{{ t('admin.desktop.members') }}</h2><p class="detail-caption mt-1">{{ t('admin.desktop.detail.membersHint') }}</p></div></header>
        <div v-if="organization" class="detail-toolbar">
          <label class="w-full min-w-0 space-y-1 sm:max-w-72 sm:flex-1"><span class="block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('admin.desktop.memberSearchLabel') }}</span><input v-model="memberSearch" class="input" type="search" :placeholder="t('admin.desktop.searchMembers')" @input="scheduleMembers" /></label>
          <div class="w-40 space-y-1"><span class="block text-xs font-medium text-gray-600 dark:text-gray-300">{{ t('common.status') }}</span><Select dropdown-class="desktop-detail-select" v-model="memberStatus" :options="statusOptions" :aria-label="t('common.status')" @change="resetMembers" /></div>
          <div class="ms-auto flex w-full flex-wrap items-center gap-3 sm:w-auto">
            <button class="btn btn-secondary shrink-0 whitespace-nowrap" type="button" data-testid="import-members" :disabled="!canCreateMember" @click="showMemberImport = true"><Icon name="upload" size="md" class="mr-1" />{{ t('admin.desktop.memberImport.button') }}</button>
            <button class="btn btn-secondary shrink-0 whitespace-nowrap" type="button" data-testid="export-member-usage" :disabled="membersExporting || !organization || organization.public_id !== organizationID" :aria-busy="membersExporting" :title="t('admin.desktop.memberUsageExport.hint')" @click="exportMemberUsage">
              <Icon :name="membersExporting ? 'refresh' : 'download'" size="md" class="mr-1" :class="{ 'animate-spin': membersExporting }" />{{ t(membersExporting ? 'admin.desktop.memberUsageExport.exporting' : 'admin.desktop.memberUsageExport.button') }}
            </button>
            <button class="btn btn-secondary" type="button" :disabled="membersLoading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadMembers"><Icon name="refresh" size="md" :class="membersLoading ? 'animate-spin' : ''" /></button>
            <button class="btn btn-primary w-full shrink-0 whitespace-nowrap sm:w-auto" type="button" :disabled="!canCreateMember" @click="openCreateMember"><Icon name="plus" size="md" class="mr-1" />{{ t('admin.desktop.createMember') }}</button>
          </div>
        </div>
        <p v-if="organization && (organization.status !== 'active' || memberLimitReached)" class="text-sm text-amber-700 dark:text-amber-300" role="status">{{ t(organization.status !== 'active' ? 'admin.desktop.errors.ORGANIZATION_DISABLED' : 'admin.desktop.errors.MEMBER_LIMIT_REACHED') }}</p>
        <div v-if="membersError" class="detail-error" role="alert"><p>{{ membersError }}</p><button class="btn btn-secondary mt-3" type="button" :disabled="membersLoading" @click="loadMembers">{{ t('admin.desktop.conversations.retry') }}</button></div>
        <p class="detail-caption" role="status">{{ membersLoading ? t('common.loading') : '' }}</p>
        <div v-if="organization && !membersError" class="min-w-0 space-y-3">
          <p class="detail-table-hint">{{ t('admin.desktop.detail.tableScrollHint') }}</p>
          <div v-if="members.some(member => !member.usage) && !membersLoading" class="detail-warning flex flex-wrap items-center justify-between gap-3" role="status"><span>{{ t('admin.desktop.usageUnavailableHint') }}</span><button class="btn btn-secondary" type="button" @click="loadMembers">{{ t('common.refresh') }}</button></div>
          <DataTable class="card overflow-hidden" tabindex="0" :aria-label="t('admin.desktop.members')" :columns="memberColumns" :data="members" :loading="membersLoading" row-key="public_id">
            <template #cell-name="{ row }"><div class="min-w-0 max-w-64"><div class="break-words font-medium text-gray-900 dark:text-white">{{ row.name }}</div><div class="mt-1 break-all font-mono text-xs text-gray-500 dark:text-dark-400">{{ row.public_id }}</div></div></template>
            <template #cell-remark="{ row }"><span class="block max-w-64 whitespace-pre-wrap break-words">{{ row.remark || '—' }}</span></template>
            <template #cell-status="{ row }"><StatusBadge :status="row.status" :label="statusLabel(row.status)" /></template>
            <template #cell-model_token_status="{ row }"><span :class="['badge', tokenBadge(row.model_token_status)]">{{ tokenStatusLabel(row.model_token_status) }}</span><span v-if="budgetWarning(row)" class="mt-1 block text-xs text-amber-700 dark:text-amber-300">{{ t(budgetWarning(row)) }}</span></template>
            <template #cell-usage_cost="{ row }">
              <div class="min-w-36 space-y-1 text-sm tabular-nums">
                <div class="grid grid-cols-[max-content_minmax(0,1fr)] items-baseline gap-x-2">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageToday') }}:</span>
                  <span class="text-right font-medium text-gray-900 dark:text-white">{{ formatUsageCost(row.usage?.today_actual_cost) }}</span>
                </div>
                <div class="grid grid-cols-[max-content_minmax(0,1fr)] items-baseline gap-x-2">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageLast30Days') }}:</span>
                  <span class="text-right font-medium text-gray-900 dark:text-white">{{ formatUsageCost(row.usage?.last_30_days_actual_cost) }}</span>
                </div>
                <div class="grid grid-cols-[max-content_minmax(0,1fr)] items-baseline gap-x-2">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageTotal') }}:</span>
                  <span class="text-right font-medium text-gray-900 dark:text-white">{{ formatUsageCost(row.usage?.total_actual_cost) }}</span>
                </div>
              </div>
            </template>
            <template #cell-usage_tokens="{ row }">
              <div class="min-w-36 space-y-1 text-sm tabular-nums">
                <div class="grid grid-cols-[max-content_minmax(0,1fr)] items-baseline gap-x-2">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageToday') }}:</span>
                  <span class="text-right" :title="formatUsageTokensFull(row.usage?.today_tokens)">{{ formatUsageTokensFull(row.usage?.today_tokens) }}</span>
                </div>
                <div class="grid grid-cols-[max-content_minmax(0,1fr)] items-baseline gap-x-2">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageLast30Days') }}:</span>
                  <span class="text-right" :title="formatUsageTokensFull(row.usage?.last_30_days_tokens)">{{ formatUsageTokensFull(row.usage?.last_30_days_tokens) }}</span>
                </div>
                <div class="grid grid-cols-[max-content_minmax(0,1fr)] items-baseline gap-x-2">
                  <span class="text-gray-500 dark:text-dark-400">{{ t('admin.desktop.usageTotal') }}:</span>
                  <span class="text-right" :title="formatUsageTokensFull(row.usage?.total_tokens)">{{ formatUsageTokensFull(row.usage?.total_tokens) }}</span>
                </div>
              </div>
            </template>
            <template #cell-updated_at="{ value }"><span class="whitespace-nowrap text-sm text-gray-500">{{ formatDateTime(value) }}</span></template>
            <template #cell-actions="{ row }">
              <div class="flex items-center gap-1">
                <button class="detail-row-action" type="button" :title="t('common.edit')" :aria-label="t('admin.desktop.detail.memberAction', { action: t('common.edit'), name: row.name })" @click="openEditMember(row)"><Icon name="edit" size="sm" /></button>
                <button class="detail-row-action" type="button" :title="row.status === 'active' ? t('common.disable') : t('common.enable')" :aria-label="t('admin.desktop.detail.memberAction', { action: t(row.status === 'active' ? 'common.disable' : 'common.enable'), name: row.name })" @click="toggleMember(row)"><Icon :name="row.status === 'active' ? 'ban' : 'checkCircle'" size="sm" /></button>
                <button class="detail-row-action" type="button" :title="t('admin.desktop.rotateModelToken')" :aria-label="t('admin.desktop.detail.memberAction', { action: t('admin.desktop.rotateModelToken'), name: row.name })" @click="confirmRotate(row)"><Icon name="refresh" size="sm" /></button>
                <button class="detail-row-action detail-row-action-danger" type="button" :title="t('common.delete')" :aria-label="t('admin.desktop.detail.memberAction', { action: t('common.delete'), name: row.name })" @click="confirmDeleteMember(row)"><Icon name="trash" size="sm" /></button>
              </div>
            </template>
            <template #empty>
              <EmptyState :title="t(hasMemberFilters ? 'admin.desktop.noMatchingMembers' : 'admin.desktop.noMembers')" :description="memberEmptyDescription" :action-text="hasMemberFilters ? t('common.reset') : canCreateMember ? t('admin.desktop.createMember') : undefined" :action-icon="!hasMemberFilters" @action="hasMemberFilters ? clearMemberFilters() : openCreateMember()" />
            </template>
          </DataTable>
          <Pagination dropdown-class="desktop-detail-select" v-if="memberPagination.total > 0" :page="memberPagination.page" :page-size="memberPagination.page_size" :total="memberPagination.total" @update:page="changeMemberPage" @update:page-size="changeMemberPageSize" />
        </div>
      </section>

      <section v-else-if="organization && activeTab === 'conversations'" :id="panelId('conversations')" tabindex="0" role="tabpanel" :aria-labelledby="tabId('conversations')">
        <DesktopConversationRecords :organization-id="organizationID" :self-managed="selfManaged" />
      </section>

      <section v-else-if="organization && activeTab === 'reports'" :id="panelId('reports')" tabindex="0" role="tabpanel" :aria-labelledby="tabId('reports')">
        <DesktopPeriodReports v-if="organization" :organization-id="organizationID" :self-managed="selfManaged" :enabled="organization.conversation_summary_enabled" :analysis-model="organization.analysis_model" :weekly-enabled="organization.weekly_summary_enabled" :monthly-enabled="organization.monthly_summary_enabled" :initial-kind="String(route.query.report_kind || 'day')" :initial-start="String(route.query.period_start || '')" :initial-report-id="String(route.query.report_id || '')" :initial-date="String(route.query.report_date || '')" />
      </section>

      <section v-else-if="organization && activeTab === 'grants'" :id="panelId('grants')" tabindex="0" role="tabpanel" :aria-labelledby="tabId('grants')">
        <div v-if="!selfManaged" class="mb-4 flex flex-wrap items-center justify-between gap-3 rounded border border-gray-200 p-4 dark:border-dark-700">
          <p class="text-sm text-gray-500">{{ t('biNative.identityHint') }}</p><button class="btn btn-secondary" type="button" @click="router.push('/admin/bi')">{{ t('nav.biOperations') }}</button>
        </div>
        <BIGrantManagement v-if="organization" :key="organizationID" :organization-id="organizationID" :self-managed="selfManaged" />
      </section>

      <section v-else-if="organization" :id="panelId('configuration')" class="min-w-0" tabindex="0" role="tabpanel" :aria-labelledby="tabId('configuration')">
        <div v-if="selfManaged" class="space-y-6">
          <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('admin.desktop.configurationReadOnly') }}</p>
          <section class="card detail-panel"><h2 class="detail-section-title mb-4">{{ t('admin.desktop.reports.title') }}</h2><dl class="detail-config-metadata"><div><dt class="detail-caption">{{ t('admin.desktop.reports.enabled') }}</dt><dd>{{ t(organization?.conversation_summary_enabled ? 'common.enabled' : 'common.disabled') }}</dd></div><div><dt class="detail-caption">{{ t('admin.desktop.reports.model') }}</dt><dd><bdi>{{ organization?.analysis_model || t('admin.desktop.notConfigured') }}</bdi></dd></div></dl></section>
          <section class="card detail-panel space-y-3"><h2 class="detail-section-title">{{ t('admin.desktop.managementPolicy.title') }}</h2><dl class="detail-config-metadata"><div><dt class="detail-caption">{{ t('admin.desktop.managementPolicy.analysisLimit') }}</dt><dd>{{ analysisLimitLabel }}</dd></div><div v-for="field in retentionFields" :key="field"><dt class="detail-caption">{{ t(`admin.desktop.managementPolicy.${field}`) }}</dt><dd>{{ organization.management_policy ? retentionLabel(organization.management_policy[field]) : '—' }}</dd></div></dl><p class="detail-caption">{{ t('admin.desktop.managementPolicy.hint') }}</p></section>
          <div v-for="target in configurationTargets" :key="target.name" class="card detail-panel">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ target.name }}</h2>
              <span class="text-sm text-gray-500 dark:text-dark-400">{{ target.config ? t(target.config.enabled ? 'common.enabled' : 'common.disabled') : t('admin.desktop.notConfigured') }}</span>
            </div>
            <dl v-if="target.config" class="detail-config-metadata">
              <div v-for="field in configurationFields(target.config)" :key="field.label" class="min-w-0">
                <dt class="text-xs text-gray-500 dark:text-dark-400">{{ t(`admin.desktop.${field.label}`) }}</dt>
                <dd class="mt-1 whitespace-pre-wrap break-words text-sm text-gray-900 dark:text-white">{{ field.value }}</dd>
              </div>
            </dl>
            <p v-else class="detail-caption">{{ t('admin.desktop.detail.targetNotConfigured') }}</p>
          </div>
        </div>
        <form v-else class="space-y-6" @submit.prevent="saveConfiguration">
          <header><h2 class="detail-section-title">{{ t('admin.desktop.configuration') }}</h2><p class="detail-caption mt-1">{{ t('admin.desktop.detail.configurationHint') }}</p></header>
          <div v-if="configurationError" class="detail-error" role="alert">{{ configurationError }} {{ t('admin.desktop.detail.saveRetryHint') }}</div>
          <div class="card detail-panel">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3"><h2 class="text-base font-semibold text-gray-900 dark:text-white">ChatGPT Codex</h2><label class="flex items-center gap-2 text-sm"><input v-model="configForm.chat.enabled" class="h-4 w-4 rounded" type="checkbox" />{{ t('common.enabled') }}</label></div>
            <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <Input v-model="configForm.chat.provider_id" :label="t('admin.desktop.providerId')" required />
              <Input v-model="configForm.chat.display_name" :label="t('admin.desktop.displayName')" required />
              <Input v-model="configForm.chat.requested_model" :label="t('admin.desktop.requestedModel')" required />
              <Input model-value="/responses" :label="t('admin.desktop.wireApi')" readonly />
              <Input v-model="configForm.chat.minimum_app_version" :label="t('admin.desktop.minimumAppVersion')" />
              <label class="flex items-center gap-2 self-end pb-3 text-sm"><input v-model="configForm.chat.restart_required" class="h-4 w-4 rounded" type="checkbox" />{{ t('admin.desktop.restartRequired') }}</label>
            </div>
          </div>
          <div class="card detail-panel">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3"><h2 class="text-base font-semibold text-gray-900 dark:text-white">Workbuddy</h2><label class="flex items-center gap-2 text-sm"><input v-model="configForm.work.enabled" class="h-4 w-4 rounded" type="checkbox" @change="onWorkbuddyEnabledChange" />{{ configForm.work.enabled ? t('common.enabled') : t('common.disabled') }}</label></div>
            <label class="mb-4 flex items-center gap-2 text-sm"><input v-model="configForm.includeWorkbuddy" class="h-4 w-4 rounded" type="checkbox" @change="onWorkbuddyIncludedChange" />{{ t('admin.desktop.configureWorkbuddy') }}</label>
            <div v-if="configForm.includeWorkbuddy" class="grid grid-cols-1 gap-4 md:grid-cols-2">
              <Input v-model="configForm.work.provider_id" :label="t('admin.desktop.providerId')" required />
              <Input v-model="configForm.work.display_name" :label="t('admin.desktop.displayName')" required />
              <Input v-model="configForm.work.requested_model" :label="t('admin.desktop.requestedModel')" required />
              <Input model-value="/chat/completions" :label="t('admin.desktop.wireApi')" readonly />
              <Input v-model="configForm.work.minimum_app_version" :label="t('admin.desktop.minimumAppVersion')" />
              <label class="flex items-center gap-2 self-end pb-3 text-sm"><input v-model="configForm.work.restart_required" class="h-4 w-4 rounded" type="checkbox" />{{ t('admin.desktop.restartRequired') }}</label>
            </div>
          </div>
          <div class="flex justify-end"><button class="btn btn-primary" type="submit" :disabled="configSaving">{{ configSaving ? t('common.saving') : t('common.save') }}</button></div>
        </form>
      </section>
    </div>

    <BaseDialog content-class="desktop-detail" :close-label="t('common.close')" v-if="!selfManaged" :show="showOrganizationEdit" :title="t('admin.desktop.editOrganization')" width="wide" @close="closeOrganizationEdit">
      <form id="desktop-organization-edit" class="space-y-6" @submit.prevent="submitOrganizationEdit">
        <p v-if="organizationEditError" class="detail-error" role="alert">{{ organizationEditError }} {{ t('admin.desktop.detail.saveRetryHint') }}</p>
        <fieldset class="detail-form-section">
          <legend class="detail-section-title">{{ t('admin.desktop.organizationForm.basic') }}</legend>
          <div class="mt-4 space-y-4">
            <Input v-model="organizationForm.name" :label="t('admin.desktop.organizationName')" required />
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <Input :model-value="organization?.code" :label="t('admin.desktop.organizationCode')" readonly :hint="t('admin.desktop.organizationForm.codeReadOnly')" />
              <div><label for="desktop-edit-status" class="input-label mb-1.5 block">{{ t('common.status') }}</label><Select id="desktop-edit-status" dropdown-class="desktop-detail-select" :aria-label="t('common.status')" v-model="organizationForm.status" :options="editableStatusOptions" /></div>
            </div>
          </div>
        </fieldset>
        <fieldset class="detail-form-section">
          <legend class="detail-section-title">{{ t('admin.desktop.organizationForm.resources') }}</legend>
          <div class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div class="sm:col-span-2">
              <label for="desktop-edit-gateway_user_id" class="input-label mb-1.5 block">{{ t('admin.desktop.gatewayUser') }}</label>
              <Select id="desktop-edit-gateway_user_id" dropdown-class="desktop-detail-select" :aria-label="t('admin.desktop.gatewayUser')" v-model="organizationForm.gateway_user_id" :options="gatewayUserOptions" searchable remote :loading="gatewayUsersLoading" :disabled="gatewayUserLocked" aria-describedby="desktop-edit-gateway-hint" @search="loadGatewayUsers" />
              <p id="desktop-edit-gateway-hint" class="detail-caption mt-1.5">{{ t(gatewayUserLocked ? 'admin.desktop.provisioningLockedHint' : 'admin.desktop.organizationForm.gatewayHint') }}</p>
            </div>
            <div>
              <label for="desktop-edit-group_id" class="input-label mb-1.5 block">{{ t('admin.desktop.group') }}</label><Select id="desktop-edit-group_id" dropdown-class="desktop-detail-select" :aria-label="t('admin.desktop.group')" v-model="organizationForm.group_id" :options="groupOptions" searchable :loading="groupsLoading" aria-describedby="desktop-edit-group-hint" />
              <p id="desktop-edit-group-hint" class="detail-caption mt-1.5">{{ t('admin.desktop.organizationForm.groupHint') }}</p>
            </div>
            <div>
              <label for="desktop-edit-member-limit" class="input-label mb-1.5 block">{{ t('admin.desktop.memberLimit') }} <span class="text-red-500">*</span></label>
              <input id="desktop-edit-member-limit" v-model.number="organizationForm.member_limit" class="input" type="number" :min="Math.max(1, organization?.member_count || 0)" step="1" required aria-describedby="desktop-edit-capacity-hint" />
              <p id="desktop-edit-capacity-hint" class="detail-caption mt-1.5">{{ t('admin.desktop.organizationForm.currentCapacity', { count: organization?.member_count || 0 }) }} {{ t('admin.desktop.organizationForm.capacityHint') }}</p>
            </div>
          </div>
        </fieldset>
        <fieldset class="detail-form-section">
          <legend class="detail-section-title">{{ t('admin.desktop.organizationForm.reporting') }}</legend>
          <div class="mt-4 space-y-4">
            <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <label class="detail-form-toggle">
                <input v-model="organizationForm.conversation_reporting_enabled" @change="!organizationForm.conversation_reporting_enabled && (organizationForm.conversation_summary_enabled = false)" data-testid="desktop-edit-conversation-reporting" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0 rounded" aria-describedby="desktop-edit-reporting-hint" />
                <span class="min-w-0"><span class="text-sm font-medium">{{ t('admin.desktop.conversationReporting') }}</span><span id="desktop-edit-reporting-hint" class="detail-caption mt-1 block">{{ t('admin.desktop.conversationReportingHint') }}</span></span>
              </label>
              <label class="detail-form-toggle" :class="{ 'cursor-not-allowed opacity-60': !organizationForm.conversation_reporting_enabled }">
                <input v-model="organizationForm.conversation_summary_enabled" data-testid="desktop-edit-summary" type="checkbox" class="mt-0.5 h-4 w-4 shrink-0 rounded" :disabled="!organizationForm.conversation_reporting_enabled" aria-describedby="desktop-edit-summary-hint" />
                <span class="min-w-0"><span class="text-sm font-medium">{{ t('admin.desktop.reports.enabled') }}</span><span id="desktop-edit-summary-hint" class="detail-caption mt-1 block">{{ t('admin.desktop.organizationForm.summaryHint') }}</span><span class="detail-caption mt-1 block">{{ t('admin.desktop.reports.schedule') }}</span></span>
              </label>
            </div>
            <div v-if="organizationForm.conversation_summary_enabled" class="space-y-3"><label class="flex gap-2"><input v-model="organizationForm.weekly_summary_enabled" type="checkbox" />{{ t('biNative.weeklyAuto') }}</label><label class="flex gap-2"><input v-model="organizationForm.monthly_summary_enabled" type="checkbox" />{{ t('biNative.monthlyAuto') }}</label><p class="detail-caption">{{ t('biNative.automaticHint') }}</p></div>
            <Input v-if="organizationForm.conversation_summary_enabled" v-model="organizationForm.analysis_model" :label="t('admin.desktop.reports.model')" required :hint="t('admin.desktop.reports.modelHint')" maxlength="200" />
          </div>
        </fieldset>
        <fieldset class="detail-form-section" aria-describedby="desktop-edit-policy-hint">
          <legend class="detail-section-title">{{ t('admin.desktop.managementPolicy.title') }}</legend>
          <p id="desktop-edit-policy-hint" class="detail-caption mt-4">{{ t('admin.desktop.managementPolicy.hint') }}</p>
          <div class="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label for="desktop-edit-analysis-limit" class="input-label mb-1.5 block">{{ t('admin.desktop.managementPolicy.analysisLimit') }}</label>
              <input id="desktop-edit-analysis-limit" v-model.number="organizationForm.management_policy.analysis_limit_1d" class="input" type="number" min="0" step="any" aria-describedby="desktop-edit-analysis-limit-hint" />
              <p id="desktop-edit-analysis-limit-hint" class="input-hint mt-1.5">{{ t('admin.desktop.organizationForm.analysisZero') }}</p>
            </div>
            <div v-for="field in retentionFields" :key="field">
              <label :for="`desktop-edit-${field}`" class="input-label mb-1.5 block">{{ t(`admin.desktop.managementPolicy.${field}`) }}</label>
              <input :id="`desktop-edit-${field}`" v-model.number="organizationForm.management_policy[field]" class="input" type="number" min="0" max="3650" step="1" :aria-describedby="`desktop-edit-${field}-hint`" />
              <p :id="`desktop-edit-${field}-hint`" class="input-hint mt-1.5">{{ t('admin.desktop.organizationForm.retentionZero') }}</p>
            </div>
          </div>
        </fieldset>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" @click="closeOrganizationEdit">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" form="desktop-organization-edit" :disabled="organizationSaving">{{ organizationSaving ? t('common.saving') : t('common.save') }}</button></div></template>
    </BaseDialog>

    <BaseDialog content-class="desktop-detail" :close-label="t('common.close')" :show="showMemberDialog" :title="editingMember ? t('admin.desktop.editMember') : t('admin.desktop.createMember')" width="narrow" @close="closeMemberDialog">
      <form id="desktop-member-form" class="space-y-4" @submit.prevent="saveMember">
        <p v-if="memberEditError" class="detail-error" role="alert">{{ memberEditError }} {{ t('admin.desktop.detail.saveRetryHint') }}</p>
        <Input v-model="memberForm.name" :label="t('admin.desktop.memberName')" required />
		<Input v-model="memberForm.phone" type="tel" :label="t('admin.desktop.phone')" required placeholder="13800138000" :hint="t('admin.desktop.phoneHint')" />
        <fieldset class="space-y-3"><legend class="input-label">{{ t('admin.desktop.budget.title') }}</legend><p class="detail-caption">{{ t('admin.desktop.budget.hint') }}</p>
          <Input v-for="field in budgetFields" :key="field" v-model.number="memberForm.budget[field]" type="number" min="0" step="0.01" :label="t(`admin.desktop.budget.${field}`)" />
        </fieldset>
        <label class="block space-y-1.5"><span class="input-label block">{{ t('admin.desktop.memberRemark') }}</span><textarea v-model="memberForm.remark" class="input w-full" rows="3" maxlength="500" :placeholder="t('admin.desktop.memberRemarkHint')" data-testid="member-remark" /></label>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" @click="closeMemberDialog">{{ t('common.cancel') }}</button><button class="btn btn-primary" type="submit" form="desktop-member-form" :disabled="memberSaving">{{ memberSaving ? t('common.saving') : t('common.save') }}</button></div></template>
    </BaseDialog>

    <DesktopMemberImportDialog v-if="organization" :show="showMemberImport" :organization-id="organizationID" :self-managed="selfManaged" :available-slots="Math.max(0, organization.member_limit - organization.member_count)" :organization-active="organization.status === 'active'" @close="showMemberImport = false" @imported="reloadAfterMemberImport" />
    <ConfirmDialog content-class="desktop-detail" :show="confirmState.show" :title="confirmState.title" :message="confirmState.message" :confirm-text="confirmState.confirmText" :loading="confirmPending" danger @confirm="runConfirmedAction" @cancel="closeConfirm">
      <div v-if="confirmError" class="detail-error space-y-3" role="alert"><p>{{ confirmError }}</p><p>{{ t('admin.desktop.detail.confirmRetryHint') }}</p><button class="btn btn-secondary" type="button" :disabled="confirmPending" @click="closeConfirm(); retryOrganization()">{{ t('admin.desktop.detail.refreshOrganization') }}</button></div>
    </ConfirmDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useResizeObserver } from '@vueuse/core'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { adminAPI } from '@/api/admin'
import desktopOrganizationAPI from '@/api/desktopOrganization'
import type { DesktopMember, DesktopModelTokenStatus, DesktopOrganization, DesktopStatus, DesktopTarget, DesktopTargetConfig, DesktopWireAPI, UpdateDesktopOrganizationRequest } from '@/api/admin/desktop'
import type { AdminGroup, AdminUser } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { formatDateTime } from '@/utils/format'
import { createDesktopMemberUsageCsv } from '@/utils/desktopMemberUsageCsv'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import StatusBadge from '@/components/common/StatusBadge.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Icon from '@/components/icons/Icon.vue'
import DesktopConversationRecords from '@/components/desktop/DesktopConversationRecords.vue'
import DesktopConversationStatistics from '@/components/desktop/DesktopConversationStatistics.vue'
import DesktopAnalyticsRangePicker from '@/components/desktop/DesktopAnalyticsRangePicker.vue'
import DesktopOrganizationUsageStatistics from '@/components/desktop/DesktopOrganizationUsageStatistics.vue'
import DesktopMemberImportDialog from '@/components/desktop/DesktopMemberImportDialog.vue'
import type { DesktopAnalyticsRange } from '@/api/desktopOrganizationUsage'
import DesktopPeriodReports from '@/components/desktop/DesktopPeriodReports.vue'
import BIGrantManagement from '@/components/bi/BIGrantManagement.vue'
import '@/styles/desktopDetail.css'

type DetailTab = 'usage' | 'members' | 'configuration' | 'conversations' | 'grants' | 'reports'
const { selfManaged = false } = defineProps<{ selfManaged?: boolean }>()
const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const organization = ref<DesktopOrganization | null>(null)
const organizationError = ref(false)
const organizationLoading = ref(false)
const membersError = ref('')
const organizationEditError = ref('')
const memberEditError = ref('')
const configurationError = ref('')
const configurationTargets = computed(() => [
  { name: 'ChatGPT Codex', config: organization.value?.target_config?.targets.chatgpt_codex },
  { name: 'Workbuddy', config: organization.value?.target_config?.targets.workbuddy },
])
const organizationID = computed(() => selfManaged ? (organization.value?.public_id || '') : String(route.params.organizationId || ''))
const organizationAPI = computed(() => selfManaged ? desktopOrganizationAPI : adminAPI.desktop)
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const canViewConversations = computed(() => !selfManaged || organization.value?.conversation_reporting_enabled === true)
const unfilteredConversationFilters = {}
const analyticsRange = ref<DesktopAnalyticsRange>({ days: 30 })
const tabScroll = ref<HTMLElement>()
const tabIcons = { usage: 'chartBar', members: 'users', configuration: 'cog', conversations: 'chat', reports: 'document', grants: 'shield' } as const
const tabs = computed(() => {
  const result: { value: DetailTab; label: string }[] = [{ value: 'usage', label: t('admin.desktop.usageStatistics.tab') }, { value: 'members', label: t('admin.desktop.members') }, { value: 'configuration', label: t('admin.desktop.configuration') }]
  if (canViewConversations.value) result.push({ value: 'conversations', label: t('admin.desktop.conversations.title') })
  if (!selfManaged || organization.value?.conversation_summary_enabled) result.push({ value: 'reports', label: t('admin.desktop.reports.title') })
  result.push({ value: 'grants', label: t('biNative.users') })
  return result
})
const activeTab = computed<DetailTab>(() => tabs.value.find(tab => tab.value === route.query.tab)?.value ?? 'usage')
const members = ref<DesktopMember[]>([])
const membersLoading = ref(false)
const membersExporting = ref(false)
const memberSearch = ref('')
const memberStatus = ref<DesktopStatus | ''>('')
const hasMemberFilters = computed(() => Boolean(memberSearch.value.trim() || memberStatus.value))
const memberPagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })
const showOrganizationEdit = ref(false)
const organizationSaving = ref(false)
const currentGatewayUser = ref<AdminUser | null>(null)
const gatewayUsers = ref<AdminUser[]>([])
const gatewayUsersLoading = ref(false)
const groups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)
const blankPolicy = () => ({ analysis_limit_1d: 0, conversation_retention_days: 0, report_retention_days: 0, execution_retention_days: 0 })
const organizationForm = reactive({ management_policy: blankPolicy(), name: '', status: 'active' as DesktopStatus, gateway_user_id: null as number | null, group_id: null as number | null, member_limit: 10, conversation_reporting_enabled: false, conversation_summary_enabled: false, weekly_summary_enabled: false, monthly_summary_enabled: false, analysis_model: '' })
const showMemberDialog = ref(false)
const showMemberImport = ref(false)
const editingMember = ref<DesktopMember | null>(null)
const memberSaving = ref(false)
const retentionFields = ['conversation_retention_days', 'report_retention_days', 'execution_retention_days'] as const
const budgetFields = ['quota', 'limit_1d', 'limit_7d'] as const
const blankBudget = () => ({ quota: 0, limit_1d: 0, limit_7d: 0 })
const memberForm = reactive({ name: '', phone: '', remark: '', budget: blankBudget() })
const configSaving = ref(false)
const blankTarget = () => ({ enabled: false, provider_id: '', display_name: '', requested_model: '', minimum_app_version: '', restart_required: false })
const configForm = reactive({ chat: { ...blankTarget(), enabled: true }, work: blankTarget(), includeWorkbuddy: false })
const confirmState = reactive({ show: false, title: '', message: '', confirmText: '', action: null as null | (() => Promise<void>) })
const confirmPending = ref(false)
const confirmError = ref('')
let organizationGeneration = 0
let memberTimer: ReturnType<typeof setTimeout> | undefined
let memberController: AbortController | undefined
let memberExportController: AbortController | undefined
let gatewayController: AbortController | undefined

const gatewayUserLocked = computed(() => (organization.value?.member_count ?? 0) > 0)
const analysisLimitLabel = computed(() => {
  const limit = organization.value?.management_policy?.analysis_limit_1d
  return limit === undefined ? '—' : limit === 0 ? t('admin.desktop.organizationForm.unlimited') : formatUsageCost(limit)
})
function retentionLabel(days: number) { return days === 0 ? t('admin.desktop.organizationForm.forever') : t('admin.desktop.organizationForm.retentionDays', { count: days }) }
const memberLimitReached = computed(() => Boolean(organization.value && organization.value.member_count >= organization.value.member_limit))
const canCreateMember = computed(() => organization.value?.status === 'active' && !memberLimitReached.value)
const memberEmptyDescription = computed(() => {
  if (hasMemberFilters.value) return t('admin.desktop.noMatchingMembersDescription')
  if (organization.value?.status !== 'active') return t('admin.desktop.errors.ORGANIZATION_DISABLED')
  if (memberLimitReached.value) return t('admin.desktop.errors.MEMBER_LIMIT_REACHED')
  return t('admin.desktop.noMembersDescription')
})
const statusOptions = computed(() => [{ value: '', label: t('common.all') }, { value: 'active', label: t('common.active') }, { value: 'disabled', label: t('common.disabled') }])
const editableStatusOptions = computed(() => statusOptions.value.slice(1))
const memberColumns = computed<Column[]>(() => [
	{ key: 'name', label: t('admin.desktop.member') }, { key: 'remark', label: t('admin.desktop.memberRemark') }, { key: 'phone', label: t('admin.desktop.phone') },
  { key: 'status', label: t('common.status') }, { key: 'model_token_status', label: t('admin.desktop.modelToken') },
  { key: 'usage_cost', label: t('admin.desktop.usageCost') },
  { key: 'usage_tokens', label: t('admin.desktop.usageTokens') },
  { key: 'updated_at', label: t('admin.desktop.updatedAt') },
  { key: 'actions', label: t('common.actions'), class: 'w-40 min-w-40' },
])
const gatewayUserOptions = computed(() => {
  const all = [...gatewayUsers.value]
  if (currentGatewayUser.value && !all.some((user) => user.id === currentGatewayUser.value?.id)) all.unshift(currentGatewayUser.value)
  return all.map((user) => ({ value: user.id, label: `${user.username || user.email} · ${user.email}` }))
})
const groupOptions = computed(() => groups.value.map((group) => ({ value: group.id, label: group.name })))

function errorMessage(error: unknown): string {
  const reason = (error as { reason?: string })?.reason
  const key = `admin.desktop.errors.${reason}`
  if (reason && te(key)) return t(key)
  return (error as { message?: string })?.message || t('admin.desktop.errors.UNKNOWN')
}
function statusLabel(value: DesktopStatus): string { return value === 'active' ? t('common.active') : t('common.disabled') }
function budgetWarning(member: DesktopMember): string {
 const b = member.budget
 if (!b) return ''
 const ratios = [[b.quota_used || 0, b.quota], [b.usage_1d || 0, b.limit_1d], [b.usage_7d || 0, b.limit_7d]].filter(([, limit]) => limit > 0).map(([used, limit]) => used / limit)
 return ratios.some(ratio => ratio >= 1) ? 'admin.desktop.budget.exhausted' : ratios.some(ratio => ratio >= 0.8) ? 'admin.desktop.budget.warning' : ''
}
function tokenStatusLabel(value: DesktopModelTokenStatus): string { return t(`admin.desktop.tokenStatus.${value}`) }
function tokenBadge(value: DesktopModelTokenStatus): string { return value === 'active' ? 'badge-success' : value === 'disabled' ? 'badge-warning' : 'badge-gray' }
function normalizeUsageNumber(value: number | undefined): number { return Number.isFinite(value) ? Number(value) : 0 }
function formatUsageCost(value: number | undefined): string { return value === undefined ? t('admin.desktop.usageUnavailable') : `$${normalizeUsageNumber(value).toFixed(4)}` }
function formatUsageTokensFull(value: number | undefined): string { return value === undefined ? t('admin.desktop.usageUnavailable') : normalizeUsageNumber(value).toLocaleString() }
function setTab(tab: DetailTab) { void router.replace({ query: { ...route.query, tab } }) }
function tabId(tab: DetailTab) { return `desktop-organization-tab-${tab}` }
function panelId(tab: DetailTab) { return `desktop-organization-panel-${tab}` }
function handleTabKeydown(event: KeyboardEvent, index: number) {
  let nextIndex: number | undefined
  if (event.key === 'ArrowRight') nextIndex = (index + 1) % tabs.value.length
  else if (event.key === 'ArrowLeft') nextIndex = (index - 1 + tabs.value.length) % tabs.value.length
  else if (event.key === 'Home') nextIndex = 0
  else if (event.key === 'End') nextIndex = tabs.value.length - 1
  if (nextIndex === undefined) return

  event.preventDefault()
  const nextTab = tabs.value[nextIndex].value
  setTab(nextTab)
  void nextTick(() => document.getElementById(tabId(nextTab))?.focus())
}

async function loadOrganization(): Promise<boolean> {
  const generation = ++organizationGeneration
  const requestedID = organizationID.value
  organizationError.value = false
  organizationLoading.value = true
  if (!selfManaged && organization.value?.public_id !== organizationID.value) organization.value = null
  try {
    const result = await organizationAPI.value.getOrganization(requestedID)
    if (generation !== organizationGeneration) return false
    if (!result) {
      organization.value = null
      if (selfManaged) void router.replace(dashboardPath.value)
      else organizationError.value = true
      return false
    }
    organization.value = result
    applyConfiguration(result.target_config)
    const gatewayUser = selfManaged ? null : await adminAPI.desktop.getGatewayUser(result.gateway_user.id)
    if (generation !== organizationGeneration) return false
    currentGatewayUser.value = gatewayUser
    return true
  } catch {
    if (generation === organizationGeneration) organizationError.value = true
    return false
  } finally { if (generation === organizationGeneration) organizationLoading.value = false }
}
async function retryOrganization() { if (await loadOrganization()) await loadMembers() }
async function loadMembers() {
  memberController?.abort()
  const request = new AbortController()
  memberController = request
  membersLoading.value = true
  membersError.value = ''
  try {
    const result = await organizationAPI.value.listMembers(organizationID.value, memberPagination.page, memberPagination.page_size, { search: memberSearch.value.trim(), status: memberStatus.value }, request.signal)
    if (!request.signal.aborted) { members.value = result.items; memberPagination.total = result.total }
  } catch (error) {
    if (!request.signal.aborted && (error as { code?: string })?.code !== 'ERR_CANCELED') {
      membersError.value = t('admin.desktop.detail.membersLoadFailed')
      appStore.showError(errorMessage(error))
    }
  } finally { if (!request.signal.aborted) membersLoading.value = false }
}
function scheduleMembers() { clearTimeout(memberTimer); memberTimer = setTimeout(resetMembers, 300) }
function resetMembers() { memberPagination.page = 1; void loadMembers() }
function clearMemberFilters() { memberSearch.value = ''; memberStatus.value = ''; resetMembers() }
function changeMemberPage(page: number) { memberPagination.page = page; void loadMembers() }
function changeMemberPageSize(size: number) { memberPagination.page_size = size; memberPagination.page = 1; void loadMembers() }

function cancelMemberExport() {
  memberExportController?.abort()
  memberExportController = undefined
  membersExporting.value = false
}

async function exportMemberUsage() {
  if (membersExporting.value || !organization.value || organization.value.public_id !== organizationID.value) return
  const request = new AbortController()
  memberExportController = request
  membersExporting.value = true
  const selectedOrganization = organization.value
  const api = organizationAPI.value
  const filters: { search: string; status: DesktopStatus | ''; as_of?: string } = { search: memberSearch.value.trim(), status: memberStatus.value }
  try {
    const allMembers: DesktopMember[] = []
    let pages = 1
    for (let page = 1; page <= pages; page++) {
      const result = await api.listMembers(selectedOrganization.public_id, page, 100, filters, request.signal)
      if (request.signal.aborted) return
      if (page === 1) { pages = result.pages; if (result.items[0]?.usage_as_of) filters.as_of = result.items[0].usage_as_of }
      allMembers.push(...result.items)
    }
    if (allMembers.length === 0) {
      appStore.showWarning(t('admin.desktop.memberUsageExport.empty'))
      return
    }
    const csv = createDesktopMemberUsageCsv(selectedOrganization, allMembers, t, { asOf: filters.as_of || '', timezone: allMembers[0]?.usage_timezone || '' })
    const file = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const code = selectedOrganization.code.replace(/[^a-zA-Z0-9_-]/g, '_')
    saveAs(file, `desktop_member_usage_${code}_${new Date().toISOString().slice(0, 10)}.csv`)
    appStore.showSuccess(t('admin.desktop.memberUsageExport.success'))
  } catch (error) {
    if (!request.signal.aborted && (error as { code?: string })?.code !== 'ERR_CANCELED') {
      appStore.showError(t('admin.desktop.memberUsageExport.failed'))
    }
  } finally {
    if (memberExportController === request) {
      memberExportController = undefined
      membersExporting.value = false
    }
  }
}

async function loadGatewayUsers(query = '') {
  gatewayController?.abort(); gatewayController = new AbortController(); gatewayUsersLoading.value = true
  try { gatewayUsers.value = (await adminAPI.desktop.listAvailableGatewayUsers(query, gatewayController.signal)).items }
  catch (error) { if ((error as { code?: string })?.code !== 'ERR_CANCELED') appStore.showError(errorMessage(error)) }
  finally { gatewayUsersLoading.value = false }
}
async function openEditOrganization() {
  if (selfManaged || !organization.value) return
  Object.assign(organizationForm, { name: organization.value.name, status: organization.value.status, gateway_user_id: organization.value.gateway_user.id, group_id: organization.value.group.id, member_limit: organization.value.member_limit, conversation_reporting_enabled: organization.value.conversation_reporting_enabled === true, conversation_summary_enabled: organization.value.conversation_summary_enabled === true, weekly_summary_enabled: organization.value.weekly_summary_enabled === true, monthly_summary_enabled: organization.value.monthly_summary_enabled === true, analysis_model: organization.value.analysis_model || '', management_policy: { ...blankPolicy(), ...organization.value.management_policy } })
  organizationEditError.value = ''
  showOrganizationEdit.value = true
  groupsLoading.value = true
  void loadGatewayUsers()
  try { groups.value = await adminAPI.desktop.listActiveGroups() } catch (error) { appStore.showError(errorMessage(error)) } finally { groupsLoading.value = false }
}
function closeOrganizationEdit() { if (!organizationSaving.value) showOrganizationEdit.value = false }
function submitOrganizationEdit() {
  if (selfManaged) return
  if (!organization.value || !organizationForm.name.trim()) { organizationEditError.value = t('admin.desktop.errors.VALIDATION_FAILED'); appStore.showError(organizationEditError.value); return }
  if (organization.value.status === 'active' && organizationForm.status === 'disabled') {
    openConfirm(t('admin.desktop.disableOrganization'), t('admin.desktop.disableOrganizationImpact', { name: organization.value.name }), t('common.disable'), saveOrganization)
    return
  }
  void saveOrganization()
}
async function saveOrganization() {
  if (selfManaged || !organization.value) return
  const requestedID = organizationID.value
  if (!Number.isInteger(organizationForm.member_limit) || organizationForm.member_limit < 1) {
    organizationEditError.value = t('admin.desktop.errors.VALIDATION_FAILED'); appStore.showError(organizationEditError.value); return
  }
  if (organizationForm.member_limit < organization.value.member_count) {
    organizationEditError.value = t('admin.desktop.errors.MEMBER_LIMIT_BELOW_CURRENT_COUNT'); appStore.showError(organizationEditError.value); return
  }
  organizationEditError.value = ''
  organizationSaving.value = true
  try {
    const input = { name: organizationForm.name.trim(), status: organizationForm.status } as UpdateDesktopOrganizationRequest
    if (JSON.stringify(organizationForm.management_policy) !== JSON.stringify({ ...blankPolicy(), ...organization.value.management_policy })) input.management_policy = { ...organizationForm.management_policy }
    input.member_limit = organizationForm.member_limit
    input.conversation_reporting_enabled = organizationForm.conversation_reporting_enabled
    input.conversation_summary_enabled = organizationForm.conversation_summary_enabled
    input.weekly_summary_enabled = organizationForm.conversation_summary_enabled && organizationForm.weekly_summary_enabled
    input.monthly_summary_enabled = organizationForm.conversation_summary_enabled && organizationForm.monthly_summary_enabled
    input.analysis_model = organizationForm.analysis_model.trim()
    if (!gatewayUserLocked.value && organizationForm.gateway_user_id && organizationForm.gateway_user_id !== organization.value.gateway_user.id) input.gateway_user_id = organizationForm.gateway_user_id
    if (organizationForm.group_id && organizationForm.group_id !== organization.value.group.id) input.group_id = organizationForm.group_id
    const updated = await adminAPI.desktop.updateOrganization(requestedID, input)
    if (requestedID !== organizationID.value) return
    organization.value = updated
    appStore.showSuccess(t('admin.desktop.organizationUpdated')); showOrganizationEdit.value = false; closeConfirm(); await loadMembers()
  } catch (error) { organizationEditError.value = errorMessage(error); if (confirmState.show) confirmError.value = organizationEditError.value; appStore.showError(organizationEditError.value) } finally { organizationSaving.value = false }
}

async function reloadAfterMemberImport() { await Promise.all([loadMembers(), loadOrganization()]) }
function openCreateMember() { memberEditError.value = ''; editingMember.value = null; Object.assign(memberForm, { name: '', phone: '', remark: '', budget: blankBudget() }); showMemberDialog.value = true }
function openEditMember(member: DesktopMember) { memberEditError.value = ''; editingMember.value = member; Object.assign(memberForm, { name: member.name, phone: member.phone, remark: member.remark || '', budget: { quota: member.budget?.quota || 0, limit_1d: member.budget?.limit_1d || 0, limit_7d: member.budget?.limit_7d || 0 } }); showMemberDialog.value = true }
function closeMemberDialog() { if (!memberSaving.value) showMemberDialog.value = false }
async function saveMember() {
	if (!editingMember.value && memberLimitReached.value) { memberEditError.value = t('admin.desktop.errors.MEMBER_LIMIT_REACHED'); appStore.showError(memberEditError.value); return }
	if (!memberForm.name.trim() || !memberForm.phone.trim() || Array.from(memberForm.remark.trim()).length > 500) { memberEditError.value = t('admin.desktop.errors.VALIDATION_FAILED'); appStore.showError(memberEditError.value); return }
	memberEditError.value = ''
	memberSaving.value = true
	try {
    if (editingMember.value) {
      const input = {
        ...(JSON.stringify(memberForm.budget) !== JSON.stringify({ quota: editingMember.value.budget?.quota || 0, limit_1d: editingMember.value.budget?.limit_1d || 0, limit_7d: editingMember.value.budget?.limit_7d || 0 }) ? { budget: { ...memberForm.budget } } : {}),
        ...(memberForm.name.trim() !== editingMember.value.name ? { name: memberForm.name.trim() } : {}),
        ...(memberForm.remark.trim() !== (editingMember.value.remark || '') ? { remark: memberForm.remark.trim() } : {}),
        ...(memberForm.phone.trim() !== editingMember.value.phone ? { phone: memberForm.phone.trim() } : {}),
      }
      if (Object.keys(input).length) await organizationAPI.value.updateMember(organizationID.value, editingMember.value.public_id, input)
    }
    else await organizationAPI.value.createMember(organizationID.value, { name: memberForm.name.trim(), phone: memberForm.phone.trim(), remark: memberForm.remark.trim(), ...(Object.values(memberForm.budget).some(value => value > 0) ? { budget: { ...memberForm.budget } } : {}) })
    appStore.showSuccess(t(editingMember.value ? 'admin.desktop.memberUpdated' : 'admin.desktop.memberCreated')); showMemberDialog.value = false; await Promise.all([loadMembers(), loadOrganization()])
  } catch (error) { memberEditError.value = errorMessage(error); appStore.showError(memberEditError.value) } finally { memberSaving.value = false }
}
function toggleMember(member: DesktopMember) {
  const action = async () => { try { await organizationAPI.value.updateMember(organizationID.value, member.public_id, { status: member.status === 'active' ? 'disabled' : 'active' }); appStore.showSuccess(t('admin.desktop.memberUpdated')); closeConfirm(); await loadMembers() } catch (error) { confirmError.value = errorMessage(error); appStore.showError(confirmError.value) } }
  if (member.status === 'active') openConfirm(t('admin.desktop.disableMember'), t('admin.desktop.disableMemberImpact', { name: member.name }), t('common.disable'), action)
  else void action()
}
function confirmRotate(member: DesktopMember) { openConfirm(t('admin.desktop.rotateModelToken'), t('admin.desktop.rotateModelTokenImpact', { name: member.name }), t('admin.desktop.rotate'), async () => { try { await organizationAPI.value.rotateModelToken(organizationID.value, member.public_id); appStore.showSuccess(t('admin.desktop.modelTokenRotated')); closeConfirm(); await loadMembers() } catch (error) { confirmError.value = errorMessage(error); appStore.showError(confirmError.value) } }) }
function confirmDeleteMember(member: DesktopMember) { openConfirm(t('admin.desktop.deleteMember'), t('admin.desktop.deleteMemberImpact', { name: member.name }), t('common.delete'), async () => { try { await organizationAPI.value.deleteMember(organizationID.value, member.public_id); appStore.showSuccess(t('admin.desktop.memberDeleted')); closeConfirm(); await Promise.all([loadMembers(), loadOrganization()]) } catch (error) { confirmError.value = errorMessage(error); appStore.showError(confirmError.value) } }) }
function openConfirm(title: string, message: string, confirmText: string, action: () => Promise<void>) { confirmError.value = ''; Object.assign(confirmState, { show: true, title, message, confirmText, action }) }
function closeConfirm() { confirmState.show = false; confirmState.action = null }
async function runConfirmedAction() {
  if (confirmPending.value || !confirmState.action) return
  confirmPending.value = true
  try { await confirmState.action() } finally { confirmPending.value = false }
}

function configurationFields(target: DesktopTarget) {
  return [
    { label: 'providerId', value: target.provider_id },
    { label: 'displayName', value: target.display_name },
    { label: 'requestedModel', value: target.requested_model },
    { label: 'wireApi', value: target.wire_api === 'responses' ? '/responses' : '/chat/completions' },
    { label: 'minimumAppVersion', value: target.minimum_app_version || '—' },
    { label: 'restartRequired', value: t(target.restart_required ? 'common.yes' : 'common.no') },
  ]
}
function applyConfiguration(value?: DesktopTargetConfig | null) {
  const chat = value?.targets.chatgpt_codex; const work = value?.targets.workbuddy
  Object.assign(configForm.chat, chat ? { ...chat, minimum_app_version: chat.minimum_app_version || '' } : { ...blankTarget(), enabled: true })
  Object.assign(configForm.work, work ? { ...work, minimum_app_version: work.minimum_app_version || '' } : blankTarget())
  configForm.includeWorkbuddy = Boolean(work)
}
function buildTarget(target: typeof configForm.chat, wireAPI: DesktopWireAPI, enabled = target.enabled) { return { enabled, provider_id: target.provider_id.trim(), display_name: target.display_name.trim(), requested_model: target.requested_model.trim(), wire_api: wireAPI, ...(target.minimum_app_version.trim() ? { minimum_app_version: target.minimum_app_version.trim() } : {}), restart_required: target.restart_required } }
function onWorkbuddyEnabledChange() { if (configForm.work.enabled) configForm.includeWorkbuddy = true }
function onWorkbuddyIncludedChange() { if (!configForm.includeWorkbuddy) configForm.work.enabled = false }
async function saveConfiguration() {
  if (selfManaged) return
  const requestedID = organizationID.value
  const chat = buildTarget(configForm.chat, 'responses')
  const work = configForm.includeWorkbuddy ? buildTarget(configForm.work, 'chat_completions') : undefined
  if (![chat, work].filter(Boolean).every((target) => target?.provider_id && target.display_name && target.requested_model)) { configurationError.value = t('admin.desktop.errors.VALIDATION_FAILED'); appStore.showError(configurationError.value); return }
  configurationError.value = ''
  configSaving.value = true
  try {
    const updated = await adminAPI.desktop.updateModelConfiguration(requestedID, { schema_version: 1, targets: { chatgpt_codex: chat, ...(work ? { workbuddy: work } : {}) } })
    if (requestedID !== organizationID.value) return
    organization.value = updated
    appStore.showSuccess(t('admin.desktop.configurationSaved'))
  }
  catch (error) { configurationError.value = errorMessage(error); appStore.showError(configurationError.value) } finally { configSaving.value = false }
}

watch([canViewConversations, () => organization.value?.conversation_summary_enabled, () => organization.value?.public_id], () => {
  if (selfManaged && organization.value && ((route.query.tab === 'reports' && !organization.value.conversation_summary_enabled) || (!canViewConversations.value && route.query.tab === 'conversations'))) setTab('usage')
})
function keepActiveTabVisible() {
  const scroll = tabScroll.value
  const selected = scroll?.querySelector<HTMLElement>('[aria-selected="true"]')
  if (!scroll || !selected) return
  const viewport = scroll.getBoundingClientRect()
  const tab = selected.getBoundingClientRect()
  if (tab.left < viewport.left) scroll.scrollLeft += tab.left - viewport.left
  else if (tab.right > viewport.right) scroll.scrollLeft += tab.right - viewport.right
}
watch([activeTab, () => organization.value?.public_id], keepActiveTabVisible, { flush: 'post' })
useResizeObserver(tabScroll, keepActiveTabVisible)

watch(() => route.params.organizationId, async () => {
  if (!selfManaged) {
    showOrganizationEdit.value = false; showMemberDialog.value = false; showMemberImport.value = false; closeConfirm()
    configurationError.value = ''; members.value = []
    await Promise.all([loadOrganization(), loadMembers()])
  }
})
watch([organizationID, () => organization.value?.public_id, () => selfManaged], cancelMemberExport, { flush: 'sync' })
onMounted(() => {
  if (selfManaged) {
    void loadOrganization().then(async (loaded) => {
      if (loaded) await loadMembers()
    })
    return
  }
  void Promise.all([loadOrganization(), loadMembers()])
})
onBeforeUnmount(() => { organizationGeneration++; clearTimeout(memberTimer); memberController?.abort(); gatewayController?.abort(); cancelMemberExport() })
</script>
