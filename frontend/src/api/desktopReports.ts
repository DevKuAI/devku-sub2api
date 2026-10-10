import { apiClient } from './client'

export interface DesktopReportExecution {
  id: number
  revision: number
  kind: 'member' | 'summary'
  member_id?: string
  model: string
  round: number
  chunk: number
  attempt: number
  request_id: string
  status: 'running' | 'completed' | 'failed' | 'blocked'
  error: string
  started_at: string
  finished_at: string | null
}
export interface DesktopGeneratedReport {
  executions?: DesktopReportExecution[]
  status: string
  content: string
  model: string
  generated_at: string | null
  error: string
}
export interface DesktopMemberReport extends DesktopGeneratedReport {
  member_id: string
  name: string
  deleted: boolean
  record_count: number
  source_ids: string[]
}
export interface DesktopReportState {
  date: string
  timezone: string
  analysis_model: string
  status: string
  reason: string
  expected_members: number
  completed_members: number
  missing_members: string[]
  task?: { id: number; status: string; reason: string; revision: number }
}
export interface DesktopMemberReports extends DesktopReportState { members: DesktopMemberReport[] }
export interface DesktopSummaryReport extends DesktopReportState { summary: DesktopGeneratedReport }
export type DesktopReportMode = 'generate' | 'retry' | 'regenerate'
function basePath(organizationID: string, selfManaged: boolean) {
  return selfManaged ? '/desktop/organization/daily-reports' : `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/daily-reports`
}
export async function getDesktopMemberReports(organizationID: string, selfManaged: boolean, date: string, signal?: AbortSignal) {
  const { data } = await apiClient.get<DesktopMemberReports>(`${basePath(organizationID, selfManaged)}/${encodeURIComponent(date)}/members`, { signal })
  return data
}
export async function getDesktopSummaryReport(organizationID: string, selfManaged: boolean, date: string, signal?: AbortSignal) {
  const { data } = await apiClient.get<DesktopSummaryReport>(`${basePath(organizationID, selfManaged)}/${encodeURIComponent(date)}/summary`, { signal })
  return data
}
export async function runDesktopReports(organizationID: string, date: string, mode: DesktopReportMode) {
  const { data } = await apiClient.post(`${basePath(organizationID, false)}/${encodeURIComponent(date)}/run`, { mode })
  return data
}
