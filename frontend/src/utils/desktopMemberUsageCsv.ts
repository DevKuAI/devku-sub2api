import type { DesktopMember, DesktopOrganization } from '@/api/admin/desktop'

function csvValue(value: string | number | undefined): string {
  let text = String(value ?? '')
  // Keep phone prefixes and untrusted text from being interpreted as formulas.
  if (typeof value === 'string' && (/^[\s\uFEFF]*[=+\-@]/.test(text) || /^[\t\r\n]/.test(text))) {
    text = `'${text}`
  }
  return /[,"\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text
}

export function createDesktopMemberUsageCsv(
  organization: Pick<DesktopOrganization, 'name' | 'code'>,
  members: DesktopMember[],
  t: (key: string) => string,
): string {
  const headers = [
    'admin.desktop.organizationName',
    'admin.desktop.organizationCode',
    'admin.desktop.memberUsageExport.memberId',
    'admin.desktop.member',
    'admin.desktop.phone',
    'common.status',
    'admin.desktop.modelToken',
    'admin.desktop.memberUsageExport.todayCost',
    'admin.desktop.memberUsageExport.last30DaysCost',
    'admin.desktop.memberUsageExport.totalCost',
    'admin.desktop.memberUsageExport.todayTokens',
    'admin.desktop.memberUsageExport.last30DaysTokens',
    'admin.desktop.memberUsageExport.totalTokens',
    'admin.desktop.updatedAt',
  ].map(key => t(key))
  const rows = members.map(member => [
    organization.name,
    organization.code,
    member.public_id,
    member.name,
    member.phone,
    t(member.status === 'active' ? 'common.active' : 'common.disabled'),
    t(`admin.desktop.tokenStatus.${member.model_token_status}`),
    member.usage?.today_actual_cost,
    member.usage?.last_30_days_actual_cost,
    member.usage?.total_actual_cost,
    member.usage?.today_tokens,
    member.usage?.last_30_days_tokens,
    member.usage?.total_tokens,
    member.updated_at,
  ])
  return '\uFEFF' + [headers, ...rows].map(row => row.map(csvValue).join(',')).join('\r\n')
}
