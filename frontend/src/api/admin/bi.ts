import { apiClient } from '../client'
import type { BIPage } from '../bi'
export interface BIOverview { bi_enabled: boolean; desktop_enabled: boolean; privacy_notice_published: boolean; identity_mode: string; config: { appid: string; app_secret_configured: boolean; jwt_secret_configured: boolean; identity_secret_configured: boolean; min_client_version: string }; counts: Record<string, number> }
export interface BIRetentionPolicy { audit_days: number; ephemeral_days: number }
export interface BIOperationRow { [key: string]: string | number | boolean | null }
export interface BIReportModelStatistics extends BIOperationRow { model: string; execution_count: string; total_tokens: string | null; actual_cost: string | null; missing_token_count: string; missing_cost_count: string }
export interface BIReportStatistics { task_count: string; organization_count: string; execution_count: string; total_tokens: string | null; actual_cost: string | null; missing_token_count: string; missing_cost_count: string; task_statuses: Record<string, string>; execution_statuses: Record<string, string>; models: BIReportModelStatistics[] }
async function getOverview() { return (await apiClient.get<BIOverview>('/admin/desktop/operations/overview')).data }
async function list(path: string, page = 1, params: Record<string, string> = {}) { return (await apiClient.get<BIPage<BIOperationRow>>(`/admin/desktop/operations/${path}`, { params: { page, page_size: 20, ...params } })).data }
async function userStatus(id: string, status: string) { await apiClient.put(`/admin/desktop/operations/identities/users/${encodeURIComponent(id)}/status`, { status }) }
async function retention() { return (await apiClient.get<BIRetentionPolicy>('/admin/desktop/operations/retention')).data }
async function saveRetention(input: BIRetentionPolicy) { return (await apiClient.put<BIRetentionPolicy>('/admin/desktop/operations/retention', input)).data }
async function cleanup() { return (await apiClient.post('/admin/desktop/operations/cleanup/run', {})).data }
async function reportStatistics(params: Record<string, string> = {}) { return (await apiClient.get<BIReportStatistics>('/admin/desktop/operations/report-statistics', { params })).data }
export default { getOverview, list, userStatus, retention, saveRetention, cleanup, reportStatistics }
