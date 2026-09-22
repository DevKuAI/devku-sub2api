import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export interface DesktopConversationSegment {
  text: string
  truncated: boolean
}

export interface DesktopConversation {
  record_id: string
  organization_id: string
  member_id: string
  member_name: string
  member_deleted: boolean
  client: 'workbuddy' | 'chatgpt_codex'
  installation_id: string
  source_session_id: string
  source_turn_id: string | null
  started_at: string
  stopped_at: string
  received_at: string
  cwd: string | null
  capture_status: 'captured' | 'response_missing'
  prompt_count: number
}

export interface DesktopConversationDetail extends DesktopConversation {
  schema_version: 2
  prompts: DesktopConversationSegment[]
  response: DesktopConversationSegment | null
}

export interface DesktopConversationFilters {
  member_id?: string
  member_search?: string
  client?: string
  capture_status?: string
  record_id?: string
  source_session_id?: string
  installation_id?: string
  received_from?: string
  received_to?: string
}

export interface DesktopConversationQuery extends DesktopConversationFilters {
  page: number
  page_size: number
  sort_order?: 'asc' | 'desc'
}

export interface DesktopConversationCounts {
  record_count: number
  prompt_count: number
}

export interface DesktopConversationStatistics {
  timezone: string
  as_of: string
  today: DesktopConversationCounts
  week: DesktopConversationCounts
  month: DesktopConversationCounts
  total: DesktopConversationCounts
}

function basePath(organizationID: string, selfManaged: boolean): string {
  return selfManaged
    ? '/desktop/organization/conversation-records'
    : `/admin/desktop/organizations/${encodeURIComponent(organizationID)}/conversation-records`
}

export async function listDesktopConversations(organizationID: string, selfManaged: boolean, query: DesktopConversationQuery, signal?: AbortSignal): Promise<PaginatedResponse<DesktopConversation>> {
  const { data } = await apiClient.get<PaginatedResponse<DesktopConversation>>(basePath(organizationID, selfManaged), { params: query, signal })
  return data
}

export async function getDesktopConversation(organizationID: string, selfManaged: boolean, recordID: string, signal?: AbortSignal): Promise<DesktopConversationDetail> {
  const { data } = await apiClient.get<DesktopConversationDetail>(`${basePath(organizationID, selfManaged)}/${encodeURIComponent(recordID)}`, { signal })
  return data
}

export async function getDesktopConversationStatistics(organizationID: string, selfManaged: boolean, filters: DesktopConversationFilters, signal?: AbortSignal): Promise<DesktopConversationStatistics> {
  const { data } = await apiClient.get<DesktopConversationStatistics>(`${basePath(organizationID, selfManaged)}/statistics`, { params: filters, signal })
  return data
}
