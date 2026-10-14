export type BITone = 'success' | 'warning' | 'danger' | 'progress' | 'neutral'

const statuses: Record<string, string> = {
  active: 'common.enabled', disabled: 'common.disabled',
  expired: 'biNative.inviteStatus.expired', revoked: 'biNative.inviteStatus.revoked', used: 'biNative.inviteStatus.used',
  pending: 'admin.desktop.reports.statuses.pending', waiting: 'admin.desktop.reports.statuses.waiting',
  running: 'admin.desktop.reports.statuses.running', completed: 'admin.desktop.reports.statuses.completed',
  partial: 'admin.desktop.reports.statuses.partial', failed: 'admin.desktop.reports.statuses.failed',
  no_records: 'admin.desktop.reports.statuses.no_records', skipped: 'admin.desktop.reports.statuses.skipped',
}
export function biStatusKey(value: string): string | undefined { return statuses[value] }
export function biStatusTone(value: string | boolean): BITone {
  if (value === true || ['active', 'completed', 'used'].includes(String(value))) return 'success'
  if (['disabled', 'failed', 'revoked'].includes(String(value))) return 'danger'
  if (['partial', 'expired'].includes(String(value))) return 'warning'
  if (['pending', 'waiting', 'running'].includes(String(value))) return 'progress'
  return 'neutral'
}

export function biDate(value: unknown): string {
  if (!value) return '—'
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat('sv-SE', {
    timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
  }).format(date)
}

export function biNumber(value: string | number): string {
  const raw = String(value)
  if (!/^-?\d+(\.\d+)?$/.test(raw)) return raw
  const [integer, fraction] = raw.split('.')
  const grouped = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return fraction === undefined ? grouped : `${grouped}.${fraction}`
}
