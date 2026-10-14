import { apiClient } from './client'
import type { BIPage } from './bi'
export interface PeriodCoverage { available_dates: string[]; partial_dates: string[]; missing_dates: string[]; no_record_dates: string[]; reasons: Record<string, string> }
export interface PeriodReport { id: string; kind: 'week' | 'month'; range: { start_date: string; end_date: string; timezone: string }; revision: number; status: string; reason: string; generated_at: string | null; coverage: PeriodCoverage; source_changed: boolean; model: string }
export interface PeriodSource { date: string; revision: number; status: string; generated_at: string | null; content_hash: string; content?: string; current_status?: string }
export function desktopPeriods(org: string, selfManaged = false) {
  const base = (selfManaged ? '/desktop/organization' : `/admin/desktop/organizations/${encodeURIComponent(org)}`) + '/period-reports'
  return {
    async list(kind: string, start: string, page = 1) { return (await apiClient.get<BIPage<PeriodReport>>(base, { params: { kind, period_start: start || undefined, page, page_size: 20 } })).data },
    async detail(id: string) { return (await apiClient.get<{ report: PeriodReport; content: string | null; summary_outline?: {sections:{kind:string;title:string;start_line:number;end_line:number;source_dates:string[]}[];validated_source_dates:string[];unverified_source_dates:string[]} }>(`${base}/${encodeURIComponent(id)}`)).data },
    async sources(id: string, page = 1) { return (await apiClient.get<BIPage<PeriodSource>>(`${base}/${encodeURIComponent(id)}/sources`, { params: { page, page_size: 20 } })).data },
    async source(id: string, date: string) { return (await apiClient.get<PeriodSource>(`${base}/${encodeURIComponent(id)}/sources/${encodeURIComponent(date)}`)).data },
    async run(kind: string, start: string, mode: string) { return (await apiClient.post<PeriodReport>(`${base}/run`, { kind, period_start: start, mode })).data },
  }
}
export function lastClosedPeriod(kind: 'week' | 'month', now = new Date()): string {
  const china = new Date(now.getTime() + 8 * 3600000)
  const date = new Date(Date.UTC(china.getUTCFullYear(), china.getUTCMonth(), china.getUTCDate()))
  if (kind === 'week') date.setUTCDate(date.getUTCDate() - (date.getUTCDay() + 6) % 7 - 7)
  else { date.setUTCDate(1); date.setUTCMonth(date.getUTCMonth() - 1) }
  return date.toISOString().slice(0, 10)
}
